package internal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// The actual codecs and complete delivered binaries have separate native callers.
// Here matching checksums must not turn wrong semantic metadata into a valid kit.
func TestMainUnpackSemanticManifest(t *testing.T) {
	root := filepath.Join(t.TempDir(), "factory")
	acceptanceUpdateFactory(t, root, strings.Repeat("a", 40))
	base := map[string][]byte{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err == nil {
			base[filepath.ToSlash(name)] = raw
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	binary := "system/bin/ocmi"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	// No execution: a small neutral payload keeps semantic cases independent of
	// the test executable size. The independent ReadyKit consumer checks real bytes.
	base[binary] = []byte("fictional semantic fixture; never executed")
	for _, name := range []string{"valid", "wrong-target", "wrong-source", "missing-agent", "missing-binary", "invalid-layout", "bad-sums", "wrong-mode", "private-path"} {
		t.Run(name, func(t *testing.T) {
			files := map[string][]byte{}
			for key, raw := range base {
				if key != "SHA256SUMS" {
					files[key] = raw
				}
			}
			if name == "wrong-target" || name == "wrong-source" {
				var info map[string]any
				if err := json.Unmarshal(files["system/build-info.json"], &info); err != nil {
					t.Fatal(err)
				}
				if name == "wrong-target" {
					info["target"] = "unsupported-fixture-target"
				} else {
					info["sourceCommit"] = "not-a-commit"
				}
				files["system/build-info.json"], _ = json.Marshal(info)
			}
			switch name {
			case "missing-agent":
				delete(files, "AGENTS.md")
			case "missing-binary":
				delete(files, binary)
			case "invalid-layout":
				files[".ocmi-home.json"] = []byte("{}")
			case "private-path":
				files["local/fictional.txt"] = []byte("not public payload")
			}
			var names []string
			for key := range files {
				names = append(names, key)
			}
			sort.Strings(names)
			var sums strings.Builder
			for _, key := range names {
				digest := sha256.Sum256(files[key])
				sums.WriteString(hex.EncodeToString(digest[:]) + "  " + key + "\n")
			}
			files["SHA256SUMS"] = []byte(sums.String())
			if name == "bad-sums" {
				files["SHA256SUMS"] = []byte("not-a-checksum  AGENTS.md\n")
			}
			names = append(names, "SHA256SUMS")
			collector := unpackCollector{names: map[string]bool{}, metadata: map[string][]byte{}}
			var err error
			for _, key := range names {
				mode := os.FileMode(0644)
				if key == binary || name == "wrong-mode" && key == "README.md" {
					mode = 0755
				}
				err = collector.file(key, int64(len(files[key])), mode, bytes.NewReader(files[key]))
				if err != nil {
					break
				}
			}
			if err == nil {
				_, err = collector.finish()
			}
			if (err == nil) != (name == "valid") {
				t.Fatalf("semantic manifest %s: %v", name, err)
			}
		})
	}
}
