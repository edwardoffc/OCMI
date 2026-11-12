package internal

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Main's fixed tests; worker-owned tests do not replace these consumer boundaries.
func TestCheckpointGitOwnedInputFile(t *testing.T) {
	tool, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	raw := []byte("owned binary input\x00\xff\r\n")
	path := filepath.Join(root, "input")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sum := sha1.Sum(append([]byte(fmt.Sprintf("blob %d\x00", len(raw))), raw...))
	want := hex.EncodeToString(sum[:])
	control := exec.Command(tool, "hash-object", "--stdin")
	control.Dir, control.Stdin = root, strings.NewReader(string(raw))
	control.Env = effectiveEnv([]string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull})
	known, err := control.Output()
	if err != nil || strings.TrimSpace(string(known)) != want {
		t.Fatalf("native Git input control failed: %v %q", err, known)
	}
	spec := commandSpec{Executable: tool, Cwd: root, Args: []string{"hash-object", "--stdin"},
		OutputLimit: 4096, Env: []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull}}
	field := reflect.ValueOf(&spec).Elem().FieldByName("Stdin")
	if !field.IsValid() || field.Type() != reflect.TypeOf(f) {
		t.Fatal("owned runner cannot consume the already-open Git input file")
	}
	field.Set(reflect.ValueOf(f))
	registered := false
	spec.Registered = func(pid, group int) error { registered = pid > 0; return nil }
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := runOwned(ctx, spec)
	if err != nil || result.ExitCode != 0 || !registered || !result.StoppedVerified || result.Unresolved || result.TimedOut || result.Truncated || strings.TrimSpace(result.Output) != want {
		t.Fatalf("owned stdin result: %+v err=%v", result, err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	registered = false
	result, err = runOwned(ctx, spec)
	if err == nil || registered || !result.StoppedVerified {
		t.Fatalf("closed input must refuse before a child starts: %+v err=%v registered=%v", result, err, registered)
	}
}

func checkpointGitFixtureForAcceptance(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	template := t.TempDir()
	tool, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command(tool, append([]string{"-c", "core.hooksPath=", "-c", "core.fsmonitor=false",
			"-c", "user.name=fixture", "-c", "user.email=fixture@example.com"}, args...)...)
		cmd.Dir = root
		cmd.Env = effectiveEnv([]string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull})
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture Git %v: %v %s", args, err, output)
		}
	}
	run("init", "--initial-branch=main", "--object-format=sha1", "--template="+template)
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("source bytes\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", "file.txt")
	run("commit", "--no-gpg-sign", "-m", "Fixture")
	return root
}

func checkpointGitAcceptanceRequest(root, action string, payload any) Response {
	raw, _ := json.Marshal(payload)
	return Handle(Request{SchemaVersion: 1, Action: action, ProjectRoot: root, Actor: "main",
		RequestID: "fixed-git-check", Payload: raw})
}

func TestCheckpointGitIndependentActivationAndDrift(t *testing.T) {
	source := checkpointGitFixtureForAcceptance(t)
	parent := t.TempDir()
	packagePath := filepath.Join(parent, "package")
	payload := map[string]any{"packagePath": packagePath, "checkpointId": "fixture", "systemVersion": "1.3.5",
		"sourceIdentity": "fixture-source", "owner": "main", "purpose": "fixed acceptance",
		"retainUntil": "qualification", "nextAction": "confirm binding", "references": []any{},
		"refs": []string{"refs/heads/main"}, "files": []string{}}
	checkpointGitCaptureBeforeVerify = func() {
		if err := os.WriteFile(filepath.Join(source, "file.txt"), []byte("changed\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { checkpointGitCaptureBeforeVerify = nil; checkpointGitRestoreBeforeActivation = nil })
	response := checkpointGitAcceptanceRequest(source, "checkpoint.git.capture", payload)
	checkpointGitCaptureBeforeVerify = nil
	if response.OK {
		t.Fatal("source drift became a complete checkpoint")
	}
	if _, err := os.Lstat(packagePath); !os.IsNotExist(err) {
		t.Fatalf("failed drift left published package: %v", err)
	}
	response = checkpointGitAcceptanceRequest(source, "checkpoint.git.capture", payload)
	if !response.OK {
		t.Fatalf("capture control: %+v", response)
	}
	for _, mode := range []string{"foreign-arrival", "staged-bytes", "staged-index", "staged-extra"} {
		t.Run(mode, func(t *testing.T) {
			destination := filepath.Join(parent, mode)
			if err := os.Mkdir(destination, 0700); err != nil {
				t.Fatal(err)
			}
			reached := false
			checkpointGitRestoreBeforeActivation = func(stagedRepository string) error {
				reached = true
				path := filepath.Join(stagedRepository, "file.txt")
				if mode == "foreign-arrival" {
					path = filepath.Join(destination, "foreign.txt")
				} else if mode == "staged-index" {
					path = filepath.Join(stagedRepository, ".git", "index")
				} else if mode == "staged-extra" {
					path = filepath.Join(stagedRepository, "unexpected.txt")
				}
				return os.WriteFile(path, []byte("unexpected\n"), 0600)
			}
			response := checkpointGitAcceptanceRequest(destination, "checkpoint.git.restore", map[string]string{"packagePath": packagePath})
			checkpointGitRestoreBeforeActivation = nil
			if !reached {
				t.Fatalf("restore refused before the activation control: %+v", response)
			}
			if response.OK {
				t.Fatal("changed activation accepted")
			}
			entries, err := os.ReadDir(destination)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "foreign-arrival" {
				data, err := os.ReadFile(filepath.Join(destination, "foreign.txt"))
				if err != nil || string(data) != "unexpected\n" || len(entries) != 1 {
					t.Fatal("foreign destination was moved, modified or replaced")
				}
			} else if len(entries) != 0 {
				t.Fatal("failed staging became active")
			}
		})
	}
}

func TestCheckpointGitIndependentIndexDrift(t *testing.T) {
	source := checkpointGitFixtureForAcceptance(t)
	parent := t.TempDir()
	packagePath := filepath.Join(parent, "package")
	if err := os.WriteFile(filepath.Join(source, "late.txt"), []byte("new staged bytes\n"), 0600); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"packagePath": packagePath, "checkpointId": "index-drift", "systemVersion": "1.3.5",
		"sourceIdentity": "fixture-source", "owner": "main", "purpose": "fixed index drift",
		"retainUntil": "qualification", "nextAction": "confirm binding", "references": []any{},
		"refs": []string{"refs/heads/main"}, "files": []string{}}
	checkpointGitCaptureBeforeVerify = func() {
		tool, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, tool, "-c", "core.hooksPath=", "-c", "core.fsmonitor=false", "add", "late.txt")
		cmd.Dir = source
		cmd.Env = effectiveEnv([]string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull})
		if raw, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("native index mutation control: %v %s", err, raw)
		}
	}
	t.Cleanup(func() { checkpointGitCaptureBeforeVerify = nil })
	response := checkpointGitAcceptanceRequest(source, "checkpoint.git.capture", payload)
	checkpointGitCaptureBeforeVerify = nil
	if response.OK {
		t.Fatal("index-only source change produced a stale successful checkpoint")
	}
	if _, err := os.Lstat(packagePath); !os.IsNotExist(err) {
		t.Fatalf("index drift left a published checkpoint: %v", err)
	}
}

func TestCheckpointGitUnknownHelperPreservesOwnedJournal(t *testing.T) {
	source := checkpointGitFixtureForAcceptance(t)
	parent := t.TempDir()
	packagePath := filepath.Join(parent, "package")
	payload := map[string]any{"packagePath": packagePath, "checkpointId": "unknown-fixture", "systemVersion": "1.3.5",
		"sourceIdentity": "fixture-source", "owner": "main-unknown-control", "purpose": "fixed unknown-helper proof",
		"retainUntil": "explicit inspection", "nextAction": "confirm binding", "references": []any{},
		"refs": []string{"refs/heads/main"}, "files": []string{}}
	before, err := os.ReadFile(filepath.Join(source, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	registeredPID := 0
	checkpointGitRunOverride = func(ctx context.Context, spec commandSpec) (commandResult, error) {
		calls++
		// Execute the actual owned Git command, then simulate lost stop evidence at
		// the wrapper boundary. Native ownership is never fabricated or signalled.
		stages, _ := filepath.Glob(filepath.Join(parent, ".ocmi-git-*"))
		journalPresent := false
		for _, stage := range stages {
			_ = filepath.WalkDir(stage, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr == nil && entry.Type().IsRegular() && strings.HasSuffix(path, ".json") {
					raw, _ := os.ReadFile(path)
					journalPresent = journalPresent || (strings.Contains(string(raw), "main-unknown-control") && strings.Contains(string(raw), "fixed unknown-helper proof"))
				}
				return nil
			})
		}
		if !journalPresent || spec.Registered == nil {
			t.Fatal("Git helper admitted before owned durable intent/registration")
		}
		registered := spec.Registered
		spec.Registered = func(pid, group int) error {
			registeredPID = pid
			return registered(pid, group)
		}
		realResult, realErr := runOwned(ctx, spec)
		if realErr != nil || !realResult.StoppedVerified || registeredPID <= 0 {
			t.Fatalf("native helper control did not finish: %+v %v", realResult, realErr)
		}
		return commandResult{Unresolved: true, Phase: "fixed-unknown"}, errors.New("fictional unresolved helper")
	}
	t.Cleanup(func() { checkpointGitRunOverride = nil })
	response := checkpointGitAcceptanceRequest(source, "checkpoint.git.capture", payload)
	checkpointGitRunOverride = nil
	if response.OK || calls != 1 {
		t.Fatalf("unknown helper completed or more work ran: %+v calls=%d", response, calls)
	}
	if _, err := os.Lstat(packagePath); !os.IsNotExist(err) {
		t.Fatal("unknown operation published a package")
	}
	owned, ok := response.Data["ownedStage"].(string)
	if !ok || !filepath.IsAbs(owned) || filepath.Dir(owned) != parent {
		t.Fatalf("unresolved owned resource lacks explicit inspection path: %+v", response)
	}
	foundPID := false
	err = filepath.WalkDir(owned, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type().IsRegular() && strings.HasSuffix(path, ".json") {
			raw, _ := os.ReadFile(path)
			foundPID = foundPID || strings.Contains(string(raw), fmt.Sprint(registeredPID))
		}
		return nil
	})
	if err != nil || !foundPID {
		t.Fatalf("unresolved helper evidence was removed/lost: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(source, ".git", "index"))
	if err != nil || string(after) != string(before) {
		t.Fatal("unknown helper changed source index")
	}
}
