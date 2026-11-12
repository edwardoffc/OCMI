package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestWorkerUpdateSystemActivation(t *testing.T) {
	for _, scenario := range []string{"valid", "foreign-parked", "changed-stage"} {
		t.Run(scenario, func(t *testing.T) {
			base := t.TempDir()
			home := filepath.Join(base, "home")
			candidate := filepath.Join(base, "candidate")
			acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
			acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
			if r := InstallHome(home, false); !r.OK {
				t.Fatal(r)
			}
			ctx, info, err := verifyFactoryHome(home)
			if err != nil {
				t.Fatal(err)
			}
			meta, err := captureUpdateMetadata(ctx, info, strings.Repeat("b", 40), candidate)
			if err != nil {
				t.Fatal(err)
			}
			old, err := captureUpdatePublicProof(home, true)
			if err != nil {
				t.Fatal(err)
			}
			next, err := captureUpdatePublicProof(candidate, false)
			if err != nil {
				t.Fatal(err)
			}
			lock, err := setupLockOwned(filepath.Join(home, "local", "bindings"))
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Release()
			id := uuid.NewString()
			j := updateProofJournal{SchemaVersion: 1, ID: id, Phase: "intent", Metadata: meta, OldSource: meta.OldSource, NewSource: meta.NewSource, OldPublic: old, NewPublic: next, Stage: "local/runs/update-" + id, OldSlot: "old", NewSlot: "new", AdmissionID: lock.nativeID}
			if err = createUpdateStage(ctx, &j, home, candidate); err != nil {
				t.Fatal(err)
			}
			if err = copyUpdateStage(ctx, &j, home, candidate); err != nil {
				t.Fatal(err)
			}
			stage := filepath.Join(home, filepath.FromSlash(j.Stage))
			oldID := old.SystemID
			newID, err := localNativeID(filepath.Join(stage, "new", "system"))
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "foreign-parked" {
				if err = os.Mkdir(filepath.Join(stage, "parked-system"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "changed-stage" {
				if err = os.WriteFile(filepath.Join(stage, "new", "README.md"), []byte("changed"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			writer, err := acquireUpdateRecoveryWriter(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Release()
			oldHook, newHook := false, false
			updateTestPhase = func(phase string) {
				if phase == "old_system" || phase == "new_system" {
					if err := verifyUpdateActivationState(ctx, j); err != nil {
						t.Errorf("split proof at %s: %v", phase, err)
					}
				}
				switch phase {
				case "old_system":
					oldHook = true
					if _, err := os.Lstat(filepath.Join(home, "system")); !os.IsNotExist(err) {
						t.Error("old system still in home")
					}
					if got, err := localNativeID(filepath.Join(stage, "parked-system")); err != nil || got != oldID {
						t.Error("old native identity lost")
					}
				case "new_system":
					newHook = true
					if got, err := localNativeID(filepath.Join(home, "system")); err != nil || got != newID {
						t.Error("new native identity lost")
					}
					if _, _, err := verifyFactoryHome(home); err == nil {
						t.Error("mixed payload appeared ready")
					}
				}
			}
			defer func() { updateTestPhase = nil }()
			err = activateUpdateSystem(ctx, &j, writer, lock)
			if scenario == "valid" {
				if err != nil || !oldHook || !newHook {
					t.Fatalf("activation: %v old=%v new=%v", err, oldHook, newHook)
				}
				if err = verifyUpdateSystem(filepath.Join(stage, "parked-system"), j.OldPublic); err != nil {
					t.Fatal(err)
				}
				if err = verifyUpdateSystem(filepath.Join(home, "system"), *j.NewStaged); err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil || oldHook || newHook {
					t.Fatalf("unsafe activation: %v", err)
				}
				if got, err := localNativeID(filepath.Join(home, "system")); err != nil || got != oldID {
					t.Fatal("home changed before refusal")
				}
			}
		})
	}
}
