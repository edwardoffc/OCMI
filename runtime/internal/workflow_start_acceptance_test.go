package internal

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func workflowFixture(t *testing.T) (string, int64) {
	t.Helper()
	root := t.TempDir()
	r := request(root, "init", "main", 0, map[string]any{"projectId": "project", "deviceId": "device", "copyId": "copy", "owner": "main"})
	if !r.OK {
		t.Fatal(r)
	}
	for name, body := range map[string]string{"criteria.md": "SEMANTIC-PRIVATE-CANARY: retain one bounded outcome", "input.txt": "INPUT-PRIVATE-CANARY"} {
		if e := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	r = request(root, "task.bind", "main", r.Revision, map[string]any{"taskId": "small", "taskRevision": 1, "outcomeId": "outcome", "completionScope": "target", "worker": "worker", "reviewer": "main", "contract": "criteria.md", "inputs": []string{"input.txt"}, "command": map[string]any{"id": "bounded-check", "executable": os.Args[0], "args": []string{"--ocmi-cli-test-helper", "0", root, "0"}, "cwd": ".", "timeoutMs": 5000, "outputLimitBytes": 1024}})
	if !r.OK {
		t.Fatal(r)
	}
	r = request(root, "task.ready", "main", r.Revision, map[string]any{})
	if !r.OK {
		t.Fatal(r)
	}
	return root, r.Revision
}

func workflowStartPayload() map[string]any {
	return map[string]any{"mode": "start", "taskId": "small", "taskRevision": 1, "worker": "worker"}
}

func TestMainWorkflowPremise(t *testing.T) {
	root, rev := workflowFixture(t)
	r := request(root, "context.inspect", "main", rev, map[string]any{"mode": "summary"})
	if !r.OK || r.Revision != rev {
		t.Fatal(r)
	}
	r = request(root, "context.inspect", "worker", rev, map[string]any{"mode": "selected", "requiredFiles": []string{"criteria.md", "input.txt"}, "budgetBytes": 1024})
	if !r.OK || !strings.Contains(string(workflowJSON(t, r.Data)), "SEMANTIC-PRIVATE-CANARY") {
		t.Fatal("ordinary bound source control failed", r)
	}
}

func workflowJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return raw
}

func TestMainWorkflowStart(t *testing.T) {
	root, rev := workflowFixture(t)
	before := request(root, "status", "main", 0, map[string]any{})
	for _, actor := range []string{"main", "worker"} {
		r := request(root, "context.inspect", actor, rev, workflowStartPayload())
		start, _ := r.Data["start"].(string)
		if !r.OK || r.Code != "CONTEXT_START" || r.Revision != rev || start == "" || r.Data["deliveryRequired"] != true || r.Data["inputFreshness"] != "verified" {
			t.Fatal("no usable observed ready START", r)
		}
		for _, want := range []string{"small", "worker", "criteria.md", root} {
			if !strings.Contains(start, want) {
				t.Fatal("missing addressed reference", want, r)
			}
		}
		raw := workflowJSON(t, r)
		if len(raw) > 4096 || strings.Contains(string(raw), "PRIVATE-CANARY") || strings.Contains(string(raw), "--ocmi-cli-test-helper") {
			t.Fatal("bloated or raw content/command disclosure", len(raw), r)
		}
	}
	after := request(root, "status", "main", 0, map[string]any{})
	if string(workflowJSON(t, before)) != string(workflowJSON(t, after)) {
		t.Fatal("view changed state")
	}
}

func TestMainWorkflowStartRefusals(t *testing.T) {
	for _, kind := range []string{"actor", "task", "task-revision", "worker", "global-revision", "stop", "input-drift", "not-ready", "extra-scope", "missing-identity"} {
		t.Run(kind, func(t *testing.T) {
			root, rev := workflowFixture(t)
			p := workflowStartPayload()
			actor := "main"
			expected := rev
			switch kind {
			case "actor":
				actor = "unassigned"
			case "task":
				p["taskId"] = "other"
			case "task-revision":
				p["taskRevision"] = 2
			case "worker":
				p["worker"] = "other"
			case "global-revision":
				expected = rev + 1
			case "stop":
				r := request(root, "stop", "main", rev, map[string]any{"enabled": true})
				if !r.OK {
					t.Fatal(r)
				}
				expected = r.Revision
			case "input-drift":
				if e := os.WriteFile(filepath.Join(root, "input.txt"), []byte("changed"), 0600); e != nil {
					t.Fatal(e)
				}
			case "not-ready":
				r := request(root, "run", "worker", rev, map[string]any{})
				if !r.OK {
					t.Fatal(r)
				}
				expected = r.Revision
			case "extra-scope":
				p["files"] = []string{"outside.txt"}
			case "missing-identity":
				delete(p, "taskId")
			}
			r := request(root, "context.inspect", actor, expected, p)
			if r.OK || r.Data["start"] != nil {
				t.Fatal("unsafe START escaped", kind, r)
			}
		})
	}
}

func TestMainWorkflowNativeCLI(t *testing.T) {
	bin := os.Getenv("OCMI_ROOT_BINARY")
	if bin == "" {
		t.Skip("built native consumer supplied by main")
	}
	root, rev := workflowFixture(t)
	payload := string(workflowJSON(t, workflowStartPayload()))
	out, e := exec.Command(bin, "call", "--root", root, "--actor", "main", "--action", "context.inspect", "--payload", payload).CombinedOutput()
	var r Response
	if e != nil || json.Unmarshal(out, &r) != nil || !r.OK || r.Code != "CONTEXT_START" || r.Revision != rev || r.Data["start"] == nil {
		t.Fatal("native START consumer", e, string(out))
	}
	out, e = exec.Command(bin, "help", "task.bind").CombinedOutput()
	if e != nil || json.Unmarshal(out, &r) != nil || !r.OK || r.Data["optionalCommandFields"] == nil {
		t.Fatal("native discoverable schema", e, string(out))
	}
}
