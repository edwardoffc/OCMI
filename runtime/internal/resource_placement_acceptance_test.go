package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// Main-owned acceptance: os.Root allows links within its root, so confinement
// alone must not confer ownership over a linked placement or protected metadata.
func TestResourcePlacementRefusesReservedAndInternalAliases(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{".git", ".ocmi"} {
		if err := os.MkdirAll(filepath.Join(root, name, "local", "runs"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, locator := range []string{".GIT/new.txt", ".OCMI/new.txt", "ordinary/../.git/new.txt"} {
		if _, err := createLocalResource(root, locator, "file", "test-only", []byte("must not be written"), false); err == nil {
			t.Fatalf("protected placement accepted: %s", locator)
		}
	}
	for _, locator := range []string{".ocmi/local/runs/not-a-uuid", ".ocmi/local/runs/01234567-89ab-4cde-8fab-0123456789ab/nested"} {
		if _, err := createLocalResource(root, locator, "directory", "test-only", nil, true); err == nil {
			t.Fatalf("overbroad internal exception accepted: %s", locator)
		}
	}
	if err := os.Symlink(filepath.Join(root, ".git"), filepath.Join(root, "alias")); err != nil {
		t.Skipf("native link privilege unavailable; other placement controls ran: %v", err)
	}
	if _, err := createLocalResource(root, "alias/new.txt", "file", "test-only", []byte("must not be written"), false); err == nil {
		t.Fatal("link inside the project granted ownership over protected metadata")
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("linked protected target changed: %v", err)
	}
}

func TestResourceFileProofRefusesChangedContents(t *testing.T) {
	root := t.TempDir()
	p, err := createLocalResource(root, "owned.txt", "file", "test-only", []byte("original"), false)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "owned.txt"), []byte("changed by outside actor"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = removeLocalResource(root, "owned.txt", "file", p.NativeIdentity, p.Inventory, false); err == nil {
		t.Fatal("native identity alone permitted deleting changed leaf content")
	}
	if data, err := os.ReadFile(filepath.Join(root, "owned.txt")); err != nil || string(data) != "changed by outside actor" {
		t.Fatalf("foreign change not preserved: %q %v", data, err)
	}
}
