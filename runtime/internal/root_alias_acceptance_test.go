package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// Independent selected-root regression: aliases must identify the same ordinary
// selected file without weakening the existing link/traversal rejection.
func TestSelectedRootRepresentations(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project spaces Ω")
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "nested", "input.txt")
	if err := os.WriteFile(want, []byte("fictional input"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, alias := range map[string]string{
		"native":           root,
		"trailing":         root + string(os.PathSeparator),
		"forward":          filepath.ToSlash(root),
		"forward-trailing": filepath.ToSlash(root) + "/",
		"dot":              root + string(os.PathSeparator) + ".",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := portableRootFile(alias, "nested/input.txt")
			if err != nil {
				t.Fatalf("equivalent selected root refused: %v", err)
			}
			if filepath.Clean(got) != want {
				t.Fatalf("different selected file: %q", got)
			}
		})
	}
	for _, name := range []string{"../escape.txt", "nested/../../escape.txt", "/absolute.txt"} {
		if _, err := portableRootFile(root, name); err == nil {
			t.Fatalf("unsafe name accepted: %s", name)
		}
	}
	// Parent traversal is not an alias guarantee: cleaning it can hide a link
	// component and change which root the caller selected.
	if _, err := portableRootFile(root+string(os.PathSeparator)+"nested"+string(os.PathSeparator)+"..", "nested/input.txt"); err == nil {
		t.Fatal("root parent traversal silently normalized")
	}
}

func TestSelectedRootStillRejectsLinks(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "ordinary")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "input.txt"), []byte("fictional"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if _, err := portableRootFile(link, "input.txt"); err == nil {
		t.Fatal("linked root accepted")
	}
	if err := os.Symlink(filepath.Join(root, "input.txt"), filepath.Join(root, "linked.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := portableRootFile(root, "linked.txt"); err == nil {
		t.Fatal("linked selected file accepted")
	}
}
