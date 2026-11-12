package adapters

import (
	"testing"
)

func TestEnvironmentTransportRemainsCLIOnly(t *testing.T) {
	for _, actor := range []string{"main", "worker-1", "consultant"} {
		for _, action := range []string{"environment.inspect", "environment.future"} {
			c := Config{ProjectRoot: t.TempDir(), Actor: actor}
			r := request(c, requestInput{Action: action, RequestID: "synthetic-transport", ExpectedRevision: 0,
				Payload: map[string]any{"sources": []string{".local/coordinator/HANDOFF.md"}}})
			if r.OK || r.Code != "ENVIRONMENT_CLI_ONLY" {
				t.Fatalf("%s %s: %+v", actor, action, r)
			}
		}
	}
}
