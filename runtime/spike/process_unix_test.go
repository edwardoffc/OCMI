//go:build linux || darwin

package spike

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
)

type ownedProcess struct {
	cmd    *exec.Cmd
	closed bool
}

func startOwned(root, mode string) (*ownedProcess, error) {
	cmd := helperCommand(context.Background(), mode, root)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &ownedProcess{cmd: cmd}, nil
}

func (p *ownedProcess) close() error {
	if p.closed {
		return nil
	}
	// The fixture retains its unreaped parent identity and never detaches its descendants.
	err := syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	if err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	waitErr := p.cmd.Wait()
	p.closed = true
	var exit *exec.ExitError
	if waitErr != nil && !errors.As(waitErr, &exit) {
		return waitErr
	}
	return nil
}
