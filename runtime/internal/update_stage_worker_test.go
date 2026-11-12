package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestWorkerUpdateStage(t *testing.T) {
	for _, scenario := range []string{"full", "partial", "stage-replaced", "slot-replaced", "extra-file", "empty-dir", "changed", "template-mismatch"} {
		t.Run(scenario, func(t *testing.T) {
			base := t.TempDir()
			home := filepath.Join(base, "home")
			next := filepath.Join(base, "next")
			acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
			acceptanceUpdateFactory(t, next, strings.Repeat("b", 40))
			if r := InstallHome(home, false); !r.OK {
				t.Fatal(r)
			}
			if scenario == "template-mismatch" {
				if r := CoordinatorPrepare(home, "en", "Fictional project"); !r.OK {
					t.Fatal(r)
				}
			}
			ctx, info, err := verifyFactoryHome(home)
			if err != nil {
				t.Fatal(err)
			}
			meta, err := captureUpdateMetadata(ctx, info, strings.Repeat("b", 40), next)
			if err != nil {
				t.Fatal(err)
			}
			old, err := captureUpdatePublicProof(home, true)
			if err != nil {
				t.Fatal(err)
			}
			newProof, err := captureUpdatePublicProof(next, false)
			if err != nil {
				t.Fatal(err)
			}
			lock, err := setupLockOwned(filepath.Join(home, "local", "bindings"))
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Release()
			id := uuid.NewString()
			j := updateProofJournal{SchemaVersion: 1, ID: id, Phase: "intent", Metadata: meta, OldSource: meta.OldSource, NewSource: meta.NewSource, OldPublic: old, NewPublic: newProof, Stage: "local/runs/update-" + id, OldSlot: "old", NewSlot: "new", AdmissionID: lock.nativeID}
			if scenario == "template-mismatch" {
				j.Metadata.OldTemplate += "changed"
				if err := verifyUpdateProofInputs(j, home, next); err == nil {
					t.Fatal("accepted forged template")
				}
				return
			}
			if err := createUpdateStage(ctx, &j, home, next); err != nil {
				t.Fatal(err)
			}
			stage := filepath.Join(home, filepath.FromSlash(j.Stage))
			if err := verifyUpdateStage(ctx, j, true); err != nil {
				t.Fatal(err)
			}
			if scenario == "partial" {
				entry := j.OldPublic.Files[0]
				raw, err := installationRead(home, entry.Path, 32<<20)
				if err != nil {
					t.Fatal(err)
				}
				dest := filepath.Join(stage, "old", filepath.FromSlash(entry.Path))
				if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(dest, raw, os.FileMode(entry.Mode)); err != nil {
					t.Fatal(err)
				}
				if err = verifyUpdateStage(ctx, j, true); err != nil {
					t.Fatal(err)
				}
				if err = copyUpdateStage(ctx, &j, home, next); err != nil {
					t.Fatal(err)
				}
				if err = verifyUpdateStage(ctx, j, false); err != nil {
					t.Fatal(err)
				}
				return
			}
			if err := copyUpdateStage(ctx, &j, home, next); err != nil {
				t.Fatal(err)
			}
			if err := verifyUpdateStage(ctx, j, false); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "full":
				return
			case "stage-replaced":
				if err = os.Rename(stage, stage+"-original"); err != nil {
					t.Fatal(err)
				}
				if err = os.Mkdir(stage, 0700); err != nil {
					t.Fatal(err)
				}
			case "slot-replaced":
				slot := filepath.Join(stage, "old")
				if err = os.Rename(slot, filepath.Join(stage, "original")); err != nil {
					t.Fatal(err)
				}
				if err = os.Mkdir(slot, 0700); err != nil {
					t.Fatal(err)
				}
			case "extra-file":
				err = os.WriteFile(filepath.Join(stage, "old", "foreign"), []byte("foreign"), 0600)
			case "empty-dir":
				err = os.Mkdir(filepath.Join(stage, "old", "empty"), 0700)
			case "changed":
				err = os.WriteFile(filepath.Join(stage, "old", "README.md"), []byte("changed"), 0700)
				if err == nil {
					sumsPath := filepath.Join(stage, "old", "SHA256SUMS")
					raw, e := os.ReadFile(sumsPath)
					if e != nil {
						t.Fatal(e)
					}
					lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
					for i, line := range lines {
						if strings.HasSuffix(line, "  README.md") {
							lines[i] = fmt.Sprintf("%s  README.md", updateDigest([]byte("changed")))
						}
					}
					err = os.WriteFile(sumsPath, []byte(strings.Join(lines, "\n")+"\n"), 0600)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = verifyUpdateStage(ctx, j, false); err == nil {
				t.Fatal("accepted changed stage")
			}
		})
	}
}
