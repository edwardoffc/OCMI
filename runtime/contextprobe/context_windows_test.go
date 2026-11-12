//go:build windows

package contextprobe

import (
	"errors"
	"golang.org/x/sys/windows/registry"
	"os"
	"strings"
)

func nativeContext() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", err
	}
	defer key.Close()
	value, _, err := key.GetStringValue("MachineGuid")
	if err != nil {
		return "", err
	}
	host, err := os.Hostname()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" || host == "" {
		return "", errors.New("missing native context")
	}
	return "windows\x00" + strings.TrimSpace(value) + "\x00" + host, nil
}
