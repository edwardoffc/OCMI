//go:build darwin

package internal

import "golang.org/x/sys/unix"

func resourceChildState(pid int) (byte, error) {
	p, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return 0, err
	}
	return byte(p.Proc.P_stat), nil
}
