//go:build !windows

package internal

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestUnixRegistrationFailureCleansOwnHeartbeatWithoutStoppingForeignControl(t *testing.T) {
	root := t.TempDir()
	owned, foreign := filepath.Join(root, "owned.txt"), filepath.Join(root, "foreign.txt")
	control := exec.Command(os.Args[0], "--runner-control", foreign)
	if err := control.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = control.Process.Kill(); _, _ = control.Process.Wait() }()
	for until := time.Now().Add(2 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if info, err := os.Stat(foreign); err == nil && info.Size() > 0 {
			break
		}
		if time.Now().After(until) {
			t.Fatal("foreign control did not heartbeat")
		}
	}
	pid, group := 0, 0
	started := time.Now()
	result, err := runOwned(context.Background(), commandSpec{Executable: os.Args[0], Args: []string{"--runner-root", owned}, Cwd: root, OutputLimit: 256, Registered: func(ownPID, ownGroup int) error {
		pid, group = ownPID, ownGroup
		for until := time.Now().Add(2 * time.Second); ; time.Sleep(10 * time.Millisecond) {
			if info, e := os.Stat(owned); e == nil && info.Size() > 0 {
				break
			}
			if time.Now().After(until) {
				return errors.New("own leaf did not heartbeat")
			}
		}
		return errors.New("intentional registration refusal")
	}})
	if err == nil || result.Unresolved || time.Since(started) > 6*time.Second || pid <= 1 || group != pid {
		t.Fatalf("result=%+v err=%v pid=%d group=%d", result, err, pid, group)
	}
	beforeOwn, _ := os.Stat(owned)
	beforeForeign, _ := os.Stat(foreign)
	time.Sleep(180 * time.Millisecond)
	afterOwn, _ := os.Stat(owned)
	afterForeign, _ := os.Stat(foreign)
	if beforeOwn == nil || afterOwn == nil || beforeOwn.Size() != afterOwn.Size() {
		t.Fatal("own leaf kept writing after cleanup")
	}
	if beforeForeign == nil || afterForeign == nil || afterForeign.Size() <= beforeForeign.Size() {
		t.Fatal("foreign control stopped progressing")
	}
}
