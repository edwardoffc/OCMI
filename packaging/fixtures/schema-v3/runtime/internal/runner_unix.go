//go:build !windows

package internal

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"
)

// runOwned places the command in its own process group. It covers ordinary
// non-detaching descendants; supervisor-death reconciliation remains later work.
func runOwned(ctx context.Context, spec commandSpec) (commandResult, error) {
	cmd := exec.Command(spec.Executable, spec.Args...)
	cmd.Dir = spec.Cwd
	cmd.Env = boundedEnv()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	buf := &limitedBuffer{limit: spec.OutputLimit}
	cmd.Stdout = unixLimitedWriter{buf}
	cmd.Stderr = unixLimitedWriter{buf}
	if err := cmd.Start(); err != nil {
		return commandResult{}, err
	}
	if spec.Registered != nil {
		group := cmd.Process.Pid
		registrationErr := error(nil)
		if group <= 1 {
			registrationErr = errors.New("invalid owned process group")
		} else {
			registrationErr = spec.Registered(cmd.Process.Pid, group)
		}
		if registrationErr != nil {
			killErr := error(nil)
			if group > 1 {
				killErr = syscall.Kill(-group, syscall.SIGKILL)
			}
			if group <= 1 || (killErr != nil && !errors.Is(killErr, syscall.ESRCH)) {
				return commandResult{Unresolved: true}, errors.New("operation registration cleanup unresolved")
			}
			waited := make(chan error, 1)
			go func() { waited <- cmd.Wait() }()
			select {
			case <-waited:
				return commandResult{}, errors.New("operation registration failed")
			case <-time.After(5 * time.Second):
				return commandResult{Unresolved: true}, errors.New("operation registration cleanup unresolved")
			}
		}
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		exit := -1
		if cmd.ProcessState != nil {
			exit = cmd.ProcessState.ExitCode()
		}
		return commandResult{ExitCode: exit, Output: buf.String(), Truncated: buf.truncated}, err
	case <-ctx.Done():
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err != nil && !errors.Is(err, syscall.ESRCH) {
			return commandResult{}, err
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			return commandResult{TimedOut: true, Pending: true}, ctx.Err()
		}
		exit := -1
		if cmd.ProcessState != nil {
			exit = cmd.ProcessState.ExitCode()
		}
		return commandResult{ExitCode: exit, Output: buf.String(), Truncated: buf.truncated, TimedOut: true}, ctx.Err()
	}
}

// unixLimitedWriter deliberately exposes only Write so exec's io.Copy cannot
// bypass limitedBuffer.Write through bytes.Buffer.ReadFrom.
type unixLimitedWriter struct{ buffer *limitedBuffer }

func (w unixLimitedWriter) Write(p []byte) (int, error) { return w.buffer.Write(p) }
