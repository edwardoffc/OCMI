package adapters

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPayloadTransportEncodedObjectConsumer(t *testing.T) {
	root := t.TempDir()
	rev := initializeBoundState(t, root)
	c := Config{ProjectRoot: root, Actor: "main"}
	for _, payload := range []any{map[string]any{"limit": 1}, `{"limit":1}`} {
		r := request(c, requestInput{Action: "resource.list", RequestID: "read-resource-list", Payload: payload})
		if !r.OK || r.Code != "RESOURCES" || r.Revision != rev {
			t.Fatalf("real object-field consumer: payload type %T, response %+v", payload, r)
		}
	}
	stop := request(c, requestInput{Action: "stop", RequestID: "encoded-stop", ExpectedRevision: rev, Payload: `{"enabled":true}`})
	if !stop.OK || stop.Code != "STOP_UPDATED" || stop.Revision != rev+1 {
		t.Fatalf("encoded mutation must reach original core gates: %+v", stop)
	}
	status := request(c, requestInput{Action: "status", RequestID: "read-stopped", Payload: `{}`})
	if !status.OK || status.Data["stopped"] != true {
		t.Fatal(status)
	}
	foreign := request(c, requestInput{Action: "status", RequestID: "foreign-encoded", Actor: "other", Payload: `{}`})
	if foreign.Code != "BOUND_IDENTITY_DENIED" {
		t.Fatal(foreign)
	}
}

func TestPayloadTransportMalformedDoesNotMutate(t *testing.T) {
	root := t.TempDir()
	rev := initializeBoundState(t, root)
	c := Config{ProjectRoot: root, Actor: "main"}
	for _, payload := range []any{"", "{", "null", "true", "[]", `"{}"`, `{} {}`, nil, []any{}, 1} {
		r := request(c, requestInput{Action: "stop", RequestID: "bad-payload", ExpectedRevision: rev, Payload: payload})
		if r.OK {
			t.Fatalf("malformed/non-object input admitted: %T %v", payload, payload)
		}
		status := request(c, requestInput{Action: "status", RequestID: "unchanged", Payload: map[string]any{}})
		if !status.OK || status.Revision != rev || status.Data["stopped"] != false {
			t.Fatalf("refusal changed state: %+v", status)
		}
	}
}

func TestPayloadTransportHookEncodedStop(t *testing.T) {
	root := t.TempDir()
	initializeBoundState(t, root)
	for _, encoded := range []string{`{"enabled":true}`, `{"enabled":false}`} {
		event, err := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "tool_name": "mcp__ocmi__ocmi_request",
			"tool_input": map[string]any{"action": "stop", "payload": encoded}})
		if err != nil {
			t.Fatal(err)
		}
		var out strings.Builder
		if err := RunHook(Config{ProjectRoot: root, Actor: "main"}, strings.NewReader(string(event)), &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"ocmi":"safe_selected_call"`) {
			t.Fatalf("hook and MCP must interpret same explicit boolean: %s", out.String())
		}
	}
}

func TestPayloadTransportSDKEncodedObject(t *testing.T) {
	root := t.TempDir()
	rev := initializeBoundState(t, root)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), serveMCP+"=1", "OCMI_TEST_ROOT="+root, "OCMI_TEST_ACTOR=main")
	client := mcp.NewClient(&mcp.Implementation{Name: "payload-consumer", Version: "v1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "ocmi_request", Arguments: map[string]any{
		"action": "resource.list", "requestId": "encoded-resource-list", "expectedRevision": 0, "payload": `{"limit":1}`}})
	if err != nil || result.IsError {
		t.Fatalf("actual stdio encoded consumer: result=%+v err=%v", result, err)
	}
	response := decodeResponse(t, result)
	if !response.OK || response.Code != "RESOURCES" || response.Revision != rev {
		t.Fatal(response)
	}
}
