//go:build darwin

package internal

import "golang.org/x/sys/unix"

func ownProcessState(pid int) (int, error) {
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return 0, err
	}
	return int(kp.Proc.P_stat), nil
}
func zombieState() int { return 5 } // SZOMB from XNU proc.h; native CI verifies this controlled transition.
