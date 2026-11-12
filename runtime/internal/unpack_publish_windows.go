//go:build windows

package internal

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

type unpackRenameInformation struct {
	ReplaceIfExists uint32
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

// Open the DELETE handle only after all factory reads have closed: ordinary
// Go directory readers do not share DELETE access on Windows.
func unpackPublish(parent *os.Root, source, destination, parentID, sourceID string) error {
	if !unpackPublicationNames(source, destination) {
		return errors.New("publication names")
	}
	p, err := parent.Open(".")
	if err != nil {
		return err
	}
	defer p.Close()
	id, err := localNativeIDFile(p)
	if err != nil || id != parentID {
		return errors.New("parent identity")
	}
	name, err := windows.NewNTUnicodeString(filepath.FromSlash(source))
	if err != nil {
		return err
	}
	oa := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(p.Fd()), ObjectName: name, Attributes: windows.OBJ_CASE_INSENSITIVE}
	oa.Length = uint32(unsafe.Sizeof(oa))
	var h windows.Handle
	var status windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&h, windows.DELETE|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE, &oa, &status, nil, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_OPEN, windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(h), source)
	defer f.Close()
	var info windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(h, &info); err != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return errors.New("source type")
	}
	id, err = localNativeIDFile(f)
	if err != nil || id != sourceID {
		return errors.New("source identity")
	}
	leaf, err := windows.UTF16FromString(destination)
	if err != nil {
		return err
	}
	leaf = leaf[:len(leaf)-1]
	var layout unpackRenameInformation
	length := int(unsafe.Offsetof(layout.FileName)) + len(leaf)*2
	buffer := make([]byte, length)
	rename := (*unpackRenameInformation)(unsafe.Pointer(&buffer[0]))
	rename.RootDirectory = windows.Handle(p.Fd())
	rename.FileNameLength = uint32(len(leaf) * 2)
	copy(unsafe.Slice(&rename.FileName[0], len(leaf)), leaf)
	err = windows.NtSetInformationFile(h, &status, &buffer[0], uint32(length), windows.FileRenameInformation)
	runtime.KeepAlive(buffer)
	return err
}
