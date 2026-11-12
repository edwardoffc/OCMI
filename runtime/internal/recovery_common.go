package internal

import (
	"database/sql"
	"encoding/json"
)

// The internal-only observation repeats the platform's native absence proof
// after qualified operation.reconcile has recorded incomplete.
func workspaceRecoveryAbsent(r Request, id string) bool {
	payload, _ := json.Marshal(map[string]string{"operationId": id})
	r.Action = "workspace.recovery.internal"
	r.Payload = payload
	result := inspectOperation(r)
	ownership, ok := result.Data["ownership"].(map[string]any)
	return result.OK && ok && ownership["observation"] == "absent" && ownership["canReconcile"] == true
}

func recoveryInspectSchema(db *sql.DB) *Response {
	version, err := databaseVersion(db)
	if err != nil {
		response := bad("STATE_CORRUPT", 0)
		return &response
	}
	if version != stateSchemaVersion {
		var revision int64
		if err := db.QueryRow("SELECT revision FROM project").Scan(&revision); err != nil {
			response := bad("STATE_CORRUPT", 0)
			return &response
		}
		response := Response{SchemaVersion: schemaVersion, Code: "UPGRADE_REQUIRED", Revision: revision, Data: map[string]any{"nextAction": "state.upgrade"}}
		return &response
	}
	return nil
}

func recoverableState(state string) bool {
	return state == "prepared" || state == "running" || state == "unknown"
}

func recoveryInspection(id, state, phase, observation string, can bool, reason string) Response {
	nextAction := "preserve_or_wait"
	if can {
		nextAction = "operation.reconcile"
	}
	if !recoverableState(state) {
		nextAction = "inspect"
	}
	return good("INSPECTED", 0, map[string]any{"operationId": id, "operationState": state, "nextAction": nextAction, "ownership": map[string]any{"phase": phase, "observation": observation, "canReconcile": can, "reason": reason}})
}
