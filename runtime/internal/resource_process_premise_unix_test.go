//go:build darwin || linux

package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestResourceProcessOwnUnreapedChildStateAndGroupCleanupPremise(t *testing.T) {
	root := t.TempDir()
	own, foreign := filepath.Join(root, "own.txt"), filepath.Join(root, "foreign.txt")
	control := exec.Command(os.Args[0], "--runner-control", foreign)
	if err := control.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = control.Process.Kill(); _, _ = control.Process.Wait() }()
	for until := time.Now().Add(2 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if info, e := os.Stat(foreign); e == nil && info.Size() > 0 {
			break
		}
		if time.Now().After(until) {
			t.Fatal("foreign control did not heartbeat")
		}
	}
	cmd := exec.Command(os.Args[0], "--runner-root", own)
	cmd.Dir = root
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	reaped := false
	defer func() {
		if !reaped {
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			_, _ = cmd.Process.Wait()
		}
	}()
	seenActive := false
	seenZombie := false
	for until := time.Now().Add(2 * time.Second); time.Now().Before(until); time.Sleep(10 * time.Millisecond) {
		state, err := ownProcessState(pid)
		if err != nil {
			continue
		}
		if state != zombieState() {
			seenActive = true
		}
		if state == zombieState() {
			seenZombie = true
			break
		}
	}
	if !seenActive || !seenZombie {
		t.Fatalf("active=%v zombie=%v", seenActive, seenZombie)
	}
	before, _ := os.Stat(foreign)
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		t.Fatal(err)
	}
	waitErr := cmd.Wait()
	reaped = true
	if waitErr != nil {
		t.Fatalf("root wait=%v", waitErr)
	}
	for until := time.Now().Add(2 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if err := syscall.Kill(-pid, 0); err == syscall.ESRCH {
			break
		}
		if time.Now().After(until) {
			t.Fatal("own group remains present")
		}
	}
	time.Sleep(80 * time.Millisecond)
	after, _ := os.Stat(foreign)
	if before == nil || after == nil || after.Size() <= before.Size() {
		t.Fatal("foreign control stopped")
	}
}
