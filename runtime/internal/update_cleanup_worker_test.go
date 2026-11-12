package internal

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestWorkerUpdateCleanup(t *testing.T) {
	for _, scenario := range []string{"normal", "resume", "resume-foreign", "normal-operations", "foreign", "foreign-inspect", "empty-dir", "replaced-stage", "replaced-slot", "applied"} {
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
			writer, err := acquireUpdateRecoveryWriter(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Release()
			if err = activateUpdateSystem(ctx, &j, writer, lock); err != nil {
				t.Fatal(err)
			}
			if err = activateUpdateRootsAndMetadata(ctx, &j, writer, lock); err != nil {
				t.Fatal(err)
			}
			stage := filepath.Join(home, filepath.FromSlash(j.Stage))
			if scenario == "applied" {
				if err = cleanupUpdateStage(ctx, &j, writer, lock); err == nil {
					t.Fatal("accepted current applied generation")
				}
				if _, err = os.Stat(stage); err != nil {
					t.Fatal("applied stage removed")
				}
				return
			}
			if err = recoverUpdatePublic(ctx, &j, writer, lock); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "foreign", "foreign-inspect":
				err = os.WriteFile(filepath.Join(stage, "old", "foreign"), []byte("foreign"), 0600)
			case "empty-dir":
				err = os.Mkdir(filepath.Join(stage, "new", "foreign"), 0700)
			case "replaced-stage":
				err = os.Rename(stage, stage+"-original")
				if err == nil {
					err = os.Mkdir(stage, 0700)
				}
			case "replaced-slot":
				err = os.Rename(filepath.Join(stage, "old"), filepath.Join(stage, "original-old"))
				if err == nil {
					err = os.Mkdir(filepath.Join(stage, "old"), 0700)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "foreign-inspect" {
				r := UpdateHome("inspect", home, "", "", "")
				if !r.OK || r.Data["phase"] != "rolled_back" || r.Data["cleanup"] != "pending" {
					t.Fatalf("cleanup status: %+v", r)
				}
			}
			if scenario == "resume" || scenario == "resume-foreign" || scenario == "normal-operations" {
				updateCleanupTestStep = func(string) { panic("cleanup-interrupted") }
				func() {
					defer func() {
						if v := recover(); v != "cleanup-interrupted" {
							t.Fatalf("interruption: %v", v)
						}
					}()
					_ = cleanupUpdateStage(ctx, &j, writer, lock)
				}()
				updateCleanupTestStep = nil
				disk, err := readUpdateProofJournal(ctx)
				if err != nil || disk.CleanupState != "pending" {
					t.Fatalf("durable intent: %v", err)
				}
				j = disk
			}
			if scenario == "normal-operations" {
				foreign := filepath.Join(stage, "foreign-after-interruption")
				if err = os.WriteFile(foreign, []byte("retain foreign"), 0600); err != nil {
					t.Fatal(err)
				}
				writer.Release()
				if err = lock.Release(); err != nil {
					t.Fatal(err)
				}
				inspection := UpdateHome("inspect", home, "", "", "")
				if !inspection.OK || inspection.Data["phase"] != "rolled_back" || inspection.Data["cleanup"] != "pending" {
					t.Fatalf("inspection: %+v", inspection)
				}
				if r := InstallHome(home, true); !r.OK {
					t.Fatalf("setup inspect: %+v", r)
				}
				req := Request{SchemaVersion: 1, Action: "stop", ProjectRoot: home, RootKind: "installation", Actor: "coordinator", RequestID: "cleanup-normal-control", Payload: json.RawMessage(`{"enabled":false}`)}
				if r := Handle(req); !r.OK {
					t.Fatalf("normal request: %+v", r)
				}
				if r := UpdateHome("apply", home, candidate, strings.Repeat("a", 40), strings.Repeat("b", 40)); r.OK || r.Data["reason"] != "cleanup_pending" {
					t.Fatalf("new generation not blocked: %+v", r)
				}
				publicPath := filepath.Join(home, "README.md")
				publicRaw, err := os.ReadFile(publicPath)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(publicPath, []byte("foreign public"), 0600); err != nil {
					t.Fatal(err)
				}
				if r := InstallHome(home, true); r.OK {
					t.Fatalf("changed public accepted by setup: %+v", r)
				}
				if r := UpdateHome("inspect", home, "", "", ""); r.OK {
					t.Fatalf("changed public accepted by update: %+v", r)
				}
				if err = os.WriteFile(publicPath, publicRaw, 0600); err != nil {
					t.Fatal(err)
				}
				receiptPath := filepath.Join(home, "local", "bindings", installationReceiptName)
				receiptRaw, err := os.ReadFile(receiptPath)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(receiptPath, []byte("foreign metadata"), 0600); err != nil {
					t.Fatal(err)
				}
				if r := InstallHome(home, true); r.OK {
					t.Fatalf("changed metadata accepted by setup: %+v", r)
				}
				if r := UpdateHome("inspect", home, "", "", ""); r.OK {
					t.Fatalf("changed metadata accepted by update: %+v", r)
				}
				if err = os.WriteFile(receiptPath, receiptRaw, 0600); err != nil {
					t.Fatal(err)
				}
				activation := j
				activation.Phase = "recovering"
				activation.RecoveryFrom = "new_system"
				activation.CleanupState = ""
				activation.CleanupEntries = nil
				if err = persistUpdateProofJournal(ctx, activation); err != nil {
					t.Fatal(err)
				}
				req.RequestID = "cleanup-active-refusal"
				if r := Handle(req); r.OK || r.Code != "UPDATE_PENDING" {
					t.Fatalf("unfinished activation accepted: %+v", r)
				}
				if err = persistUpdateProofJournal(ctx, j); err != nil {
					t.Fatal(err)
				}
				if err = installationWrite(home, ctx.Paths["bindings"]+"/update.json", []byte("{")); err != nil {
					t.Fatal(err)
				}
				req.RequestID = "cleanup-malformed-refusal"
				if r := Handle(req); r.OK || r.Code != "UPDATE_PENDING" {
					t.Fatalf("malformed journal accepted: %+v", r)
				}
				raw, err := os.ReadFile(foreign)
				if err != nil || string(raw) != "retain foreign" {
					t.Fatal("foreign stage content changed")
				}
				return
			}
			if scenario == "resume-foreign" {
				if err = os.WriteFile(filepath.Join(stage, "foreign-after-interruption"), []byte("retain"), 0600); err != nil {
					t.Fatal(err)
				}
				before := workerUpdateTreeSnapshot(t, stage)
				if err = cleanupUpdateStage(ctx, &j, writer, lock); err == nil {
					t.Fatal("accepted foreign member during resume")
				}
				if !reflect.DeepEqual(before, workerUpdateTreeSnapshot(t, stage)) {
					t.Fatal("resume refusal changed stage")
				}
				return
			}
			err = cleanupUpdateStage(ctx, &j, writer, lock)
			if scenario == "foreign" || scenario == "foreign-inspect" || scenario == "empty-dir" || scenario == "replaced-stage" || scenario == "replaced-slot" {
				if err == nil {
					t.Fatal("accepted foreign stage")
				}
				if scenario == "replaced-stage" {
					if _, e := os.Stat(stage + "-original"); e != nil {
						t.Fatal("original stage lost")
					}
				} else if scenario == "replaced-slot" {
					if _, e := os.Stat(filepath.Join(stage, "original-old")); e != nil {
						t.Fatal("original slot lost")
					}
				} else {
					name := "old/foreign"
					if scenario == "empty-dir" {
						name = "new/foreign"
					}
					if _, e := os.Stat(filepath.Join(stage, filepath.FromSlash(name))); e != nil {
						t.Fatal("foreign object removed")
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if j.CleanupState != "done" || !updatePathAbsent(stage) {
				t.Fatal("stage cleanup incomplete")
			}
			if err = cleanupUpdateStage(ctx, &j, writer, lock); err != nil {
				t.Fatalf("repeat cleanup: %v", err)
			}
		})
	}
}

func TestWorkerUpdateRecoveryImageProbe(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows live-image predicate")
	}
	for _, location := range []string{"home", "parked"} {
		t.Run(location, func(t *testing.T) {
			base := t.TempDir()
			home := filepath.Join(base, "home")
			entry := acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
			ctx, code := resolveLayout(home, "installation")
			if code != "" {
				t.Fatal(code)
			}
			j := updateProofJournal{Stage: "local/runs/update-fixture"}
			target := entry
			if location == "parked" {
				target = filepath.Join(home, "local", "runs", "update-fixture", "parked-system", "bin", "ocmi.exe")
				if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
					t.Fatal(err)
				}
				raw, err := os.ReadFile(entry)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(target, raw, 0700); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(target, "-test.run=^TestWorkerUpdateImageChild$")
			cmd.Env = append(os.Environ(), "WORKER_IMAGE_GATE="+base)
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			finished := false
			defer func() {
				if !finished {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
			}()
			deadline := time.Now().Add(10 * time.Second)
			for {
				if _, err = os.Stat(filepath.Join(base, "ready")); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("child not ready")
				}
				time.Sleep(20 * time.Millisecond)
			}
			if err = probeRecoveryImages(ctx, j); err == nil {
				t.Fatal("live recovery image accepted")
			}
			after, err := os.ReadFile(target)
			if err != nil || string(before) != string(after) {
				t.Fatal("image changed")
			}
			if err = os.WriteFile(filepath.Join(base, "release"), []byte("done"), 0600); err != nil {
				t.Fatal(err)
			}
			err = cmd.Wait()
			finished = true
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWorkerUpdateBusyRecoveryChild(t *testing.T) {
	home := os.Getenv("WORKER_BUSY_HOME")
	if home == "" {
		t.Skip("child only")
	}
	r := UpdateHome(os.Getenv("WORKER_BUSY_ACTION"), home, "", "", "")
	if r.OK || r.Data["reason"] != "image_busy" {
		t.Fatalf("busy recovery: %+v", r)
	}
}

func TestWorkerUpdateBusyRecoveryAPI(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows live-image predicate")
	}
	for _, item := range []struct{ location, action string }{{"home", "rollback"}, {"parked", "rollback"}, {"home", "recover"}, {"parked", "recover"}} {
		t.Run(item.location+"-"+item.action, func(t *testing.T) {
			base := t.TempDir()
			home := filepath.Join(base, "home")
			candidate := filepath.Join(base, "candidate")
			acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
			candidateEntry := acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
			if r := InstallHome(home, false); !r.OK {
				t.Fatal(r)
			}
			apply := exec.Command(candidateEntry, "-test.run=^TestWorkerUpdateAPIChild$")
			apply.Env = append(os.Environ(), "WORKER_UPDATE_HOME="+home, "WORKER_UPDATE_CANDIDATE="+candidate, "WORKER_UPDATE_ACTION=apply", "WORKER_UPDATE_PHASE=applied")
			if out, err := apply.CombinedOutput(); err != nil {
				t.Fatalf("apply: %v %s", err, out)
			}
			ctx, code := resolveLayout(home, "installation")
			if code != "" {
				t.Fatal(code)
			}
			j, err := readUpdateProofJournal(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if item.action == "recover" {
				j.Phase = "metadata"
				if err = persistUpdateProofJournal(ctx, j); err != nil {
					t.Fatal(err)
				}
			}
			target := filepath.Join(home, "system", "bin", "ocmi.exe")
			if item.location == "parked" {
				target = filepath.Join(home, filepath.FromSlash(j.Stage), "parked-system", "bin", "ocmi.exe")
			}
			before := workerUpdateTreeSnapshot(t, home)
			cmd := exec.Command(target, "-test.run=^TestWorkerUpdateImageChild$")
			cmd.Env = append(os.Environ(), "WORKER_IMAGE_GATE="+base)
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			finished := false
			defer func() {
				if !finished {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
			}()
			deadline := time.Now().Add(10 * time.Second)
			for {
				if _, err = os.Stat(filepath.Join(base, "ready")); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("live image not ready")
				}
				time.Sleep(20 * time.Millisecond)
			}
			check := exec.Command(candidateEntry, "-test.run=^TestWorkerUpdateBusyRecoveryChild$")
			check.Env = append(os.Environ(), "WORKER_BUSY_HOME="+home, "WORKER_BUSY_ACTION="+item.action)
			if out, err := check.CombinedOutput(); err != nil {
				t.Fatalf("busy refusal: %v %s", err, out)
			}
			after := workerUpdateTreeSnapshot(t, home)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("busy recovery changed home")
			}
			if err = os.WriteFile(filepath.Join(base, "release"), []byte("done"), 0600); err != nil {
				t.Fatal(err)
			}
			err = cmd.Wait()
			finished = true
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
