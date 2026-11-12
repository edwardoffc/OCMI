package internal

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

func probeRecoveryImages(context layoutContext, j updateProofJournal) error {
	name := "ocmi"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	for _, path := range []string{filepath.Join(context.Root, "system", "bin", name), filepath.Join(context.Root, filepath.FromSlash(j.Stage), "parked-system", "bin", name)} {
		if _, err := os.Lstat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if environmentPlainAncestors(path) != nil || plainPath(path, false) != nil {
			return errors.New("unsafe recovery image")
		}
		if err := probeInstalledImage(path); err != nil {
			return errors.New("recovery image busy")
		}
	}
	return nil
}

type updateCleanupEntry struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	NativeID string `json:"nativeId"`
	Digest   string `json:"digest,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Mode     uint32 `json:"mode"`
}

var updateCleanupTestStep func(string)

func updateCleanupPlan(j updateProofJournal) (map[string]updatePublicEntry, map[string]bool) {
	files := map[string]updatePublicEntry{}
	dirs := map[string]bool{".": true, "old": true, "new": true}
	old, newProof := j.OldPublic, j.NewPublic
	if j.OldStaged != nil {
		old = *j.OldStaged
		newProof = *j.NewStaged
	}
	for _, side := range []struct {
		name  string
		proof updatePublicProof
	}{{"old", old}, {"new", newProof}} {
		for _, e := range side.proof.Files {
			path := side.name + "/" + e.Path
			files[path] = e
			p := filepath.ToSlash(filepath.Dir(filepath.FromSlash(path)))
			for p != "." {
				dirs[p] = true
				p = filepath.ToSlash(filepath.Dir(filepath.FromSlash(p)))
			}
		}
	}
	return files, dirs
}

func validateUpdateCleanupJournal(j updateProofJournal) error {
	if j.CleanupState == "" {
		if len(j.CleanupEntries) != 0 {
			return errors.New("unexpected cleanup inventory")
		}
		return nil
	}
	if j.Phase != "rolled_back" || (j.CleanupState != "pending" && j.CleanupState != "done") {
		return errors.New("invalid cleanup state")
	}
	if j.CleanupState == "done" {
		if len(j.CleanupEntries) != 0 {
			return errors.New("finished cleanup inventory")
		}
		return nil
	}
	if len(j.CleanupEntries) == 0 {
		return nil
	}
	if j.StageID == "" || len(j.CleanupEntries) > 16384 {
		return errors.New("invalid cleanup inventory")
	}
	files, dirs := updateCleanupPlan(j)
	seen := map[string]bool{}
	for _, e := range j.CleanupEntries {
		if seen[e.Path] || e.NativeID == "" || len(e.NativeID) > 512 || e.Mode > 0777 {
			return errors.New("invalid cleanup member")
		}
		seen[e.Path] = true
		if e.Kind == "directory" {
			if !dirs[e.Path] || e.Digest != "" || e.Size != 0 {
				return errors.New("invalid cleanup directory")
			}
			if e.Path == "." && e.NativeID != j.StageID || e.Path == "old" && e.NativeID != j.OldSlotID || e.Path == "new" && e.NativeID != j.NewSlotID {
				return errors.New("cleanup directory identity")
			}
			if j.OldStaged != nil && (e.Path == "old/system" && e.NativeID != j.OldStaged.SystemID || e.Path == "new/system" && e.NativeID != j.NewStaged.SystemID) {
				return errors.New("cleanup system identity")
			}
		} else if e.Kind == "file" {
			p, ok := files[e.Path]
			if !ok || e.Digest != p.Digest || e.Size != p.Size || e.Mode != p.Mode || j.OldStaged != nil && e.NativeID != p.NativeID {
				return errors.New("cleanup file proof")
			}
		} else {
			return errors.New("invalid cleanup member")
		}
	}
	if !seen["."] || !seen["old"] || !seen["new"] {
		return errors.New("missing cleanup roots")
	}
	return nil
}

func verifyRestoredUpdateHome(context layoutContext, j updateProofJournal) error {
	if err := verifyUpdatePublicProof(context.Root, j.OldPublic, true, false); err != nil {
		return err
	}
	if err := verifyUpdateSystem(filepath.Join(context.Root, "system"), j.OldPublic); err != nil {
		return err
	}
	backup := map[string]updatePublicEntry{}
	if j.OldStaged != nil {
		for _, e := range updateRootEntries(*j.OldStaged) {
			backup[e.Path] = e
		}
	}
	for _, e := range updateRootEntries(j.OldPublic) {
		if !updateEntryMatches(context.Root, e) {
			b, ok := backup[e.Path]
			if !ok || !updateEntryMatches(context.Root, b) {
				return errors.New("restored root identity")
			}
		}
	}
	if err := verifyUpdateMetadataBefore(context, j.Metadata); err != nil {
		return err
	}
	return verifyRecoveryMetadata(context, j.Metadata)
}

func verifyUpdateCleanupInventory(context layoutContext, j updateProofJournal) error {
	if err := validateUpdateCleanupJournal(j); err != nil {
		return err
	}
	stage := filepath.Join(context.Root, filepath.FromSlash(j.Stage))
	if j.CleanupState == "done" {
		if !updatePathAbsent(stage) {
			return errors.New("finished stage occupied")
		}
		return nil
	}
	if j.CleanupState != "pending" {
		return errors.New("cleanup not pending")
	}
	if len(j.CleanupEntries) == 0 {
		if j.StageID == "" {
			if !updatePathAbsent(stage) {
				return errors.New("unattributed stage")
			}
			return nil
		}
		if j.RecoveryFrom == "intent" {
			return verifyUpdateStage(context, j, true)
		}
		return verifyUpdateRecoveryState(context, j)
	}
	if updatePathAbsent(stage) {
		return nil
	}
	if environmentPlainAncestors(stage) != nil {
		return errors.New("unsafe cleanup stage")
	}
	expected := map[string]updateCleanupEntry{}
	for _, e := range j.CleanupEntries {
		expected[e.Path] = e
	}
	return filepath.WalkDir(stage, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		e, ok := expected[rel]
		if !ok {
			return errors.New("unknown cleanup member")
		}
		if d.Type()&os.ModeSymlink != 0 || d.Type()&os.ModeIrregular != 0 || environmentPlainAncestors(path) != nil {
			return errors.New("linked cleanup member")
		}
		info, err := os.Lstat(path)
		if err != nil || uint32(info.Mode().Perm()) != e.Mode || (e.Kind == "directory") != info.IsDir() || e.Kind == "file" && !info.Mode().IsRegular() {
			return errors.New("cleanup mode")
		}
		id, err := localNativeID(path)
		if err != nil || id != e.NativeID {
			return errors.New("cleanup identity")
		}
		if e.Kind == "file" {
			raw, err := installationRead(stage, rel, 32<<20)
			if err != nil || int64(len(raw)) != e.Size || updateDigest(raw) != e.Digest {
				return errors.New("cleanup content")
			}
		}
		return nil
	})
}

func verifyUpdateCleanupMember(stage string, e updateCleanupEntry) error {
	path := stage
	if e.Path != "." {
		path = filepath.Join(stage, filepath.FromSlash(e.Path))
	}
	if environmentPlainAncestors(path) != nil {
		return errors.New("unsafe cleanup member")
	}
	info, err := os.Lstat(path)
	if err != nil || uint32(info.Mode().Perm()) != e.Mode || (e.Kind == "directory") != info.IsDir() || e.Kind == "file" && !info.Mode().IsRegular() {
		return errors.New("cleanup member changed")
	}
	id, err := localNativeID(path)
	if err != nil || id != e.NativeID {
		return errors.New("cleanup identity changed")
	}
	if e.Kind == "file" {
		raw, err := installationRead(stage, e.Path, 32<<20)
		if err != nil || int64(len(raw)) != e.Size || updateDigest(raw) != e.Digest {
			return errors.New("cleanup content changed")
		}
	} else {
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) != 0 {
			return errors.New("cleanup directory not empty")
		}
	}
	return nil
}

func cleanupUpdateStage(context layoutContext, j *updateProofJournal, writer *heldUpdateWriter, lock *ownedSetupLock) error {
	if j == nil || j.Phase != "rolled_back" || writer == nil || writer.conn == nil || writer.db == nil || writer.homeID != j.Metadata.HomeID || lock == nil || lock.nativeID != j.AdmissionID || lock.path != filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"]), ".setup-lock") {
		return errors.New("cleanup admission")
	}
	if environmentPlainAncestors(lock.path) != nil || plainPath(lock.path, true) != nil {
		return errors.New("unsafe admission")
	}
	id, err := localNativeID(lock.path)
	if err != nil || id != j.AdmissionID {
		return errors.New("admission identity")
	}
	members, err := os.ReadDir(lock.path)
	if err != nil || len(members) != 0 {
		return errors.New("admission contents")
	}
	if err = verifyRestoredUpdateHome(context, *j); err != nil {
		return err
	}
	stage := filepath.Join(context.Root, filepath.FromSlash(j.Stage))
	if j.CleanupState == "done" {
		return verifyUpdateCleanupInventory(context, *j)
	}
	if j.CleanupState == "" || j.CleanupState == "pending" && len(j.CleanupEntries) == 0 {
		if j.StageID == "" {
			if !updatePathAbsent(stage) {
				return errors.New("unattributed stage")
			}
			j.CleanupState = "done"
			return persistUpdateProofJournal(context, *j)
		}
		if j.RecoveryFrom == "intent" {
			if err = verifyUpdateStage(context, *j, true); err != nil {
				return err
			}
		} else {
			if err = verifyUpdateRecoveryState(context, *j); err != nil {
				return err
			}
		}
		var captured []updateCleanupEntry
		err = filepath.WalkDir(stage, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(stage, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if d.Type()&os.ModeSymlink != 0 || d.Type()&os.ModeIrregular != 0 || environmentPlainAncestors(path) != nil {
				return errors.New("unsafe cleanup member")
			}
			info, err := os.Lstat(path)
			if err != nil {
				return err
			}
			id, err := localNativeID(path)
			if err != nil {
				return err
			}
			e := updateCleanupEntry{Path: rel, NativeID: id, Mode: uint32(info.Mode().Perm())}
			if d.IsDir() {
				e.Kind = "directory"
			} else {
				if !info.Mode().IsRegular() {
					return errors.New("cleanup file type")
				}
				raw, err := installationRead(stage, rel, 32<<20)
				if err != nil {
					return err
				}
				e.Kind = "file"
				e.Digest = updateDigest(raw)
				e.Size = int64(len(raw))
			}
			captured = append(captured, e)
			return nil
		})
		if err != nil {
			return err
		}
		j.CleanupState = "pending"
		j.CleanupEntries = captured
		if err = validateUpdateCleanupJournal(*j); err != nil {
			return err
		}
		if err = verifyUpdateCleanupInventory(context, *j); err != nil {
			return err
		}
		if err = persistUpdateProofJournal(context, *j); err != nil {
			return err
		}
	}
	if err = verifyUpdateCleanupInventory(context, *j); err != nil {
		return err
	}
	entries := append([]updateCleanupEntry(nil), j.CleanupEntries...)
	sort.Slice(entries, func(i, k int) bool {
		di, dk := strings.Count(entries[i].Path, "/"), strings.Count(entries[k].Path, "/")
		if entries[i].Path == "." {
			di = -1
		}
		if entries[k].Path == "." {
			dk = -1
		}
		if di != dk {
			return di > dk
		}
		return entries[i].Kind == "file" && entries[k].Kind == "directory"
	})
	for _, e := range entries {
		path := stage
		if e.Path != "." {
			path = filepath.Join(stage, filepath.FromSlash(e.Path))
		}
		if updatePathAbsent(path) {
			continue
		}
		if err = verifyUpdateCleanupMember(stage, e); err != nil {
			return err
		}
		if err = os.Remove(path); err != nil {
			return err
		}
		if updateCleanupTestStep != nil {
			updateCleanupTestStep(e.Path)
		}
	}
	j.CleanupState = "done"
	j.CleanupEntries = nil
	return persistUpdateProofJournal(context, *j)
}
