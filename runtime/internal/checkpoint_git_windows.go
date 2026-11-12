//go:build windows

package internal

import "golang.org/x/sys/windows"

func checkpointGitReparse(path string) bool {
	attributes, err := windows.GetFileAttributes(windows.StringToUTF16Ptr(path))
	return err != nil || attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
