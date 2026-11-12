package internal

import (
	"bytes"
	"os"
	"path/filepath"
)

func updatePathAbsent(path string) bool { absent, _ := observeUpdateAbsence(path); return absent }
func observeUpdateAbsence(path string) (bool, error) {
	_, err := os.Lstat(path)
	return os.IsNotExist(err), err
}

func verifyRecoveryMetadata(context layoutContext, p updateMetadataProof) error {
	for _, m := range updateMetadataMembers(context, p) {
		raw, err := installationRead(context.Root, m.path, 1<<20)
		if err != nil || (!bytes.Equal(raw, m.before) && !bytes.Equal(raw, m.after)) {
			return updateObserved("unknown metadata", err, "read")
		}
	}
	if len(p.BindingBefore) == 0 {
		path := filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"]), "coordinator")
		if absent, err := observeUpdateAbsence(path); !absent {
			return updateObserved("unknown coordinator", err, "stat")
		}
	}
	return nil
}

// verifyUpdateRecoveryState validates every accessible public member and all
// stage children before recovery makes an effect. Private trees stay opaque.
func verifyUpdateRecoveryState(context layoutContext, j updateProofJournal) (result error) {
	defer func() { result = updateValidator(result, "recovery_state") }()
	if _, err := encodeUpdateProofJournal(j); err != nil {
		return err
	}
	if j.OldStaged == nil || j.NewStaged == nil {
		return updateMismatch("missing staged proof")
	}
	home := context.Root
	stage := filepath.Join(home, filepath.FromSlash(j.Stage))
	if err := environmentPlainAncestors(stage); err != nil {
		return updateObserved("unsafe stage", err, "path_check")
	}
	id, err := localNativeID(stage)
	if err != nil || id != j.StageID {
		return updateObserved("stage identity", err, "native_id")
	}
	id, err = localNativeID(home)
	if err != nil || id != j.Metadata.HomeID {
		return updateObserved("home identity", err, "native_id")
	}
	oldRoot, newRoot := filepath.Join(stage, "old"), filepath.Join(stage, "new")
	for _, slot := range []struct{ path, id string }{{oldRoot, j.OldSlotID}, {newRoot, j.NewSlotID}} {
		if err := environmentPlainAncestors(slot.path); err != nil {
			return updateObserved("unsafe slot", err, "path_check")
		}
		if err := plainPath(slot.path, true); err != nil {
			return updateObserved("unsafe slot", err, "path_check")
		}
		id, err = localNativeID(slot.path)
		if err != nil || id != slot.id {
			return updateObserved("slot identity", err, "native_id")
		}
	}
	if err = verifyUpdateSystem(filepath.Join(oldRoot, "system"), *j.OldStaged); err != nil {
		return err
	}
	originalHomeObservation := verifyUpdateSystem(filepath.Join(home, "system"), j.OldPublic)
	originalParkedObservation := verifyUpdateSystem(filepath.Join(stage, "parked-system"), j.OldPublic)
	newHomeObservation := verifyUpdateSystem(filepath.Join(home, "system"), *j.NewStaged)
	newStageObservation := verifyUpdateSystem(filepath.Join(newRoot, "system"), *j.NewStaged)
	originalHome, originalParked := originalHomeObservation == nil, originalParkedObservation == nil
	newHome, newStage := newHomeObservation == nil, newStageObservation == nil
	if originalHome == originalParked || newHome == newStage || originalHome && newHome {
		return updatePlacementFailure("system placement", originalHomeObservation, originalParkedObservation, newHomeObservation, newStageObservation)
	}
	if !originalHome && !newHome {
		if absent, err := observeUpdateAbsence(filepath.Join(home, "system")); !absent {
			return updateObserved("unknown home system", err, "stat")
		}
	}
	if !originalParked {
		if absent, err := observeUpdateAbsence(filepath.Join(stage, "parked-system")); !absent {
			return updateObserved("unknown parked system", err, "stat")
		}
	}
	if !newStage {
		if absent, err := observeUpdateAbsence(filepath.Join(newRoot, "system")); !absent {
			return updateObserved("unknown staged system", err, "stat")
		}
	}
	oldEntries, backupEntries, newEntries := updateRootEntries(j.OldPublic), updateRootEntries(*j.OldStaged), updateRootEntries(*j.NewStaged)
	if len(oldEntries) != len(backupEntries) || len(oldEntries) != len(newEntries) {
		return updateMismatch("root inventories")
	}
	allowed := map[string]bool{}
	for i, old := range oldEntries {
		backup, next := backupEntries[i], newEntries[i]
		if old.Path != backup.Path || old.Path != next.Path {
			return updateMismatch("root inventories")
		}
		allowed[old.Path] = true
		homeOldObservation := observeUpdateEntry(home, old)
		homeBackupObservation := observeUpdateEntry(home, backup)
		homeNewObservation := observeUpdateEntry(home, next)
		oldSlotObservation := observeUpdateEntry(oldRoot, backup)
		newSlotObservation := observeUpdateEntry(newRoot, next)
		homeOld, homeBackup, homeNew := homeOldObservation == nil, homeBackupObservation == nil, homeNewObservation == nil
		oldSlot, newSlot := oldSlotObservation == nil, newSlotObservation == nil
		homeAbsent := updatePathAbsent(filepath.Join(home, filepath.FromSlash(old.Path)))
		oldAbsent := updatePathAbsent(filepath.Join(oldRoot, filepath.FromSlash(old.Path)))
		newAbsent := updatePathAbsent(filepath.Join(newRoot, filepath.FromSlash(old.Path)))
		if !((homeOld && oldSlot && newSlot) || (homeNew && oldSlot && newAbsent) || (homeAbsent && oldSlot && newSlot) || (homeBackup && oldAbsent && newSlot)) {
			return updatePlacementFailure("unknown root placement", homeOldObservation, homeBackupObservation, homeNewObservation, oldSlotObservation, newSlotObservation)
		}
	}
	for _, item := range []struct {
		root    string
		private bool
		system  bool
	}{{home, true, true}, {oldRoot, false, true}, {newRoot, false, true}, {stage, false, false}} {
		entries, err := os.ReadDir(item.root)
		if err != nil {
			return updateObserved("stage inventory", err, "read_dir")
		}
		for _, e := range entries {
			name := e.Name()
			if item.root == stage {
				if name != "old" && name != "new" && name != "parked-system" {
					return updateMismatch("unknown stage member")
				}
				if !e.IsDir() {
					return updateMismatch("unsafe stage member")
				}
				continue
			}
			if name == "system" && item.system {
				if err := plainPath(filepath.Join(item.root, name), true); err != nil {
					return updateObserved("unsafe system", err, "path_check")
				}
				continue
			}
			if item.private && (name == "local" || name == "OCMI_Sync") {
				if err := plainPath(filepath.Join(item.root, name), true); err != nil {
					return updateObserved("unsafe private root", err, "path_check")
				}
				continue
			}
			if !allowed[name] || !e.Type().IsRegular() {
				return updateMismatch("unknown root member")
			}
		}
	}
	return verifyRecoveryMetadata(context, j.Metadata)
}

func verifyUpdateRecoveryStart(context layoutContext, j updateProofJournal) (result error) {
	defer func() { result = updateValidator(result, "recovery_start") }()
	switch j.Phase {
	case "staged":
		if err := verifyUpdatePublicProof(context.Root, j.OldPublic, true, true); err == nil {
			if err = verifyUpdateStage(context, j, false); err == nil {
				return verifyUpdateMetadataBefore(context, j.Metadata)
			}
		}
		// The first rename can be durable while the staged journal remains.
		ahead := j
		ahead.Phase = "old_system"
		if err := verifyUpdateActivationState(context, ahead); err != nil {
			return err
		}
		return verifyUpdateMetadataBefore(context, j.Metadata)
	case "old_system", "new_system":
		if err := verifyUpdateActivationState(context, j); err != nil {
			if j.Phase != "old_system" {
				return err
			}
			ahead := j
			ahead.Phase = "new_system"
			if err = verifyUpdateActivationState(context, ahead); err != nil {
				return err
			}
		}
		return verifyUpdateMetadataBefore(context, j.Metadata)
	case "root_switch", "roots", "metadata_switch", "metadata", "applied":
		if err := verifyUpdateRootPlacement(context, j); err != nil {
			return updateValidator(err, "recovery_start.root_placement")
		}
		return updateValidator(verifyUpdateMetadataPlacement(context, j), "recovery_start.metadata_placement")
	case "recovering":
		return nil
	case "rolled_back":
		if err := verifyPublicInventory(context.Root); err != nil {
			return err
		}
		if err := verifyUpdateSystem(filepath.Join(context.Root, "system"), j.OldPublic); err != nil {
			return err
		}
		for _, m := range updateMetadataMembers(context, j.Metadata) {
			raw, err := installationRead(context.Root, m.path, 1<<20)
			if err != nil || !bytes.Equal(raw, m.before) {
				return updateObserved("metadata not restored", err, "read")
			}
		}
		return nil
	default:
		return updateMismatch("invalid recovery phase")
	}
}

func recoverUpdatePublic(context layoutContext, j *updateProofJournal, writer *heldUpdateWriter, lock *ownedSetupLock) (result error) {
	defer func() { result = updateValidator(result, "recover_public") }()
	if j == nil || writer == nil || writer.conn == nil || writer.db == nil || writer.homeID != j.Metadata.HomeID || lock == nil || lock.nativeID != j.AdmissionID || lock.path != filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"]), ".setup-lock") {
		return updateMismatch("recovery admission")
	}
	if err := environmentPlainAncestors(lock.path); err != nil {
		return updateObserved("unsafe admission", err, "path_check")
	}
	if err := plainPath(lock.path, true); err != nil {
		return updateObserved("unsafe admission", err, "path_check")
	}
	id, err := localNativeID(lock.path)
	if err != nil || id != j.AdmissionID {
		return updateObserved("admission identity", err, "native_id")
	}
	entries, err := os.ReadDir(lock.path)
	if err != nil || len(entries) != 0 {
		return updateObserved("admission contents", err, "read_dir")
	}
	if j.Phase == "intent" || j.Phase == "rolled_back" && j.RecoveryFrom == "intent" {
		if err = verifyUpdatePublicProof(context.Root, j.OldPublic, true, true); err != nil {
			return err
		}
		if err = verifyUpdateMetadataBefore(context, j.Metadata); err != nil {
			return err
		}
		stage := filepath.Join(context.Root, filepath.FromSlash(j.Stage))
		if j.StageID == "" {
			if absent, err := observeUpdateAbsence(stage); !absent {
				return updateObserved("unattributed stage", err, "stat")
			}
		} else {
			if err = verifyUpdateStage(context, *j, true); err != nil {
				return err
			}
		}
		if j.Phase == "rolled_back" {
			return nil
		}
		j.RecoveryFrom = "intent"
		j.Phase = "rolled_back"
		return persistUpdateProofJournal(context, *j)
	}
	if err = verifyUpdateRecoveryStart(context, *j); err != nil {
		return err
	}
	if err = verifyUpdateRecoveryState(context, *j); err != nil {
		return err
	}
	stageForReader := filepath.Join(context.Root, filepath.FromSlash(j.Stage))
	oldSystemForReader := filepath.Join(stageForReader, "parked-system")
	if updatePathAbsent(oldSystemForReader) {
		oldSystemForReader = filepath.Join(stageForReader, "old", "system")
	}
	if err = toolStorageReaderGuardSystem(context, oldSystemForReader); err != nil {
		return err
	}
	if j.Phase == "rolled_back" {
		return nil
	}
	if j.Phase != "recovering" {
		j.RecoveryFrom = j.Phase
		j.Phase = "recovering"
		if err = persistUpdateProofJournal(context, *j); err != nil {
			return err
		}
	}
	stage := filepath.Join(context.Root, filepath.FromSlash(j.Stage))
	newRoot := filepath.Join(stage, "new")
	oldRoot := filepath.Join(stage, "old")
	step := func() error {
		j.RecoveryStep++
		err := persistUpdateProofJournal(context, *j)
		if err != nil {
			return updateObserved("recovery journal", err, "write")
		}
		return nil
	}
	if verifyUpdateSystem(filepath.Join(context.Root, "system"), *j.NewStaged) == nil {
		if absent, err := observeUpdateAbsence(filepath.Join(newRoot, "system")); !absent {
			return updateObserved("occupied new system slot", err, "stat")
		}
		if err = os.Rename(filepath.Join(context.Root, "system"), filepath.Join(newRoot, "system")); err != nil {
			return updateObserved("recovery rename", err, "rename")
		}
		if err = step(); err != nil {
			return err
		}
	}
	if verifyUpdateSystem(filepath.Join(stage, "parked-system"), j.OldPublic) == nil {
		if err = verifyUpdateRecoveryState(context, *j); err != nil {
			return err
		}
		if absent, err := observeUpdateAbsence(filepath.Join(context.Root, "system")); !absent {
			return updateObserved("occupied home system", err, "stat")
		}
		if err = os.Rename(filepath.Join(stage, "parked-system"), filepath.Join(context.Root, "system")); err != nil {
			return updateObserved("recovery rename", err, "rename")
		}
		if err = step(); err != nil {
			return err
		}
	}
	oldEntries, backupEntries, newEntries := updateRootEntries(j.OldPublic), updateRootEntries(*j.OldStaged), updateRootEntries(*j.NewStaged)
	for i, old := range oldEntries {
		if err = verifyUpdateRecoveryState(context, *j); err != nil {
			return err
		}
		next, backup := newEntries[i], backupEntries[i]
		homePath := filepath.Join(context.Root, filepath.FromSlash(old.Path))
		newPath := filepath.Join(newRoot, filepath.FromSlash(next.Path))
		if updateEntryMatches(context.Root, next) {
			if absent, err := observeUpdateAbsence(newPath); !absent {
				return updateObserved("occupied new slot", err, "stat")
			}
			if err = os.Rename(homePath, newPath); err != nil {
				return updateObserved("recovery rename", err, "rename")
			}
			if err = step(); err != nil {
				return err
			}
		}
		if updatePathAbsent(homePath) {
			if err = verifyUpdateRecoveryState(context, *j); err != nil {
				return err
			}
			if err := observeUpdateEntry(oldRoot, backup); err != nil {
				return err
			}
			if err = os.Rename(filepath.Join(oldRoot, filepath.FromSlash(backup.Path)), homePath); err != nil {
				return updateObserved("recovery rename", err, "rename")
			}
			if err = step(); err != nil {
				return err
			}
		}
	}
	for _, m := range updateMetadataMembers(context, j.Metadata) {
		if err = verifyUpdateRecoveryState(context, *j); err != nil {
			return err
		}
		raw, err := installationRead(context.Root, m.path, 1<<20)
		if err != nil {
			return err
		}
		if bytes.Equal(raw, m.after) && !bytes.Equal(raw, m.before) {
			if err = installationWrite(context.Root, m.path, m.before); err != nil {
				return updateObserved("recovery write", err, "write")
			}
			if err = step(); err != nil {
				return err
			}
		}
	}
	if err = verifyUpdateRecoveryState(context, *j); err != nil {
		return err
	}
	if err = verifyPublicInventory(context.Root); err != nil {
		return err
	}
	j.Phase = "rolled_back"
	return persistUpdateProofJournal(context, *j)
}
