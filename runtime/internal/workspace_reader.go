package internal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

var errWorkspaceReaderCapability = errors.New("workspace inventory reader capability")

// Any retained original automatic workspace requires the new reader, even if its
// last inventory was small. This is a metadata association check, not adoption
// or a filesystem inventory/cleanup capability.
func workspaceReaderRequired(c layoutContext) (bool, error) {
	db, err := openLayout(c)
	if err != nil {
		return false, errWorkspaceReaderCapability
	}
	defer db.Close()
	version, err := databaseVersion(db)
	if err != nil || version != stateSchemaVersion {
		return false, errWorkspaceReaderCapability
	}
	var projects, relevant, original int
	if db.QueryRow("SELECT count(*) FROM project").Scan(&projects) != nil || projects != 1 {
		return false, errWorkspaceReaderCapability
	}
	const relevantRows = `(l.id GLOB 'ocmi-workspace-*' OR (l.kind='directory' AND l.mode='create' AND l.ownership='project' AND l.owner_ref='ocmi-runtime' AND l.operation_id<>'')) AND (l.observed_state IS NULL OR l.observed_state<>'removed')`
	if db.QueryRow("SELECT count(*) FROM resource_ledger l WHERE "+relevantRows).Scan(&relevant) != nil {
		return false, errWorkspaceReaderCapability
	}
	root, err := filepath.EvalSymlinks(c.Root)
	if err != nil {
		return false, errWorkspaceReaderCapability
	}
	runs, err := filepath.Rel(c.Root, c.Runs)
	if err != nil {
		return false, errWorkspaceReaderCapability
	}
	// Counts must agree: missing/imported/cross-context/ambiguous associations are
	// uncertainty, never proof that no retained workspace exists.
	err = db.QueryRow(`SELECT count(*) FROM resource_ledger l
 JOIN project p ON p.id=l.project_id AND p.copy_id=l.copy_id AND p.device_id=l.device_id
 JOIN operation o ON o.id=l.operation_id AND o.task_id=l.task_id
 JOIN operation_revision m ON m.operation_id=o.id AND m.task_id=l.task_id AND m.task_revision=l.task_revision
 JOIN task_revision t ON t.task_id=m.task_id AND t.task_revision=m.task_revision AND t.outcome_id=m.outcome_id
 JOIN operation_owner ow ON ow.operation_id=o.id AND ow.copy_id=p.copy_id AND ow.device_id=p.device_id
 WHERE `+relevantRows+` AND l.id='ocmi-workspace-'||o.id
 AND l.provider='local' AND l.kind='directory' AND l.mode='create' AND l.ownership='project'
 AND l.owner_ref='ocmi-runtime' AND l.native_identity<>'' AND l.inventory<>''
 AND l.imported=0 AND l.applying=0 AND l.transfer_id=''
 AND l.observed_state IN ('active','keep') AND l.task_revision>0
 AND l.locator=?||'/'||o.id AND ow.root=? AND ow.phase='registered'
 AND ow.parent_pid>0 AND ow.child_pid>0 AND (ow.process_group>0 OR ow.job_attached=1)
 AND EXISTS(SELECT 1 FROM resource_event e WHERE e.resource_id=l.id AND e.event='automatic_intent'
 AND e.proof_class='pre_effect' AND e.task_id=l.task_id AND e.task_revision=l.task_revision
 AND e.request_id=ow.origin_request_id)`, filepath.ToSlash(runs), root).Scan(&original)
	if err != nil || original != relevant {
		return false, errWorkspaceReaderCapability
	}
	return relevant != 0, nil
}

func workspaceReaderGuardSystem(c layoutContext, system string) error {
	required, err := workspaceReaderRequired(c)
	if err != nil {
		return errWorkspaceReaderCapability
	}
	if !required {
		return nil
	}
	raw, err := installationRead(system, "build-info.json", 1<<20)
	if err != nil {
		return errWorkspaceReaderCapability
	}
	var fields map[string]json.RawMessage
	if !checkpointStrictJSON(raw, &fields) || string(fields["operationWorkspaceInventoryPolicy"]) != "1" {
		return errWorkspaceReaderCapability
	}
	return nil
}

func workspaceReaderGuard(c layoutContext, candidate string) error {
	return workspaceReaderGuardSystem(c, filepath.Join(candidate, "system"))
}

func workspaceReaderTransitionGuard(c layoutContext, candidate string) error {
	// The current generation becomes rollback/recovery's fallback. Do not start
	// an activation whose retained work would make that fallback unreadable.
	if err := workspaceReaderGuard(c, c.Root); err != nil {
		return err
	}
	return workspaceReaderGuard(c, candidate)
}

func workspacePreviousReaderGuard(c layoutContext, j updateProofJournal, recoverOnly bool) error {
	// Terminal applied recovery merely retires/releases; rolled_back cleanup has
	// already selected its reader. Other recovery paths can restore the old image.
	if j.Phase == "intent" || j.Phase == "rolled_back" || recoverOnly && j.Phase == "applied" {
		return nil
	}
	stage := filepath.Join(c.Root, filepath.FromSlash(j.Stage))
	system := filepath.Join(stage, "parked-system")
	if _, err := os.Lstat(system); os.IsNotExist(err) {
		system = filepath.Join(stage, "old", "system")
	}
	return workspaceReaderGuardSystem(c, system)
}

func workspaceReaderFailure(stage string) Response {
	return updateFailure("workspace_inventory_reader_capability", stage, "use an operation-workspace-policy-1 capable ReadyKit")
}
