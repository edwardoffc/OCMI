//go:build darwin || linux

package internal

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func unixDirectoryID(t *testing.T, f *os.File) (uint64, uint64) {
	t.Helper()
	info, e := f.Stat()
	if e != nil {
		t.Fatal(e)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("missing Stat_t")
	}
	return uint64(st.Dev), uint64(st.Ino)
}

func TestResourceProviderUnixDirectoryIdentitySurvivesRenameNotReplacement(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	original := filepath.Join(root, "directory")
	if e := os.MkdirAll(original, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(original, "marker.txt"), []byte("copied marker"), 0600); e != nil {
		t.Fatal(e)
	}
	held, e := os.Open(original)
	if e != nil {
		t.Fatal(e)
	}
	defer held.Close()
	beforeDev, beforeIno := unixDirectoryID(t, held)
	if e := os.Rename(original, filepath.Join(root, "prior")); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(original, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(original, "marker.txt"), []byte("copied marker"), 0600); e != nil {
		t.Fatal(e)
	}
	replacement, e := os.Open(original)
	if e != nil {
		t.Fatal(e)
	}
	defer replacement.Close()
	afterDev, afterIno := unixDirectoryID(t, replacement)
	if beforeDev == afterDev && beforeIno == afterIno {
		t.Fatal("replacement directory retained dev/inode identity")
	}
	heldDev, heldIno := unixDirectoryID(t, held)
	if heldDev != beforeDev || heldIno != beforeIno {
		t.Fatal("held directory identity changed")
	}
}

func TestResourceProviderUnixOpenRootKnownEntryCleanupPremise(t *testing.T) {
	base := t.TempDir()
	rootPath := filepath.Join(base, "owned")
	if err := os.Mkdir(rootPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootPath, "known.txt"), []byte("known"), 0600); err != nil {
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
	if err := os.Symlink(outside, filepath.Join(rootPath, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := root.Open("escape.txt"); err == nil {
		t.Fatal("link escape accepted")
	}
	if err := root.Remove("known.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := root.Stat("unexpected.txt"); err != nil {
		t.Fatal("unexpected entry was not preserved")
	}
}
