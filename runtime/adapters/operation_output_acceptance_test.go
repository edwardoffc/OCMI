package adapters

import (
	"bytes"
	"encoding/json"
	"testing"
)

// The read-only evidence request must reach core policy through the real hook,
// even while state is unavailable; the hook itself never returns evidence.
func TestMainOperationOutputHookConsumer(t *testing.T) {
	c := Config{ProjectRoot: t.TempDir(), Actor: "worker"}
	for _, action := range []string{"operation.output", "unknown.output"} {
		raw, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "tool_name": "mcp__ocmi__ocmi_request", "tool_input": map[string]any{"action": action, "payload": map[string]any{"operationId": "fixture-operation"}}})
		var out bytes.Buffer
		if err := RunHook(c, bytes.NewReader(raw), &out); err != nil {
			t.Fatal(err)
		}
		var response map[string]any
		if json.Unmarshal(out.Bytes(), &response) != nil {
			t.Fatal(out.String())
		}
		if action == "operation.output" {
			if response["ocmi"] != "safe_selected_call" || response["output"] != nil {
				t.Fatal("read-only evidence blocked or disclosed by hook", response)
			}
		} else if _, ok := response["hookSpecificOutput"]; !ok {
			t.Fatal("unknown action silently admitted", response)
		}
	}
}
