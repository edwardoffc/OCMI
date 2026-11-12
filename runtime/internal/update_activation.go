package internal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func verifyUpdateRootMembers(root string, proof updatePublicProof, allowPrivate bool) error {
	id, err := localNativeID(root)
	if err != nil || id != proof.RootID {
		return updateObserved("root identity", err, "native_id")
	}
	want := map[string]updatePublicEntry{}
	for _, e := range proof.Files {
		if !strings.HasPrefix(e.Path, "system/") {
			want[e.Path] = e
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return updateObserved("stage inventory", err, "read_dir")
	}
	seen := 0
	for _, entry := range entries {
		name := entry.Name()
		if name == "system" {
			continue
		}
		if allowPrivate && (name == "local" || name == "OCMI_Sync") {
			if err := plainPath(filepath.Join(root, name), true); err != nil {
				return updateObserved("private root", err, "path_check")
			}
			continue
		}
		expected, ok := want[name]
		if !ok || !entry.Type().IsRegular() {
			return updateMismatch("root member")
		}
		raw, err := installationRead(root, name, 32<<20)
		if err != nil || int64(len(raw)) != expected.Size || updateDigest(raw) != expected.Digest {
			return updateObserved("root content", err, "read")
		}
		info, err := os.Lstat(filepath.Join(root, name))
		if err != nil || uint32(info.Mode().Perm()) != expected.Mode {
			return updateObserved("root mode", err, "stat")
		}
		id, err := localNativeID(filepath.Join(root, name))
		if err != nil || id != expected.NativeID {
			return updateObserved("root member identity", err, "native_id")
		}
		seen++
	}
	if seen != len(want) {
		return updateMismatch("missing root member")
	}
	return nil
}

// verifyUpdateActivationState checks the split system and root inventories
// at the two physical boundaries, without asking a mixed factory to verify.
func verifyUpdateActivationState(context layoutContext, j updateProofJournal) error {
	if j.Phase != "old_system" && j.Phase != "new_system" {
		return updateMismatch("activation phase")
	}
	if _, err := encodeUpdateProofJournal(j); err != nil {
		return err
	}
	stage := filepath.Join(context.Root, filepath.FromSlash(j.Stage))
	if err := environmentPlainAncestors(stage); err != nil {
		return updateObserved("stage path", err, "path_check")
	}
	id, err := localNativeID(stage)
	if err != nil || id != j.StageID {
		return updateObserved("stage identity", err, "native_id")
	}
	entries, err := os.ReadDir(stage)
	if err != nil || len(entries) != 3 {
		return updateObserved("stage inventory", err, "read_dir")
	}
	for _, e := range entries {
		if !e.IsDir() || (e.Name() != "old" && e.Name() != "new" && e.Name() != "parked-system") {
			return updateMismatch("stage member")
		}
	}
	if err = verifyUpdatePublicProof(filepath.Join(stage, "old"), *j.OldStaged, false, true); err != nil {
		return err
	}
	if err = verifyUpdateSystem(filepath.Join(stage, "parked-system"), j.OldPublic); err != nil {
		return err
	}
	if err = verifyUpdateRootMembers(context.Root, j.OldPublic, true); err != nil {
		return err
	}
	if j.Phase == "old_system" {
		if _, err = os.Lstat(filepath.Join(context.Root, "system")); !os.IsNotExist(err) {
			return updateObserved("old system phase", err, "stat")
		}
		return verifyUpdatePublicProof(filepath.Join(stage, "new"), *j.NewStaged, false, true)
	}
	if _, err = os.Lstat(filepath.Join(stage, "new", "system")); !os.IsNotExist(err) {
		return updateObserved("new system phase", err, "stat")
	}
	if err = verifyUpdateRootMembers(filepath.Join(stage, "new"), *j.NewStaged, false); err != nil {
		return err
	}
	return verifyUpdateSystem(filepath.Join(context.Root, "system"), *j.NewStaged)
}

// verifyUpdateSystem checks a moved system against its captured inventory and
// native identities without consulting a potentially mixed root SHA256SUMS.
func verifyUpdateSystem(root string, proof updatePublicProof) error {
	if err := environmentPlainAncestors(root); err != nil {
		return updateObserved("unsafe system", err, "path_check")
	}
	if err := plainPath(root, true); err != nil {
		return updateObserved("unsafe system", err, "path_check")
	}
	id, err := localNativeID(root)
	if err != nil || id != proof.SystemID {
		return updateObserved("system identity", err, "native_id")
	}
	want := map[string]updatePublicEntry{}
	dirs := map[string]bool{}
	for _, entry := range proof.Files {
		if !strings.HasPrefix(entry.Path, "system/") {
			continue
		}
		name := strings.TrimPrefix(entry.Path, "system/")
		want[name] = entry
		p := filepath.ToSlash(filepath.Dir(filepath.FromSlash(name)))
		for p != "." {
			dirs[p] = true
			p = filepath.ToSlash(filepath.Dir(filepath.FromSlash(p)))
		}
	}
	seen := map[string]bool{}
	used := map[string]bool{}
	err = filepath.WalkDir(root, func(path string, e os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if e.Type()&os.ModeSymlink != 0 || e.Type()&os.ModeIrregular != 0 {
			return updateMismatch("system link")
		}
		if err := plainComponents(root, filepath.FromSlash(rel), false); err != nil {
			return updateObserved("system link", err, "path_check")
		}
		if e.IsDir() {
			if !dirs[rel] {
				return updateMismatch("system directory")
			}
			return nil
		}
		expected, ok := want[rel]
		if !ok || !e.Type().IsRegular() {
			return updateMismatch("system member")
		}
		raw, err := installationRead(root, rel, 32<<20)
		if err != nil || int64(len(raw)) != expected.Size || updateDigest(raw) != expected.Digest {
			return updateObserved("system content", err, "read")
		}
		info, err := os.Lstat(path)
		if err != nil || uint32(info.Mode().Perm()) != expected.Mode {
			return updateObserved("system mode", err, "stat")
		}
		id, err := localNativeID(path)
		if err != nil || id != expected.NativeID {
			return updateObserved("system member identity", err, "native_id")
		}
		seen[rel] = true
		p := filepath.ToSlash(filepath.Dir(filepath.FromSlash(rel)))
		for p != "." {
			used[p] = true
			p = filepath.ToSlash(filepath.Dir(filepath.FromSlash(p)))
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(seen) != len(want) {
		return updateMismatch("missing system member")
	}
	for d := range dirs {
		if !used[d] {
			return updateMismatch("empty system directory")
		}
	}
	return nil
}

func activateUpdateSystem(context layoutContext, j *updateProofJournal, writer *heldUpdateWriter, lock *ownedSetupLock) error {
	if j == nil || j.Phase != "staged" || writer == nil || writer.conn == nil || writer.db == nil || lock == nil || lock.nativeID != j.AdmissionID || writer.homeID != j.Metadata.HomeID {
		return errors.New("activation admission")
	}
	if lock.path != filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"]), ".setup-lock") || environmentPlainAncestors(lock.path) != nil || plainPath(lock.path, true) != nil {
		return errors.New("activation lock")
	}
	id, err := localNativeID(lock.path)
	if err != nil || id != j.AdmissionID {
		return errors.New("activation lock identity")
	}
	entries, err := os.ReadDir(lock.path)
	if err != nil || len(entries) != 0 {
		return errors.New("activation lock contents")
	}
	if err = verifyUpdateMetadataBefore(context, j.Metadata); err != nil {
		return err
	}
	if err = verifyUpdatePublicProof(context.Root, j.OldPublic, true, true); err != nil {
		return err
	}
	if err = verifyUpdateStage(context, *j, false); err != nil {
		return err
	}
	stage := filepath.Join(context.Root, filepath.FromSlash(j.Stage))
	if j.OldStaged == nil || j.NewStaged == nil {
		return errors.New("missing staged native proofs")
	}
	if err = verifyUpdatePublicProof(filepath.Join(stage, "old"), *j.OldStaged, false, true); err != nil {
		return err
	}
	if err = verifyUpdatePublicProof(filepath.Join(stage, "new"), *j.NewStaged, false, true); err != nil {
		return err
	}
	if err = toolStorageReaderGuardSystem(context, filepath.Join(stage, "new", "system")); err != nil {
		return err
	}
	parked := filepath.Join(stage, "parked-system")
	if _, err = os.Lstat(parked); err == nil || !os.IsNotExist(err) {
		return errors.New("occupied parked system")
	}
	if err = persistUpdateProofJournal(context, *j); err != nil {
		return err
	}
	if err = os.Rename(filepath.Join(context.Root, "system"), parked); err != nil {
		return err
	}
	j.Phase = "old_system"
	if err = persistUpdateProofJournal(context, *j); err != nil {
		return err
	}
	updatePhase("old_system")
	if err = verifyUpdateSystem(parked, j.OldPublic); err != nil {
		return err
	}
	if err = verifyUpdateSystem(filepath.Join(stage, "new", "system"), *j.NewStaged); err != nil {
		return err
	}
	if err = os.Rename(filepath.Join(stage, "new", "system"), filepath.Join(context.Root, "system")); err != nil {
		return err
	}
	j.Phase = "new_system"
	if err = persistUpdateProofJournal(context, *j); err != nil {
		return err
	}
	updatePhase("new_system")
	return verifyUpdateSystem(filepath.Join(context.Root, "system"), *j.NewStaged)
}
