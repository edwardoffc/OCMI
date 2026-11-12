package internal

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestWorkerUpdateRecovery(t *testing.T) {
	for _, scenario := range []string{"intent", "staged", "old_system", "new_system", "root_switch", "metadata_switch", "applied", "recovery_mid", "altered-stage", "altered-public", "altered-metadata"} {
		t.Run(scenario, func(t *testing.T) {
			base := t.TempDir()
			home := filepath.Join(base, "home")
			candidate := filepath.Join(base, "candidate")
			acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
			acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
			if r := InstallHome(home, false); !r.OK {
				t.Fatal(r)
			}
			if scenario == "metadata_switch" || scenario == "altered-metadata" {
				if r := CoordinatorPrepare(home, "en", "Fictional project"); !r.OK {
					t.Fatal(r)
				}
			}
			ctx, info, err := verifyFactoryHome(home)
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := installationRead(home, ctx.Paths["bindings"]+"/"+installationReceiptName, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			before := acceptancePublicSnapshot(t, home)
			db, err := openLayout(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec("UPDATE project SET stopped=1"); err != nil {
				t.Fatal(err)
			}
			db.Close()
			canary := filepath.Join(home, "OCMI_Sync", "private-canary")
			if err = os.WriteFile(canary, []byte("later private"), 0600); err != nil {
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
			writer, err := acquireUpdateRecoveryWriter(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Release()
			if scenario != "intent" {
				if err = createUpdateStage(ctx, &j, home, candidate); err != nil {
					t.Fatal(err)
				}
				if err = copyUpdateStage(ctx, &j, home, candidate); err != nil {
					t.Fatal(err)
				}
			}
			if scenario != "intent" && scenario != "staged" && scenario != "altered-stage" {
				updateTestPhase = func(phase string) {
					if phase == scenario {
						panic("stop-at-phase")
					}
				}
				func() {
					defer func() {
						if v := recover(); v != nil && v != "stop-at-phase" {
							panic(v)
						}
					}()
					err = activateUpdateSystem(ctx, &j, writer, lock)
				}()
				updateTestPhase = nil
				if err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "root_switch" {
				first := updateRootEntries(*j.NewStaged)[0]
				j.Phase = "root_switch"
				if err = persistUpdateProofJournal(ctx, j); err != nil {
					t.Fatal(err)
				}
				if err = os.Rename(filepath.Join(home, filepath.FromSlash(j.Stage), "new", first.Path), filepath.Join(home, first.Path)); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "metadata_switch" || scenario == "applied" || scenario == "recovery_mid" || scenario == "altered-public" || scenario == "altered-metadata" {
				if err = activateUpdateRootsAndMetadata(ctx, &j, writer, lock); err != nil {
					t.Fatal(err)
				}
				if scenario == "metadata_switch" {
					j.Phase = "metadata_switch"
					j.MetadataProgress = 0
					for _, m := range updateMetadataMembers(ctx, j.Metadata)[1:] {
						if err = installationWrite(home, m.path, m.before); err != nil {
							t.Fatal(err)
						}
					}
					if err = persistUpdateProofJournal(ctx, j); err != nil {
						t.Fatal(err)
					}
				}
				if scenario == "recovery_mid" {
					j.RecoveryFrom = "applied"
					j.Phase = "recovering"
					if err = persistUpdateProofJournal(ctx, j); err != nil {
						t.Fatal(err)
					}
					if err = os.Rename(filepath.Join(home, "system"), filepath.Join(home, filepath.FromSlash(j.Stage), "new", "system")); err != nil {
						t.Fatal(err)
					}
					j.RecoveryStep = 1
					if err = persistUpdateProofJournal(ctx, j); err != nil {
						t.Fatal(err)
					}
				}
			}
			changed := ""
			switch scenario {
			case "altered-stage":
				changed = filepath.Join(home, filepath.FromSlash(j.Stage), "new", "README.md")
			case "altered-public":
				changed = filepath.Join(home, "README.md")
			case "altered-metadata":
				changed = filepath.Join(home, "local", "bindings", "coordinator", "STARTUP.md")
			}
			if changed != "" {
				if err = os.WriteFile(changed, []byte("foreign WIP"), 0600); err != nil {
					t.Fatal(err)
				}
				systemID, _ := localNativeID(filepath.Join(home, "system"))
				if err = recoverUpdatePublic(ctx, &j, writer, lock); err == nil {
					t.Fatal("accepted altered state")
				}
				raw, e := os.ReadFile(changed)
				if e != nil || string(raw) != "foreign WIP" {
					t.Fatal("altered bytes overwritten")
				}
				afterID, _ := localNativeID(filepath.Join(home, "system"))
				if afterID != systemID {
					t.Fatal("refusal changed system")
				}
				return
			}
			if err = recoverUpdatePublic(ctx, &j, writer, lock); err != nil {
				t.Fatal(err)
			}
			if j.Phase != "rolled_back" {
				t.Fatal(j.Phase)
			}
			after := acceptancePublicSnapshot(t, home)
			if len(after) != len(before) {
				t.Fatal("inventory changed")
			}
			for name, sha := range before {
				if after[name] != sha {
					t.Fatalf("public changed: %s", name)
				}
			}
			raw, err := installationRead(home, ctx.Paths["bindings"]+"/"+installationReceiptName, 1<<20)
			if err != nil || !bytes.Equal(raw, receipt) {
				t.Fatal("receipt not restored")
			}
			raw, err = os.ReadFile(canary)
			if err != nil || string(raw) != "later private" {
				t.Fatal("private WIP lost")
			}
			var stopped int
			if err = writer.conn.QueryRowContext(t.Context(), "SELECT stopped FROM project").Scan(&stopped); err != nil || stopped != 1 {
				t.Fatal("STOP lost")
			}
			if err = recoverUpdatePublic(ctx, &j, writer, lock); err != nil {
				t.Fatalf("repeat recovery: %v", err)
			}
		})
	}
}
