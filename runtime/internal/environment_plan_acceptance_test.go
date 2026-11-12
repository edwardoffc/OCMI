package internal

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func environmentAcceptanceWire(t *testing.T, response Response) map[string]any {
	t.Helper()
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if json.Unmarshal(raw, &wire) != nil {
		t.Fatal("response format")
	}
	return wire["data"].(map[string]any)
}

func TestEnvironmentAcceptanceRecoveryPriority(t *testing.T) {
	r := environmentAcceptanceRequest(t, true)
	var in environmentInput
	if json.Unmarshal(r.Payload, &in) != nil {
		t.Fatal("fixture")
	}
	in.Tools = nil
	if err := os.WriteFile(filepath.Join(r.ProjectRoot, "package.json"), []byte(`{"engines":{"node":">=23 <20"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	r.Payload, _ = json.Marshal(in)
	response := Handle(r)
	if !response.OK {
		t.Fatal(response)
	}
	steps := environmentAcceptanceWire(t, response)["recoverySteps"].([]any)
	first := steps[0].(map[string]any)
	if !strings.Contains(strings.ToLower(first["action"].(string)), "resolv") || first["source"] != "package.json" {
		t.Fatalf("unresolved declaration became tool installation: %+v", steps)
	}
}

func TestEnvironmentAcceptanceRecoveryDependenciesAndOwnership(t *testing.T) {
	r := environmentAcceptanceRequest(t, false)
	var in environmentInput
	_ = json.Unmarshal(r.Payload, &in)
	if err := os.WriteFile(filepath.Join(r.ProjectRoot, "setup.md"), []byte("Setup\nDependencies\nData\nTask\n"), 0600); err != nil {
		t.Fatal(err)
	}
	in.Sources = append(in.Sources, environmentSource{Kind: "guide", Path: "setup.md"})
	in.Provisions = []environmentReference{{Kind: "data", Source: "setup.md", Line: 3}}
	in.Continuation = &struct {
		Source string
		Line   int
	}{Source: "setup.md", Line: 4}
	r.Payload, _ = json.Marshal(in)
	previous := environmentRunOverride
	environmentRunOverride = func(ctx context.Context, spec commandSpec) (commandResult, error) {
		if err := spec.Registered(43210, 54321); err != nil {
			return commandResult{}, err
		}
		return commandResult{StoppedVerified: true, Output: "go version go1.26.2 windows/amd64", ExitCode: 0}, nil
	}
	t.Cleanup(func() { environmentRunOverride = previous })
	response := Handle(r)
	if !response.OK {
		t.Fatal(response)
	}
	data := environmentAcceptanceWire(t, response)
	steps := data["recoverySteps"].([]any)
	dependency, provision := -1, -1
	for i, raw := range steps {
		step := raw.(map[string]any)
		kind, _ := step["kind"].(string)
		if kind == "dependencies" {
			dependency = i
		}
		if kind == "provision" {
			provision = i
			if step["line"] != float64(3) {
				t.Fatal(step)
			}
		}
	}
	if dependency < 0 || provision <= dependency || steps[len(steps)-1].(map[string]any)["kind"] != "continuation" {
		t.Fatalf("matching tools skipped ordered recovery: %+v", steps)
	}
	stage, ok := data["ownedStage"].(string)
	if !ok || data["operationState"] != "complete" {
		t.Fatal(data)
	}
	bound, registered := false, false
	entries, err := os.ReadDir(stage)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(stage, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var record map[string]any
		if json.Unmarshal(raw, &record) != nil {
			t.Fatal("journal format")
		}
		if record["projectRoot"] == r.ProjectRoot || record["project"] == r.ProjectRoot {
			bound = true
		}
		if record["state"] == "registered" {
			registered = true
			if record["group"] != float64(54321) || record["tool"] != "go" {
				t.Fatalf("native group/tool identity invented or lost: %+v", record)
			}
		}
	}
	if !bound || !registered {
		t.Fatalf("operation cannot be attributed to selected project: bound=%v registered=%v", bound, registered)
	}
}
