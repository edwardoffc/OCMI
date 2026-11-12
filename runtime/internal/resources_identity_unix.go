//go:build !windows

package internal

import (
	"fmt"
	"os"
	"syscall"
)

func localNativeID(path string) (string, error) {
	i, e := os.Lstat(path)
	if e != nil {
		return "", e
	}
	s, ok := i.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("missing native identity")
	}
	return fmt.Sprintf("%x-%x", s.Dev, s.Ino), nil
}

func localNativeIDFile(f *os.File) (string, error) {
	i, e := f.Stat()
	if e != nil {
		return "", e
	}
	s, ok := i.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("missing native identity")
	}
	return fmt.Sprintf("%x-%x", s.Dev, s.Ino), nil
}
