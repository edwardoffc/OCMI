package internal

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// Independent acceptance helper: only the test executable reads these variables.
func TestResourceCrashChild(t *testing.T) {
	if os.Getenv("OCMI_RESOURCE_CRASH_CHILD") != "1" {
		t.Skip("spawned only by the isolated crash acceptance control")
	}
	root := os.Getenv("OCMI_RESOURCE_CRASH_ROOT")
	marker, err := os.ReadFile(filepath.Join(root, "resource-crash-fixture.marker"))
	if err != nil || string(marker) != "OCMI fictional crash fixture" {
		t.Fatal("not the owned fictional fixture")
	}
	rev, err := strconv.ParseInt(os.Getenv("OCMI_RESOURCE_CRASH_REV"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	resourceEffectFault = func(stage string) error {
		if stage == "after-effect-before-record" {
			os.Exit(91)
		}
		return nil
	}
	response := Handle(Request{SchemaVersion: 1, Action: "resource.apply", ProjectRoot: root,
		Actor: "main", RequestID: "crash-apply-once", ExpectedRevision: rev,
		Payload: json.RawMessage(`{"resourceId":"crash-output"}`)})
	t.Fatalf("fault seam did not terminate the actual effect path: %+v", response)
}

func TestResourceCrashPreservesIntentAndRefusesReplayEffects(t *testing.T) {
	root, rev := setup(t, "0", 5000, 4096)
	if err := os.WriteFile(filepath.Join(root, "resource-crash-fixture.marker"), []byte("OCMI fictional crash fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	intent := request(root, "resource.intent", "main", rev, map[string]any{
		"resourceId": "crash-output", "provider": "local", "kind": "directory", "mode": "create",
		"locator": "crash-output", "purpose": "Fictional crash acceptance", "lifetime": "test",
		"disposition": "remove", "ownerRef": "main",
	})
	if !intent.OK {
		t.Fatal(intent)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestResourceCrashChild$", "-test.timeout=10s")
	cmd.Env = append(os.Environ(), "OCMI_RESOURCE_CRASH_CHILD=1", "OCMI_RESOURCE_CRASH_ROOT="+root,
		"OCMI_RESOURCE_CRASH_REV="+strconv.FormatInt(intent.Revision, 10))
	output, err := cmd.CombinedOutput()
	if err == nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 91 {
		t.Fatalf("expected actual controller exit91, got %v: %s", err, output)
	}
	path := filepath.Join(root, "crash-output")
	before, err := os.Stat(path)
	if err != nil || !before.IsDir() {
		t.Fatalf("effect did not occur: %v", err)
	}
	state := request(root, "resource.inspect", "main", 0, map[string]any{"resourceId": "crash-output"})
	if !state.OK || state.Data["pending"] != true {
		t.Fatalf("crashed intent falsely completed: %+v", state)
	}
	replay := Handle(Request{SchemaVersion: 1, Action: "resource.apply", ProjectRoot: root,
		Actor: "main", RequestID: "crash-apply-once", ExpectedRevision: intent.Revision,
		Payload: json.RawMessage(`{"resourceId":"crash-output"}`)})
	_ = replay // A pending response or refusal is valid; another physical effect is not.
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("replay replaced the existing object: %v", err)
	}
	state = request(root, "resource.inspect", "main", 0, map[string]any{"resourceId": "crash-output"})
	if !state.OK || state.Data["pending"] != true {
		t.Fatalf("replay laundered ambiguous creation into success: %+v", state)
	}
	for _, action := range []string{"resource.cancel", "resource.dispose"} {
		payload := map[string]any{"resourceId": "crash-output", "reason": "A reason is not ownership proof", "disposition": "remove"}
		if r := request(root, action, "main", state.Revision, payload); r.OK {
			t.Fatalf("ambiguous creation accepted %s: %+v", action, r)
		}
	}
	if r := request(root, "run", "worker", state.Revision, map[string]any{}); r.OK {
		t.Fatalf("unresolved create allowed a conflicting task run: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(root, "effect.txt")); !os.IsNotExist(err) {
		t.Fatal("blocked command produced an effect")
	}
	t.Log("actual controller exit91 retained intent; restart/replay/false completion refused")
}
