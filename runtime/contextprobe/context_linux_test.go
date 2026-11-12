//go:build linux

package contextprobe

import (
	"errors"
	"os"
	"strings"
)

func nativeContext() (string, error) {
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", err
	}
	namespace, err := os.Readlink("/proc/self/ns/pid")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(boot)) == "" || namespace == "" {
		return "", errors.New("missing native context")
	}
	return "linux\x00" + strings.TrimSpace(string(boot)) + "\x00" + namespace, nil
}
