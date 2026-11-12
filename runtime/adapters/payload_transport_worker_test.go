package adapters

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/edwardoffc/OCMI/runtime/internal"
)

func TestWorkerPayloadRawNumbersPreserved(t *testing.T) {
	const payload = `{"wide":9007199254740993,"huge":9223372036854775808123456789,"spelled":1.2300e+40}`
	for _, encoded := range []bool{false, true} {
		wirePayload := json.RawMessage(payload)
		if encoded {
			raw, e := json.Marshal(payload)
			if e != nil {
				t.Fatal(e)
			}
			wirePayload = raw
		}
		raw, e := json.Marshal(map[string]any{"action": "status", "requestId": "wide", "expectedRevision": int64(9007199254740993), "payload": wirePayload})
		if e != nil {
			t.Fatal(e)
		}
		var in requestInput
		if e = json.Unmarshal(raw, &in); e != nil {
			t.Fatal(e)
		}
		if in.ExpectedRevision != 9007199254740993 {
			t.Fatal("revision rounded")
		}
		got, e := normalizePayload(in.Payload)
		if e != nil || string(got) != payload {
			t.Fatalf("raw number spelling lost: %s %v", got, e)
		}
	}
	for _, value := range []any{json.RawMessage(`"\"{}\""`), `"{}"`, json.RawMessage(`null`), json.RawMessage(`[]`), `{} {}`, strings.Repeat(" ", maxRequest+1)} {
		if _, e := normalizePayload(value); e == nil {
			t.Fatal("nonobject/nested/oversize payload admitted")
		}
	}
}

func TestWorkerPayloadMalformedMatchedHookDenies(t *testing.T) {
	c := Config{ProjectRoot: t.TempDir(), Actor: "main"}
	for _, input := range []any{nil, "not an object", []any{}, map[string]any{"action": "resource.list", "payload": "null"}, map[string]any{"action": "stop", "payload": `"{}"`}, map[string]any{"action": "stop", "payload": `{"enabled":"true"}`}} {
		raw, e := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "tool_name": "mcp__ocmi__ocmi_request", "tool_input": input})
		if e != nil {
			t.Fatal(e)
		}
		var out strings.Builder
		if e = RunHook(c, strings.NewReader(string(raw)), &out); e != nil {
			t.Fatal("matched malformed call escaped as hook error", e)
		}
		if !strings.Contains(out.String(), `"permissionDecision":"deny"`) {
			t.Fatal(out.String())
		}
	}
	var uncovered strings.Builder
	if e := RunHook(c, strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":"unsupported shape"}`), &uncovered); e != nil || !strings.Contains(uncovered.String(), `"ocmi":"no_coverage"`) {
		t.Fatal("unsupported coverage changed", e, uncovered.String())
	}
}

func TestWorkerPayloadEncodedStopKeepsRunGate(t *testing.T) {
	root := t.TempDir()
	rev := initializeBoundState(t, root)
	c := Config{ProjectRoot: root, Actor: "main"}
	r := request(c, requestInput{Action: "stop", RequestID: "worker-encoded-stop", ExpectedRevision: rev, Payload: `{"enabled":true}`})
	if !r.OK {
		t.Fatal(r)
	}
	worker := Config{ProjectRoot: root, Actor: "worker"}
	for _, payload := range []any{map[string]any{}, `{}`} {
		blocked := request(worker, requestInput{Action: "run", RequestID: "worker-stopped-run", ExpectedRevision: r.Revision, Payload: payload})
		if blocked.OK || blocked.Code != "STOPPED" {
			t.Fatal("encoded route bypassed core STOP", blocked)
		}
		raw, e := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "tool_name": "mcp__ocmi__ocmi_request", "tool_input": map[string]any{"action": "run", "payload": payload}})
		if e != nil {
			t.Fatal(e)
		}
		var out strings.Builder
		if e = RunHook(worker, strings.NewReader(string(raw)), &out); e != nil || !strings.Contains(out.String(), `"permissionDecision":"deny"`) {
			t.Fatal("encoded run bypassed hook STOP", e, out.String())
		}
	}
	status := internal.Handle(internal.Request{SchemaVersion: 1, Action: "status", ProjectRoot: root, Actor: "main", RequestID: "worker-check", Payload: json.RawMessage(`{}`)})
	if !status.OK || status.Revision != r.Revision || status.Data["stopped"] != true {
		t.Fatal("refused calls changed state", status)
	}
}
