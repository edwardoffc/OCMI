package internal

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainCompactMetadataPolicy(t *testing.T) {
	root, rev := compactFixture(t)
	before := request(root, "status", "main", 0, map[string]any{})
	start := request(root, "context.inspect", "main", rev, workflowStartPayload())
	if !start.OK || !strings.Contains(string(workflowJSON(t, start.Data["metadataOnlyFiles"])), ".gitignore") {
		t.Fatal("no machine-readable metadata-only source policy", start)
	}
	text := start.Data["start"].(string)
	policy := strings.Index(text, "Metadata-only")
	if policy < 0 || strings.Contains(text[:policy], ".gitignore") || !strings.Contains(text[policy:], ".gitignore") {
		t.Fatal("bound config still presented as required text", start)
	}
	p := compactPayload()
	p["requiredFiles"] = []string{"input.txt", ".gitignore"}
	r := request(root, "context.inspect", "worker", rev, p)
	if r.OK || r.Code != "CONTEXT_METADATA_ONLY" || !strings.Contains(string(workflowJSON(t, r.Data["metadataOnlyFiles"])), ".gitignore") || r.Data["context"] != nil || strings.Contains(string(workflowJSON(t, r)), "CANARY") {
		t.Fatal("no safe actionable metadata-only refusal", r)
	}
	p["requiredFiles"] = []string{"input.txt"}
	p["expectedBinding"] = start.Data["bindingRef"]
	r = request(root, "context.inspect", "worker", rev, p)
	if !r.OK || r.Data["inputFreshness"] != "verified" || !strings.Contains(r.Data["context"].(string), "BOUND-COMPACT-CANARY") || strings.Contains(r.Data["context"].(string), "CONFIG-COMPACT-PRIVATE-CANARY") {
		t.Fatal("supported local recovery failed", r)
	}
	after := request(root, "status", "main", 0, map[string]any{})
	if string(workflowJSON(t, before)) != string(workflowJSON(t, after)) {
		t.Fatal("read policy changed task/state")
	}
}

func TestMainCompactMetadataNativeCLI(t *testing.T) {
	bin := os.Getenv("OCMI_ROOT_BINARY")
	if bin == "" {
		t.Skip("actual native core supplied by main")
	}
	root, rev := compactFixture(t)
	call := func(actor string, p map[string]any, ok bool) Response {
		out, e := exec.Command(bin, "call", "--root", root, "--actor", actor, "--action", "context.inspect", "--payload", string(workflowJSON(t, p))).CombinedOutput()
		var r Response
		if json.Unmarshal(out, &r) != nil || r.OK != ok || (ok && e != nil) || r.Revision != rev {
			t.Fatal("actual source-policy CLI", e, string(out))
		}
		return r
	}
	start := call("main", workflowStartPayload(), true)
	if !strings.Contains(string(workflowJSON(t, start.Data["metadataOnlyFiles"])), ".gitignore") {
		t.Fatal("CLI source policy missing", start)
	}
	p := compactPayload()
	p["requiredFiles"] = []string{".gitignore"}
	if r := call("worker", p, false); r.Code != "CONTEXT_METADATA_ONLY" {
		t.Fatal("unactionable CLI refusal", r)
	}
	p["requiredFiles"] = []string{"input.txt"}
	p["expectedBinding"] = start.Data["bindingRef"]
	if r := call("worker", p, true); r.Data["inputFreshness"] != "verified" {
		t.Fatal(r)
	}
}

func TestMainCompactMetadataContract(t *testing.T) {
	root := t.TempDir()
	r := request(root, "init", "main", 0, map[string]any{"projectId": "project", "deviceId": "device", "copyId": "copy", "owner": "main"})
	if !r.OK {
		t.Fatal(r)
	}
	if e := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("METADATA-CONTRACT-PRIVATE-CANARY"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(root, "input.txt"), []byte("ordinary original"), 0600); e != nil {
		t.Fatal(e)
	}
	r = request(root, "task.bind", "main", r.Revision, map[string]any{"taskId": "small", "taskRevision": 1, "outcomeId": "outcome", "completionScope": "target", "worker": "worker", "reviewer": "main", "contract": ".gitignore", "inputs": []string{"input.txt"}, "command": map[string]any{"id": "check", "executable": os.Args[0], "args": []string{"--ocmi-cli-test-helper", "0", root, "0"}, "cwd": ".", "timeoutMs": 5000, "outputLimitBytes": 1024}})
	if !r.OK {
		t.Fatal("bind premise", r)
	}
	r = request(root, "task.ready", "main", r.Revision, map[string]any{})
	if !r.OK {
		t.Fatal("ready premise", r)
	}
	for _, payload := range []map[string]any{workflowStartPayload(), {"mode": "compact"}} {
		t.Run(payload["mode"].(string), func(t *testing.T) {
			view := request(root, "context.inspect", "worker", r.Revision, payload)
			if view.OK || view.Data["context"] != nil || view.Data["start"] != nil || strings.Contains(string(workflowJSON(t, view)), "PRIVATE-CANARY") {
				t.Fatal("metadata-only Contract escaped source policy", view)
			}
		})
	}
}
