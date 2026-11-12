package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMainToolAuthoringRelocationReviewDestinationParent(t *testing.T) {
	f := mainRelocationSetup(t)
	parent := filepath.Join(f.base, "selected-target-parent")
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	opts := mainRelocationOptions(t, f)
	opts["toolDataRoot"] = filepath.Join(parent, "ToolData")
	p := mainRelocationPrepare(t, f, opts)
	id := mainToolID(t, parent)
	retained := filepath.Join(f.base, "retained-target-parent")
	if err := os.Rename(parent, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	if mainToolID(t, parent) == id {
		t.Fatal("qualified parent replacement premise failed")
	}
	r := mainRelocationCall(f, "apply", mainRelocationApproval(p))
	if r.OK {
		t.Fatal("replaced destination parent adopted", r)
	}
	if _, err := os.Lstat(opts["toolDataRoot"].(string)); !os.IsNotExist(err) {
		t.Fatal("unapproved parent received effects", err)
	}
	if mainToolID(t, retained) != id {
		t.Fatal("retained original parent changed")
	}
}

func TestMainToolAuthoringRelocationReviewReceiptIdentity(t *testing.T) {
	f := mainRelocationSetup(t)
	opts := mainRelocationOptions(t, f)
	p := mainRelocationPrepare(t, f, opts)
	path := filepath.Join(f.home, "local", "bindings", "tool-storage", "receipts", "go.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	id := mainToolID(t, path)
	retained := filepath.Join(f.base, "retained-native-receipt")
	if err := os.Rename(path, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if mainToolID(t, path) == id {
		t.Fatal("qualified same-byte replacement premise failed")
	}
	if r := mainRelocationCall(f, "apply", mainRelocationApproval(p)); r.OK {
		t.Fatal("receipt replacement silently adopted", r)
	}
	if _, err := os.Lstat(opts["toolDataRoot"].(string)); !os.IsNotExist(err) {
		t.Fatal("stale native receipt allowed destination effects", err)
	}
	if mainToolID(t, retained) != id {
		t.Fatal("old receipt changed")
	}
}

func TestMainToolAuthoringRelocationReviewMalformedTerminal(t *testing.T) {
	f := mainRelocationSetup(t)
	root, _ := mainRelocationReadOnlyProject(t, f, "journal-bound-consumer")
	command, _ := json.Marshal(map[string]any{"executable": os.Args[0], "toolStorage": mainRelocationRef(t, f)})
	if _, err := toolStorageFromCommand(root, command); err != nil {
		t.Fatal("native positive binding control", err)
	}
	path := filepath.Join(f.home, "local", "bindings", "tool-storage", "relocation.json")
	raw, _ := json.Marshal(map[string]any{"phase": "active", "closed": true})
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := toolStorageFromCommand(root, command); err == nil {
		t.Fatal("structurally incomplete terminal journal bypassed pending barrier")
	}
}

func TestMainToolAuthoringRelocationReviewForeignUseRoot(t *testing.T) {
	f := mainAuthorSetup(t)
	goExe, _, _ := mainAuthorNativeTools(t)
	root := filepath.Join(f.home, "local", "bindings", "tool-storage", "uses", "go")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "foreign-proof")
	if err := os.WriteFile(marker, []byte("preserve preexisting unknown registry"), 0600); err != nil {
		t.Fatal(err)
	}
	options := mainAuthorOptions(f, "go")
	options["executable"] = goExe
	p := mainAuthorCall(f, "prepare", "coordinator", options)
	if p.OK {
		if r := mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, p, "go")); r.OK {
			t.Fatal("fresh native profile adopted foreign use registry", r)
		}
	}
	if _, err := os.Lstat(f.data); !os.IsNotExist(err) {
		t.Fatal("foreign registry was not rejected before external effects", err)
	}
	if raw, err := os.ReadFile(marker); err != nil || string(raw) != "preserve preexisting unknown registry" {
		t.Fatal("foreign registry modified", err)
	}
}

func TestMainToolAuthoringRelocationReviewUseProvenance(t *testing.T) {
	f := mainRelocationSetup(t)
	uses := mainRelocationPair(t, f)
	db, err := openExisting(uses[0].root)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	err = db.QueryRow("SELECT o.id FROM operation o JOIN operation_owner p ON p.operation_id=o.id WHERE p.phase='registered' AND o.state IN ('prepared','running')").Scan(&id)
	var command string
	if err == nil {
		err = db.QueryRow("SELECT t.command_json FROM operation o JOIN task t ON t.id=o.task_id WHERE o.id=?", id).Scan(&command)
	}
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	binding, err := toolStorageFromCommand(uses[0].root, json.RawMessage(command))
	if err != nil || toolStorageUseVerify(binding, id, uses[0].root) != nil {
		t.Fatal("registered native use positive control", err)
	}
	path := filepath.Join(f.home, "local", "bindings", "tool-storage", "uses", "go", id+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	original := append([]byte(nil), raw...)
	defer func() {
		if err := os.WriteFile(path, original, 0600); err != nil {
			t.Error("restore only owned fixture proof", err)
		}
	}()
	var record map[string]any
	if json.Unmarshal(raw, &record) != nil {
		t.Fatal("fixture use proof")
	}
	record["taskId"] = "unrelated-task"
	raw, _ = json.Marshal(record)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uses[0].root, "release"), []byte("close own child"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-uses[0].result:
		uses[0].done = true
		t.Logf("native operation closure code=%s", r.Code)
	case <-time.After(40 * time.Second):
		t.Fatal("owned child did not stop")
	}
	mainRelocationFinish(t, uses[1])
	if _, err := os.Lstat(path); err != nil {
		t.Fatal("uncertain task provenance was silently released", err)
	}
	r := mainRelocationCall(f, "prepare", mainRelocationOptions(t, f))
	reason, _ := r.Data["reason"].(string)
	next, _ := r.Data["nextAction"].(string)
	if r.OK || strings.TrimSpace(reason) == "" || strings.TrimSpace(next) == "" {
		t.Fatal("uncertain use became idle/unactionable", r)
	}
}
