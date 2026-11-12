package internal

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func workerUpdateTreeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if e.IsDir() {
			out[rel] = "directory"
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = updateDigest(raw)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestWorkerUpdateAPIChild(t *testing.T) {
	home := os.Getenv("WORKER_UPDATE_HOME")
	if home == "" {
		t.Skip("child only")
	}
	action := os.Getenv("WORKER_UPDATE_ACTION")
	candidate := os.Getenv("WORKER_UPDATE_CANDIDATE")
	crash := os.Getenv("WORKER_UPDATE_CRASH")
	if crash != "" {
		updateTestPhase = func(phase string) {
			if phase == crash {
				os.Exit(89)
			}
		}
	}
	r := UpdateHome(action, home, candidate, strings.Repeat("a", 40), strings.Repeat("b", 40))
	want := os.Getenv("WORKER_UPDATE_PHASE")
	if !r.OK || r.Data["phase"] != want {
		t.Fatalf("action %s: %+v", action, r)
	}
}

func TestWorkerUpdateAPI(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	candidate := filepath.Join(base, "candidate")
	acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
	entry := acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
	if r := InstallHome(home, false); !r.OK {
		t.Fatal(r)
	}
	before := acceptancePublicSnapshot(t, home)
	run := func(action, phase string) {
		t.Helper()
		cmd := exec.Command(entry, "-test.run=^TestWorkerUpdateAPIChild$")
		cmd.Env = append(os.Environ(), "WORKER_UPDATE_HOME="+home, "WORKER_UPDATE_CANDIDATE="+candidate, "WORKER_UPDATE_ACTION="+action, "WORKER_UPDATE_PHASE="+phase)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v %s", action, err, out)
		}
	}
	if r := UpdateHome("inspect", home, "", "", ""); !r.OK || r.Data["phase"] != "idle" {
		t.Fatalf("idle: %+v", r)
	}
	run("apply", "applied")
	ctx, code := resolveLayout(home, "installation")
	if code != "" {
		t.Fatal(code)
	}
	journalBefore, err := installationRead(home, ctx.Paths["bindings"]+"/update.json", 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	j, err := readUpdateProofJournal(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tampered := j
	newStaged := *j.NewStaged
	newStaged.Files = append([]updatePublicEntry(nil), j.NewStaged.Files...)
	newStaged.Files[0].Digest = strings.Repeat("0", 64)
	tampered.NewStaged = &newStaged
	if _, err = encodeUpdateProofJournal(tampered); err == nil {
		t.Fatal("accepted staged proof inconsistent with captured candidate")
	}
	snapshotBeforeReplay := workerUpdateTreeSnapshot(t, home)
	run("apply", "applied")
	if !reflect.DeepEqual(snapshotBeforeReplay, workerUpdateTreeSnapshot(t, home)) {
		t.Fatal("replay changed home files")
	}
	journalAfter, err := installationRead(home, ctx.Paths["bindings"]+"/update.json", 8<<20)
	if err != nil || !bytes.Equal(journalBefore, journalAfter) {
		t.Fatal("replay wrote journal")
	}
	run("recover", "applied")
	run("rollback", "rolled_back")
	cleaned, err := readUpdateProofJournal(ctx)
	if err != nil || cleaned.CleanupState != "done" || !updatePathAbsent(filepath.Join(home, filepath.FromSlash(cleaned.Stage))) {
		t.Fatalf("automatic cleanup: %v", err)
	}
	run("recover", "rolled_back")
	after := acceptancePublicSnapshot(t, home)
	for name, sha := range before {
		if after[name] != sha {
			t.Fatalf("rollback changed %s", name)
		}
	}
	if r := UpdateHome("inspect", home, "", "", ""); !r.OK || r.Data["phase"] != "rolled_back" {
		t.Fatalf("rolled back inspect: %+v", r)
	}
}

func TestWorkerUpdateRenameJournalGaps(t *testing.T) {
	for _, phase := range []string{"intent", "staged", "old_system", "new_system"} {
		t.Run(phase, func(t *testing.T) {
			base := t.TempDir()
			home := filepath.Join(base, "home")
			candidate := filepath.Join(base, "candidate")
			acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
			entry := acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
			if r := InstallHome(home, false); !r.OK {
				t.Fatal(r)
			}
			before := acceptancePublicSnapshot(t, home)
			cmd := exec.Command(entry, "-test.run=^TestWorkerUpdateAPIChild$")
			cmd.Env = append(os.Environ(), "WORKER_UPDATE_HOME="+home, "WORKER_UPDATE_CANDIDATE="+candidate, "WORKER_UPDATE_ACTION=apply", "WORKER_UPDATE_CRASH="+phase)
			out, err := cmd.CombinedOutput()
			if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 89 {
				t.Fatalf("phase crash: %v %s", err, out)
			}
			ctx, code := resolveLayout(home, "installation")
			if code != "" {
				t.Fatal(code)
			}
			j, err := readUpdateProofJournal(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if phase == "old_system" {
				j.Phase = "staged"
			} else if phase == "new_system" {
				j.Phase = "old_system"
			}
			if err = persistUpdateProofJournal(ctx, j); err != nil {
				t.Fatal(err)
			}
			if r := UpdateHome("inspect", home, "", "", ""); r.OK || r.Data["phase"] != "pending" {
				t.Fatalf("pending: %+v", r)
			}
			recovery := exec.Command(entry, "-test.run=^TestWorkerUpdateAPIChild$")
			recovery.Env = append(os.Environ(), "WORKER_UPDATE_HOME="+home, "WORKER_UPDATE_CANDIDATE="+candidate, "WORKER_UPDATE_ACTION=recover", "WORKER_UPDATE_PHASE=rolled_back")
			if out, err = recovery.CombinedOutput(); err != nil {
				t.Fatalf("recover: %v %s", err, out)
			}
			cleaned, err := readUpdateProofJournal(ctx)
			if err != nil || cleaned.CleanupState != "done" || !updatePathAbsent(filepath.Join(home, filepath.FromSlash(cleaned.Stage))) {
				t.Fatalf("recovery cleanup: %v", err)
			}
			after := acceptancePublicSnapshot(t, home)
			for name, sha := range before {
				if after[name] != sha {
					t.Fatalf("gap recovery changed %s", name)
				}
			}
		})
	}
}

func TestWorkerUpdateMalformedBarrier(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
	if r := InstallHome(home, false); !r.OK {
		t.Fatal(r)
	}
	ctx, code := resolveLayout(home, "installation")
	if code != "" {
		t.Fatal(code)
	}
	req := Request{SchemaVersion: 1, Action: "stop", ProjectRoot: home, RootKind: "installation", Actor: "coordinator", RequestID: "barrier-control", Payload: json.RawMessage(`{"enabled":false}`)}
	if r := Handle(req); !r.OK {
		t.Fatalf("control: %+v", r)
	}
	if err := installationWrite(home, ctx.Paths["bindings"]+"/update.json", []byte("{")); err != nil {
		t.Fatal(err)
	}
	req.RequestID = "barrier-refusal"
	if r := Handle(req); r.OK || r.Code != "UPDATE_PENDING" {
		t.Fatalf("service barrier: %+v", r)
	}
	if r := InstallHome(home, false); r.OK {
		t.Fatalf("install barrier: %+v", r)
	}
	if r := CoordinatorPrepare(home, "en", "Fictional project"); r.OK {
		t.Fatalf("prepare barrier: %+v", r)
	}
	if r := CoordinatorConfirm(home, "Fictional project", "user-current", "evidence"); r.OK {
		t.Fatalf("confirm barrier: %+v", r)
	}
	if r := UpdateHome("inspect", home, "", "", ""); r.OK {
		t.Fatalf("inspect accepted malformed journal: %+v", r)
	}
}
