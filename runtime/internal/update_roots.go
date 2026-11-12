package internal

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func updateRootEntries(p updatePublicProof) []updatePublicEntry {
	var out []updatePublicEntry
	for _, e := range p.Files {
		if !strings.HasPrefix(e.Path, "system/") {
			out = append(out, e)
		}
	}
	return out
}
func updateEntryMatches(root string, e updatePublicEntry) bool {
	return observeUpdateEntry(root, e) == nil
}
func observeUpdateEntry(root string, e updatePublicEntry) error {
	raw, err := installationRead(root, e.Path, 32<<20)
	if err != nil || int64(len(raw)) != e.Size || updateDigest(raw) != e.Digest {
		return updateObserved("root content", err, "read")
	}
	path := filepath.Join(root, filepath.FromSlash(e.Path))
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || uint32(info.Mode().Perm()) != e.Mode {
		return updateObserved("root mode", err, "stat")
	}
	id, err := localNativeID(path)
	if err != nil || id != e.NativeID {
		return updateObserved("root member identity", err, "native_id")
	}
	return nil
}

// verifyUpdateRootPlacement accepts only a captured old home member paired
// with its staged successor, or that same successor moved into the home.
func verifyUpdateRootPlacement(context layoutContext, j updateProofJournal) error {
	if _, err := encodeUpdateProofJournal(j); err != nil {
		return err
	}
	if j.NewStaged == nil || j.OldStaged == nil {
		return updateMismatch("missing stage proof")
	}
	stage := filepath.Join(context.Root, filepath.FromSlash(j.Stage))
	if err := environmentPlainAncestors(stage); err != nil {
		return updateObserved("unsafe stage", err, "path_check")
	}
	id, err := localNativeID(stage)
	if err != nil || id != j.StageID {
		return updateObserved("stage identity", err, "native_id")
	}
	dirs, err := os.ReadDir(stage)
	if err != nil || len(dirs) != 3 {
		return updateObserved("stage inventory", err, "read_dir")
	}
	for _, e := range dirs {
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
	if err = verifyUpdateSystem(filepath.Join(context.Root, "system"), *j.NewStaged); err != nil {
		return err
	}
	id, err = localNativeID(context.Root)
	if err != nil || id != j.Metadata.HomeID {
		return updateObserved("home identity", err, "native_id")
	}
	newRoot := filepath.Join(stage, "new")
	id, err = localNativeID(newRoot)
	if err != nil || id != j.NewSlotID {
		return updateObserved("new slot identity", err, "native_id")
	}
	if err := environmentPlainAncestors(newRoot); err != nil {
		return updateObserved("new slot identity", err, "path_check")
	}
	oldEntries, newEntries := updateRootEntries(j.OldPublic), updateRootEntries(*j.NewStaged)
	if len(oldEntries) != len(newEntries) {
		return updateMismatch("root inventory mismatch")
	}
	homeAllowed := map[string]bool{"system": true, "local": true, "OCMI_Sync": true}
	slotAllowed := map[string]bool{}
	for i, newEntry := range newEntries {
		oldEntry := oldEntries[i]
		if oldEntry.Path != newEntry.Path || strings.Contains(newEntry.Path, "/") {
			return updateMismatch("root inventory mismatch")
		}
		homeAllowed[newEntry.Path] = true
		slotAllowed[newEntry.Path] = true
		oldObservation := observeUpdateEntry(context.Root, oldEntry)
		newObservation := observeUpdateEntry(context.Root, newEntry)
		slotObservation := observeUpdateEntry(newRoot, newEntry)
		oldInHome, newInHome, newInSlot := oldObservation == nil, newObservation == nil, slotObservation == nil
		if newInHome {
			if _, err := os.Lstat(filepath.Join(newRoot, filepath.FromSlash(newEntry.Path))); !os.IsNotExist(err) {
				return updateObserved("occupied moved root slot", err, "stat")
			}
		}
		if oldInHome == newInHome || oldInHome != newInSlot {
			if i < j.RootProgress && newObservation != nil {
				return newObservation
			}
			if i > j.RootProgress && oldObservation != nil {
				return oldObservation
			}
			return updatePlacementFailure("root placement", oldObservation, newObservation, slotObservation)
		}
		if i < j.RootProgress && !newInHome || i > j.RootProgress && !oldInHome {
			return updateMismatch("root progress")
		}
	}
	for _, item := range []struct {
		root    string
		allowed map[string]bool
	}{{context.Root, homeAllowed}, {newRoot, slotAllowed}} {
		entries, err := os.ReadDir(item.root)
		if err != nil {
			return updateObserved("stage inventory", err, "read_dir")
		}
		for _, e := range entries {
			if !item.allowed[e.Name()] {
				return updateMismatch("unknown root member")
			}
			if item.root == context.Root && (e.Name() == "system" || e.Name() == "local" || e.Name() == "OCMI_Sync") {
				if err := plainPath(filepath.Join(item.root, e.Name()), true); err != nil {
					return updateObserved("unsafe root directory", err, "path_check")
				}
			} else if !e.Type().IsRegular() {
				return updateMismatch("unsafe root member")
			}
		}
	}
	return nil
}

type updateMetadataMember struct {
	path          string
	before, after []byte
}

func updateMetadataMembers(context layoutContext, p updateMetadataProof) []updateMetadataMember {
	out := []updateMetadataMember{{context.Paths["bindings"] + "/" + installationReceiptName, p.ReceiptBefore, p.ReceiptAfter}}
	if len(p.BindingBefore) > 0 {
		out = append(out, updateMetadataMember{context.Paths["bindings"] + "/coordinator/coordinator.json", p.BindingBefore, p.BindingAfter}, updateMetadataMember{context.Paths["bindings"] + "/coordinator/STARTUP.md", p.PromptBefore, p.PromptAfter})
	}
	return out
}
func verifyUpdateMetadataPlacement(context layoutContext, j updateProofJournal) error {
	for i, m := range updateMetadataMembers(context, j.Metadata) {
		raw, err := installationRead(context.Root, m.path, 1<<20)
		if err != nil {
			return updateObserved("metadata unavailable", err, "read")
		}
		before, after := bytes.Equal(raw, m.before), bytes.Equal(raw, m.after)
		if !before && !after || i < j.MetadataProgress && !after || i > j.MetadataProgress && !before {
			return updateMismatch("metadata changed")
		}
	}
	if len(j.Metadata.BindingBefore) == 0 {
		dir := filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"]), "coordinator")
		if _, err := os.Lstat(dir); err == nil || !os.IsNotExist(err) {
			return updateObserved("unexpected local coordinator", err, "stat")
		}
	}
	return nil
}

func activateUpdateRootsAndMetadata(context layoutContext, j *updateProofJournal, writer *heldUpdateWriter, lock *ownedSetupLock) error {
	if j == nil || j.Phase != "new_system" || writer == nil || writer.conn == nil || writer.db == nil || writer.homeID != j.Metadata.HomeID || lock == nil || lock.nativeID != j.AdmissionID || lock.path != filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"]), ".setup-lock") {
		return errors.New("activation admission")
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
	j.Phase = "root_switch"
	if err = persistUpdateProofJournal(context, *j); err != nil {
		return err
	}
	newRoot := filepath.Join(context.Root, filepath.FromSlash(j.Stage), "new")
	roots := updateRootEntries(*j.NewStaged)
	for i, e := range roots {
		if err = verifyUpdateRootPlacement(context, *j); err != nil {
			return err
		}
		if err = verifyUpdateMetadataPlacement(context, *j); err != nil {
			return err
		}
		if err = os.Rename(filepath.Join(newRoot, filepath.FromSlash(e.Path)), filepath.Join(context.Root, filepath.FromSlash(e.Path))); err != nil {
			return err
		}
		j.RootProgress = i + 1
		if err = persistUpdateProofJournal(context, *j); err != nil {
			return err
		}
	}
	j.Phase = "roots"
	if err = persistUpdateProofJournal(context, *j); err != nil {
		return err
	}
	if err = verifyUpdateRootPlacement(context, *j); err != nil {
		return err
	}
	if err = verifyPublicInventory(context.Root); err != nil {
		return err
	}
	updatePhase("roots")
	j.Phase = "metadata_switch"
	if err = persistUpdateProofJournal(context, *j); err != nil {
		return err
	}
	for i, m := range updateMetadataMembers(context, j.Metadata) {
		if err = verifyUpdateRootPlacement(context, *j); err != nil {
			return err
		}
		if err = verifyUpdateMetadataPlacement(context, *j); err != nil {
			return err
		}
		raw, err := installationRead(context.Root, m.path, 1<<20)
		if err != nil || !bytes.Equal(raw, m.before) {
			return errors.New("metadata changed")
		}
		if err = installationWrite(context.Root, m.path, m.after); err != nil {
			return err
		}
		j.MetadataProgress = i + 1
		if err = persistUpdateProofJournal(context, *j); err != nil {
			return err
		}
	}
	j.Phase = "metadata"
	if err = persistUpdateProofJournal(context, *j); err != nil {
		return err
	}
	if err = verifyUpdateMetadataPlacement(context, *j); err != nil {
		return err
	}
	if err = verifyUpdateRootPlacement(context, *j); err != nil {
		return err
	}
	if err = verifyPublicInventory(context.Root); err != nil {
		return err
	}
	updatePhase("metadata")
	j.Phase = "applied"
	if j.Previous != nil {
		j.RetirementState = "pending"
	}
	if err = persistUpdateProofJournal(context, *j); err != nil {
		return err
	}
	updatePhase("applied")
	return nil
}
