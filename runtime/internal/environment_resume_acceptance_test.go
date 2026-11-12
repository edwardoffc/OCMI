package internal

// Independent completion of the already-issued E2/E3 ownership and pre-effect checks.
import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnvironmentAcceptancePrelaunchSourceDrift(t *testing.T) {
	r := environmentAcceptanceRequest(t, false)
	oldRecord, oldRun := environmentRecordOverride, environmentRunOverride
	count := 0
	environmentRecordOverride = func(state string) error {
		if state == "intent" {
			return os.WriteFile(filepath.Join(r.ProjectRoot, "go.mod"), []byte("module changed\ngo 1.20\n"), 0600)
		}
		return nil
	}
	environmentRunOverride = func(context.Context, commandSpec) (commandResult, error) {
		count++
		return commandResult{StoppedVerified: true, Output: "go version go1.26.2 windows/amd64"}, nil
	}
	t.Cleanup(func() { environmentRecordOverride, environmentRunOverride = oldRecord, oldRun })
	response := Handle(r)
	if response.OK || count != 0 || response.Data["ownedStage"] == nil || response.Data["nextAction"] == nil {
		t.Fatalf("prelaunch drift ran or lost retained artifacts: count=%d %+v", count, response)
	}
}

func TestEnvironmentAcceptanceFinalDriftRetainsOwnership(t *testing.T) {
	r := environmentAcceptanceRequest(t, false)
	old := environmentRunOverride
	environmentRunOverride = func(ctx context.Context, spec commandSpec) (commandResult, error) {
		if err := spec.Registered(43210, 54321); err != nil {
			return commandResult{}, err
		}
		if err := os.WriteFile(filepath.Join(r.ProjectRoot, "go.mod"), []byte("module changed\ngo 1.20\n"), 0600); err != nil {
			t.Fatal(err)
		}
		return commandResult{StoppedVerified: true, Output: "go version go1.26.2 windows/amd64"}, nil
	}
	t.Cleanup(func() { environmentRunOverride = old })
	response := Handle(r)
	if response.OK || response.Data["ownedStage"] == nil || response.Data["nextAction"] == nil {
		t.Fatalf("final drift lost the owned operation: %+v", response)
	}
}

func TestEnvironmentAcceptanceConflictsBeforeProbe(t *testing.T) {
	r := environmentAcceptanceRequest(t, false)
	var in environmentInput
	if err := json.Unmarshal(r.Payload, &in); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.ProjectRoot, "go.mod"), []byte("module fixture\ngo 1.26\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.ProjectRoot, "package.json"), []byte(`{"engines":{"node":">=23 <20"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	in.Sources = append(in.Sources, environmentSource{Kind: "node-package", Path: "package.json"})
	// Bind an existing executable, but no runner may be called for conflicting Node declarations.
	in.Tools = []environmentTool{{Kind: "node", Executable: in.Tools[0].Executable}}
	r.Payload, _ = json.Marshal(in)
	old := environmentRunOverride
	count := 0
	environmentRunOverride = func(context.Context, commandSpec) (commandResult, error) {
		count++
		return commandResult{StoppedVerified: true, Output: "v24.1.0"}, nil
	}
	t.Cleanup(func() { environmentRunOverride = old })
	response := Handle(r)
	if !response.OK || count != 0 {
		t.Fatalf("conflict probed before resolution: count=%d %+v", count, response)
	}
	for _, raw := range environmentAcceptanceWire(t, response)["requirements"].([]any) {
		q := raw.(map[string]any)
		if q["kind"] == "node" && q["status"] != "conflict" {
			t.Fatal(q)
		}
	}
}

func TestEnvironmentAcceptanceIntentNamesProject(t *testing.T) {
	r := environmentAcceptanceRequest(t, false)
	old := environmentRunOverride
	environmentRunOverride = func(ctx context.Context, spec commandSpec) (commandResult, error) {
		raw, err := os.ReadFile(filepath.Join(spec.Cwd, "journal-001.json"))
		if err != nil {
			t.Fatal(err)
		}
		var intent map[string]any
		if err := json.Unmarshal(raw, &intent); err != nil {
			t.Fatal(err)
		}
		if intent["projectRoot"] != r.ProjectRoot {
			t.Fatalf("initial ownership intent cannot be attributed: %+v", intent)
		}
		if err := spec.Registered(43210, 54321); err != nil {
			return commandResult{}, err
		}
		return commandResult{StoppedVerified: true, Output: "go version go1.26.2 windows/amd64"}, nil
	}
	t.Cleanup(func() { environmentRunOverride = old })
	if response := Handle(r); !response.OK {
		t.Fatal(response)
	}
}
