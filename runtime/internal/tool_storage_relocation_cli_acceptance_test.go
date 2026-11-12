package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// Reuse the qualified B1 request adapter; its first inspect is the control.
func TestMainToolAuthoringRelocationNativeCLI(t *testing.T) {
	binary := os.Getenv("OCMI_ROOT_BINARY")
	if binary == "" {
		t.Skip("actual native binary required by integration runner")
	}
	f := mainRelocationSetup(t)
	call := func(action string, payload any) Response {
		raw, err := json.Marshal(map[string]any{"schemaVersion": 1, "action": "tool-storage." + action,
			"projectRoot": f.home, "rootKind": "installation", "actor": "coordinator",
			"requestId": "actual-cli-relocation-" + action, "expectedRevision": 0, "payload": payload})
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary, "request")
		cmd.Stdin = bytes.NewReader(raw)
		out, err := cmd.CombinedOutput()
		var r Response
		if json.Unmarshal(out, &r) != nil {
			t.Fatalf("actual relocation CLI response invalid: %v %.500s", err, out)
		}
		if err != nil || !r.OK {
			t.Fatal("actual relocation CLI", action, err, r)
		}
		return r
	}
	if r := call("inspect", map[string]any{"toolId": "go"}); r.Data["phase"] != "active" {
		t.Fatal("qualified existing CLI control", r)
	}
	before := mainRelocationSnapshot(t, f.data)
	opts := mainRelocationOptions(t, f)
	p := call("relocate.prepare", opts)
	if p.Data["phase"] != "prepared" {
		t.Fatal("actual CLI did not prepare", p)
	}
	if _, err := os.Lstat(opts["toolDataRoot"].(string)); !os.IsNotExist(err) {
		t.Fatal("CLI prepare created destination", err)
	}
	approval := mainRelocationApproval(p)
	if r := call("relocate.apply", approval); r.Data["phase"] != "active" {
		t.Fatal("actual CLI did not activate", r)
	}
	newPackage := filepath.Join(opts["toolDataRoot"].(string), "go", "workspace", "src", "selected-module")
	root, ready := mainRelocationReadOnlyProject(t, f, "cli-selected-module")
	mainRelocationRun(t, root, ready, newPackage)
	if r := call("relocate.inspect", map[string]any{"toolId": "go"}); fmt.Sprint(r.Data["activeUses"]) != "0" || fmt.Sprint(r.Data["pendingUses"]) != "0" {
		t.Fatal("CLI ordinary use did not close", r)
	}
	if r := call("relocate.rollback", approval); r.Data["phase"] != "rolled_back" {
		t.Fatal("actual CLI did not reverse", r)
	}
	if !reflect.DeepEqual(before, mainRelocationSnapshot(t, f.data)) {
		t.Fatal("CLI reverse changed retained source before ordinary consumer")
	}
	root, ready = mainRelocationProject(t, f, "cli-original-module", f.packageDir)
	mainRelocationRun(t, root, ready, f.packageDir)
	if !reflect.DeepEqual(mainRelocationStableInputs(before), mainRelocationStableInputs(mainRelocationSnapshot(t, f.data))) {
		t.Fatal("CLI reverse consumer changed durable/offline/foreign input")
	}
	fmt.Println("TOOL_STORAGE_RELOCATION_NATIVE_CLI_OK")
}

// The qualified fixture has ten total entries but only three files.
// A file-only counter would incorrectly accept this four-entry budget.
func TestMainToolAuthoringRelocationBoundedTree(t *testing.T) {
	f := mainRelocationSetup(t)
	before := mainRelocationSnapshot(t, f.data)
	_, raw := mainAuthorProfile(t, f.mainAuthorFixture, "go")
	var policy map[string]any
	if json.Unmarshal(raw, &policy) != nil || policy["schemaVersion"] != float64(2) {
		t.Fatal("lifecycle-capable native premise missing")
	}
	opts := mainRelocationOptions(t, f)
	opts["maxEntries"] = 4
	if r := mainRelocationCall(f, "prepare", opts); r.OK {
		t.Fatal("entry budget ignored directories", r)
	}
	if _, err := os.Lstat(opts["toolDataRoot"].(string)); !os.IsNotExist(err) {
		t.Fatal("bounded refusal created target", err)
	}
	if !reflect.DeepEqual(before, mainRelocationSnapshot(t, f.data)) {
		t.Fatal("bounded refusal altered source")
	}
}
