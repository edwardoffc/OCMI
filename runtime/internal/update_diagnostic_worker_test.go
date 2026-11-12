package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"testing"
)

func workerDiagnosticData(t *testing.T, r Response) map[string]any {
	t.Helper()
	if r.OK || r.Code != "UPDATE_REFUSED" || r.Data["reason"] != "provenance" || r.Data["stage"] != "recovery" {
		t.Fatal(r)
	}
	d, ok := r.Data["diagnostic"].(map[string]any)
	if !ok {
		t.Fatal("missing diagnostic", r)
	}
	raw, err := json.Marshal(d)
	if err != nil || len(raw) > 4096 {
		t.Fatal("unbounded diagnostic", err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	var check func(any)
	safe := regexp.MustCompile(`^[A-Za-z0-9_.-]{1,80}$`)
	check = func(v any) {
		switch v := v.(type) {
		case string:
			if !safe.MatchString(v) {
				t.Fatalf("non-enum string: %q", v)
			}
		case map[string]any:
			for k, value := range v {
				check(k)
				check(value)
			}
		case []any:
			for _, value := range v {
				check(value)
			}
		case float64:
		default:
			t.Fatalf("unexpected diagnostic value %T", v)
		}
	}
	check(decoded)
	return d
}

func TestWorkerUpdateDiagnosticObservations(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "secret-member")
	if err := os.WriteFile(path, []byte("private bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	id, err := localNativeID(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entry := updatePublicEntry{Path: "secret-member", Size: 13, Digest: updateDigest([]byte("private bytes")), Mode: uint32(info.Mode().Perm()), NativeID: id}
	// Capture a real original read error; subsequently restoring the member
	// must not replace that captured result with a fresh successful probe.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	observation := observeUpdateEntry(base, entry)
	var originalCode syscall.Errno
	if !errors.As(observation, &originalCode) {
		t.Fatal("original OS error lost", observation)
	}
	if err := os.WriteFile(path, []byte("private bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	j := updateProofJournal{Phase: "applied", RecoveryStep: 7}
	d := workerDiagnosticData(t, updateProvenanceFailure("before_admission", j, updateValidator(observation, "recovery_start.root_placement")))
	if d["kind"] != "io_error" || d["operation"] != "read" || d["osCode"] != uint64(originalCode) || d["recoveryStep"] != 7 {
		t.Fatal(d)
	}
	for _, unknown := range []error{errors.New("PRIVATE/path/identity/content"), updateObserved("unrecognized PRIVATE/path", errors.New("PRIVATE bytes"), "PRIVATE operation")} {
		d := workerDiagnosticData(t, updateProvenanceFailure("unsafe/boundary", updateProofJournal{Phase: "PRIVATE/path"}, unknown))
		if d["kind"] != "unknown" || d["predicate"] != "unknown" || d["phase"] != "unknown" || d["boundary"] != "unknown" {
			t.Fatal(d)
		}
	}
	aggregate := updatePlacementFailure("root placement", nil, observation, updateMismatch("root member identity"))
	d = workerDiagnosticData(t, updateProvenanceFailure("before_admission", j, updateValidator(aggregate, "recovery_state")))
	if d["predicate"] != "root_placement" || d["kind"] != "mismatch" || len(d["observations"].([]any)) != 3 {
		t.Fatal(d)
	}
}

func TestWorkerUpdateDiagnosticCapturedPathError(t *testing.T) {
	base := t.TempDir()
	// Capture the actual helper failure once. Creating the missing component
	// afterwards must not turn the retained path-check error into a mismatch
	// or cause a diagnostic-time filesystem probe.
	pathErr := plainComponents(base, "private-missing-component", false)
	var code syscall.Errno
	if !errors.As(pathErr, &code) || code == 0 {
		t.Fatal("path-error premise", pathErr)
	}
	observation := updateObserved("system link", pathErr, "path_check")
	if err := os.Mkdir(filepath.Join(base, "private-missing-component"), 0700); err != nil {
		t.Fatal(err)
	}
	j := updateProofJournal{Phase: "applied"}
	d := workerDiagnosticData(t, updateProvenanceFailure("before_admission", j, updateValidator(observation, "recovery_state")))
	if d["kind"] != "io_error" || d["predicate"] != "system_path" || d["operation"] != "path_check" || d["osCode"] != uint64(code) {
		t.Fatal(d)
	}
	if !errors.Is(observation, pathErr) {
		t.Fatal("original helper error not retained")
	}
	d = workerDiagnosticData(t, updateProvenanceFailure("before_admission", j, updateValidator(updateObserved("system link", errors.New("PRIVATE unclassified path error"), "path_check"), "recovery_state")))
	if d["kind"] != "unknown" {
		t.Fatal("invented mismatch for unclassified path failure", d)
	}
	d = workerDiagnosticData(t, updateProvenanceFailure("before_admission", j, updateValidator(updateMismatch("system link"), "recovery_state")))
	if d["kind"] != "mismatch" {
		t.Fatal("lost direct type/link mismatch", d)
	}
}

func TestWorkerUpdateDiagnosticBoundaries(t *testing.T) {
	for _, admitted := range []bool{false, true} {
		t.Run(map[bool]string{false: "before-admission", true: "after-admission"}[admitted], func(t *testing.T) {
			base := t.TempDir()
			home, candidate := filepath.Join(base, "home"), filepath.Join(base, "candidate")
			acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
			entry := acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
			if r := InstallHome(home, false); !r.OK {
				t.Fatal(r)
			}
			private := filepath.Join(home, "OCMI_Sync", "private-canary")
			canary := []byte("PRIVATE-DIAGNOSTIC-WORKER-CANARY")
			if err := os.WriteFile(private, canary, 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(entry, "-test.run=^TestWorkerUpdateAPIChild$")
			cmd.Env = append(os.Environ(), "WORKER_UPDATE_HOME="+home, "WORKER_UPDATE_CANDIDATE="+candidate, "WORKER_UPDATE_ACTION=apply", "WORKER_UPDATE_PHASE=applied")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("native apply premise: %v %s", err, out)
			}
			mutate := func() {
				if err := os.WriteFile(filepath.Join(home, "README.md"), []byte("Fictional modified public content.\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			called := false
			if admitted {
				updateTestPhase = func(phase string) {
					if phase == "recovery_admitted" {
						called = true
						mutate()
					}
				}
				t.Cleanup(func() { updateTestPhase = nil })
			} else {
				mutate()
			}
			before := workerUpdateTreeSnapshot(t, home)
			r := UpdateHome("rollback", home, "", "", "")
			updateTestPhase = nil
			d := workerDiagnosticData(t, r)
			boundary := "before_admission"
			if admitted {
				boundary = "recover_public"
			}
			if called != admitted || d["boundary"] != boundary || d["validator"] != "recovery_start.root_placement" || d["predicate"] != "root_content" || d["kind"] != "mismatch" {
				t.Fatal(d)
			}
			if !admitted && !reflect.DeepEqual(before, workerUpdateTreeSnapshot(t, home)) {
				t.Fatal("pre-admission refusal wrote fixture")
			}
			if actual, err := os.ReadFile(private); err != nil || !bytes.Equal(actual, canary) {
				t.Fatal("private bytes changed", err)
			}
		})
	}
}
