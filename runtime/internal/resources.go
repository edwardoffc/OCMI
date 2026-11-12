package internal

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// resourceEffectFault is test-only fault injection. Requests cannot set it.
var resourceEffectFault func(string) error

type resourceInput struct {
	ResourceID, Provider, Kind, Mode, Locator, Purpose, Lifetime, Disposition, OwnerRef string
	DependsOn                                                                           []string
	Reason, Access, Until, TransferID, Path, ContentBase64                              string
	InventorySHA256                                                                     string
	Enabled                                                                             *bool
}

func resourceMutate(tx *sql.Tx, r Request, rev int64) Response {
	var p resourceInput
	if json.Unmarshal(r.Payload, &p) != nil || p.ResourceID == "" {
		return bad("INVALID_RESOURCE", rev)
	}
	if r.Action == "resource.intent" {
		if strings.HasPrefix(strings.ToLower(p.ResourceID), "ocmi-") {
			return bad("RESERVED_RESOURCE_ID", rev)
		}
		return resourceIntent(tx, r, rev, p)
	}
	if r.Action == "resource.write" || r.Action == "resource.use" || r.Action == "resource.release" {
		if !resourceTaskActor(tx, r.Actor) {
			return bad("NOT_RESOURCE_ACTOR", rev)
		}
	} else if !owner(tx, r.Actor) {
		return bad("NOT_OWNER", rev)
	}
	if stopped(tx) && (r.Action == "resource.apply" || r.Action == "resource.write" || r.Action == "resource.use") {
		return bad("STOPPED", rev)
	}
	return resourceExisting(tx, r, rev, p)
}

func resourceTaskActor(tx *sql.Tx, actor string) bool {
	t, err := loadTask(tx)
	return err == nil && (actor == t.Worker || owner(tx, actor))
}

func resourceIntent(tx *sql.Tx, r Request, rev int64, p resourceInput) Response {
	if !owner(tx, r.Actor) {
		return bad("NOT_OWNER", rev)
	}
	if stopped(tx) {
		return bad("STOPPED", rev)
	}
	if p.Provider == "" || p.Kind == "" || (p.Mode != "create" && p.Mode != "use") || p.Locator == "" || p.Purpose == "" || p.Lifetime == "" || (p.Disposition != "remove" && p.Disposition != "keep" && p.Disposition != "release") || p.OwnerRef == "" || len(p.DependsOn) > 32 {
		return bad("INVALID_RESOURCE_INTENT", rev)
	}
	if p.Mode == "create" && !safeOwnedLocator(p.Locator) {
		return bad("INVALID_RESOURCE_LOCATOR", rev)
	}
	if r.RootKind == "installation" && p.Mode == "create" && !homeResourceAllowed(p.Locator) {
		return bad("INVALID_RESOURCE_LOCATOR", rev)
	}
	if p.Mode == "use" && p.Provider == "local" && !strings.HasPrefix(p.Locator, "/") && !strings.Contains(p.Locator, ":\\") {
		return bad("INVALID_RESOURCE_LOCATOR", rev)
	}
	for _, dep := range p.DependsOn {
		if dep == "" || dep == p.ResourceID || !resourceAvailable(tx, dep) {
			return bad("RESOURCE_DEPENDENCY_UNAVAILABLE", rev)
		}
	}
	var project, copyID, device string
	if tx.QueryRow("SELECT id,copy_id,device_id FROM project").Scan(&project, &copyID, &device) != nil {
		return bad("STATE_CORRUPT", rev)
	}
	t, _ := loadTask(tx)
	depends, _ := json.Marshal(p.DependsOn)
	ownership := "project"
	if p.Mode == "use" {
		ownership = "external"
	}
	n, err := next(tx, rev)
	if err != nil {
		return bad("STATE_BUSY", rev)
	}
	_, err = tx.Exec("INSERT INTO resource_ledger(id,provider,kind,mode,locator,purpose,lifetime,intended_disposition,owner_ref,depends_json,ownership,observed_state,task_id,task_revision,project_id,copy_id,device_id,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", p.ResourceID, p.Provider, p.Kind, p.Mode, p.Locator, p.Purpose, p.Lifetime, p.Disposition, p.OwnerRef, string(depends), ownership, "intent", t.ID, t.Revision, project, copyID, device, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return bad("RESOURCE_EXISTS", rev)
	}
	if err = resourceEvent(tx, p.ResourceID, "intent", r, t, "intent", "declared", p.Purpose); err != nil {
		return bad("STATE_BUSY", rev)
	}
	return good("RESOURCE_INTENDED", n, map[string]any{"resourceId": p.ResourceID, "observedState": "intent", "nextAction": "resource.apply"})
}

func resourceExisting(tx *sql.Tx, r Request, rev int64, p resourceInput) Response {
	var provider, kind, mode, state, ownership, locator, native, operationID string
	var taskID string
	var taskRev int64
	if tx.QueryRow("SELECT provider,kind,mode,observed_state,ownership,locator,native_identity,operation_id,task_id,task_revision FROM resource_ledger WHERE id=?", p.ResourceID).Scan(&provider, &kind, &mode, &state, &ownership, &locator, &native, &operationID, &taskID, &taskRev) != nil {
		return bad("RESOURCE_NOT_FOUND", rev)
	}
	switch r.Action {
	case "resource.use":
		if p.Access != "read" && p.Access != "write" || p.Reason == "" || !resourceAvailable(tx, p.ResourceID) {
			return bad("RESOURCE_USE_REFUSED", rev)
		}
		if ownership != "project" && p.Access == "write" {
			return bad("RESOURCE_USE_REFUSED", rev)
		}
		if provider == "local" {
			var proof localResourceProof
			var observeErr error
			var qualified bool
			if trustedWorkspaceLocator(p.ResourceID, locator) && tx.QueryRow("SELECT "+automaticWorkspaceProjection+" FROM resource_ledger WHERE id=?", p.ResourceID).Scan(&qualified) == nil && qualified {
				proof, observeErr = captureAutomaticWorkspace(r.ProjectRoot, locator, native)
			} else {
				proof, observeErr = inspectLocalResource(r.ProjectRoot, locator, kind, ownership == "external")
			}
			if observeErr != nil || proof.NativeIdentity == "" || proof.NativeIdentity != native {
				return bad("RESOURCE_USE_REFUSED", rev)
			}
		}
		var activeUses, writers int
		if tx.QueryRow("SELECT count(*),COALESCE(sum(CASE WHEN access='write' THEN 1 ELSE 0 END),0) FROM resource_use WHERE resource_id=? AND active=1", p.ResourceID).Scan(&activeUses, &writers) != nil {
			return bad("STATE_BUSY", rev)
		}
		if (p.Access == "write" && activeUses != 0) || (p.Access == "read" && writers != 0) {
			return bad("RESOURCE_USE_CONFLICT", rev)
		}
		t, _ := loadTask(tx)
		n, err := next(tx, rev)
		if err != nil {
			return bad("STATE_BUSY", rev)
		}
		res, updateErr := tx.Exec("UPDATE resource_use SET active=1,access=?,reason=? WHERE resource_id=? AND task_id=? AND task_revision=? AND actor=? AND active=0", p.Access, p.Reason, p.ResourceID, t.ID, t.Revision, r.Actor)
		if updateErr != nil {
			return bad("STATE_BUSY", rev)
		}
		if rows, rowErr := res.RowsAffected(); rowErr != nil {
			return bad("STATE_BUSY", rev)
		} else if rows == 0 {
			if _, err = tx.Exec("INSERT INTO resource_use(resource_id,task_id,task_revision,actor,access,reason) VALUES(?,?,?,?,?,?)", p.ResourceID, t.ID, t.Revision, r.Actor, p.Access, p.Reason); err != nil {
				return bad("RESOURCE_USE_CONFLICT", rev)
			}
		} else if rows != 1 {
			return bad("STATE_BUSY", rev)
		}
		if err = resourceEvent(tx, p.ResourceID, "use", r, t, state, "verified", p.Reason); err != nil {
			return bad("STATE_BUSY", rev)
		}
		return good("RESOURCE_IN_USE", n, map[string]any{"resourceId": p.ResourceID})
	case "resource.release":
		if p.Reason == "" {
			return bad("INVALID_RESOURCE_RELEASE", rev)
		}
		t, _ := loadTask(tx)
		var projectOwner string
		if tx.QueryRow("SELECT owner FROM project").Scan(&projectOwner) != nil {
			return bad("STATE_BUSY", rev)
		}
		ownerFinishing := false
		if r.Actor == projectOwner && r.Actor != t.Worker {
			var ownActive int
			if tx.QueryRow("SELECT count(*) FROM resource_use WHERE resource_id=? AND task_id=? AND task_revision=? AND actor=? AND active=1", p.ResourceID, t.ID, t.Revision, r.Actor).Scan(&ownActive) != nil {
				return bad("STATE_BUSY", rev)
			}
			ownerFinishing = ownActive == 0
		}
		if ownerFinishing {
			var stoppedOperations int
			if tx.QueryRow("SELECT count(*) FROM operation o JOIN resource_ledger process ON process.id='ocmi-process-' || o.id WHERE o.task_id=? AND o.state NOT IN ('prepared','running','unknown') AND process.observed_state='stopped'", t.ID).Scan(&stoppedOperations) != nil || stoppedOperations == 0 {
				return bad("RESOURCE_USE_REFUSED", rev)
			}
		}
		n, err := next(tx, rev)
		if err != nil {
			return bad("STATE_BUSY", rev)
		}
		query := "UPDATE resource_use SET active=0 WHERE resource_id=? AND task_id=? AND task_revision=? AND actor=? AND active=1"
		args := []any{p.ResourceID, t.ID, t.Revision, r.Actor}
		if ownerFinishing {
			query = "UPDATE resource_use SET active=0 WHERE resource_id=? AND task_id=? AND task_revision=? AND active=1"
			args = []any{p.ResourceID, t.ID, t.Revision}
		}
		result, err := tx.Exec(query, args...)
		if err != nil {
			return bad("STATE_BUSY", rev)
		}
		changed, rowErr := result.RowsAffected()
		if rowErr != nil || changed != 1 {
			return bad("RESOURCE_USE_NOT_FOUND", rev)
		}
		if err = resourceEvent(tx, p.ResourceID, "release", r, t, state, "declared", p.Reason); err != nil {
			return bad("STATE_BUSY", rev)
		}
		return good("RESOURCE_RELEASED", n, map[string]any{"resourceId": p.ResourceID})
	case "resource.hold":
		var applying int
		_ = tx.QueryRow("SELECT applying FROM resource_ledger WHERE id=?", p.ResourceID).Scan(&applying)
		if p.Enabled == nil || p.Reason == "" || state == "remove_pending" || applying != 0 {
			return bad("INVALID_RESOURCE_HOLD", rev)
		}
		n, err := next(tx, rev)
		if err != nil {
			return bad("STATE_BUSY", rev)
		}
		if _, err = tx.Exec("UPDATE resource_ledger SET held=?,hold_reason=?,hold_until=? WHERE id=?", boolInt(*p.Enabled), p.Reason, p.Until, p.ResourceID); err != nil {
			return bad("STATE_BUSY", rev)
		}
		t, _ := loadTask(tx)
		if err = resourceEvent(tx, p.ResourceID, "hold", r, t, state, "declared", p.Reason); err != nil {
			return bad("STATE_BUSY", rev)
		}
		return good("RESOURCE_HOLD_UPDATED", n, map[string]any{"resourceId": p.ResourceID, "held": *p.Enabled})
	case "resource.cancel":
		var issued, applying int
		_ = tx.QueryRow("SELECT effect_issued,applying FROM resource_ledger WHERE id=?", p.ResourceID).Scan(&issued, &applying)
		if p.Reason == "" || state != "intent" || issued != 0 || applying != 0 {
			return bad("RESOURCE_CANCEL_REFUSED", rev)
		}
		n, err := next(tx, rev)
		if err != nil {
			return bad("STATE_BUSY", rev)
		}
		result, err := tx.Exec("UPDATE resource_ledger SET observed_state='removed',actual_disposition='cancelled' WHERE id=? AND effect_issued=0 AND applying=0", p.ResourceID)
		if err != nil {
			return bad("STATE_BUSY", rev)
		}
		changed, rowErr := result.RowsAffected()
		if rowErr != nil || changed != 1 {
			return bad("STATE_BUSY", rev)
		}
		t, _ := loadTask(tx)
		if err = resourceEvent(tx, p.ResourceID, "cancel", r, t, "removed", "no_effect", p.Reason); err != nil {
			return bad("STATE_BUSY", rev)
		}
		return good("RESOURCE_CANCELLED", n, map[string]any{"resourceId": p.ResourceID})
	case "resource.apply":
		if state != "intent" || provider != "local" || (mode != "create" && mode != "use") {
			return bad("RESOURCE_APPLY_REFUSED", rev)
		}
		if _, err := base64.StdEncoding.DecodeString(p.ContentBase64); err != nil || len(p.ContentBase64) > 2<<20 {
			return bad("INVALID_RESOURCE_CONTENT", rev)
		}
		n, err := next(tx, rev)
		if err != nil {
			return bad("STATE_BUSY", rev)
		}
		claimed, claimErr := tx.Exec("UPDATE resource_ledger SET applying=1,effect_issued=1 WHERE id=? AND observed_state='intent' AND applying=0 AND effect_issued=0", p.ResourceID)
		if claimErr != nil {
			return bad("STATE_BUSY", rev)
		}
		if rows, rowErr := claimed.RowsAffected(); rowErr != nil || rows != 1 {
			return bad("RESOURCE_APPLY_REFUSED", rev)
		}
		t, _ := loadTask(tx)
		if err = resourceEvent(tx, p.ResourceID, "apply_intent", r, t, "intent", "issued", p.ContentBase64); err != nil {
			return bad("STATE_BUSY", rev)
		}
		return good("RESOURCE_APPLYING", n, map[string]any{"resourceId": p.ResourceID})
	case "resource.write":
		if state != "active" || provider != "local" || p.Path == "" {
			return bad("RESOURCE_WRITE_REFUSED", rev)
		}
		if _, err := base64.StdEncoding.DecodeString(p.ContentBase64); err != nil || len(p.ContentBase64) > 2<<20 {
			return bad("INVALID_RESOURCE_CONTENT", rev)
		}
		n, err := next(tx, rev)
		if err != nil {
			return bad("STATE_BUSY", rev)
		}
		claimed, claimErr := tx.Exec("UPDATE resource_ledger SET applying=1,effect_issued=1 WHERE id=? AND observed_state='active' AND applying=0", p.ResourceID)
		if claimErr != nil {
			return bad("STATE_BUSY", rev)
		}
		if rows, rowErr := claimed.RowsAffected(); rowErr != nil || rows != 1 {
			return bad("RESOURCE_WRITE_REFUSED", rev)
		}
		t, _ := loadTask(tx)
		if err = resourceEvent(tx, p.ResourceID, "write_intent", r, t, state, "issued", p.Path+":"+p.ContentBase64); err != nil {
			return bad("STATE_BUSY", rev)
		}
		return good("RESOURCE_WRITING", n, map[string]any{"resourceId": p.ResourceID})
	case "resource.dispose":
		if p.Disposition != "keep" && p.Disposition != "remove" && p.Disposition != "transfer" || p.Reason == "" {
			return bad("INVALID_RESOURCE_DISPOSITION", rev)
		}
		automaticWorkspace := operationID != "" && mode == "create" && trustedWorkspaceLocator(p.ResourceID, locator)
		if p.Disposition == "remove" && (ownership != "project" || (state != "active" && !(state == "keep" && automaticWorkspace))) {
			return bad("RESOURCE_REMOVE_REFUSED", rev)
		}
		if p.Disposition == "keep" && (state != "active" || p.Until == "") {
			return bad("RESOURCE_KEEP_REFUSED", rev)
		}
		if p.Disposition == "transfer" && (state != "active" && state != "keep" || p.Until == "") {
			return bad("RESOURCE_TRANSFER_REFUSED", rev)
		}
		if (p.Disposition == "keep" || p.Disposition == "transfer") && provider == "local" {
			var proof localResourceProof
			var observeErr error
			var qualified bool
			if automaticWorkspace && tx.QueryRow("SELECT "+automaticWorkspaceProjection+" FROM resource_ledger WHERE id=?", p.ResourceID).Scan(&qualified) == nil && qualified {
				proof, observeErr = captureAutomaticWorkspace(r.ProjectRoot, locator, native)
			} else {
				proof, observeErr = inspectLocalResource(r.ProjectRoot, locator, kind, ownership == "external")
			}
			if observeErr != nil || proof.NativeIdentity == "" || proof.NativeIdentity != native {
				return bad("RESOURCE_PENDING", rev)
			}
		}
		var uses, held int
		_ = tx.QueryRow("SELECT count(*) FROM resource_use WHERE resource_id=? AND active=1", p.ResourceID).Scan(&uses)
		_ = tx.QueryRow("SELECT held FROM resource_ledger WHERE id=?", p.ResourceID).Scan(&held)
		if p.Disposition == "remove" && (uses != 0 || held != 0) {
			return bad("RESOURCE_PENDING", rev)
		}
		if p.Disposition == "remove" && automaticWorkspace {
			var recordedRoot string
			err := tx.QueryRow("SELECT detail_digest FROM resource_event WHERE resource_id=? AND event='workspace_recovery_root' AND proof_class='native_root' ORDER BY id DESC LIMIT 1", p.ResourceID).Scan(&recordedRoot)
			if err != nil && err != sql.ErrNoRows {
				return bad("RESOURCE_PENDING", rev)
			}
			if err == nil {
				layout, code := selectedLayout(r)
				if code != "" || environmentPlainAncestors(layout.Root) != nil {
					return bad("RESOURCE_PENDING", rev)
				}
				current, e := localNativeID(layout.Root)
				sum := sha256.Sum256([]byte(current))
				if e != nil || current == "" || hex.EncodeToString(sum[:]) != recordedRoot {
					return bad("RESOURCE_PENDING", rev)
				}
			}
		}
		n, err := next(tx, rev)
		if err != nil {
			return bad("STATE_BUSY", rev)
		}
		if p.Disposition == "keep" {
			if _, err = tx.Exec("UPDATE resource_ledger SET observed_state='keep',actual_disposition='keep',hold_until=? WHERE id=?", p.Until, p.ResourceID); err != nil {
				return bad("STATE_BUSY", rev)
			}
			t, _ := loadTask(tx)
			if err = resourceEvent(tx, p.ResourceID, "keep", r, t, "keep", "verified", p.Reason); err != nil {
				return bad("STATE_BUSY", rev)
			}
			return good("RESOURCE_KEPT", n, map[string]any{"resourceId": p.ResourceID})
		}
		if p.Disposition == "transfer" {
			if p.TransferID == "" {
				return bad("INVALID_RESOURCE_DISPOSITION", rev)
			}
			if _, err = tx.Exec("UPDATE resource_ledger SET observed_state='transfer_pending',transfer_id=?,held=1,hold_reason=?,hold_until=? WHERE id=?", p.TransferID, p.Reason, p.Until, p.ResourceID); err != nil {
				return bad("STATE_BUSY", rev)
			}
			t, _ := loadTask(tx)
			if err = resourceEvent(tx, p.ResourceID, "transfer", r, t, "transfer_pending", "declared", p.Reason); err != nil {
				return bad("STATE_BUSY", rev)
			}
			return good("RESOURCE_TRANSFER_PENDING", n, map[string]any{"resourceId": p.ResourceID, "held": true})
		}
		result, removeErr := tx.Exec("UPDATE resource_ledger SET observed_state='remove_pending',applying=1,effect_issued=1 WHERE id=? AND (observed_state='active' OR (observed_state='keep' AND ?))", p.ResourceID, automaticWorkspace)
		if removeErr != nil {
			return bad("STATE_BUSY", rev)
		}
		if rows, rowErr := result.RowsAffected(); rowErr != nil || rows != 1 {
			return bad("RESOURCE_REMOVE_REFUSED", rev)
		}
		t, _ := loadTask(tx)
		if err = resourceEvent(tx, p.ResourceID, "remove_intent", r, t, "remove_pending", "issued", p.Reason); err != nil {
			return bad("STATE_BUSY", rev)
		}
		return good("RESOURCE_REMOVING", n, map[string]any{"resourceId": p.ResourceID})
	case "resource.reconcile":
		if p.Reason == "" {
			return bad("INVALID_RESOURCE_RECONCILE", rev)
		}
		if !trustedWorkspaceLocator(p.ResourceID, locator) {
			return bad("RESOURCE_PROVIDER_PENDING", rev)
		}
		snapshot, ok := workspaceRecoverySnapshot(tx, r, p.ResourceID)
		if !ok || len(p.InventorySHA256) != 64 || p.InventorySHA256 != snapshot.digest {
			return bad("RESOURCE_PENDING", rev)
		}
		again, ok := workspaceRecoverySnapshot(tx, r, p.ResourceID)
		if !ok || again.digest != snapshot.digest || again.inventory != snapshot.inventory {
			return bad("RESOURCE_PENDING", rev)
		}
		n, err := next(tx, rev)
		if err != nil {
			return bad("STATE_BUSY", rev)
		}
		res, err := tx.Exec("UPDATE resource_ledger SET inventory=?, observed_state='keep',actual_disposition='keep',hold_until='owner disposition required' WHERE id=? AND ((observed_state='active' AND effect_issued=1) OR (observed_state='intent' AND effect_issued=0)) AND applying=0 AND held=0", snapshot.inventory, p.ResourceID)
		if err != nil {
			return bad("STATE_BUSY", rev)
		}
		count, err := res.RowsAffected()
		if err != nil || count != 1 {
			return bad("RESOURCE_PENDING", rev)
		}
		t, _ := loadTask(tx)
		if resourceEvent(tx, p.ResourceID, "workspace_reconciled", r, t, "keep", "verified_inventory", p.Reason) != nil {
			return bad("STATE_BUSY", rev)
		}
		if resourceEvent(tx, p.ResourceID, "workspace_recovery_root", r, t, "keep", "native_root", snapshot.rootID) != nil {
			return bad("STATE_BUSY", rev)
		}
		return good("RESOURCE_RECONCILED", n, map[string]any{"resourceId": p.ResourceID, "inventorySHA256": snapshot.digest, "nextAction": "resource.dispose"})
	case "resource.rebind":
		if p.Locator == "" || p.Reason == "" {
			return bad("INVALID_RESOURCE_REBIND", rev)
		}
		n, err := next(tx, rev)
		if err != nil {
			return bad("STATE_BUSY", rev)
		}
		if p.OwnerRef == "" {
			return bad("INVALID_RESOURCE_REBIND", rev)
		}
		proof, observeErr := inspectLocalResource(r.ProjectRoot, p.Locator, kind, true)
		if observeErr != nil {
			return bad("RESOURCE_REBIND_REFUSED", rev)
		}
		if _, err = tx.Exec("UPDATE resource_ledger SET locator=?,owner_ref=?,mode='use',ownership='external',native_identity=?,inventory='',observed_state='active',imported=1 WHERE id=?", p.Locator, p.OwnerRef, proof.NativeIdentity, p.ResourceID); err != nil {
			return bad("STATE_BUSY", rev)
		}
		t, _ := loadTask(tx)
		if err = resourceEvent(tx, p.ResourceID, "rebind", r, t, "active", "external_native", p.Reason); err != nil {
			return bad("STATE_BUSY", rev)
		}
		return good("RESOURCE_REBOUND", n, map[string]any{"resourceId": p.ResourceID, "nextAction": "resource.use"})
	}
	return bad("UNKNOWN_ACTION", rev)
}

func resourceAvailable(tx *sql.Tx, id string) bool {
	var state string
	return tx.QueryRow("SELECT observed_state FROM resource_ledger WHERE id=?", id).Scan(&state) == nil && (state == "active" || state == "keep")
}

func resourceEvent(tx *sql.Tx, id, event string, r Request, t task, observed, proof, detail string) error {
	h := sha256.Sum256([]byte(detail))
	_, err := tx.Exec("INSERT INTO resource_event(resource_id,event,actor,request_id,task_id,task_revision,operation_id,observed_state,proof_class,detail_digest,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)", id, event, r.Actor, r.RequestID, t.ID, t.Revision, "", observed, proof, hex.EncodeToString(h[:]), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func safeOwnedLocator(locator string) bool {
	if locator == "" || strings.HasPrefix(locator, "/") || strings.Contains(locator, "\\") || strings.Contains(locator, "..") {
		return false
	}
	for _, part := range strings.Split(locator, "/") {
		part = strings.ToLower(part)
		if part == "" || part == "." || part == ".git" || part == ".ocmi" {
			return false
		}
	}
	return true
}

func resourceRead(r Request) Response {
	db, err := openExistingFor(r)
	if err != nil {
		return bad("STATE_UNAVAILABLE", 0)
	}
	defer db.Close()
	version, err := databaseVersion(db)
	if err != nil {
		return bad("STATE_CORRUPT", 0)
	}
	if version != stateSchemaVersion {
		return bad("UPGRADE_REQUIRED", 0)
	}
	var rev int64
	if db.QueryRow("SELECT revision FROM project").Scan(&rev) != nil {
		return bad("STATE_CORRUPT", 0)
	}
	if r.Action == "resource.inspect" {
		var p struct {
			ResourceID string `json:"resourceId"`
		}
		if json.Unmarshal(r.Payload, &p) != nil || p.ResourceID == "" {
			return bad("INVALID_RESOURCE", rev)
		}
		var provider, kind, ownership, state, disposition, native, locator, purpose, ownerRef, lifetime, holdReason, holdUntil, taskID, operationID string
		var taskRevision int64
		var held, applying int
		var historical, automatic bool
		var active int
		err = db.QueryRow("SELECT provider,kind,ownership,observed_state,actual_disposition,native_identity,locator,held,purpose,owner_ref,lifetime,hold_reason,hold_until,task_id,task_revision,operation_id,applying,"+borrowedHistoryProjection+","+automaticWorkspaceProjection+" FROM resource_ledger WHERE id=?", p.ResourceID).Scan(&provider, &kind, &ownership, &state, &disposition, &native, &locator, &held, &purpose, &ownerRef, &lifetime, &holdReason, &holdUntil, &taskID, &taskRevision, &operationID, &applying, &historical, &automatic)
		if err != nil {
			return bad("RESOURCE_NOT_FOUND", rev)
		}
		_ = db.QueryRow("SELECT count(*) FROM resource_use WHERE resource_id=? AND active=1", p.ResourceID).Scan(&active)
		logical := map[string]any{"purpose": purpose, "ownerRef": ownerRef, "lifetime": lifetime, "holdReason": holdReason, "holdUntil": holdUntil, "taskId": taskID, "taskRevision": taskRevision, "operationId": operationID}
		if state == "needs_rebind" {
			return good("RESOURCE", rev, map[string]any{"resourceId": p.ResourceID, "provider": provider, "kind": kind, "ownership": ownership, "lifecycleState": state, "observedState": "needs_rebind", "disposition": disposition, "held": held != 0, "activeUses": active, "pending": true, "nextAction": "resource.rebind", "logical": logical})
		}
		if state == "historical_stopped" {
			return good("RESOURCE", rev, map[string]any{"resourceId": p.ResourceID, "provider": provider, "kind": kind, "ownership": ownership, "lifecycleState": state, "observedState": "historical_stopped", "disposition": disposition, "held": held != 0, "activeUses": active, "pending": false, "nextAction": "inspect", "logical": logical})
		}
		observed, nextAction := observedResource(r.ProjectRoot, p.ResourceID, provider, kind, ownership, state, native, locator, automatic)
		historical = historicalBorrowed(historical, native)
		if historical {
			nextAction = "inspect"
		}
		pending := resourceObservationPending(provider, state, observed, applying, historical)
		data := map[string]any{"resourceId": p.ResourceID, "provider": provider, "kind": kind, "ownership": ownership, "lifecycleState": state, "observedState": observed, "nativeIdentity": native, "disposition": disposition, "held": held != 0, "activeUses": active, "historicalReleased": historical, "pending": pending, "nextAction": nextAction, "logical": logical}
		if trustedWorkspaceLocator(p.ResourceID, locator) {
			if snapshot, ok := workspaceRecoverySnapshot(db, r, p.ResourceID); ok {
				data["recovery"] = snapshot.projection
			}
		}
		return good("RESOURCE", rev, data)
	}
	var p struct {
		Limit  int    `json:"limit"`
		Cursor int    `json:"cursor"`
		TaskID string `json:"taskId"`
	}
	if json.Unmarshal(r.Payload, &p) != nil || p.Limit < 0 || p.Cursor < 0 {
		return bad("INVALID_RESOURCE_LIST", rev)
	}
	if p.Limit == 0 || p.Limit > 100 {
		p.Limit = 100
	}
	rows, err := db.Query("SELECT id,provider,kind,ownership,observed_state,native_identity,locator,operation_id,purpose,owner_ref,lifetime,held,hold_reason,hold_until,task_id,task_revision,actual_disposition,applying,"+borrowedHistoryProjection+","+automaticWorkspaceProjection+" FROM resource_ledger WHERE (?='' OR task_id=?) ORDER BY id LIMIT ? OFFSET ?", p.TaskID, p.TaskID, p.Limit+1, p.Cursor)
	if err != nil {
		return bad("STATE_CORRUPT", rev)
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, provider, kind, ownership, state, native, locator, operationID, purpose, ownerRef, lifetime, holdReason, holdUntil, taskID, disposition string
		var held, applying int
		var historical, automatic bool
		var taskRevision int64
		if rows.Scan(&id, &provider, &kind, &ownership, &state, &native, &locator, &operationID, &purpose, &ownerRef, &lifetime, &held, &holdReason, &holdUntil, &taskID, &taskRevision, &disposition, &applying, &historical, &automatic) != nil {
			return bad("STATE_CORRUPT", rev)
		}
		if len(items) < p.Limit {
			observed, _ := observedResource(r.ProjectRoot, id, provider, kind, ownership, state, native, locator, automatic)
			historical = historicalBorrowed(historical, native)
			items = append(items, map[string]any{"resourceId": id, "provider": provider, "kind": kind, "ownership": ownership, "observedState": observed, "disposition": disposition, "operationId": operationID, "historicalReleased": historical, "pending": resourceObservationPending(provider, state, observed, applying, historical), "dispositionPending": resourceDispositionPending(provider, ownership, state, observed, applying, historical), "automatic": strings.HasPrefix(id, "ocmi-"), "logical": map[string]any{"purpose": purpose, "ownerRef": ownerRef, "lifetime": lifetime, "held": held != 0, "holdReason": holdReason, "holdUntil": holdUntil, "taskId": taskID, "taskRevision": taskRevision}})
		}
	}
	if rows.Err() != nil {
		return bad("STATE_CORRUPT", rev)
	}
	rows.Close()
	next := 0
	if len(items) == p.Limit {
		var count int
		_ = db.QueryRow("SELECT count(*) FROM resource_ledger WHERE (?='' OR task_id=?)", p.TaskID, p.TaskID).Scan(&count)
		if p.Cursor+p.Limit < count {
			next = p.Cursor + p.Limit
		}
	}
	total, pending, activeUses, summaryErr := resourceSummary(db, r.ProjectRoot, p.TaskID)
	if summaryErr != nil {
		return bad("STATE_CORRUPT", rev)
	}
	return good("RESOURCES", rev, map[string]any{"resources": items, "summary": map[string]any{"total": total, "pending": pending + activeUses, "activeUses": activeUses}, "nextCursor": next, "truncated": next != 0})
}

// resourceSummary treats stored lifecycle state as a claim, not proof.  It is
// intentionally conservative: any required local fact that is absent, changed
// or unobservable remains visible as pending to status/list/close callers.
func resourceSummary(db *sql.DB, root, taskID string) (total, pending, activeUses int, err error) {
	rows, err := db.Query("SELECT id,provider,kind,ownership,observed_state,native_identity,locator,applying,held,"+borrowedHistoryProjection+","+automaticWorkspaceProjection+" FROM resource_ledger WHERE (?='' OR task_id=?)", taskID, taskID)
	if err != nil {
		return 0, 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, provider, kind, ownership, state, native, locator string
		var applying, held int
		var historical, automatic bool
		if err = rows.Scan(&id, &provider, &kind, &ownership, &state, &native, &locator, &applying, &held, &historical, &automatic); err != nil {
			return 0, 0, 0, err
		}
		total++
		observed := state
		if provider == "local" && state != "removed" && state != "historical_stopped" && state != "needs_rebind" {
			observed, _ = observedResource(root, id, provider, kind, ownership, state, native, locator, automatic)
		}
		unresolved := resourceDispositionPending(provider, ownership, state, observed, applying, historicalBorrowed(historical, native))
		if unresolved {
			pending++
		}
	}
	if err = rows.Err(); err != nil {
		return 0, 0, 0, err
	}
	rows.Close()
	err = db.QueryRow("SELECT count(*) FROM resource_use WHERE active=1 AND (?='' OR task_id=?)", taskID, taskID).Scan(&activeUses)
	return
}

// observedResource is deliberately read-only: stored lifecycle state is not proof that a
// local object still has the identity the ledger created or bound.
func observedResource(root, id, provider, kind, ownership, state, native, locator string, automatic bool) (string, string) {
	if provider != "local" {
		return state, "resource.reconcile"
	}
	internalRunGroup := trustedWorkspaceLocator(id, locator)
	var proof localResourceProof
	var err error
	if internalRunGroup {
		if automatic && kind == "directory" && ownership == "project" {
			proof, err = captureAutomaticWorkspace(root, locator, native)
		} else {
			proof, err = captureLocalResource(root, locator, kind, native, true)
		}
	} else {
		proof, err = inspectLocalResource(root, locator, kind, ownership == "external")
	}
	if err == nil {
		if native != "" && proof.NativeIdentity != "" && native != proof.NativeIdentity {
			return "unknown", "resource.reconcile"
		}
		observed := proof.ObservedState
		if observed == "" {
			observed = "present"
		}
		if state == "active" || state == "keep" {
			return observed, "resource.use"
		}
		return observed, "resource.reconcile"
	}
	if os.IsNotExist(err) {
		return "absent", "resource.reconcile"
	}
	return "unknown", "resource.reconcile"
}

func executeResourceEffect(db *sql.DB, r Request, prepared Response) Response {
	id, _ := prepared.Data["resourceId"].(string)
	var p resourceInput
	if json.Unmarshal(r.Payload, &p) != nil || id == "" || p.ResourceID != id {
		return bad("STATE_CORRUPT", prepared.Revision)
	}
	var locator, kind, native, inventory, state, mode string
	var automatic bool
	if db.QueryRow("SELECT locator,kind,native_identity,inventory,observed_state,mode,"+automaticWorkspaceProjection+" FROM resource_ledger WHERE id=?", id).Scan(&locator, &kind, &native, &inventory, &state, &mode, &automatic) != nil {
		return bad("RESOURCE_NOT_FOUND", prepared.Revision)
	}
	if resourceEffectFault != nil {
		if err := resourceEffectFault("after-intent"); err != nil {
			return bad("RESOURCE_PENDING", prepared.Revision)
		}
	}
	var proof localResourceProof
	var effectErr error
	content, decodeErr := base64.StdEncoding.DecodeString(p.ContentBase64)
	if decodeErr != nil {
		return bad("INVALID_RESOURCE_CONTENT", prepared.Revision)
	}
	switch r.Action {
	case "resource.apply":
		if mode == "use" {
			proof, effectErr = inspectLocalResource(r.ProjectRoot, locator, kind, true)
		} else {
			proof, effectErr = createLocalResource(r.ProjectRoot, locator, kind, id, content, false)
		}
	case "resource.write":
		proof, effectErr = writeLocalResource(r.ProjectRoot, locator, kind, native, inventory, p.Path, content, false)
	case "resource.dispose":
		internalRunGroup := trustedWorkspaceLocator(id, locator)
		if automatic && internalRunGroup && kind == "directory" {
			effectErr = removeAutomaticWorkspace(r.ProjectRoot, locator, native, inventory)
		} else {
			effectErr = removeLocalResource(r.ProjectRoot, locator, kind, native, inventory, internalRunGroup)
		}
		if effectErr == nil {
			proof = localResourceProof{ObservedState: "removed"}
		}
	}
	if resourceEffectFault != nil && (effectErr == nil || proof.EffectStarted) {
		if err := resourceEffectFault("after-effect-before-record"); err != nil {
			return bad("RESOURCE_PENDING", prepared.Revision)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return bad("STATE_BUSY", prepared.Revision)
	}
	defer tx.Rollback()
	rev, err := revision(tx)
	if err != nil {
		return bad("STATE_CORRUPT", prepared.Revision)
	}
	var nextState, code, proofClass string
	if effectErr != nil {
		if proof.EffectStarted {
			nextState = "unknown"
			code = "RESOURCE_PENDING"
			proofClass = "ambiguous_effect"
		} else {
			nextState = "intent"
			code = "RESOURCE_APPLY_FAILED"
			proofClass = "no_effect"
		}
	} else if r.Action == "resource.dispose" {
		nextState = "removed"
		code = "RESOURCE_REMOVED"
		proofClass = "verified_absence"
	} else {
		nextState = "active"
		proofClass = "native_identity"
		if r.Action == "resource.write" {
			code = "RESOURCE_WRITTEN"
		} else if mode == "use" {
			code = "RESOURCE_BOUND"
			proofClass = "external_native"
		} else {
			code = "RESOURCE_APPLIED"
		}
	}
	clearIssued := effectErr != nil && !proof.EffectStarted
	result, err := tx.Exec("UPDATE resource_ledger SET observed_state=?,native_identity=CASE WHEN ?='' THEN native_identity ELSE ? END,inventory=CASE WHEN ?='' THEN inventory ELSE ? END,applying=0,effect_issued=CASE WHEN ? THEN 0 ELSE effect_issued END,actual_disposition=CASE WHEN ?='removed' THEN 'remove' ELSE actual_disposition END WHERE id=? AND applying=1 AND effect_issued=1", nextState, proof.NativeIdentity, proof.NativeIdentity, proof.Inventory, proof.Inventory, clearIssued, nextState, id)
	if err != nil {
		return bad("STATE_BUSY", rev)
	}
	if rows, rowErr := result.RowsAffected(); rowErr != nil || rows != 1 {
		return bad("RESOURCE_PENDING", rev)
	}
	t, _ := loadTask(tx)
	detail := ""
	if effectErr != nil {
		detail = effectErr.Error()
	}
	if err = resourceEvent(tx, id, "effect_result", r, t, nextState, proofClass, detail); err != nil {
		return bad("STATE_BUSY", rev)
	}
	n, err := next(tx, rev)
	if err != nil {
		return bad("STATE_BUSY", rev)
	}
	out := good(code, n, map[string]any{"resourceId": id, "observedState": nextState})
	if effectErr != nil {
		out = bad(code, n)
	}
	b, _ := json.Marshal(out)
	if _, err = tx.Exec("UPDATE request_log SET response_json=? WHERE id=?", string(b), r.RequestID); err != nil {
		return bad("STATE_BUSY", rev)
	}
	if err = tx.Commit(); err != nil {
		return bad("STATE_BUSY", rev)
	}
	return out
}

func trustedWorkspaceLocator(id, locator string) bool {
	if !strings.HasPrefix(id, "ocmi-workspace-") {
		return false
	}
	operationID := strings.TrimPrefix(id, "ocmi-workspace-")
	if !isUUID(operationID) {
		return false
	}
	locator = filepath.ToSlash(locator)
	return locator == ".ocmi/local/runs/"+operationID || locator == "local/runs/"+operationID
}
