//go:build linux

package internal

import (
	"fmt"
	"os"
	"strings"
)

func resourceChildState(pid int) (byte, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}
	i := strings.LastIndex(string(b), ") ")
	if i < 0 || i+2 >= len(b) {
		return 0, fmt.Errorf("short process state")
	}
	return b[i+2], nil
}
