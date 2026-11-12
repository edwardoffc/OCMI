package adapters

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edwardoffc/OCMI/runtime/internal"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const serveMCP = "OCMI_TEST_SERVE_MCP"

func TestMain(m *testing.M) {
	if os.Getenv(serveMCP) != "" {
		root, actor := os.Getenv("OCMI_TEST_ROOT"), os.Getenv("OCMI_TEST_ACTOR")
		if err := ServeMCP(context.Background(), Config{ProjectRoot: root, Actor: actor}); err != nil {
			os.Exit(2)
		}
		return
	}
	os.Exit(m.Run())
}

func TestMCPSubprocessHandshakeToolsAndCoreRejection(t *testing.T) {
	root := t.TempDir()
	revision := initializeBoundState(t, root)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), serveMCP+"=1", "OCMI_TEST_ROOT="+root, "OCMI_TEST_ACTOR=main")
	client := mcp.NewClient(&mcp.Implementation{Name: "adapter-test", Version: "v1"}, nil)
	session, err := client.Connect(t.Context(), &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	tools, err := session.ListTools(t.Context(), nil)
	if err != nil || len(tools.Tools) != 1 || tools.Tools[0].Name != "ocmi_request" {
		t.Fatalf("tools=%+v err=%v", tools, err)
	}
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "ocmi_request", Arguments: map[string]any{"action": "status", "requestId": "status-1", "expectedRevision": 0, "payload": map[string]any{}}})
	if err != nil || result.IsError {
		t.Fatalf("status=%+v err=%v", result, err)
	}
	response := decodeResponse(t, result)
	if !response.OK || response.Code != "STATUS" || response.Revision != revision {
		t.Fatalf("status response=%+v", response)
	}
	result, err = session.CallTool(t.Context(), &mcp.CallToolParams{Name: "ocmi_request", Arguments: map[string]any{"action": "not-real", "requestId": "bad-1", "expectedRevision": revision, "payload": map[string]any{}}})
	if err != nil || !result.IsError {
		t.Fatalf("rejection=%+v err=%v", result, err)
	}
	if response = decodeResponse(t, result); response.Code != "UNKNOWN_ACTION" {
		t.Fatalf("rejection response=%+v", response)
	}
}

func TestHookDeniesStopAndIsHonestAboutUnsupportedEvents(t *testing.T) {
	root := t.TempDir()
	revision := initializeBoundState(t, root)
	stop := internal.Handle(internal.Request{SchemaVersion: 1, Action: "stop", ProjectRoot: root, Actor: "main", RequestID: "stop-1", ExpectedRevision: revision, Payload: json.RawMessage(`{"enabled":true}`)})
	if !stop.OK {
		t.Fatal(stop)
	}
	var output strings.Builder
	if err := RunHook(Config{ProjectRoot: root, Actor: "main"}, strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"mcp__ocmi__ocmi_request","tool_input":{"action":"run","payload":{}}}`), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"permissionDecision":"deny"`) || !strings.Contains(output.String(), "STOP") {
		t.Fatalf("hook=%s", output.String())
	}
	output.Reset()
	if err := RunHook(Config{ProjectRoot: root, Actor: "main"}, strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"Bash"}`), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "no_coverage") {
		t.Fatalf("unsupported=%s", output.String())
	}
	output.Reset()
	if err := RunHook(Config{ProjectRoot: root, Actor: "main"}, strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"mcp__ocmi__ocmi_request","tool_input":{"action":"stop","payload":{"enabled":false}}}`), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "safe_selected_call") {
		t.Fatalf("clear stop=%s", output.String())
	}
}

func TestTransferPathsNeedConfiguredConfinedRoot(t *testing.T) {
	root := t.TempDir()
	initializeState(t, root)
	payload := json.RawMessage(`{"transferPath":"C:\\outside"}`)
	response := request(Config{ProjectRoot: root, Actor: "main"}, requestInput{Action: "handover.export", RequestID: "x", Payload: payload})
	if response.Code != "TRANSFER_PATH_DENIED" {
		t.Fatal(response)
	}
	allowed := filepath.Join(root, "transfer")
	if err := os.MkdirAll(allowed, 0700); err != nil {
		t.Fatal(err)
	}
	response = request(Config{ProjectRoot: root, Actor: "main", TransferRoot: allowed}, requestInput{Action: "handover.export", RequestID: "y", Payload: json.RawMessage(`{"transferPath":"` + strings.ReplaceAll(filepath.Join(allowed, "out.json"), `\`, `\\`) + `"}`)})
	if response.Code == "TRANSFER_PATH_DENIED" {
		t.Fatal(response)
	}
}

func TestRequestKeepsBoundIdentityAndSize(t *testing.T) {
	root := t.TempDir()
	initializeState(t, root)
	malformed := request(Config{ProjectRoot: root, Actor: "main"}, requestInput{RequestID: "missing-action", Payload: map[string]any{}})
	if malformed.Code != "INVALID_REQUEST" {
		t.Fatal(malformed)
	}
	foreign := request(Config{ProjectRoot: root, Actor: "main"}, requestInput{Action: "status", RequestID: "identity", ProjectRoot: filepath.Join(root, "other"), Payload: map[string]any{}})
	if foreign.Code != "BOUND_IDENTITY_DENIED" {
		t.Fatal(foreign)
	}
	foreign = request(Config{ProjectRoot: root, Actor: "main"}, requestInput{Action: "status", RequestID: "actor", Actor: "other", Payload: map[string]any{}})
	if foreign.Code != "BOUND_IDENTITY_DENIED" {
		t.Fatal(foreign)
	}
	oversized := request(Config{ProjectRoot: root, Actor: "main"}, requestInput{Action: "status", RequestID: "large", Payload: map[string]any{"padding": strings.Repeat("x", maxRequest)}})
	if oversized.Code != "INVALID_REQUEST" {
		t.Fatal(oversized)
	}
}

func TestHookRequiresExplicitStopClearAndTreatsRebindAsConsequential(t *testing.T) {
	root := t.TempDir()
	initializeBoundState(t, root)
	for _, event := range []string{`{"hook_event_name":"PreToolUse","tool_name":"mcp__ocmi__ocmi_request","tool_input":{"action":"stop","payload":{}}}`, `{"hook_event_name":"PreToolUse","tool_name":"mcp__ocmi__ocmi_request","tool_input":{"action":"stop","payload":{"enabled":null}}}`, `{"hook_event_name":"PreToolUse","tool_name":"mcp__ocmi__ocmi_request","tool_input":{"action":"stop","payload":{"enabled":"true"}}}`} {
		var out strings.Builder
		if err := RunHook(Config{ProjectRoot: root, Actor: "main"}, strings.NewReader(event), &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"permissionDecision":"deny"`) {
			t.Fatalf("event=%s output=%s", event, out.String())
		}
	}
	for _, value := range []string{"true", "false"} {
		var out strings.Builder
		event := `{"hook_event_name":"PreToolUse","tool_name":"mcp__ocmi__ocmi_request","tool_input":{"action":"stop","payload":{"enabled":` + value + `}}}`
		if err := RunHook(Config{ProjectRoot: root, Actor: "main"}, strings.NewReader(event), &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"ocmi":"safe_selected_call"`) {
			t.Fatalf("explicit stop=%s", out.String())
		}
	}
	if !consequential("handover.rebind") {
		t.Fatal("rebind is not consequential")
	}
	var rebind strings.Builder
	event := `{"hook_event_name":"PreToolUse","tool_name":"mcp__ocmi__ocmi_request","tool_input":{"action":"handover.rebind","payload":{}}}`
	if err := RunHook(Config{ProjectRoot: root, Actor: "main"}, strings.NewReader(event), &rebind); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rebind.String(), `"ocmi":"state_allows_selected_call"`) {
		t.Fatalf("supported rebind=%s", rebind.String())
	}
}

func TestHookPreservesLifecycleCorePassThrough(t *testing.T) {
	root := t.TempDir()
	initializeBoundState(t, root)
	for _, action := range []string{"task.history", "operation.inspect", "state.upgrade", "operation.reconcile", "init"} {
		var out strings.Builder
		event := `{"hook_event_name":"PreToolUse","tool_name":"mcp__ocmi__ocmi_request","tool_input":{"action":"` + action + `","payload":{}}}`
		if err := RunHook(Config{ProjectRoot: root, Actor: "main"}, strings.NewReader(event), &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"ocmi":"safe_selected_call"`) {
			t.Fatalf("%s: %s", action, out.String())
		}
	}
	if !consequential("task.revise") || !consequential("outcome.decide") {
		t.Fatal("lifecycle mutations are not guarded")
	}
}

func TestSessionStartUsesCurrentStatusOnly(t *testing.T) {
	root := t.TempDir()
	initializeBoundState(t, root)
	var output strings.Builder
	if err := RunHook(Config{ProjectRoot: root, Actor: "main"}, strings.NewReader(`{"hook_event_name":"SessionStart"}`), &output); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"task-1", "ready", "run", "false"} {
		if !strings.Contains(output.String(), field) {
			t.Fatalf("missing %q in %s", field, output.String())
		}
	}
}

func initializeState(t *testing.T, root string) {
	t.Helper()
	r := internal.Handle(internal.Request{SchemaVersion: 1, Action: "init", ProjectRoot: root, Actor: "main", RequestID: "init-1", ExpectedRevision: 0, Payload: json.RawMessage(`{"projectId":"p","deviceId":"d","copyId":"c","owner":"main"}`)})
	if !r.OK {
		t.Fatal(r)
	}
}

func initializeBoundState(t *testing.T, root string) int64 {
	t.Helper()
	initializeState(t, root)
	for name, body := range map[string]string{"contract.md": "contract", "input.txt": "input"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	payload, err := json.Marshal(map[string]any{"taskId": "task-1", "outcomeId": "outcome-1", "taskRevision": 1, "worker": "worker", "reviewer": "main", "completionScope": "target", "contract": "contract.md", "inputs": []string{"input.txt"}, "command": map[string]any{"id": "noop", "executable": os.Args[0], "args": []string{}, "cwd": ".", "timeoutMs": 1000, "outputLimitBytes": 1024}})
	if err != nil {
		t.Fatal(err)
	}
	bound := internal.Handle(internal.Request{SchemaVersion: 1, Action: "task.bind", ProjectRoot: root, Actor: "main", RequestID: "bind-1", ExpectedRevision: 0, Payload: payload})
	if !bound.OK {
		t.Fatal(bound)
	}
	ready := internal.Handle(internal.Request{SchemaVersion: 1, Action: "task.ready", ProjectRoot: root, Actor: "main", RequestID: "ready-1", ExpectedRevision: bound.Revision, Payload: json.RawMessage(`{}`)})
	if !ready.OK {
		t.Fatal(ready)
	}
	return ready.Revision
}

func decodeResponse(t *testing.T, result *mcp.CallToolResult) internal.Response {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("content=%+v", result.Content)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content=%T", result.Content[0])
	}
	var response internal.Response
	if err := json.Unmarshal([]byte(text.Text), &response); err != nil {
		t.Fatal(err)
	}
	return response
}
