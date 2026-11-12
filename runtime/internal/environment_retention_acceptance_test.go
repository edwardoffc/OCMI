package internal

// Same E2/E3 completion boundary: failure at first intent and drift between probes.
import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEnvironmentAcceptanceInitialJournalFailureRetainsStage(t *testing.T) {
	r := environmentAcceptanceRequest(t, false)
	oldRecord, oldRun := environmentRecordOverride, environmentRunOverride
	count := 0
	environmentRecordOverride = func(state string) error {
		if state == "intent" {
			return os.ErrPermission
		}
		return nil
	}
	environmentRunOverride = func(context.Context, commandSpec) (commandResult, error) { count++; return commandResult{}, nil }
	t.Cleanup(func() { environmentRecordOverride, environmentRunOverride = oldRecord, oldRun })
	response := Handle(r)
	stage, ok := response.Data["ownedStage"].(string)
	if response.OK || count != 0 || !ok || response.Data["nextAction"] == nil {
		t.Fatalf("first journal failure lost created stage or launched: count=%d %+v", count, response)
	}
	if info, err := os.Stat(stage); err != nil || !info.IsDir() {
		t.Fatalf("retained stage unavailable: %s %v", stage, err)
	}
}

func TestEnvironmentAcceptanceDriftBetweenProbesStops(t *testing.T) {
	r := environmentAcceptanceRequest(t, true)
	old := environmentRunOverride
	count := 0
	environmentRunOverride = func(ctx context.Context, spec commandSpec) (commandResult, error) {
		count++
		if err := spec.Registered(43210, 54321); err != nil {
			return commandResult{}, err
		}
		if count == 1 {
			if err := os.WriteFile(filepath.Join(r.ProjectRoot, "package.json"), []byte(`{"engines":{"node":">=22"}}`), 0600); err != nil {
				t.Fatal(err)
			}
			return commandResult{StoppedVerified: true, Output: "go version go1.26.2 windows/amd64"}, nil
		}
		return commandResult{StoppedVerified: true, Output: "v24.1.0"}, nil
	}
	t.Cleanup(func() { environmentRunOverride = old })
	response := Handle(r)
	if response.OK || count != 1 || response.Data["ownedStage"] == nil || response.Data["nextAction"] == nil {
		t.Fatalf("changed selection launched another probe or lost stage: count=%d %+v", count, response)
	}
}
