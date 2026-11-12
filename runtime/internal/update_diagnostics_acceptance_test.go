package internal

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Main-owned boundary oracle: exercise the actual admitted recovery path.
// The callback exists only inside Go tests and cannot be set by a CLI request.
func TestMainUpdateDiagnosticAdmittedFailure(t *testing.T) {
	base := t.TempDir()
	home, candidate := filepath.Join(base, "home"), filepath.Join(base, "candidate")
	acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
	entry := acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
	if r := InstallHome(home, false); !r.OK {
		t.Fatal(r)
	}
	private := filepath.Join(home, "OCMI_Sync", "private-canary")
	const canary = "PRIVATE-UPDATE-DIAGNOSTIC-CANARY"
	if err := os.WriteFile(private, []byte(canary), 0600); err != nil {
		t.Fatal(err)
	}
	apply := exec.Command(entry, "-test.run=^TestWorkerUpdateAPIChild$")
	apply.Env = append(os.Environ(), "WORKER_UPDATE_HOME="+home, "WORKER_UPDATE_CANDIDATE="+candidate,
		"WORKER_UPDATE_ACTION=apply", "WORKER_UPDATE_PHASE=applied", "WORKER_UPDATE_CRASH=")
	if out, err := apply.CombinedOutput(); err != nil {
		t.Fatalf("actual candidate-entry apply premise: %v %s", err, out)
	}
	called := false
	updateTestPhase = func(phase string) {
		if phase == "recovery_admitted" {
			called = true
			if err := os.WriteFile(filepath.Join(home, "README.md"), []byte("Fictional changed public content.\n"), 0700); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Cleanup(func() { updateTestPhase = nil })
	r := UpdateHome("rollback", home, "", "", "")
	updateTestPhase = nil
	if !called {
		t.Fatal("deterministic admitted-recovery premise was not reached")
	}
	if r.OK || r.Code != "UPDATE_REFUSED" || r.Data["reason"] != "provenance" || r.Data["stage"] != "recovery" {
		t.Fatal("admitted proof failure did not refuse", r)
	}
	raw, err := json.Marshal(r.Data["diagnostic"])
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if json.Unmarshal(raw, &d) != nil || d["boundary"] != "recover_public" || d["validator"] != "recovery_start.root_placement" || d["kind"] != "mismatch" {
		t.Fatalf("lost actual recovery boundary: %s", raw)
	}
	if predicate, ok := d["predicate"].(string); !ok || !strings.Contains(predicate, "content") {
		t.Fatalf("content mismatch predicate lost: %s", raw)
	}
	if len(raw) > 4096 || strings.Contains(string(raw), canary) || strings.Contains(string(raw), "README") || strings.Contains(string(raw), "private-canary") {
		t.Fatalf("unsafe diagnostic: %s", raw)
	}
	if bytes, err := os.ReadFile(private); err != nil || string(bytes) != canary {
		t.Fatal("private original changed", err)
	}
	t.Log("UPDATE_DIAGNOSTIC_ADMITTED_OK actual post-admission refusal, original content predicate, private data preserved")
}
