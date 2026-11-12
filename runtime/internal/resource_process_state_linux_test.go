//go:build linux

package internal

import (
	"fmt"
	"os"
	"strings"
)

func ownProcessState(pid int) (int, error) {
	b, e := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if e != nil {
		return 0, e
	}
	tail := b[strings.LastIndex(string(b), ") ")+2:]
	if len(tail) == 0 {
		return 0, fmt.Errorf("short stat")
	}
	return int(tail[0]), nil
}
func zombieState() int { return int('Z') }
