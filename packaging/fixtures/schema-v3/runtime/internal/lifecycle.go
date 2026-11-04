package internal

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// upgradeBeforeTransaction is an internal test seam for proving the transaction recheck.
var upgradeBeforeTransaction func()

func upgradeState(db *sql.DB, r Request, version int) Response {
	if version == stateSchemaVersion {
		var fingerprint, stored string
		if db.QueryRow("SELECT fingerprint,response_json FROM request_log WHERE id=?", r.RequestID).Scan(&fingerprint, &stored) == nil && fingerprint == canonical(r) {
			var response Response
			if json.Unmarshal([]byte(stored), &response) == nil {
				return response
			}
		}
		return bad("ALREADY_UPGRADED", 0)
	}
	var owner string
	var rev int64
	if db.QueryRow("SELECT owner,revision FROM project").Scan(&owner, &rev) != nil || owner != r.Actor {
		return bad("NOT_OWNER", rev)
	}
	if r.ExpectedRevision != rev {
		return bad("STALE_REVISION", rev)
	}
	var empty map[string]any
	if json.Unmarshal(r.Payload, &empty) != nil || len(empty) != 0 {
		return bad("INVALID_UPGRADE", rev)
	}
	var prior, stored string
	if db.QueryRow("SELECT fingerprint,response_json FROM request_log WHERE id=?", r.RequestID).Scan(&prior, &stored) == nil {
		if prior != canonical(r) {
			return bad("IDEMPOTENCY_CONFLICT", rev)
		}
		var response Response
		if json.Unmarshal([]byte(stored), &response) == nil {
			return response
		}
		return bad("STATE_CORRUPT", rev)
	}
	backup := storePath(r.ProjectRoot) + ".v1-backup"
	if version == 2 {
		backup = storePath(r.ProjectRoot) + ".v2-backup"
	}
	if _, err := os.Stat(backup); err == nil {
		return bad("UPGRADE_UNAVAILABLE", rev)
	} else if !errors.Is(err, os.ErrNotExist) {
		return bad("UPGRADE_UNAVAILABLE", rev)
	}
	var count int
	if db.QueryRow("SELECT count(*) FROM task").Scan(&count) != nil || (version == 1 && count > 1) {
		return bad("UPGRADE_AMBIGUOUS", rev)
	}
	var current task
	var contract []byte
	if version == 1 && count == 1 {
		if db.QueryRow("SELECT id,outcome_id,state,worker,reviewer,contract_path,inputs_json,command_json,task_revision,command_fingerprint,input_fingerprints FROM task").Scan(&current.ID, &current.OutcomeID, &current.State, &current.Worker, &current.Reviewer, &current.Contract, &current.Inputs, &current.Command, &current.Revision, &current.CommandFingerprint, &current.BindingFingerprint) != nil {
			return bad("UPGRADE_AMBIGUOUS", rev)
		}
		contractPath, e := rootFile(r.ProjectRoot, current.Contract)
		if e != nil {
			return bad("UPGRADE_AMBIGUOUS", rev)
		}
		contract, e = os.ReadFile(contractPath)
		if e != nil {
			return bad("UPGRADE_AMBIGUOUS", rev)
		}
		currentBindings, e := fingerprints(r.ProjectRoot, current.Contract, func() []string {
			var inputs []string
			_ = json.Unmarshal([]byte(current.Inputs), &inputs)
			return inputs
		}())
		if e != nil || currentBindings != current.BindingFingerprint {
			return bad("UPGRADE_AMBIGUOUS", rev)
		}
		current.CompletionScope = "legacy_unknown"
	}
	if _, err := db.Exec("VACUUM INTO ?", backup); err != nil {
		return bad("UPGRADE_UNAVAILABLE", rev)
	}
	if upgradeBeforeTransaction != nil {
		upgradeBeforeTransaction()
	}
	tx, err := db.Begin()
	if err != nil {
		return bad("STATE_BUSY", rev)
	}
	defer tx.Rollback()
	var txOwner string
	var txRev int64
	var txVersion int
	if tx.QueryRow("SELECT owner,revision FROM project").Scan(&txOwner, &txRev) != nil || txOwner != r.Actor || txRev != rev || tx.QueryRow("SELECT schema_version FROM meta").Scan(&txVersion) != nil || txVersion != version {
		return bad("STALE_REVISION", rev)
	}
	var txCount int
	if tx.QueryRow("SELECT count(*) FROM task").Scan(&txCount) != nil || txCount != count {
		return bad("UPGRADE_AMBIGUOUS", rev)
	}
	if version == 1 {
		if _, err = tx.Exec(lifecycleSchemaSQL); err != nil {
			return bad("UPGRADE_UNAVAILABLE", rev)
		}
		if _, err = tx.Exec("ALTER TABLE task ADD COLUMN completion_scope TEXT NOT NULL DEFAULT 'legacy_unknown'"); err != nil {
			return bad("UPGRADE_UNAVAILABLE", rev)
		}
	}
	if version == 2 {
		if _, err = tx.Exec("ALTER TABLE task ADD COLUMN completion_scope TEXT NOT NULL DEFAULT 'legacy_unknown'; ALTER TABLE task_revision ADD COLUMN completion_scope TEXT NOT NULL DEFAULT 'legacy_unknown'"); err != nil {
			return bad("UPGRADE_UNAVAILABLE", rev)
		}
	}
	if version == 1 && count == 1 {
		if _, err = tx.Exec("INSERT INTO task_revision(task_id,task_revision,outcome_id,contract_snapshot,contract_fingerprint,input_fingerprints,command_json,reason,completion_scope) VALUES(?,?,?,?,?,?,?,?,?)", current.ID, current.Revision, current.OutcomeID, string(contract), current.BindingFingerprint, current.BindingFingerprint, current.Command, "migrated v1 snapshot", "legacy_unknown"); err != nil {
			return bad("UPGRADE_UNAVAILABLE", rev)
		}
		if _, err = tx.Exec("INSERT INTO active_task(slot,task_id) VALUES(1,?)", current.ID); err != nil {
			return bad("UPGRADE_UNAVAILABLE", rev)
		}
	}
	if version == 1 && count == 1 {
		if _, err = tx.Exec("INSERT INTO operation_revision(operation_id,task_id,task_revision,outcome_id) SELECT id,?,?,? FROM operation", current.ID, current.Revision, current.OutcomeID); err != nil {
			return bad("UPGRADE_UNAVAILABLE", rev)
		}
		if _, err = tx.Exec("INSERT INTO review_revision(review_id,task_id,task_revision,outcome_id) SELECT rowid,?,?,? FROM review_record", current.ID, current.Revision, current.OutcomeID); err != nil {
			return bad("UPGRADE_UNAVAILABLE", rev)
		}
		if _, err = tx.Exec("INSERT INTO integration_revision(integration_id,task_id,task_revision,outcome_id) SELECT rowid,?,?,? FROM integration_record", current.ID, current.Revision, current.OutcomeID); err != nil {
			return bad("UPGRADE_UNAVAILABLE", rev)
		}
		if _, err = tx.Exec("INSERT INTO resource_revision(resource_id,task_id,task_revision,outcome_id) SELECT rowid,?,?,? FROM resource", current.ID, current.Revision, current.OutcomeID); err != nil {
			return bad("UPGRADE_UNAVAILABLE", rev)
		}
		var failures int
		if err = tx.QueryRow("SELECT (SELECT count(*) FROM operation WHERE state IN ('failed','incomplete')) + (SELECT count(*) FROM review_record WHERE accepted=0)").Scan(&failures); err != nil {
			return bad("UPGRADE_UNAVAILABLE", rev)
		}
		if _, err = tx.Exec("INSERT INTO outcome_policy(outcome_id,failure_count,gate) VALUES(?,?,?)", current.OutcomeID, failures, gateForFailures(failures, 0, 0)); err != nil {
			return bad("UPGRADE_UNAVAILABLE", rev)
		}
	}
	if _, err = tx.Exec("UPDATE meta SET schema_version=?", stateSchemaVersion); err != nil {
		return bad("UPGRADE_UNAVAILABLE", rev)
	}
	nextRev, err := next(tx, rev)
	if err != nil {
		return bad("STATE_BUSY", rev)
	}
	response := good("UPGRADED", nextRev, map[string]any{"backupPath": backup})
	encoded, _ := json.Marshal(response)
	if _, err = tx.Exec("INSERT INTO request_log(id,fingerprint,response_json) VALUES(?,?,?)", r.RequestID, canonical(r), string(encoded)); err != nil {
		return bad("UPGRADE_UNAVAILABLE", rev)
	}
	if err = tx.Commit(); err != nil {
		return bad("UPGRADE_UNAVAILABLE", rev)
	}
	return response
}

func gateForFailures(failures, reassess, review int) string {
	if failures >= 2 {
		if reassess == 0 {
			return "reassess_required"
		}
		if review != 0 {
			return "owner_decision_required"
		}
		return "outcome_review_required"
	}
	return "open"
}

func validCompletionScope(scope string) bool {
	return scope == "target" || scope == "preparation" || scope == "legacy_unknown"
}
func freshCompletionScope(scope string) bool { return scope == "target" || scope == "preparation" }

func recordRevision(tx *sql.Tx, root string, t task, reason string) error {
	path, err := rootFile(root, t.Contract)
	if err != nil {
		return err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = tx.Exec("INSERT INTO task_revision(task_id,task_revision,outcome_id,contract_snapshot,contract_fingerprint,input_fingerprints,command_json,reason,completion_scope) VALUES(?,?,?,?,?,?,?,?,?)", t.ID, t.Revision, t.OutcomeID, string(contents), t.BindingFingerprint, t.BindingFingerprint, t.Command, reason, t.CompletionScope)
	return err
}

func reviseTask(tx *sql.Tx, r Request, rev int64) Response {
	var p struct {
		TaskID, OutcomeID                                   string
		TaskRevision                                        int64
		Worker, Reviewer, Contract, Reason, CompletionScope string
		Inputs                                              []string
		Command                                             json.RawMessage
	}
	if json.Unmarshal(r.Payload, &p) != nil || p.Reason == "" || p.TaskID == "" || p.OutcomeID == "" || p.TaskRevision < 1 || p.Worker == "" || p.Reviewer == "" || p.Contract == "" || len(p.Inputs) == 0 || len(p.Command) == 0 || !freshCompletionScope(p.CompletionScope) {
		return bad("INVALID_REVISE", rev)
	}
	if p.Worker == p.Reviewer {
		return bad("INVALID_REVISE", rev)
	}
	if !owner(tx, r.Actor) {
		return bad("NOT_OWNER", rev)
	}
	current, err := loadTask(tx)
	if err != nil || current.ID != p.TaskID || current.OutcomeID != p.OutcomeID || p.TaskRevision != current.Revision+1 || current.State == "accepted" || current.State == "integrated" || unresolvedOperation(tx) {
		return bad("INVALID_REVISE", rev)
	}
	var cmd struct {
		ID, Executable, Cwd         string
		TimeoutMs, OutputLimitBytes int
	}
	if json.Unmarshal(p.Command, &cmd) != nil || cmd.ID == "" || cmd.Executable == "" || cmd.Cwd == "" || cmd.TimeoutMs < 1 || cmd.OutputLimitBytes < 1 {
		return bad("INVALID_REVISE", rev)
	}
	for _, input := range p.Inputs {
		if _, err := rootFile(r.ProjectRoot, input); err != nil {
			return bad("INVALID_REVISE", rev)
		}
	}
	bindings, err := fingerprints(r.ProjectRoot, p.Contract, p.Inputs)
	if err != nil {
		return bad("BINDING_UNAVAILABLE", rev)
	}
	command, err := commandFingerprint(r.ProjectRoot, p.Command)
	if err != nil {
		return bad("BINDING_UNAVAILABLE", rev)
	}
	n, err := next(tx, rev)
	if err != nil {
		return bad("STATE_BUSY", rev)
	}
	inputs, _ := json.Marshal(p.Inputs)
	if _, err = tx.Exec("UPDATE task SET task_revision=?,state='draft',worker=?,reviewer=?,contract_path=?,inputs_json=?,command_json=?,contract_fingerprint=?,input_fingerprints=?,command_fingerprint=?,completion_scope=? WHERE id=?", p.TaskRevision, p.Worker, p.Reviewer, p.Contract, string(inputs), string(p.Command), bindings, bindings, command, p.CompletionScope, current.ID); err != nil {
		return bad("STATE_BUSY", rev)
	}
	nextTask := task{ID: current.ID, OutcomeID: current.OutcomeID, Revision: p.TaskRevision, Contract: p.Contract, Inputs: string(inputs), Command: string(p.Command), BindingFingerprint: bindings, CompletionScope: p.CompletionScope}
	if err = recordRevision(tx, r.ProjectRoot, nextTask, p.Reason); err != nil {
		return bad("BINDING_UNAVAILABLE", rev)
	}
	return good("REVISED", n, map[string]any{})
}

func outcomePolicy(tx *sql.Tx, r Request, rev int64) Response {
	if !owner(tx, r.Actor) {
		return bad("NOT_OWNER", rev)
	}
	var p struct{ OutcomeID, Reason, ChangedApproach, NextEvidence, Decision, Findings string }
	if json.Unmarshal(r.Payload, &p) != nil || p.OutcomeID == "" {
		return bad("INVALID_OUTCOME_POLICY", rev)
	}
	var failures, reassess, review, allowance int
	if tx.QueryRow("SELECT failure_count,reassess_used,review_used,allowance FROM outcome_policy WHERE outcome_id=?", p.OutcomeID).Scan(&failures, &reassess, &review, &allowance) != nil || failures < 2 || unresolvedOperation(tx) {
		return bad("OUTCOME_NOT_BLOCKED", rev)
	}
	if r.Action == "outcome.reassess" {
		if p.Reason == "" || p.ChangedApproach == "" || p.NextEvidence == "" || reassess != 0 || allowance != 0 {
			return bad("REASSESS_REFUSED", rev)
		}
		n, err := next(tx, rev)
		if err != nil {
			return bad("STATE_BUSY", rev)
		}
		_, err = tx.Exec("UPDATE outcome_policy SET reassess_used=1,allowance=1,gate='reassessment_attempt' WHERE outcome_id=?", p.OutcomeID)
		if err != nil {
			return bad("STATE_BUSY", rev)
		}
		if _, err = tx.Exec("INSERT INTO outcome_authorization(outcome_id,kind,basis,changed_approach,next_evidence) VALUES(?,?,?,?,?)", p.OutcomeID, "reassess", p.Reason, p.ChangedApproach, p.NextEvidence); err != nil {
			return bad("STATE_BUSY", rev)
		}
		return good("REASSESS_GRANTED", n, map[string]any{})
	}
	if (p.Decision != "continue" && p.Decision != "stop") || p.Findings == "" || p.ChangedApproach == "" || p.NextEvidence == "" || reassess == 0 || review != 0 || allowance != 0 {
		return bad("OUTCOME_REVIEW_REFUSED", rev)
	}
	n, err := next(tx, rev)
	if err != nil {
		return bad("STATE_BUSY", rev)
	}
	grant := 0
	gate := "owner_decision_required"
	if p.Decision == "continue" {
		grant = 1
		gate = "review_attempt"
	}
	_, err = tx.Exec("UPDATE outcome_policy SET review_used=1,allowance=?,gate=? WHERE outcome_id=?", grant, gate, p.OutcomeID)
	if err != nil {
		return bad("STATE_BUSY", rev)
	}
	if _, err = tx.Exec("INSERT INTO outcome_authorization(outcome_id,kind,decision,basis,changed_approach,next_evidence) VALUES(?,?,?,?,?,?)", p.OutcomeID, "review", p.Decision, p.Findings, p.ChangedApproach, p.NextEvidence); err != nil {
		return bad("STATE_BUSY", rev)
	}
	return good("OUTCOME_REVIEW_GRANTED", n, map[string]any{})
}

func outcomeDecide(tx *sql.Tx, r Request, rev int64) Response {
	if !owner(tx, r.Actor) {
		return bad("NOT_OWNER", rev)
	}
	var p struct {
		OutcomeID, Decision, Basis, ChangedApproach, NextEvidence string
		Evidence                                                  []struct{ Path, SHA256 string }
	}
	if json.Unmarshal(r.Payload, &p) != nil || p.OutcomeID == "" || p.Basis == "" || (p.Decision != "continue" && p.Decision != "stop") {
		return bad("INVALID_OUTCOME_DECISION", rev)
	}
	var failures, reassess, reviewed, allowance int
	if tx.QueryRow("SELECT failure_count,reassess_used,review_used,allowance FROM outcome_policy WHERE outcome_id=?", p.OutcomeID).Scan(&failures, &reassess, &reviewed, &allowance) != nil || gateForFailures(failures, reassess, reviewed) != "owner_decision_required" || allowance != 0 || unresolvedOperation(tx) {
		return bad("OUTCOME_DECISION_NOT_REQUIRED", rev)
	}
	if p.Decision == "continue" && (p.ChangedApproach == "" || p.NextEvidence == "" || len(p.Evidence) < 1 || len(p.Evidence) > 8) {
		return bad("INVALID_OUTCOME_DECISION", rev)
	}
	if len(p.Evidence) > 8 {
		return bad("INVALID_OUTCOME_DECISION", rev)
	}
	seen := map[string]bool{}
	canonicalEvidence := make([]string, 0, len(p.Evidence))
	for _, evidence := range p.Evidence {
		if evidence.Path == "" || len(evidence.SHA256) != 64 || strings.ToLower(evidence.SHA256) != evidence.SHA256 || seen[evidence.Path] {
			return bad("INVALID_OUTCOME_EVIDENCE", rev)
		}
		if !ordinaryEvidenceFile(r.ProjectRoot, evidence.Path) {
			return bad("INVALID_OUTCOME_EVIDENCE", rev)
		}
		path, err := rootFile(r.ProjectRoot, evidence.Path)
		if err != nil {
			return bad("INVALID_OUTCOME_EVIDENCE", rev)
		}
		actual, err := digest(path)
		if err != nil || actual != evidence.SHA256 {
			return bad("INVALID_OUTCOME_EVIDENCE", rev)
		}
		seen[evidence.Path] = true
		canonicalEvidence = append(canonicalEvidence, evidence.Path+"\x00"+evidence.SHA256)
	}
	sort.Strings(canonicalEvidence)
	h := sha256.Sum256([]byte(strings.Join(canonicalEvidence, "\n")))
	evidenceDigest := hex.EncodeToString(h[:])
	n, err := next(tx, rev)
	if err != nil {
		return bad("STATE_BUSY", rev)
	}
	encoded, _ := json.Marshal(p.Evidence)
	grant := 0
	gate := "owner_decision_stopped"
	code := "OUTCOME_STOPPED"
	if p.Decision == "continue" {
		grant = 1
		gate = "owner_decision_attempt"
		code = "OUTCOME_DECISION_GRANTED"
	}
	if _, err = tx.Exec("INSERT INTO outcome_decision(outcome_id,decision,basis,changed_approach,next_evidence,evidence_json,evidence_digest,granted) VALUES(?,?,?,?,?,?,?,?)", p.OutcomeID, p.Decision, p.Basis, p.ChangedApproach, p.NextEvidence, string(encoded), evidenceDigest, grant); err != nil {
		return bad("OUTCOME_DECISION_REUSED", rev)
	}
	if _, err = tx.Exec("UPDATE outcome_policy SET allowance=?,gate=? WHERE outcome_id=?", grant, gate, p.OutcomeID); err != nil {
		return bad("STATE_BUSY", rev)
	}
	return good(code, n, map[string]any{})
}

func ordinaryEvidenceFile(root, name string) bool {
	if _, err := portableRootFile(root, name); err != nil {
		return false
	}
	if filepath.IsAbs(name) {
		return false
	}
	clean := filepath.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return false
	}
	current := root
	for _, part := range strings.Split(clean, string(os.PathSeparator)) {
		if strings.EqualFold(part, ".ocmi") || strings.EqualFold(part, ".git") || strings.HasPrefix(strings.ToLower(part), ".env") || strings.EqualFold(part, ".local") {
			return false
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	info, err := os.Stat(current)
	return err == nil && info.Mode().IsRegular()
}

func taskHistory(r Request) Response {
	db, err := openExisting(r.ProjectRoot)
	if err != nil {
		return bad("STATE_UNAVAILABLE", 0)
	}
	defer db.Close()
	version, err := databaseVersion(db)
	if err != nil {
		return bad("STATE_CORRUPT", 0)
	}
	if version != stateSchemaVersion {
		var rev int64
		_ = db.QueryRow("SELECT revision FROM project").Scan(&rev)
		return Response{SchemaVersion: schemaVersion, Code: "UPGRADE_REQUIRED", Revision: rev, Data: map[string]any{"nextAction": "state.upgrade"}}
	}
	for _, name := range []string{"active_task", "task_revision", "operation_revision", "review_revision", "integration_revision", "resource_revision", "outcome_policy", "outcome_decision", "outcome_authorization"} {
		var n int
		if db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&n) != nil || n != 1 {
			return bad("STATE_CORRUPT", 0)
		}
	}
	var p struct {
		TaskID string `json:"taskId"`
		Limit  int    `json:"limit"`
	}
	if json.Unmarshal(r.Payload, &p) != nil || p.Limit < 0 {
		return bad("INVALID_HISTORY", 0)
	}
	if p.Limit == 0 || p.Limit > 100 {
		p.Limit = 100
	}
	data := map[string]any{"tasks": []map[string]any{}, "revisions": []map[string]any{}, "operations": []map[string]any{}, "reviews": []map[string]any{}, "integrations": []map[string]any{}, "resources": []map[string]any{}, "outcomes": []map[string]any{}, "decisions": []map[string]any{}}
	closeRows := func(rows *sql.Rows) bool {
		if rows == nil {
			return false
		}
		err := rows.Err()
		rows.Close()
		return err == nil
	}
	query := "SELECT id,outcome_id,task_revision,state,completion_scope FROM task WHERE (?='' OR id=?) LIMIT ?"
	rows, e := db.Query(query, p.TaskID, p.TaskID, p.Limit)
	if e != nil {
		return bad("STATE_CORRUPT", 0)
	}
	for rows != nil && rows.Next() {
		var id, o, s, scope string
		var v int64
		if rows.Scan(&id, &o, &v, &s, &scope) != nil {
			return bad("STATE_CORRUPT", 0)
		}
		if p.TaskID == "" || p.TaskID == id {
			data["tasks"] = append(data["tasks"].([]map[string]any), map[string]any{"taskId": id, "outcomeId": o, "taskRevision": v, "state": s, "completionScope": scope})
		}
	}
	if !closeRows(rows) {
		return bad("STATE_CORRUPT", 0)
	}
	rows, e = db.Query("SELECT task_id,task_revision,outcome_id,contract_snapshot,reason,completion_scope FROM task_revision WHERE (?='' OR task_id=?) ORDER BY task_id,task_revision LIMIT ?", p.TaskID, p.TaskID, p.Limit)
	if e != nil {
		return bad("STATE_CORRUPT", 0)
	}
	for rows != nil && rows.Next() {
		var id, o, c, reason, scope string
		var v int64
		if rows.Scan(&id, &v, &o, &c, &reason, &scope) != nil {
			return bad("STATE_CORRUPT", 0)
		}
		if p.TaskID == "" || p.TaskID == id {
			data["revisions"] = append(data["revisions"].([]map[string]any), map[string]any{"taskId": id, "taskRevision": v, "outcomeId": o, "contractSnapshot": c, "reason": reason, "completionScope": scope})
		}
	}
	if !closeRows(rows) {
		return bad("STATE_CORRUPT", 0)
	}
	rows, e = db.Query("SELECT o.id,m.task_id,m.task_revision,o.state FROM operation o JOIN operation_revision m ON m.operation_id=o.id WHERE (?='' OR m.task_id=?) ORDER BY o.created_at LIMIT ?", p.TaskID, p.TaskID, p.Limit)
	if e != nil {
		return bad("STATE_CORRUPT", 0)
	}
	for rows != nil && rows.Next() {
		var id, tid, state string
		var v int64
		if rows.Scan(&id, &tid, &v, &state) != nil {
			return bad("STATE_CORRUPT", 0)
		}
		if p.TaskID == "" || p.TaskID == tid {
			data["operations"] = append(data["operations"].([]map[string]any), map[string]any{"operationId": id, "taskId": tid, "taskRevision": v, "state": state})
		}
	}
	if !closeRows(rows) {
		return bad("STATE_CORRUPT", 0)
	}
	rows, e = db.Query("SELECT r.reason,r.accepted,m.task_id,m.task_revision,tr.completion_scope FROM review_record r JOIN review_revision m ON m.review_id=r.rowid JOIN task_revision tr ON tr.task_id=m.task_id AND tr.task_revision=m.task_revision WHERE (?='' OR m.task_id=?) LIMIT ?", p.TaskID, p.TaskID, p.Limit)
	if e != nil {
		return bad("STATE_CORRUPT", 0)
	}
	for rows != nil && rows.Next() {
		var reason, tid, scope string
		var accepted int
		var v int64
		if rows.Scan(&reason, &accepted, &tid, &v, &scope) != nil {
			return bad("STATE_CORRUPT", 0)
		}
		if p.TaskID == "" || p.TaskID == tid {
			data["reviews"] = append(data["reviews"].([]map[string]any), map[string]any{"taskId": tid, "taskRevision": v, "reason": reason, "accepted": accepted != 0, "completionScope": scope})
		}
	}
	if !closeRows(rows) {
		return bad("STATE_CORRUPT", 0)
	}
	rows, e = db.Query("SELECT i.candidate,i.reason,m.task_id,m.task_revision FROM integration_record i JOIN integration_revision m ON m.integration_id=i.rowid WHERE (?='' OR m.task_id=?) LIMIT ?", p.TaskID, p.TaskID, p.Limit)
	if e != nil {
		return bad("STATE_CORRUPT", 0)
	}
	for rows != nil && rows.Next() {
		var candidate, reason, tid string
		var v int64
		if rows.Scan(&candidate, &reason, &tid, &v) != nil {
			return bad("STATE_CORRUPT", 0)
		}
		if p.TaskID == "" || p.TaskID == tid {
			data["integrations"] = append(data["integrations"].([]map[string]any), map[string]any{"taskId": tid, "taskRevision": v, "candidate": candidate, "reason": reason})
		}
	}
	if !closeRows(rows) {
		return bad("STATE_CORRUPT", 0)
	}
	rows, e = db.Query("SELECT r.disposition,r.reason,r.closed,m.task_id,m.task_revision FROM resource r JOIN resource_revision m ON m.resource_id=r.rowid WHERE (?='' OR m.task_id=?) LIMIT ?", p.TaskID, p.TaskID, p.Limit)
	if e != nil {
		return bad("STATE_CORRUPT", 0)
	}
	for rows != nil && rows.Next() {
		var disposition, reason, tid string
		var closed int
		var v int64
		if rows.Scan(&disposition, &reason, &closed, &tid, &v) != nil {
			return bad("STATE_CORRUPT", 0)
		}
		if p.TaskID == "" || p.TaskID == tid {
			data["resources"] = append(data["resources"].([]map[string]any), map[string]any{"taskId": tid, "taskRevision": v, "disposition": disposition, "reason": reason, "closed": closed != 0})
		}
	}
	if !closeRows(rows) {
		return bad("STATE_CORRUPT", 0)
	}
	rows, e = db.Query("SELECT outcome_id,failure_count,gate FROM outcome_policy WHERE (?='' OR outcome_id IN (SELECT outcome_id FROM task WHERE id=?)) LIMIT ?", p.TaskID, p.TaskID, p.Limit)
	if e != nil {
		return bad("STATE_CORRUPT", 0)
	}
	for rows != nil && rows.Next() {
		var o, g string
		var f int
		if rows.Scan(&o, &f, &g) != nil {
			return bad("STATE_CORRUPT", 0)
		}
		data["outcomes"] = append(data["outcomes"].([]map[string]any), map[string]any{"outcomeId": o, "failureCount": f, "gate": g})
	}
	if !closeRows(rows) {
		return bad("STATE_CORRUPT", 0)
	}
	rows, e = db.Query("SELECT outcome_id,decision,basis,changed_approach,next_evidence,evidence_digest,granted FROM outcome_decision WHERE (?='' OR outcome_id IN (SELECT outcome_id FROM task WHERE id=?)) LIMIT ?", p.TaskID, p.TaskID, p.Limit)
	if e != nil {
		return bad("STATE_CORRUPT", 0)
	}
	for rows != nil && rows.Next() {
		var o, d, b, c, n, h string
		var g int
		if rows.Scan(&o, &d, &b, &c, &n, &h, &g) != nil {
			return bad("STATE_CORRUPT", 0)
		}
		data["decisions"] = append(data["decisions"].([]map[string]any), map[string]any{"outcomeId": o, "decision": d, "basis": b, "changedApproach": c, "nextEvidence": n, "evidenceDigest": h, "granted": g != 0})
	}
	if !closeRows(rows) {
		return bad("STATE_CORRUPT", 0)
	}
	rows, e = db.Query("SELECT outcome_id,kind,decision,basis,changed_approach,next_evidence FROM outcome_authorization WHERE (?='' OR outcome_id IN (SELECT outcome_id FROM task WHERE id=?)) LIMIT ?", p.TaskID, p.TaskID, p.Limit)
	if e != nil {
		return bad("STATE_CORRUPT", 0)
	}
	for rows != nil && rows.Next() {
		var o, k, d, b, c, n string
		if rows.Scan(&o, &k, &d, &b, &c, &n) != nil {
			return bad("STATE_CORRUPT", 0)
		}
		data["decisions"] = append(data["decisions"].([]map[string]any), map[string]any{"outcomeId": o, "kind": k, "decision": d, "basis": b, "changedApproach": c, "nextEvidence": n})
	}
	if !closeRows(rows) {
		return bad("STATE_CORRUPT", 0)
	}
	return good("HISTORY", 0, data)
}

func _lifecycleTime() string { return time.Now().UTC().Format(time.RFC3339Nano) }
