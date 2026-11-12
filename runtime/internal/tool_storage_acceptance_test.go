package internal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type mainToolFixture struct {
	base, home, data, cache, profile string
	policy                           map[string]any
}

func mainToolHash(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }
func mainToolID(t *testing.T, path string) string {
	t.Helper()
	id, e := localNativeID(path)
	if e != nil {
		t.Fatal(e)
	}
	return id
}
func mainToolWrite(t *testing.T, path string, value any) {
	t.Helper()
	b, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func mainToolSetup(t *testing.T) mainToolFixture {
	t.Helper()
	f := mainToolFixture{base: t.TempDir()}
	f.home = filepath.Join(f.base, "selected home")
	f.data = filepath.Join(f.base, "ToolData")
	f.cache = filepath.Join(f.data, "widget", "cache")
	f.profile = filepath.Join(f.home, "local", "bindings", "tool-storage", "widget.json")
	if e := os.MkdirAll(filepath.Dir(f.profile), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.MkdirAll(f.cache, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(f.home, homeMarker), installationLayoutJSON, 0600); e != nil {
		t.Fatal(e)
	}
	f.policy = map[string]any{"schemaVersion": 1, "toolId": "widget", "homeIdentity": mainToolID(t, f.home),
		"toolDataRoot": f.data, "toolDataIdentity": mainToolID(t, f.data),
		"owner": "fixture-owner", "purpose": "Shared fictional tool cache", "retention": "until approved maintenance",
		"consentReference": "fictional-owner-approval", "maxBytes": 1048576, "maxAgeDays": 30,
		"stores":     []any{map[string]any{"key": "WIDGET_CACHE", "path": "widget/cache", "nativeIdentity": mainToolID(t, f.cache), "kind": "cache"}},
		"scratchEnv": []string{"WIDGET_TMP"}}
	mainToolWrite(t, f.profile, f.policy)
	return f
}
func (f mainToolFixture) ref(t *testing.T) map[string]any {
	t.Helper()
	raw, e := os.ReadFile(f.profile)
	if e != nil {
		t.Fatal(e)
	}
	return map[string]any{"home": f.home, "toolId": "widget", "sha256": mainToolHash(raw)}
}
func (f mainToolFixture) project(t *testing.T, name string, profile any, include bool) (string, Response) {
	t.Helper()
	root := filepath.Join(f.base, name)
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	r := request(root, "init", "main", 0, map[string]any{"projectId": name, "deviceId": "fixture-device", "copyId": name, "owner": "main"})
	if !r.OK {
		t.Fatal(r)
	}
	for _, name := range []string{"contract.md", "input.txt"} {
		if e := os.WriteFile(filepath.Join(root, name), []byte("Fictional tool placement fixture."), 0600); e != nil {
			t.Fatal(e)
		}
	}
	command := map[string]any{"id": "placement", "executable": os.Args[0], "args": []string{"-test.run=^TestMainToolStorageChild$", "tool-storage-child"}, "cwd": ".", "timeoutMs": 5000, "outputLimitBytes": 4096}
	if include {
		command["toolStorage"] = profile
	}
	r = request(root, "task.bind", "main", r.Revision, map[string]any{"taskId": "tool-task", "outcomeId": "tool-outcome", "taskRevision": 1,
		"completionScope": "target", "worker": "worker", "reviewer": "main", "contract": "contract.md", "inputs": []string{"input.txt"}, "command": command})
	return root, r
}
func mainToolReady(t *testing.T, root string, r Response) Response {
	t.Helper()
	if !r.OK {
		t.Fatal("bind", r)
	}
	r = request(root, "task.ready", "main", r.Revision, map[string]any{})
	if !r.OK {
		t.Fatal("ready", r)
	}
	return r
}
func mainToolRun(t *testing.T, root string, rev int64) Response {
	t.Helper()
	r := request(root, "run", "worker", rev, map[string]any{})
	if !r.OK || r.Data["operationState"] != "succeeded" {
		t.Fatal("ordinary run", r)
	}
	return r
}
func mainToolObserved(t *testing.T, root string) map[string]string {
	t.Helper()
	raw, e := os.ReadFile(filepath.Join(root, "tool-storage-effect.json"))
	if e != nil {
		t.Fatal(e)
	}
	var d map[string]string
	if json.Unmarshal(raw, &d) != nil {
		t.Fatal("fixture report")
	}
	return d
}
func TestMainToolStorageChild(t *testing.T) {
	child := false
	for _, a := range os.Args {
		if a == "tool-storage-child" {
			child = true
		}
	}
	if !child {
		t.Skip("spawned only by independent ordinary-run fixture")
	}
	d := map[string]string{"cache": os.Getenv("WIDGET_CACHE"), "scratch": os.Getenv("WIDGET_TMP"),
		"tmp": os.Getenv("TMP"), "temp": os.Getenv("TEMP"), "tmpdir": os.Getenv("TMPDIR")}
	d["gobin"], d["gotmp"] = os.Getenv("GOBIN"), os.Getenv("GOTMPDIR")
	if info, e := os.Stat(d["tmp"]); e == nil && info.IsDir() {
		d["tmpObserved"] = "present"
	}
	raw, e := json.Marshal(d)
	if e != nil {
		os.Exit(7)
	}
	if os.WriteFile("tool-storage-effect.json", raw, 0600) != nil {
		os.Exit(8)
	}
	os.Exit(0)
}
func TestMainToolStorageFixtureControl(t *testing.T) {
	f := mainToolSetup(t)
	root, r := f.project(t, "control", nil, false)
	r = mainToolReady(t, root, r)
	mainToolRun(t, root, r.Revision)
	d := mainToolObserved(t, root)
	if d["tmpObserved"] != "present" || d["tmp"] == "" || d["tmp"] != d["temp"] || d["tmp"] != d["tmpdir"] {
		t.Fatal("existing native owned TMP fixture is not valid", d)
	}
	if d["cache"] != "" || d["scratch"] != "" {
		t.Fatal("control inherited undeclared environment", d)
	}
	if d["cache"] == f.cache {
		t.Fatal("targeted wrong-observation oracle failed")
	}
	raw, e := os.ReadFile(f.profile)
	if e != nil {
		t.Fatal(e)
	}
	before := mainToolID(t, f.profile)
	if e = os.Rename(f.profile, f.profile+".original"); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(f.profile, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if mainToolID(t, f.profile) == before {
		t.Fatal("same-byte native replacement premise not qualified")
	}
}
func TestMainToolStorageAppliedSharedAndExternal(t *testing.T) {
	f := mainToolSetup(t)
	profileBefore, e := os.ReadFile(f.profile)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"project-a", "project-b"} {
		root, r := f.project(t, name, f.ref(t), true)
		r = mainToolReady(t, root, r)
		run := mainToolRun(t, root, r.Revision)
		d := mainToolObserved(t, root)
		if d["cache"] != f.cache || d["scratch"] != d["tmp"] || d["tmpObserved"] != "present" {
			t.Fatal("profile not applied by ordinary child", d)
		}
		db, e := openExistingFor(Request{ProjectRoot: root})
		if e != nil {
			t.Fatal(e)
		}
		var id, mode, ownership, disposition, inventory string
		e = db.QueryRow("SELECT id,mode,ownership,intended_disposition,inventory FROM resource_ledger WHERE operation_id=? AND locator=?", run.Data["operationId"], f.cache).Scan(&id, &mode, &ownership, &disposition, &inventory)
		if e != nil {
			db.Close()
			t.Fatal("shared store missing from pre-effect operation ledger", e)
		}
		var active int
		e = db.QueryRow("SELECT count(*) FROM resource_use WHERE resource_id=? AND active=1", id).Scan(&active)
		var profileUse int
		profileErr := db.QueryRow("SELECT count(*) FROM resource_ledger WHERE operation_id=? AND locator=? AND mode='use' AND ownership='external'", run.Data["operationId"], f.profile).Scan(&profileUse)
		db.Close()
		if profileErr != nil || profileUse != 1 {
			t.Fatal("selected profile missing from operation dependencies", profileErr, profileUse)
		}
		if e != nil || mode != "use" || ownership != "external" || disposition != "release" || inventory != "" || active != 0 {
			t.Fatal("shared store incorrectly owned/scanned or use not released", mode, ownership, disposition, inventory, active, e)
		}
		denied := request(root, "resource.dispose", "main", run.Revision, map[string]any{"resourceId": id, "disposition": "remove", "reason": "must never delete a shared cache"})
		if denied.OK {
			t.Fatal("project acquired shared-store deletion", denied)
		}
		if _, e = os.Stat(f.cache); e != nil {
			t.Fatal("shared cache lost", e)
		}
	}
	after, e := os.ReadFile(f.profile)
	if e != nil || !bytes.Equal(profileBefore, after) {
		t.Fatal("consumer rewrote machine policy")
	}
}
func TestMainToolStorageChangedBindingPreventsLaunch(t *testing.T) {
	for _, kind := range []string{"bytes", "same-byte-native", "store-native"} {
		t.Run(kind, func(t *testing.T) {
			f := mainToolSetup(t)
			root, r := f.project(t, "project", f.ref(t), true)
			r = mainToolReady(t, root, r)
			switch kind {
			case "bytes":
				f.policy["purpose"] = "Changed after readiness"
				mainToolWrite(t, f.profile, f.policy)
			case "same-byte-native":
				raw, e := os.ReadFile(f.profile)
				if e != nil {
					t.Fatal(e)
				}
				if e = os.Rename(f.profile, f.profile+".old"); e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(f.profile, raw, 0600); e != nil {
					t.Fatal(e)
				}
			case "store-native":
				if e := os.Rename(f.cache, f.cache+".old"); e != nil {
					t.Fatal(e)
				}
				if e := os.Mkdir(f.cache, 0700); e != nil {
					t.Fatal(e)
				}
			}
			denied := request(root, "run", "worker", r.Revision, map[string]any{})
			if denied.OK {
				t.Fatal("changed binding launched", denied)
			}
			if _, e := os.Stat(filepath.Join(root, "tool-storage-effect.json")); !os.IsNotExist(e) {
				t.Fatal("unqualified child effect", e)
			}
		})
	}
}
func TestMainToolStorageInvalidProfileRefused(t *testing.T) {
	for _, kind := range []string{"null-ref", "unknown-ref", "bad-tool-id", "unknown-policy", "missing-consent", "reserved-env", "outside-store", "case-duplicate", "bad-policy-kind", "zero-policy", "bad-identity", "foreign-env", "shared-temp", "case-ref", "case-policy"} {
		t.Run(kind, func(t *testing.T) {
			f := mainToolSetup(t)
			ref := f.ref(t)
			var value any = ref
			switch kind {
			case "null-ref":
				value = nil
			case "unknown-ref":
				ref["unexpected"] = "reject"
			case "bad-tool-id":
				ref["toolId"] = "../widget"
			case "unknown-policy":
				f.policy["unexpected"] = "reject"
			case "missing-consent":
				f.policy["consentReference"] = ""
			case "reserved-env":
				f.policy["stores"].([]any)[0].(map[string]any)["key"] = "PATH"
			case "outside-store":
				f.policy["stores"].([]any)[0].(map[string]any)["path"] = "../outside"
			case "case-duplicate":
				f.policy["stores"] = []any{f.policy["stores"].([]any)[0], map[string]any{"key": "widget_cache", "path": "widget/cache", "nativeIdentity": mainToolID(t, f.cache), "kind": "cache"}}
			case "bad-policy-kind":
				f.policy["stores"].([]any)[0].(map[string]any)["kind"] = "secret"
			case "zero-policy":
				f.policy["maxBytes"] = 0
			case "bad-identity":
				f.policy["toolDataIdentity"] = "not-current"
			case "foreign-env":
				f.policy["stores"].([]any)[0].(map[string]any)["key"] = "FOREIGN_CACHE"
			case "shared-temp":
				f.policy["stores"].([]any)[0].(map[string]any)["key"] = "GOTMPDIR"
			case "case-ref":
				ref["Home"] = ref["home"]
				delete(ref, "home")
			case "case-policy":
				f.policy["Owner"] = f.policy["owner"]
				delete(f.policy, "owner")
			}
			if kind != "null-ref" && kind != "unknown-ref" && kind != "bad-tool-id" && kind != "case-ref" {
				mainToolWrite(t, f.profile, f.policy)
				ref = f.ref(t)
				value = ref
			}
			root, r := f.project(t, "project", value, true)
			if r.OK {
				r = request(root, "task.ready", "main", r.Revision, map[string]any{})
			}
			if r.OK {
				t.Fatal("invalid profile became ready", kind, r)
			}
			if _, e := os.Stat(filepath.Join(root, "tool-storage-effect.json")); !os.IsNotExist(e) {
				t.Fatal("invalid profile caused effect")
			}
		})
	}
}
func TestMainToolStorageLinkedStoreRefused(t *testing.T) {
	f := mainToolSetup(t)
	outside := filepath.Join(f.base, "foreign")
	if e := os.Mkdir(outside, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Rename(f.cache, f.cache+".original"); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(outside, f.cache); e != nil {
		t.Skip("native symlink privilege unavailable", e)
	}
	f.policy["stores"].([]any)[0].(map[string]any)["nativeIdentity"] = mainToolID(t, outside)
	mainToolWrite(t, f.profile, f.policy)
	root, r := f.project(t, "project", f.ref(t), true)
	if r.OK {
		r = request(root, "task.ready", "main", r.Revision, map[string]any{})
	}
	if r.OK {
		t.Fatal("linked shared store accepted", r)
	}
}
func TestMainToolStorageLegacyFingerprint(t *testing.T) {
	f := mainToolSetup(t)
	root, r := f.project(t, "legacy-fingerprint", nil, false)
	if !r.OK {
		t.Fatal(r)
	}
	db, e := openExistingFor(Request{ProjectRoot: root})
	if e != nil {
		t.Fatal(e)
	}
	var raw, actual string
	e = db.QueryRow("SELECT command_json,command_fingerprint FROM task WHERE id='tool-task'").Scan(&raw, &actual)
	db.Close()
	if e != nil {
		t.Fatal(e)
	}
	executable, e := digest(os.Args[0])
	if e != nil {
		t.Fatal(e)
	}
	old, _ := json.Marshal(struct {
		Command    json.RawMessage
		Executable string
	}{json.RawMessage(raw), executable})
	if actual != mainToolHash(old) {
		t.Fatal("no-profile fingerprint changed from historical command representation")
	}
}

func TestMainToolStorageGoBinAndScratch(t *testing.T) {
	f := mainToolSetup(t)
	bin := filepath.Join(f.data, "go", "bin")
	if e := os.MkdirAll(bin, 0700); e != nil {
		t.Fatal(e)
	}
	f.profile = filepath.Join(filepath.Dir(f.profile), "go.json")
	f.policy["toolId"] = "go"
	f.policy["stores"] = []any{map[string]any{"key": "GOBIN", "path": "go/bin", "nativeIdentity": mainToolID(t, bin), "kind": "data"}}
	f.policy["scratchEnv"] = []string{"GOTMPDIR"}
	mainToolWrite(t, f.profile, f.policy)
	raw, e := os.ReadFile(f.profile)
	if e != nil {
		t.Fatal(e)
	}
	ref := map[string]any{"home": f.home, "toolId": "go", "sha256": mainToolHash(raw)}
	root, r := f.project(t, "go-project", ref, true)
	r = mainToolReady(t, root, r)
	mainToolRun(t, root, r.Revision)
	d := mainToolObserved(t, root)
	if d["gobin"] != bin || d["gotmp"] != d["tmp"] {
		t.Fatal("supported native Go bin layout or scratch not applied", d)
	}
}

func TestMainToolStorageStrictRepresentation(t *testing.T) {
	for _, kind := range []string{"value-equals-key", "custom-namespace-key", "case-store-key", "duplicate-store-key"} {
		t.Run(kind, func(t *testing.T) {
			f := mainToolSetup(t)
			accept := kind == "value-equals-key" || kind == "custom-namespace-key"
			if kind == "value-equals-key" {
				f.policy["owner"] = "purpose"
			}
			if kind == "custom-namespace-key" {
				f.policy["stores"].([]any)[0].(map[string]any)["key"] = "WIDGET_STORAGE"
			}
			mainToolWrite(t, f.profile, f.policy)
			raw, e := os.ReadFile(f.profile)
			if e != nil {
				t.Fatal(e)
			}
			if kind == "case-store-key" {
				raw = bytes.Replace(raw, []byte("\"key\":\"WIDGET_CACHE\""), []byte("\"Key\":\"WIDGET_CACHE\""), 1)
			}
			if kind == "duplicate-store-key" {
				raw = bytes.Replace(raw, []byte("\"kind\":\"cache\""), []byte("\"kind\":\"cache\",\"kind\":\"data\""), 1)
			}
			if e = os.WriteFile(f.profile, raw, 0600); e != nil {
				t.Fatal(e)
			}
			root, r := f.project(t, "project", f.ref(t), true)
			if r.OK {
				r = request(root, "task.ready", "main", r.Revision, map[string]any{})
			}
			if r.OK != accept {
				t.Fatal("strict representation or declared namespace mismatch", kind, r)
			}
		})
	}
}

func TestMainToolStorageTopLevelCommandKey(t *testing.T) {
	for _, key := range []string{"toolStorage", "ToolStorage"} {
		t.Run(key, func(t *testing.T) {
			f := mainToolSetup(t)
			root, r := f.project(t, "project", nil, false)
			r = mainToolReady(t, root, r)
			db, e := openExistingFor(Request{ProjectRoot: root})
			if e != nil {
				t.Fatal(e)
			}
			var raw string
			e = db.QueryRow("SELECT command_json FROM task WHERE id='tool-task'").Scan(&raw)
			db.Close()
			if e != nil {
				t.Fatal(e)
			}
			var command map[string]any
			if json.Unmarshal([]byte(raw), &command) != nil {
				t.Fatal("command fixture")
			}
			command[key] = f.ref(t)
			r = request(root, "task.revise", "main", r.Revision, map[string]any{"taskId": "tool-task", "outcomeId": "tool-outcome", "taskRevision": 2, "completionScope": "target", "worker": "worker", "reviewer": "main", "contract": "contract.md", "inputs": []string{"input.txt"}, "command": command, "reason": "Fictional exact-key control"})
			if r.OK {
				r = request(root, "task.ready", "main", r.Revision, map[string]any{})
			}
			if r.OK != (key == "toolStorage") {
				t.Fatal("exact-key control or ambiguous top-level field", key, r)
			}
		})
	}
}

func TestMainToolStorageNativeCLI(t *testing.T) {
	binary := os.Getenv("OCMI_ROOT_BINARY")
	if binary == "" {
		t.Skip("set after actual native CLI build")
	}
	f := mainToolSetup(t)
	root, r := f.project(t, "cli-project", f.ref(t), true)
	r = mainToolReady(t, root, r)
	raw, _ := json.Marshal(map[string]any{"schemaVersion": 1, "action": "run", "projectRoot": root, "actor": "worker", "requestId": "tool-storage-native-cli", "expectedRevision": r.Revision, "payload": map[string]any{}})
	cmd := exec.Command(binary, "request")
	cmd.Stdin = bytes.NewReader(raw)
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatal("native CLI", e, string(out))
	}
	var response Response
	if json.Unmarshal(out, &response) != nil || !response.OK || response.Data["operationState"] != "succeeded" {
		t.Fatal("native CLI response", string(out))
	}
	if d := mainToolObserved(t, root); d["cache"] != f.cache || d["scratch"] != d["tmp"] || !strings.Contains(d["tmp"], string(os.PathSeparator)+"runs"+string(os.PathSeparator)) {
		t.Fatal("actual CLI did not use bound storage", d)
	}
}
