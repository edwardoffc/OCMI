package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mainContextData(t *testing.T, r Response) map[string]any {
	t.Helper()
	raw, err := json.Marshal(r.Data)
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if json.Unmarshal(raw, &d) != nil {
		t.Fatal("invalid context JSON")
	}
	return d
}

func mainContextFixture(t *testing.T) (string, int64) {
	t.Helper()
	root, revision := setup(t, "0", 3000, 1024)
	if err := os.Mkdir(filepath.Join(root, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one.go", "two.go"} {
		if err := os.WriteFile(filepath.Join(root, "src", name), []byte("package fixture\nfunc compute() int { return 1 }\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root, revision
}

func mainContextRead(root string, rev int64, files []string) Response {
	return request(root, "context.inspect", "main", rev, map[string]any{"mode": "selected", "files": files, "terms": []string{"compute"}})
}

func TestMainContextCacheAndFreshContent(t *testing.T) {
	root, rev := mainContextFixture(t)
	first := mainContextRead(root, rev, []string{"src/one.go", "src/two.go"})
	if !first.OK {
		t.Fatal(first)
	}
	second := mainContextRead(root, rev, []string{"src/one.go", "src/two.go"})
	if !second.OK {
		t.Fatal(second)
	}
	d := mainContextData(t, second)
	if d["cache"].(map[string]any)["reused"].(float64) < 1 {
		t.Fatal("same-process parser cache not reused", d)
	}
	p := filepath.Join(root, "src", "one.go")
	stamp, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(p, []byte("package fixture\nfunc compute() int { return 2 }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Chtimes(p, stamp.ModTime(), stamp.ModTime()); err != nil {
		t.Fatal(err)
	}
	fresh := mainContextRead(root, rev, []string{"src/one.go"})
	if !fresh.OK {
		t.Fatal(fresh)
	}
	d = mainContextData(t, fresh)
	encoded, _ := json.Marshal(d["selected"])
	if !strings.Contains(string(encoded), "return 2") || strings.Contains(string(encoded), "src/two.go") {
		t.Fatal("stale or out-of-scope snippets", d)
	}
	cache := d["cache"].(map[string]any)
	if cache["parsed"].(float64) < 1 || cache["dropped"].(float64) < 1 {
		t.Fatal("changed/narrowed cache did not invalidate", cache)
	}
	other, otherRev := mainContextFixture(t)
	r := mainContextRead(other, otherRev, []string{"src/one.go"})
	if !r.OK || mainContextData(t, r)["cache"].(map[string]any)["reused"].(float64) != 0 {
		t.Fatal("cross-root index reuse", r)
	}
}

func TestMainContextObservedSourceDrift(t *testing.T) {
	root, rev := mainContextFixture(t)
	defer func() { contextTestAfterRead = nil }()
	contextTestAfterRead = func() {
		if err := os.WriteFile(filepath.Join(root, "src", "one.go"), []byte("package fixture\nfunc compute() int { return 9 }\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r := mainContextRead(root, rev, []string{"src/one.go"})
	if r.OK || r.Code != "CONTEXT_CHANGED" {
		t.Fatal("observed read-window drift not refused", r)
	}
	d := mainContextData(t, r)
	if selected, ok := d["selected"].([]any); ok && len(selected) > 0 {
		t.Fatal("drift exposed old excerpts", d)
	}
	contextTestAfterRead = nil
	r = mainContextRead(root, rev, []string{"src/one.go"})
	if !r.OK || mainContextData(t, r)["cache"].(map[string]any)["reused"].(float64) != 0 {
		t.Fatal("failed retrieval retained cache authority", r)
	}
}

func TestMainContextInstallationScope(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, homeMarker), installationLayoutJSON, 0600); err != nil {
		t.Fatal(err)
	}
	invoke := func(action string, rev int64, p any) Response {
		raw, _ := json.Marshal(p)
		return Handle(Request{SchemaVersion: 1, ProjectRoot: root, RootKind: "installation", Action: action, Actor: "main", RequestID: "home-context-" + action, ExpectedRevision: rev, Payload: raw})
	}
	r := invoke("init", 0, map[string]any{"projectId": "home-context", "deviceId": "device", "copyId": "copy", "owner": "main"})
	if !r.OK {
		t.Fatal(r)
	}
	scope := filepath.Join(root, "OCMI_Sync", "coordinator")
	if err := os.MkdirAll(scope, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"contract.md", "input.txt", "main.go"} {
		if err := os.WriteFile(filepath.Join(scope, name), []byte("package fixture\nfunc compute() int { return 1 }\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r = invoke("task.bind", 0, map[string]any{"taskId": "home-task", "outcomeId": "home-outcome", "taskRevision": 1, "completionScope": "target", "worker": "worker", "reviewer": "main", "contract": "OCMI_Sync/coordinator/contract.md", "inputs": []string{"OCMI_Sync/coordinator/input.txt"}, "command": map[string]any{"id": "check", "executable": os.Args[0], "args": []string{"-test.run=^$"}, "cwd": "OCMI_Sync/coordinator", "timeoutMs": 3000, "outputLimitBytes": 1024}})
	if !r.OK {
		t.Fatal(r)
	}
	rev := r.Revision
	if err := os.WriteFile(filepath.Join(root, "private.json"), []byte("compute PRIVATE-HOME-CANARY"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "private.md"), []byte("compute PRIVATE-HOME-CANARY"), 0600); err != nil {
		t.Fatal(err)
	}
	r = invoke("context.inspect", rev, map[string]any{"mode": "selected", "files": []string{"OCMI_Sync/coordinator/main.go"}, "terms": []string{"compute"}})
	if !r.OK || mainContextData(t, r)["provenance"].(map[string]any)["rootKind"] != "installation" {
		t.Fatal(r)
	}
	for _, name := range []string{"local/state/state.sqlite", "OCMI_Sync/personal/other.md", "OCMI_Sync/recovery/other.md", "private.json", "private.md"} {
		r = invoke("context.inspect", rev, map[string]any{"mode": "selected", "files": []string{name}, "terms": []string{"compute"}})
		if r.OK || r.Code != "CONTEXT_SCOPE_REFUSED" {
			t.Fatal("home private scope widened", name, r)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".ocmi")); !os.IsNotExist(err) {
		t.Fatal("parallel project state created", err)
	}
}

func TestMainContextLinkAndSummaryIsolation(t *testing.T) {
	root, rev := mainContextFixture(t)
	outside := t.TempDir()
	target := filepath.Join(outside, "foreign.go")
	if err := os.WriteFile(target, []byte("FOREIGN-CANARY-DO-NOT-READ"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "src", "linked.go")
	if err := os.Symlink(target, link); err != nil {
		t.Logf("ordinary symlink unavailable: %v", err)
	} else {
		r := mainContextRead(root, rev, []string{"src/linked.go"})
		if r.OK || r.Code != "CONTEXT_SCOPE_REFUSED" {
			t.Fatal("foreign link admitted", r)
		}
		raw, _ := json.Marshal(r)
		if strings.Contains(string(raw), "FOREIGN-CANARY") {
			t.Fatal("foreign source exposed")
		}
	}
	t.Run("directory-link-metadata", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(outside, "oversized.go"), make([]byte, (128<<10)+1), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "src", "outside")); err != nil {
			t.Skipf("native directory symlink unavailable: %v", err)
		}
		r := mainContextRead(root, rev, []string{"src/outside/oversized.go"})
		if r.OK || r.Code != "CONTEXT_SCOPE_REFUSED" {
			t.Fatal("foreign directory was treated as a legitimate size omission", r)
		}
		if selected, ok := mainContextData(t, r)["selected"].([]any); ok && len(selected) > 0 {
			t.Fatal("unsafe parent link returned excerpts")
		}
	})
	// Summary must not read even a now-unavailable required original.
	if err := os.Rename(filepath.Join(root, "input.txt"), filepath.Join(root, "input.saved")); err != nil {
		t.Fatal(err)
	}
	r := request(root, "context.inspect", "main", rev, map[string]any{})
	if !r.OK || mainContextData(t, r)["inputFreshness"] != "not-read" {
		t.Fatal("summary read task files", r)
	}
}
