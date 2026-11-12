package internal

// This file deliberately keeps portable state additive to the v1 tables.  A v1
// reader can still inspect its task/operation records without understanding a transfer.

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const portableSchema = 3
const maxPortableFiles = 32
const maxPortableBytes = 4 << 20
const maxPortablePackageBytes = 7 << 20

type portableFile struct {
	Content, SHA256 string
	Size            int
}
type portableTask struct {
	ID, OutcomeID, Worker, Reviewer, Contract string
	Inputs                                    []string
	Command                                   json.RawMessage
	Revision                                  int64
	CompletionScope                           string `json:"completionScope,omitempty"`
}
type portableRevision struct {
	TaskID, OutcomeID, ContractSnapshot, Reason string
	Revision                                    int64
	CompletionScope                             string `json:"completionScope,omitempty"`
}
type portableOperation struct {
	ID, TaskID, State, Output, BeforeFingerprint, AfterFingerprint, CreatedAt string
	ExitCode                                                                  *int `json:"exitCode,omitempty"`
	Truncated                                                                 bool
	TaskRevision                                                              int64 `json:"taskRevision,omitempty"`
}
type portableReview struct {
	Reason       string
	Accepted     bool
	TaskID       string `json:"taskId,omitempty"`
	TaskRevision int64  `json:"taskRevision,omitempty"`
}
type portableIntegration struct {
	Candidate, Reason, TaskID string
	TaskRevision              int64 `json:"taskRevision,omitempty"`
}
type portableResource struct {
	Disposition, Reason string
	Closed              bool
	TaskID              string `json:"taskId,omitempty"`
	TaskRevision        int64  `json:"taskRevision,omitempty"`
}
type portableOutcome struct {
	OutcomeID, Gate                                   string
	FailureCount, ReassessUsed, ReviewUsed, Allowance int
}
type portableDecision struct {
	OutcomeID, Decision, Basis, ChangedApproach, NextEvidence, EvidenceJSON, EvidenceDigest string
	Granted                                                                                 bool
}
type portableAuthorization struct{ OutcomeID, Kind, Decision, Basis, ChangedApproach, NextEvidence string }
type portablePackage struct {
	SchemaVersion int    `json:"schemaVersion"`
	System        string `json:"system"`
	TransferID    string `json:"transferId"`
	Generation    int64  `json:"generation"`
	Source        struct {
		ProjectID string `json:"projectId"`
		DeviceID  string `json:"deviceId"`
		CopyID    string `json:"copyId"`
	} `json:"source"`
	Files             map[string]portableFile `json:"files"`
	Task              portableTask            `json:"task"`
	Tasks             []portableTask          `json:"tasks,omitempty"`
	Revisions         []portableRevision      `json:"revisions,omitempty"`
	Outcomes          []portableOutcome       `json:"outcomes,omitempty"`
	Decisions         []portableDecision      `json:"decisions,omitempty"`
	Authorizations    []portableAuthorization `json:"authorizations,omitempty"`
	OriginalTaskState string                  `json:"originalTaskState"`
	Operations        []portableOperation     `json:"operations"`
	Reviews           []portableReview        `json:"reviews"`
	Integrations      []portableIntegration   `json:"integrations"`
	Resources         []portableResource      `json:"resources"`
	OutputOmitted     bool                    `json:"outputOmitted"`
	Stopped           bool                    `json:"stopped"`
	Digest            string                  `json:"digest"`
}

func ensurePortable(db *sql.DB, root string) error {
	var found int
	err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='portable_meta'").Scan(&found)
	if err != nil {
		return err
	}
	if found == 1 {
		var v int
		if err = db.QueryRow("SELECT version FROM portable_meta").Scan(&v); err != nil || v > portableSchema {
			return errors.New("newer portable schema")
		}
		return nil
	}
	backup := storePath(root) + ".portable-v1-backup"
	if _, err = os.Stat(backup); errors.Is(err, os.ErrNotExist) {
		if _, err = db.Exec("VACUUM INTO ?", backup); err != nil {
			return err
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`CREATE TABLE portable_meta(version INTEGER NOT NULL); CREATE TABLE transfer(id TEXT PRIMARY KEY, generation INTEGER NOT NULL, state TEXT NOT NULL, package_json TEXT NOT NULL, destination TEXT NOT NULL DEFAULT '');`)
	if err == nil {
		_, err = tx.Exec("INSERT INTO portable_meta VALUES(?)", portableSchema)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func portableDigest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func packageDigest(p portablePackage) string { p.Digest = ""; return portableDigest(p) }
func portable(tx *sql.Tx, r Request, rev int64) Response {
	if !owner(tx, r.Actor) {
		return bad("NOT_OWNER", rev)
	}
	switch r.Action {
	case "operation.reconcile":
		var p struct{ OperationID, Disposition, Reason string }
		if json.Unmarshal(r.Payload, &p) != nil || p.OperationID == "" || p.Disposition != "abandon" || p.Reason == "" {
			return bad("INVALID_RECONCILE", rev)
		}
		var state, outcome, origin string
		if tx.QueryRow("SELECT state FROM operation WHERE id=?", p.OperationID).Scan(&state) != nil || (state != "prepared" && state != "running" && state != "unknown") {
			return bad("NOT_UNKNOWN", rev)
		}
		if tx.QueryRow("SELECT m.outcome_id,x.origin_request_id FROM operation_revision m JOIN operation_owner x ON x.operation_id=m.operation_id WHERE m.operation_id=?", p.OperationID).Scan(&outcome, &origin) != nil {
			return bad("RECOVERY_UNSUPPORTED", rev)
		}
		if !recoveryAbsent(r.ProjectRoot, p.OperationID) {
			return bad("RECOVERY_UNSUPPORTED", rev)
		}
		n, e := next(tx, rev)
		if e != nil {
			return bad("STATE_BUSY", rev)
		}
		if _, e = tx.Exec("UPDATE operation SET state='incomplete',exit_code=NULL,output='',after_fingerprint='unavailable' WHERE id=? AND state IN ('prepared','running','unknown')", p.OperationID); e != nil {
			return bad("STATE_BUSY", rev)
		}
		if _, e = tx.Exec("UPDATE operation_owner SET recovery_reason=? WHERE operation_id=?", p.Reason, p.OperationID); e != nil {
			return bad("STATE_BUSY", rev)
		}
		var f, ra, rv int
		if tx.QueryRow("SELECT failure_count,reassess_used,review_used FROM outcome_policy WHERE outcome_id=?", outcome).Scan(&f, &ra, &rv) == nil {
			f++
			_, e = tx.Exec("UPDATE outcome_policy SET failure_count=?,gate=? WHERE outcome_id=?", f, gateForFailures(f, ra, rv), outcome)
			if e != nil {
				return bad("STATE_BUSY", rev)
			}
		}
		out := good("RECONCILED", n, map[string]any{"operationId": p.OperationID, "operationState": "incomplete"})
		b, _ := json.Marshal(out)
		if _, e = tx.Exec("UPDATE request_log SET response_json=? WHERE id=?", string(b), origin); e != nil {
			return bad("STATE_BUSY", rev)
		}
		return out
	case "handover.export":
		var p struct {
			TransferID string
			Files      []string
		}
		if json.Unmarshal(r.Payload, &p) != nil || p.TransferID == "" || len(p.Files) == 0 {
			return bad("INVALID_EXPORT", rev)
		}
		var active int
		if tx.QueryRow("SELECT count(*) FROM operation WHERE state IN ('prepared','running','unknown')").Scan(&active) != nil || active > 0 {
			return bad("TRANSFER_BLOCKED", rev)
		}
		var projectID, device, copyID string
		_ = tx.QueryRow("SELECT id,device_id,copy_id FROM project").Scan(&projectID, &device, &copyID)
		if len(p.Files) > maxPortableFiles {
			return bad("INVALID_EXPORT", rev)
		}
		entries := map[string]portableFile{}
		seen := map[string]bool{}
		total := 0
		for _, name := range p.Files {
			key := strings.ToLower(name)
			if !portableName(name) || seen[key] {
				return bad("INVALID_EXPORT_PATH", rev)
			}
			seen[key] = true
			path, e := portableRootFile(r.ProjectRoot, name)
			if e != nil {
				return bad("INVALID_EXPORT_PATH", rev)
			}
			contents, e := os.ReadFile(path)
			if e != nil || len(contents) > 1<<20 || total+len(contents) > maxPortableBytes {
				return bad("INVALID_EXPORT_PATH", rev)
			}
			total += len(contents)
			d := sha256.Sum256(contents)
			entries[name] = portableFile{Content: base64.StdEncoding.EncodeToString(contents), SHA256: hex.EncodeToString(d[:]), Size: len(contents)}
		}
		var pkg portablePackage
		var pt portableTask
		var inputs, command string
		if tx.QueryRow("SELECT t.id,t.outcome_id,t.worker,t.reviewer,t.contract_path,t.inputs_json,t.command_json,t.task_revision,t.completion_scope FROM task t JOIN active_task a ON a.task_id=t.id LIMIT 1").Scan(&pt.ID, &pt.OutcomeID, &pt.Worker, &pt.Reviewer, &pt.Contract, &inputs, &command, &pt.Revision, &pt.CompletionScope) != nil || json.Unmarshal([]byte(inputs), &pt.Inputs) != nil {
			return bad("TASK_UNBOUND", rev)
		}
		pt.Command = json.RawMessage(command)
		if _, ok := entries[pt.Contract]; !ok {
			return bad("INVALID_EXPORT_PATH", rev)
		}
		for _, input := range pt.Inputs {
			if _, ok := entries[input]; !ok {
				return bad("INVALID_EXPORT_PATH", rev)
			}
		}
		_ = tx.QueryRow("SELECT state FROM task WHERE id=?", pt.ID).Scan(&pkg.OriginalTaskState)
		tasks, _ := tx.Query("SELECT id,outcome_id,worker,reviewer,contract_path,inputs_json,command_json,task_revision,completion_scope FROM task")
		for tasks != nil && tasks.Next() {
			var x portableTask
			var in, cmd string
			_ = tasks.Scan(&x.ID, &x.OutcomeID, &x.Worker, &x.Reviewer, &x.Contract, &in, &cmd, &x.Revision, &x.CompletionScope)
			_ = json.Unmarshal([]byte(in), &x.Inputs)
			x.Command = json.RawMessage(cmd)
			pkg.Tasks = append(pkg.Tasks, x)
		}
		if tasks != nil {
			tasks.Close()
		}
		revs, _ := tx.Query("SELECT task_id,task_revision,outcome_id,contract_snapshot,reason,completion_scope FROM task_revision")
		for revs != nil && revs.Next() {
			var x portableRevision
			_ = revs.Scan(&x.TaskID, &x.Revision, &x.OutcomeID, &x.ContractSnapshot, &x.Reason, &x.CompletionScope)
			pkg.Revisions = append(pkg.Revisions, x)
		}
		if revs != nil {
			revs.Close()
		}
		rows, _ := tx.Query("SELECT o.id,m.task_id,o.state,o.exit_code,o.output,o.truncated,o.before_fingerprint,o.after_fingerprint,o.created_at,m.task_revision FROM operation o JOIN operation_revision m ON m.operation_id=o.id")
		for rows != nil && rows.Next() {
			var o portableOperation
			var exit sql.NullInt64
			var trunc int
			_ = rows.Scan(&o.ID, &o.TaskID, &o.State, &exit, &o.Output, &trunc, &o.BeforeFingerprint, &o.AfterFingerprint, &o.CreatedAt, &o.TaskRevision)
			if exit.Valid {
				v := int(exit.Int64)
				o.ExitCode = &v
			}
			o.Truncated = trunc != 0
			o.Output = ""
			pkg.Operations = append(pkg.Operations, o)
		}
		if rows != nil {
			rows.Close()
		}
		rv, _ := tx.Query("SELECT r.reason,r.accepted,m.task_id,m.task_revision FROM review_record r JOIN review_revision m ON m.review_id=r.rowid")
		for rv != nil && rv.Next() {
			var x portableReview
			var a int
			_ = rv.Scan(&x.Reason, &a, &x.TaskID, &x.TaskRevision)
			x.Accepted = a != 0
			pkg.Reviews = append(pkg.Reviews, x)
		}
		if rv != nil {
			rv.Close()
		}
		iv, _ := tx.Query("SELECT i.candidate,i.reason,m.task_id,m.task_revision FROM integration_record i JOIN integration_revision m ON m.integration_id=i.rowid")
		for iv != nil && iv.Next() {
			var x portableIntegration
			_ = iv.Scan(&x.Candidate, &x.Reason, &x.TaskID, &x.TaskRevision)
			pkg.Integrations = append(pkg.Integrations, x)
		}
		if iv != nil {
			iv.Close()
		}
		rsrc, _ := tx.Query("SELECT r.disposition,r.reason,r.closed,m.task_id,m.task_revision FROM resource r JOIN resource_revision m ON m.resource_id=r.rowid")
		for rsrc != nil && rsrc.Next() {
			var x portableResource
			var closed int
			_ = rsrc.Scan(&x.Disposition, &x.Reason, &closed, &x.TaskID, &x.TaskRevision)
			x.Closed = closed != 0
			pkg.Resources = append(pkg.Resources, x)
		}
		if rsrc != nil {
			rsrc.Close()
		}
		policies, _ := tx.Query("SELECT outcome_id,failure_count,reassess_used,review_used,allowance,gate FROM outcome_policy")
		for policies != nil && policies.Next() {
			var x portableOutcome
			_ = policies.Scan(&x.OutcomeID, &x.FailureCount, &x.ReassessUsed, &x.ReviewUsed, &x.Allowance, &x.Gate)
			pkg.Outcomes = append(pkg.Outcomes, x)
		}
		if policies != nil {
			policies.Close()
		}
		decisions, _ := tx.Query("SELECT outcome_id,decision,basis,changed_approach,next_evidence,evidence_json,evidence_digest,granted FROM outcome_decision")
		for decisions != nil && decisions.Next() {
			var x portableDecision
			var granted int
			_ = decisions.Scan(&x.OutcomeID, &x.Decision, &x.Basis, &x.ChangedApproach, &x.NextEvidence, &x.EvidenceJSON, &x.EvidenceDigest, &granted)
			x.Granted = granted != 0
			pkg.Decisions = append(pkg.Decisions, x)
		}
		if decisions != nil {
			decisions.Close()
		}
		auths, _ := tx.Query("SELECT outcome_id,kind,decision,basis,changed_approach,next_evidence FROM outcome_authorization")
		for auths != nil && auths.Next() {
			var x portableAuthorization
			_ = auths.Scan(&x.OutcomeID, &x.Kind, &x.Decision, &x.Basis, &x.ChangedApproach, &x.NextEvidence)
			pkg.Authorizations = append(pkg.Authorizations, x)
		}
		if auths != nil {
			auths.Close()
		}
		pkg.SchemaVersion, pkg.System, pkg.TransferID, pkg.Generation = portableSchema, "ocmi-runtime-preview", p.TransferID, rev+1
		pkg.Files, pkg.Task, pkg.Stopped, pkg.OutputOmitted = entries, pt, true, true
		pkg.Source.ProjectID, pkg.Source.DeviceID, pkg.Source.CopyID = projectID, device, copyID
		pkg.Digest = packageDigest(pkg)
		raw, _ := json.Marshal(pkg)
		if len(raw) > maxPortablePackageBytes {
			return bad("PACKAGE_TOO_LARGE", rev)
		}
		n, e := next(tx, rev)
		if e != nil {
			return bad("STATE_BUSY", rev)
		}
		_, e = tx.Exec("INSERT INTO transfer(id,generation,state,package_json) VALUES(?,?, 'exported',?)", p.TransferID, n, string(raw))
		if e != nil {
			return bad("TRANSFER_EXISTS", rev)
		}
		if _, e = tx.Exec("UPDATE project SET stopped=1"); e != nil {
			return bad("STATE_BUSY", rev)
		}
		return good("EXPORTED", n, map[string]any{"package": pkg, "nextAction": "handover.import"})
	case "handover.cancel":
		var p struct{ TransferID, Reason string }
		if json.Unmarshal(r.Payload, &p) != nil || p.TransferID == "" || p.Reason == "" {
			return bad("INVALID_CANCEL", rev)
		}
		return bad("CANCEL_UNSUPPORTED", rev)
	case "handover.ack":
		var p struct {
			TransferID string
			Generation int64
		}
		if json.Unmarshal(r.Payload, &p) != nil || p.TransferID == "" || p.Generation < 1 {
			return bad("INVALID_ACK", rev)
		}
		n, e := next(tx, rev)
		if e != nil {
			return bad("STATE_BUSY", rev)
		}
		res, e := tx.Exec("UPDATE transfer SET state='acknowledged' WHERE id=? AND generation=? AND state='exported'", p.TransferID, p.Generation)
		rows, _ := res.RowsAffected()
		if e != nil || rows != 1 {
			return bad("ACK_CONFLICT", rev)
		}
		return good("ACKNOWLEDGED", n, map[string]any{})
	case "handover.rebind":
		var p struct{ Command json.RawMessage }
		if json.Unmarshal(r.Payload, &p) != nil || len(p.Command) == 0 {
			return bad("INVALID_REBIND", rev)
		}
		if unresolvedOperation(tx) {
			return bad("RECOVERY_REQUIRED", rev)
		}
		t, e := loadTask(tx)
		if e != nil {
			return bad("TASK_UNBOUND", rev)
		}
		fp, e := commandFingerprint(r.ProjectRoot, p.Command)
		if e != nil {
			return bad("INVALID_REBIND", rev)
		}
		bindings, e := fingerprints(r.ProjectRoot, t.Contract, func() []string { var in []string; _ = json.Unmarshal([]byte(t.Inputs), &in); return in }())
		if e != nil {
			return bad("INVALID_REBIND", rev)
		}
		n, e := next(tx, rev)
		if e != nil {
			return bad("STATE_BUSY", rev)
		}
		_, e = tx.Exec("UPDATE task SET command_json=?,command_fingerprint=?,input_fingerprints=?,contract_fingerprint=?,state='draft' WHERE id=?", string(p.Command), fp, bindings, bindings, t.ID)
		if e != nil {
			return bad("STATE_BUSY", rev)
		}
		return good("REBIND_REQUIRED_READY", n, map[string]any{"nextAction": "task.ready"})
	}
	return bad("UNKNOWN_ACTION", rev)
}

var errImportDestinationConflict = errors.New("import destination conflict")

// importFault is internal test-only fault injection. It is never controlled by a request.
var importFault func(string) error

func importFaultAt(point string) error {
	if importFault != nil {
		return importFault(point)
	}
	return nil
}

func emptyImportRoot(root string) error {
	for current := filepath.Clean(root); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errImportDestinationConflict
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errImportDestinationConflict
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		return errImportDestinationConflict
	}
	return nil
}

func portableRootFile(root, name string) (string, error) {
	if !portableName(name) {
		return "", errors.New("nonportable path")
	}
	rawRoot := filepath.FromSlash(root)
	for _, component := range strings.Split(rawRoot, string(os.PathSeparator)) {
		if component == ".." {
			return "", errors.New("unsafe root")
		}
	}
	selectedRoot := filepath.Clean(rawRoot)
	rootInfo, err := os.Lstat(selectedRoot)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("unsafe root")
	}
	path := filepath.Join(selectedRoot, filepath.FromSlash(name))
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("unsafe selected path")
		}
		if current == selectedRoot {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("path escaped root")
		}
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("selected file unavailable")
	}
	return path, nil
}

func stagePortableImport(stage string, r Request, p struct {
	Package  portablePackage `json:"package"`
	DeviceID string          `json:"deviceId"`
	CopyID   string          `json:"copyId"`
}, names []string) error {
	for _, name := range names {
		contents, _ := base64.StdEncoding.DecodeString(p.Package.Files[name].Content)
		target := filepath.Join(stage, name)
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(target, contents, 0600); err != nil {
			return err
		}
	}
	init := Request{SchemaVersion: 1, Action: "init", ProjectRoot: stage, Actor: r.Actor, RequestID: r.RequestID + "-init", Payload: mustJSON(map[string]string{"projectId": p.Package.Source.ProjectID, "deviceId": p.DeviceID, "copyId": p.CopyID, "owner": r.Actor})}
	if response := initialize(init); !response.OK {
		return errors.New(response.Code)
	}
	db, err := openExisting(stage)
	if err != nil {
		return err
	}
	defer db.Close()
	if err = ensurePortable(db, stage); err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tasks := p.Package.Tasks
	if len(tasks) == 0 {
		tasks = []portableTask{p.Package.Task}
	}
	for _, importedTask := range tasks {
		if importedTask.CompletionScope == "" {
			importedTask.CompletionScope = "legacy_unknown"
		}
		if !validCompletionScope(importedTask.CompletionScope) {
			return errors.New("invalid imported completion scope")
		}
		inputs, _ := json.Marshal(importedTask.Inputs) // Imported bindings remain quarantined until explicit rebind/ready.
		if _, err = tx.Exec("INSERT INTO task VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)", importedTask.ID, importedTask.OutcomeID, importedTask.Revision, "draft", importedTask.Worker, importedTask.Reviewer, importedTask.Contract, string(inputs), string(importedTask.Command), "stale-import", "stale-import", "stale-import", importedTask.CompletionScope); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("INSERT INTO active_task(slot,task_id) VALUES(1,?)", p.Package.Task.ID); err != nil {
		return err
	}
	for _, revision := range p.Package.Revisions {
		if revision.CompletionScope == "" {
			revision.CompletionScope = "legacy_unknown"
		}
		if !validCompletionScope(revision.CompletionScope) {
			return errors.New("invalid imported revision scope")
		}
		if _, err = tx.Exec("INSERT INTO task_revision(task_id,task_revision,outcome_id,contract_snapshot,contract_fingerprint,input_fingerprints,command_json,reason,completion_scope) VALUES(?,?,?,?,?,?,?,?,?)", revision.TaskID, revision.Revision, revision.OutcomeID, revision.ContractSnapshot, "stale-import", "stale-import", "{}", revision.Reason, revision.CompletionScope); err != nil {
			return err
		}
	}
	if len(p.Package.Revisions) == 0 {
		scope := p.Package.Task.CompletionScope
		if scope == "" {
			scope = "legacy_unknown"
		}
		if !validCompletionScope(scope) {
			return errors.New("invalid imported task scope")
		}
		if _, err = tx.Exec("INSERT INTO task_revision(task_id,task_revision,outcome_id,contract_snapshot,contract_fingerprint,input_fingerprints,command_json,reason,completion_scope) VALUES(?,?,?,?,?,?,?,?,?)", p.Package.Task.ID, p.Package.Task.Revision, p.Package.Task.OutcomeID, "", "stale-import", "stale-import", string(p.Package.Task.Command), "imported v1 snapshot", scope); err != nil {
			return err
		}
	}
	for _, operation := range p.Package.Operations {
		if _, err = tx.Exec("INSERT INTO operation(id,task_id,state,exit_code,output,truncated,before_fingerprint,after_fingerprint,created_at) VALUES(?,?,?,?,?,?,?,?,?)", operation.ID, p.Package.Task.ID, operation.State, operation.ExitCode, "", boolInt(operation.Truncated), operation.BeforeFingerprint, operation.AfterFingerprint, operation.CreatedAt); err != nil {
			return err
		}
		tid, tr := operation.TaskID, operation.TaskRevision
		if tid == "" {
			tid = p.Package.Task.ID
		}
		if tr == 0 {
			tr = p.Package.Task.Revision
		}
		if _, err = tx.Exec("INSERT INTO operation_revision(operation_id,task_id,task_revision,outcome_id) VALUES(?,?,?,?)", operation.ID, tid, tr, p.Package.Task.OutcomeID); err != nil {
			return err
		}
	}
	for _, review := range p.Package.Reviews {
		result, err := tx.Exec("INSERT INTO review_record(reason,accepted) VALUES(?,?)", review.Reason, boolInt(review.Accepted))
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		tid, tr := review.TaskID, review.TaskRevision
		if tid == "" {
			tid = p.Package.Task.ID
		}
		if tr == 0 {
			tr = p.Package.Task.Revision
		}
		if _, err = tx.Exec("INSERT INTO review_revision(review_id,task_id,task_revision,outcome_id) VALUES(?,?,?,?)", id, tid, tr, p.Package.Task.OutcomeID); err != nil {
			return err
		}
	}
	for _, integration := range p.Package.Integrations {
		result, err := tx.Exec("INSERT INTO integration_record(candidate,reason) VALUES(?,?)", integration.Candidate, integration.Reason)
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		tid, tr := integration.TaskID, integration.TaskRevision
		if tid == "" {
			tid = p.Package.Task.ID
		}
		if tr == 0 {
			tr = p.Package.Task.Revision
		}
		if _, err = tx.Exec("INSERT INTO integration_revision(integration_id,task_id,task_revision,outcome_id) VALUES(?,?,?,?)", id, tid, tr, p.Package.Task.OutcomeID); err != nil {
			return err
		}
	}
	for _, resource := range p.Package.Resources {
		result, err := tx.Exec("INSERT INTO resource(disposition,reason,closed) VALUES(?,?,?)", resource.Disposition, resource.Reason, boolInt(resource.Closed))
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		tid, tr := resource.TaskID, resource.TaskRevision
		if tid == "" {
			tid = p.Package.Task.ID
		}
		if tr == 0 {
			tr = p.Package.Task.Revision
		}
		if _, err = tx.Exec("INSERT INTO resource_revision(resource_id,task_id,task_revision,outcome_id) VALUES(?,?,?,?)", id, tid, tr, p.Package.Task.OutcomeID); err != nil {
			return err
		}
	}
	for _, outcome := range p.Package.Outcomes {
		if _, err = tx.Exec("INSERT INTO outcome_policy(outcome_id,failure_count,reassess_used,review_used,allowance,gate) VALUES(?,?,?,?,?,?)", outcome.OutcomeID, outcome.FailureCount, outcome.ReassessUsed, outcome.ReviewUsed, outcome.Allowance, outcome.Gate); err != nil {
			return err
		}
	}
	for _, decision := range p.Package.Decisions {
		if _, err = tx.Exec("INSERT INTO outcome_decision(outcome_id,decision,basis,changed_approach,next_evidence,evidence_json,evidence_digest,granted) VALUES(?,?,?,?,?,?,?,?)", decision.OutcomeID, decision.Decision, decision.Basis, decision.ChangedApproach, decision.NextEvidence, decision.EvidenceJSON, decision.EvidenceDigest, boolInt(decision.Granted)); err != nil {
			return err
		}
	}
	for _, auth := range p.Package.Authorizations {
		if _, err = tx.Exec("INSERT INTO outcome_authorization(outcome_id,kind,decision,basis,changed_approach,next_evidence) VALUES(?,?,?,?,?,?)", auth.OutcomeID, auth.Kind, auth.Decision, auth.Basis, auth.ChangedApproach, auth.NextEvidence); err != nil {
			return err
		}
	}
	if len(p.Package.Outcomes) == 0 {
		if _, err = tx.Exec("INSERT INTO outcome_policy(outcome_id) VALUES(?)", p.Package.Task.OutcomeID); err != nil {
			return err
		}
	}
	if p.Package.Stopped {
		if _, err = tx.Exec("UPDATE project SET stopped=1"); err != nil {
			return err
		}
	}
	raw, _ := json.Marshal(p.Package)
	if _, err = tx.Exec("INSERT INTO transfer(id,generation,state,package_json) VALUES(?,?, 'imported',?)", p.Package.TransferID, p.Package.Generation, string(raw)); err != nil {
		return err
	}
	response := good("IMPORTED", 0, map[string]any{"transferId": p.Package.TransferID, "nextAction": "handover.ack_or_rebind"})
	stored, _ := json.Marshal(response)
	if _, err = tx.Exec("INSERT INTO request_log(id,fingerprint,response_json) VALUES(?,?,?)", r.RequestID, canonical(r), string(stored)); err != nil {
		return err
	}
	return tx.Commit()
}

func importPortable(r Request) Response {
	var p struct {
		Package  portablePackage `json:"package"`
		DeviceID string          `json:"deviceId"`
		CopyID   string          `json:"copyId"`
	}
	if len(r.Payload) > maxPortablePackageBytes || json.Unmarshal(r.Payload, &p) != nil || (p.Package.SchemaVersion != 1 && p.Package.SchemaVersion != 2 && p.Package.SchemaVersion != portableSchema) || p.Package.System != "ocmi-runtime-preview" || p.Package.TransferID == "" || p.Package.Generation < 1 || p.Package.Source.ProjectID == "" || len(p.Package.Files) == 0 || len(p.Package.Files) > maxPortableFiles || p.DeviceID == "" || p.CopyID == "" || p.Package.Task.ID == "" || p.Package.Task.OutcomeID == "" || p.Package.Task.Worker == "" || p.Package.Task.Reviewer == "" || p.Package.Task.Contract == "" || len(p.Package.Task.Inputs) == 0 || len(p.Package.Task.Command) == 0 {
		return bad("INVALID_PACKAGE", 0)
	}
	if p.Package.Digest != packageDigest(p.Package) {
		return bad("PACKAGE_CHECKSUM", 0)
	}
	if p.Package.SchemaVersion == portableSchema {
		tasks := p.Package.Tasks
		if len(tasks) == 0 {
			tasks = []portableTask{p.Package.Task}
		}
		scopes := map[string]string{}
		for _, revision := range p.Package.Revisions {
			if !validCompletionScope(revision.CompletionScope) {
				return bad("INVALID_PACKAGE", 0)
			}
			scopes[revision.TaskID+"\x00"+strconv.FormatInt(revision.Revision, 10)] = revision.CompletionScope
		}
		if !validCompletionScope(p.Package.Task.CompletionScope) || scopes[p.Package.Task.ID+"\x00"+strconv.FormatInt(p.Package.Task.Revision, 10)] != p.Package.Task.CompletionScope {
			return bad("INVALID_PACKAGE", 0)
		}
		for _, task := range tasks {
			if !validCompletionScope(task.CompletionScope) || scopes[task.ID+"\x00"+strconv.FormatInt(task.Revision, 10)] != task.CompletionScope {
				return bad("INVALID_PACKAGE", 0)
			}
		}
	} else {
		if p.Package.Task.CompletionScope != "" {
			return bad("INVALID_PACKAGE", 0)
		}
		for _, task := range p.Package.Tasks {
			if task.CompletionScope != "" {
				return bad("INVALID_PACKAGE", 0)
			}
		}
		for _, revision := range p.Package.Revisions {
			if revision.CompletionScope != "" {
				return bad("INVALID_PACKAGE", 0)
			}
		}
	}
	if existing, err := openExisting(r.ProjectRoot); err == nil {
		defer existing.Close()
		var saved string
		if existing.QueryRow("SELECT package_json FROM transfer WHERE id=? AND generation=? AND state='imported'", p.Package.TransferID, p.Package.Generation).Scan(&saved) == nil && saved == string(mustJSON(p.Package)) {
			return good("IMPORTED", 0, map[string]any{"transferId": p.Package.TransferID, "nextAction": "handover.ack_or_rebind"})
		}
		return bad("DESTINATION_CONFLICT", 0)
	}
	if e := emptyImportRoot(r.ProjectRoot); e != nil {
		return bad("DESTINATION_CONFLICT", 0)
	}
	// Validate the complete bounded package before writing even owned staging files.
	names := make([]string, 0, len(p.Package.Files))
	seen := map[string]bool{}
	total := 0
	for name, entry := range p.Package.Files {
		key := strings.ToLower(name)
		if seen[key] {
			return bad("INVALID_PACKAGE_PATH", 0)
		}
		seen[key] = true
		if !portableName(name) || entry.Size < 0 || entry.Size > 1<<20 {
			return bad("INVALID_PACKAGE_PATH", 0)
		}
		b, e := base64.StdEncoding.DecodeString(entry.Content)
		d := sha256.Sum256(b)
		if e != nil || len(b) != entry.Size || hex.EncodeToString(d[:]) != entry.SHA256 || total+len(b) > maxPortableBytes {
			return bad("PACKAGE_CHECKSUM", 0)
		}
		total += len(b)
		names = append(names, name)
	}
	if _, ok := p.Package.Files[p.Package.Task.Contract]; !ok {
		return bad("INVALID_PACKAGE_PATH", 0)
	}
	for _, name := range p.Package.Task.Inputs {
		if _, ok := p.Package.Files[name]; !ok {
			return bad("INVALID_PACKAGE_PATH", 0)
		}
	}
	sort.Strings(names)
	stage, e := os.MkdirTemp(filepath.Dir(r.ProjectRoot), ".ocmi-import-")
	if e != nil {
		return bad("DESTINATION_UNAVAILABLE", 0)
	}
	defer os.RemoveAll(stage)
	if e = stagePortableImport(stage, r, p, names); e != nil {
		return bad("DESTINATION_UNAVAILABLE", 0)
	}
	if e = importFaultAt("before-activation"); e != nil {
		return bad("DESTINATION_UNAVAILABLE", 0)
	}
	if e = emptyImportRoot(r.ProjectRoot); e != nil {
		return bad("DESTINATION_CONFLICT", 0)
	}
	backup := stage + "-empty-root"
	if e = os.Rename(r.ProjectRoot, backup); e != nil {
		return bad("DESTINATION_UNAVAILABLE", 0)
	}
	if e = importFaultAt("after-empty-root-rename"); e != nil {
		_ = os.Rename(backup, r.ProjectRoot)
		return bad("DESTINATION_UNAVAILABLE", 0)
	}
	if e = os.Rename(stage, r.ProjectRoot); e != nil {
		_ = os.Rename(backup, r.ProjectRoot)
		return bad("DESTINATION_UNAVAILABLE", 0)
	}
	stage = ""
	if e = os.Remove(backup); e != nil {
		return bad("DESTINATION_UNAVAILABLE", 0)
	}
	return good("IMPORTED", 0, map[string]any{"transferId": p.Package.TransferID, "nextAction": "handover.ack_or_rebind"})
}
func portableName(name string) bool {
	if name == "" || filepath.IsAbs(name) || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") || strings.Contains(name, "//") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		lower := strings.ToLower(part)
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") || lower == ".ocmi" || lower == ".git" || strings.HasPrefix(lower, ".env") || strings.HasSuffix(lower, ".sqlite") || strings.HasSuffix(lower, ".db") || strings.HasSuffix(lower, "-wal") || strings.HasSuffix(lower, "-shm") || strings.HasSuffix(lower, ".key") || strings.HasSuffix(lower, ".pem") {
			return false
		}
		base := strings.Split(lower, ".")[0]
		if base == "con" || base == "prn" || base == "aux" || base == "nul" || (len(base) == 4 && (strings.HasPrefix(base, "com") || strings.HasPrefix(base, "lpt")) && base[3] >= '1' && base[3] <= '9') {
			return false
		}
	}
	return true
}
func mustJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

var _ = time.Now
