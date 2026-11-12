//go:build linux || darwin

package recoveryprobe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const flag = "--ocmi-recovery-probe"

type identity struct{ Parent, Child, Group int }

func TestMain(m *testing.M) {
	if len(os.Args) > 2 && os.Args[1] == flag {
		if err := helper(os.Args[2], os.Args[3:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(90)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func atomicFile(path string, data []byte) error {
	if err := os.WriteFile(path+".tmp", data, 0600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}
func await(path string, predicate func([]byte) bool, timeout time.Duration) ([]byte, error) {
	until := time.Now().Add(timeout)
	for time.Now().Before(until) {
		b, err := os.ReadFile(path)
		if err == nil && predicate(b) {
			return b, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil, fmt.Errorf("bounded observation expired: %s", filepath.Base(path))
}
func helper(mode string, args []string) error {
	switch mode {
	case "effect":
		f, err := os.OpenFile(args[0], os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString("effect\n")
		return err
	case "gate":
		marker, outcome, ready := args[0], args[1], args[2]
		gate := os.NewFile(3, "gate")
		if gate == nil {
			return errors.New("missing gate")
		}
		defer gate.Close()
		id := identity{Parent: os.Getppid(), Child: os.Getpid(), Group: syscall.Getpgrp()}
		b, _ := json.Marshal(id)
		if err := atomicFile(ready, b); err != nil {
			return err
		}
		var token [1]byte
		n, err := gate.Read(token[:])
		if err == io.EOF {
			return atomicFile(outcome, []byte("eof"))
		}
		if err != nil {
			return err
		}
		if n != 1 || token[0] != 'R' {
			return atomicFile(outcome, []byte("refused"))
		}
		cmd := exec.Command(os.Args[0], flag, "effect", marker)
		if err = cmd.Run(); err != nil {
			return err
		}
		return atomicFile(outcome, []byte("released"))
	case "supervisor":
		marker, outcome, ready, childReady := args[0], args[1], args[2], args[3]
		r, w, err := os.Pipe()
		if err != nil {
			return err
		}
		defer r.Close()
		defer w.Close()
		child := exec.Command(os.Args[0], flag, "gate", marker, outcome, childReady)
		child.ExtraFiles = []*os.File{r}
		child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err = child.Start(); err != nil {
			return err
		}
		_ = r.Close()
		raw, err := await(childReady, func(b []byte) bool { var i identity; return json.Unmarshal(b, &i) == nil }, 3*time.Second)
		if err != nil {
			_ = child.Process.Kill()
			_ = child.Wait()
			return err
		}
		var id identity
		if err = json.Unmarshal(raw, &id); err != nil {
			return err
		}
		if id.Parent != os.Getpid() || id.Child != child.Process.Pid || id.Group != id.Child {
			return errors.New("wrong owned identity")
		}
		if err = atomicFile(ready, raw); err != nil {
			return err
		}
		// w stays live through its later Close; no deadlock panic or finalizer-induced early EOF.
		time.Sleep(30 * time.Second)
		_ = w.Close()
		return child.Wait()
	case "control":
		until := time.Now().Add(10 * time.Second)
		for time.Now().Before(until) {
			f, err := os.OpenFile(args[0], os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			_, err = f.WriteString(".")
			_ = f.Close()
			if err != nil {
				return err
			}
			time.Sleep(10 * time.Millisecond)
		}
		return nil
	}
	return errors.New("unknown helper")
}
func command(t *testing.T, mode string, args ...string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], append([]string{flag, mode}, args...)...)
	t.Cleanup(func() {
		if cmd.Process != nil && cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	return cmd
}
func classify(err error) string {
	if err == nil {
		return "live"
	}
	if errors.Is(err, syscall.ESRCH) {
		return "absent"
	}
	return "unknown"
}
func group(id int) string {
	if id <= 1 {
		return "unknown"
	}
	return classify(syscall.Kill(-id, 0))
}
func waitGroup(t *testing.T, id int, want string) {
	t.Helper()
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		if group(id) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("group observation did not reach %s (last %s)", want, group(id))
}
func size(t *testing.T, path string, prior int) int {
	t.Helper()
	b, err := await(path, func(b []byte) bool { return len(b) > prior }, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return len(b)
}
func absentMarker(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unexpected external effect: %v", err)
	}
}
func TestReleaseEOFAndRefusal(t *testing.T) {
	for _, tc := range []struct {
		name, token, outcome string
		effect               bool
	}{
		{"release", "R", "released", true}, {"EOF", "", "eof", false}, {"refuse", "X", "refused", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			marker, outcome, ready := filepath.Join(root, "effect"), filepath.Join(root, "outcome"), filepath.Join(root, "ready")
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			defer w.Close()
			cmd := command(t, "gate", marker, outcome, ready)
			cmd.ExtraFiles = []*os.File{r}
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			_ = r.Close()
			if tc.token != "" {
				if _, err = w.Write([]byte(tc.token)); err != nil {
					t.Fatal(err)
				}
			}
			_ = w.Close()
			if err = cmd.Wait(); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(outcome)
			if err != nil || string(b) != tc.outcome {
				t.Fatalf("gate outcome=%q err=%v", b, err)
			}
			if tc.effect {
				b, err = os.ReadFile(marker)
				if err != nil || string(b) != "effect\n" {
					t.Fatalf("effect=%q err=%v", b, err)
				}
			} else {
				absentMarker(t, marker)
			}
		})
	}
}
func TestActualSupervisorDeathProducesHelperEOF(t *testing.T) {
	root := t.TempDir()
	marker, outcome, ready, childReady := filepath.Join(root, "effect"), filepath.Join(root, "outcome"), filepath.Join(root, "ready"), filepath.Join(root, "child-ready")
	heartbeat := filepath.Join(root, "foreign")
	foreign := command(t, "control", heartbeat)
	foreign.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := foreign.Start(); err != nil {
		t.Fatal(err)
	}
	before := size(t, heartbeat, 0)
	sup := command(t, "supervisor", marker, outcome, ready, childReady)
	if err := sup.Start(); err != nil {
		t.Fatal(err)
	}
	raw, err := await(ready, func(b []byte) bool { var i identity; return json.Unmarshal(b, &i) == nil }, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var id identity
	if err = json.Unmarshal(raw, &id); err != nil {
		t.Fatal(err)
	}
	if id.Parent != sup.Process.Pid || id.Child <= 1 || id.Group != id.Child {
		t.Fatalf("identity=%+v", id)
	}
	if err = sup.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("supervisor not alive: %v", err)
	}
	if group(id.Group) != "live" {
		t.Fatal("helper group not alive before parent death")
	}
	if _, err = os.Stat(outcome); !os.IsNotExist(err) {
		t.Fatal("helper observed EOF before parent death")
	}
	if err = sup.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err = sup.Wait(); err == nil {
		t.Fatal("forced supervisor death unexpectedly succeeded")
	}
	b, err := await(outcome, func(b []byte) bool { return string(b) == "eof" }, 3*time.Second)
	if err != nil {
		t.Fatalf("helper EOF=%q err=%v", b, err)
	}
	waitGroup(t, id.Group, "absent")
	absentMarker(t, marker)
	if size(t, heartbeat, before) <= before {
		t.Fatal("foreign progress missing")
	}
	if group(foreign.Process.Pid) != "live" {
		t.Fatal("foreign process group was affected")
	}
}
func TestLiveAbsentAndUncertainGroupObservation(t *testing.T) {
	root := t.TempDir()
	cmd := command(t, "control", filepath.Join(root, "heartbeat"))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = size(t, filepath.Join(root, "heartbeat"), 0)
	id := cmd.Process.Pid
	if group(id) != "live" {
		t.Fatal("actual owned group not live")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	waitGroup(t, id, "absent")
	if classify(syscall.EPERM) != "unknown" || classify(syscall.EINVAL) != "unknown" || group(0) != "unknown" || group(1) != "unknown" {
		t.Fatal("uncertain observation treated as absence")
	}
	// EPERM classification is a unit control; no native cross-user permission setup is claimed.
}
