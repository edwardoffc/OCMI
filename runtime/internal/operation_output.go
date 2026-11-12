package internal

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"
)

// operationOutput reads only bounded registered evidence from an exact revision.
func operationOutput(r Request) Response {
	if len(r.Payload) > 4096 {
		return bad("INVALID_PAYLOAD", 0)
	}
	fields, ok := outputObject(r.Payload)
	if !ok || string(fields["limitBytes"]) == "null" {
		return bad("INVALID_PAYLOAD", 0)
	}
	for key := range fields {
		if key != "operationId" && key != "limitBytes" {
			return bad("INVALID_PAYLOAD", 0)
		}
	}
	var p struct {
		OperationID string `json:"operationId"`
		LimitBytes  *int   `json:"limitBytes"`
	}
	dec := json.NewDecoder(bytes.NewReader(r.Payload))
	dec.DisallowUnknownFields()
	if dec.Decode(&p) != nil || p.OperationID == "" || len(p.OperationID) > 512 {
		return bad("INVALID_PAYLOAD", 0)
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return bad("INVALID_PAYLOAD", 0)
	}
	limit := 4096
	if p.LimitBytes != nil {
		limit = *p.LimitBytes
	}
	if limit < 1 || limit > 65536 {
		return bad("INVALID_PAYLOAD", 0)
	}
	db, e := openExistingFor(r)
	if e != nil {
		return bad("STATE_UNAVAILABLE", 0)
	}
	defer db.Close()
	if b := recoveryInspectSchema(db); b != nil {
		return *b
	}
	tx, e := db.Begin()
	if e != nil {
		return bad("STATE_BUSY", 0)
	}
	defer tx.Rollback()
	rev, e := revision(tx)
	if e != nil {
		return bad("STATE_CORRUPT", 0)
	}
	if r.ExpectedRevision > 0 && r.ExpectedRevision != rev {
		return bad("STALE_REVISION", rev)
	}
	t, e := loadTask(tx)
	if e != nil {
		return bad("OUTPUT_UNAVAILABLE", rev)
	}
	if !owner(tx, r.Actor) && r.Actor != t.Worker && r.Actor != t.Reviewer {
		return bad("NOT_ASSIGNED", rev)
	}
	var state, taskID, outcome, command, phase string
	var taskRev, commandSize, sourceBytes int64
	var exit sql.NullInt64
	var truncated int
	e = tx.QueryRow(`SELECT o.state,o.task_id,m.task_revision,m.outcome_id,length(CAST(tr.command_json AS BLOB)),o.exit_code,o.truncated,length(CAST(o.output AS BLOB)),ow.phase
 FROM operation o JOIN operation_revision m ON m.operation_id=o.id
 JOIN task_revision tr ON tr.task_id=m.task_id AND tr.task_revision=m.task_revision AND tr.outcome_id=m.outcome_id
 JOIN operation_owner ow ON ow.operation_id=o.id WHERE o.id=? AND m.task_id=o.task_id`, p.OperationID).Scan(&state, &taskID, &taskRev, &outcome, &commandSize, &exit, &truncated, &sourceBytes, &phase)
	if e != nil || taskID != t.ID || taskRev != t.Revision || outcome != t.OutcomeID || (state != "succeeded" && state != "failed" && state != "incomplete") || !exit.Valid || phase != "registered" || (truncated != 0 && truncated != 1) || commandSize < 2 || commandSize > 65536 || sourceBytes < 0 {
		return bad("OUTPUT_UNAVAILABLE", rev)
	}
	if tx.QueryRow("SELECT command_json FROM task_revision WHERE task_id=? AND task_revision=?", taskID, taskRev).Scan(&command) != nil {
		return bad("OUTPUT_UNAVAILABLE", rev)
	}
	var policy struct {
		OutputPolicy     string `json:"outputPolicy"`
		OutputLimitBytes int    `json:"outputLimitBytes"`
	}
	commandFields, ok := outputObject([]byte(command))
	if !ok {
		return bad("OUTPUT_WITHHELD", rev)
	}
	if json.Unmarshal(commandFields["outputPolicy"], &policy.OutputPolicy) != nil || json.Unmarshal(commandFields["outputLimitBytes"], &policy.OutputLimitBytes) != nil || policy.OutputPolicy != "shareable" || policy.OutputLimitBytes < 1 || sourceBytes > int64(policy.OutputLimitBytes) {
		return bad("OUTPUT_WITHHELD", rev)
	}
	var raw []byte
	if tx.QueryRow("SELECT substr(CAST(output AS BLOB),1,?) FROM operation WHERE id=?", limit+1, p.OperationID).Scan(&raw) != nil {
		return bad("OUTPUT_UNAVAILABLE", rev)
	}
	returned := sourceBytes > int64(limit)
	if len(raw) > limit {
		raw = raw[:limit]
	}
	valid := 0
	invalid := false
	for valid < len(raw) {
		if !utf8.FullRune(raw[valid:]) && returned {
			break
		}
		runeValue, n := utf8.DecodeRune(raw[valid:])
		if runeValue == utf8.RuneError && n == 1 {
			invalid = true
			break
		}
		valid += n
	}
	raw = raw[:valid]
	return good("OPERATION_OUTPUT", rev, map[string]any{"operationId": p.OperationID, "taskId": taskID, "taskRevision": taskRev, "outcomeId": outcome, "operationState": state, "exitCode": int(exit.Int64), "output": string(raw), "complete": truncated == 0 && !returned && !invalid, "capturedTruncated": truncated == 1, "returnedTruncated": returned || invalid, "invalidUTF8": invalid, "sourceBytes": sourceBytes})
}

// Reject duplicate object fields rather than selecting one ambiguous policy.
func outputObject(raw []byte) (map[string]json.RawMessage, bool) {
	d := json.NewDecoder(bytes.NewReader(raw))
	token, e := d.Token()
	if e != nil || token != json.Delim('{') {
		return nil, false
	}
	fields := map[string]json.RawMessage{}
	seen := map[string]bool{}
	for d.More() {
		k, e := d.Token()
		if e != nil {
			return nil, false
		}
		key, ok := k.(string)
		if !ok {
			return nil, false
		}
		folded := strings.ToLower(key)
		if seen[folded] {
			return nil, false
		}
		seen[folded] = true
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return nil, false
		}
		fields[key] = value
	}
	if _, e = d.Token(); e != nil {
		return nil, false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, false
	}
	return fields, true
}
