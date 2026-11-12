package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalResourceKnownInventoryRefusesUnexpectedCleanup(t *testing.T) {
	root := t.TempDir()
	if _, e := createLocalResource(root, "managed", "directory", "token", nil, false); e != nil {
		t.Fatal(e)
	}
	p, e := captureLocalResource(root, "managed", "directory", "", false)
	if e != nil {
		t.Fatal(e)
	}
	p, e = writeLocalResource(root, "managed", "directory", p.NativeIdentity, p.Inventory, "known.txt", []byte("known"), false)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "managed", "unexpected.txt"), []byte("keep"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = removeLocalResource(root, "managed", "directory", p.NativeIdentity, p.Inventory, false); e == nil {
		t.Fatal("unexpected inventory accepted")
	}
	if _, e = os.Stat(filepath.Join(root, "managed", "unexpected.txt")); e != nil {
		t.Fatal(e)
	}
}

func TestLocalResourceDirectoryProvenanceAndNestedRemoval(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "parent"), 0700); err != nil {
		t.Fatal(err)
	}
	p, err := createLocalResource(root, "parent/managed", "directory", "token", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(root, "parent", "managed", "tmp"), 0700); err != nil {
		t.Fatal(err)
	}
	p, err = captureLocalResource(root, "parent/managed", "directory", p.NativeIdentity, false)
	if err != nil {
		t.Fatal(err)
	}
	p, err = writeLocalResource(root, "parent/managed", "directory", p.NativeIdentity, p.Inventory, "tmp/result.txt", []byte("result"), false)
	if err != nil {
		t.Fatal(err)
	}
	if err = removeLocalResource(root, "parent/managed", "directory", p.NativeIdentity, p.Inventory, false); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Lstat(filepath.Join(root, "parent", "managed")); !os.IsNotExist(err) {
		t.Fatalf("managed directory remains: %v", err)
	}
}

func TestLocalResourceEmptyChildAndDirectoryBound(t *testing.T) {
	root := t.TempDir()
	p, err := createLocalResource(root, "managed", "directory", "token", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(root, "managed", "tmp"), 0700); err != nil {
		t.Fatal(err)
	}
	p, err = captureLocalResource(root, "managed", "directory", p.NativeIdentity, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = removeLocalResource(root, "managed", "directory", p.NativeIdentity, p.Inventory, false); err != nil {
		t.Fatal(err)
	}

	p, err = createLocalResource(root, "bounded", "directory", "token", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 256; i++ {
		if err = os.Mkdir(filepath.Join(root, "bounded", fmt.Sprintf("d%03d", i)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = captureLocalResource(root, "bounded", "directory", p.NativeIdentity, false); err == nil {
		t.Fatal("empty directories bypassed entry bound")
	}
}

func TestExternalLocalInspectionResolvesTargetAndKind(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "tool.txt")
	if err := os.WriteFile(file, []byte("tool"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectLocalResource(root, file, "directory", true); err == nil {
		t.Fatal("file accepted as directory")
	}
	if p, err := inspectLocalResource(root, file, "file", true); err != nil || p.NativeIdentity == "" || p.Inventory != "" {
		t.Fatalf("file metadata=%+v err=%v", p, err)
	}
	if err := os.Symlink(file, filepath.Join(root, "selected-link")); err != nil {
		t.Skipf("native link privilege unavailable: %v", err)
	}
	if p, err := inspectLocalResource(root, filepath.Join(root, "selected-link"), "file", true); err != nil || p.NativeIdentity == "" {
		t.Fatalf("valid selected link metadata=%+v err=%v", p, err)
	}
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "broken-link")); err != nil {
		t.Skipf("native link privilege unavailable: %v", err)
	}
	if _, err := inspectLocalResource(root, filepath.Join(root, "broken-link"), "file", true); err == nil {
		t.Fatal("broken selected link accepted")
	}
}
