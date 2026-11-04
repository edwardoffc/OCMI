//go:build !linux && !darwin

package internal

import "fmt"

func resourceChildState(pid int) (byte, error) {
	return 0, fmt.Errorf("unsupported native child state")
}
