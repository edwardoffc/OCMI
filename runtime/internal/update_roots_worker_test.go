package internal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func workerAddRussianText(t *testing.T, root string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "system", "packaging", "home", "text.json"))
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "system", "locales", "ru", "packaging", "home", "text.json")
	if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(dest, raw, 0700); err != nil {
		t.Fatal(err)
	}
	sum := filepath.Join(root, "SHA256SUMS")
	f, err := os.OpenFile(sum, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fmt.Fprintf(f, "%s  system/locales/ru/packaging/home/text.json\n", updateDigest(raw))
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestWorkerUpdateRootsMetadata(t *testing.T) {
	for _, scenario := range []string{"init", "en", "ru", "changed-root", "changed-prompt", "partial-root"} {
		t.Run(scenario, func(t *testing.T) {
			base := t.TempDir()
			home := filepath.Join(base, "home")
			candidate := filepath.Join(base, "candidate")
			acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
			acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
			if scenario == "ru" {
				workerAddRussianText(t, home)
				workerAddRussianText(t, candidate)
			}
			if r := InstallHome(home, false); !r.OK {
				t.Fatal(r)
			}
			if scenario == "en" || scenario == "ru" || scenario == "changed-prompt" {
				language := "en"
				if scenario == "ru" {
					language = "ru"
				}
				if r := CoordinatorPrepare(home, language, "Fictional project"); !r.OK {
					t.Fatal(r)
				}
			}
			ctx, info, err := verifyFactoryHome(home)
			if err != nil {
				t.Fatal(err)
			}
			db, err := openLayout(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec("UPDATE project SET stopped=1"); err != nil {
				t.Fatal(err)
			}
			db.Close()
			receipt, err := readInstallationReceiptContext(ctx)
			if err != nil {
				t.Fatal(err)
			}
			canary := filepath.Join(home, "OCMI_Sync", "private-canary")
			if err = os.WriteFile(canary, []byte("preserve private"), 0600); err != nil {
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
			writer, err := acquireUpdateRecoveryWriter(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Release()
			if err = activateUpdateSystem(ctx, &j, writer, lock); err != nil {
				t.Fatal(err)
			}
			if scenario == "partial-root" {
				first := updateRootEntries(*j.NewStaged)[0]
				stageRoot := filepath.Join(home, filepath.FromSlash(j.Stage), "new")
				j.Phase = "root_switch"
				if err = os.Rename(filepath.Join(stageRoot, first.Path), filepath.Join(home, first.Path)); err != nil {
					t.Fatal(err)
				}
				if err = verifyUpdateRootPlacement(ctx, j); err != nil {
					t.Fatalf("unrecorded rename: %v", err)
				}
				collision := filepath.Join(stageRoot, first.Path)
				if err = os.WriteFile(collision, []byte("foreign slot"), 0600); err != nil {
					t.Fatal(err)
				}
				if err = verifyUpdateRootPlacement(ctx, j); err == nil {
					t.Fatal("accepted foreign successor slot")
				}
				if err = os.Remove(collision); err != nil {
					t.Fatal(err)
				}
				j.RootProgress = 1
				if err = verifyUpdateRootPlacement(ctx, j); err != nil {
					t.Fatalf("recorded rename: %v", err)
				}
				if err = os.WriteFile(filepath.Join(home, first.Path), []byte("foreign"), 0600); err != nil {
					t.Fatal(err)
				}
				if err = verifyUpdateRootPlacement(ctx, j); err == nil {
					t.Fatal("accepted altered mixed root")
				}
				return
			}
			changed := ""
			if scenario == "changed-root" {
				changed = filepath.Join(home, "README.md")
			}
			if scenario == "changed-prompt" {
				changed = filepath.Join(home, "local", "bindings", "coordinator", "STARTUP.md")
			}
			if changed != "" {
				if err = os.WriteFile(changed, []byte("foreign change"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			rootsHook, metadataHook := false, false
			updateTestPhase = func(phase string) {
				switch phase {
				case "roots":
					rootsHook = true
					if j.Phase != "roots" || verifyPublicInventory(home) != nil {
						t.Error("roots phase not complete")
					}
					raw, err := installationRead(home, ctx.Paths["bindings"]+"/"+installationReceiptName, 1<<20)
					if err != nil || !bytes.Equal(raw, meta.ReceiptBefore) {
						t.Error("receipt changed before roots hook")
					}
				case "metadata":
					metadataHook = true
					if j.Phase != "metadata" || verifyUpdateMetadataPlacement(ctx, j) != nil {
						t.Error("metadata hook not complete")
					}
				}
			}
			defer func() { updateTestPhase = nil }()
			err = activateUpdateRootsAndMetadata(ctx, &j, writer, lock)
			if changed != "" {
				if err == nil || rootsHook || metadataHook {
					t.Fatalf("accepted changed input: %v", err)
				}
				raw, e := os.ReadFile(changed)
				if e != nil || string(raw) != "foreign change" {
					t.Fatal("changed input overwritten")
				}
				return
			}
			if err != nil || !rootsHook || !metadataHook || j.Phase != "applied" {
				t.Fatalf("activation: %v phase=%s roots=%v metadata=%v", err, j.Phase, rootsHook, metadataHook)
			}
			after, err := readInstallationReceiptContext(ctx)
			if err != nil || after.InstallationID != receipt.InstallationID || after.DeviceID != receipt.DeviceID || after.CopyID != receipt.CopyID || after.SourceCommit != strings.Repeat("b", 40) {
				t.Fatal("receipt identity changed")
			}
			raw, err := os.ReadFile(canary)
			if err != nil || string(raw) != "preserve private" {
				t.Fatal("private canary changed")
			}
			var stopped int
			if err = writer.conn.QueryRowContext(t.Context(), "SELECT stopped FROM project").Scan(&stopped); err != nil || stopped != 1 {
				t.Fatal("STOP changed")
			}
		})
	}
}
