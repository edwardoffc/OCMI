package internal

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// Unlike the generic update snapshot helper, do not read a reparse directory
// as a regular file or follow its foreign target.
func unpackTestSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if e := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 || checkpointGitReparse(p) {
			target, e := os.Readlink(p)
			if e != nil {
				return e
			}
			out[rel] = "link:" + target
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		id, e := localNativeID(p)
		if e != nil {
			return e
		}
		if d.IsDir() {
			out[rel] = "directory:" + id
			return nil
		}
		raw, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		out[rel] = fmt.Sprintf("%s:%x", id, sha256.Sum256(raw))
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	return out
}

func TestWorkerUnpackNativePublication(t *testing.T) {
	for _, scenario := range []string{"absent", "file", "empty", "nonempty", "link", "wrong_source", "wrong_parent", "closed_parent", "replaced_source"} {
		t.Run(scenario, func(t *testing.T) {
			base := t.TempDir()
			source := filepath.Join(base, "source")
			destination := filepath.Join(base, "destination")
			if e := os.Mkdir(source, 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(source, "payload"), []byte("fictional native payload"), 0600); e != nil {
				t.Fatal(e)
			}
			if scenario == "file" {
				if e := os.WriteFile(destination, []byte("foreign canary"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			if scenario == "empty" || scenario == "nonempty" {
				if e := os.Mkdir(destination, 0700); e != nil {
					t.Fatal(e)
				}
			}
			if scenario == "nonempty" {
				if e := os.WriteFile(filepath.Join(destination, "canary"), []byte("foreign"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			var outside string
			if scenario == "link" {
				outside = t.TempDir()
				if e := os.WriteFile(filepath.Join(outside, "canary"), []byte("foreign"), 0600); e != nil {
					t.Fatal(e)
				}
				if runtime.GOOS == "windows" {
					if out, e := exec.Command("cmd.exe", "/c", "mklink", "/J", destination, outside).CombinedOutput(); e != nil {
						t.Fatalf("junction premise %v %s", e, out)
					}
				} else {
					if e := os.Symlink(outside, destination); e != nil {
						t.Fatal(e)
					}
				}
			}
			parent, e := os.OpenRoot(base)
			if e != nil {
				t.Fatal(e)
			}
			defer parent.Close()
			parentID, e := unpackRootID(parent)
			if e != nil {
				t.Fatal(e)
			}
			sourceID, e := localNativeID(source)
			if e != nil {
				t.Fatal(e)
			}
			if scenario == "wrong_source" {
				sourceID += "-wrong"
			}
			if scenario == "wrong_parent" {
				parentID += "-wrong"
			}
			if scenario == "replaced_source" {
				if e := os.Rename(source, filepath.Join(base, "original")); e != nil {
					t.Fatal(e)
				}
				if e := os.Mkdir(source, 0700); e != nil {
					t.Fatal(e)
				}
			}
			before := unpackTestSnapshot(t, base)
			var foreign map[string]string
			if outside != "" {
				foreign = unpackTestSnapshot(t, outside)
			}
			if scenario == "closed_parent" {
				parent.Close()
			}
			e = unpackPublish(parent, "source", "destination", parentID, sourceID)
			if scenario == "absent" {
				if e != nil {
					t.Fatal(e)
				}
				now, e := localNativeID(destination)
				if e != nil || now != sourceID {
					t.Fatal("published source identity changed", e)
				}
				if raw, e := os.ReadFile(filepath.Join(destination, "payload")); e != nil || string(raw) != "fictional native payload" {
					t.Fatal("payload changed", e)
				}
				if _, e := os.Lstat(source); !os.IsNotExist(e) {
					t.Fatal("source not moved", e)
				}
			} else {
				if e == nil {
					t.Fatal("publication should refuse")
				}
				if !reflect.DeepEqual(before, unpackTestSnapshot(t, base)) {
					t.Fatal("refusal changed source or foreign destination")
				}
				if outside != "" && !reflect.DeepEqual(foreign, unpackTestSnapshot(t, outside)) {
					t.Fatal("link target changed")
				}
			}
		})
	}
}

func TestWorkerUnpackPublicationNames(t *testing.T) {
	for _, leaf := range []string{"", ".", "..", "a/b", `a\b`, "a:", "NUL", "con.txt", "COM1", "LPT9.log", "home.", "home ", "a\n", "\xff"} {
		if unpackPublicationNames("stage/home", leaf) {
			t.Fatal("unsafe destination accepted", leaf)
		}
	}
	if !unpackPublicationNames(filepath.Join("stage", "home"), "selected home Ω") {
		t.Fatal("ordinary selected leaf refused")
	}
}
