package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// The private test seams model a change after staging, before the final effect.
// They are not request-selectable and do not grant native execution authority.
func TestCheckpointIndependentActivationGuards(t *testing.T) {
	for _, change := range []string{"destination-arrival", "stage-drift"} {
		t.Run(change, func(t *testing.T) {
			source := t.TempDir()
			if err := os.WriteFile(filepath.Join(source, "note.txt"), []byte("expected"), 0600); err != nil {
				t.Fatal(err)
			}
			packagePath := filepath.Join(t.TempDir(), "checkpoint")
			if result := checkpointRequest(t, source, "checkpoint.capture", checkpointPayload(packagePath)); !result.OK {
				t.Fatal(result)
			}
			parent := t.TempDir()
			destination := filepath.Join(parent, "destination")
			if err := os.Mkdir(destination, 0700); err != nil {
				t.Fatal(err)
			}
			checkpointRestoreBeforeActivation = func() error {
				if change == "destination-arrival" {
					return os.WriteFile(filepath.Join(destination, "foreign.txt"), []byte("preserve"), 0600)
				}
				stages, err := filepath.Glob(filepath.Join(parent, ".ocmi-checkpoint-restore-*"))
				if err != nil || len(stages) != 1 {
					t.Fatalf("stages=%v err=%v", stages, err)
				}
				return os.WriteFile(filepath.Join(stages[0], "note.txt"), []byte("unexpected"), 0600)
			}
			t.Cleanup(func() { checkpointRestoreBeforeActivation = nil })
			result := checkpointRequest(t, destination, "checkpoint.restore", map[string]any{"packagePath": packagePath})
			if result.OK {
				t.Fatalf("accepted changed activation input: %+v", result)
			}
			entries, err := os.ReadDir(destination)
			if err != nil {
				t.Fatal(err)
			}
			if change == "stage-drift" {
				if len(entries) != 0 {
					t.Fatalf("published changed staged payload: %v", entries)
				}
			} else {
				data, err := os.ReadFile(filepath.Join(destination, "foreign.txt"))
				if err != nil || string(data) != "preserve" || len(entries) != 1 {
					t.Fatalf("foreign data moved/changed: entries=%v err=%v", entries, err)
				}
			}
		})
	}
}
