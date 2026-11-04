//go:build darwin

package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"golang.org/x/sys/unix"
)

func recoveryContext() (string, error) {
	value, err := unix.Sysctl("kern.bootsessionuuid")
	if err != nil || strings.TrimSpace(value) == "" {
		if err != nil {
			return "", err
		}
		return "", errors.New("missing native context")
	}
	h := sha256.Sum256([]byte("darwin\x00" + strings.TrimSpace(value)))
	return hex.EncodeToString(h[:]), nil
}
