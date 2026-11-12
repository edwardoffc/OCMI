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

func TestWorkflowAcceptanceNativeMCP(t *testing.T) {
	bin := os.Getenv("OCMI_ROOT_BINARY")
	if bin == "" {
		t.Skip("actual native core supplied after build")
	}
	root := t.TempDir()
	for _, name := range []string{"criteria.md", "input.txt"} {
		if e := os.WriteFile(filepath.Join(root, name), []byte("PRIVATE-WORKFLOW-CANARY"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "workflow-check", Version: "v1"}, nil)
	session, e := client.Connect(ctx, &mcp.CommandTransport{Command: exec.Command(bin, "mcp", "--project-root", root, "--actor", "main")}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer session.Close()
	call := func(action string, payload map[string]any, rev float64, ok bool) map[string]any {
		out, e := session.CallTool(ctx, &mcp.CallToolParams{Name: "ocmi_request", Arguments: map[string]any{"action": action, "expectedRevision": rev, "requestId": time.Now().Format(time.RFC3339Nano), "payload": payload}})
		if e != nil || out.IsError == ok {
			t.Fatalf("%s: %v %+v", action, e, out)
		}
		raw, e := json.Marshal(out.StructuredContent)
		if e != nil {
			t.Fatal(e)
		}
		var r map[string]any
		if json.Unmarshal(raw, &r) != nil {
			t.Fatal("invalid transport")
		}
		return r
	}
	call("init", map[string]any{"projectId": "workflow", "deviceId": "device", "copyId": "copy", "owner": "main"}, 0, true)
	r := call("task.bind", map[string]any{"taskId": "small", "taskRevision": 1, "outcomeId": "outcome", "completionScope": "target", "worker": "worker", "reviewer": "main", "contract": "criteria.md", "inputs": []string{"input.txt"}, "command": map[string]any{"id": "check", "executable": bin, "args": []string{"--help"}, "cwd": ".", "timeoutMs": 5000, "outputLimitBytes": 1024}}, 0, true)
	r = call("task.ready", map[string]any{}, r["revision"].(float64), true)
	rev := r["revision"].(float64)
	payload := map[string]any{"mode": "start", "taskId": "small", "taskRevision": 1, "worker": "worker"}
	r = call("context.inspect", payload, rev, true)
	d := r["data"].(map[string]any)
	start, _ := d["start"].(string)
	if r["code"] != "CONTEXT_START" || r["revision"] != rev || start == "" || d["deliveryRequired"] != true || strings.Contains(start, "PRIVATE-WORKFLOW-CANARY") {
		t.Fatal("MCP did not preserve derived-only assignment", r)
	}
	r = call("stop", map[string]any{"enabled": true}, rev, true)
	call("context.inspect", payload, r["revision"].(float64), false)
	hook := exec.Command(bin, "hook", "--project-root", root, "--actor", "main")
	hook.Stdin = strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"mcp__ocmi__ocmi_request","tool_input":{"action":"context.inspect","payload":{"mode":"start","taskId":"small","taskRevision":1,"worker":"worker"}}}`)
	out, e := hook.Output()
	if e != nil || strings.Contains(string(out), `"permissionDecision":"deny"`) {
		t.Fatal("read-only hook admission changed", e, string(out))
	}
}
