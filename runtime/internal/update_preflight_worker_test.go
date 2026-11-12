package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestWorkerUpdateCandidateChild(t *testing.T) {
	home := os.Getenv("WORKER_PREFLIGHT_HOME")
	if home == "" {
		t.Skip("child only")
	}
	candidate := os.Getenv("WORKER_PREFLIGHT_CANDIDATE")
	_, _, _, _, err := preflightUpdateCandidate(home, candidate, os.Getenv("WORKER_PREFLIGHT_OLD"), os.Getenv("WORKER_PREFLIGHT_NEW"))
	if (err == nil) != (os.Getenv("WORKER_PREFLIGHT_OK") == "1") {
		t.Fatalf("unexpected preflight: %v", err)
	}
}

func workerSetRelease(t *testing.T, root, version string) {
	t.Helper()
	path := filepath.Join(root, "system", "build-info.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err = json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	value["systemVersion"] = version
	raw, err = json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0700); err != nil {
		t.Fatal(err)
	}
	versionRaw := []byte("# OCMI " + version + "\n")
	if err = os.WriteFile(filepath.Join(root, "system", "VERSION.md"), versionRaw, 0700); err != nil {
		t.Fatal(err)
	}
	sumPath := filepath.Join(root, "SHA256SUMS")
	sums, err := os.ReadFile(sumPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(sums)), "\n")
	for i, line := range lines {
		if strings.HasSuffix(line, "  system/build-info.json") {
			lines[i] = fmt.Sprintf("%s  system/build-info.json", updateDigest(raw))
		}
		if strings.HasSuffix(line, "  system/VERSION.md") {
			lines[i] = fmt.Sprintf("%s  system/VERSION.md", updateDigest(versionRaw))
		}
	}
	if err = os.WriteFile(sumPath, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerUpdateCandidatePreflight(t *testing.T) {
	for _, scenario := range []string{"valid", "wrong-caller", "initialized", "uninitialized-home", "nested", "source-mismatch", "downgrade"} {
		t.Run(scenario, func(t *testing.T) {
			base := t.TempDir()
			home := filepath.Join(base, "home")
			candidate := filepath.Join(base, "candidate")
			acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
			entry := acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
			if scenario != "uninitialized-home" {
				if r := InstallHome(home, false); !r.OK {
					t.Fatal(r)
				}
			}
			old := strings.Repeat("a", 40)
			if scenario == "source-mismatch" {
				old = strings.Repeat("c", 40)
			}
			if scenario == "initialized" {
				if err := os.Mkdir(filepath.Join(candidate, "local"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "nested" {
				candidate = filepath.Join(home, "local", "nested")
			}
			if scenario == "downgrade" {
				workerSetRelease(t, candidate, "1.3.4")
			}
			if scenario == "wrong-caller" {
				entry = filepath.Join(home, "system", "bin", "ocmi")
				if runtime.GOOS == "windows" {
					entry += ".exe"
				}
			}
			cmd := exec.Command(entry, "-test.run=^TestWorkerUpdateCandidateChild$")
			cmd.Env = append(os.Environ(), "WORKER_PREFLIGHT_HOME="+home, "WORKER_PREFLIGHT_CANDIDATE="+candidate, "WORKER_PREFLIGHT_OLD="+old, "WORKER_PREFLIGHT_NEW="+strings.Repeat("b", 40), "WORKER_PREFLIGHT_OK="+map[bool]string{true: "1", false: "0"}[scenario == "valid"])
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("child: %v %s", err, out)
			}
		})
	}
}

func TestWorkerUpdateImageChild(t *testing.T) {
	gate := os.Getenv("WORKER_IMAGE_GATE")
	if gate == "" {
		t.Skip("child only")
	}
	if err := os.WriteFile(filepath.Join(gate, "ready"), []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(gate, "release")); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("child deadline")
}

func TestWorkerUpdateImageProbe(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows live-image predicate")
	}
	base := t.TempDir()
	root := filepath.Join(base, "home")
	entry := acceptanceUpdateFactory(t, root, strings.Repeat("a", 40))
	candidate := filepath.Join(base, "candidate")
	candidateEntry := acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
	if r := InstallHome(root, false); !r.OK {
		t.Fatal(r)
	}
	publicBefore := acceptancePublicSnapshot(t, root)
	before, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(entry, "-test.run=^TestWorkerUpdateImageChild$")
	cmd.Env = append(os.Environ(), "WORKER_IMAGE_GATE="+base)
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	finished := false
	defer func() {
		if !finished {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err = os.Stat(filepath.Join(base, "ready")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child not ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err = probeInstalledImage(entry); err == nil {
		t.Fatal("live image accepted")
	}
	check := exec.Command(candidateEntry, "-test.run=^TestWorkerUpdateCandidateChild$")
	check.Env = append(os.Environ(), "WORKER_PREFLIGHT_HOME="+root, "WORKER_PREFLIGHT_CANDIDATE="+candidate, "WORKER_PREFLIGHT_OLD="+strings.Repeat("a", 40), "WORKER_PREFLIGHT_NEW="+strings.Repeat("b", 40), "WORKER_PREFLIGHT_OK=0")
	if output, err := check.CombinedOutput(); err != nil {
		t.Fatalf("live-image preflight child: %v %s", err, output)
	}
	if !reflect.DeepEqual(publicBefore, acceptancePublicSnapshot(t, root)) {
		t.Fatal("preflight changed public payload")
	}
	after, err := os.ReadFile(entry)
	if err != nil || string(before) != string(after) {
		t.Fatal("image changed")
	}
	if err = os.WriteFile(filepath.Join(base, "release"), []byte("done"), 0600); err != nil {
		t.Fatal(err)
	}
	err = cmd.Wait()
	finished = true
	if err != nil {
		t.Fatal(err)
	}
}
