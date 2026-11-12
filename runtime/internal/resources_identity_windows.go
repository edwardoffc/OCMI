//go:build windows

package internal

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
)

func localNativeID(path string) (string, error) {
	h, e := windows.CreateFile(windows.StringToUTF16Ptr(path), windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if e != nil {
		return "", e
	}
	defer windows.CloseHandle(h)
	var i windows.ByHandleFileInformation
	if e = windows.GetFileInformationByHandle(h, &i); e != nil {
		return "", e
	}
	return fmtNativeID(i.VolumeSerialNumber, i.FileIndexHigh, i.FileIndexLow), nil
}
func localNativeIDFile(f *os.File) (string, error) {
	var i windows.ByHandleFileInformation
	if e := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &i); e != nil {
		return "", e
	}
	return fmtNativeID(i.VolumeSerialNumber, i.FileIndexHigh, i.FileIndexLow), nil
}
func fmtNativeID(a, b, c uint32) string { return fmt.Sprintf("%08x-%08x%08x", a, b, c) }

var _ = os.ErrNotExist
