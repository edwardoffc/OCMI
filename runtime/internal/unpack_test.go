package internal

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"testing"
)

func writerUnpackFixture(t *testing.T, base string) (string, string) {
	t.Helper()
	factory := filepath.Join(base, "factory")
	acceptanceUpdateFactory(t, factory, strings.Repeat("a", 40))
	archive := filepath.Join(base, "kit.zip")
	out, e := os.Create(archive)
	if e != nil {
		t.Fatal(e)
	}
	zw := zip.NewWriter(out)
	names := []string{}
	if e = filepath.WalkDir(factory, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			rel, e := filepath.Rel(factory, p)
			if e != nil {
				return e
			}
			names = append(names, filepath.ToSlash(rel))
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	sort.Strings(names)
	for _, name := range names {
		raw, e := os.ReadFile(filepath.Join(factory, filepath.FromSlash(name)))
		if e != nil {
			t.Fatal(e)
		}
		var compressed bytes.Buffer
		fw, e := flate.NewWriter(&compressed, flate.BestSpeed)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = fw.Write(raw); e != nil {
			t.Fatal(e)
		}
		if e = fw.Close(); e != nil {
			t.Fatal(e)
		}
		w, e := zw.CreateRaw(&zip.FileHeader{Name: name, CreatorVersion: 20, ReaderVersion: 20, Method: 8, CRC32: crc32.ChecksumIEEE(raw), CompressedSize64: uint64(compressed.Len()), UncompressedSize64: uint64(len(raw))})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write(compressed.Bytes()); e != nil {
			t.Fatal(e)
		}
	}
	if e = zw.Close(); e != nil {
		t.Fatal(e)
	}
	if e = out.Close(); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(archive)
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(raw)
	return archive, hex.EncodeToString(sum[:])
}

func TestWorkerUnpackLifecycle(t *testing.T) {
	base := t.TempDir()
	archive, sha := writerUnpackFixture(t, base)
	for _, scenario := range []string{"ready", "bad_hash", "occupied", "stage_fault", "arrival", "stage_changed", "stage_replaced", "parent_changed", "published_fault"} {
		t.Run(scenario, func(t *testing.T) {
			caseRoot := t.TempDir()
			parent := filepath.Join(caseRoot, "selected")
			if e := os.Mkdir(parent, 0700); e != nil {
				t.Fatal(e)
			}
			destination := filepath.Join(parent, "home")
			digest := sha
			if scenario == "bad_hash" {
				digest = strings.Repeat("0", 64)
			}
			if scenario == "occupied" {
				if e := os.Mkdir(destination, 0700); e != nil {
					t.Fatal(e)
				}
			}
			before := workerUpdateTreeSnapshot(t, parent)
			blockedRename := false
			renameFailure := func(e error) error {
				var code syscall.Errno
				if runtime.GOOS != "windows" || !errors.As(e, &code) || (code != syscall.Errno(5) && code != syscall.Errno(32)) {
					t.Fatalf("replacement fixture could not rename owned root: %v", e)
				}
				blockedRename = true
				t.Logf("native held-directory protection refused replacement, Windows error=%d", code)
				return e
			}
			unpackTestPhase = func(phase, stage string) error {
				if scenario == "stage_fault" && phase == "stage_created" {
					return errors.New("synthetic interruption")
				}
				if scenario == "arrival" && phase == "before_publish" {
					return os.WriteFile(destination, []byte("foreign arrival"), 0600)
				}
				if scenario == "stage_changed" && phase == "before_publish" {
					return os.WriteFile(filepath.Join(stage, "home", "README.md"), []byte("changed"), 0600)
				}
				if scenario == "stage_replaced" && phase == "before_publish" {
					if e := os.Rename(stage, stage+"-original"); e != nil {
						return renameFailure(e)
					}
					if e := os.Mkdir(stage, 0700); e != nil {
						return e
					}
					return os.WriteFile(filepath.Join(stage, "foreign-canary"), []byte("preserve stage replacement"), 0600)
				}
				if scenario == "parent_changed" && phase == "before_publish" {
					if e := os.Rename(parent, filepath.Join(caseRoot, "original-parent")); e != nil {
						return renameFailure(e)
					}
					if e := os.Mkdir(parent, 0700); e != nil {
						return e
					}
					return os.WriteFile(filepath.Join(parent, "foreign-canary"), []byte("preserve parent replacement"), 0600)
				}
				if scenario == "published_fault" && phase == "published" {
					return errors.New("lost reply")
				}
				return nil
			}
			t.Cleanup(func() { unpackTestPhase = nil })
			r := UnpackHome(archive, destination, digest, false)
			unpackTestPhase = nil
			if scenario == "ready" {
				if !r.OK || r.Code != "UNPACKED" || r.Data["phase"] != "ready" || r.Data["cleanup"] != "done" {
					t.Fatal(r)
				}
				before := workerUpdateTreeSnapshot(t, parent)
				inspect := UnpackHome("", destination, "", true)
				if !inspect.OK || inspect.Code != "UNPACK_INSPECTED" || inspect.Data["sourceCommit"] != strings.Repeat("a", 40) {
					t.Fatal(inspect)
				}
				if !reflect.DeepEqual(before, workerUpdateTreeSnapshot(t, parent)) {
					t.Fatal("inspect wrote state")
				}
				if again := UnpackHome(archive, destination, sha, false); again.OK {
					t.Fatal("existence silently adopted")
				}
			} else {
				if r.OK {
					t.Fatal("fault became success", r)
				}
				switch scenario {
				case "bad_hash", "occupied", "stage_fault":
					if r.Data["cleanup"] != "done" || !reflect.DeepEqual(before, workerUpdateTreeSnapshot(t, parent)) {
						t.Fatal("normal refusal left effects", r)
					}
				case "arrival":
					raw, e := os.ReadFile(destination)
					if e != nil || string(raw) != "foreign arrival" || r.Data["cleanup"] != "done" {
						t.Fatal("arrival overwritten", r, e)
					}
				case "stage_changed":
					if r.Data["cleanup"] != "pending" {
						t.Fatal("changed stage was deleted", r)
					}
					if _, e := os.Lstat(destination); !os.IsNotExist(e) {
						t.Fatal("changed payload published")
					}
				case "stage_replaced", "parent_changed":
					if blockedRename {
						if r.Data["cleanup"] != "done" || !reflect.DeepEqual(before, workerUpdateTreeSnapshot(t, parent)) {
							t.Fatal("native-blocked replacement changed owned tree", r)
						}
						break
					}
					if r.Data["cleanup"] != "pending" {
						t.Fatal("replacement was cleaned", r)
					}
					canary := filepath.Join(parent, "foreign-canary")
					if scenario == "stage_replaced" {
						canary = filepath.Join(r.Data["ownedStage"].(string), "foreign-canary")
					}
					if _, e := os.ReadFile(canary); e != nil {
						t.Fatal("foreign replacement lost", e)
					}
					if _, e := os.Lstat(destination); !os.IsNotExist(e) {
						t.Fatal("replacement payload published")
					}
				case "published_fault":
					if r.Data["cleanup"] != "pending" || !UnpackHome("", destination, "", true).OK {
						t.Fatal("published lost reply not inspectable", r)
					}
				}
			}
		})
	}
}

func TestWorkerUnpackInterruptedChild(t *testing.T) {
	if os.Getenv("OCMI_UNPACK_TEST_CHILD") != "1" {
		t.Skip("owned Go child only")
	}
	unpackTestPhase = func(phase, stage string) error {
		if phase == os.Getenv("OCMI_UNPACK_TEST_PHASE") {
			os.Exit(92)
		}
		return nil
	}
	r := UnpackHome(os.Getenv("OCMI_UNPACK_TEST_ARCHIVE"), os.Getenv("OCMI_UNPACK_TEST_DESTINATION"), os.Getenv("OCMI_UNPACK_TEST_SHA"), false)
	t.Fatal("fault not reached", r)
}

func TestWorkerUnpackActualInterruption(t *testing.T) {
	base := t.TempDir()
	archive, sha := writerUnpackFixture(t, base)
	for _, phase := range []string{"stage_created", "published"} {
		t.Run(phase, func(t *testing.T) {
			parent := t.TempDir()
			destination := filepath.Join(parent, "home")
			cmd := exec.Command(os.Args[0], "-test.run=^TestWorkerUnpackInterruptedChild$")
			cmd.Env = append(os.Environ(), "OCMI_UNPACK_TEST_CHILD=1", "OCMI_UNPACK_TEST_PHASE="+phase, "OCMI_UNPACK_TEST_ARCHIVE="+archive, "OCMI_UNPACK_TEST_DESTINATION="+destination, "OCMI_UNPACK_TEST_SHA="+sha)
			out, e := cmd.CombinedOutput()
			if e == nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 92 {
				t.Fatalf("actual exit92 missing: %v %s", e, out)
			}
			entries, e := os.ReadDir(parent)
			if e != nil {
				t.Fatal(e)
			}
			stage := ""
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".ocmi-unpack-") {
					stage = entry.Name()
				}
			}
			if stage == "" {
				t.Fatal("attributable stage lost")
			}
			raw, e := os.ReadFile(filepath.Join(parent, stage, "unpack-owner.json"))
			if e != nil {
				t.Fatal(e)
			}
			var record unpackOwnership
			if json.Unmarshal(raw, &record) != nil || record.SchemaVersion != 1 || record.Destination != "home" || record.ArchiveSHA256 != sha || record.StageID == "" || record.PayloadID == "" {
				t.Fatal("invalid ownership record")
			}
			if phase == "stage_created" {
				if _, e = os.Lstat(destination); !os.IsNotExist(e) {
					t.Fatal("unpublished interruption created target")
				}
			} else {
				before := workerUpdateTreeSnapshot(t, parent)
				if r := UnpackHome("", destination, "", true); !r.OK {
					t.Fatal(r)
				}
				if !reflect.DeepEqual(before, workerUpdateTreeSnapshot(t, parent)) {
					t.Fatal("inspection changed interrupted evidence")
				}
			}
		})
	}
}
