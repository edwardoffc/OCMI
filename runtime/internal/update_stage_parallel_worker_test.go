package internal

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func workerParallelStage(t *testing.T) (layoutContext, *updateProofJournal, string, string) {
	t.Helper()
	base := t.TempDir()
	home := filepath.Join(base, "home")
	candidate := filepath.Join(base, "candidate")
	workerBatchFactory(t, home, strings.Repeat("a", 40), nil, 70)
	workerBatchFactory(t, candidate, strings.Repeat("b", 40), nil, 70)
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
	t.Cleanup(func() { _ = lock.Release() })
	id := uuid.NewString()
	j := &updateProofJournal{SchemaVersion: 1, ID: id, Phase: "intent", Metadata: meta, OldSource: meta.OldSource, NewSource: meta.NewSource, OldPublic: old, NewPublic: next, Stage: "local/runs/update-" + id, OldSlot: "old", NewSlot: "new", AdmissionID: lock.nativeID}
	if err = createUpdateStage(ctx, j, home, candidate); err != nil {
		t.Fatal(err)
	}
	return ctx, j, home, candidate
}

func TestWorkerUpdateStageDrain(t *testing.T) {
	for _, scenario := range []string{"source-read", "write", "sync", "sync-panic", "staged-read"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, j, home, candidate := workerParallelStage(t)
			var handles []*os.File
			var readCount, writeCount int
			var syncCount, active atomic.Int32
			injected := errors.New("injected stage error")
			ops := updateStageIO{read: func(root, name string, limit int) ([]byte, error) {
				if root == home {
					readCount++
					if scenario == "source-read" && readCount == 5 {
						return nil, injected
					}
				} else if scenario == "staged-read" && strings.Contains(root, "update-") {
					return nil, injected
				}
				return installationRead(root, name, limit)
			}, write: func(f *os.File, raw []byte) (int, error) {
				handles = append(handles, f)
				writeCount++
				if scenario == "write" && writeCount == 5 {
					n, err := f.Write(raw[:len(raw)/2])
					if err != nil {
						return n, err
					}
					return n, injected
				}
				return f.Write(raw)
			}, sync: func(f *os.File) error {
				active.Add(1)
				defer active.Add(-1)
				n := syncCount.Add(1)
				err := f.Sync()
				if err != nil {
					return err
				}
				if n == 3 && scenario == "sync-panic" {
					panic("injected sync panic")
				}
				if n == 3 && scenario == "sync" {
					return injected
				}
				return nil
			}}
			if err := copyUpdateStageWithIO(ctx, j, home, candidate, ops); err == nil {
				t.Fatal("injected failure accepted")
			}
			if active.Load() != 0 {
				t.Fatal("sync worker still active")
			}
			if int(syncCount.Load()) != len(handles) {
				t.Fatalf("not all handles drained: synced=%d handles=%d", syncCount.Load(), len(handles))
			}
			for _, f := range handles {
				if err := f.Close(); err == nil {
					t.Fatal("handle remained open")
				}
			}
			saved, err := readUpdateProofJournal(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if saved.OldCopied != 0 || saved.NewCopied != 0 {
				t.Fatalf("advanced failed batch: %d/%d", saved.OldCopied, saved.NewCopied)
			}
		})
	}
}

func TestWorkerUpdateStageOutOfOrder(t *testing.T) {
	ctx, j, home, candidate := workerParallelStage(t)
	firstStarted := make(chan struct{})
	otherDone := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	var calls, active, maxActive atomic.Int32
	var handles []*os.File
	var batchFiles int
	var batchBytes int64
	ops := updateStageIO{read: installationRead, write: func(f *os.File, b []byte) (int, error) {
		if batchFiles >= updateStageBatchFiles || batchFiles > 0 && batchBytes+int64(len(b)) > updateStageBatchBytes {
			t.Error("batch bounds exceeded")
		}
		batchFiles++
		batchBytes += int64(len(b))
		handles = append(handles, f)
		return f.Write(b)
	}, sync: func(f *os.File) error {
		n := calls.Add(1)
		now := active.Add(1)
		for {
			max := maxActive.Load()
			if now <= max || maxActive.CompareAndSwap(max, now) {
				break
			}
		}
		defer active.Add(-1)
		if n == 1 {
			close(firstStarted)
			<-release
		}
		err := f.Sync()
		if n == 2 {
			close(otherDone)
		}
		return err
	}}
	observerCalls := 0
	updateStageTestStep = func(string, int) {
		observerCalls++
		batchFiles, batchBytes = 0, 0
		if active.Load() != 0 {
			t.Error("observer ran before workers drained")
		}
		for _, f := range handles {
			if err := f.Close(); err == nil {
				t.Error("observer saw open handle")
				break
			}
		}
	}
	defer func() { updateStageTestStep = nil }()
	go func() { done <- copyUpdateStageWithIO(ctx, j, home, candidate, ops) }()
	released, finished := false, false
	defer func() {
		if !finished {
			if !released {
				close(release)
			}
			<-done
		}
	}()
	select {
	case <-firstStarted:
	case <-time.After(10 * time.Second):
		t.Fatal("first sync did not start")
	}
	select {
	case <-otherDone:
	case <-time.After(10 * time.Second):
		t.Fatal("other sync did not finish first")
	}
	saved, err := readUpdateProofJournal(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if saved.OldCopied != 0 || saved.NewCopied != 0 {
		t.Fatalf("prefix advanced before all syncs: %d/%d", saved.OldCopied, saved.NewCopied)
	}
	close(release)
	released = true
	select {
	case err = <-done:
		finished = true
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(90 * time.Second):
		t.Fatal("stage did not finish")
	}
	if active.Load() != 0 || maxActive.Load() < 2 || maxActive.Load() > updateStageSyncWorkers || observerCalls == 0 {
		t.Fatalf("workers active=%d max=%d observers=%d", active.Load(), maxActive.Load(), observerCalls)
	}
	if j.Phase != "staged" || j.OldCopied != len(j.OldPublic.Files) || j.NewCopied != len(j.NewPublic.Files) {
		t.Fatal("completed stage prefix")
	}
}

func TestWorkerUpdateStageNativeBoundary(t *testing.T) {
	for _, scenario := range []string{"source-replaced", "stage-replaced", "stage-link", "stage-junction"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, j, home, candidate := workerParallelStage(t)
			if scenario == "stage-junction" {
				target := filepath.Join(home, "system")
				link := filepath.Join(home, filepath.FromSlash(j.Stage), "old", "system")
				if runtime.GOOS == "windows" {
					out, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, target).CombinedOutput()
					if err != nil {
						t.Fatalf("junction creation: %v %s", err, out)
					}
				} else if err := os.Symlink(target, link); err != nil {
					t.Fatal(err)
				}
				if err := copyUpdateStage(ctx, j, home, candidate); err == nil {
					t.Fatal("accepted staged directory link")
				}
				return
			}
			if scenario == "stage-link" {
				entry := j.OldPublic.Files[0]
				target := filepath.Join(home, filepath.FromSlash(entry.Path))
				link := filepath.Join(home, filepath.FromSlash(j.Stage), "old", filepath.FromSlash(entry.Path))
				if err := os.Symlink(target, link); err != nil {
					t.Skipf("symbolic-link creation unavailable: %v", err)
				}
				if err := copyUpdateStage(ctx, j, home, candidate); err == nil {
					t.Fatal("accepted staged symbolic link")
				}
				return
			}
			var handles []*os.File
			count := 0
			changed := false
			ops := updateStageIO{read: func(root, name string, limit int) ([]byte, error) {
				if scenario == "source-replaced" && root == home {
					count++
					if count == 5 {
						raw, err := installationRead(root, name, limit)
						if err != nil {
							return nil, err
						}
						path := filepath.Join(root, filepath.FromSlash(name))
						if err = os.Rename(path, path+".moved"); err != nil {
							return nil, err
						}
						if err = os.WriteFile(path, raw, 0700); err != nil {
							return nil, err
						}
						changed = true
					}
				} else if scenario == "stage-replaced" && strings.Contains(root, "update-") && !changed {
					raw, err := installationRead(root, name, limit)
					if err != nil {
						return nil, err
					}
					path := filepath.Join(root, filepath.FromSlash(name))
					if err = os.Rename(path, path+".moved"); err != nil {
						return nil, err
					}
					if err = os.WriteFile(path, raw, 0700); err != nil {
						return nil, err
					}
					changed = true
				}
				return installationRead(root, name, limit)
			}, write: func(f *os.File, b []byte) (int, error) { handles = append(handles, f); return f.Write(b) }, sync: (*os.File).Sync}
			if err := copyUpdateStageWithIO(ctx, j, home, candidate, ops); err == nil || !changed {
				t.Fatal("accepted replaced native member")
			}
			for _, f := range handles {
				if err := f.Close(); err == nil {
					t.Fatal("handle remained open")
				}
			}
			saved, err := readUpdateProofJournal(ctx)
			if err != nil || saved.OldCopied != 0 {
				t.Fatalf("advanced replaced batch: %v", err)
			}
		})
	}
}

func TestWorkerUpdateStageObserverPanic(t *testing.T) {
	ctx, j, home, candidate := workerParallelStage(t)
	var handles []*os.File
	var active atomic.Int32
	ops := updateStageIO{read: installationRead, write: func(f *os.File, b []byte) (int, error) { handles = append(handles, f); return f.Write(b) }, sync: func(f *os.File) error { active.Add(1); defer active.Add(-1); return f.Sync() }}
	updateStageTestStep = func(string, int) {
		if active.Load() != 0 {
			t.Error("observer ran with active workers")
		}
		panic("observer-panic")
	}
	defer func() { updateStageTestStep = nil }()
	observed := false
	func() {
		defer func() {
			if v := recover(); v != nil {
				if v != "observer-panic" {
					panic(v)
				}
				observed = true
			}
		}()
		_ = copyUpdateStageWithIO(ctx, j, home, candidate, ops)
	}()
	if !observed || active.Load() != 0 {
		t.Fatal("observer panic did not drain workers")
	}
	for _, f := range handles {
		if err := f.Close(); err == nil {
			t.Fatal("observer panic left handle open")
		}
	}
	saved, err := readUpdateProofJournal(ctx)
	if err != nil || saved.OldCopied != 32 || saved.NewCopied != 0 {
		t.Fatalf("observer checkpoint: %+v %v", saved, err)
	}
}
