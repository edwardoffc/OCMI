//go:build windows

package internal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProductionRunnerBoundsInheritedOutputAndPreservesControl(t *testing.T) {
	root := t.TempDir()
	owned := filepath.Join(root, "owned")
	control := filepath.Join(root, "control")
	foreign := exec.Command(os.Args[0], "--runner-control", control)
	if err := foreign.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = foreign.Process.Kill(); _ = foreign.Wait() }()
	before := int64(0)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(control); err == nil && info.Size() > 0 {
			before = info.Size()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if before == 0 {
		t.Fatal("control did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := runOwned(ctx, commandSpec{Executable: os.Args[0], Args: []string{"--runner-root", owned}, Cwd: root, OutputLimit: 64})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Pending || !result.Truncated || len(result.Output) > 64 {
		t.Fatalf("result=%+v", result)
	}
	time.Sleep(80 * time.Millisecond)
	info, err := os.Stat(control)
	if err != nil || info.Size() <= before {
		t.Fatalf("foreign control stopped: %v, %v", info, err)
	}
	ownedInfo, err := os.Stat(owned)
	if err != nil || ownedInfo.Size() == 0 {
		t.Fatalf("owned child did not start: %v, %v", ownedInfo, err)
	}
}

func TestWindowsRunnerUsesScopedTempEnvironment(t *testing.T) {
	cmd, err := exec.LookPath("cmd.exe")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	tmp := filepath.Join(root, "owned tmp")
	if err = os.Mkdir(tmp, 0700); err != nil {
		t.Fatal(err)
	}
	result, err := runOwned(context.Background(), commandSpec{
		Executable: cmd, Args: []string{"/d", "/c", "echo %TEMP%&echo %TMP%&echo %TMPDIR%"}, Cwd: root, OutputLimit: 4096,
		Env: []string{"temp=" + tmp, "TMP=" + tmp, "tmpdir=" + tmp},
	})
	if err != nil || result.Pending || !result.StoppedVerified {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, line := range strings.Split(strings.TrimSpace(result.Output), "\n") {
		line = strings.TrimSpace(line)
		if !strings.EqualFold(filepath.Clean(line), filepath.Clean(tmp)) {
			t.Fatalf("child received %q, want %q; output=%q", line, tmp, result.Output)
		}
	}
}

func TestEffectiveEnvironmentIsCaseDeduplicatedAndSorted(t *testing.T) {
	env := effectiveEnv([]string{"temp=owned", "TMP=owned", "tmpdir=owned"})
	seen := map[string]string{}
	for i, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed entry %q", entry)
		}
		key := strings.ToUpper(parts[0])
		if _, exists := seen[key]; exists {
			t.Fatalf("duplicate key %q in %q", key, env)
		}
		seen[key] = parts[1]
		if i > 0 && strings.ToUpper(env[i-1]) > strings.ToUpper(entry) {
			t.Fatalf("environment block unsorted: %q", env)
		}
	}
	for _, key := range []string{"TEMP", "TMP", "TMPDIR"} {
		if seen[key] != "owned" {
			t.Fatalf("%s=%q, want scoped override", key, seen[key])
		}
	}
}
