package internal

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func insertRecoveryOwner(t *testing.T, root, id, state, phase string) *sql.DB {
	t.Helper()
	db, err := openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	contextHash, err := recoveryContext()
	if err != nil {
		db.Close()
		t.Skipf("native recovery context unavailable: %v", err)
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO operation(id,task_id,state,output,truncated,before_fingerprint,after_fingerprint,created_at) VALUES(?,?,?,'',0,'x','','now')", id, "t", state); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO operation_revision(operation_id,task_id,task_revision,outcome_id) VALUES(?,?,?,?)", id, "t", 1, "o"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO operation_owner(operation_id,phase,root,copy_id,device_id,context_hash,parent_pid,origin_request_id) VALUES(?,?,?,?,?,?,?,?)", id, phase, canonical, "c", "d", contextHash, os.Getpid(), "original-run"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	return db
}

func TestRecoveryRegistrationRequiresOneLaunchingRow(t *testing.T) {
	root, _ := setup(t, "0", 5000, 4096)
	db := insertRecoveryOwner(t, root, "registration", "prepared", "launching")
	defer db.Close()
	if err := registerOperation(db, "registration", 12345, 12345); err != nil {
		t.Fatalf("first registration: %v", err)
	}
	if err := registerOperation(db, "registration", 12345, 12345); err == nil {
		t.Fatal("second registration changed a non-launching record")
	}
}

func TestRecoveryTerminalAndLateCompletionStayFailClosed(t *testing.T) {
	root, rev := setup(t, "sleep", 5000, 4096)
	operationID := uuid.NewString()
	db := insertRecoveryOwner(t, root, operationID, "prepared", "launching")
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	task, err := loadTask(tx)
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = automaticRunResources(tx, Request{ProjectRoot: root, Actor: "worker", RequestID: "original-run"}, task, operationID, "c", "d"); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	done := make(chan Response, 1)
	go func() {
		done <- executeRun(db, Request{ProjectRoot: root, RequestID: "original-run"}, good("RUN_PREPARED", rev, map[string]any{"operationId": operationID}))
	}()
	// Interleave a recovered terminal decision while the real runner is still
	// executing. executeRun must retain it rather than write a late success.
	time.Sleep(20 * time.Millisecond)
	if _, err := db.Exec("UPDATE operation SET state='incomplete',exit_code=NULL,after_fingerprint='unavailable' WHERE id=?", operationID); err != nil {
		t.Fatal(err)
	}
	response := <-done
	if response.Code != "RECOVERED_OPERATION" {
		t.Fatalf("late completion response=%+v", response)
	}
	var state string
	var exit sql.NullInt64
	if err := db.QueryRow("SELECT state,exit_code FROM operation WHERE id=?", operationID).Scan(&state, &exit); err != nil || state != "incomplete" || exit.Valid {
		t.Fatalf("late completion overwrote recovered terminal: state=%q exit=%+v err=%v", state, exit, err)
	}
}

func TestRecoveryMalformedNativeOwnershipIsUnknown(t *testing.T) {
	root, _ := setup(t, "0", 5000, 4096)
	db := insertRecoveryOwner(t, root, "malformed-owner", "prepared", "registered")
	defer db.Close()
	if _, err := db.Exec("UPDATE operation_owner SET child_pid=0,process_group=0 WHERE operation_id=?", "malformed-owner"); err != nil {
		t.Fatal(err)
	}
	response := inspectOperation(Request{SchemaVersion: 1, Action: "operation.inspect", ProjectRoot: root, Actor: "main", RequestID: "inspect-malformed", Payload: []byte(`{"operationId":"malformed-owner"}`)})
	if !response.OK {
		t.Fatal(response)
	}
	ownership := response.Data["ownership"].(map[string]any)
	if ownership["observation"] != "unknown" || ownership["canReconcile"] != false {
		t.Fatalf("malformed native ownership=%+v", ownership)
	}
}

func TestStatusDirectsUnresolvedExecutionToInspection(t *testing.T) {
	root, _ := setup(t, "0", 5000, 4096)
	db := insertRecoveryOwner(t, root, "unresolved-status", "prepared", "launching")
	if _, err := db.Exec("UPDATE task SET state='in_progress' WHERE id='t'"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	response := request(root, "status", "main", 0, map[string]any{})
	if !response.OK || response.Data["operationState"] != "unknown" || response.Data["execution"] != "unknown/unverified" || response.Data["nextAction"] != "operation.inspect" {
		t.Fatalf("unresolved status=%+v", response)
	}
}
