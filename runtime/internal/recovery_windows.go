//go:build windows

package internal

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"os"
	"path/filepath"
	"strings"
)

func recoveryContext() (string, error) {
	key, e := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if e != nil {
		return "", e
	}
	defer key.Close()
	machine, _, e := key.GetStringValue("MachineGuid")
	if e != nil {
		return "", e
	}
	host, e := os.Hostname()
	if e != nil || strings.TrimSpace(machine) == "" || strings.TrimSpace(host) == "" {
		return "", errors.New("missing native context")
	}
	h := sha256.Sum256([]byte("windows\x00" + strings.TrimSpace(machine) + "\x00" + host))
	return hex.EncodeToString(h[:]), nil
}
func registerOperation(db *sql.DB, id string, pid, group int) error {
	r, e := db.Exec("UPDATE operation_owner SET phase='registered',child_pid=?,process_group=?,job_attached=1 WHERE operation_id=? AND phase='launching'", pid, group, id)
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
	var state string
	if db.QueryRow("SELECT state FROM operation WHERE id=?", p.OperationID).Scan(&state) != nil {
		return bad("STATE_CORRUPT", 0)
	}
	var phase, root, copy, device, ctx string
	var parent, child sql.NullInt64
	var job int
	if e = db.QueryRow("SELECT phase,root,copy_id,device_id,context_hash,parent_pid,child_pid,job_attached FROM operation_owner WHERE operation_id=?", p.OperationID).Scan(&phase, &root, &copy, &device, &ctx, &parent, &child, &job); e == sql.ErrNoRows {
		return good("INSPECTED", 0, map[string]any{"operationId": p.OperationID, "operationState": state, "nextAction": "inspect", "ownership": map[string]any{"phase": "unregistered", "observation": "unknown", "canReconcile": false, "reason": "unregistered"}})
	} else if e != nil {
		return bad("STATE_CORRUPT", 0)
	}
	if !recoverableState(state) {
		return recoveryInspection(p.OperationID, state, phase, "unknown", false, "terminal")
	}
	actual, _ := filepath.EvalSymlinks(r.ProjectRoot)
	obs := "unknown"
	can := false
	reason := "unqualified"
	var currentCopy, currentDevice string
	_ = db.QueryRow("SELECT copy_id,device_id FROM project").Scan(&currentCopy, &currentDevice)
	currentContext, contextErr := recoveryContext()
	if actual == root && copy == currentCopy && device == currentDevice && contextErr == nil && ctx != "" && ctx == currentContext && phase == "registered" && validWindowsPID(parent) && validWindowsPID(child) && job == 1 {
		plive, pe := windowsProcessLive(uint32(parent.Int64))
		clive, ce := windowsProcessLive(uint32(child.Int64))
		if pe == nil && plive {
			obs = "live"
			reason = "supervisor_live"
		} else if ce == nil && clive {
			obs = "live"
			reason = "registered_live"
		} else if windowsAbsenceProven(plive, pe, clive, ce) {
			obs = "absent"
			can = true
			reason = "registered_absent"
		} else {
			reason = "native_unknown"
		}
	}
	return recoveryInspection(p.OperationID, state, phase, obs, can, reason)
}

func windowsAbsenceProven(parentLive bool, parentErr error, childLive bool, childErr error) bool {
	return parentErr == nil && childErr == nil && !parentLive && !childLive
}

func validWindowsPID(pid sql.NullInt64) bool {
	return pid.Valid && pid.Int64 > 1 && pid.Int64 <= int64(^uint32(0))
}

func windowsProcessLive(pid uint32) (bool, error) {
	if pid == 0 {
		return false, errors.New("invalid recorded pid")
	}
	h, e := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if e != nil {
		// OpenProcess reports this specific error when the PID is no longer in
		// the process table. Access denial and every other native error remain
		// unknown, never an absence proof.
		if errors.Is(e, windows.ERROR_INVALID_PARAMETER) {
			return false, nil
		}
		return false, e
	}
	defer windows.CloseHandle(h)
	var code uint32
	if e = windows.GetExitCodeProcess(h, &code); e != nil {
		return false, e
	}
	return code == 259, nil
}
func recoveryAbsent(request Request, id string) bool {
	r := inspectOperation(Request{SchemaVersion: 1, Action: "operation.inspect", ProjectRoot: request.ProjectRoot, RootKind: request.RootKind, Actor: "x", RequestID: "x", Payload: json.RawMessage(`{"operationId":"` + id + `"}`), layout: request.layout})
	if !r.OK {
		return false
	}
	o, _ := r.Data["ownership"].(map[string]any)
	return o["canReconcile"] == true
}
