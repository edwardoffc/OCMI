// Package internal implements the bounded local OCMI CLI state service.
package internal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

const schemaVersion = 1
const stateSchemaVersion = 2

type Request struct {
	SchemaVersion    int             `json:"schemaVersion"`
	Action           string          `json:"action"`
	ProjectRoot      string          `json:"projectRoot"`
	Actor            string          `json:"actor"`
	RequestID        string          `json:"requestId"`
	ExpectedRevision int64           `json:"expectedRevision"`
	Payload          json.RawMessage `json:"payload"`
}

type Response struct {
	SchemaVersion int            `json:"schemaVersion"`
	OK            bool           `json:"ok"`
	Code          string         `json:"code"`
	Revision      int64          `json:"revision"`
	Data          map[string]any `json:"data"`
}

type task struct {
	ID, OutcomeID, State, Worker, Reviewer, Contract, Inputs, Command string
	CommandFingerprint                                                string
	BindingFingerprint                                                string
	Revision                                                          int64
}

func bad(code string, revision int64) Response {
	return Response{SchemaVersion: schemaVersion, OK: false, Code: code, Revision: revision, Data: map[string]any{}}
}
func good(code string, revision int64, data map[string]any) Response {
	return Response{SchemaVersion: schemaVersion, OK: true, Code: code, Revision: revision, Data: data}
}

func Handle(r Request) Response {
	if r.SchemaVersion != schemaVersion || r.Action == "" || r.Actor == "" || r.ProjectRoot == "" || r.RequestID == "" || r.ExpectedRevision < 0 || !filepath.IsAbs(r.ProjectRoot) {
		return bad("INVALID_REQUEST", 0)
	}
	if len(r.Payload) == 0 || !json.Valid(r.Payload) || string(r.Payload) == "null" {
		return bad("INVALID_PAYLOAD", 0)
	}
	if r.Action == "status" {
		return status(r)
	}
	if r.Action == "task.history" {
		return taskHistory(r)
	}
	if r.Action == "operation.inspect" {
		return inspectOperation(r)
	}
	if r.Action == "init" {
		return initialize(r)
	}
	if r.Action == "handover.import" {
		return importPortable(r)
	}
	db, err := openExisting(r.ProjectRoot)
	if err != nil {
		return bad("STATE_UNAVAILABLE", 0)
	}
	defer db.Close()
	stateVersion, err := databaseVersion(db)
	if err != nil {
		return bad("STATE_CORRUPT", 0)
	}
	if stateVersion == 1 {
		if r.Action == "status" || r.Action == "task.history" {
			return bad("UPGRADE_REQUIRED", 0)
		}
		if r.Action != "state.upgrade" {
			return bad("UPGRADE_REQUIRED", 0)
		}
	}
	if r.Action == "state.upgrade" {
		return upgradeState(db, r, stateVersion)
	}
	if r.Action == "handover.export" || r.Action == "handover.cancel" || r.Action == "handover.ack" || r.Action == "handover.rebind" {
		if err := ensurePortable(db, r.ProjectRoot); err != nil {
			return bad("PORTABLE_SCHEMA_UNAVAILABLE", 0)
		}
	}
	return mutate(db, r)
}

func storePath(root string) string { return filepath.Join(root, ".ocmi", "local", "state.sqlite") }
func openExisting(root string) (*sql.DB, error) {
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return nil, errors.New("root missing")
	}
	if _, err := os.Stat(storePath(root)); err != nil {
		return nil, err
	}
	if err := confinedStore(root); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.ToSlash(storePath(root)))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec("PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;")
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func initialize(r Request) Response {
	var p struct{ ProjectID, DeviceID, CopyID, Owner string }
	if err := json.Unmarshal(r.Payload, &p); err != nil || p.ProjectID == "" || p.DeviceID == "" || p.CopyID == "" || p.Owner == "" || r.Actor != p.Owner {
		return bad("INVALID_INIT", 0)
	}
	info, err := os.Stat(r.ProjectRoot)
	if err != nil || !info.IsDir() {
		return bad("ROOT_NOT_FOUND", 0)
	}
	if err := privateBoundary(r.ProjectRoot); err != nil {
		return bad("PRIVATE_BOUNDARY_UNSAFE", 0)
	}
	path := storePath(r.ProjectRoot)
	if _, err = os.Stat(path); err == nil {
		db, openErr := openExisting(r.ProjectRoot)
		if openErr != nil || verifySchema(db) != nil {
			if db != nil {
				db.Close()
			}
			return bad("STORE_EXISTS", 0)
		}
		defer db.Close()
		var id, device, copyID, owner string
		if err = db.QueryRow("SELECT id,device_id,copy_id,owner FROM project").Scan(&id, &device, &copyID, &owner); err != nil || id != p.ProjectID || device != p.DeviceID || copyID != p.CopyID || owner != p.Owner {
			return bad("STORE_EXISTS", 0)
		}
		var fingerprint, saved string
		if err = db.QueryRow("SELECT fingerprint,response_json FROM request_log WHERE id=?", r.RequestID).Scan(&fingerprint, &saved); err != nil {
			return bad("IDEMPOTENCY_CONFLICT", 0)
		}
		if fingerprint != canonical(r) {
			return bad("IDEMPOTENCY_CONFLICT", 0)
		}
		var response Response
		if json.Unmarshal([]byte(saved), &response) != nil {
			return bad("STATE_CORRUPT", 0)
		}
		return response
	}
	if !errors.Is(err, os.ErrNotExist) {
		return bad("STORE_UNAVAILABLE", 0)
	}
	lock := filepath.Join(r.ProjectRoot, ".ocmi", "local", ".init-lock")
	if err = os.MkdirAll(filepath.Dir(lock), 0700); err != nil {
		return bad("STORE_UNAVAILABLE", 0)
	}
	if err = confinedStoreRoot(r.ProjectRoot); err != nil {
		return bad("PRIVATE_BOUNDARY_UNSAFE", 0)
	}
	if err = os.Mkdir(lock, 0700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return bad("INIT_IN_PROGRESS", 0)
		}
		return bad("STORE_UNAVAILABLE", 0)
	}
	defer os.Remove(lock)
	if _, err = os.Stat(path); err == nil {
		return bad("INIT_IN_PROGRESS", 0)
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return bad("STORE_UNAVAILABLE", 0)
	}
	temporary := path + ".initializing-" + uuid.NewString()
	db, err := sql.Open("sqlite", filepath.ToSlash(temporary))
	if err != nil {
		return bad("STORE_UNAVAILABLE", 0)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err == nil {
		_, err = tx.Exec(schemaSQL)
	}
	if err == nil {
		_, err = tx.Exec(lifecycleSchemaSQL)
	}
	if err == nil {
		_, err = tx.Exec("INSERT INTO meta(schema_version) VALUES(?)", stateSchemaVersion)
	}
	if err == nil {
		_, err = tx.Exec("INSERT INTO project(id,device_id,copy_id,owner,revision,stopped) VALUES(?,?,?,?,0,0)", p.ProjectID, p.DeviceID, p.CopyID, p.Owner)
	}
	if err == nil {
		response, _ := json.Marshal(good("INITIALIZED", 0, map[string]any{}))
		_, err = tx.Exec("INSERT INTO request_log(id,fingerprint,response_json) VALUES(?,?,?)", r.RequestID, canonical(r), string(response))
	}
	if err == nil {
		err = tx.Commit()
	} else if tx != nil {
		_ = tx.Rollback()
	}
	if err != nil {
		_ = os.Remove(temporary)
		return bad("STORE_UNAVAILABLE", 0)
	}
	if err = db.Close(); err != nil {
		return bad("STORE_UNAVAILABLE", 0)
	}
	if err = os.Rename(temporary, path); err != nil {
		return bad("STORE_UNAVAILABLE", 0)
	}
	return good("INITIALIZED", 0, map[string]any{})
}

func privateBoundary(root string) error {
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		cmd := exec.Command("git", "-C", root, "check-ignore", "-q", "--", ".ocmi/local/state.sqlite")
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return confinedStoreRoot(root)
}
func confinedStoreRoot(root string) error {
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	for _, relative := range []string{".ocmi", ".ocmi/local"} {
		path := filepath.Join(root, relative)
		if _, err := os.Lstat(path); err == nil {
			real, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(rootReal, real)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				return errors.New("store escape")
			}
		}
	}
	return nil
}
func confinedStore(root string) error {
	if err := confinedStoreRoot(root); err != nil {
		return err
	}
	real, err := filepath.EvalSymlinks(storePath(root))
	if err != nil {
		return err
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootReal, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return errors.New("store escape")
	}
	return nil
}

const schemaSQL = `
CREATE TABLE meta(schema_version INTEGER NOT NULL);
CREATE TABLE project(id TEXT PRIMARY KEY, device_id TEXT NOT NULL, copy_id TEXT NOT NULL, owner TEXT NOT NULL, revision INTEGER NOT NULL, stopped INTEGER NOT NULL);
CREATE TABLE task(id TEXT PRIMARY KEY, outcome_id TEXT NOT NULL, task_revision INTEGER NOT NULL, state TEXT NOT NULL, worker TEXT NOT NULL, reviewer TEXT NOT NULL, contract_path TEXT NOT NULL, inputs_json TEXT NOT NULL, command_json TEXT NOT NULL, contract_fingerprint TEXT NOT NULL, input_fingerprints TEXT NOT NULL, command_fingerprint TEXT NOT NULL);
CREATE TABLE operation(id TEXT PRIMARY KEY, task_id TEXT NOT NULL REFERENCES task(id), state TEXT NOT NULL, exit_code INTEGER, output TEXT NOT NULL, truncated INTEGER NOT NULL, before_fingerprint TEXT NOT NULL, after_fingerprint TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE TABLE request_log(id TEXT PRIMARY KEY, fingerprint TEXT NOT NULL, response_json TEXT NOT NULL);
CREATE TABLE review_record(reason TEXT NOT NULL, accepted INTEGER NOT NULL);
CREATE TABLE integration_record(candidate TEXT NOT NULL, reason TEXT NOT NULL);
CREATE TABLE resource(disposition TEXT, reason TEXT, closed INTEGER NOT NULL DEFAULT 0);`

const lifecycleSchemaSQL = `
CREATE TABLE task_revision(task_id TEXT NOT NULL, task_revision INTEGER NOT NULL, outcome_id TEXT NOT NULL, contract_snapshot TEXT NOT NULL, contract_fingerprint TEXT NOT NULL, input_fingerprints TEXT NOT NULL, command_json TEXT NOT NULL, reason TEXT NOT NULL, PRIMARY KEY(task_id,task_revision));
CREATE TABLE operation_revision(operation_id TEXT PRIMARY KEY, task_id TEXT NOT NULL, task_revision INTEGER NOT NULL, outcome_id TEXT NOT NULL);
CREATE TABLE operation_owner(operation_id TEXT PRIMARY KEY, phase TEXT NOT NULL, root TEXT NOT NULL, copy_id TEXT NOT NULL, device_id TEXT NOT NULL, context_hash TEXT NOT NULL, parent_pid INTEGER NOT NULL, child_pid INTEGER, process_group INTEGER, job_attached INTEGER NOT NULL DEFAULT 0, origin_request_id TEXT NOT NULL, recovery_reason TEXT NOT NULL DEFAULT '');
CREATE TABLE review_revision(review_id INTEGER PRIMARY KEY, task_id TEXT NOT NULL, task_revision INTEGER NOT NULL, outcome_id TEXT NOT NULL);
CREATE TABLE integration_revision(integration_id INTEGER PRIMARY KEY, task_id TEXT NOT NULL, task_revision INTEGER NOT NULL, outcome_id TEXT NOT NULL);
CREATE TABLE resource_revision(resource_id INTEGER PRIMARY KEY, task_id TEXT NOT NULL, task_revision INTEGER NOT NULL, outcome_id TEXT NOT NULL);
CREATE TABLE outcome_policy(outcome_id TEXT PRIMARY KEY, failure_count INTEGER NOT NULL DEFAULT 0, reassess_used INTEGER NOT NULL DEFAULT 0, review_used INTEGER NOT NULL DEFAULT 0, allowance INTEGER NOT NULL DEFAULT 0, gate TEXT NOT NULL DEFAULT 'open');
CREATE TABLE outcome_decision(id INTEGER PRIMARY KEY, outcome_id TEXT NOT NULL, decision TEXT NOT NULL, basis TEXT NOT NULL, changed_approach TEXT NOT NULL, next_evidence TEXT NOT NULL, evidence_json TEXT NOT NULL, evidence_digest TEXT NOT NULL UNIQUE, granted INTEGER NOT NULL);
CREATE TABLE outcome_authorization(id INTEGER PRIMARY KEY, outcome_id TEXT NOT NULL, kind TEXT NOT NULL, decision TEXT NOT NULL DEFAULT '', basis TEXT NOT NULL, changed_approach TEXT NOT NULL, next_evidence TEXT NOT NULL);
CREATE TABLE active_task(slot INTEGER PRIMARY KEY CHECK(slot=1), task_id TEXT NOT NULL);`

func databaseVersion(db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRow("SELECT schema_version FROM meta").Scan(&version); err != nil || (version != 1 && version != stateSchemaVersion) {
		return 0, errors.New("unsupported schema")
	}
	var n int
	if err := db.QueryRow("SELECT count(*) FROM project").Scan(&n); err != nil {
		return 0, err
	}
	return version, nil
}
func verifySchema(db *sql.DB) error { _, err := databaseVersion(db); return err }
func revision(tx *sql.Tx) (int64, error) {
	var v int64
	err := tx.QueryRow("SELECT revision FROM project").Scan(&v)
	return v, err
}

func canonical(r Request) string {
	var p any
	_ = json.Unmarshal(r.Payload, &p)
	b, _ := json.Marshal(struct {
		Action, Root, Actor string
		Payload             any
	}{r.Action, r.ProjectRoot, r.Actor, p})
	return string(b)
}

func mutate(db *sql.DB, r Request) Response {
	tx, err := db.Begin()
	if err != nil {
		return bad("STATE_BUSY", 0)
	}
	defer tx.Rollback()
	fingerprint := canonical(r)
	var prior, stored string
	err = tx.QueryRow("SELECT fingerprint,response_json FROM request_log WHERE id=?", r.RequestID).Scan(&prior, &stored)
	if err == nil {
		if prior != fingerprint {
			return bad("IDEMPOTENCY_CONFLICT", 0)
		}
		var out Response
		if json.Unmarshal([]byte(stored), &out) != nil {
			return bad("STATE_CORRUPT", 0)
		}
		return out
	}
	if err != sql.ErrNoRows {
		return bad("STATE_CORRUPT", 0)
	}
	rev, err := revision(tx)
	if err != nil {
		return bad("STATE_CORRUPT", 0)
	}
	if r.ExpectedRevision != rev {
		return bad("STALE_REVISION", rev)
	}
	if sourceTransferFrozen(tx) && r.Action != "handover.ack" {
		return bad("SOURCE_TRANSFERRED", rev)
	}
	if r.Action == "task.bind" || r.Action == "task.ready" || r.Action == "run" || r.Action == "handover.rebind" || r.Action == "handover.export" {
		if unresolvedOperation(tx) {
			return bad("RECOVERY_REQUIRED", rev)
		}
	}
	var out Response
	switch r.Action {
	case "task.bind":
		out = bind(tx, r, rev)
	case "task.revise":
		out = reviseTask(tx, r, rev)
	case "outcome.reassess", "outcome.review":
		out = outcomePolicy(tx, r, rev)
	case "outcome.decide":
		out = outcomeDecide(tx, r, rev)
	case "task.ready":
		out = ready(tx, r, rev)
	case "stop":
		out = stop(tx, r, rev)
	case "run":
		out = prepareRun(tx, r, rev)
	case "submit":
		out = submit(tx, r, rev)
	case "review":
		out = review(tx, r, rev)
	case "integrate":
		out = integrate(tx, r, rev)
	case "resource.close":
		out = closeResource(tx, r, rev)
	case "operation.reconcile", "handover.export", "handover.cancel", "handover.ack", "handover.rebind":
		out = portable(tx, r, rev)
	default:
		return bad("UNKNOWN_ACTION", rev)
	}
	if !out.OK {
		return out
	}
	b, _ := json.Marshal(out)
	if _, err = tx.Exec("INSERT INTO request_log(id,fingerprint,response_json) VALUES(?,?,?)", r.RequestID, fingerprint, string(b)); err != nil {
		return bad("STATE_BUSY", rev)
	}
	if err = tx.Commit(); err != nil {
		return bad("STATE_BUSY", rev)
	}
	if r.Action == "run" {
		return executeRun(db, r, out)
	}
	return out
}

func next(tx *sql.Tx, rev int64) (int64, error) {
	n := rev + 1
	result, e := tx.Exec("UPDATE project SET revision=? WHERE revision=?", n, rev)
	if e != nil {
		return 0, e
	}
	affected, e := result.RowsAffected()
	if e != nil || affected != 1 {
		return 0, errors.New("stale revision")
	}
	return n, nil
}
func rootFile(root, path string) (string, error) {
	if filepath.IsAbs(path) || path == "" {
		return "", errors.New("bad path")
	}
	clean := filepath.Clean(path)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", errors.New("escape")
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(filepath.Join(rootReal, clean))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootReal, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("escape")
	}
	return real, nil
}
func digest(path string) (string, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
func fingerprints(root string, contract string, inputs []string) (string, error) {
	c, e := rootFile(root, contract)
	if e != nil {
		return "", e
	}
	out := []string{}
	d, e := digest(c)
	if e != nil {
		return "", e
	}
	out = append(out, "contract:"+d)
	for _, p := range inputs {
		f, e := rootFile(root, p)
		if e != nil {
			return "", e
		}
		d, e := digest(f)
		if e != nil {
			return "", e
		}
		out = append(out, p+":"+d)
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}
func commandFingerprint(root string, raw json.RawMessage) (string, error) {
	var c struct{ Executable, Cwd string }
	if json.Unmarshal(raw, &c) != nil || c.Executable == "" || c.Cwd == "" {
		return "", errors.New("bad command")
	}
	if _, err := rootFile(root, c.Cwd); err != nil {
		return "", err
	}
	executable := c.Executable
	if !filepath.IsAbs(executable) {
		var err error
		executable, err = rootFile(root, executable)
		if err != nil {
			return "", err
		}
	}
	d, err := digest(executable)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(struct {
		Command    json.RawMessage
		Executable string
	}{raw, d})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
func stopped(tx *sql.Tx) bool {
	var value int
	return tx.QueryRow("SELECT stopped FROM project").Scan(&value) == nil && value != 0
}
func owner(tx *sql.Tx, actor string) bool {
	var value string
	return tx.QueryRow("SELECT owner FROM project").Scan(&value) == nil && value == actor
}
func unresolvedOperation(tx *sql.Tx) bool {
	var exists int
	if tx.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='operation'").Scan(&exists) != nil || exists == 0 {
		return false
	}
	var count int
	return tx.QueryRow("SELECT count(*) FROM operation WHERE state IN ('prepared','running','unknown')").Scan(&count) == nil && count != 0
}
func sourceTransferFrozen(tx *sql.Tx) bool {
	var exists int
	if tx.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='transfer'").Scan(&exists) != nil || exists == 0 {
		return false
	}
	var count int
	return tx.QueryRow("SELECT count(*) FROM transfer WHERE state IN ('exported','acknowledged')").Scan(&count) == nil && count != 0
}
func currentBindings(root string, t task) bool {
	var inputs []string
	if json.Unmarshal([]byte(t.Inputs), &inputs) != nil {
		return false
	}
	fp, err := fingerprints(root, t.Contract, inputs)
	if err != nil || fp != t.BindingFingerprint {
		return false
	}
	command, err := commandFingerprint(root, json.RawMessage(t.Command))
	return err == nil && command == t.CommandFingerprint
}
func loadTask(tx *sql.Tx) (task, error) {
	var t task
	e := tx.QueryRow("SELECT t.id,t.outcome_id,t.state,t.worker,t.reviewer,t.contract_path,t.inputs_json,t.command_json,t.task_revision,t.command_fingerprint,t.input_fingerprints FROM task t JOIN active_task a ON a.task_id=t.id LIMIT 1").Scan(&t.ID, &t.OutcomeID, &t.State, &t.Worker, &t.Reviewer, &t.Contract, &t.Inputs, &t.Command, &t.Revision, &t.CommandFingerprint, &t.BindingFingerprint)
	return t, e
}
func bind(tx *sql.Tx, r Request, rev int64) Response {
	var p struct {
		TaskID, OutcomeID          string
		TaskRevision               int64
		Worker, Reviewer, Contract string
		Inputs                     []string
		Command                    json.RawMessage
	}
	if !owner(tx, r.Actor) {
		return bad("NOT_OWNER", rev)
	}
	if json.Unmarshal(r.Payload, &p) != nil || p.TaskID == "" || p.OutcomeID == "" || p.TaskRevision < 1 || p.Worker == "" || p.Reviewer == "" || p.Worker == p.Reviewer || p.Contract == "" || len(p.Inputs) == 0 || len(p.Command) == 0 {
		return bad("INVALID_BIND", rev)
	}
	var c struct {
		ID, Executable, Cwd         string
		Args                        []string
		TimeoutMs, OutputLimitBytes int
	}
	if json.Unmarshal(p.Command, &c) != nil || c.ID == "" || c.Executable == "" || c.Cwd == "" || c.TimeoutMs < 1 || c.OutputLimitBytes < 1 {
		return bad("INVALID_COMMAND", rev)
	}
	for _, x := range p.Inputs {
		if _, e := rootFile(r.ProjectRoot, x); e != nil {
			return bad("INVALID_INPUT", rev)
		}
	}
	fp, e := fingerprints(r.ProjectRoot, p.Contract, p.Inputs)
	if e != nil {
		return bad("BINDING_UNAVAILABLE", rev)
	}
	commandFP, e := commandFingerprint(r.ProjectRoot, p.Command)
	if e != nil {
		return bad("BINDING_UNAVAILABLE", rev)
	}
	existing, existingErr := loadTask(tx)
	if existingErr == nil {
		var closed int
		if existing.State != "integrated" || tx.QueryRow("SELECT closed FROM resource_revision JOIN resource ON resource.rowid=resource_revision.resource_id WHERE task_id=? AND task_revision=? ORDER BY resource_id DESC LIMIT 1", existing.ID, existing.Revision).Scan(&closed) != nil || closed == 0 {
			return bad("ACTIVE_TASK_EXISTS", rev)
		}
	}
	n, e := next(tx, rev)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	inputs, _ := json.Marshal(p.Inputs)
	_, e = tx.Exec("INSERT INTO task VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", p.TaskID, p.OutcomeID, p.TaskRevision, "draft", p.Worker, p.Reviewer, p.Contract, string(inputs), string(p.Command), fp, fp, commandFP)
	if e != nil {
		return bad("TASK_EXISTS", rev)
	}
	current := task{ID: p.TaskID, OutcomeID: p.OutcomeID, Revision: p.TaskRevision, Contract: p.Contract, Inputs: string(inputs), Command: string(p.Command), BindingFingerprint: fp}
	if e = recordRevision(tx, r.ProjectRoot, current, "initial bind"); e != nil {
		return bad("BINDING_UNAVAILABLE", rev)
	}
	if _, e = tx.Exec("INSERT OR IGNORE INTO outcome_policy(outcome_id) VALUES(?)", p.OutcomeID); e != nil {
		return bad("STATE_BUSY", rev)
	}
	if _, e = tx.Exec("INSERT OR REPLACE INTO active_task(slot,task_id) VALUES(1,?)", p.TaskID); e != nil {
		return bad("STATE_BUSY", rev)
	}
	return good("BOUND", n, map[string]any{})
}
func ready(tx *sql.Tx, r Request, rev int64) Response {
	t, e := loadTask(tx)
	if e != nil || t.State != "draft" {
		return bad("NOT_DRAFT", rev)
	}
	if !owner(tx, r.Actor) {
		return bad("NOT_OWNER", rev)
	}
	if !currentBindings(r.ProjectRoot, t) {
		return bad("BINDING_UNAVAILABLE", rev)
	}
	n, e := next(tx, rev)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	_, e = tx.Exec("UPDATE task SET state='ready' WHERE id=?", t.ID)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	return good("READY", n, map[string]any{})
}
func stop(tx *sql.Tx, r Request, rev int64) Response {
	var p struct {
		Enabled *bool `json:"enabled"`
	}
	if json.Unmarshal(r.Payload, &p) != nil || p.Enabled == nil {
		return bad("INVALID_STOP", rev)
	}
	var owner string
	_ = tx.QueryRow("SELECT owner FROM project").Scan(&owner)
	if r.Actor != owner {
		return bad("NOT_OWNER", rev)
	}
	n, e := next(tx, rev)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	_, e = tx.Exec("UPDATE project SET stopped=?", boolInt(*p.Enabled))
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	return good("STOP_UPDATED", n, map[string]any{})
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
func prepareRun(tx *sql.Tx, r Request, rev int64) Response {
	t, e := loadTask(tx)
	if e != nil || t.State != "ready" {
		return bad("NOT_READY", rev)
	}
	if r.Actor != t.Worker {
		return bad("NOT_WORKER", rev)
	}
	if stopped(tx) {
		return bad("STOPPED", rev)
	}
	var failures, allowance, reassess, review int
	if tx.QueryRow("SELECT failure_count,allowance,reassess_used,review_used FROM outcome_policy WHERE outcome_id=?", t.OutcomeID).Scan(&failures, &allowance, &reassess, &review) == nil && failures >= 2 {
		if allowance == 0 {
			return bad("OUTCOME_BLOCKED", rev)
		}
		if _, e := tx.Exec("UPDATE outcome_policy SET allowance=0,gate=? WHERE outcome_id=?", gateForFailures(failures, reassess, review), t.OutcomeID); e != nil {
			return bad("STATE_BUSY", rev)
		}
	}
	// An exported transfer freezes this source independently from the user-visible STOP flag.
	var frozen int
	if tx.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='transfer'").Scan(&frozen) == nil && frozen != 0 {
		var pending int
		if tx.QueryRow("SELECT count(*) FROM transfer WHERE state='exported'").Scan(&pending) == nil && pending != 0 {
			return bad("TRANSFER_FROZEN", rev)
		}
	}
	if !currentBindings(r.ProjectRoot, t) {
		return bad("BINDING_UNAVAILABLE", rev)
	}
	var inputs []string
	_ = json.Unmarshal([]byte(t.Inputs), &inputs)
	fp, _ := fingerprints(r.ProjectRoot, t.Contract, inputs)
	id := uuid.NewString()
	canonicalRoot, e := filepath.EvalSymlinks(r.ProjectRoot)
	if e != nil {
		return bad("BINDING_UNAVAILABLE", rev)
	}
	var copyID, deviceID string
	if tx.QueryRow("SELECT copy_id,device_id FROM project").Scan(&copyID, &deviceID) != nil {
		return bad("STATE_CORRUPT", rev)
	}
	n, e := next(tx, rev)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	_, e = tx.Exec("INSERT INTO operation(id,task_id,state,output,truncated,before_fingerprint,after_fingerprint,created_at) VALUES(?,?, 'prepared','',0,?,?,?)", id, t.ID, fp, "", time.Now().UTC().Format(time.RFC3339Nano))
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	if _, e = tx.Exec("INSERT INTO operation_revision(operation_id,task_id,task_revision,outcome_id) VALUES(?,?,?,?)", id, t.ID, t.Revision, t.OutcomeID); e != nil {
		return bad("STATE_BUSY", rev)
	}
	contextHash, e := recoveryContext()
	if e != nil || contextHash == "" {
		return bad("NATIVE_CONTEXT_UNAVAILABLE", rev)
	}
	if _, e = tx.Exec("INSERT INTO operation_owner(operation_id,phase,root,copy_id,device_id,context_hash,parent_pid,origin_request_id) VALUES(?,?,?,?,?,?,?,?)", id, "launching", canonicalRoot, copyID, deviceID, contextHash, os.Getpid(), r.RequestID); e != nil {
		return bad("STATE_BUSY", rev)
	}
	_, e = tx.Exec("UPDATE task SET state='in_progress' WHERE id=?", t.ID)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	return good("RUN_PREPARED", n, map[string]any{"operationId": id, "operationState": "prepared"})
}
func executeRun(db *sql.DB, r Request, prepared Response) Response {
	id := prepared.Data["operationId"].(string)
	var command, contract, inputs, commandFP, outcome string
	if e := db.QueryRow("SELECT t.command_json,t.contract_path,t.inputs_json,t.command_fingerprint,t.outcome_id FROM operation o JOIN task t ON t.id=o.task_id WHERE o.id=?", id).Scan(&command, &contract, &inputs, &commandFP, &outcome); e != nil {
		return bad("STATE_CORRUPT", prepared.Revision)
	}
	var c struct {
		Executable, Cwd             string
		Args                        []string
		TimeoutMs, OutputLimitBytes int
	}
	if json.Unmarshal([]byte(command), &c) != nil {
		return bad("STATE_CORRUPT", prepared.Revision)
	}
	if now, err := commandFingerprint(r.ProjectRoot, json.RawMessage(command)); err != nil || now != commandFP {
		return bad("BINDING_CHANGED", prepared.Revision)
	}
	cwd, e := rootFile(r.ProjectRoot, c.Cwd)
	if e != nil {
		return bad("INVALID_COMMAND", prepared.Revision)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.TimeoutMs)*time.Millisecond)
	defer cancel()
	run, err := runOwned(ctx, commandSpec{Executable: c.Executable, Args: c.Args, Cwd: cwd, OutputLimit: c.OutputLimitBytes, Registered: func(pid, group int) error { return registerOperation(db, id, pid, group) }})
	if run.Unresolved {
		// The launching ledger must remain unresolved when bounded cleanup cannot
		// prove that our own process group stopped; do not hide a live command in a
		// terminal result.
		return bad("RECOVERY_REQUIRED", prepared.Revision)
	}
	exit := run.ExitCode
	state := "succeeded"
	if run.TimedOut || run.Truncated || run.Pending {
		state = "incomplete"
	} else if err != nil || exit != 0 {
		state = "failed"
	}
	var in []string
	_ = json.Unmarshal([]byte(inputs), &in)
	after, fe := fingerprints(r.ProjectRoot, contract, in)
	if fe != nil {
		after = "unavailable"
		state = "incomplete"
	}
	tx, e := db.Begin()
	if e != nil {
		return bad("STATE_BUSY", prepared.Revision)
	}
	defer tx.Rollback()
	rev, e := revision(tx)
	if e != nil {
		return bad("STATE_CORRUPT", prepared.Revision)
	}
	result, e := tx.Exec("UPDATE operation SET state=?,exit_code=?,output=?,truncated=?,after_fingerprint=? WHERE id=? AND state IN ('prepared','running')", state, exit, run.Output, boolInt(run.Truncated), after, id)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return bad("RECOVERED_OPERATION", rev)
	}
	if state == "failed" || state == "incomplete" {
		var failures, reassess, review int
		if tx.QueryRow("SELECT failure_count,reassess_used,review_used FROM outcome_policy WHERE outcome_id=?", outcome).Scan(&failures, &reassess, &review) == nil {
			failures++
			if _, e = tx.Exec("UPDATE outcome_policy SET failure_count=?,gate=? WHERE outcome_id=?", failures, gateForFailures(failures, reassess, review), outcome); e != nil {
				return bad("STATE_BUSY", rev)
			}
		}
	}
	n, e := next(tx, rev)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	out := good("RUN_RECORDED", n, map[string]any{"operationId": id, "operationState": state, "exitCode": exit})
	b, _ := json.Marshal(out)
	if _, e = tx.Exec("UPDATE request_log SET response_json=? WHERE id=?", string(b), r.RequestID); e != nil {
		return bad("STATE_BUSY", rev)
	}
	if e = tx.Commit(); e != nil {
		return bad("STATE_BUSY", rev)
	}
	return out
}

type limitedBuffer struct {
	bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		b.Buffer.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	return b.Buffer.Write(p)
}
func submit(tx *sql.Tx, r Request, rev int64) Response {
	t, e := loadTask(tx)
	if e != nil || t.State != "in_progress" {
		return bad("NOT_IN_PROGRESS", rev)
	}
	if r.Actor != t.Worker {
		return bad("NOT_WORKER", rev)
	}
	if stopped(tx) {
		return bad("STOPPED", rev)
	}
	if !currentBindings(r.ProjectRoot, t) {
		return bad("EVIDENCE_INVALID", rev)
	}
	var state string
	var truncated int
	var before, after string
	e = tx.QueryRow("SELECT o.state,o.truncated,o.before_fingerprint,o.after_fingerprint FROM operation o JOIN operation_revision m ON m.operation_id=o.id WHERE m.task_id=? AND m.task_revision=? ORDER BY o.created_at DESC LIMIT 1", t.ID, t.Revision).Scan(&state, &truncated, &before, &after)
	if e != nil || state != "succeeded" || truncated != 0 || before != after {
		return bad("EVIDENCE_INVALID", rev)
	}
	n, e := next(tx, rev)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	_, e = tx.Exec("UPDATE task SET state='review' WHERE id=?", t.ID)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	return good("SUBMITTED", n, map[string]any{})
}
func review(tx *sql.Tx, r Request, rev int64) Response {
	t, e := loadTask(tx)
	if e != nil || t.State != "review" {
		return bad("NOT_REVIEW", rev)
	}
	if r.Actor != t.Reviewer {
		return bad("NOT_REVIEWER", rev)
	}
	if stopped(tx) {
		return bad("STOPPED", rev)
	}
	if !currentBindings(r.ProjectRoot, t) {
		return bad("EVIDENCE_INVALID", rev)
	}
	var p struct {
		Accept bool
		Reason string
	}
	if json.Unmarshal(r.Payload, &p) != nil || p.Reason == "" {
		return bad("INVALID_REVIEW", rev)
	}
	n, e := next(tx, rev)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	state := "draft"
	code := "REWORK"
	if p.Accept {
		state = "accepted"
		code = "ACCEPTED"
	}
	result, e := tx.Exec("INSERT INTO review_record(reason,accepted) VALUES(?,?)", p.Reason, boolInt(p.Accept))
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	if id, e := result.LastInsertId(); e != nil {
		return bad("STATE_BUSY", rev)
	} else if _, e = tx.Exec("INSERT INTO review_revision(review_id,task_id,task_revision,outcome_id) VALUES(?,?,?,?)", id, t.ID, t.Revision, t.OutcomeID); e != nil {
		return bad("STATE_BUSY", rev)
	}
	if p.Accept {
		if _, e = tx.Exec("UPDATE outcome_policy SET failure_count=0,reassess_used=0,review_used=0,allowance=0,gate='open' WHERE outcome_id=?", t.OutcomeID); e != nil {
			return bad("STATE_BUSY", rev)
		}
	} else {
		if _, e = tx.Exec("UPDATE outcome_policy SET failure_count=failure_count+1,allowance=0,gate='open' WHERE outcome_id=?", t.OutcomeID); e != nil {
			return bad("STATE_BUSY", rev)
		}
		var failures, reassess, reviewed int
		if e = tx.QueryRow("SELECT failure_count,reassess_used,review_used FROM outcome_policy WHERE outcome_id=?", t.OutcomeID).Scan(&failures, &reassess, &reviewed); e != nil || failures < 1 {
			return bad("STATE_BUSY", rev)
		}
		if _, e = tx.Exec("UPDATE outcome_policy SET gate=? WHERE outcome_id=?", gateForFailures(failures, reassess, reviewed), t.OutcomeID); e != nil {
			return bad("STATE_BUSY", rev)
		}
	}
	_, e = tx.Exec("UPDATE task SET state=? WHERE id=?", state, t.ID)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	return good(code, n, map[string]any{})
}
func integrate(tx *sql.Tx, r Request, rev int64) Response {
	t, e := loadTask(tx)
	if e != nil || t.State != "accepted" {
		return bad("NOT_ACCEPTED", rev)
	}
	var owner string
	_ = tx.QueryRow("SELECT owner FROM project").Scan(&owner)
	if r.Actor != owner {
		return bad("NOT_OWNER", rev)
	}
	if stopped(tx) {
		return bad("STOPPED", rev)
	}
	if !currentBindings(r.ProjectRoot, t) {
		return bad("EVIDENCE_INVALID", rev)
	}
	var p struct{ Candidate, Reason string }
	if json.Unmarshal(r.Payload, &p) != nil || p.Candidate == "" || p.Reason == "" {
		return bad("INVALID_INTEGRATION", rev)
	}
	n, e := next(tx, rev)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	result, e := tx.Exec("INSERT INTO integration_record(candidate,reason) VALUES(?,?)", p.Candidate, p.Reason)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	if id, e := result.LastInsertId(); e != nil {
		return bad("STATE_BUSY", rev)
	} else if _, e = tx.Exec("INSERT INTO integration_revision(integration_id,task_id,task_revision,outcome_id) VALUES(?,?,?,?)", id, t.ID, t.Revision, t.OutcomeID); e != nil {
		return bad("STATE_BUSY", rev)
	}
	_, e = tx.Exec("UPDATE task SET state='integrated' WHERE id=?", t.ID)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	return good("INTEGRATED", n, map[string]any{})
}
func closeResource(tx *sql.Tx, r Request, rev int64) Response {
	t, e := loadTask(tx)
	if e != nil || t.State != "integrated" {
		return bad("NOT_INTEGRATED", rev)
	}
	var owner string
	_ = tx.QueryRow("SELECT owner FROM project").Scan(&owner)
	var p struct{ Disposition, Reason string }
	if r.Actor != owner {
		return bad("NOT_OWNER", rev)
	}
	if stopped(tx) {
		return bad("STOPPED", rev)
	}
	if json.Unmarshal(r.Payload, &p) != nil || p.Reason == "" || (p.Disposition != "keep" && p.Disposition != "remove" && p.Disposition != "transfer") {
		return bad("INVALID_RESOURCE_CLOSE", rev)
	}
	n, e := next(tx, rev)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	result, e := tx.Exec("INSERT INTO resource(disposition,reason,closed) VALUES(?,?,1)", p.Disposition, p.Reason)
	if e != nil {
		return bad("STATE_BUSY", rev)
	}
	if id, e := result.LastInsertId(); e != nil {
		return bad("STATE_BUSY", rev)
	} else if _, e = tx.Exec("INSERT INTO resource_revision(resource_id,task_id,task_revision,outcome_id) VALUES(?,?,?,?)", id, t.ID, t.Revision, t.OutcomeID); e != nil {
		return bad("STATE_BUSY", rev)
	}
	return good("RESOURCE_CLOSED", n, map[string]any{})
}
func status(r Request) Response {
	db, e := openExisting(r.ProjectRoot)
	if e != nil {
		return bad("STATE_UNAVAILABLE", 0)
	}
	defer db.Close()
	version, e := databaseVersion(db)
	if e != nil {
		return bad("STATE_CORRUPT", 0)
	}
	if version == 1 {
		var rev int64
		_ = db.QueryRow("SELECT revision FROM project").Scan(&rev)
		return Response{SchemaVersion: schemaVersion, Code: "UPGRADE_REQUIRED", Revision: rev, Data: map[string]any{"nextAction": "state.upgrade"}}
	}
	var rev int64
	var projectID string
	var stoppedValue int
	var t task
	e = db.QueryRow("SELECT id,revision,stopped FROM project").Scan(&projectID, &rev, &stoppedValue)
	if e != nil {
		return bad("STATE_CORRUPT", 0)
	}
	e = db.QueryRow("SELECT t.id,t.outcome_id,t.state,t.worker,t.reviewer,t.contract_path,t.inputs_json,t.command_json,t.task_revision,t.command_fingerprint,t.input_fingerprints FROM task t JOIN active_task a ON a.task_id=t.id LIMIT 1").Scan(&t.ID, &t.OutcomeID, &t.State, &t.Worker, &t.Reviewer, &t.Contract, &t.Inputs, &t.Command, &t.Revision, &t.CommandFingerprint, &t.BindingFingerprint)
	if e != nil {
		return bad("TASK_UNBOUND", rev)
	}
	var id, state sql.NullString
	var exit sql.NullInt64
	_ = db.QueryRow("SELECT o.id,o.state,o.exit_code FROM operation o LEFT JOIN operation_revision m ON m.operation_id=o.id WHERE o.task_id=? AND ((m.task_id=? AND m.task_revision=?) OR m.operation_id IS NULL) ORDER BY o.created_at DESC LIMIT 1", t.ID, t.ID, t.Revision).Scan(&id, &state, &exit)
	cleanup := "pending"
	var closed int
	_ = db.QueryRow("SELECT r.closed FROM resource r JOIN resource_revision m ON m.resource_id=r.rowid WHERE m.task_id=? AND m.task_revision=? ORDER BY r.rowid DESC LIMIT 1", t.ID, t.Revision).Scan(&closed)
	if t.State == "integrated" && closed == 1 {
		cleanup = "done"
	}
	nextAction := "inspect"
	if id.Valid && (state.String == "prepared" || state.String == "running" || state.String == "unknown") {
		nextAction = "operation.inspect"
	} else if stoppedValue != 0 {
		nextAction = "clear_stop_or_cancel"
	} else if t.State == "ready" {
		nextAction = "run"
	} else if t.State == "in_progress" {
		nextAction = "submit_or_reconcile"
	} else if t.State == "review" {
		nextAction = "review"
	}
	data := map[string]any{"projectId": projectID, "taskId": t.ID, "outcomeId": t.OutcomeID, "taskRevision": t.Revision, "stopped": stoppedValue != 0, "nextAction": nextAction, "taskState": t.State, "cleanup": cleanup}
	var failures int
	var gate string
	if db.QueryRow("SELECT failure_count,gate FROM outcome_policy WHERE outcome_id=?", t.OutcomeID).Scan(&failures, &gate) == nil {
		data["failureCount"] = failures
		data["outcomeGate"] = gate
		if t.State == "ready" && failures >= 2 {
			if gate == "reassess_required" {
				nextAction = "outcome.reassess"
			} else if gate == "outcome_review_required" {
				nextAction = "outcome.review"
			} else if gate == "owner_decision_required" {
				nextAction = "outcome.decide"
			}
			data["nextAction"] = nextAction
		}
	}
	if id.Valid {
		if state.String == "prepared" || state.String == "running" {
			state.String = "unknown"
			data["execution"] = "unknown/unverified"
		}
		data["operationId"] = id.String
		data["operationState"] = state.String
		if exit.Valid {
			data["exitCode"] = exit.Int64
		}
	}
	data["nextAction"] = nextAction
	return good("STATUS", rev, data)
}

var _ io.Writer = (*limitedBuffer)(nil)
