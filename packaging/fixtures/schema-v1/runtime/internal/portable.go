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
	"strings"
	"time"
)

const portableSchema = 1
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
}
type portableOperation struct {
	ID, TaskID, State, Output, BeforeFingerprint, AfterFingerprint, CreatedAt string
	ExitCode                                                                  *int `json:"exitCode,omitempty"`
	Truncated                                                                 bool
}
type portableReview struct {
	Reason   string
	Accepted bool
}
type portableIntegration struct{ Candidate, Reason string }
type portableResource struct {
	Disposition, Reason string
	Closed              bool
}
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
		_, err = tx.Exec("INSERT INTO portable_meta VALUES(1)")
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
		var state string
		if tx.QueryRow("SELECT state FROM operation WHERE id=?", p.OperationID).Scan(&state) != nil || (state != "prepared" && state != "running" && state != "unknown") {
			return bad("NOT_UNKNOWN", rev)
		}
		return bad("RECOVERY_UNSUPPORTED", rev)
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
		if tx.QueryRow("SELECT id,outcome_id,worker,reviewer,contract_path,inputs_json,command_json,task_revision FROM task LIMIT 1").Scan(&pt.ID, &pt.OutcomeID, &pt.Worker, &pt.Reviewer, &pt.Contract, &inputs, &command, &pt.Revision) != nil || json.Unmarshal([]byte(inputs), &pt.Inputs) != nil {
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
		rows, _ := tx.Query("SELECT id,task_id,state,exit_code,output,truncated,before_fingerprint,after_fingerprint,created_at FROM operation")
		for rows != nil && rows.Next() {
			var o portableOperation
			var exit sql.NullInt64
			var trunc int
			_ = rows.Scan(&o.ID, &o.TaskID, &o.State, &exit, &o.Output, &trunc, &o.BeforeFingerprint, &o.AfterFingerprint, &o.CreatedAt)
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
		rv, _ := tx.Query("SELECT reason,accepted FROM review_record")
		for rv != nil && rv.Next() {
			var x portableReview
			var a int
			_ = rv.Scan(&x.Reason, &a)
			x.Accepted = a != 0
			pkg.Reviews = append(pkg.Reviews, x)
		}
		if rv != nil {
			rv.Close()
		}
		iv, _ := tx.Query("SELECT candidate,reason FROM integration_record")
		for iv != nil && iv.Next() {
			var x portableIntegration
			_ = iv.Scan(&x.Candidate, &x.Reason)
			pkg.Integrations = append(pkg.Integrations, x)
		}
		if iv != nil {
			iv.Close()
		}
		rsrc, _ := tx.Query("SELECT disposition,reason,closed FROM resource")
		for rsrc != nil && rsrc.Next() {
			var x portableResource
			var closed int
			_ = rsrc.Scan(&x.Disposition, &x.Reason, &closed)
			x.Closed = closed != 0
			pkg.Resources = append(pkg.Resources, x)
		}
		if rsrc != nil {
			rsrc.Close()
		}
		pkg.SchemaVersion, pkg.System, pkg.TransferID, pkg.Generation = 1, "ocmi-runtime-preview", p.TransferID, rev+1
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
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("unsafe root")
	}
	path := filepath.Join(root, filepath.FromSlash(name))
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("unsafe selected path")
		}
		if current == root {
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
	inputs, _ := json.Marshal(p.Package.Task.Inputs) // Imported bindings remain quarantined until explicit rebind/ready.
	if _, err = tx.Exec("INSERT INTO task VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", p.Package.Task.ID, p.Package.Task.OutcomeID, p.Package.Task.Revision, "draft", p.Package.Task.Worker, p.Package.Task.Reviewer, p.Package.Task.Contract, string(inputs), string(p.Package.Task.Command), "stale-import", "stale-import", "stale-import"); err != nil {
		return err
	}
	for _, operation := range p.Package.Operations {
		if _, err = tx.Exec("INSERT INTO operation(id,task_id,state,exit_code,output,truncated,before_fingerprint,after_fingerprint,created_at) VALUES(?,?,?,?,?,?,?,?,?)", operation.ID, p.Package.Task.ID, operation.State, operation.ExitCode, "", boolInt(operation.Truncated), operation.BeforeFingerprint, operation.AfterFingerprint, operation.CreatedAt); err != nil {
			return err
		}
	}
	for _, review := range p.Package.Reviews {
		if _, err = tx.Exec("INSERT INTO review_record(reason,accepted) VALUES(?,?)", review.Reason, boolInt(review.Accepted)); err != nil {
			return err
		}
	}
	for _, integration := range p.Package.Integrations {
		if _, err = tx.Exec("INSERT INTO integration_record(candidate,reason) VALUES(?,?)", integration.Candidate, integration.Reason); err != nil {
			return err
		}
	}
	for _, resource := range p.Package.Resources {
		if _, err = tx.Exec("INSERT INTO resource(disposition,reason,closed) VALUES(?,?,?)", resource.Disposition, resource.Reason, boolInt(resource.Closed)); err != nil {
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
	if len(r.Payload) > maxPortablePackageBytes || json.Unmarshal(r.Payload, &p) != nil || p.Package.SchemaVersion != 1 || p.Package.System != "ocmi-runtime-preview" || p.Package.TransferID == "" || p.Package.Generation < 1 || p.Package.Source.ProjectID == "" || len(p.Package.Files) == 0 || len(p.Package.Files) > maxPortableFiles || p.DeviceID == "" || p.CopyID == "" || p.Package.Task.ID == "" || p.Package.Task.OutcomeID == "" || p.Package.Task.Worker == "" || p.Package.Task.Reviewer == "" || p.Package.Task.Contract == "" || len(p.Package.Task.Inputs) == 0 || len(p.Package.Task.Command) == 0 {
		return bad("INVALID_PACKAGE", 0)
	}
	if p.Package.Digest != packageDigest(p.Package) {
		return bad("PACKAGE_CHECKSUM", 0)
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
