package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWorkerUpdateRetentionChild(t *testing.T) {
	home := os.Getenv("WORKER_RETENTION_HOME")
	if home == "" {
		t.Skip("child only")
	}
	action := os.Getenv("WORKER_RETENTION_ACTION")
	candidate := os.Getenv("WORKER_RETENTION_CANDIDATE")
	crash := os.Getenv("WORKER_RETENTION_CRASH")
	mutation := os.Getenv("WORKER_RETENTION_MUTATION")
	if crash == "old_system" {
		updateTestPhase = func(phase string) {
			if phase == crash {
				os.Exit(89)
			}
		}
	}
	if crash == "retirement" {
		updateCleanupTestStep = func(string) { os.Exit(91) }
	}
	if mutation == "cleanup-foreign" {
		inserted := false
		updateCleanupTestStep = func(string) {
			if inserted {
				return
			}
			inserted = true
			ctx, code := resolveLayout(home, "installation")
			if code != "" {
				t.Fatal(code)
			}
			j, err := readUpdateProofJournal(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(home, filepath.FromSlash(j.Stage), "foreign-cleanup"), []byte("retain"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if mutation != "" {
		updateTestPhase = func(phase string) {
			if phase != "metadata" {
				return
			}
			ctx, code := resolveLayout(home, "installation")
			if code != "" {
				t.Fatal(code)
			}
			j, err := readUpdateProofJournal(ctx)
			if err != nil || j.Previous == nil {
				t.Fatalf("previous journal: %v", err)
			}
			stage := filepath.Join(home, filepath.FromSlash(j.Previous.Stage))
			if mutation == "foreign" {
				if err = os.WriteFile(filepath.Join(stage, "foreign"), []byte("retain"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err = os.Rename(stage, stage+"-original"); err != nil {
					t.Fatal(err)
				}
				if err = os.Mkdir(stage, 0700); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	r := UpdateHome(action, home, candidate, os.Getenv("WORKER_RETENTION_OLD"), os.Getenv("WORKER_RETENTION_NEW"))
	if mutation == "cleanup-foreign" {
		if r.OK || r.Data["phase"] != "rolled_back" || r.Data["cleanup"] != "pending" || r.Data["reason"] != "cleanup_pending" {
			t.Fatalf("cleanup failure status: %+v", r)
		}
		return
	}
	want := os.Getenv("WORKER_RETENTION_PHASE")
	if !r.OK || r.Data["phase"] != want {
		t.Fatalf("%s: %+v", action, r)
	}
	if os.Getenv("WORKER_RETENTION_PENDING") == "1" && r.Data["cleanup"] != "pending" {
		t.Fatalf("missing pending status: %+v", r)
	}
}

func TestWorkerUpdateRetention(t *testing.T) {
	for _, scenario := range []string{"chain", "interrupted-update", "interrupted-retirement", "foreign", "replaced", "cleanup-failure"} {
		t.Run(scenario, func(t *testing.T) {
			base := t.TempDir()
			home := filepath.Join(base, "home")
			b := filepath.Join(base, "candidate-b")
			c := filepath.Join(base, "candidate-c")
			acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
			bEntry := acceptanceUpdateFactory(t, b, strings.Repeat("b", 40))
			cEntry := acceptanceUpdateFactory(t, c, strings.Repeat("c", 40))
			if r := InstallHome(home, false); !r.OK {
				t.Fatal(r)
			}
			canary := filepath.Join(home, "OCMI_Sync", "canary")
			if err := os.WriteFile(canary, []byte("initial"), 0600); err != nil {
				t.Fatal(err)
			}
			run := func(entry, action, old, next, phase, crash, mutation string, pending bool) {
				t.Helper()
				cmd := exec.Command(entry, "-test.run=^TestWorkerUpdateRetentionChild$")
				cmd.Env = append(os.Environ(), "WORKER_RETENTION_HOME="+home, "WORKER_RETENTION_CANDIDATE="+filepath.Dir(filepath.Dir(filepath.Dir(entry))), "WORKER_RETENTION_ACTION="+action, "WORKER_RETENTION_OLD="+strings.Repeat(old, 40), "WORKER_RETENTION_NEW="+strings.Repeat(next, 40), "WORKER_RETENTION_PHASE="+phase, "WORKER_RETENTION_CRASH="+crash, "WORKER_RETENTION_MUTATION="+mutation)
				if pending {
					cmd.Env = append(cmd.Env, "WORKER_RETENTION_PENDING=1")
				}
				out, err := cmd.CombinedOutput()
				if crash != "" {
					want := 89
					if crash == "retirement" {
						want = 91
					}
					if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != want {
						t.Fatalf("crash %s: %v %s", crash, err, out)
					}
					return
				}
				if err != nil {
					t.Fatalf("%s: %v %s", action, err, out)
				}
			}
			run(bEntry, "apply", "a", "b", "applied", "", "", false)
			ctx, code := resolveLayout(home, "installation")
			if code != "" {
				t.Fatal(code)
			}
			previous, err := readUpdateProofJournal(ctx)
			if err != nil {
				t.Fatal(err)
			}
			previousStage := filepath.Join(home, filepath.FromSlash(previous.Stage))
			if err = os.WriteFile(canary, []byte("later private WIP"), 0600); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "cleanup-failure":
				run(bEntry, "rollback", "a", "b", "rolled_back", "", "cleanup-foreign", false)
				current, err := readUpdateProofJournal(ctx)
				if err != nil || current.Phase != "rolled_back" || current.CleanupState != "pending" {
					t.Fatalf("cleanup journal: %+v %v", current, err)
				}
				if _, err = os.Stat(filepath.Join(previousStage, "foreign-cleanup")); err != nil {
					t.Fatal("foreign cleanup file lost")
				}
				inspection := UpdateHome("inspect", home, "", "", "")
				if !inspection.OK || inspection.Data["phase"] != "rolled_back" || inspection.Data["cleanup"] != "pending" {
					t.Fatalf("cleanup inspection: %+v", inspection)
				}
				recovery := UpdateHome("recover", home, "", "", "")
				if recovery.OK || recovery.Data["phase"] != "rolled_back" || recovery.Data["cleanup"] != "pending" {
					t.Fatalf("cleanup recovery refusal: %+v", recovery)
				}
				return
			case "interrupted-update":
				run(cEntry, "apply", "b", "c", "", "old_system", "", false)
				run(cEntry, "recover", "b", "c", "rolled_back", "", "", false)
				failed, err := readUpdateProofJournal(ctx)
				if err != nil || failed.Phase != "rolled_back" || failed.Previous == nil || failed.Previous.ID != previous.ID || failed.CleanupState != "done" {
					t.Fatalf("failed generation: %+v %v", failed, err)
				}
				if _, err = os.Stat(previousStage); err != nil {
					t.Fatal("previous stage lost")
				}
				failedID := failed.ID
				run(cEntry, "apply", "b", "c", "applied", "", "", false)
				current, err := readUpdateProofJournal(ctx)
				if err != nil || current.LastFailure == nil || current.LastFailure.ID != failedID || current.LastFailure.RecoveryFrom != "old_system" {
					t.Fatalf("failure fact lost: %v", err)
				}
			case "interrupted-retirement":
				run(cEntry, "apply", "b", "c", "", "retirement", "", false)
				current, err := readUpdateProofJournal(ctx)
				if err != nil || current.Phase != "applied" || current.Previous == nil || current.Previous.ID != previous.ID || current.RetirementState != "pending" || len(current.RetirementEntries) == 0 {
					t.Fatalf("retirement intent: %v", err)
				}
				run(cEntry, "recover", "b", "c", "applied", "", "", false)
			case "foreign", "replaced":
				run(cEntry, "apply", "b", "c", "applied", "", scenario, true)
				current, err := readUpdateProofJournal(ctx)
				if err != nil || current.Phase != "applied" || current.Previous == nil || current.Previous.ID != previous.ID || current.RetirementState != "pending" {
					t.Fatalf("pending status: %v", err)
				}
				if scenario == "foreign" {
					if _, err = os.Stat(filepath.Join(previousStage, "foreign")); err != nil {
						t.Fatal("foreign file lost")
					}
				} else {
					if _, err = os.Stat(previousStage + "-original"); err != nil {
						t.Fatal("original stage lost")
					}
				}
				if r := UpdateHome("apply", home, c, strings.Repeat("c", 40), strings.Repeat("d", 40)); r.OK || r.Data["reason"] != "cleanup_pending" {
					t.Fatalf("pending did not block new generation: %+v", r)
				}
				beforeReplay := workerUpdateTreeSnapshot(t, home)
				run(cEntry, "apply", "b", "c", "applied", "", "", true)
				if !reflect.DeepEqual(beforeReplay, workerUpdateTreeSnapshot(t, home)) {
					t.Fatal("pending replay wrote or cleaned")
				}
				run(cEntry, "recover", "b", "c", "applied", "", "", true)
				return
			default:
				run(cEntry, "apply", "b", "c", "applied", "", "", false)
			}
			current, err := readUpdateProofJournal(ctx)
			if err != nil || current.Phase != "applied" || current.Previous != nil || current.LastRetired == nil || current.LastRetired.ID != previous.ID {
				t.Fatalf("latest generation: %+v %v", current, err)
			}
			if !updatePathAbsent(previousStage) {
				t.Fatal("retired stage remains")
			}
			if _, err = os.Stat(filepath.Join(home, filepath.FromSlash(current.Stage))); err != nil {
				t.Fatal("latest stage missing")
			}
			if err = os.WriteFile(canary, []byte("post-C private WIP"), 0600); err != nil {
				t.Fatal(err)
			}
			run(cEntry, "rollback", "b", "c", "rolled_back", "", "", false)
			_, info, err := verifyFactoryHome(home)
			if err != nil || info.SourceCommit != strings.Repeat("b", 40) {
				t.Fatal("rollback did not restore B")
			}
			raw, err := os.ReadFile(canary)
			if err != nil || string(raw) != "post-C private WIP" {
				t.Fatal("private WIP lost")
			}
		})
	}
}
