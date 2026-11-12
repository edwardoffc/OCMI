package internal

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCheckpointOwnershipWorkerJournalCollisionFreezesEffects(t *testing.T) {
	op, err := newCheckpointGitOperation(t.TempDir(), "fixture", "phase collision", "review", "inspect")
	if err != nil {
		t.Fatal(err)
	}
	defer op.cancel()
	foreign := filepath.Join(op.journalRoot, "journal-"+strconv.Itoa(op.sequence+1)+".json")
	if err := os.WriteFile(foreign, []byte("foreign"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := op.record("next-phase", 0, 0, ""); err == nil || !op.unknown {
		t.Fatal("receipt failure did not freeze operation")
	}
	prior := checkpointGitRunOverride
	checkpointGitRunOverride = func(context.Context, commandSpec) (commandResult, error) {
		t.Fatal("frozen receipt admitted helper")
		return commandResult{}, nil
	}
	t.Cleanup(func() { checkpointGitRunOverride = prior })
	if _, err := op.run(t.TempDir(), nil, commandSpec{}); err == nil {
		t.Fatal("frozen operation continued")
	}
	out := good("CHECKPOINT_GIT_INSPECTED", 0, nil)
	op.finish(&out)
	if out.OK || out.Data["ownedStage"] == nil || out.Data["nextAction"] == nil {
		t.Fatal(out)
	}
	if raw, err := os.ReadFile(foreign); err != nil || string(raw) != "foreign" {
		t.Fatalf("foreign receipt changed: %q %v", raw, err)
	}
}

func TestCheckpointOwnershipWorkerRetiredInputNotReadopted(t *testing.T) {
	op, err := newCheckpointGitOperation(t.TempDir(), "fixture", "input lifecycle", "review", "capture")
	if err != nil {
		t.Fatal(err)
	}
	g, err := op.newGroup(op.artifactRoot, "capture-", "git-capture", nil)
	if err != nil {
		t.Fatal(err)
	}
	f, err := checkpointGitTemp(op, g.Path, []byte("issued"))
	if err != nil {
		t.Fatal(err)
	}
	if !g.Issued[filepath.Base(f.Name())] {
		t.Fatal("input issuance absent")
	}
	if err := op.closeInput(f); err != nil {
		t.Fatal(err)
	}
	if g.Issued[filepath.Base(f.Name())] {
		t.Fatal("input retirement absent")
	}
	if err := os.WriteFile(f.Name(), []byte("foreign reappearance"), 0600); err != nil {
		t.Fatal(err)
	}
	out := good("CHECKPOINT_GIT_CAPTURED", 0, nil)
	op.finish(&out)
	if out.OK || out.Data["ownedStage"] == nil || out.Data["nextAction"] == nil {
		t.Fatal(out)
	}
	if raw, err := os.ReadFile(f.Name()); err != nil || string(raw) != "foreign reappearance" {
		t.Fatalf("retired input readopted: %q %v", raw, err)
	}
}

func TestCheckpointGitOwnershipWorkerRoundTrips(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		parent, source := t.TempDir(), t.TempDir()
		want := []byte("portable\x00\xff\r\n")
		if err := os.WriteFile(filepath.Join(source, "note.txt"), want, 0600); err != nil {
			t.Fatal(err)
		}
		closed := filepath.Join(parent, "package")
		if got := checkpointRequest(t, source, "checkpoint.capture", checkpointPayload(closed)); !got.OK {
			t.Fatal(got)
		}
		if err := os.Remove(filepath.Join(source, "note.txt")); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(source); err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(parent, "destination")
		if err := os.Mkdir(destination, 0700); err != nil {
			t.Fatal(err)
		}
		if got := checkpointRequest(t, destination, "checkpoint.inspect", map[string]any{"packagePath": closed}); !got.OK {
			t.Fatal(got)
		}
		if got := checkpointRequest(t, destination, "checkpoint.restore", map[string]any{"packagePath": closed}); !got.OK {
			t.Fatal(got)
		}
		if raw, err := os.ReadFile(filepath.Join(destination, "note.txt")); err != nil || !bytes.Equal(raw, want) {
			t.Fatalf("restored bytes: %q %v", raw, err)
		}
		if got := checkpointRequest(t, destination, "checkpoint.restore", map[string]any{"packagePath": closed}); got.OK {
			t.Fatal("occupied replay accepted")
		}
		entries, err := os.ReadDir(parent)
		if err != nil || len(entries) != 2 {
			t.Fatalf("normal close retained temporary groups: %v %v", entries, err)
		}
	})
	t.Run("git", func(t *testing.T) {
		source := checkpointGitFixtureForAcceptance(t)
		tool, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		run := func(root string, args ...string) []byte {
			cmd := exec.Command(tool, append([]string{"-c", "core.hooksPath=", "-c", "core.longpaths=true"}, args...)...)
			cmd.Dir = root
			cmd.Env = checkpointFixtureGitEnv()
			raw, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("round-trip Git %v: %v %s", args, err, raw)
			}
			return raw
		}
		if err := os.WriteFile(filepath.Join(source, "file.txt"), []byte("staged\n"), 0600); err != nil {
			t.Fatal(err)
		}
		run(source, "add", "file.txt")
		work := []byte("unstaged\x00\xff\n")
		if err := os.WriteFile(filepath.Join(source, "file.txt"), work, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, "extra.txt"), []byte("extra\n"), 0600); err != nil {
			t.Fatal(err)
		}
		head := run(source, "rev-parse", "HEAD")
		refs := run(source, "show-ref")
		index := run(source, "ls-files", "--stage", "-z")
		parent := t.TempDir()
		closed := filepath.Join(parent, "package")
		payload := map[string]any{"packagePath": closed, "checkpointId": "roundtrip", "systemVersion": "1.3.5", "sourceIdentity": "fictional", "owner": "fixture", "purpose": "roundtrip", "retainUntil": "review", "nextAction": "rebind", "references": []any{}, "refs": []string{"refs/heads/main"}, "files": []string{"extra.txt"}}
		if got := checkpointGitAcceptanceRequest(source, "checkpoint.git.capture", payload); !got.OK {
			t.Fatal(got)
		}
		// The complete package must not depend on its original source location.
		if err := os.Rename(source, source+"-offline"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Rename(source+"-offline", source); err != nil {
				t.Error(err)
			}
		})
		destination := filepath.Join(parent, "destination")
		if err := os.Mkdir(destination, 0700); err != nil {
			t.Fatal(err)
		}
		if got := checkpointGitAcceptanceRequest(destination, "checkpoint.git.inspect", map[string]any{"packagePath": closed}); !got.OK {
			t.Fatal(got)
		}
		if got := checkpointGitAcceptanceRequest(destination, "checkpoint.git.restore", map[string]any{"packagePath": closed}); !got.OK {
			t.Fatal(got)
		}
		for _, check := range []struct {
			args []string
			want []byte
		}{{[]string{"rev-parse", "HEAD"}, head}, {[]string{"show-ref"}, refs}, {[]string{"ls-files", "--stage", "-z"}, index}} {
			if got := run(destination, check.args...); !bytes.Equal(got, check.want) {
				t.Fatalf("Git state %v changed: %q %q", check.args, got, check.want)
			}
		}
		if raw, err := os.ReadFile(filepath.Join(destination, "file.txt")); err != nil || !bytes.Equal(raw, work) {
			t.Fatalf("worktree changed: %q %v", raw, err)
		}
		if raw, err := os.ReadFile(filepath.Join(destination, "extra.txt")); err != nil || string(raw) != "extra\n" {
			t.Fatalf("extra changed: %q %v", raw, err)
		}
		if got := checkpointGitAcceptanceRequest(destination, "checkpoint.git.restore", map[string]any{"packagePath": closed}); got.OK {
			t.Fatal("occupied Git replay accepted")
		}
		if raw, err := os.ReadFile(filepath.Join(closed, "manifest.json")); err != nil || strings.Contains(string(raw), "RootID") || strings.Contains(string(raw), parent) {
			t.Fatalf("private ownership leaked: %q %v", raw, err)
		}
		entries, err := os.ReadDir(parent)
		if err != nil || len(entries) != 2 {
			t.Fatalf("normal Git close retained temporary groups: %v %v", entries, err)
		}
	})
}

func TestCheckpointOwnershipWorkerUnknownMemberPreserved(t *testing.T) {
	op, err := newCheckpointGitOperation(t.TempDir(), "fixture", "owned producer", "review", "inspect")
	if err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(op.artifactRoot, "unexpected.txt")
	if err := os.WriteFile(foreign, []byte("foreign"), 0600); err != nil {
		t.Fatal(err)
	}
	out := good("CHECKPOINT_GIT_INSPECTED", 0, nil)
	op.finish(&out)
	if out.OK || out.Data["ownedStage"] == nil || out.Data["nextAction"] == nil {
		t.Fatal(out)
	}
	if raw, err := os.ReadFile(foreign); err != nil || string(raw) != "foreign" {
		t.Fatalf("unknown member lost: %q %v", raw, err)
	}
}

func TestCheckpointOwnershipWorkerPublishCollisionPreserved(t *testing.T) {
	parent := t.TempDir()
	op, err := newCheckpointGitOperation(parent, "fixture", "producer", "review", "capture")
	if err != nil {
		t.Fatal(err)
	}
	out := good("CHECKPOINT_CAPTURED", 0, nil)
	defer op.finish(&out)
	g, err := op.newGroup(op.artifactRoot, "capture-", "git-capture", nil)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(parent, "occupied")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	id, err := localNativeID(destination)
	if err != nil {
		t.Fatal(err)
	}
	if err := op.publish(g, destination); err == nil {
		t.Fatal("occupied publication accepted")
	}
	if got, err := localNativeID(destination); err != nil || got != id {
		t.Fatalf("occupied identity changed: %q %v", got, err)
	}
	if err := g.identities(); err != nil {
		t.Fatalf("original source lost on collision: %v", err)
	}
}

func TestCheckpointOwnershipWorkerReservationCollisionPreserved(t *testing.T) {
	parent := t.TempDir()
	destinationPath := filepath.Join(parent, "destination")
	if err := os.Mkdir(destinationPath, 0700); err != nil {
		t.Fatal(err)
	}
	destination, err := checkpointHold(destinationPath, "original-empty-destination", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer destination.closeHandles()
	op, err := newCheckpointGitOperation(parent, "fixture", "restore", "review", "restore")
	if err != nil {
		t.Fatal(err)
	}
	out := good("CHECKPOINT_RESTORED", 0, nil)
	defer op.finish(&out)
	if err = op.recordDestination(destination); err != nil {
		t.Fatal(err)
	}
	g, err := op.newGroup(op.artifactRoot, "restore-", "git-restore", nil)
	if err != nil {
		t.Fatal(err)
	}
	backup := op.journalRoot + "-empty-root"
	if err := os.Mkdir(backup, 0700); err != nil {
		t.Fatal(err)
	}
	foreignID, err := localNativeID(backup)
	if err != nil {
		t.Fatal(err)
	}
	if err := op.activate(g, destination); err == nil {
		t.Fatal("reservation collision accepted")
	}
	if err := destination.identities(); err != nil {
		t.Fatalf("original destination lost: %v", err)
	}
	if got, err := localNativeID(backup); err != nil || got != foreignID {
		t.Fatalf("foreign reservation lost: %q %v", got, err)
	}
}
