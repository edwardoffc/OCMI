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

func TestCompactAcceptanceNativeMCP(t *testing.T) {
	bin := os.Getenv("OCMI_ROOT_BINARY")
	if bin == "" {
		t.Skip("actual native consumer supplied by main")
	}
	root := t.TempDir()
	for name, body := range map[string]string{"criteria.md": "semantic criteria", "input.txt": "whole original", "rules.md": "RETAINED-MCP-RULE", ".gitignore": "PRIVATE-MCP-CONFIG"} {
		if e := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "compact-check", Version: "v1"}, nil)
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
	call("init", map[string]any{"projectId": "project", "deviceId": "device", "copyId": "copy", "owner": "main"}, 0, true)
	r := call("task.bind", map[string]any{"taskId": "small", "taskRevision": 1, "outcomeId": "outcome", "completionScope": "target", "worker": "worker", "reviewer": "main", "contract": "criteria.md", "inputs": []string{"input.txt", ".gitignore"}, "command": map[string]any{"id": "check", "executable": bin, "args": []string{"--help"}, "cwd": ".", "timeoutMs": 5000, "outputLimitBytes": 1024}}, 0, true)
	r = call("task.ready", map[string]any{}, r["revision"].(float64), true)
	rev := r["revision"].(float64)
	start := call("context.inspect", map[string]any{"mode": "start", "taskId": "small", "taskRevision": 1, "worker": "worker"}, rev, true)
	ref := start["data"].(map[string]any)["bindingRef"]
	metadata, _ := json.Marshal(start["data"].(map[string]any)["metadataOnlyFiles"])
	if !strings.Contains(string(metadata), ".gitignore") {
		t.Fatal("MCP metadata-only source policy missing", start)
	}
	refused := call("context.inspect", map[string]any{"mode": "compact", "requiredFiles": []string{".gitignore"}}, rev, false)
	if refused["code"] != "CONTEXT_METADATA_ONLY" || refused["data"].(map[string]any)["context"] != nil {
		t.Fatal("MCP lost actionable metadata refusal", refused)
	}
	p := map[string]any{"mode": "compact", "files": []string{"rules.md"}, "requiredFiles": []string{"input.txt"}, "expectedBinding": ref, "budgetBytes": 4096}
	r = call("context.inspect", p, rev, true)
	d := r["data"].(map[string]any)
	if r["code"] != "CONTEXT_COMPACT" || d["bindingRef"] != ref || !strings.Contains(d["context"].(string), "RETAINED-MCP-RULE") || strings.Contains(d["context"].(string), "PRIVATE-MCP-CONFIG") {
		t.Fatal("MCP compact source/privacy", r)
	}
	p["receipt"] = d["receipt"]
	r = call("context.inspect", p, rev, true)
	d = r["data"].(map[string]any)
	if d["receiptUnchanged"] != true || strings.Contains(d["context"].(string), "RETAINED-MCP-RULE") {
		t.Fatal("MCP receipt reuse", r)
	}
	if e := os.WriteFile(filepath.Join(root, "rules.md"), []byte("CHANGED-MCP-RULE"), 0600); e != nil {
		t.Fatal(e)
	}
	r = call("context.inspect", p, rev, true)
	if !strings.Contains(r["data"].(map[string]any)["context"].(string), "CHANGED-MCP-RULE") {
		t.Fatal("MCP stale receipt hid changed rule", r)
	}
	r = call("stop", map[string]any{"enabled": true}, rev, true)
	rev = r["revision"].(float64)
	call("context.inspect", p, rev, false)
	delete(p, "expectedBinding")
	r = call("context.inspect", p, rev, true)
	if r["data"].(map[string]any)["state"].(map[string]any)["stopped"] != true {
		t.Fatal("STOP observation hidden", r)
	}
	hook := exec.Command(bin, "hook", "--project-root", root, "--actor", "main")
	hook.Stdin = strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"mcp__ocmi__ocmi_request","tool_input":{"action":"context.inspect","payload":{"mode":"compact","files":["rules.md"]}}}`)
	out, e := hook.Output()
	if e != nil || strings.Contains(string(out), `"permissionDecision":"deny"`) {
		t.Fatal("read-only compact hook admission", e, string(out))
	}
}
