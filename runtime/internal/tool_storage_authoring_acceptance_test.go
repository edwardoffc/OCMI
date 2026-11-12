package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMainToolAuthoringCrashChild(t *testing.T) {
	home := os.Getenv("OCMI_AUTHORING_CRASH_HOME")
	if home == "" {
		t.Skip("physical crash helper only")
	}
	phase := os.Getenv("OCMI_AUTHORING_CRASH_PHASE")
	var approval map[string]any
	if json.Unmarshal([]byte(os.Getenv("OCMI_AUTHORING_CRASH_APPROVAL")), &approval) != nil {
		t.Fatal("fixture approval")
	}
	toolStorageAuthoringTestPhase = func(observed string) {
		if observed == phase {
			os.Exit(89)
		}
	}
	r := mainAuthorCall(mainAuthorFixture{home: home}, "apply", "coordinator", approval)
	t.Fatal("durable crash boundary not reached", phase, r)
}
func TestMainToolAuthoringCrashRecovery(t *testing.T) {
	for _, phase := range []string{"intent", "directories", "profile", "active"} {
		t.Run(phase, func(t *testing.T) {
			f := mainAuthorSetup(t)
			p := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
			approval := mainAuthorApproval(t, p, "widget")
			raw, _ := json.Marshal(approval)
			child := exec.Command(os.Args[0], "-test.run=^TestMainToolAuthoringCrashChild$")
			child.Env = append(os.Environ(), "OCMI_AUTHORING_CRASH_HOME="+f.home, "OCMI_AUTHORING_CRASH_PHASE="+phase, "OCMI_AUTHORING_CRASH_APPROVAL="+string(raw))
			out, e := child.CombinedOutput()
			if exit, ok := e.(*exec.ExitError); !ok || exit.ExitCode() != 89 {
				t.Fatalf("actual crash boundary: %v %.400s", e, out)
			}
			profile := filepath.Join(f.home, "local", "bindings", "tool-storage", "widget.json")
			if _, e = os.Lstat(profile); e == nil {
				project, r := mainAuthorProject(t, f, "before-recovery")
				if r.OK {
					r = request(project, "task.ready", "main", r.Revision, map[string]any{})
				}
				if r.OK {
					t.Fatal("pending transaction admitted new ordinary work", phase, r)
				}
			}
			identities := map[string]string{}
			for _, path := range []string{f.data, filepath.Join(f.data, "widget"), filepath.Join(f.data, "widget", "cache")} {
				if _, e = os.Lstat(path); e == nil {
					identities[path] = mainToolID(t, path)
				}
			}
			recovered := mainAuthorCall(f, "recover", "coordinator", approval)
			for path, id := range identities {
				if mainToolID(t, path) != id {
					t.Fatal("recovery adopted/deleted/replaced retained data")
				}
			}
			if phase == "active" {
				if !recovered.OK || recovered.Data["phase"] != "active" {
					t.Fatal("completed receipt could not finish metadata closure", recovered)
				}
				project, r := mainAuthorProject(t, f, "after-recovery")
				r = mainToolReady(t, project, r)
				mainToolRun(t, project, r.Revision)
			} else if recovered.OK && recovered.Data["phase"] == "active" {
				t.Fatal("unproved interrupted setup activated", phase, recovered)
			}
		})
	}
}

func TestMainToolAuthoringPendingAndStop(t *testing.T) {
	for _, kind := range []string{"pending", "stop"} {
		t.Run(kind, func(t *testing.T) {
			f := mainAuthorSetup(t)
			candidate := filepath.Join(f.base, "candidate")
			acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
			before := acceptancePublicSnapshot(t, f.home)
			if kind == "pending" {
				directory := filepath.Join(f.home, "local", "bindings", "tool-storage")
				if e := os.Mkdir(directory, 0700); e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(filepath.Join(directory, "transaction.json"), []byte("{\"unexpected\":\"unknown\"}"), 0600); e != nil {
					t.Fatal(e)
				}
				if r := UpdateHome("apply", f.home, candidate, strings.Repeat("a", 40), strings.Repeat("b", 40)); r.OK {
					t.Fatal("unknown tool transaction bypassed updater", r)
				}
			} else {
				ctx, _ := resolveLayout(f.home, "installation")
				db, e := openLayout(ctx)
				if e != nil {
					t.Fatal(e)
				}
				_, e = db.Exec("UPDATE project SET stopped=1")
				db.Close()
				if e != nil {
					t.Fatal(e)
				}
			}
			if r := mainAuthorCall(f, "prepare", "coordinator", mainAuthorOptions(f, "widget")); r.OK {
				t.Fatal("new setup started with pending/STOP", kind, r)
			}
			mainAuthorNoEffect(t, f)
			if !reflect.DeepEqual(before, acceptancePublicSnapshot(t, f.home)) {
				t.Fatal("refusal changed public image")
			}
		})
	}
}
func TestMainToolAuthoringPlanReplacement(t *testing.T) {
	f := mainAuthorSetup(t)
	p := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
	file := filepath.Join(f.home, "local", "bindings", "tool-storage", "plans", "widget.json")
	raw, e := os.ReadFile(file)
	if e != nil {
		t.Fatal("bounded canonical plan missing", e)
	}
	if e = os.Rename(file, file+".saved"); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(file, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if r := mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, p, "widget")); r.OK {
		t.Fatal("same-byte plan identity replacement accepted", r)
	}
	mainAuthorNoEffect(t, f)
}
func TestMainToolAuthoringFailedProbeRetainsData(t *testing.T) {
	f := mainAuthorSetup(t)
	options := mainAuthorOptions(f, "go")
	_, wrongGo, _ := mainAuthorNativeTools(t)
	// Native Node treats the fixed Go argv as a missing script and exits. A Go
	// test executable here would rerun tests recursively rather than probe Go.
	options["executable"] = wrongGo
	p := mainAuthorPrepare(t, f, options)
	r := mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, p, "go"))
	if r.OK {
		t.Fatal("wrong native executable falsely verified", r)
	}
	retained := filepath.Join(f.data, "go")
	if _, e := os.Stat(retained); e != nil {
		t.Fatal("failed probe data silently deleted instead of retained", e)
	}
	if _, e := os.Lstat(filepath.Join(f.home, "local", "bindings", "tool-storage", "go.json")); !os.IsNotExist(e) {
		t.Fatal("failed probe published usable profile", e)
	}
	recovered := mainAuthorCall(f, "recover", "coordinator", mainAuthorApproval(t, p, "go"))
	if recovered.OK && recovered.Data["phase"] == "active" {
		t.Fatal("failed probe recovered as configured", recovered)
	}
	if _, e := os.Stat(retained); e != nil {
		t.Fatal("recovery removed unproved tool data", e)
	}
}
func TestMainToolAuthoringOneWinner(t *testing.T) {
	f := mainAuthorSetup(t)
	p := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
	results := make(chan Response, 2)
	for i := 0; i < 2; i++ {
		go func() { results <- mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, p, "widget")) }()
	}
	one, two := <-results, <-results
	if !one.OK && !two.OK {
		t.Fatal("no legitimate setup winner", one, two)
	}
	if r := mainAuthorCall(f, "inspect", "coordinator", map[string]any{"toolId": "widget"}); !r.OK || r.Data["phase"] != "active" {
		t.Fatal("concurrent apply corrupted binding", r)
	}
}

type mainAuthorFixture struct{ base, home, data string }

func mainAuthorSetup(t *testing.T) mainAuthorFixture {
	t.Helper()
	f := mainAuthorFixture{base: t.TempDir()}
	f.home = filepath.Join(f.base, "installed home Ω")
	f.data = filepath.Join(f.base, "ToolData Ω")
	acceptanceUpdateFactory(t, f.home, strings.Repeat("a", 40))
	if r := InstallHome(f.home, false); !r.OK {
		t.Fatal("qualified native installation", r)
	}
	if r := CoordinatorPrepare(f.home, "en", "Fictional client"); !r.OK {
		t.Fatal("prepare coordinator", r)
	}
	if r := CoordinatorConfirm(f.home, "Fictional client", "user-current", "Fictional owner binding evidence"); !r.OK {
		t.Fatal("confirm coordinator", r)
	}
	return f
}
func mainAuthorCall(f mainAuthorFixture, action, actor string, payload any) Response {
	raw, _ := json.Marshal(payload)
	return Handle(Request{SchemaVersion: 1, Action: "tool-storage." + action, ProjectRoot: f.home, RootKind: "installation", Actor: actor, RequestID: "fictional-" + action, Payload: raw})
}
func mainAuthorOptions(f mainAuthorFixture, tool string) map[string]any {
	p := map[string]any{"toolId": tool, "toolDataRoot": f.data, "owner": "fictional-owner", "purpose": "Fictional selected tool storage", "retention": "until approved maintenance", "maxBytes": 1048576, "maxAgeDays": 30, "executable": "", "script": "", "stores": []any{}, "scratchEnv": []string{}}
	if tool == "widget" {
		p["stores"] = []any{map[string]any{"key": "WIDGET_CACHE", "path": "cache", "kind": "cache"}}
		p["scratchEnv"] = []string{"WIDGET_TMP"}
	}
	return p
}
func mainAuthorPrepare(t *testing.T, f mainAuthorFixture, p map[string]any) Response {
	t.Helper()
	r := mainAuthorCall(f, "prepare", "coordinator", p)
	if !r.OK || r.Code != "TOOL_STORAGE_PREPARED" || r.Data["phase"] != "prepared" {
		if r.Code == "STALE_REVISION" {
			ctx, _ := resolveLayout(f.home, "installation")
			db, e := openLayout(ctx)
			if e != nil {
				t.Fatal(e)
			}
			var rev int64
			e = db.QueryRow("SELECT revision FROM project").Scan(&rev)
			db.Close()
			if e != nil {
				t.Fatal(e)
			}
			raw, _ := json.Marshal(p)
			control := Handle(Request{SchemaVersion: 1, Action: "tool-storage.prepare", ProjectRoot: f.home, RootKind: "installation", Actor: "coordinator", RequestID: "baseline-valid-revision", ExpectedRevision: rev, Payload: raw})
			t.Fatalf("authoring route absent or wrongly dispatched: native initial=%s, qualified current-revision control=%s", r.Code, control.Code)
		}
		t.Fatal("native authoring preparation", r)
	}
	id, ok := r.Data["planId"].(string)
	if !ok || !isUUID(id) {
		t.Fatal("bounded plan identity", r)
	}
	sum, ok := r.Data["planSha256"].(string)
	if !ok || !toolStorageHash(sum) {
		t.Fatal("immutable approval hash", r)
	}
	return r
}
func mainAuthorApproval(t *testing.T, p Response, tool string) map[string]any {
	t.Helper()
	return map[string]any{"toolId": tool, "planId": p.Data["planId"], "planSha256": p.Data["planSha256"], "consentReference": "fictional-current-owner-approval"}
}
func mainAuthorApply(t *testing.T, f mainAuthorFixture, p Response, tool string) Response {
	t.Helper()
	r := mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, p, tool))
	if !r.OK || r.Code != "TOOL_STORAGE_ACTIVE" || r.Data["phase"] != "active" {
		t.Fatal("native apply", r)
	}
	return r
}
func mainAuthorProfile(t *testing.T, f mainAuthorFixture, tool string) (map[string]any, []byte) {
	t.Helper()
	raw, e := os.ReadFile(filepath.Join(f.home, "local", "bindings", "tool-storage", tool+".json"))
	if e != nil {
		t.Fatal(e)
	}
	var p map[string]any
	if json.Unmarshal(raw, &p) != nil {
		t.Fatal("profile JSON")
	}
	return p, raw
}
func mainAuthorProject(t *testing.T, f mainAuthorFixture, name string) (string, Response) {
	t.Helper()
	helper := mainToolFixture{base: f.base, home: f.home, data: f.data, cache: filepath.Join(f.data, "widget", "cache"), profile: filepath.Join(f.home, "local", "bindings", "tool-storage", "widget.json")}
	return helper.project(t, name, helper.ref(t), true)
}
func mainAuthorNoEffect(t *testing.T, f mainAuthorFixture) {
	t.Helper()
	if _, e := os.Lstat(f.data); !os.IsNotExist(e) {
		t.Fatal("external data created before consent/refused approval", e)
	}
	if _, e := os.Lstat(filepath.Join(f.home, "local", "bindings", "tool-storage", "widget.json")); !os.IsNotExist(e) {
		t.Fatal("profile published before consent", e)
	}
}
func TestMainToolAuthoringFixtureControl(t *testing.T) {
	f := mainAuthorSetup(t)
	ctx, info, e := verifyFactoryHome(f.home)
	if e != nil {
		t.Fatal(e)
	}
	receipt, e := currentInstallation(ctx, info)
	if e != nil {
		t.Fatal(e)
	}
	binding, e := readCoordinatorBindingContext(ctx)
	if e != nil || binding.Phase != "confirmed" || !currentCoordinator(ctx, receipt, binding) {
		t.Fatal("actual confirmed fixture", e)
	}
	// Qualify both actual native replacement detection and factory tamper refusal.
	file := filepath.Join(f.home, "README.md")
	before := mainToolID(t, file)
	raw, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(file, filepath.Join(f.base, "README.original")); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(file, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if mainToolID(t, file) == before {
		t.Fatal("same-byte replacement premise failed")
	}
	if _, _, e = verifyFactoryHome(f.home); e != nil {
		t.Fatal("identical public bytes should retain checksum validity", e)
	}
	if e = os.WriteFile(file, []byte("Fictional targeted wrong bytes"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, _, e = verifyFactoryHome(f.home); e == nil {
		t.Fatal("factory oracle accepted modified public bytes")
	}
}
func TestMainToolAuthoringPrepareConsentAndConsumer(t *testing.T) {
	f := mainAuthorSetup(t)
	opts := mainAuthorOptions(f, "widget")
	prepared := mainAuthorPrepare(t, f, opts)
	mainAuthorNoEffect(t, f)
	again := mainAuthorPrepare(t, f, opts)
	if again.Data["planId"] != prepared.Data["planId"] || again.Data["planSha256"] != prepared.Data["planSha256"] {
		t.Fatal("same selection generated another plan")
	}
	bad := mainAuthorApproval(t, prepared, "widget")
	bad["planSha256"] = strings.Repeat("0", 64)
	if r := mainAuthorCall(f, "apply", "coordinator", bad); r.OK {
		t.Fatal("changed approval admitted", r)
	}
	mainAuthorNoEffect(t, f)
	active := mainAuthorApply(t, f, prepared, "widget")
	if active.Data["verification"] != "path-variables-only" {
		t.Fatal("unknown tool falsely verified", active)
	}
	p, raw := mainAuthorProfile(t, f, "widget")
	if p["toolId"] != "widget" || !strings.HasPrefix(p["consentReference"].(string), "ocmi-tool-storage:") {
		t.Fatal("profile not tied to native receipt")
	}
	receiptPath := filepath.Join(f.home, "local", "bindings", "tool-storage", "receipts", "widget.json")
	receiptBytes, e := os.ReadFile(receiptPath)
	if e != nil {
		t.Fatal("native authority receipt missing", e)
	}
	for _, name := range []string{"project-a", "project-b"} {
		project, r := mainAuthorProject(t, f, name)
		r = mainToolReady(t, project, r)
		mainToolRun(t, project, r.Revision)
		observed := mainToolObserved(t, project)
		if observed["cache"] != filepath.Join(f.data, "widget", "cache") || observed["scratch"] != observed["tmp"] {
			t.Fatal("authored profile not consumed by real ordinary child", observed)
		}
	}
	after, e := os.ReadFile(receiptPath)
	if e != nil || !bytes.Equal(after, receiptBytes) {
		t.Fatal("use rewrote authority")
	}
	_, after = mainAuthorProfile(t, f, "widget")
	if !bytes.Equal(raw, after) {
		t.Fatal("use rewrote profile")
	}
	replay := mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, prepared, "widget"))
	if !replay.OK {
		t.Fatal("exact apply replay", replay)
	}
	if _, e = os.Lstat(filepath.Join(f.home, "local", "bindings", ".setup-lock")); !os.IsNotExist(e) {
		t.Fatal("successful setup leaked admission lock", e)
	}
}
func TestMainToolAuthoringBoundaries(t *testing.T) {
	for _, kind := range []string{"actor", "root-kind", "unconfirmed", "duplicate", "alias", "unknown", "relative", "home-overlap", "existing-tool", "unsafe-store", "arbitrary-command"} {
		t.Run(kind, func(t *testing.T) {
			f := mainAuthorSetup(t)
			p := mainAuthorOptions(f, "widget")
			actor := "coordinator"
			rootKind := "installation"
			switch kind {
			case "actor":
				actor = "worker"
			case "root-kind":
				rootKind = "project"
			case "unconfirmed":
				ctx, _ := resolveLayout(f.home, "installation")
				b, e := readCoordinatorBindingContext(ctx)
				if e != nil {
					t.Fatal(e)
				}
				b.Phase = "handover_pending"
				if e = writeCoordinatorBinding(ctx, b); e != nil {
					t.Fatal(e)
				}
			case "relative":
				p["toolDataRoot"] = "relative"
			case "home-overlap":
				p["toolDataRoot"] = filepath.Join(f.home, "new ToolData")
			case "existing-tool":
				if e := os.MkdirAll(filepath.Join(f.data, "widget"), 0700); e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(filepath.Join(f.data, "widget", "foreign.txt"), []byte("not owned"), 0600); e != nil {
					t.Fatal(e)
				}
			case "unsafe-store":
				p["stores"] = []any{map[string]any{"key": "PATH", "path": "cache", "kind": "cache"}}
			case "arbitrary-command":
				p["executable"] = os.Args[0]
			case "unknown":
				p["extra"] = "not allowed"
			}
			raw, _ := json.Marshal(p)
			if kind == "duplicate" {
				raw = bytes.Replace(raw, []byte("\"toolId\":\"widget\""), []byte("\"toolId\":\"widget\",\"toolId\":\"widget\""), 1)
			}
			if kind == "alias" {
				raw = bytes.Replace(raw, []byte("\"toolId\""), []byte("\"ToolId\""), 1)
			}
			r := Handle(Request{SchemaVersion: 1, Action: "tool-storage.prepare", ProjectRoot: f.home, RootKind: rootKind, Actor: actor, RequestID: "boundary", Payload: raw})
			if r.OK {
				t.Fatal("unsafe preparation accepted", kind, r)
			}
			if kind != "existing-tool" {
				mainAuthorNoEffect(t, f)
			} else {
				b, e := os.ReadFile(filepath.Join(f.data, "widget", "foreign.txt"))
				if e != nil || string(b) != "not owned" {
					t.Fatal("existing data adopted/mutated")
				}
			}
		})
	}
}
func TestMainToolAuthoringReceiptFreshness(t *testing.T) {
	for _, kind := range []string{"missing", "phase", "same-bytes", "home", "store"} {
		t.Run(kind, func(t *testing.T) {
			f := mainAuthorSetup(t)
			p := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
			mainAuthorApply(t, f, p, "widget")
			project, r := mainAuthorProject(t, f, "freshness")
			r = mainToolReady(t, project, r)
			file := filepath.Join(f.home, "local", "bindings", "tool-storage", "receipts", "widget.json")
			raw, e := os.ReadFile(file)
			if e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "missing":
				e = os.Rename(file, file+".saved")
			case "same-bytes":
				e = os.Rename(file, file+".saved")
				if e == nil {
					e = os.WriteFile(file, raw, 0600)
				}
			case "phase", "home":
				var m map[string]any
				if json.Unmarshal(raw, &m) != nil {
					t.Fatal("receipt fixture")
				}
				if kind == "phase" {
					m["phase"] = "committing"
				} else {
					m["home"] = filepath.Join(f.base, "other home")
				}
				mainToolWrite(t, file, m)
			case "store":
				cache := filepath.Join(f.data, "widget", "cache")
				e = os.Rename(cache, cache+".saved")
				if e == nil {
					e = os.Mkdir(cache, 0700)
				}
			}
			if e != nil {
				t.Fatal(e)
			}
			denied := request(project, "run", "worker", r.Revision, map[string]any{})
			if denied.OK && denied.Data["operationState"] == "succeeded" {
				t.Fatal("drifted authority launched", kind, denied)
			}
			if _, e = os.Lstat(filepath.Join(project, "tool-storage-effect.json")); !os.IsNotExist(e) {
				t.Fatal("child effect after receipt drift", e)
			}
		})
	}
}
func TestMainToolAuthoringExistingRootAndCancel(t *testing.T) {
	f := mainAuthorSetup(t)
	if e := os.Mkdir(f.data, 0700); e != nil {
		t.Fatal(e)
	}
	marker := filepath.Join(f.data, "foreign.txt")
	if e := os.WriteFile(marker, []byte("external root"), 0600); e != nil {
		t.Fatal(e)
	}
	p := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
	cancel := map[string]any{"toolId": "widget", "planId": p.Data["planId"], "planSha256": p.Data["planSha256"]}
	if r := mainAuthorCall(f, "cancel", "coordinator", cancel); !r.OK {
		t.Fatal("cancel prepared plan", r)
	}
	if _, e := os.Lstat(filepath.Join(f.data, "widget")); !os.IsNotExist(e) {
		t.Fatal("cancel created store")
	}
	p = mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
	mainAuthorApply(t, f, p, "widget")
	raw, e := os.ReadFile(marker)
	if e != nil || string(raw) != "external root" {
		t.Fatal("existing shared root affected")
	}
	if r := mainAuthorCall(f, "inspect", "coordinator", map[string]any{"toolId": "widget"}); !r.OK || r.Data["phase"] != "active" {
		t.Fatal("inspect active", r)
	}
}
func TestMainToolAuthoringLegacyProfileFingerprint(t *testing.T) {
	f := mainToolSetup(t)
	root := filepath.Join(f.base, "legacy-project")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	command := map[string]any{"executable": os.Args[0], "cwd": ".", "toolStorage": f.ref(t)}
	raw, _ := json.Marshal(command)
	got, e := commandFingerprint(root, raw)
	if e != nil {
		t.Fatal(e)
	}
	executable, e := digest(os.Args[0])
	if e != nil {
		t.Fatal(e)
	}
	legacy := struct {
		Profile, Home, Data string
		Stores              []string
	}{mainToolID(t, f.profile) + ":" + f.ref(t)["sha256"].(string), mainToolID(t, f.home), mainToolID(t, f.data), []string{"WIDGET_CACHE:" + mainToolID(t, f.cache)}}
	old, _ := json.Marshal(struct {
		Command    json.RawMessage
		Executable string
		Profile    any
	}{raw, executable, legacy})
	if got != mainToolHash(old) {
		t.Fatal("already bound legacy profile fingerprint changed")
	}
}
func mainAuthorNativeTools(t *testing.T) (string, string, string) {
	t.Helper()
	goExe := os.Getenv("OCMI_TEST_GO")
	node := os.Getenv("OCMI_TEST_NODE")
	script := os.Getenv("OCMI_TEST_NPM_CLI")
	if goExe == "" {
		goExe, _ = exec.LookPath("go")
	}
	if node == "" {
		node, _ = exec.LookPath("node")
	}
	if script == "" {
		if npm, e := exec.LookPath("npm"); e == nil {
			if resolved, e := environmentCanonicalExecutable(npm); e == nil && strings.HasSuffix(resolved, "npm-cli.js") {
				script = resolved
			}
		}
		if script == "" {
			for _, candidate := range []string{filepath.Join(filepath.Dir(node), "node_modules", "npm", "bin", "npm-cli.js"), filepath.Join(filepath.Dir(node), "..", "lib", "node_modules", "npm", "bin", "npm-cli.js")} {
				if info, e := os.Stat(candidate); e == nil && info.Mode().IsRegular() {
					script = candidate
					break
				}
			}
		}
	}
	for _, p := range []string{goExe, node, script} {
		if p == "" {
			t.Fatal("actual Go/Node/npm prerequisite unavailable; not a simulated pass")
		}
	}
	for i, p := range []*string{&goExe, &node, &script} {
		resolved, e := environmentCanonicalExecutable(*p)
		if e != nil {
			t.Fatalf("resolve native %s prerequisite %q: %v", []string{"Go", "Node", "npm CLI"}[i], *p, e)
		}
		*p = resolved
	}
	return goExe, node, script
}
func TestMainToolAuthoringNativeProbes(t *testing.T) {
	goExe, node, script := mainAuthorNativeTools(t)
	f := mainAuthorSetup(t)
	previous := environmentRunOverride
	defer func() { environmentRunOverride = previous }()
	environmentRunOverride = func(ctx context.Context, spec commandSpec) (commandResult, error) {
		result, err := runOwned(ctx, spec)
		t.Logf("native authoring probe executable=%s exit=%d error=%v timeout=%t truncated=%t pending=%t unresolved=%t stopped=%t phase=%s class=%s group=%s pump=%s outputBytes=%d", filepath.Base(spec.Executable), result.ExitCode, err, result.TimedOut, result.Truncated, result.Pending, result.Unresolved, result.StoppedVerified, result.Phase, result.ErrorClass, result.GroupObservation, result.PumpObservation, len(result.Output))
		if filepath.Base(spec.Executable) == filepath.Base(goExe) {
			var observed map[string]string
			if json.Unmarshal([]byte(result.Output), &observed) == nil {
				for _, entry := range spec.Env {
					key, expected, ok := strings.Cut(entry, "=")
					if ok && (key == "GOCACHE" || key == "GOMODCACHE" || key == "GOPATH" || key == "GOBIN" || key == "GOTMPDIR") {
						t.Logf("native Go path %s exact=%t clean=%t", key, observed[key] == expected, filepath.Clean(observed[key]) == filepath.Clean(expected))
					}
				}
			}
		} else {
			for _, entry := range spec.Env {
				if expected, ok := strings.CutPrefix(entry, "npm_config_cache="); ok {
					t.Logf("native npm path clean=%t", filepath.Clean(strings.TrimSpace(result.Output)) == filepath.Clean(expected))
				}
			}
		}
		return result, err
	}
	for _, tool := range []string{"go", "npm"} {
		options := mainAuthorOptions(f, tool)
		if tool == "go" {
			options["executable"] = goExe
		} else {
			options["executable"] = node
			options["script"] = script
		}
		p := mainAuthorPrepare(t, f, options)
		if _, e := os.Lstat(filepath.Join(f.data, tool)); !os.IsNotExist(e) {
			t.Fatal("prepare executed before consent")
		}
		active := mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, p, tool))
		if !active.OK {
			mainAuthorLogProbeState(t, f, tool)
			t.Fatal("native apply", tool, active)
		}
		if active.Data["verification"] != "effective-paths-verified" {
			t.Fatal("real fixed probe not verified", active)
		}
		_, raw := mainAuthorProfile(t, f, tool)
		var policy toolStoragePolicy
		if json.Unmarshal(raw, &policy) != nil {
			t.Fatal("policy")
		}
		expected := 4
		if tool == "npm" {
			expected = 1
		}
		if len(policy.Stores) != expected {
			t.Fatal("qualified stores missing", tool, policy.Stores)
		}
	}
}
func TestMainToolAuthoringUpdateCompatibility(t *testing.T) {
	f := mainAuthorSetup(t)
	p := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
	mainAuthorApply(t, f, p, "widget")
	profile := filepath.Join(f.home, "local", "bindings", "tool-storage", "widget.json")
	receipt := filepath.Join(f.home, "local", "bindings", "tool-storage", "receipts", "widget.json")
	before := map[string]string{}
	for _, file := range []string{profile, receipt} {
		b, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		before[file] = mainToolID(t, file) + ":" + mainToolHash(b)
	}
	candidate := filepath.Join(f.base, "candidate")
	entry := acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
	for _, action := range []string{"apply", "rollback"} {
		source, next, selected := strings.Repeat("a", 40), strings.Repeat("b", 40), candidate
		if action == "rollback" {
			source, next, selected = next, source, ""
		}
		mainAuthorUpdate(t, entry, action, f.home, selected, source, next)
		after := map[string]string{}
		for _, file := range []string{profile, receipt} {
			b, e := os.ReadFile(file)
			if e != nil {
				t.Fatal(e)
			}
			after[file] = mainToolID(t, file) + ":" + mainToolHash(b)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatal("compatible update rewrote local binding", action)
		}
		project, r := mainAuthorProject(t, f, "after-"+action)
		r = mainToolReady(t, project, r)
		mainToolRun(t, project, r.Revision)
	}
}

func TestMainToolAuthoringUpdateChild(t *testing.T) {
	home := os.Getenv("OCMI_AUTHORING_UPDATE_HOME")
	if home == "" {
		t.Skip("actual candidate-entry helper only")
	}
	r := UpdateHome(os.Getenv("OCMI_AUTHORING_UPDATE_ACTION"), home, os.Getenv("OCMI_AUTHORING_UPDATE_CANDIDATE"), os.Getenv("OCMI_AUTHORING_UPDATE_SOURCE"), os.Getenv("OCMI_AUTHORING_UPDATE_NEXT"))
	if !r.OK {
		t.Fatal("actual candidate updater", r)
	}
}

func mainAuthorUpdate(t *testing.T, entry, action, home, candidate, source, next string) {
	t.Helper()
	child := exec.Command(entry, "-test.run=^TestMainToolAuthoringUpdateChild$", "-test.v")
	child.Env = append(os.Environ(), "OCMI_AUTHORING_UPDATE_HOME="+home, "OCMI_AUTHORING_UPDATE_ACTION="+action, "OCMI_AUTHORING_UPDATE_CANDIDATE="+candidate, "OCMI_AUTHORING_UPDATE_SOURCE="+source, "OCMI_AUTHORING_UPDATE_NEXT="+next)
	out, err := child.CombinedOutput()
	if err != nil {
		t.Fatalf("candidate-entry %s: %v %.1600s", action, err, out)
	}
}

func TestMainToolAuthoringUpdateProtocolControl(t *testing.T) {
	f := mainAuthorSetup(t)
	candidate := filepath.Join(f.base, "candidate")
	entry := acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
	mainAuthorUpdate(t, entry, "apply", f.home, candidate, strings.Repeat("a", 40), strings.Repeat("b", 40))
	mainAuthorUpdate(t, entry, "rollback", f.home, "", strings.Repeat("b", 40), strings.Repeat("a", 40))
	ctx, info, err := verifyFactoryHome(f.home)
	if err != nil || info.SourceCommit != strings.Repeat("a", 40) {
		t.Fatal("native update protocol control did not return original image", err)
	}
	if _, err = currentInstallation(ctx, info); err != nil {
		t.Fatal("native protocol control lost current installation", err)
	}
}
func TestMainToolAuthoringNativeCLI(t *testing.T) {
	binary := os.Getenv("OCMI_ROOT_BINARY")
	if binary == "" {
		t.Skip("actual native binary required by integration runner")
	}
	f := mainAuthorSetup(t)
	call := func(action string, payload any) Response {
		raw, _ := json.Marshal(map[string]any{"schemaVersion": 1, "action": "tool-storage." + action, "projectRoot": f.home, "rootKind": "installation", "actor": "coordinator", "requestId": "actual-cli-" + action, "expectedRevision": 0, "payload": payload})
		cmd := exec.Command(binary, "request")
		cmd.Stdin = bytes.NewReader(raw)
		out, e := cmd.CombinedOutput()
		var r Response
		if json.Unmarshal(out, &r) != nil {
			t.Fatalf("actual CLI invalid result: %v", e)
		}
		if e != nil || !r.OK {
			t.Fatal("actual CLI", action, e, r)
		}
		return r
	}
	p := call("prepare", mainAuthorOptions(f, "widget"))
	call("apply", mainAuthorApproval(t, p, "widget"))
	if r := call("inspect", map[string]any{"toolId": "widget"}); r.Data["phase"] != "active" {
		t.Fatal(r)
	}
	project, r := mainAuthorProject(t, f, "actual-cli-project")
	r = mainToolReady(t, project, r)
	mainToolRun(t, project, r.Revision)
	fmt.Println("TOOL_STORAGE_AUTHORING_NATIVE_CLI_OK")
}
