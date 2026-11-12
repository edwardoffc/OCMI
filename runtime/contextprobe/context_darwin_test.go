//go:build darwin

package contextprobe

import (
	"errors"
	"golang.org/x/sys/unix"
	"strings"
)

func nativeContext() (string, error) {
	value, err := unix.Sysctl("kern.bootsessionuuid")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" {
		return "", errors.New("missing native context")
	}
	return "darwin\x00" + strings.TrimSpace(value), nil
}
