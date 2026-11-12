//go:build windows

package internal

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

type windowsNativeID struct{ volume, indexHigh, indexLow uint32 }

func heldWindowsID(t *testing.T, f *os.File) windowsNativeID {
	t.Helper()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info); err != nil {
		t.Fatal(err)
	}
	return windowsNativeID{info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow}
}

func TestResourceProviderWindowsNativeIdentityAndBoundedCleanupPremise(t *testing.T) {
	base := t.TempDir()
	rootPath := filepath.Join(base, "owned")
	if err := os.Mkdir(rootPath, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(rootPath, "known.txt")
	if err := os.WriteFile(path, []byte("copied marker"), 0600); err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(windows.StringToUTF16Ptr(path), windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	held := os.NewFile(uintptr(handle), path)
	defer held.Close()
	before := heldWindowsID(t, held)
	if err := os.Rename(path, filepath.Join(rootPath, "prior.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("copied marker"), 0600); err != nil {
		t.Fatal(err)
	}
	replacement, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Close()
	if before == heldWindowsID(t, replacement) {
		t.Fatal("replacement with copied marker retained native identity")
	}
	if before != heldWindowsID(t, held) {
		t.Fatal("held handle did not retain original identity")
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootPath, "unexpected.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := root.Open("../outside"); err == nil {
		t.Fatal("root escape accepted")
	}
	outside := filepath.Join(base, "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(rootPath, "escape.txt")); err == nil {
		if _, err := root.Open("escape.txt"); err == nil {
			t.Fatal("link escape accepted")
		}
	} else {
		t.Logf("link escape control skipped: %v", err)
	}
	if err := root.Remove("known.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := root.Stat("unexpected.txt"); err != nil {
		t.Fatal("unexpected entry was not preserved")
	}
}

func TestResourceProviderWindowsDirectoryIdentitySurvivesRenameNotReplacement(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	original := filepath.Join(root, "directory")
	if err := os.MkdirAll(original, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(original, "marker.txt"), []byte("copied marker"), 0600); err != nil {
		t.Fatal(err)
	}
	openDirectory := func(path string) (*os.File, windowsNativeID) {
		h, err := windows.CreateFile(windows.StringToUTF16Ptr(path), windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
		if err != nil {
			t.Fatal(err)
		}
		f := os.NewFile(uintptr(h), path)
		return f, heldWindowsID(t, f)
	}
	held, before := openDirectory(original)
	if err := os.Rename(original, filepath.Join(root, "prior")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(original, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(original, "marker.txt"), []byte("copied marker"), 0600); err != nil {
		t.Fatal(err)
	}
	replacement, after := openDirectory(original)
	if before == after {
		t.Fatal("replacement directory with copied marker retained native identity")
	}
	if before != heldWindowsID(t, held) {
		t.Fatal("held directory handle lost original identity")
	}
	_ = replacement.Close()
	_ = held.Close()
}
