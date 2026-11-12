//go:build windows

package internal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// environmentCanonicalExecutable obtains the canonical target from the opened
// Windows file handle.  EvalSymlinks is not reliable for cross-volume junctions.
func environmentCanonicalExecutable(path string) (string, error) {
	h, err := windows.CreateFile(windows.StringToUTF16Ptr(path), windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, 32768)
	n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), 0)
	if err != nil {
		return "", err
	}
	if n == 0 || n >= uint32(len(buf)) {
		return "", errors.New("canonical executable path")
	}
	p := windows.UTF16ToString(buf[:n])
	if strings.HasPrefix(p, `\\?\UNC\`) {
		p = `\\` + strings.TrimPrefix(p, `\\?\UNC\`)
	} else {
		p = strings.TrimPrefix(p, `\\?\`)
	}
	if !filepath.IsAbs(p) {
		return "", errors.New("canonical executable path")
	}
	return p, nil
}

func environmentSelectedExecutablePath(path string) error {
	lower := strings.ToLower(path)
	if strings.HasPrefix(lower, `\\.\`) || strings.HasPrefix(lower, `\\?\globalroot`) || strings.HasPrefix(lower, `\\?\pipe\`) {
		return errors.New("special executable path")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > environmentExecutableLimit {
		return errors.New("selected executable type")
	}
	return nil
}
