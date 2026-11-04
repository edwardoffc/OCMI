package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckpointGitRejectsCaseAliasBeforeStructDecode(t *testing.T) {
	if checkpointGitNoCaseAlias([]byte(`{"packagePath":"x","Kind":"git-wip"}`)) {
		t.Fatal("case alias accepted")
	}
	if !checkpointGitNoCaseAlias([]byte(`{"packagePath":"x","kind":"git-wip"}`)) {
		t.Fatal("canonical keys refused")
	}
}

// Diagnostic coverage for the fixed consumer's foreign-arrival boundary. It
// records whether the hook ran, so a failed consumer result cannot be mistaken
// for unsafe activation when reconstruction failed earlier.
func TestCheckpointGitForeignArrivalDiagnostic(t *testing.T) {
	source := checkpointGitFixtureForAcceptance(t)
	parent := t.TempDir()
	packagePath := filepath.Join(parent, "package")
	payload := map[string]any{"packagePath": packagePath, "checkpointId": "foreign-diagnostic", "systemVersion": "1.3.5", "sourceIdentity": "fixture-source", "owner": "main", "purpose": "diagnostic", "retainUntil": "qualification", "nextAction": "confirm binding", "references": []any{}, "refs": []string{"refs/heads/main"}, "files": []string{}}
	checkpointGitCaptureBeforeVerify = func() {
		if err := os.WriteFile(filepath.Join(source, "file.txt"), []byte("changed\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if response := checkpointGitAcceptanceRequest(source, "checkpoint.git.capture", payload); response.OK {
		t.Fatal("drift capture accepted")
	}
	checkpointGitCaptureBeforeVerify = nil
	if response := checkpointGitAcceptanceRequest(source, "checkpoint.git.capture", payload); !response.OK {
		t.Fatalf("capture: %+v", response)
	}
	destination := filepath.Join(parent, "destination")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	reached := false
	checkpointGitRestoreBeforeActivation = func(string) error {
		reached = true
		return os.WriteFile(filepath.Join(destination, "foreign.txt"), []byte("unexpected\n"), 0600)
	}
	var helperFailure string
	checkpointGitRunOverride = func(ctx context.Context, spec commandSpec) (commandResult, error) {
		result, err := runOwned(ctx, spec)
		if err != nil || result.ExitCode != 0 || result.Truncated || result.TimedOut || result.Unresolved {
			helperFailure = fmt.Sprintf("args=%q exit=%d err=%v output=%q", spec.Args, result.ExitCode, err, result.Output)
		}
		return result, err
	}
	defer func() {
		checkpointGitCaptureBeforeVerify = nil
		checkpointGitRestoreBeforeActivation = nil
		checkpointGitRunOverride = nil
	}()
	raw, _ := json.Marshal(map[string]string{"packagePath": packagePath})
	response := Handle(Request{SchemaVersion: 1, Action: "checkpoint.git.restore", ProjectRoot: destination, Actor: "main", RequestID: "foreign-diagnostic", Payload: raw})
	entries, err := os.ReadDir(destination)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	data, dataErr := os.ReadFile(filepath.Join(destination, "foreign.txt"))
	if helperFailure != "" {
		t.Fatalf("Git helper diagnostic %s", helperFailure)
	}
	if !reached || response.OK || len(names) != 1 || names[0] != "foreign.txt" || dataErr != nil || string(data) != "unexpected\n" {
		t.Fatalf("diagnostic reachedHook=%t response=%+v destinationEntries=%v foreign=%q read=%v", reached, response, names, data, dataErr)
	}
}

func TestCheckpointGitCaptureRejectsSameByteIdentityReplacement(t *testing.T) {
	source := checkpointGitFixtureForAcceptance(t)
	parent := t.TempDir()
	payload := map[string]any{"packagePath": filepath.Join(parent, "package"), "checkpointId": "identity-drift", "systemVersion": "1.3.5", "sourceIdentity": "fixture-source", "owner": "main", "purpose": "identity diagnostic", "retainUntil": "qualification", "nextAction": "confirm binding", "references": []any{}, "refs": []string{"refs/heads/main"}, "files": []string{}}
	checkpointGitCaptureBeforeVerify = func() {
		path := filepath.Join(source, "file.txt")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		oldID, err := localNativeID(path)
		if err != nil {
			t.Fatal(err)
		}
		replacement := filepath.Join(source, ".identity-replacement")
		if err = os.WriteFile(replacement, data, 0600); err == nil {
			var replacementID string
			replacementID, err = localNativeID(replacement)
			if err == nil && replacementID == oldID {
				err = fmt.Errorf("replacement retained original identity")
			}
		}
		if err == nil {
			err = os.Rename(replacement, path)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	defer func() { checkpointGitCaptureBeforeVerify = nil }()
	if response := checkpointGitAcceptanceRequest(source, "checkpoint.git.capture", payload); response.OK {
		t.Fatal("same-byte identity replacement accepted")
	}
}
