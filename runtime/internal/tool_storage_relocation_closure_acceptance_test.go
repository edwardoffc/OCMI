package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A successful implementation remains accepted, but its unknown shared-use
// reference must prevent resource.close from claiming complete cleanup.
func TestMainToolAuthoringRelocationCleanupPendingAtTaskClose(t *testing.T) {
	f := mainRelocationSetup(t)
	uses := mainRelocationPair(t, f)
	db, err := openExisting(uses[0].root)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	err = db.QueryRow("SELECT o.id FROM operation o JOIN operation_owner p ON p.operation_id=o.id WHERE p.phase='registered' AND o.state IN ('prepared','running')").Scan(&id)
	db.Close()
	if err != nil {
		t.Fatal("qualified registered operation", err)
	}
	path := filepath.Join(f.home, "local", "bindings", "tool-storage", "uses", "go", id+".json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.WriteFile(path, original, 0600); err != nil {
			t.Error("restore owned fixture proof", err)
		}
	}()
	var record map[string]any
	if json.Unmarshal(original, &record) != nil {
		t.Fatal("qualified use record")
	}
	record["taskId"] = "unknown-task-context"
	raw, _ := json.Marshal(record)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uses[0].root, "release"), []byte("close own child"), 0600); err != nil {
		t.Fatal(err)
	}
	var run Response
	select {
	case run = <-uses[0].result:
		uses[0].done = true
	case <-time.After(40 * time.Second):
		t.Fatal("owned child did not close")
	}
	mainRelocationFinish(t, uses[1])
	if !run.OK || run.Data["operationState"] != "succeeded" {
		t.Fatal("cleanup uncertainty invalidated accepted command behavior", run)
	}
	if _, err := os.Lstat(path); err != nil {
		t.Fatal("uncertain reference disappeared", err)
	}
	root := uses[0].root
	next := request(root, "submit", "worker", run.Revision, map[string]any{})
	if !next.OK {
		t.Fatal("valid implementation submission", next)
	}
	next = request(root, "review", "main", next.Revision, map[string]any{"accept": true, "reason": "Verified read-only behavior; cleanup tracked separately."})
	if !next.OK {
		t.Fatal("implementation acceptance", next)
	}
	next = request(root, "integrate", "main", next.Revision, map[string]any{"candidate": "fictional-read-only-component", "reason": "Accepted implementation; retained use remains unknown."})
	if !next.OK {
		t.Fatal("implementation integration", next)
	}
	closed := request(root, "resource.close", "main", next.Revision, map[string]any{"disposition": "keep", "reason": "Preserve source; complete only verified cleanup."})
	if closed.OK {
		t.Fatal("task claimed resource cleanup despite unknown shared reference", closed)
	}
	if closed.Code != "RESOURCE_PENDING" {
		t.Fatal("cleanup refusal is not actionable pending state", closed)
	}
	if _, err := os.Lstat(path); err != nil {
		t.Fatal("cleanup refusal altered unknown reference", err)
	}
}
