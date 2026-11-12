package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/google/uuid"
)

var updateTestPhase func(string)

func updatePhase(phase string) {
	if updateTestPhase != nil {
		updateTestPhase(phase)
	}
}
func validCommit(v string) bool      { return len(v) == 40 && strings.Trim(v, "0123456789abcdef") == "" }
func updateDigest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func updateFailure(reason, stage, next string) Response {
	return installationResponse("UPDATE_REFUSED", false, map[string]any{"reason": reason, "stage": stage, "nextAction": next})
}
func updatePendingResponse() Response {
	return installationResponse("UPDATE_PENDING", false, map[string]any{"reason": "unfinished_update", "stage": "recovery", "nextAction": "run update recover", "phase": "pending"})
}

func readUpdateProofJournal(context layoutContext) (updateProofJournal, error) {
	raw, err := installationRead(context.Root, context.Paths["bindings"]+"/update.json", 16<<20)
	if err != nil {
		return updateProofJournal{}, err
	}
	j, err := decodeUpdateProofJournal(raw)
	if err != nil {
		return updateProofJournal{}, err
	}
	id, err := localNativeID(context.Root)
	if err != nil || j.Metadata.Home != context.Root || j.Metadata.HomeID != id {
		return updateProofJournal{}, errors.New("journal attribution")
	}
	return j, nil
}

// Malformed or unattributable journal data must not bypass mutation barriers.
func updatePending(context layoutContext) bool {
	j, err := readUpdateProofJournal(context)
	if os.IsNotExist(err) {
		return false
	}
	return err != nil || j.Phase != "applied" && j.Phase != "rolled_back"
}

func updateCandidate(home string) (layoutContext, factoryInfo, error) {
	context, info, err := verifyFactoryHome(home)
	if err != nil {
		return context, info, err
	}
	raw, err := installationRead(context.Root, "system/build-info.json", 1<<20)
	if err != nil {
		return context, info, err
	}
	var v struct {
		UpdateCompatibility map[string]int `json:"updateCompatibility"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return context, info, errors.New("compatibility")
	}
	want := map[string]int{"schemaVersion": 1, "layoutVersion": 1, "installationReceiptVersion": 1, "coordinatorMemoryVersion": 1, "coordinatorBindingVersion": 1, "stateSchemaVersion": 4}
	if len(v.UpdateCompatibility) != len(want) {
		return context, info, errors.New("compatibility")
	}
	for k, n := range want {
		if v.UpdateCompatibility[k] != n {
			return context, info, errors.New("compatibility")
		}
	}
	return context, info, nil
}

func UpdateHome(action, home, candidate, expectedSource, expectedCandidate string) Response {
	if !filepath.IsAbs(home) {
		return updateFailure("home_path", "home", "provide an absolute home")
	}
	context, code := resolveLayout(home, "installation")
	if code != "" {
		return updateFailure("factory_payload", "home", "inspect installed home")
	}
	j, jerr := readUpdateProofJournal(context)
	if jerr != nil && !os.IsNotExist(jerr) {
		return updateFailure("journal", "recovery", "inspect update state")
	}
	if toolStoragePending(context) && (action == "apply" || action == "rollback") {
		return updateFailure("tool_storage_pending", "admission", "inspect retained tool storage transaction")
	}
	if action == "inspect" {
		if jerr == nil && j.Phase != "applied" && j.Phase != "rolled_back" {
			return updatePendingResponse()
		}
		_, info, err := verifyFactoryHome(home)
		if err != nil {
			return updateFailure("factory_payload", "home", "inspect installed home")
		}
		receipt, err := currentInstallation(context, info)
		if err != nil {
			return updateFailure("installation_context", "home", "inspect installed home")
		}
		if candidate != "" {
			if !filepath.IsAbs(candidate) {
				return updateFailure("candidate_path", "candidate", "provide an absolute candidate")
			}
			if _, _, err = updateCandidate(candidate); err != nil {
				return updateFailure("candidate_payload", "candidate", "inspect candidate factory")
			}
		}
		phase := "idle"
		if jerr == nil {
			phase = j.Phase
		} else if candidate != "" {
			phase = "ready"
		}
		data := map[string]any{"phase": phase, "sourceCommit": info.SourceCommit, "installationId": receipt.InstallationID}
		if jerr == nil && (j.RetirementState == "pending" || j.Phase == "rolled_back" && j.CleanupState != "done") {
			data["cleanup"] = "pending"
			data["nextAction"] = "run update recover to resume cleanup"
		}
		return installationResponse("UPDATE_INSPECTED", true, data)
	}
	if action == "recover" || action == "rollback" {
		if err := updateExternalCaller(home); err != nil {
			return updateFailure("caller", "recovery", "run an external updater")
		}
		if jerr != nil {
			return updateFailure("no_update", "recovery", "inspect update state")
		}
		if action == "rollback" {
			stage := filepath.Join(context.Root, filepath.FromSlash(j.Stage))
			oldSystem := filepath.Join(stage, "parked-system")
			if _, err := os.Lstat(oldSystem); os.IsNotExist(err) {
				oldSystem = filepath.Join(stage, "old", "system")
			}
			if err := toolStorageReaderGuardSystem(context, oldSystem); err != nil {
				return updateFailure("tool_storage_reader_capability", "recovery", "use a schema2-capable ReadyKit")
			}
		}
		if workspacePreviousReaderGuard(context, j, action == "recover") != nil {
			return workspaceReaderFailure("recovery")
		}
		return performUpdateRecovery(context, j, action == "recover")
	}
	if action != "apply" {
		return updateFailure("invalid_action", "cli", "review command options")
	}
	if jerr == nil {
		if j.Phase == "applied" && j.OldSource == expectedSource && j.NewSource == expectedCandidate {
			_, info, _, _, err := preflightUpdateCandidate(home, candidate, expectedCandidate, expectedCandidate)
			if err != nil {
				if errors.Is(err, errWorkspaceReaderCapability) {
					return workspaceReaderFailure("preflight")
				}
				return updateFailure("replay_context", "preflight", "inspect candidate and home")
			}
			if info.SourceCommit != j.NewSource || verifyUpdateRootPlacement(context, j) != nil || verifyUpdateMetadataPlacement(context, j) != nil || verifyUpdatePublicProof(candidate, j.NewPublic, false, false) != nil {
				return updateFailure("replay_context", "preflight", "inspect recorded generation")
			}
			receipt, err := readInstallationReceiptContext(context)
			if err != nil {
				return updateFailure("installation_context", "preflight", "inspect installed home")
			}
			writer, err := acquireUpdateRecoveryWriter(context)
			if err != nil {
				return updateFailure("state_busy", "preflight", "retry when state writer is idle")
			}
			defer writer.Release()
			if err = updateDBReady(writer, receipt); err != nil {
				return updateFailure("state_busy", "preflight", "inspect installation state")
			}
			current, err := readUpdateProofJournal(context)
			if err != nil || !reflect.DeepEqual(current, j) {
				return updateFailure("journal", "preflight", "inspect update state")
			}
			if _, _, _, _, err = preflightUpdateCandidate(home, candidate, expectedCandidate, expectedCandidate); err != nil || verifyUpdateRootPlacement(context, j) != nil || verifyUpdateMetadataPlacement(context, j) != nil || verifyUpdatePublicProof(candidate, j.NewPublic, false, false) != nil {
				if errors.Is(err, errWorkspaceReaderCapability) {
					return workspaceReaderFailure("admission")
				}
				return updateFailure("replay_context", "preflight", "inspect recorded generation")
			}
			data := map[string]any{"phase": "applied", "sourceCommit": j.NewSource, "installationId": receipt.InstallationID}
			if j.RetirementState == "pending" {
				data["cleanup"] = "pending"
				data["nextAction"] = "run update recover to resume retirement"
			}
			return installationResponse("UPDATED", true, data)
		}
		if j.Phase != "applied" && j.Phase != "rolled_back" {
			return updatePendingResponse()
		}
		if j.CleanupState == "pending" || j.RetirementState == "pending" {
			return installationResponse("UPDATE_REFUSED", false, map[string]any{"phase": j.Phase, "reason": "cleanup_pending", "stage": "retirement", "nextAction": "run update recover to resume cleanup"})
		}
		if j.Phase == "rolled_back" && j.CleanupState != "done" {
			return updateFailure("cleanup_pending", "preflight", "run update recover to finish cleanup")
		}
	}
	oldCtx, oldInfo, newCtx, newInfo, err := preflightUpdateCandidate(home, candidate, expectedSource, expectedCandidate)
	if err != nil {
		if errors.Is(err, errWorkspaceReaderCapability) {
			return workspaceReaderFailure("preflight")
		}
		if errors.Is(err, errToolStorageReaderCapability) {
			return updateFailure("tool_storage_reader_capability", "preflight", "use a schema2-capable ReadyKit")
		}
		return updateFailure("preflight", "preflight", "inspect candidate and home")
	}
	lock, err := setupLockOwned(filepath.Join(home, filepath.FromSlash(context.Paths["bindings"])))
	if err != nil {
		return updateFailure("admission", "admission", "retry when setup is idle")
	}
	defer lock.Release()
	writer, err := acquireUpdateRecoveryWriter(context)
	if err != nil {
		return updateFailure("state_busy", "admission", "retry when state writer is idle")
	}
	defer writer.Release()
	if toolStoragePending(context) {
		return updateFailure("tool_storage_pending", "admission", "inspect retained tool storage transaction")
	}
	current, currentErr := readUpdateProofJournal(context)
	if jerr == nil {
		if currentErr != nil || !reflect.DeepEqual(current, j) {
			return updateFailure("journal", "admission", "inspect update state")
		}
		if err = verifyRecoveryBeforeAdmission(context, j); err != nil {
			return updateFailure("provenance", "admission", "inspect recorded generation")
		}
		if j.Phase == "applied" {
			if err = verifyPreviousStage(context, j); err != nil {
				return updateFailure("provenance", "admission", "inspect retained stage")
			}
		} else if j.Previous != nil {
			if err = verifyPreviousStage(context, *j.Previous); err != nil {
				return updateFailure("provenance", "admission", "inspect retained stage")
			}
		}
	} else if !os.IsNotExist(currentErr) {
		return updateFailure("journal", "admission", "inspect update state")
	}
	oldCtx, oldInfo, newCtx, newInfo, err = preflightUpdateCandidate(home, candidate, expectedSource, expectedCandidate)
	if err != nil {
		if errors.Is(err, errWorkspaceReaderCapability) {
			return workspaceReaderFailure("admission")
		}
		if errors.Is(err, errToolStorageReaderCapability) {
			return updateFailure("tool_storage_reader_capability", "admission", "use a schema2-capable ReadyKit")
		}
		return updateFailure("preflight", "admission", "inspect candidate and home")
	}
	receipt, err := currentInstallation(oldCtx, oldInfo)
	if err != nil || updateDBReady(writer, receipt) != nil {
		return updateFailure("state_busy", "admission", "inspect installation state")
	}
	metadata, err := captureUpdateMetadata(oldCtx, oldInfo, newInfo.SourceCommit, newCtx.Root)
	if err != nil {
		return updateFailure("metadata", "preflight", "inspect coordinator context")
	}
	oldProof, err := captureUpdatePublicProof(home, true)
	if err != nil {
		return updateFailure("public", "preflight", "inspect home payload")
	}
	newProof, err := captureUpdatePublicProof(candidate, false)
	if err != nil {
		return updateFailure("public", "preflight", "inspect candidate payload")
	}
	id := uuid.NewString()
	var previous *updateProofJournal
	var failure, retired *updateGenerationFact
	if jerr == nil {
		failure = j.LastFailure
		retired = j.LastRetired
		if j.Phase == "applied" {
			prior := j
			previous = &prior
		} else {
			previous = j.Previous
			failure = &updateGenerationFact{ID: j.ID, OldSource: j.OldSource, NewSource: j.NewSource, Phase: "rolled_back", RecoveryFrom: j.RecoveryFrom}
		}
	}
	j = updateProofJournal{SchemaVersion: 1, ID: id, Phase: "intent", Metadata: metadata, OldSource: oldInfo.SourceCommit, NewSource: newInfo.SourceCommit, OldPublic: oldProof, NewPublic: newProof, Stage: "local/runs/update-" + id, OldSlot: "old", NewSlot: "new", AdmissionID: lock.nativeID, Previous: previous, LastFailure: failure, LastRetired: retired}
	if err = verifyUpdateProofInputs(j, home, candidate); err != nil || verifyUpdateMetadataBefore(context, metadata) != nil || updateDBReady(writer, receipt) != nil {
		return updateFailure("preflight", "admission", "inspect installation state")
	}
	installed := filepath.Join(home, "system", "bin", "ocmi")
	if runtime.GOOS == "windows" {
		installed += ".exe"
	}
	if err = probeInstalledImage(installed); err != nil {
		return updateFailure("image_busy", "admission", "stop the installed executable")
	}
	if err = persistUpdateProofJournal(context, j); err != nil {
		return updateFailure("journal", "intent", "inspect update state")
	}
	updatePhase("intent")
	if err = createUpdateStage(context, &j, home, candidate); err != nil {
		return updateFailure("stage", "staging", "run update recover")
	}
	if err = copyUpdateStage(context, &j, home, candidate); err != nil {
		return updateFailure("stage", "staging", "run update recover")
	}
	updatePhase("staged")
	if err = activateUpdateSystem(context, &j, writer, lock); err != nil {
		return updateFailure("activation", "system", "run update recover")
	}
	if err = activateUpdateRootsAndMetadata(context, &j, writer, lock); err != nil {
		return updateFailure("activation", "metadata", "run update recover")
	}
	if j.Previous != nil {
		if err = retirePreviousStage(context, &j, writer, lock); err != nil {
			return installationResponse("UPDATED", true, map[string]any{"phase": "applied", "sourceCommit": j.NewSource, "installationId": receipt.InstallationID, "cleanup": "pending", "reason": "retirement_pending", "nextAction": "run update recover to resume retirement"})
		}
	}
	return installationResponse("UPDATED", true, map[string]any{"phase": "applied", "sourceCommit": j.NewSource, "installationId": receipt.InstallationID})
}

func verifyRecoveryBeforeAdmission(context layoutContext, j updateProofJournal) error {
	if j.Phase == "rolled_back" && (j.CleanupState == "pending" || j.CleanupState == "done") {
		if err := verifyRestoredUpdateHome(context, j); err != nil {
			return err
		}
		return verifyUpdateCleanupInventory(context, j)
	}
	if j.Phase == "intent" || j.Phase == "rolled_back" && j.RecoveryFrom == "intent" {
		if err := verifyUpdatePublicProof(context.Root, j.OldPublic, true, true); err != nil {
			return err
		}
		if err := verifyUpdateMetadataBefore(context, j.Metadata); err != nil {
			return err
		}
		stage := filepath.Join(context.Root, filepath.FromSlash(j.Stage))
		if j.StageID == "" {
			if !updatePathAbsent(stage) {
				return errors.New("unattributed stage")
			}
			return nil
		}
		return verifyUpdateStage(context, j, true)
	}
	if err := verifyUpdateRecoveryStart(context, j); err != nil {
		return err
	}
	return verifyUpdateRecoveryState(context, j)
}

// A terminal no-op does not create or rewrite admission.  If a crash left
// one behind, only the recorded native identity may be released while the
// installation writer remains held.
func releaseTerminalUpdateAdmission(context layoutContext, j updateProofJournal, writer *heldUpdateWriter) error {
	path := filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"]), ".setup-lock")
	if _, err := os.Lstat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return releaseRecordedSetupLock(context, j.AdmissionID, writer)
}

func performUpdateRecovery(context layoutContext, j updateProofJournal, recoverOnly bool) Response {
	writer, err := acquireUpdateRecoveryWriter(context)
	if err != nil {
		return updateFailure("state_busy", "recovery", "wait for active update")
	}
	defer writer.Release()
	if !recoverOnly && toolStoragePending(context) {
		return updateFailure("tool_storage_pending", "admission", "inspect retained tool storage transaction")
	}
	current, err := readUpdateProofJournal(context)
	if err != nil || !reflect.DeepEqual(current, j) {
		return updateFailure("journal", "recovery", "inspect update state")
	}
	receipt, err := readInstallationReceiptContext(context)
	if err != nil || updateDBReady(writer, receipt) != nil {
		return updateFailure("state_busy", "recovery", "inspect installation state")
	}
	if workspacePreviousReaderGuard(context, j, recoverOnly) != nil {
		return workspaceReaderFailure("recovery")
	}
	if err = verifyRecoveryBeforeAdmission(context, j); err != nil {
		if j.Phase == "rolled_back" && j.CleanupState != "done" {
			return installationResponse("UPDATE_REFUSED", false, map[string]any{"phase": "rolled_back", "cleanup": "pending", "reason": "cleanup_pending", "stage": "cleanup", "nextAction": "inspect retained stage"})
		}
		return updateProvenanceFailure("before_admission", j, err)
	}
	if recoverOnly && j.Phase == "applied" && j.Previous == nil {
		if err = releaseTerminalUpdateAdmission(context, j, writer); err != nil {
			return updateFailure("admission", "recovery", "inspect recorded admission")
		}
		receipt, err := readInstallationReceiptContext(context)
		if err != nil {
			return updateFailure("metadata", "recovery", "inspect installation")
		}
		return installationResponse("RECOVERED", true, map[string]any{"phase": "applied", "sourceCommit": j.NewSource, "installationId": receipt.InstallationID})
	}
	if !recoverOnly && j.Phase == "applied" && j.RetirementState == "pending" {
		return installationResponse("UPDATE_REFUSED", false, map[string]any{"phase": "applied", "reason": "cleanup_pending", "stage": "retirement", "nextAction": "run update recover to resume retirement"})
	}
	if j.Phase == "rolled_back" && j.CleanupState == "done" {
		if err = releaseTerminalUpdateAdmission(context, j, writer); err != nil {
			return updateFailure("admission", "recovery", "inspect recorded admission")
		}
		return installationResponse("ROLLED_BACK", true, map[string]any{"phase": "rolled_back", "sourceCommit": j.OldSource, "installationId": receipt.InstallationID})
	}
	if !(recoverOnly && j.Phase == "applied") {
		if err = probeRecoveryImages(context, j); err != nil {
			return updateFailure("image_busy", "recovery", "stop the installed executable")
		}
	}
	lockPath := filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"]), ".setup-lock")
	if _, err = os.Lstat(lockPath); err == nil {
		if err = releaseRecordedSetupLock(context, j.AdmissionID, writer); err != nil {
			return updateFailure("admission", "recovery", "inspect recorded admission")
		}
	} else if !os.IsNotExist(err) {
		return updateFailure("admission", "recovery", "inspect recorded admission")
	}
	lock, err := setupLockOwned(filepath.Dir(lockPath))
	if err != nil {
		return updateFailure("admission", "recovery", "retry when setup is idle")
	}
	defer lock.Release()
	j.AdmissionID = lock.nativeID
	if err = persistUpdateProofJournal(context, j); err != nil {
		return updateFailure("journal", "recovery", "inspect update state")
	}
	if recoverOnly && j.Phase == "applied" && j.RetirementState == "pending" {
		if err = retirePreviousStage(context, &j, writer, lock); err != nil {
			return installationResponse("RECOVERED", true, map[string]any{"phase": "applied", "sourceCommit": j.NewSource, "installationId": receipt.InstallationID, "cleanup": "pending", "reason": "retirement_pending", "nextAction": "inspect retained stage"})
		}
		return installationResponse("RECOVERED", true, map[string]any{"phase": "applied", "sourceCommit": j.NewSource, "installationId": receipt.InstallationID})
	}
	if j.CleanupState != "pending" {
		updatePhase("recovery_admitted")
		if err = recoverUpdatePublic(context, &j, writer, lock); err != nil {
			return updateProvenanceFailure("recover_public", j, err)
		}
	}
	if err = cleanupUpdateStage(context, &j, writer, lock); err != nil {
		if saved, readErr := readUpdateProofJournal(context); readErr == nil && saved.Phase == "rolled_back" && saved.CleanupState == "" {
			saved.CleanupState = "pending"
			saved.CleanupEntries = nil
			_ = persistUpdateProofJournal(context, saved)
		}
		return installationResponse("UPDATE_REFUSED", false, map[string]any{"phase": "rolled_back", "cleanup": "pending", "reason": "cleanup_pending", "stage": "cleanup", "nextAction": "inspect retained stage"})
	}
	updatePhase("cleanup_done")
	receipt, err = readInstallationReceiptContext(context)
	if err != nil {
		return updateFailure("metadata", "recovery", "inspect installation")
	}
	return installationResponse("ROLLED_BACK", true, map[string]any{"phase": "rolled_back", "sourceCommit": j.OldSource, "installationId": receipt.InstallationID})
}
