package internal

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// START exposes binding references, not source contents. Root public Git
// configuration must not make an otherwise ready assignment unusable.
func TestMainWorkflowBoundConfigNativeCLI(t *testing.T) {
	for _, name := range []string{".gitignore", ".gitattributes", ".env", ".private/input.txt", ".git/config"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			r := request(root, "init", "main", 0, map[string]any{"projectId": "project", "deviceId": "device", "copyId": "copy", "owner": "main"})
			if !r.OK {
				t.Fatal(r)
			}
			if e := os.WriteFile(filepath.Join(root, "criteria.md"), []byte("bounded outcome"), 0600); e != nil {
				t.Fatal(e)
			}
			p := filepath.Join(root, filepath.FromSlash(name))
			if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, []byte("CONFIG-PRIVATE-CANARY\n"), 0600); e != nil {
				t.Fatal(e)
			}
			r = request(root, "task.bind", "main", r.Revision, map[string]any{"taskId": "small", "taskRevision": 1, "outcomeId": "outcome", "completionScope": "target", "worker": "worker", "reviewer": "main", "contract": "criteria.md", "inputs": []string{name}, "command": map[string]any{"id": "bounded-check", "executable": os.Args[0], "args": []string{"--ocmi-cli-test-helper", "0", root, "0"}, "cwd": ".", "timeoutMs": 5000, "outputLimitBytes": 1024}})
			if !r.OK {
				t.Fatal("bound-input premise", r)
			}
			r = request(root, "task.ready", "main", r.Revision, map[string]any{})
			if !r.OK {
				t.Fatal("ready premise", r)
			}
			rev := r.Revision
			before := request(root, "status", "main", 0, map[string]any{})
			r = request(root, "context.inspect", "main", rev, workflowStartPayload())
			public := name == ".gitignore" || name == ".gitattributes"
			if r.OK != public || (public && (r.Code != "CONTEXT_START" || !strings.Contains(r.Data["start"].(string), name))) {
				t.Fatal("bound root config START", r)
			}
			if strings.Contains(string(workflowJSON(t, r)), "CONFIG-PRIVATE-CANARY") {
				t.Fatal("config content disclosure")
			}
			if binary := os.Getenv("OCMI_ROOT_BINARY"); binary != "" {
				out, err := exec.Command(binary, "call", "--root", root, "--actor", "main", "--action", "context.inspect", "--payload", string(workflowJSON(t, workflowStartPayload()))).CombinedOutput()
				var native Response
				if json.Unmarshal(out, &native) != nil || native.OK != public || (public && (err != nil || native.Code != "CONTEXT_START" || !strings.Contains(string(out), name))) || strings.Contains(string(out), "CONFIG-PRIVATE-CANARY") {
					t.Fatal("native bound-config boundary", err, string(out))
				}
			}
			selected := request(root, "context.inspect", "main", rev, map[string]any{"mode": "selected", "requiredFiles": []string{name}, "budgetBytes": 1024})
			if selected.OK || strings.Contains(string(workflowJSON(t, selected)), "CONFIG-PRIVATE-CANARY") {
				t.Fatal("source disclosure scope broadened", selected)
			}
			if public {
				if e := os.WriteFile(p, []byte("changed\n"), 0600); e != nil {
					t.Fatal(e)
				}
				changed := request(root, "context.inspect", "main", rev, workflowStartPayload())
				if changed.OK || changed.Data["start"] != nil {
					t.Fatal("stale config binding escaped", changed)
				}
			}
			after := request(root, "status", "main", 0, map[string]any{})
			if string(workflowJSON(t, before)) != string(workflowJSON(t, after)) {
				t.Fatal("view changed state")
			}
		})
	}
}
