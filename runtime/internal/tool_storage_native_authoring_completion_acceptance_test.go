package internal

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainToolAuthoringCorruptAuthorityNotCancelled(t *testing.T) {
	f := mainAuthorSetup(t)
	plan := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
	mainAuthorApply(t, f, plan, "widget")
	receipt := filepath.Join(f.home, "local", "bindings", "tool-storage", "receipts", "widget.json")
	if e := os.WriteFile(receipt, []byte("{broken but not disposable"), 0600); e != nil {
		t.Fatal(e)
	}
	planPath := filepath.Join(f.home, "local", "bindings", "tool-storage", "plans", "widget.json")
	before, e := os.ReadFile(planPath)
	if e != nil {
		t.Fatal(e)
	}
	result := mainAuthorCall(f, "cancel", "coordinator", map[string]any{"toolId": "widget", "planId": plan.Data["planId"], "planSha256": plan.Data["planSha256"]})
	if result.OK {
		t.Error("unreadable authority interpreted as absent/prepared")
	}
	after, e := os.ReadFile(planPath)
	if e != nil || string(after) != string(before) {
		t.Fatal("corrupt receipt caused destructive authority cancellation", e)
	}
}
func TestMainToolAuthoringChangedPlanRecoveryNoEffect(t *testing.T) {
	f := mainAuthorSetup(t)
	plan := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
	approval := mainAuthorApproval(t, plan, "widget")
	raw, _ := json.Marshal(approval)
	child := exec.Command(os.Args[0], "-test.run=^TestMainToolAuthoringCrashChild$")
	child.Env = append(os.Environ(), "OCMI_AUTHORING_CRASH_HOME="+f.home, "OCMI_AUTHORING_CRASH_PHASE=active", "OCMI_AUTHORING_CRASH_APPROVAL="+string(raw))
	out, e := child.CombinedOutput()
	if exit, ok := e.(*exec.ExitError); !ok || exit.ExitCode() != 89 {
		t.Fatalf("actual active crash: %v %.400s", e, out)
	}
	lock := filepath.Join(f.home, "local", "bindings", ".setup-lock")
	lockID := mainToolID(t, lock)
	file := filepath.Join(f.home, "local", "bindings", "tool-storage", "plans", "widget.json")
	bytes, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(file, file+".saved"); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(file, bytes, 0600); e != nil {
		t.Fatal(e)
	}
	result := mainAuthorCall(f, "recover", "coordinator", approval)
	if result.OK {
		t.Error("replaced plan activated by stale approval")
	}
	if current, e := localNativeID(lock); e != nil || current != lockID {
		t.Fatal("recovery mutated admission before checking current plan native authority", e)
	}
}
func TestMainToolAuthoringTypedAuthorityFailure(t *testing.T) {
	f := mainAuthorSetup(t)
	result := mainAuthorCall(f, "prepare", "worker", mainAuthorOptions(f, "widget"))
	if result.OK {
		t.Fatal("wrong actor admitted")
	}
	for _, key := range []string{"reason", "stage", "nextAction"} {
		if value, ok := result.Data[key].(string); !ok || strings.TrimSpace(value) == "" {
			t.Fatal("authoring failure lacks bounded coordinator diagnosis", key, result)
		}
	}
	mainAuthorNoEffect(t, f)
}

func TestMainToolAuthoringPreparedOnlyCancel(t *testing.T) {
	f := mainAuthorSetup(t)
	plan := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
	mainAuthorApply(t, f, plan, "widget")
	file := filepath.Join(f.home, "local", "bindings", "tool-storage", "plans", "widget.json")
	before, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	cancel := map[string]any{"toolId": "widget", "planId": plan.Data["planId"], "planSha256": plan.Data["planSha256"]}
	result := mainAuthorCall(f, "cancel", "coordinator", cancel)
	if result.OK {
		t.Error("prepared-only cancel admitted after activation")
	}
	after, e := os.ReadFile(file)
	if e != nil || string(after) != string(before) {
		t.Fatal("cancel destroyed active provider authority", e)
	}
	project, r := mainAuthorProject(t, f, "after-rejected-cancel")
	r = mainToolReady(t, project, r)
	mainToolRun(t, project, r.Revision)
}
func TestMainToolAuthoringEmptyCustomBeforeEffect(t *testing.T) {
	f := mainAuthorSetup(t)
	options := mainAuthorOptions(f, "widget")
	options["stores"] = []any{}
	if result := mainAuthorCall(f, "prepare", "coordinator", options); result.OK {
		t.Fatal("producer accepted custom profile that ordinary consumer cannot use", result)
	}
	mainAuthorNoEffect(t, f)
}
func TestMainToolAuthoringRecoverScopeAndStoppedClosure(t *testing.T) {
	f := mainAuthorSetup(t)
	plan := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
	approval := mainAuthorApproval(t, plan, "widget")
	raw, _ := json.Marshal(approval)
	child := exec.Command(os.Args[0], "-test.run=^TestMainToolAuthoringCrashChild$")
	child.Env = append(os.Environ(), "OCMI_AUTHORING_CRASH_HOME="+f.home, "OCMI_AUTHORING_CRASH_PHASE=active", "OCMI_AUTHORING_CRASH_APPROVAL="+string(raw))
	out, e := child.CombinedOutput()
	if exit, ok := e.(*exec.ExitError); !ok || exit.ExitCode() != 89 {
		t.Fatalf("qualified active crash: %v %.400s", e, out)
	}
	lock := filepath.Join(f.home, "local", "bindings", ".setup-lock")
	before := mainToolID(t, lock)
	wrong := mainAuthorApproval(t, plan, "widget")
	wrong["planSha256"] = strings.Repeat("0", 64)
	rejected := mainAuthorCall(f, "recover", "coordinator", wrong)
	if rejected.OK {
		t.Error("wrong recovery approval admitted")
	}
	if after, e := localNativeID(lock); e != nil || after != before {
		t.Fatal("wrong recovery destroyed original admission and future recovery", e)
	}
	c, _ := resolveLayout(f.home, "installation")
	db, e := openLayout(c)
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec("UPDATE project SET stopped=1")
	db.Close()
	if e != nil {
		t.Fatal(e)
	}
	recovered := mainAuthorCall(f, "recover", "coordinator", approval)
	if !recovered.OK || recovered.Data["phase"] != "active" {
		t.Fatal("STOP blocked verified metadata-only active closure", recovered)
	}
}

func TestMainToolAuthoringPendingInspect(t *testing.T) {
	f := mainAuthorSetup(t)
	_, node, _ := mainAuthorNativeTools(t)
	options := mainAuthorOptions(f, "go")
	options["executable"] = node
	plan := mainAuthorPrepare(t, f, options)
	failed := mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, plan, "go"))
	if failed.OK {
		t.Fatal("wrong native provider falsely configured")
	}
	inspected := mainAuthorCall(f, "inspect", "coordinator", map[string]any{"toolId": "go"})
	phase, _ := inspected.Data["phase"].(string)
	if phase != "pending" && phase != "unresolved" && phase != "failed" {
		t.Fatal("created/failed resources reported as merely prepared or disappeared", inspected)
	}
	for _, key := range []string{"reason", "stage", "nextAction"} {
		if value, ok := inspected.Data[key].(string); !ok || strings.TrimSpace(value) == "" {
			t.Fatal("pending inspection omitted actionable cause", key, inspected)
		}
	}
	found := false
	for _, value := range mainAuthorDataStrings(inspected.Data) {
		if value == "fictional-owner" {
			found = true
		}
	}
	if !found {
		t.Fatal("pending resources lost their declared owner", inspected)
	}
}

func TestMainToolAuthoringStoppedInspect(t *testing.T) {
	f := mainAuthorSetup(t)
	plan := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
	mainAuthorApply(t, f, plan, "widget")
	c, _ := resolveLayout(f.home, "installation")
	db, e := openLayout(c)
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec("UPDATE project SET stopped=1")
	db.Close()
	if e != nil {
		t.Fatal(e)
	}
	inspected := mainAuthorCall(f, "inspect", "coordinator", map[string]any{"toolId": "widget"})
	if !inspected.OK || inspected.Data["phase"] != "active" {
		t.Fatal("STOP blocked read-only inspection of established resources", inspected)
	}
}
func TestMainToolAuthoringUpdaterRecoveryRouting(t *testing.T) {
	f := mainAuthorSetup(t)
	before := UpdateHome("recover", f.home, "", "", "")
	if before.OK || before.Data["reason"] != "no_update" {
		t.Fatal("external no-update recovery control invalid", before)
	}
	base := filepath.Join(f.home, "local", "bindings", "tool-storage")
	if e := os.MkdirAll(base, 0700); e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(base, "transaction.json")
	raw := []byte("{\"unknown\":\"retain this exact data\"}")
	if e := os.WriteFile(file, raw, 0600); e != nil {
		t.Fatal(e)
	}
	after := UpdateHome("recover", f.home, "", "", "")
	if after.Data["reason"] != "no_update" {
		t.Fatal("tool transaction disabled existing updater recovery route", after)
	}
	got, e := os.ReadFile(file)
	if e != nil || string(got) != string(raw) {
		t.Fatal("updater recovery changed unrelated tool state")
	}
}
func TestMainToolAuthoringClosedNestedAndReceipt(t *testing.T) {
	t.Run("nested-store-case", func(t *testing.T) {
		f := mainAuthorSetup(t)
		p := mainAuthorOptions(f, "widget")
		p["stores"] = []any{map[string]any{"Key": "WIDGET_CACHE", "path": "cache", "kind": "cache"}}
		result := mainAuthorCall(f, "prepare", "coordinator", p)
		if result.OK {
			t.Fatal("noncanonical nested store field admitted", result)
		}
		mainAuthorNoEffect(t, f)
	})
	for _, kind := range []string{"case-alias", "invalid-verification"} {
		t.Run(kind, func(t *testing.T) {
			f := mainAuthorSetup(t)
			plan := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
			mainAuthorApply(t, f, plan, "widget")
			file := filepath.Join(f.home, "local", "bindings", "tool-storage", "receipts", "widget.json")
			raw, e := os.ReadFile(file)
			if e != nil {
				t.Fatal(e)
			}
			var value map[string]any
			if json.Unmarshal(raw, &value) != nil {
				t.Fatal("receipt fixture")
			}
			if kind == "case-alias" {
				value["Phase"] = "active"
			} else {
				value["verification"] = "invented-proof"
			}
			mainToolWrite(t, file, value)
			project, r := mainAuthorProject(t, f, "closed-receipt")
			if r.OK {
				r = request(project, "task.ready", "main", r.Revision, map[string]any{})
			}
			if r.OK {
				t.Fatal("fresh bind accepted ambiguous/impossible native receipt", kind, r)
			}
		})
	}
}

func mainAuthorDataStrings(value any) []string {
	raw, _ := json.Marshal(value)
	var decoded any
	_ = json.Unmarshal(raw, &decoded)
	var out []string
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			out = append(out, x)
		case []any:
			for _, c := range x {
				walk(c)
			}
		case map[string]any:
			for _, c := range x {
				walk(c)
			}
		}
	}
	walk(decoded)
	return out
}
func TestMainToolAuthoringConcretePlanAndInspect(t *testing.T) {
	f := mainAuthorSetup(t)
	options := mainAuthorOptions(f, "widget")
	prepared := mainAuthorPrepare(t, f, options)
	found := false
	for _, value := range mainAuthorDataStrings(prepared.Data) {
		if strings.HasPrefix(value, f.data) {
			found = true
		}
	}
	if !found {
		t.Fatal("prepared response omitted selected concrete effect paths", prepared)
	}
	inspected := mainAuthorCall(f, "inspect", "coordinator", map[string]any{"toolId": "widget"})
	if !inspected.OK || inspected.Data["phase"] != "prepared" {
		t.Fatal("prepared plan cannot be inspected by coordinator", inspected)
	}
	invalid := mainAuthorApproval(t, prepared, "widget")
	invalid["planSha256"] = strings.Repeat("0", 64)
	refused := mainAuthorCall(f, "apply", "coordinator", invalid)
	if refused.OK {
		t.Fatal("changed approval accepted")
	}
	for _, key := range []string{"reason", "stage", "nextAction"} {
		if value, ok := refused.Data[key].(string); !ok || strings.TrimSpace(value) == "" {
			t.Fatal("refusal omitted bounded actionable diagnosis", key, refused)
		}
	}
}
func TestMainToolAuthoringProviderIdentityFreshness(t *testing.T) {
	f := mainAuthorSetup(t)
	goExe, _, _ := mainAuthorNativeTools(t)
	image, e := os.ReadFile(goExe)
	if e != nil {
		t.Fatal(e)
	}
	// cmd/go discovers a relocated GOROOT from bin/ plus pkg/tool. This is
	// sufficient for fixed go env queries; it is not a complete Go installation.
	goRoot := filepath.Join(f.base, "native-go")
	for _, path := range []string{filepath.Join(goRoot, "bin"), filepath.Join(goRoot, "pkg", "tool")} {
		if e = os.MkdirAll(path, 0700); e != nil {
			t.Fatal(e)
		}
	}
	selected := filepath.Join(goRoot, "bin", "go")
	if strings.HasSuffix(strings.ToLower(goExe), ".exe") {
		selected += ".exe"
	}
	if e = os.WriteFile(selected, image, 0700); e != nil {
		t.Fatal(e)
	}
	options := mainAuthorOptions(f, "go")
	options["executable"] = selected
	plan := mainAuthorPrepare(t, f, options)
	mainAuthorApply(t, f, plan, "go")
	_, profile := mainAuthorProfile(t, f, "go")
	ref := map[string]any{"home": f.home, "toolId": "go", "sha256": mainToolHash(profile)}
	helper := mainToolFixture{base: f.base, home: f.home, data: f.data}
	project, r := helper.project(t, "provider-positive", ref, true)
	r = mainToolReady(t, project, r)
	mainToolRun(t, project, r.Revision)
	// Qualify the native replacement without changing the installed host tool.
	if e = os.Rename(selected, selected+".saved"); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(selected, image, 0700); e != nil {
		t.Fatal(e)
	}
	inspected := mainAuthorCall(f, "inspect", "coordinator", map[string]any{"toolId": "go"})
	if inspected.OK && inspected.Data["phase"] == "active" {
		t.Error("inspect reused stale provider identity")
	}
	project, r = helper.project(t, "provider-replaced", ref, true)
	if r.OK {
		r = request(project, "task.ready", "main", r.Revision, map[string]any{})
	}
	if r.OK {
		t.Fatal("fresh task reused receipt after selected provider replacement")
	}
}
func TestMainToolAuthoringActiveReplayFreshness(t *testing.T) {
	f := mainAuthorSetup(t)
	plan := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
	mainAuthorApply(t, f, plan, "widget")
	profile := filepath.Join(f.home, "local", "bindings", "tool-storage", "widget.json")
	if e := os.Rename(profile, profile+".saved"); e != nil {
		t.Fatal(e)
	}
	replay := mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, plan, "widget"))
	if replay.OK {
		t.Fatal("active replay claimed success despite missing profile", replay)
	}
	inspected := mainAuthorCall(f, "inspect", "coordinator", map[string]any{"toolId": "widget"})
	if inspected.OK && inspected.Data["phase"] == "active" {
		t.Fatal("inspect claimed active from receipt phase alone", inspected)
	}
}
func TestMainToolAuthoringBoundedScriptAdmission(t *testing.T) {
	f := mainAuthorSetup(t)
	_, node, _ := mainAuthorNativeTools(t)
	script := filepath.Join(f.base, "oversize-npm-cli.js")
	file, e := os.OpenFile(script, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	e = file.Truncate(environmentExecutableLimit + 1)
	file.Close()
	if e != nil {
		t.Fatal(e)
	}
	options := mainAuthorOptions(f, "npm")
	options["executable"] = node
	options["script"] = script
	result := mainAuthorCall(f, "prepare", "coordinator", options)
	if result.OK {
		t.Fatal("oversize provider script accepted", result)
	}
	if _, e = os.Lstat(f.data); !os.IsNotExist(e) {
		t.Fatal("oversize admission had external effects", e)
	}
}
