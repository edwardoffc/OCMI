//go:build windows

package internal

import "os"

// Opening for write does not modify the executable. A live Windows image can
// deny this operation, in which case activation must not begin.
func probeInstalledImage(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	return f.Close()
}
