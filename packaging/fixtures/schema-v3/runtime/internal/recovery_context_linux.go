//go:build linux

package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
)

func recoveryContext() (string, error) {
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", err
	}
	namespace, err := os.Readlink("/proc/self/ns/pid")
	if err != nil || strings.TrimSpace(string(boot)) == "" || namespace == "" {
		if err != nil {
			return "", err
		}
		return "", errors.New("missing native context")
	}
	h := sha256.Sum256([]byte("linux\x00" + strings.TrimSpace(string(boot)) + "\x00" + namespace))
	return hex.EncodeToString(h[:]), nil
}
