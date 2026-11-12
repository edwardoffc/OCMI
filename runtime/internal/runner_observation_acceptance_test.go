package internal

import (
	"encoding/json"
	"strings"
	"testing"
)

// Fixed response/privacy oracle. This does not claim to reproduce a native
// process failure; the original ARM failure and next native trace are separate.
func TestRunnerObservationIndependentResponse(t *testing.T) {
	waited := false
	response := runnerRecoveryResponse(9, "fixture-op", commandResult{
		Unresolved: true, Phase: "child_state", ErrorClass: "not_found", GroupObservation: "present",
		PumpObservation: "open", WaitCalled: &waited, Output: "DO_NOT_EXPORT_FICTIONAL_COMMAND_OUTPUT",
	})
	if response.OK || response.Code != "RECOVERY_REQUIRED" || response.Revision != 9 {
		t.Fatal(response)
	}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Data map[string]any `json:"data"`
	}
	if err = json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for key, expected := range map[string]any{"operationId": "fixture-op", "nextAction": "operation.inspect",
		"phase": "child_state", "errorClass": "not_found", "groupObservation": "present", "pumpObservation": "open", "waitCalled": false} {
		if decoded.Data[key] != expected {
			t.Fatalf("%s=%v want=%v", key, decoded.Data[key], expected)
		}
	}
	if strings.Contains(string(raw), "DO_NOT_EXPORT") {
		t.Fatal("command output leaked")
	}
	unknown := runnerRecoveryResponse(9, "fixture-op", commandResult{
		Phase: "DO_NOT_EXPORT_PHASE", ErrorClass: "DO_NOT_EXPORT_ERROR", GroupObservation: "DO_NOT_EXPORT_GROUP", PumpObservation: "DO_NOT_EXPORT_PIPE",
	})
	raw, err = json.Marshal(unknown)
	if err != nil || strings.Contains(string(raw), "DO_NOT_EXPORT") {
		t.Fatalf("unbounded observation leaked: %s", raw)
	}
	decoded.Data = nil
	if err = json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Data["waitCalled"] != nil {
		t.Fatalf("unknown Wait became false proof: %v", decoded.Data)
	}
	for _, key := range []string{"phase", "errorClass", "groupObservation", "pumpObservation"} {
		if decoded.Data[key] != "unknown" {
			t.Fatalf("unqualified %s=%v", key, decoded.Data[key])
		}
	}
}
