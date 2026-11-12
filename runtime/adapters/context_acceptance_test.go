package adapters

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestContextAcceptanceNativeMCP(t *testing.T) {
	binary := os.Getenv("OCMI_ROOT_BINARY")
	if binary == "" {
		t.Skip("actual compiled CLI is supplied after build by the acceptance runner")
	}
	root := t.TempDir()
	for _, name := range []string{"contract.md", "input.txt", "main.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package fixture\nfunc compute() int { return 1 }\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "context-check", Version: "v1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: exec.Command(binary, "mcp", "--project-root", root, "--actor", "main")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	serial := 0
	call := func(action string, payload map[string]any, rev float64, extra map[string]any, ok bool) map[string]any {
		serial++
		args := map[string]any{"action": action, "requestId": time.Now().Format(time.RFC3339Nano), "expectedRevision": rev, "payload": payload}
		for key, value := range extra {
			args[key] = value
		}
		r, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "ocmi_request", Arguments: args})
		if err != nil || r.IsError == ok {
			t.Fatalf("%s result=%+v error=%v", action, r, err)
		}
		encoded, err := json.Marshal(r.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var response map[string]any
		if err = json.Unmarshal(encoded, &response); err != nil {
			t.Fatal(err)
		}
		return response
	}
	call("init", map[string]any{"projectId": "context-mcp", "deviceId": "device", "copyId": "copy", "owner": "main"}, 0, nil, true)
	r := call("task.bind", map[string]any{"taskId": "context-task", "outcomeId": "context-outcome", "taskRevision": 1, "completionScope": "target", "worker": "worker", "reviewer": "main", "contract": "contract.md", "inputs": []string{"input.txt"}, "command": map[string]any{"id": "check", "executable": binary, "args": []string{"request"}, "cwd": ".", "timeoutMs": 3000, "outputLimitBytes": 1024}}, 0, nil, true)
	r = call("stop", map[string]any{"enabled": true}, r["revision"].(float64), nil, true)
	revision := r["revision"].(float64)
	payload := map[string]any{"mode": "selected", "files": []string{"main.go"}, "terms": []string{"compute"}}
	call("context.inspect", payload, revision, nil, true)
	r = call("context.inspect", payload, revision, nil, true)
	d := r["data"].(map[string]any)
	if d["cache"].(map[string]any)["reused"].(float64) < 1 || d["state"].(map[string]any)["stopped"] != true {
		t.Fatal("bound session lost cache or STOP", d)
	}
	r = call("context.inspect", map[string]any{}, revision, nil, true)
	summary := r["data"].(map[string]any)["summary"].(string)
	call("context.inspect", payload, revision, map[string]any{"projectRoot": t.TempDir()}, false)
	// Startup must reuse the core summary and must not open required source text.
	if err := os.Rename(filepath.Join(root, "input.txt"), filepath.Join(root, "input.saved")); err != nil {
		t.Fatal(err)
	}
	hook := exec.Command(binary, "hook", "--project-root", root, "--actor", "main")
	hook.Stdin = strings.NewReader(`{"hook_event_name":"SessionStart"}`)
	out, err := hook.Output()
	if err != nil {
		t.Fatal(err)
	}
	var event map[string]any
	if json.Unmarshal(out, &event) != nil {
		t.Fatal("bad hook JSON")
	}
	if event["hookSpecificOutput"].(map[string]any)["additionalContext"] != summary {
		t.Fatal("hook duplicates/drifts from native summary", string(out), summary)
	}
	hook = exec.Command(binary, "hook", "--project-root", root, "--actor", "main")
	hook.Stdin = strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"mcp__ocmi__ocmi_request","tool_input":{"action":"context.inspect","payload":{}}}`)
	out, err = hook.Output()
	if err != nil || strings.Contains(string(out), `"permissionDecision":"deny"`) {
		t.Fatal("read-only context denied under STOP", string(out), err)
	}
}
