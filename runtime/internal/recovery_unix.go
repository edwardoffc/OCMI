//go:build !windows

package internal

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"syscall"
)

func registerOperation(db *sql.DB, id string, pid, group int) error {
	r, e := db.Exec("UPDATE operation_owner SET phase='registered',child_pid=?,process_group=? WHERE operation_id=? AND phase='launching'", pid, group, id)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil || n != 1 {
		return sql.ErrNoRows
	}
	return nil
}
func inspectOperation(r Request) Response {
	var p struct{ OperationID string }
	if json.Unmarshal(r.Payload, &p) != nil || p.OperationID == "" {
		return bad("INVALID_INSPECT", 0)
	}
	db, e := openExistingFor(r)
	if e != nil {
		return bad("STATE_UNAVAILABLE", 0)
	}
	defer db.Close()
	if blocked := recoveryInspectSchema(db); blocked != nil {
		return *blocked
	}
	var phase, root, copy, device, ctx, state string
	var parent, child, group sql.NullInt64
	if e = db.QueryRow("SELECT state FROM operation WHERE id=?", p.OperationID).Scan(&state); e != nil {
		return bad("STATE_CORRUPT", 0)
	}
	if e = db.QueryRow("SELECT phase,root,copy_id,device_id,context_hash,parent_pid,child_pid,process_group FROM operation_owner WHERE operation_id=?", p.OperationID).Scan(&phase, &root, &copy, &device, &ctx, &parent, &child, &group); e == sql.ErrNoRows {
		return recoveryInspection(p.OperationID, state, "unregistered", "unknown", false, "unregistered")
	} else if e != nil {
		return bad("STATE_CORRUPT", 0)
	}
	if !recoverableState(state) {
		return recoveryInspection(p.OperationID, state, phase, "unknown", false, "terminal")
	}
	actual, e := filepath.EvalSymlinks(r.ProjectRoot)
	obs := "unknown"
	can := false
	reason := "unqualified"
	var currentCopy, currentDevice string
	if e == nil {
		e = db.QueryRow("SELECT copy_id,device_id FROM project").Scan(&currentCopy, &currentDevice)
	}
	currentContext, contextErr := recoveryContext()
	if e == nil && contextErr == nil && actual == root && copy == currentCopy && device == currentDevice && ctx == currentContext && phase == "registered" && validUnixOwnership(parent, child, group) {
		pe := syscall.Kill(int(parent.Int64), 0)
		ce := syscall.Kill(int(child.Int64), 0)
		ge := syscall.Kill(-int(group.Int64), 0)
		if pe == nil || ce == nil || ge == nil {
			obs = "live"
			reason = "registered_live"
		} else if pe == syscall.ESRCH && ce == syscall.ESRCH && ge == syscall.ESRCH {
			obs = "absent"
			can = true
			reason = "registered_absent"
		} else {
			reason = "native_unknown"
		}
	}
	return recoveryInspection(p.OperationID, state, phase, obs, can, reason)
}

func validUnixOwnership(parent, child, group sql.NullInt64) bool {
	return parent.Valid && child.Valid && group.Valid && parent.Int64 > 1 && child.Int64 > 1 && group.Int64 == child.Int64
}
func recoveryAbsent(request Request, id string) bool {
	r := inspectOperation(Request{SchemaVersion: 1, Action: "operation.inspect", ProjectRoot: request.ProjectRoot, RootKind: request.RootKind, Actor: "x", RequestID: "x", Payload: json.RawMessage(`{"operationId":"` + id + `"}`), layout: request.layout})
	if !r.OK {
		return false
	}
	o, _ := r.Data["ownership"].(map[string]any)
	return o["canReconcile"] == true
}
