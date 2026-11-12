package internal

import (
	"encoding/json"
	"os"
	"testing"
)

func TestPortableTransferPreservesHistoryAndAcknowledgedSourceStaysFrozen(t *testing.T) {
	source, revision := setup(t, "0", 5000, 4096)
	run := request(source, "run", "worker", revision, map[string]any{})
	if !run.OK {
		t.Fatal(run)
	}
	submitted := request(source, "submit", "worker", run.Revision, map[string]any{})
	if !submitted.OK {
		t.Fatal(submitted)
	}
	reviewed := request(source, "review", "main", submitted.Revision, map[string]any{"accept": true, "reason": "synthetic review"})
	if !reviewed.OK {
		t.Fatal(reviewed)
	}
	integrated := request(source, "integrate", "main", reviewed.Revision, map[string]any{"candidate": "synthetic", "reason": "synthetic integration"})
	if !integrated.OK {
		t.Fatal(integrated)
	}
	closed := request(source, "resource.close", "main", integrated.Revision, map[string]any{"disposition": "keep", "reason": "synthetic evidence"})
	if !closed.OK {
		t.Fatal(closed)
	}
	if missing := request(source, "handover.export", "main", closed.Revision, map[string]any{"transferId": "missing-input", "files": []string{"contract.md"}}); missing.OK || missing.Code != "INVALID_EXPORT_PATH" {
		t.Fatalf("missing selected input=%+v", missing)
	}
	exported := request(source, "handover.export", "main", closed.Revision, map[string]any{"transferId": "full-history", "files": []string{"contract.md", "input.txt"}})
	if !exported.OK {
		t.Fatal(exported)
	}
	if response := request(source, "stop", "main", exported.Revision, map[string]any{"enabled": false}); response.OK || response.Code != "SOURCE_TRANSFERRED" {
		t.Fatalf("exported source stop=%+v", response)
	}
	packageRaw, err := json.Marshal(exported.Data["package"])
	if err != nil {
		t.Fatal(err)
	}
	var pkg portablePackage
	if err = json.Unmarshal(packageRaw, &pkg); err != nil {
		t.Fatal(err)
	}
	if pkg.OriginalTaskState != "integrated" || len(pkg.Operations) == 0 || len(pkg.Reviews) != 1 || len(pkg.Integrations) != 1 || len(pkg.Resources) != 1 || !pkg.OutputOmitted || pkg.Operations[0].Output != "" {
		t.Fatalf("incomplete exported history=%+v", pkg)
	}
	destination := t.TempDir()
	imported := request(destination, "handover.import", "main", 0, map[string]any{"package": json.RawMessage(packageRaw), "deviceId": "history-device", "copyId": "history-copy"})
	if !imported.OK {
		t.Fatal(imported)
	}
	db, err := openExisting(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, query := range []string{"SELECT count(*) FROM operation", "SELECT count(*) FROM review_record", "SELECT count(*) FROM integration_record", "SELECT count(*) FROM resource"} {
		var count int
		if err = db.QueryRow(query).Scan(&count); err != nil || count != 1 {
			t.Fatalf("restored history %q count=%d err=%v", query, count, err)
		}
	}
	acknowledged := request(source, "handover.ack", "main", exported.Revision, map[string]any{"transferId": "full-history", "generation": exported.Revision})
	if !acknowledged.OK {
		t.Fatal(acknowledged)
	}
	if response := request(source, "stop", "main", acknowledged.Revision, map[string]any{"enabled": false}); response.OK || response.Code != "SOURCE_TRANSFERRED" {
		t.Fatalf("acknowledged source stop=%+v", response)
	}
	if response := request(source, "handover.rebind", "main", acknowledged.Revision, map[string]any{"command": pkg.Task.Command}); response.OK || response.Code != "SOURCE_TRANSFERRED" {
		t.Fatalf("acknowledged source rebind=%+v", response)
	}
}

func TestPortableUnknownOperationStaysBlocked(t *testing.T) {
	root, revision := setup(t, "0", 5000, 4096)
	db, err := openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO operation(id,task_id,state,output,truncated,before_fingerprint,after_fingerprint,created_at) VALUES('lost','t','prepared','',0,'x','','2099-01-01')")
	if err == nil {
		_, err = db.Exec("UPDATE task SET state='in_progress'")
	}
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response := request(root, "operation.reconcile", "main", revision, map[string]any{"operationId": "lost", "disposition": "abandon", "reason": "reason only"}); response.OK || response.Code != "RECOVERY_UNSUPPORTED" {
		t.Fatalf("reason-only reconcile=%+v", response)
	}
	if response := request(root, "handover.export", "main", revision, map[string]any{"transferId": "unsafe", "files": []string{"contract.md", "input.txt"}}); response.OK || response.Code != "RECOVERY_REQUIRED" {
		t.Fatalf("unsafe export=%+v", response)
	}
	if response := request(root, "handover.rebind", "main", revision, map[string]any{"command": json.RawMessage(`{"id":"new"}`)}); response.OK || response.Code != "RECOVERY_REQUIRED" {
		t.Fatalf("unsafe rebind=%+v", response)
	}
	db, err = openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var state string
	if err = db.QueryRow("SELECT state FROM operation WHERE id='lost'").Scan(&state); err != nil || state != "prepared" {
		t.Fatalf("unknown operation changed state=%q err=%v", state, err)
	}
}

func TestPortableNamesRejectPrivateAndNonportableEntries(t *testing.T) {
	for _, name := range []string{"../escape", `nested\\escape`, "name:stream", ".ocmi/state", ".git/config", ".env", "store.sqlite", "state-wal", "private.key", "CON", "folder/file. ", ""} {
		if portableName(name) {
			t.Fatalf("accepted nonportable name %q", name)
		}
	}
	if !portableName("nested/ordinary-file.txt") {
		t.Fatal("rejected ordinary nested path")
	}
	root := t.TempDir()
	if err := os.MkdirAll(root+"/nested", 0700); err != nil {
		t.Fatal(err)
	}
}
