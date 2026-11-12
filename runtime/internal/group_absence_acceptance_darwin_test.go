//go:build darwin

package internal

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// Fixed independent native control. An unreaped owned root makes Darwin's
// permission observation reproducible without ever targeting another group.
type groupGuardFixture struct {
	cmd                 *exec.Cmd
	waitStarted, joined bool
	done                chan error
}

func ownedZombieForGroupGuard(t *testing.T) *groupGuardFixture {
	t.Helper()
	fixture := &groupGuardFixture{done: make(chan error, 1)}
	fixture.cmd = exec.Command(os.Args[0], "--ocmi-cli-test-helper", t.TempDir(), "0")
	fixture.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := fixture.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if fixture.waitStarted {
			if !fixture.joined {
				<-fixture.done
			}
			return
		}
		_ = fixture.cmd.Process.Kill() // exact direct child, still unreaped
		_ = fixture.cmd.Wait()
	})
	for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		state, err := resourceChildState(fixture.cmd.Process.Pid)
		if err == nil && state == 5 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("owned zombie not observed: state=%v err=%v", state, err)
		}
	}
	if err := syscall.Kill(-fixture.cmd.Process.Pid, 0); !errors.Is(err, syscall.EPERM) {
		t.Fatalf("qualified Darwin permission premise changed: %v", err)
	}
	return fixture
}

func TestDarwinGroupGuardObservesPermissionUntilAbsence(t *testing.T) {
	fixture := ownedZombieForGroupGuard(t)
	fixture.waitStarted = true
	go func() { time.Sleep(150 * time.Millisecond); fixture.done <- fixture.cmd.Wait() }()
	started := time.Now()
	verified := waitGroupAbsent(fixture.cmd.Process.Pid, 2*time.Second)
	elapsed := time.Since(started)
	err := <-fixture.done
	fixture.joined = true
	if err != nil {
		t.Fatal(err)
	}
	if !verified {
		t.Fatal("permission observation prematurely rejected eventual actual absence")
	}
	if elapsed < 100*time.Millisecond || elapsed > 3*time.Second {
		t.Fatalf("premature or unbounded proof: %s", elapsed)
	}
	if err := syscall.Kill(-fixture.cmd.Process.Pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("claimed absence without native absence: %v", err)
	}
}

func TestDarwinGroupGuardNeverTreatsPermissionAsAbsence(t *testing.T) {
	fixture := ownedZombieForGroupGuard(t)
	started := time.Now()
	if waitGroupAbsent(fixture.cmd.Process.Pid, 80*time.Millisecond) {
		t.Fatal("permission became false absence")
	}
	if elapsed := time.Since(started); elapsed < 60*time.Millisecond || elapsed > time.Second {
		t.Fatalf("existing observation budget not used/bounded: %s", elapsed)
	}
}
