package internal

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkerCompactMetadataScopeAndClosedContract(t *testing.T) {
	root, rev := compactFixture(t)
	for _, payload := range []map[string]any{
		{"mode": "compact", "files": []string{".gitignore"}},
		{"mode": "compact", "requiredFiles": []string{".gitattributes"}},
		{"mode": "compact", "files": []string{".gitignore", ".env"}},
	} {
		r := request(root, "context.inspect", "worker", rev, payload)
		if r.OK || r.Data["context"] != nil || r.Data["inputFreshness"] != nil {
			t.Fatal(r)
		}
		if files, ok := payload["files"].([]string); ok && len(files) == 2 && r.Code != "CONTEXT_SCOPE_REFUSED" {
			t.Fatal(r)
		}
	}
	// Bind a metadata-only Contract, then close through the real lifecycle.
	root = t.TempDir()
	r := request(root, "init", "main", 0, map[string]any{"projectId": "p", "deviceId": "d", "copyId": "c", "owner": "main"})
	if !r.OK {
		t.Fatal(r)
	}
	for name, content := range map[string]string{".gitignore": "CONTRACT-CANARY", "input.txt": "regular"} {
		if e := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); e != nil {
			t.Fatal(e)
		}
	}
	r = request(root, "task.bind", "main", r.Revision, map[string]any{"taskId": "t", "taskRevision": 1, "outcomeId": "o", "completionScope": "target", "worker": "worker", "reviewer": "main", "contract": ".gitignore", "inputs": []string{"input.txt"}, "command": map[string]any{"id": "check", "executable": os.Args[0], "args": []string{"--ocmi-cli-test-helper", "0", root, "0"}, "cwd": ".", "timeoutMs": 5000, "outputLimitBytes": 1024}})
	if !r.OK {
		t.Fatal(r)
	}
	for _, s := range []struct {
		action, actor string
		payload       map[string]any
	}{
		{"task.ready", "main", map[string]any{}}, {"run", "worker", map[string]any{}}, {"submit", "worker", map[string]any{}},
		{"review", "main", map[string]any{"accept": true, "reason": "checked"}}, {"integrate", "main", map[string]any{"candidate": "c", "reason": "checked"}}, {"resource.close", "main", map[string]any{"disposition": "keep", "reason": "retained"}},
	} {
		r = request(root, s.action, s.actor, r.Revision, s.payload)
		if !r.OK {
			t.Fatal(s.action, r)
		}
	}
	rev = r.Revision
	r = request(root, "context.inspect", "main", rev, map[string]any{"mode": "compact", "requiredFiles": []string{"input.txt"}})
	if !r.OK || strings.Contains(r.Data["context"].(string), "CONTRACT-CANARY") {
		t.Fatal(r)
	}
	r = request(root, "context.inspect", "main", rev, map[string]any{"mode": "compact", "files": []string{".gitignore"}})
	if r.OK || r.Code != "CONTEXT_METADATA_ONLY" || r.Data["context"] != nil {
		t.Fatal(r)
	}
}

func TestWorkerCompactHomeScope(t *testing.T) {
	root := t.TempDir()
	if e := os.WriteFile(filepath.Join(root, homeMarker), installationLayoutJSON, 0600); e != nil {
		t.Fatal(e)
	}
	invoke := func(action string, rev int64, p any) Response {
		raw, _ := json.Marshal(p)
		return Handle(Request{SchemaVersion: 1, ProjectRoot: root, RootKind: "installation", Action: action, Actor: "main", RequestID: "compact-home-" + action, ExpectedRevision: rev, Payload: raw})
	}
	r := invoke("init", 0, map[string]any{"projectId": "home", "deviceId": "device", "copyId": "copy", "owner": "main"})
	if !r.OK {
		t.Fatal(r)
	}
	scope := filepath.Join(root, "OCMI_Sync", "coordinator")
	if e := os.MkdirAll(scope, 0700); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"criteria.md", "input.txt"} {
		if e := os.WriteFile(filepath.Join(scope, name), []byte("safe"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(filepath.Join(root, "private.md"), []byte("HOME-PRIVATE-CANARY"), 0600); e != nil {
		t.Fatal(e)
	}
	r = invoke("task.bind", 0, map[string]any{"taskId": "small", "outcomeId": "outcome", "taskRevision": 1, "completionScope": "target", "worker": "worker", "reviewer": "main", "contract": "OCMI_Sync/coordinator/criteria.md", "inputs": []string{"OCMI_Sync/coordinator/input.txt"}, "command": map[string]any{"id": "check", "executable": os.Args[0], "args": []string{"-test.run=^$"}, "cwd": "OCMI_Sync/coordinator", "timeoutMs": 3000, "outputLimitBytes": 1024}})
	if !r.OK {
		t.Fatal(r)
	}
	r = invoke("task.ready", r.Revision, map[string]any{})
	if !r.OK {
		t.Fatal(r)
	}
	rev := r.Revision
	r = invoke("context.inspect", rev, map[string]any{"mode": "compact", "requiredFiles": []string{"OCMI_Sync/coordinator/input.txt"}})
	if !r.OK {
		t.Fatal("home positive", r)
	}
	r = invoke("context.inspect", rev, map[string]any{"mode": "compact", "files": []string{"private.md"}})
	raw, _ := json.Marshal(r)
	if r.OK || r.Data["context"] != nil || strings.Contains(string(raw), "HOME-PRIVATE-CANARY") {
		t.Fatal(r)
	}
}

func TestWorkerCompactThinStartSixInputs(t *testing.T) {
	root := t.TempDir()
	r := request(root, "init", "main", 0, map[string]any{"projectId": "p", "deviceId": "d", "copyId": "c", "owner": "main"})
	if !r.OK {
		t.Fatal(r)
	}
	names := []string{"one.md", "two.md", "three.md", "four.md", "five.md", ".gitignore"}
	for _, name := range append([]string{"criteria.md"}, names...) {
		if e := os.WriteFile(filepath.Join(root, name), []byte("fixture"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	r = request(root, "task.bind", "main", r.Revision, map[string]any{"taskId": "small", "outcomeId": "outcome", "taskRevision": 1, "completionScope": "target", "worker": "worker", "reviewer": "main", "contract": "criteria.md", "inputs": names, "command": map[string]any{"id": "check", "executable": os.Args[0], "args": []string{"-test.run=^$"}, "cwd": ".", "timeoutMs": 3000, "outputLimitBytes": 1024}})
	if !r.OK {
		t.Fatal(r)
	}
	r = request(root, "task.ready", "main", r.Revision, map[string]any{})
	if !r.OK {
		t.Fatal(r)
	}
	rev := r.Revision
	r = request(root, "context.inspect", "main", rev, workflowStartPayload())
	start, _ := r.Data["start"].(string)
	ref, _ := r.Data["bindingRef"].(string)
	if !r.OK || len([]byte(start)) > 700 || !strings.Contains(start, "compact expectedBinding="+ref) || !strings.Contains(start, "criteria.md") || !strings.Contains(start, root) || !strings.Contains(start, "observed global revision") {
		t.Fatal(r, len([]byte(start)))
	}
	for _, name := range names {
		if !strings.Contains(start, name) {
			t.Fatal("missing required path", name, r)
		}
	}
	if !strings.Contains(start, "Metadata-only") || !strings.Contains(string(workflowJSON(t, r.Data["metadataOnlyFiles"])), ".gitignore") {
		t.Fatal(r)
	}
	t.Logf("six-input START bytes=%d", len([]byte(start)))
}

func TestWorkerCompactReceiptBoundaries(t *testing.T) {
	root, rev := compactFixture(t)
	db, e := openExisting(root)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	before := historyStored(t, db)
	dirBefore, e := os.ReadDir(root)
	if e != nil {
		t.Fatal(e)
	}
	p := compactPayload()
	r := request(root, "context.inspect", "worker", rev, p)
	if !r.OK {
		t.Fatal(r)
	}
	dirAfter, e := os.ReadDir(root)
	if e != nil || len(dirAfter) != len(dirBefore) {
		t.Fatal("context changed root inventory", e)
	}
	for i := range dirBefore {
		if dirBefore[i].Name() != dirAfter[i].Name() {
			t.Fatal("context changed root inventory")
		}
	}
	token := r.Data["receipt"].(string)
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatal(err)
	}
	var malformed map[string]any
	if err = json.Unmarshal(decoded, &malformed); err != nil {
		t.Fatal(err)
	}
	malformed["files"] = nil
	encoded, err := json.Marshal(malformed)
	if err != nil {
		t.Fatal(err)
	}
	nullPayload := compactPayload()
	nullPayload["receipt"] = base64.RawURLEncoding.EncodeToString(encoded)
	if r = request(root, "context.inspect", "worker", rev, nullPayload); r.OK || r.Code != "INVALID_RECEIPT" || r.Data["context"] != nil {
		t.Fatal("null files accepted", r)
	}
	p["receipt"] = token
	if r = request(root, "context.inspect", "main", rev, p); r.OK {
		t.Fatal("foreign actor receipt", r)
	}
	other, otherRev := compactFixture(t)
	if r = request(other, "context.inspect", "worker", otherRev, p); r.OK {
		t.Fatal("foreign root receipt", r)
	}
	for _, bad := range []string{"x", strings.Repeat("A", 8193)} {
		p["receipt"] = bad
		if r = request(root, "context.inspect", "worker", rev, p); r.OK {
			t.Fatal("malformed receipt", r)
		}
	}
	p["receipt"] = token
	file := filepath.Join(root, "rules.md")
	raw, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(file, file+".old"); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(file, raw, 0600); e != nil {
		t.Fatal(e)
	}
	r = request(root, "context.inspect", "worker", rev, p)
	if !r.OK || !strings.Contains(r.Data["context"].(string), "RULE-COMPACT-CANARY") || r.Data["receipt"] == token {
		t.Fatal("native replacement hidden", r)
	}
	if historyStored(t, db) != before {
		t.Fatal("context modified stored history")
	}
}

func TestWorkerCompactClosedContract(t *testing.T) {
	root, rev := compactFixture(t)
	for _, step := range []struct {
		action, actor string
		payload       map[string]any
	}{
		{"run", "worker", map[string]any{}}, {"submit", "worker", map[string]any{}},
		{"review", "main", map[string]any{"accept": true, "reason": "reviewed"}},
		{"integrate", "main", map[string]any{"candidate": "candidate", "reason": "accepted"}},
		{"resource.close", "main", map[string]any{"disposition": "keep", "reason": "retained"}},
	} {
		r := request(root, step.action, step.actor, rev, step.payload)
		if !r.OK {
			t.Fatal(step.action, r)
		}
		rev = r.Revision
	}
	p := map[string]any{"mode": "compact", "files": []string{"rules.md"}}
	r := request(root, "context.inspect", "main", rev, p)
	if !r.OK || strings.Contains(r.Data["context"].(string), "SEMANTIC-COMPACT-CANARY") {
		t.Fatal(r)
	}
	p["requiredFiles"] = []string{"criteria.md"}
	r = request(root, "context.inspect", "main", rev, p)
	if !r.OK || !strings.Contains(r.Data["context"].(string), "SEMANTIC-COMPACT-CANARY") {
		t.Fatal(r)
	}
}
