package adapters

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/edwardoffc/OCMI/runtime/internal"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// This joins actual native transports on one fictional task. Synthetic hook
// events prove the hook's response, not client installation or enforcement.
func TestExecutionChannelsAcceptanceNativeTask(t *testing.T) {
	bin := os.Getenv("OCMI_ROOT_BINARY")
	if bin == "" {
		t.Skip("actual native core supplied by acceptance runner after build")
	}
	root, otherRoot := t.TempDir(), t.TempDir()
	for _, name := range []string{"contract.md", "input.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("Fictional execution-channel fixture."), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	cli := func(actor, action string, rev int64, payload map[string]any) internal.Response {
		t.Helper()
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(ctx, bin, "call", "--root", root, "--actor", actor, "--action", action, "--revision", strconv.FormatInt(rev, 10), "--payload", string(raw))
		out, err := cmd.CombinedOutput()
		var response internal.Response
		if err != nil || json.Unmarshal(out, &response) != nil || !response.OK {
			t.Fatalf("CLI %s: err=%v response=%s", action, err, out)
		}
		return response
	}
	connect := func(actor string) *mcp.ClientSession {
		t.Helper()
		client := mcp.NewClient(&mcp.Implementation{Name: "execution-channels-check", Version: "v1"}, nil)
		session, err := client.Connect(ctx, &mcp.CommandTransport{Command: exec.CommandContext(ctx, bin, "mcp", "--project-root", root, "--actor", actor)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = session.Close() })
		tools, err := session.ListTools(ctx, nil)
		if err != nil || len(tools.Tools) != 1 || tools.Tools[0].Name != "ocmi_request" {
			t.Fatalf("MCP %s tools: %v %+v", actor, err, tools)
		}
		return session
	}
	call := func(session *mcp.ClientSession, action string, rev int64, payload, overrides map[string]any, wantCode string) internal.Response {
		t.Helper()
		args := map[string]any{"action": action, "requestId": uuid.NewString(), "expectedRevision": rev, "payload": payload}
		for key, value := range overrides {
			args[key] = value
		}
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "ocmi_request", Arguments: args})
		if err != nil {
			t.Fatalf("MCP %s: %v", action, err)
		}
		response := decodeResponse(t, result)
		if response.Code != wantCode || result.IsError == response.OK {
			t.Fatalf("MCP %s: %+v isError=%v wantCode=%s", action, response, result.IsError, wantCode)
		}
		return response
	}
	empty := map[string]any{}
	r := cli("main", "init", 0, map[string]any{"projectId": "channels", "deviceId": "device", "copyId": "copy", "owner": "main"})
	owner, worker := connect("main"), connect("worker")
	r = cli("main", "task.bind", r.Revision, map[string]any{
		"taskId": "shared-task", "taskRevision": 1, "outcomeId": "shared-outcome", "completionScope": "target",
		"worker": "worker", "reviewer": "main", "contract": "contract.md", "inputs": []string{"input.txt"},
		"command": map[string]any{"id": "native-help", "executable": bin, "args": []string{"--help"}, "cwd": ".", "timeoutMs": 5000, "outputLimitBytes": 65536},
	})
	stale := r.Revision
	r = cli("main", "task.ready", r.Revision, empty)
	readyRevision := r.Revision
	status := func() internal.Response {
		s := cli("main", "status", 0, empty)
		delete(s.Data, "requestId")
		return s
	}
	assertUnchanged := func(before internal.Response) {
		t.Helper()
		if after := status(); !reflect.DeepEqual(before, after) {
			t.Fatalf("rejected/uncovered call changed status: before=%+v after=%+v", before, after)
		}
	}
	before := status()
	if before.Data["taskId"] != "shared-task" || before.Data["taskRevision"] != float64(1) || before.Data["taskState"] != "ready" {
		t.Fatal("CLI did not ready selected task", before)
	}
	for _, tc := range []struct {
		name, code string
		session    *mcp.ClientSession
		revision   int64
		overrides  map[string]any
	}{
		{"stale", "STALE_REVISION", worker, stale, nil},
		{"wrong-root", "BOUND_IDENTITY_DENIED", worker, readyRevision, map[string]any{"projectRoot": otherRoot}},
		{"wrong-actor", "BOUND_IDENTITY_DENIED", worker, readyRevision, map[string]any{"actor": "main"}},
		{"bound-owner-is-not-worker", "NOT_WORKER", owner, readyRevision, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := call(tc.session, "run", tc.revision, empty, tc.overrides, tc.code)
			if response.OK {
				t.Fatal("rejection admitted run", response)
			}
			assertUnchanged(before)
		})
	}
	r = cli("main", "stop", readyRevision, map[string]any{"enabled": true})
	stopped := status()
	if denied := call(worker, "run", r.Revision, empty, nil, "STOPPED"); denied.OK {
		t.Fatal("MCP bypassed CLI STOP", denied)
	}
	assertUnchanged(stopped)
	hook := func(tool string) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "tool_name": tool, "tool_input": map[string]any{"action": "run", "payload": empty}})
		cmd := exec.CommandContext(ctx, bin, "hook", "--project-root", root, "--actor", "worker")
		cmd.Stdin = strings.NewReader(string(raw))
		out, err := cmd.CombinedOutput()
		var response map[string]any
		if err != nil || json.Unmarshal(out, &response) != nil {
			t.Fatalf("hook %s: %v %s", tool, err, out)
		}
		return response
	}
	selected := hook("mcp__ocmi__ocmi_request")
	guard, ok := selected["hookSpecificOutput"].(map[string]any)
	if !ok || guard["permissionDecision"] != "deny" || !strings.Contains(guard["permissionDecisionReason"].(string), "STOP") {
		t.Fatal("selected hook did not deny STOP", selected)
	}
	assertUnchanged(stopped)
	for _, tool := range []string{"Bash", "Write", "spawn_agent"} {
		if response := hook(tool); !reflect.DeepEqual(response, map[string]any{"ocmi": "no_coverage"}) {
			t.Fatalf("unsupported hook %s claims coverage: %+v", tool, response)
		}
		assertUnchanged(stopped)
	}
	if _, err := os.Stat(filepath.Join(otherRoot, ".ocmi")); !os.IsNotExist(err) {
		t.Fatalf("wrong-root request touched other root: %v", err)
	}
	r = cli("main", "stop", r.Revision, map[string]any{"enabled": false})
	r = call(worker, "run", r.Revision, empty, nil, "RUN_RECORDED")
	if !r.OK || r.Data["operationState"] != "succeeded" {
		t.Fatal("native ordinary child did not succeed", r)
	}
	operationID, ok := r.Data["operationId"].(string)
	if !ok || operationID == "" {
		t.Fatal("native run omitted operation identity", r)
	}
	r = call(worker, "submit", r.Revision, empty, nil, "SUBMITTED")
	if !r.OK {
		t.Fatal(r)
	}
	r = cli("main", "review", r.Revision, map[string]any{"accept": true, "reason": "Reviewed fictional native command evidence."})
	r = cli("main", "integrate", r.Revision, map[string]any{"candidate": "fictional-channel-component", "reason": "Accepted selected task only."})
	r = cli("main", "resource.close", r.Revision, map[string]any{"disposition": "keep", "reason": "Retain fictional primary evidence until test cleanup."})
	final := call(owner, "status", 0, empty, nil, "STATUS")
	summary, ok := final.Data["resourceSummary"].(map[string]any)
	if !final.OK || final.Revision != r.Revision || final.Data["projectId"] != "channels" || final.Data["taskId"] != "shared-task" || final.Data["taskRevision"] != float64(1) || final.Data["taskState"] != "integrated" || final.Data["cleanup"] != "done" || final.Data["operationId"] != operationID || final.Data["operationState"] != "succeeded" || !ok || summary["pending"] != float64(0) || summary["activeUses"] != float64(0) || summary["legacyCoverageUnknown"] != false {
		t.Fatal("selected task/resources did not close", final)
	}
	if total, ok := summary["total"].(float64); !ok || total == 0 {
		t.Fatal("resource closure had no registered resources", final)
	}
	if entries, err := os.ReadDir(filepath.Join(root, ".ocmi", "local", "runs")); err != nil || len(entries) != 0 {
		t.Fatalf("owned run scratch remained: %v %v", entries, err)
	}
	for _, session := range []*mcp.ClientSession{worker, owner} {
		if err := session.Close(); err != nil {
			t.Fatal("MCP child close", err)
		}
	}
	t.Log("CLI owner + bound MCP worker shared one task; stale/identity/STOP refused; synthetic selected hook denied; raw shell/file/spawn uncovered; operation succeeded, cleanup done, pending=0 activeUses=0, run scratch empty, MCP sessions closed")
}
