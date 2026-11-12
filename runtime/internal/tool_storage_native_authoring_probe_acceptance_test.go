package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainToolAuthoringProbeStopProof(t *testing.T) {
	f := mainAuthorSetup(t)
	goExe, _, _ := mainAuthorNativeTools(t)
	options := mainAuthorOptions(f, "go")
	options["executable"] = goExe
	plan := mainAuthorPrepare(t, f, options)
	called := false
	registered := false
	previous := environmentRunOverride
	defer func() { environmentRunOverride = previous }()
	// Reuse the existing native-probe I/O seam. The real Go/npm route is proved
	// separately; this injects the otherwise unsafe "exit0 but not stopped" result.
	environmentRunOverride = func(_ context.Context, spec commandSpec) (commandResult, error) {
		called = true
		registered = spec.Registered != nil
		values := map[string]string{}
		for _, entry := range spec.Env {
			key, value, ok := strings.Cut(entry, "=")
			if ok && (key == "GOCACHE" || key == "GOMODCACHE" || key == "GOPATH" || key == "GOBIN" || key == "GOTMPDIR") {
				values[key] = value
			}
		}
		raw, _ := json.Marshal(values)
		return commandResult{ExitCode: 0, Output: string(raw), StoppedVerified: false}, nil
	}
	result := mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, plan, "go"))
	if !called {
		t.Fatal("authoring bypassed existing native-probe qualification seam")
	}
	if !registered {
		t.Error("probe lacks durable registered-child callback")
	}
	if result.OK {
		t.Fatal("exit0 and matching paths replaced stopped proof", result)
	}
	if _, e := os.Lstat(filepath.Join(f.home, "local", "bindings", "tool-storage", "go.json")); !os.IsNotExist(e) {
		t.Fatal("unknown stopped state published a usable profile", e)
	}
	// No child was actually launched by this fault seam. A real uncertain child
	// would retain scratch until its recorded native context proves closure.
}
func TestMainToolAuthoringNativeAuthorityBeforeBinding(t *testing.T) {
	for _, field := range []string{"installationId", "deviceId", "copyId"} {
		t.Run(field, func(t *testing.T) {
			f := mainAuthorSetup(t)
			plan := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
			mainAuthorApply(t, f, plan, "widget")
			file := filepath.Join(f.home, "local", "bindings", "tool-storage", "receipts", "widget.json")
			raw, e := os.ReadFile(file)
			if e != nil {
				t.Fatal(e)
			}
			var receipt map[string]any
			if json.Unmarshal(raw, &receipt) != nil {
				t.Fatal("receipt fixture")
			}
			matched := false
			for key := range receipt {
				if strings.EqualFold(key, field) {
					receipt[key] = "11111111-1111-4111-8111-111111111111"
					matched = true
				}
			}
			if !matched {
				t.Fatal("native authority ID missing", field)
			}
			mainToolWrite(t, file, receipt)
			project, r := mainAuthorProject(t, f, "foreign-authority")
			if r.OK {
				r = request(project, "task.ready", "main", r.Revision, map[string]any{})
			}
			if r.OK {
				t.Fatal("fresh task bound to foreign installation authority", field, r)
			}
		})
	}
}

// Main-owned integration control: the actual packaged CLI authors both supported
// providers, not merely an in-process helper or a custom-variable fixture.
func TestMainToolAuthoringNativeCLIProbes(t *testing.T) {
	binary := os.Getenv("OCMI_ROOT_BINARY")
	if binary == "" {
		t.Skip("integration runner supplies actual native CLI")
	}
	goExe, node, script := mainAuthorNativeTools(t)
	f := mainAuthorSetup(t)
	call := func(action string, payload any) Response {
		raw, e := json.Marshal(map[string]any{"schemaVersion": 1, "action": "tool-storage." + action, "projectRoot": f.home, "rootKind": "installation", "actor": "coordinator", "requestId": "packaged-supported-" + action, "expectedRevision": 0, "payload": payload})
		if e != nil {
			t.Fatal(e)
		}
		child := exec.Command(binary, "request")
		child.Stdin = bytes.NewReader(raw)
		out, e := child.CombinedOutput()
		var response Response
		if json.Unmarshal(out, &response) != nil {
			t.Fatal("packaged provider response invalid", e)
		}
		if e != nil || !response.OK {
			if action == "apply" {
				mainAuthorLogProbeState(t, f, "packaged")
			}
			t.Fatal("packaged provider operation", action, e, response)
		}
		return response
	}
	for _, tool := range []string{"go", "npm"} {
		options := mainAuthorOptions(f, tool)
		if tool == "go" {
			options["executable"] = goExe
		} else {
			options["executable"] = node
			options["script"] = script
		}
		plan := call("prepare", options)
		active := call("apply", mainAuthorApproval(t, plan, tool))
		if active.Data["verification"] != "effective-paths-verified" {
			t.Fatal("packaged provider falsely qualified", tool, active)
		}
		inspected := call("inspect", map[string]any{"toolId": tool})
		if inspected.Data["phase"] != "active" || inspected.Data["verification"] != "effective-paths-verified" {
			t.Fatal("packaged proof did not survive fresh CLI", tool, inspected)
		}
	}
}

// Only known fictional fixture state is summarized. Never log provider output,
// config contents, consent references, environment values or ambient files.
func mainAuthorLogProbeState(t *testing.T, f mainAuthorFixture, tool string) {
	t.Helper()
	c, code := resolveLayout(f.home, "installation")
	if code != "" {
		t.Logf("native probe state %s layout=%s", tool, code)
		return
	}
	tx, err := toolStorageReadTransaction(c)
	if err != nil {
		t.Logf("native probe state %s transaction unavailable: %v", tool, err)
		return
	}
	state, _, _ := strings.Cut(tx.ProbeState, ":")
	t.Logf("native probe state %s phase=%s tool=%s probe=%s stageRecorded=%t", tool, tx.Phase, tx.ToolID, state, tx.Stage != "")
	if tx.Stage != "" && tx.StageID != "" {
		locator, relErr := filepath.Rel(c.Root, tx.Stage)
		if relErr == nil && !filepath.IsAbs(locator) && !strings.HasPrefix(locator, "..") {
			proof, captureErr := captureLocalResource(c.Root, filepath.ToSlash(locator), "directory", tx.StageID, true)
			t.Logf("native probe stage capture=%v inventoryBytes=%d", captureErr, len(proof.Inventory))
		}
	}
}
