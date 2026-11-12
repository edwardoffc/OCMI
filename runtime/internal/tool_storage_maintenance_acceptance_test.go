package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMainToolAuthoringMaintenanceInspect(t *testing.T) {
	mainMaintenanceInspect(t, "")
}

func TestMainToolAuthoringMaintenanceRefusals(t *testing.T) {
	f := mainRelocationSetup(t)
	_, raw := mainAuthorProfile(t, f.mainAuthorFixture, "go")
	var policy toolStoragePolicy
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	for _, store := range policy.Stores {
		if store.Key == "GOCACHE" {
			dir := filepath.Join(policy.ToolDataRoot, filepath.FromSlash(store.Path), "bounded-fixture")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "value"), []byte("preserve finite cache fixture"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	before := mainRelocationSnapshot(t, f.data)
	profileID := mainToolID(t, filepath.Join(f.home, "local", "bindings", "tool-storage", "go.json"))
	for _, scenario := range []string{"worker", "unknown-key", "case-key", "duplicate-key", "data-store", "unknown-store", "unsupported-tool", "wrong-profile", "zero-entries", "entry-overflow", "zero-bytes", "byte-overflow", "entry-bound", "byte-bound"} {
		t.Run(scenario, func(t *testing.T) {
			input := map[string]any{"toolId": "go", "profileSha256": mainToolHash(raw), "storeKey": "GOCACHE", "maxEntries": 1024, "maxBytes": 16 << 20}
			actor := "coordinator"
			switch scenario {
			case "worker":
				actor = "worker"
			case "unknown-key":
				input["delete"] = true
			case "case-key":
				input["StoreKey"] = "GOCACHE"
			case "data-store":
				input["storeKey"] = "GOPATH"
			case "unknown-store":
				input["storeKey"] = "UNDECLARED_CACHE"
			case "unsupported-tool":
				input["toolId"] = "widget"
			case "wrong-profile":
				input["profileSha256"] = strings.Repeat("0", 64)
			case "zero-entries":
				input["maxEntries"] = 0
			case "entry-overflow":
				input["maxEntries"] = 1025
			case "zero-bytes":
				input["maxBytes"] = 0
			case "byte-overflow":
				input["maxBytes"] = (16 << 20) + 1
			case "entry-bound":
				input["maxEntries"] = 1
			case "byte-bound":
				input["maxBytes"] = 1
			}
			encoded, _ := json.Marshal(input)
			if scenario == "duplicate-key" {
				encoded = append(encoded[:len(encoded)-1], []byte(`,"storeKey":"GOCACHE"}`)...)
			}
			r := Handle(Request{SchemaVersion: 1, Action: "tool-storage.maintain.inspect", ProjectRoot: f.home, RootKind: "installation", Actor: actor, RequestID: "fictional-maintenance-refusal-" + scenario, Payload: encoded})
			if r.OK {
				t.Fatal("unsupported/incomplete measurement accepted", scenario, r)
			}
			if !reflect.DeepEqual(before, mainRelocationSnapshot(t, f.data)) {
				t.Fatal("refusal changed selected/foreign data")
			}
			_, current := mainAuthorProfile(t, f.mainAuthorFixture, "go")
			if !bytes.Equal(raw, current) || mainToolID(t, filepath.Join(f.home, "local", "bindings", "tool-storage", "go.json")) != profileID {
				t.Fatal("measurement refusal changed profile authority")
			}
		})
	}
}

func TestMainToolAuthoringMaintenanceUseAndPending(t *testing.T) {
	for _, scenario := range []string{"active", "unknown-use", "pending-transition"} {
		t.Run(scenario, func(t *testing.T) {
			f := mainRelocationSetup(t)
			_, raw := mainAuthorProfile(t, f.mainAuthorFixture, "go")
			if scenario == "active" {
				mainRelocationPair(t, f)
			}
			var retained string
			if scenario == "unknown-use" {
				retained = filepath.Join(f.home, "local", "bindings", "tool-storage", "uses", "go", "unproved-reference.json")
			}
			if scenario == "pending-transition" {
				retained = filepath.Join(f.home, "local", "bindings", "tool-storage", "relocation.json")
			}
			if retained != "" {
				if err := os.WriteFile(retained, []byte(`{"phase":"active","closed":true}`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before := mainRelocationSnapshot(t, f.data)
			input, _ := json.Marshal(map[string]any{"toolId": "go", "profileSha256": mainToolHash(raw), "storeKey": "GOCACHE", "maxEntries": 1024, "maxBytes": 16 << 20})
			r := Handle(Request{SchemaVersion: 1, Action: "tool-storage.maintain.inspect", ProjectRoot: f.home, RootKind: "installation", Actor: "coordinator", RequestID: "fictional-maintenance-state-" + scenario, Payload: input})
			switch scenario {
			case "active":
				encoded, _ := json.Marshal(r.Data)
				var report map[string]any
				_ = json.Unmarshal(encoded, &report)
				if !r.OK || report["activeUses"] != float64(2) || report["pendingUses"] != float64(0) || report["mutationSupported"] != false {
					t.Fatal("active use was hidden or blocked harmless bounded observation", r)
				}
			case "unknown-use":
				if r.OK && (r.Data["pendingUses"] != "unknown" || r.Data["mutationSupported"] != false) {
					t.Fatal("unproved use became zero/clean", r)
				}
			case "pending-transition":
				if r.OK {
					t.Fatal("unproved transition authorized a fresh measurement", r)
				}
			}
			if !reflect.DeepEqual(before, mainRelocationSnapshot(t, f.data)) {
				t.Fatal("maintenance observation changed storage")
			}
			if retained != "" {
				if current, err := os.ReadFile(retained); err != nil || string(current) != `{"phase":"active","closed":true}` {
					t.Fatal("uncertain evidence changed", err)
				}
			}
		})
	}
}

func TestMainToolAuthoringMaintenanceNativeCLI(t *testing.T) {
	binary := os.Getenv("OCMI_ROOT_BINARY")
	if binary == "" {
		t.Skip("actual native binary required by integration runner")
	}
	mainMaintenanceInspect(t, binary)
}

func mainMaintenanceInspect(t *testing.T, binary string) {
	t.Helper()
	f := mainRelocationSetup(t)
	_, raw := mainAuthorProfile(t, f.mainAuthorFixture, "go")
	var policy toolStoragePolicy
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	var cache string
	for _, store := range policy.Stores {
		if store.Key == "GOCACHE" && store.Kind == "cache" {
			cache = filepath.Join(policy.ToolDataRoot, filepath.FromSlash(store.Path))
		}
	}
	if cache == "" {
		t.Fatal("qualified native fixture has no selected cache")
	}
	path := filepath.Join(cache, "fictional-measurement-sentinel")
	if err := os.WriteFile(path, []byte("selected reproducible fixture bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Duration(policy.MaxAgeDays+2) * 24 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	var entries int
	var size int64
	if err := filepath.WalkDir(cache, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == cache {
			return nil
		}
		entries++
		info, err := d.Info()
		if err == nil && info.Mode().IsRegular() {
			size += info.Size()
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	before := mainRelocationSnapshot(t, f.data)
	input, _ := json.Marshal(map[string]any{"toolId": "go", "profileSha256": mainToolHash(raw), "storeKey": "GOCACHE", "maxEntries": 1024, "maxBytes": 16 << 20})
	call := func(action string, payload json.RawMessage) Response {
		req := Request{SchemaVersion: 1, Action: action, ProjectRoot: f.home, RootKind: "installation", Actor: "coordinator", RequestID: "fictional-bounded-" + action, Payload: payload}
		if binary == "" {
			return Handle(req)
		}
		encoded, _ := json.Marshal(req)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, "request")
		cmd.Stdin = bytes.NewReader(encoded)
		out, err := cmd.CombinedOutput()
		var response Response
		if json.Unmarshal(out, &response) != nil || ctx.Err() != nil {
			t.Fatalf("actual coordinator request protocol: %v %.400s", err, out)
		}
		if response.OK && err != nil {
			t.Fatal("successful response had failing process exit", err)
		}
		return response
	}
	if r := call("tool-storage.inspect", json.RawMessage(`{"toolId":"go"}`)); !r.OK || r.Data["phase"] != "active" {
		t.Fatal("existing actual coordinator inspection control", r)
	}
	r := call("tool-storage.maintain.inspect", input)
	if !r.OK || r.Code != "TOOL_STORAGE_MAINTENANCE_INSPECTED" {
		t.Fatal("new bounded coordinator inspection missing", r)
	}
	encoded, _ := json.Marshal(r.Data)
	var report map[string]any
	if err := json.Unmarshal(encoded, &report); err != nil {
		t.Fatal(err)
	}
	if report["observedEntries"] != float64(entries) || report["observedBytes"] != float64(size) || report["sizeLimitBytes"] != float64(policy.MaxBytes) || report["ageLimitDays"] != float64(policy.MaxAgeDays) || report["ageExceeded"] != true || report["maintenanceRequired"] != true || report["mutationSupported"] != false || report["activeUses"] != float64(0) || report["pendingUses"] != float64(0) || report["owner"] != policy.Owner || report["purpose"] != policy.Purpose || report["retention"] != policy.Retention || report["nextAction"] == nil || report["nextAction"] == "" {
		t.Fatal("incomplete or false maintenance report", report, entries, size)
	}
	if after := mainRelocationSnapshot(t, f.data); len(after) != len(before) {
		t.Fatal("inspection changed selected/foreign storage")
	} else {
		for path, content := range before {
			if after[path] != content {
				t.Fatal("inspection changed storage bytes", path)
			}
		}
	}
}
