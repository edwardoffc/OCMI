package internal

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
)

type workspaceRecoveryQuery interface {
	QueryRow(string, ...any) *sql.Row
	Query(string, ...any) (*sql.Rows, error)
}
type workspaceRecoveryResult struct {
	inventory, digest, rootID string
	entries                   []map[string]any
	projection                map[string]any
}

// A snapshot is authority only when re-established in the owner mutation.
func workspaceRecoverySnapshot(q workspaceRecoveryQuery, r Request, id string) (workspaceRecoveryResult, bool) {
	var locator, native, oldInventory, op, taskID, state, project, copyID, device string
	var taskRev int64
	err := q.QueryRow(`SELECT l.locator,l.native_identity,l.inventory,l.operation_id,l.task_id,l.task_revision,l.observed_state,l.project_id,l.copy_id,l.device_id
 FROM resource_ledger l JOIN operation o ON o.id=l.operation_id AND o.task_id=l.task_id
 JOIN operation_revision m ON m.operation_id=o.id AND m.task_id=l.task_id AND m.task_revision=l.task_revision
 JOIN operation_owner ow ON ow.operation_id=o.id
 JOIN project pr ON pr.id=l.project_id AND pr.copy_id=l.copy_id AND pr.device_id=l.device_id
 JOIN resource_ledger process ON process.id='ocmi-process-'||o.id AND process.operation_id=o.id
 WHERE l.id=? AND l.provider='local' AND l.kind='directory' AND l.mode='create' AND l.ownership='project'
 AND l.owner_ref='ocmi-runtime' AND l.imported=0 AND l.applying=0 AND l.held=0 AND l.transfer_id=''
 AND ((l.observed_state='active' AND l.effect_issued=1) OR (l.observed_state='intent' AND l.effect_issued=0))
 AND l.native_identity<>'' AND l.inventory<>''
 AND EXISTS(SELECT 1 FROM resource_event e WHERE e.resource_id=l.id AND e.event='automatic_intent'
 AND e.proof_class='pre_effect' AND e.task_id=l.task_id AND e.task_revision=l.task_revision AND e.request_id=ow.origin_request_id)
 AND o.state='incomplete' AND ow.phase='registered' AND ow.recovery_reason<>'' AND ow.copy_id=pr.copy_id AND ow.device_id=pr.device_id
 AND process.provider='supervised_process' AND process.observed_state='stopped' AND process.imported=0 AND process.task_id=l.task_id AND process.task_revision=l.task_revision`, id).Scan(&locator, &native, &oldInventory, &op, &taskID, &taskRev, &state, &project, &copyID, &device)
	if err != nil || id != "ocmi-workspace-"+op || !trustedWorkspaceLocator(id, locator) {
		return workspaceRecoveryResult{}, false
	}
	layout, code := selectedLayout(r)
	if code != "" {
		return workspaceRecoveryResult{}, false
	}
	expected, e := filepath.Rel(layout.Root, filepath.Join(layout.Runs, op))
	if e != nil || filepath.ToSlash(expected) != filepath.ToSlash(locator) {
		return workspaceRecoveryResult{}, false
	}
	rootID, e := localNativeID(layout.Root)
	if e != nil || rootID == "" || environmentPlainAncestors(layout.Root) != nil {
		return workspaceRecoveryResult{}, false
	}
	var count int
	if q.QueryRow("SELECT count(*) FROM resource_use WHERE active=1").Scan(&count) != nil || count != 0 {
		return workspaceRecoveryResult{}, false
	}
	if q.QueryRow("SELECT count(*) FROM operation WHERE state IN ('prepared','running','unknown')").Scan(&count) != nil || count != 0 {
		return workspaceRecoveryResult{}, false
	}
	if state == "intent" {
		if q.QueryRow("SELECT count(*) FROM resource_event WHERE resource_id=? AND event='effect_result' AND observed_state='intent' AND proof_class='no_effect'", id).Scan(&count) != nil || count < 1 {
			return workspaceRecoveryResult{}, false
		}
	}
	rows, e := q.Query("SELECT id,depends_json,observed_state FROM resource_ledger WHERE id<>?", id)
	if e != nil {
		return workspaceRecoveryResult{}, false
	}
	safe := true
	for rows.Next() {
		var other, deps, otherState string
		if rows.Scan(&other, &deps, &otherState) != nil {
			safe = false
			break
		}
		var list []string
		if json.Unmarshal([]byte(deps), &list) != nil {
			safe = false
			break
		}
		for _, dep := range list {
			if strings.EqualFold(dep, id) && otherState != "removed" {
				safe = false
			}
		}
	}
	if rows.Err() != nil {
		safe = false
	}
	rows.Close()
	if !safe {
		return workspaceRecoveryResult{}, false
	}
	if !workspaceRecoveryAbsent(r, op) {
		return workspaceRecoveryResult{}, false
	}
	proof, e := captureAutomaticWorkspace(r.ProjectRoot, locator, native)
	if e != nil || proof.NativeIdentity != native {
		return workspaceRecoveryResult{}, false
	}
	if !workspaceMarkerMatches(oldInventory, proof.Inventory, op) {
		return workspaceRecoveryResult{}, false
	}
	if !workspaceRecoveryAbsent(r, op) {
		return workspaceRecoveryResult{}, false
	}
	currentRoot, e := localNativeID(layout.Root)
	if e != nil || currentRoot != rootID {
		return workspaceRecoveryResult{}, false
	}
	// Bind the reviewed inventory to the root identity observed at inspection,
	// so moving the workspace into a replaced root invalidates that approval.
	sum := sha256.Sum256([]byte(rootID + "\x00" + id + "\x00" + proof.Inventory))
	digest := hex.EncodeToString(sum[:])
	projection, e := recoveryProjection(proof.Inventory, digest)
	if e != nil {
		return workspaceRecoveryResult{}, false
	}
	entries := projection["entries"].([]map[string]any)
	return workspaceRecoveryResult{inventory: proof.Inventory, digest: digest, rootID: rootID, entries: entries, projection: projection}, true
}

// Compare the original marker record, not merely the marker filename/content.
func workspaceMarkerMatches(original, current, op string) bool {
	if len(original) > automaticInventoryPolicy.encodedBytes || len(current) > automaticInventoryPolicy.encodedBytes {
		return false
	}
	markerName := base64.RawURLEncoding.EncodeToString([]byte(localMarker))
	find := func(inv string) string {
		found := ""
		for _, item := range strings.Split(inv, "|") {
			f := strings.Split(item, ",")
			if len(f) == 4 && f[0] == "f" && f[1] == markerName {
				if found != "" {
					return ""
				}
				found = item
			}
		}
		return found
	}
	marker := find(original)
	tokenHash := sha256.Sum256([]byte("ocmi-" + op))
	// The original automatic create records only the marker. A later failed
	// disposal historically reports "no_effect" even after partial removal;
	// only this original single-marker inventory plus an unchanged marker
	// proves that such a disposal did not remove any recorded member.
	return marker != "" && original == marker && marker == find(current) && strings.HasSuffix(marker, ","+hex.EncodeToString(tokenHash[:])+":41")
}
