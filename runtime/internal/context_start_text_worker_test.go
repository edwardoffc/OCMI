package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkerContextTextStartReferences(t *testing.T) {
	root := t.TempDir()
	r := request(root, "init", "main", 0, map[string]any{"projectId": "p", "deviceId": "d", "copyId": "c", "owner": "main"})
	if !r.OK {
		t.Fatal(r)
	}
	names := []string{"one.md", "two.md", "three.md", "four.md", "five.md", ".gitignore"}
	for _, name := range append([]string{"criteria.md"}, names...) {
		if e := os.WriteFile(filepath.Join(root, name), []byte("bounded source"), 0600); e != nil {
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
	r = request(root, "context.inspect", "main", r.Revision, workflowStartPayload())
	if !r.OK {
		t.Fatal(r)
	}
	start := r.Data["start"].(string)
	if strings.Count(start, "criteria.md") != 1 || !strings.Contains(start, "expectedBinding=") || !strings.Contains(start, "main must deliver") || !strings.Contains(start, "required originals") {
		t.Fatal(start)
	}
	for _, required := range []string{"Ready START", "verified", "small", "worker", "observed global revision", "Root ", "Contract ", "Required sources:", "Metadata-only", "omit text only if Contract needs none", "Checker ", "identity/state", "Before writes", "whole Contract", "only bound checker"} {
		if !strings.Contains(start, required) {
			t.Fatal("missing semantic guard", required, start)
		}
	}
	for _, name := range names {
		if !strings.Contains(start, name) {
			t.Fatal(name, start)
		}
	}
	t.Logf("six-input START bytes=%d", len([]byte(start)))
}
