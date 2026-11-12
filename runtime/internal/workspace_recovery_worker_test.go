package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
)

func TestWorkerWorkspaceRecoveryRejectsLinks(t *testing.T) {
	root := t.TempDir()
	op := "8c43d039-8893-4b53-87f7-a90d29b2f91a"
	if e := os.MkdirAll(filepath.Join(root, ".ocmi", "local", "runs"), 0700); e != nil {
		t.Fatal(e)
	}
	locator := ".ocmi/local/runs/" + op
	proof, e := createLocalResource(root, locator, "directory", "ocmi-"+op, nil, true)
	if e != nil {
		t.Fatal(e)
	}
	outside := t.TempDir()
	if e = os.WriteFile(filepath.Join(outside, "foreign"), []byte("foreign"), 0600); e != nil {
		t.Fatal(e)
	}
	link := filepath.Join(root, filepath.FromSlash(locator), "link")
	if runtime.GOOS == "windows" {
		out, e := exec.Command("cmd.exe", "/c", "mklink", "/J", link, outside).CombinedOutput()
		if e != nil {
			t.Fatalf("junction premise: %v %s", e, out)
		}
	} else if e = os.Symlink(outside, link); e != nil {
		t.Fatal(e)
	}
	if _, e = captureLocalResource(root, locator, "directory", proof.NativeIdentity, true); e == nil {
		t.Fatal("linked inventory accepted")
	}
}

func TestWorkerWorkspaceRecoveryQualification(t *testing.T) {
	root, rev := setup(t, "0", 5000, 4096)
	op := uuid.NewString()
	db := insertRecoveryOwner(t, root, op, "prepared", "launching")
	defer db.Close()
	exitPID := func() int {
		c := exec.Command(os.Args[0], "-test.run=^$")
		if e := c.Run(); e != nil {
			t.Fatal(e)
		}
		return c.Process.Pid
	}
	parent, child := exitPID(), exitPID()
	tx, e := db.Begin()
	if e != nil {
		t.Fatal(e)
	}
	task, e := loadTask(tx)
	if e != nil {
		t.Fatal(e)
	}
	if e = automaticRunResources(tx, Request{ProjectRoot: root, Actor: "worker", RequestID: "original-run"}, task, op, "c", "d"); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	locator := ".ocmi/local/runs/" + op
	if e = os.MkdirAll(filepath.Join(root, ".ocmi", "local", "runs"), 0700); e != nil {
		t.Fatal(e)
	}
	proof, e := createLocalResource(root, locator, "directory", "ocmi-"+op, nil, true)
	if e != nil {
		t.Fatal(e)
	}
	id := "ocmi-workspace-" + op
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{"UPDATE operation SET state='incomplete' WHERE id=?", []any{op}},
		{"UPDATE operation_owner SET phase='registered',parent_pid=?,child_pid=?,process_group=?,job_attached=1,recovery_reason='qualified fixture' WHERE operation_id=?", []any{parent, child, child, op}},
		{"UPDATE resource_ledger SET observed_state='active',native_identity=?,inventory=?,effect_issued=1 WHERE id=?", []any{proof.NativeIdentity, proof.Inventory, id}},
		{"UPDATE resource_ledger SET observed_state='stopped' WHERE id=?", []any{"ocmi-process-" + op}},
		{"UPDATE resource_use SET active=0", nil},
	} {
		if _, e = db.Exec(stmt.sql, stmt.args...); e != nil {
			t.Fatal(e)
		}
	}
	req := Request{ProjectRoot: root, Actor: "main"}
	initialSnapshot, ok := workspaceRecoverySnapshot(db, req, id)
	if !ok {
		t.Fatal("qualified synthetic positive not established")
	}
	var originalContext string
	if e = db.QueryRow("SELECT context_hash FROM operation_owner WHERE operation_id=?", op).Scan(&originalContext); e != nil {
		t.Fatal(e)
	}
	for _, change := range []struct{ name, sql, restore string }{
		{"unresolved-intent", "UPDATE resource_ledger SET observed_state='intent',effect_issued=1 WHERE id=?", "UPDATE resource_ledger SET observed_state='active',effect_issued=1 WHERE id=?"},
		{"unknown-effect", "UPDATE resource_ledger SET observed_state='unknown',effect_issued=1 WHERE id=?", "UPDATE resource_ledger SET observed_state='active',effect_issued=1 WHERE id=?"},
		{"applying", "UPDATE resource_ledger SET applying=1 WHERE id=?", "UPDATE resource_ledger SET applying=0 WHERE id=?"},
		{"hold", "UPDATE resource_ledger SET held=1 WHERE id=?", "UPDATE resource_ledger SET held=0 WHERE id=?"},
		{"imported", "UPDATE resource_ledger SET imported=1 WHERE id=?", "UPDATE resource_ledger SET imported=0 WHERE id=?"},
		{"wrong-context", "UPDATE operation_owner SET context_hash='foreign' WHERE operation_id=?", ""},
	} {
		t.Run(change.name, func(t *testing.T) {
			target := id
			if change.name == "wrong-context" {
				target = op
			}
			if _, e = db.Exec(change.sql, target); e != nil {
				t.Fatal(e)
			}
			if _, ok := workspaceRecoverySnapshot(db, req, id); ok {
				t.Fatal("unsafe snapshot accepted")
			}
			if change.restore != "" {
				if _, e = db.Exec(change.restore, target); e != nil {
					t.Fatal(e)
				}
			} else if change.name == "wrong-context" {
				if _, e = db.Exec("UPDATE operation_owner SET context_hash=? WHERE operation_id=?", originalContext, op); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
	// Replace only the enclosing root after review while retaining the original
	// DB and workspace native identities. The prior approval must become stale.
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	oldRoot := root + ".old"
	if e = os.Rename(root, oldRoot); e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(filepath.Join(root, ".ocmi", "local", "runs"), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(filepath.Join(oldRoot, ".ocmi", "local", "state.sqlite"), filepath.Join(root, ".ocmi", "local", "state.sqlite")); e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(filepath.Join(oldRoot, filepath.FromSlash(locator)), filepath.Join(root, filepath.FromSlash(locator))); e != nil {
		t.Fatal(e)
	}
	reopened, e := openExisting(root)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	newSnapshot, ok := workspaceRecoverySnapshot(reopened, req, id)
	if !ok || newSnapshot.digest == initialSnapshot.digest {
		t.Fatal("replacement root did not invalidate reviewed snapshot", ok)
	}
	r := request(root, "resource.reconcile", "main", rev, map[string]any{"resourceId": id, "reason": "reviewed old snapshot", "inventorySHA256": initialSnapshot.digest})
	if r.OK || r.Code != "RESOURCE_PENDING" {
		t.Fatal("replaced root did not refuse stale approval", r)
	}
	r = request(root, "resource.reconcile", "main", rev, map[string]any{"resourceId": id, "reason": "reviewed current snapshot", "inventorySHA256": newSnapshot.digest})
	if !r.OK {
		t.Fatal("current snapshot refused", r)
	}
	var issued int
	if e = reopened.QueryRow("SELECT effect_issued FROM resource_ledger WHERE id=?", id).Scan(&issued); e != nil || issued != 1 {
		t.Fatal("historical issued flag was changed", issued, e)
	}
	rev = r.Revision
	if e = reopened.Close(); e != nil {
		t.Fatal(e)
	}
	secondRoot := root + ".second"
	if e = os.Rename(root, secondRoot); e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(filepath.Join(root, ".ocmi", "local", "runs"), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(filepath.Join(secondRoot, ".ocmi", "local", "state.sqlite"), filepath.Join(root, ".ocmi", "local", "state.sqlite")); e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(filepath.Join(secondRoot, filepath.FromSlash(locator)), filepath.Join(root, filepath.FromSlash(locator))); e != nil {
		t.Fatal(e)
	}
	r = request(root, "resource.dispose", "main", rev, map[string]any{"resourceId": id, "disposition": "remove", "reason": "root changed after approval"})
	if r.OK || r.Code != "RESOURCE_PENDING" {
		t.Fatal("dispose accepted replaced root", r)
	}
	if _, e = os.Stat(filepath.Join(root, filepath.FromSlash(locator))); e != nil {
		t.Fatal("workspace was not preserved", e)
	}
}

func TestWorkerWorkspaceRecoveryMarkerAndInventory(t *testing.T) {
	root := t.TempDir()
	op := "8c43d039-8893-4b53-87f7-a90d29b2f91a"
	runs := filepath.Join(root, ".ocmi", "local", "runs")
	if e := os.MkdirAll(runs, 0700); e != nil {
		t.Fatal(e)
	}
	locator := ".ocmi/local/runs/" + op
	initial, e := createLocalResource(root, locator, "directory", "ocmi-"+op, nil, true)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(root, filepath.FromSlash(locator))
	if e = os.Mkdir(filepath.Join(path, "tmp"), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(path, "tmp", "fictional.txt"), []byte("diagnostic"), 0600); e != nil {
		t.Fatal(e)
	}
	current, e := captureLocalResource(root, locator, "directory", initial.NativeIdentity, true)
	if e != nil {
		t.Fatal(e)
	}
	if !workspaceMarkerMatches(initial.Inventory, current.Inventory, op) || initial.Inventory == current.Inventory {
		t.Fatal("legitimate changed inventory refused")
	}
	if workspaceMarkerMatches(current.Inventory, current.Inventory, op) {
		t.Fatal("ambiguous post-effect original inventory accepted")
	}
	marker := filepath.Join(path, localMarker)
	if e = os.Rename(marker, marker+".old"); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(marker, []byte("ocmi-"+op), 0600); e != nil {
		t.Fatal(e)
	}
	changed, e := captureLocalResource(root, locator, "directory", initial.NativeIdentity, true)
	if e != nil {
		t.Fatal(e)
	}
	if workspaceMarkerMatches(initial.Inventory, changed.Inventory, op) {
		t.Fatal("same-content marker replacement accepted")
	}
	if _, e = captureLocalResource(root, locator, "directory", "foreign-native-id", true); e == nil {
		t.Fatal("foreign workspace identity accepted")
	}
}
