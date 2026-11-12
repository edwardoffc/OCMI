package internal

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func mainMaintenanceRequest(t *testing.T, f mainRelocationFixture) Response {
	t.Helper()
	_, raw := mainAuthorProfile(t, f.mainAuthorFixture, "go")
	input, _ := json.Marshal(map[string]any{"toolId": "go", "profileSha256": mainToolHash(raw), "storeKey": "GOCACHE", "maxEntries": 1024, "maxBytes": 16 << 20})
	return Handle(Request{SchemaVersion: 1, Action: "tool-storage.maintain.inspect", ProjectRoot: f.home, RootKind: "installation", Actor: "coordinator", RequestID: "fictional-current-maintenance", Payload: input})
}

func TestMainToolAuthoringMaintenanceClosedTransition(t *testing.T) {
	f := mainRelocationSetup(t)
	before := mainRelocationSnapshot(t, f.data)
	plan := mainRelocationPrepare(t, f, mainRelocationOptions(t, f))
	approval := mainRelocationApproval(plan)
	if moved := mainRelocationCall(f, "apply", approval); !moved.OK {
		t.Fatal("qualified preserving relocation control", moved)
	}
	if r := mainMaintenanceRequest(t, f); !r.OK || r.Code != "TOOL_STORAGE_MAINTENANCE_INSPECTED" || r.Data["mutationSupported"] != false {
		t.Fatal("valid closed relocation incorrectly treated as unfinished", r)
	}
	if reversed := mainRelocationCall(f, "rollback", approval); !reversed.OK {
		t.Fatal("read-only inspection invalidated unchanged reverse", reversed)
	}
	if r := mainMaintenanceRequest(t, f); !r.OK || r.Code != "TOOL_STORAGE_MAINTENANCE_INSPECTED" {
		t.Fatal("valid closed reverse incorrectly treated as unfinished", r)
	}
	if !reflect.DeepEqual(before, mainRelocationSnapshot(t, f.data)) {
		t.Fatal("inspection/transition changed retained source or foreign bytes")
	}
}

func TestMainToolAuthoringMaintenanceReceiptContext(t *testing.T) {
	f := mainRelocationSetup(t)
	c, code := resolveLayout(f.home, "installation")
	if code != "" {
		t.Fatal(code)
	}
	path := toolStorageReceiptPath(c, "go")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var receipt toolStorageReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.HomeIdentity = "fictional-different-home"
	changed, _ := json.Marshal(receipt)
	if err := os.WriteFile(path, changed, 0600); err != nil {
		t.Fatal(err)
	}
	before := mainRelocationSnapshot(t, f.data)
	if r := mainMaintenanceRequest(t, f); r.OK {
		t.Fatal("receipt from a different context authorized inspection", r)
	}
	if current, err := os.ReadFile(path); err != nil || !bytes.Equal(current, changed) {
		t.Fatal("invalid native receipt was rewritten", err)
	}
	if !reflect.DeepEqual(before, mainRelocationSnapshot(t, f.data)) {
		t.Fatal("receipt refusal changed selected/foreign storage")
	}
}
