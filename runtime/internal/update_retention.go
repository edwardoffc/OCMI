package internal

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func retirementPlan(p updateProofJournal) (map[string]updatePublicEntry, map[string]bool) {
	files := map[string]updatePublicEntry{}
	dirs := map[string]bool{".": true, "old": true, "new": true, "parked-system": true}
	if p.OldStaged == nil {
		return files, dirs
	}
	for _, e := range p.OldStaged.Files {
		name := "old/" + e.Path
		files[name] = e
		parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(name)))
		for parent != "." {
			dirs[parent] = true
			parent = filepath.ToSlash(filepath.Dir(filepath.FromSlash(parent)))
		}
	}
	for _, e := range p.OldPublic.Files {
		if !strings.HasPrefix(e.Path, "system/") {
			continue
		}
		name := "parked-system/" + strings.TrimPrefix(e.Path, "system/")
		files[name] = e
		parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(name)))
		for parent != "." {
			dirs[parent] = true
			parent = filepath.ToSlash(filepath.Dir(filepath.FromSlash(parent)))
		}
	}
	return files, dirs
}

func validateRetirementJournal(j updateProofJournal) error {
	if j.RetirementState == "" {
		if len(j.RetirementEntries) != 0 {
			return errors.New("unexpected retirement inventory")
		}
		if j.Phase == "applied" && j.Previous != nil {
			return errors.New("unaccounted previous generation")
		}
		return nil
	}
	if j.RetirementState != "pending" || j.Phase != "applied" || j.Previous == nil {
		return errors.New("invalid retirement state")
	}
	if len(j.RetirementEntries) == 0 {
		return nil
	}
	if len(j.RetirementEntries) > 16384 {
		return errors.New("retirement inventory size")
	}
	p := *j.Previous
	files, dirs := retirementPlan(p)
	seen := map[string]bool{}
	for _, e := range j.RetirementEntries {
		if seen[e.Path] || e.NativeID == "" || len(e.NativeID) > 512 || e.Mode > 0777 {
			return errors.New("invalid retirement member")
		}
		seen[e.Path] = true
		if e.Kind == "directory" {
			if !dirs[e.Path] || e.Digest != "" || e.Size != 0 {
				return errors.New("invalid retirement directory")
			}
			if e.Path == "." && e.NativeID != p.StageID || e.Path == "old" && e.NativeID != p.OldSlotID || e.Path == "new" && e.NativeID != p.NewSlotID || e.Path == "old/system" && e.NativeID != p.OldStaged.SystemID || e.Path == "parked-system" && e.NativeID != p.OldPublic.SystemID {
				return errors.New("retirement directory identity")
			}
		} else if e.Kind == "file" {
			want, ok := files[e.Path]
			if !ok || e.NativeID != want.NativeID || e.Digest != want.Digest || e.Size != want.Size || e.Mode != want.Mode {
				return errors.New("retirement file proof")
			}
		} else {
			return errors.New("invalid retirement type")
		}
	}
	for _, name := range []string{".", "old", "new", "parked-system", "old/system"} {
		if !seen[name] {
			return errors.New("missing retirement roots")
		}
	}
	for name := range files {
		if !seen[name] {
			return errors.New("incomplete retirement inventory")
		}
	}
	for name := range dirs {
		if !seen[name] {
			return errors.New("incomplete retirement directories")
		}
	}
	return nil
}

func verifyPreviousStage(context layoutContext, p updateProofJournal) error {
	if p.Phase != "applied" || p.OldStaged == nil || p.NewStaged == nil {
		return errors.New("previous generation")
	}
	stage := filepath.Join(context.Root, filepath.FromSlash(p.Stage))
	if environmentPlainAncestors(stage) != nil {
		return errors.New("unsafe previous stage")
	}
	id, err := localNativeID(stage)
	if err != nil || id != p.StageID {
		return errors.New("previous stage identity")
	}
	entries, err := os.ReadDir(stage)
	if err != nil || len(entries) != 3 {
		return errors.New("previous stage inventory")
	}
	for _, e := range entries {
		if !e.IsDir() || (e.Name() != "old" && e.Name() != "new" && e.Name() != "parked-system") {
			return errors.New("previous stage member")
		}
	}
	if err = verifyUpdatePublicProof(filepath.Join(stage, "old"), *p.OldStaged, false, true); err != nil {
		return err
	}
	if err = verifyUpdateSystem(filepath.Join(stage, "parked-system"), p.OldPublic); err != nil {
		return err
	}
	newSlot := filepath.Join(stage, "new")
	if environmentPlainAncestors(newSlot) != nil || plainPath(newSlot, true) != nil {
		return errors.New("previous slot")
	}
	id, err = localNativeID(newSlot)
	if err != nil || id != p.NewSlotID {
		return errors.New("previous slot identity")
	}
	entries, err = os.ReadDir(newSlot)
	if err != nil || len(entries) != 0 {
		return errors.New("previous slot contents")
	}
	return nil
}

func verifyRetirementInventory(context layoutContext, j updateProofJournal) error {
	if err := validateRetirementJournal(j); err != nil {
		return err
	}
	if j.Previous == nil || j.RetirementState != "pending" {
		return errors.New("retirement not pending")
	}
	if len(j.RetirementEntries) == 0 {
		return verifyPreviousStage(context, *j.Previous)
	}
	stage := filepath.Join(context.Root, filepath.FromSlash(j.Previous.Stage))
	if updatePathAbsent(stage) {
		return nil
	}
	if environmentPlainAncestors(stage) != nil {
		return errors.New("unsafe retired stage")
	}
	expected := map[string]updateCleanupEntry{}
	for _, e := range j.RetirementEntries {
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
			return errors.New("unknown retirement member")
		}
		if d.Type()&os.ModeSymlink != 0 || d.Type()&os.ModeIrregular != 0 || environmentPlainAncestors(path) != nil {
			return errors.New("linked retirement member")
		}
		info, err := os.Lstat(path)
		if err != nil || uint32(info.Mode().Perm()) != e.Mode || (e.Kind == "directory") != info.IsDir() || e.Kind == "file" && !info.Mode().IsRegular() {
			return errors.New("retirement mode")
		}
		id, err := localNativeID(path)
		if err != nil || id != e.NativeID {
			return errors.New("retirement identity")
		}
		if e.Kind == "file" {
			raw, err := installationRead(stage, rel, 32<<20)
			if err != nil || int64(len(raw)) != e.Size || updateDigest(raw) != e.Digest {
				return errors.New("retirement content")
			}
		}
		return nil
	})
}

// retirePreviousStage persists only the outer current journal. The previous
// generation never supplies its old admission as deletion authority.
func retirePreviousStage(context layoutContext, j *updateProofJournal, writer *heldUpdateWriter, lock *ownedSetupLock) error {
	if j == nil || j.Phase != "applied" || j.Previous == nil || j.RetirementState != "pending" || writer == nil || writer.conn == nil || writer.db == nil || writer.homeID != j.Metadata.HomeID || lock == nil || lock.nativeID != j.AdmissionID || lock.path != filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"]), ".setup-lock") {
		return errors.New("retirement admission")
	}
	if environmentPlainAncestors(lock.path) != nil || plainPath(lock.path, true) != nil {
		return errors.New("unsafe admission")
	}
	id, err := localNativeID(lock.path)
	if err != nil || id != j.AdmissionID {
		return errors.New("admission identity")
	}
	entries, err := os.ReadDir(lock.path)
	if err != nil || len(entries) != 0 {
		return errors.New("admission contents")
	}
	if err = verifyUpdateRootPlacement(context, *j); err != nil {
		return err
	}
	if err = verifyUpdateMetadataPlacement(context, *j); err != nil {
		return err
	}
	if err = verifyRetirementInventory(context, *j); err != nil {
		return err
	}
	stage := filepath.Join(context.Root, filepath.FromSlash(j.Previous.Stage))
	if len(j.RetirementEntries) == 0 {
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
				return errors.New("unsafe retirement member")
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
					return errors.New("retirement file type")
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
		j.RetirementEntries = captured
		if err = validateRetirementJournal(*j); err != nil {
			return err
		}
		if err = verifyRetirementInventory(context, *j); err != nil {
			return err
		}
		if err = persistUpdateProofJournal(context, *j); err != nil {
			return err
		}
	}
	if err = verifyRetirementInventory(context, *j); err != nil {
		return err
	}
	members := append([]updateCleanupEntry(nil), j.RetirementEntries...)
	sort.Slice(members, func(i, k int) bool {
		di, dk := strings.Count(members[i].Path, "/"), strings.Count(members[k].Path, "/")
		if members[i].Path == "." {
			di = -1
		}
		if members[k].Path == "." {
			dk = -1
		}
		if di != dk {
			return di > dk
		}
		return members[i].Kind == "file" && members[k].Kind == "directory"
	})
	for _, e := range members {
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
	p := j.Previous
	j.LastRetired = &updateGenerationFact{ID: p.ID, OldSource: p.OldSource, NewSource: p.NewSource, Phase: "retired"}
	j.Previous = nil
	j.RetirementState = ""
	j.RetirementEntries = nil
	return persistUpdateProofJournal(context, *j)
}
