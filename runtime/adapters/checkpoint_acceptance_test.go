package adapters

import "testing"

// Fixed independent check: the existing MCP transport calls this same request
// boundary; an unqualified checkpoint must never reach the filesystem service.
func TestCheckpointIndependentAdapterBoundary(t *testing.T) {
	for _, actor := range []string{"main", "worker-1"} {
		config := Config{ProjectRoot: t.TempDir(), Actor: actor}
		for _, action := range []string{"checkpoint.capture", "checkpoint.inspect", "checkpoint.restore", "checkpoint.future"} {
			got := request(config, requestInput{Action: action, RequestID: "bounded-checkpoint-test", ExpectedRevision: 0,
				Payload: map[string]any{"packagePath": "unqualified-external-package"}})
			if got.OK || got.Code != "CHECKPOINT_CLI_ONLY" {
				t.Fatalf("actor=%s action=%s response=%+v", actor, action, got)
			}
		}
	}
}
