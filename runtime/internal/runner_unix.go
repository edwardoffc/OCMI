//go:build !windows

package internal

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// runOwned keeps one explicit pipe pump.  A zombie root with a non-EOF pipe
// proves inherited output is still held; group teardown is then a forced,
// incomplete completion even if it later reaches an empty process group.
func runOwned(ctx context.Context, spec commandSpec) (commandResult, error) {
	cmd := exec.Command(spec.Executable, spec.Args...)
	cmd.Dir, cmd.Env = spec.Cwd, effectiveEnv(spec.Env)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if spec.Stdin != nil {
		info, inputErr := spec.Stdin.Stat()
		if inputErr != nil || !info.Mode().IsRegular() {
			return commandResult{StoppedVerified: true}, errors.New("owned stdin unavailable")
		}
		cmd.Stdin = spec.Stdin
	}
	buf := &limitedBuffer{limit: spec.OutputLimit}
	readOutput, writeOutput, err := os.Pipe()
	if err != nil {
		return commandResult{}, err
	}
	defer readOutput.Close()
	cmd.Stdout, cmd.Stderr = writeOutput, writeOutput
	if err = cmd.Start(); err != nil {
		writeOutput.Close()
		return commandResult{StoppedVerified: true}, err
	}
	_ = writeOutput.Close()
	pump := make(chan error, 1)
	pumpState := &outputPumpState{}
	go func() {
		_, e := io.Copy(unixLimitedWriter{buf}, readOutput)
		pumpState.finish(e)
		pump <- e
	}()
	pid := cmd.Process.Pid
	if pid <= 1 {
		err := errors.New("invalid owned process group")
		return commandResult{Unresolved: true, Phase: "registration", ErrorClass: "other", GroupObservation: "unknown", PumpObservation: pumpState.observation(), WaitCalled: observedBool(false), NativeErrno: unixNativeErrno(err)}, err
	}
	if spec.Registered != nil {
		if err = spec.Registered(pid, pid); err != nil {
			return finishUnix(cmd, pid, buf, pump, pumpState, false, false, false, err)
		}
	}
	for {
		state, stateErr := resourceChildState(pid)
		if stateErr != nil {
			return commandResult{Output: buf.String(), Truncated: buf.truncated, Unresolved: true, Phase: "child_state", ErrorClass: unixObservationErrorClass(stateErr), GroupObservation: observeUnixGroup(pid), PumpObservation: pumpState.observation(), WaitCalled: observedBool(false), NativeErrno: unixNativeErrno(stateErr)}, stateErr
		}
		if state == 'Z' || state == 5 {
			eof := pumpEOFWithin(pump, 50*time.Millisecond)
			return finishUnix(cmd, pid, buf, pump, pumpState, eof, false, !eof, nil)
		}
		select {
		case <-ctx.Done():
			return finishUnix(cmd, pid, buf, pump, pumpState, false, true, true, ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func pumpEOF(pump <-chan error) bool {
	select {
	case <-pump:
		return true
	default:
		return false
	}
}

func pumpEOFWithin(pump <-chan error, grace time.Duration) bool {
	if pumpEOF(pump) {
		return true
	}
	select {
	case <-pump:
		return true
	case <-time.After(grace):
		return false
	}
}

// finishUnix never signals after Wait/reap.  Darwin may report EPERM for an
// already-zombie root-only group; only in that exact state may we reap and
// then require non-mutating group absence, never treating EPERM as success.
func finishUnix(cmd *exec.Cmd, pgid int, buf *limitedBuffer, pump <-chan error, pumpState *outputPumpState, pumpDone, timedOut, forced bool, original error) (commandResult, error) {
	state, stateErr := resourceChildState(cmd.Process.Pid)
	if stateErr != nil {
		return commandResult{Output: buf.String(), Truncated: buf.truncated, Unresolved: true, Phase: "teardown", ErrorClass: unixObservationErrorClass(stateErr), GroupObservation: observeUnixGroup(pgid), PumpObservation: pumpState.observation(), WaitCalled: observedBool(false), NativeErrno: unixNativeErrno(stateErr)}, stateErr
	}
	zombie := state == 'Z' || state == 5
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		if !(zombie && errors.Is(err, syscall.EPERM)) {
			return commandResult{Output: buf.String(), Truncated: buf.truncated, TimedOut: timedOut, Unresolved: true, Phase: "signal", ErrorClass: unixObservationErrorClass(err), GroupObservation: observeUnixGroup(pgid), PumpObservation: pumpState.observation(), WaitCalled: observedBool(false), NativeErrno: unixNativeErrno(err)}, err
		}
	}
	w := make(chan error, 1)
	go func() { w <- cmd.Wait() }()
	var waitErr error
	select {
	case waitErr = <-w:
	case <-time.After(nativeTeardownBudget):
		return commandResult{Output: buf.String(), Truncated: buf.truncated, TimedOut: timedOut, Pending: true, Unresolved: true, Phase: "wait", ErrorClass: "none", GroupObservation: "unknown", PumpObservation: pumpState.observation(), WaitCalled: observedBool(true), NativeErrno: unixNativeErrno(original)}, original
	}
	if !pumpDone {
		select {
		case <-pump:
			pumpDone = true
		case <-time.After(nativeTeardownBudget):
		}
	}
	verified := waitGroupAbsent(pgid, nativeTeardownBudget)
	exit := -1
	if cmd.ProcessState != nil {
		exit = cmd.ProcessState.ExitCode()
	}
	pending := !verified || forced || buf.truncated || !pumpDone
	result := commandResult{ExitCode: exit, Output: buf.String(), Truncated: buf.truncated, TimedOut: timedOut, Pending: pending, StoppedVerified: verified, Unresolved: !verified, WaitCalled: observedBool(true)}
	if !verified {
		result.Phase = "group"
		result.ErrorClass = "none"
		result.GroupObservation = "unknown"
		result.PumpObservation = pumpState.observation()
	}
	if original != nil {
		return result, original
	}
	return result, waitErr
}

func unixObservationErrorClass(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, syscall.ESRCH):
		return "not_found"
	case errors.Is(err, syscall.EPERM), errors.Is(err, syscall.EACCES):
		return "permission"
	default:
		return "other"
	}
}

func unixNativeErrno(err error) *int {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return nil
	}
	value := int(errno)
	if value < 0 || value > 4095 {
		return nil
	}
	return &value
}

func observeUnixGroup(pgid int) string {
	if pgid <= 1 {
		return "unknown"
	}
	err := syscall.Kill(-pgid, 0)
	switch {
	case err == nil:
		return "present"
	case errors.Is(err, syscall.ESRCH):
		return "absent"
	case errors.Is(err, syscall.EPERM), errors.Is(err, syscall.EACCES):
		return "permission"
	default:
		return "unknown"
	}
}

func waitGroupAbsent(pgid int, budget time.Duration) bool {
	deadline := time.Now().Add(budget)
	for {
		err := syscall.Kill(-pgid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return true
		}
		// Darwin can report EPERM for an owned unreaped zombie group before
		// the scheduled Wait makes the same group observably absent. Keep this
		// read-only observation inside the existing finite budget; only ESRCH
		// proves absence.
		if err != nil && !errors.Is(err, syscall.EPERM) {
			return false
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
}

type unixLimitedWriter struct{ buffer *limitedBuffer }

func (w unixLimitedWriter) Write(p []byte) (int, error) { return w.buffer.Write(p) }
