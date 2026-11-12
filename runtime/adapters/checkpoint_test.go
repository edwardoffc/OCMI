package adapters

import "testing"

func TestCheckpointRequestsRemainCLIOnly(t *testing.T) {
	root := t.TempDir()
	response := request(Config{ProjectRoot: root, Actor: "main"}, requestInput{Action: "checkpoint.inspect", RequestID: "checkpoint-adapter", Payload: map[string]any{"packagePath": root}})
	if response.OK || response.Code != "CHECKPOINT_CLI_ONLY" {
		t.Fatalf("response=%+v", response)
	}
}
