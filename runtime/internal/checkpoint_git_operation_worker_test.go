package internal

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckpointGitOperationFinishRetainsUnknown(t *testing.T) {
	op, err := newCheckpointGitOperation(t.TempDir(), "worker", "fictional", "review", "capture")
	if err != nil {
		t.Fatal(err)
	}
	op.mu.Lock()
	op.unknown = true
	op.mu.Unlock()
	out := good("CHECKPOINT_GIT_CAPTURED", 7, map[string]any{})
	op.finish(&out)
	if out.OK || out.Code != "CHECKPOINT_UNRESOLVED" || out.Data["ownedStage"] != op.journalRoot || out.Data["primaryCode"] != "CHECKPOINT_GIT_CAPTURED" {
		t.Fatalf("unknown finish=%+v", out)
	}
	if info, err := os.Stat(op.journalRoot); err != nil || !info.IsDir() {
		t.Fatalf("unknown evidence removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(op.journalRoot, "journal-1.json")); err != nil {
		t.Fatalf("intent journal missing: %v", err)
	}
}

func TestCheckpointGitOperationRegistrationJournalFailureFreezesOperation(t *testing.T) {
	op, err := newCheckpointGitOperation(t.TempDir(), "worker", "fictional", "review", "capture")
	if err != nil {
		t.Fatal(err)
	}
	old := checkpointGitRunOverride
	checkpointGitRunOverride = func(ctx context.Context, spec commandSpec) (commandResult, error) {
		if err := os.RemoveAll(op.journalRoot); err != nil {
			t.Fatal(err)
		}
		if err := spec.Registered(123, 123); err == nil {
			t.Fatal("registered journal failure admitted success")
		}
		return commandResult{StoppedVerified: true}, errors.New("registered journal failed")
	}
	t.Cleanup(func() { checkpointGitRunOverride = old })
	if _, err = op.run(t.TempDir(), nil, commandSpec{}); err == nil {
		t.Fatal("post-launch journal failure returned success")
	}
	if _, err = op.run(t.TempDir(), nil, commandSpec{}); err == nil {
		t.Fatal("unknown operation admitted another launch")
	}
}

func TestCheckpointGitOperationFinishCleansVerifiedSuccess(t *testing.T) {
	op, err := newCheckpointGitOperation(t.TempDir(), "worker", "fictional", "review", "capture")
	if err != nil {
		t.Fatal(err)
	}
	op.mu.Lock()
	op.launched, op.stopped = true, true
	op.mu.Unlock()
	out := good("CHECKPOINT_GIT_CAPTURED", 0, map[string]any{})
	op.finish(&out)
	if !out.OK || out.Code != "CHECKPOINT_GIT_CAPTURED" {
		t.Fatalf("verified finish=%+v", out)
	}
	if _, err := os.Lstat(op.journalRoot); !os.IsNotExist(err) {
		t.Fatalf("verified operation retained: %v", err)
	}
}

func TestCheckpointGitOperationFinishCleansKnownRefusal(t *testing.T) {
	op, err := newCheckpointGitOperation(t.TempDir(), "worker", "fictional", "review", "capture")
	if err != nil {
		t.Fatal(err)
	}
	out := bad("CHECKPOINT_CAPTURE_REFUSED", 4)
	op.finish(&out)
	if out.Code != "CHECKPOINT_CAPTURE_REFUSED" || out.OK {
		t.Fatalf("known refusal changed: %+v", out)
	}
	if _, err := os.Lstat(op.journalRoot); !os.IsNotExist(err) {
		t.Fatalf("known refusal retained owned root: %v", err)
	}
}

func TestCheckpointGitOperationFinishReportsActualGitCleanup(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("native Git unavailable")
	}
	op, err := newCheckpointGitOperation(t.TempDir(), "worker", "fictional", "review", "capture")
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(op.artifactRoot, "fixture")
	template := filepath.Join(op.artifactRoot, "empty-template")
	hooks := filepath.Join(op.artifactRoot, "empty-hooks")
	if err := os.Mkdir(template, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(hooks, 0700); err != nil {
		t.Fatal(err)
	}
	env := checkpointFixtureGitEnv()
	gitArgs := func(args ...string) []string {
		fixed := []string{"-c", "core.hooksPath=" + hooks, "-c", "core.fsmonitor=false", "-c", "maintenance.auto=false", "-c", "commit.gpgSign=false"}
		return append(fixed, args...)
	}
	runGit := func(args ...string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, git, gitArgs(args...)...)
		cmd.Env = env
		if raw, e := cmd.CombinedOutput(); e != nil {
			cancel()
			t.Fatalf("git %v: %v %s", args, e, raw)
		}
		cancel()
	}
	runGit("init", "--template="+template, repo)
	runGit("-C", repo, "config", "user.name", "fixture")
	runGit("-C", repo, "config", "user.email", "fixture@example.com")
	if err := os.WriteFile(filepath.Join(repo, "fixture.txt"), []byte("fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-C", repo, "add", "fixture.txt"}, {"-C", repo, "commit", "-m", "fixture"}} {
		runGit(args...)
	}
	op.mu.Lock()
	op.launched, op.stopped = true, true
	op.mu.Unlock()
	out := good("CHECKPOINT_GIT_CAPTURED", 0, map[string]any{})
	op.finish(&out)
	if out.Code == "CHECKPOINT_CLEANUP_REQUIRED" {
		if out.Data["primaryCode"] != "CHECKPOINT_GIT_CAPTURED" || out.Data["ownedStage"] != op.journalRoot {
			t.Fatalf("cleanup response=%+v", out)
		}
		return
	}
	if !out.OK || out.Code != "CHECKPOINT_GIT_CAPTURED" {
		t.Fatalf("cleanup changed response=%+v", out)
	}
	if _, err := os.Lstat(op.journalRoot); !os.IsNotExist(err) {
		t.Fatalf("owned Git root remains without visible cleanup state: %v", err)
	}
}

func checkpointFixtureGitEnv() []string {
	var env []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToUpper(name), "GIT_") {
			continue
		}
		env = append(env, entry)
	}
	return append(env,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0",
	)
}
