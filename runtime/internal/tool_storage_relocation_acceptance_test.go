package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type mainRelocationFixture struct {
	mainAuthorFixture
	goExe, packageDir, toolID string
}

// Ordinary native consumers use a fictional private provider home as well as
// their selected stores. Reuse the qualified Go telemetry-off ordering.
func mainRelocationIsolateProvider(t *testing.T, base, goExe string) {
	t.Helper()
	root := filepath.Join(base, "fixture-provider-home")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME"} {
		t.Setenv(key, root)
	}
	for _, name := range []string{"npm-userconfig", "npm-globalconfig"} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(strings.ReplaceAll(name, "-", "_config_"), path)
	}
	t.Setenv("GOENV", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOPROXY", "off")
	if goExe != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		r, err := runOwned(ctx, commandSpec{Executable: goExe, Args: []string{"telemetry", "off"}, Cwd: root, OutputLimit: 2048,
			Registered: func(int, int) error { return nil }})
		if err != nil || r.ExitCode != 0 || !r.StoppedVerified || r.Pending || r.Unresolved || r.Truncated || r.TimedOut {
			t.Fatal("fixture private native telemetry-off prerequisite", r, err)
		}
	}
}

func mainRelocationSetup(t *testing.T) mainRelocationFixture {
	t.Helper()
	f := mainRelocationFixture{mainAuthorFixture: mainAuthorSetup(t)}
	f.goExe, _, _ = mainAuthorNativeTools(t)
	mainRelocationIsolateProvider(t, f.base, f.goExe)
	options := mainAuthorOptions(f.mainAuthorFixture, "go")
	options["executable"] = f.goExe
	p := mainAuthorPrepare(t, f.mainAuthorFixture, options)
	mainAuthorApply(t, f.mainAuthorFixture, p, "go")
	f.packageDir = filepath.Join(f.data, "go", "workspace", "src", "selected-module")
	if err := os.MkdirAll(f.packageDir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"go.mod": "module fixture.example/retained\n\ngo 1.22.0\n", "retained.go": "package retained\nconst Value = \"unique offline fixture\"\n"} {
		if err := os.WriteFile(filepath.Join(f.packageDir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.data, "go", "modules", "offline-sentinel"), []byte("preserve offline bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.data, "foreign-sibling"), []byte("outside selected tool subtree"), 0600); err != nil {
		t.Fatal(err)
	}
	return f
}

func mainRelocationRef(t *testing.T, f mainRelocationFixture) map[string]any {
	t.Helper()
	tool := f.toolID
	if tool == "" {
		tool = "go"
	}
	_, raw := mainAuthorProfile(t, f.mainAuthorFixture, tool)
	return map[string]any{"home": f.home, "toolId": tool, "sha256": mainToolHash(raw)}
}

func mainRelocationProject(t *testing.T, f mainRelocationFixture, name, packageDir string) (string, Response) {
	command := map[string]any{"id": "selected-native-module", "executable": f.goExe, "args": []string{"-C", packageDir, "list", "-mod=readonly", "-f", "{{.Dir}}", "."}, "cwd": ".", "timeoutMs": 15000, "outputLimitBytes": 2048}
	return mainRelocationCommandProject(t, f, name, command)
}

func mainRelocationReadOnlyProject(t *testing.T, f mainRelocationFixture, name string) (string, Response) {
	t.Helper()
	command := map[string]any{"id": "selected-read-only-module", "executable": os.Args[0],
		"args": []string{"-test.run=^TestMainToolAuthoringRelocationChild$", "relocation-use-child"},
		"cwd":  ".", "timeoutMs": 15000, "outputLimitBytes": 2048}
	root, ready := mainRelocationCommandProject(t, f, name, command)
	if err := os.WriteFile(filepath.Join(root, "release"), []byte("read-only fixture may close"), 0600); err != nil {
		t.Fatal(err)
	}
	return root, ready
}

func mainRelocationCommandProject(t *testing.T, f mainRelocationFixture, name string, command map[string]any) (string, Response) {
	t.Helper()
	root := filepath.Join(f.base, name)
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	r := request(root, "init", "main", 0, map[string]any{"projectId": name, "deviceId": "fictional-device", "copyId": name, "owner": "main"})
	if !r.OK {
		t.Fatal(r)
	}
	for _, name := range []string{"contract.md", "input.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("Fictional selected native module read."), 0600); err != nil {
			t.Fatal(err)
		}
	}
	command["toolStorage"] = mainRelocationRef(t, f)
	r = request(root, "task.bind", "main", r.Revision, map[string]any{"taskId": "native-module", "outcomeId": "module-read", "taskRevision": 1, "completionScope": "target", "worker": "worker", "reviewer": "main", "contract": "contract.md", "inputs": []string{"input.txt"}, "command": command})
	return root, mainToolReady(t, root, r)
}

func mainRelocationRun(t *testing.T, root string, r Response, expectedDir string) {
	t.Helper()
	r = mainToolRun(t, root, r.Revision)
	db, err := openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var output string
	if err := db.QueryRow("SELECT output FROM operation WHERE id=?", r.Data["operationId"]).Scan(&output); err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(strings.TrimSpace(output)) != filepath.Clean(expectedDir) {
		mainToolRunFailureDiagnostic(t, root, r)
		t.Fatal("native Go did not consume selected preserved module: output mismatch")
	}
}

func mainRelocationSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	count := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		count++
		if count > 2048 {
			t.Fatal("fixture inventory unexpectedly grew")
		}
		if entry.Type()&os.ModeSymlink != 0 {
			t.Fatal("unqualified fixture link")
		}
		if !entry.IsDir() {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, _ := filepath.Rel(root, path)
			result[filepath.ToSlash(relative)] = mainToolHash(raw)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// A later explicitly requested ordinary Go consumer may populate its own build
// cache. Transition actions themselves must preserve the entire original tree.
func mainRelocationStableInputs(snapshot map[string]string) map[string]string {
	result := map[string]string{}
	for path, digest := range snapshot {
		if !strings.HasPrefix(path, "go/build-cache/") {
			result[path] = digest
		}
	}
	return result
}

func mainRelocationCall(f mainRelocationFixture, action string, payload any) Response {
	return mainAuthorCall(f.mainAuthorFixture, "relocate."+action, "coordinator", payload)
}

func mainRelocationOptions(t *testing.T, f mainRelocationFixture) map[string]any {
	t.Helper()
	ref := mainRelocationRef(t, f)
	return map[string]any{"toolId": "go", "profileSha256": ref["sha256"], "toolDataRoot": filepath.Join(f.base, "New ToolData Ω"), "maxEntries": 1024, "maxBytes": 16 << 20}
}

func mainRelocationPrepare(t *testing.T, f mainRelocationFixture, payload map[string]any) Response {
	t.Helper()
	r := mainRelocationCall(f, "prepare", payload)
	if !r.OK || r.Code != "TOOL_STORAGE_RELOCATION_PREPARED" || r.Data["phase"] != "prepared" {
		t.Fatal("relocation preparation", r)
	}
	id, _ := r.Data["planId"].(string)
	sum, _ := r.Data["planSha256"].(string)
	if !isUUID(id) || !toolStorageHash(sum) {
		t.Fatal("missing native immutable transition approval", r)
	}
	return r
}

func mainRelocationApproval(p Response) map[string]any {
	return map[string]any{"toolId": "go", "planId": p.Data["planId"], "planSha256": p.Data["planSha256"], "consentReference": "fictional-current-relocation-approval"}
}

func TestMainToolAuthoringRelocationNativeFixture(t *testing.T) {
	f := mainRelocationSetup(t)
	project, r := mainRelocationProject(t, f, "native-control", f.packageDir)
	mainRelocationRun(t, project, r, f.packageDir)
	data := mainRelocationSnapshot(t, filepath.Join(f.data, "go"))
	if data["workspace/src/selected-module/retained.go"] == "" || data["modules/offline-sentinel"] == "" {
		t.Fatal("native fixture lost durable/offline inputs")
	}
	entries := 0
	if err := filepath.WalkDir(filepath.Join(f.data, "go"), func(_ string, _ fs.DirEntry, err error) error { entries++; return err }); err != nil {
		t.Fatal(err)
	}
	t.Logf("qualified native module read; storeFiles=%d totalEntries=%d", len(data), entries)
}

func TestMainToolAuthoringRelocationMoveAndRollback(t *testing.T) {
	f := mainRelocationSetup(t)
	oldProject, r := mainRelocationProject(t, f, "before-move", f.packageDir)
	mainRelocationRun(t, oldProject, r, f.packageDir)
	old := mainRelocationSnapshot(t, f.data)
	opts := mainRelocationOptions(t, f)
	p := mainRelocationPrepare(t, f, opts)
	newRoot := opts["toolDataRoot"].(string)
	if _, err := os.Lstat(newRoot); !os.IsNotExist(err) {
		t.Fatal("prepare created destination before approval", err)
	}
	wrong := mainRelocationApproval(p)
	wrong["planSha256"] = strings.Repeat("0", 64)
	if r := mainRelocationCall(f, "apply", wrong); r.OK {
		t.Fatal("unapproved relocation accepted", r)
	}
	if _, err := os.Lstat(newRoot); !os.IsNotExist(err) {
		t.Fatal("wrong approval created destination", err)
	}
	applied := mainRelocationCall(f, "apply", mainRelocationApproval(p))
	if !applied.OK || applied.Data["phase"] != "active" {
		t.Fatal("preserving apply", applied)
	}
	if !reflect.DeepEqual(old, mainRelocationSnapshot(t, f.data)) {
		t.Fatal("old/foreign source data changed")
	}
	if !reflect.DeepEqual(mainRelocationSnapshot(t, filepath.Join(f.data, "go")), mainRelocationSnapshot(t, filepath.Join(newRoot, "go"))) {
		t.Fatal("copied native store bytes differ")
	}
	newPackage := filepath.Join(newRoot, "go", "workspace", "src", "selected-module")
	project, r := mainRelocationReadOnlyProject(t, f, "after-move")
	mainRelocationRun(t, project, r, newPackage)
	rolled := mainRelocationCall(f, "rollback", mainRelocationApproval(p))
	if !rolled.OK || rolled.Data["phase"] != "rolled_back" {
		t.Fatal("unchanged data reverse transition", rolled)
	}
	if !reflect.DeepEqual(old, mainRelocationSnapshot(t, f.data)) {
		t.Fatal("reverse transition changed original data before the next consumer")
	}
	project, r = mainRelocationProject(t, f, "after-reverse", f.packageDir)
	mainRelocationRun(t, project, r, f.packageDir)
	if !reflect.DeepEqual(mainRelocationStableInputs(old), mainRelocationStableInputs(mainRelocationSnapshot(t, f.data))) {
		t.Fatal("ordinary reverse consumer changed durable/offline/foreign inputs")
	}
	if _, err := os.Stat(filepath.Join(newRoot, "go", "workspace", "src", "selected-module", "retained.go")); err != nil {
		t.Fatal("rollback deleted preserved destination", err)
	}
}

func TestMainToolAuthoringRelocationCopyPremises(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "selected-source")
	target := filepath.Join(base, "known-owned-copy")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "durable.bin"), []byte("unique offline bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	held, err := os.OpenRoot(source)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	beforeID := mainToolID(t, filepath.Join(source, "durable.bin"))
	size, digest, err := checkpointGitDigestFile(held, "durable.bin", 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkpointGitCopyPayload(held, "durable.bin", target, 1024); err != nil {
		t.Fatal("native held-source/exclusive-target copy", err)
	}
	if raw, err := os.ReadFile(target); err != nil || int64(len(raw)) != size || mainToolHash(raw) != digest {
		t.Fatal("copied bytes changed", err)
	}
	if err := checkpointGitCopyPayload(held, "durable.bin", target, 1024); err == nil {
		t.Fatal("copy overwrote an existing destination")
	}
	if afterID := mainToolID(t, filepath.Join(source, "durable.bin")); afterID != beforeID {
		t.Fatal("copy changed source identity")
	}
	if err := os.Rename(filepath.Join(source, "durable.bin"), filepath.Join(source, "retained-original.bin")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "durable.bin"), []byte("unique offline bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	_, afterHash, err := checkpointGitDigestFile(held, "durable.bin", 1024)
	if err != nil || afterHash != digest || mainToolID(t, filepath.Join(source, "durable.bin")) == beforeID {
		t.Fatal("same-byte native replacement control invalid", err)
	}
}

func TestMainToolAuthoringRelocationNpmConsumer(t *testing.T) {
	f := mainRelocationFixture{mainAuthorFixture: mainAuthorSetup(t), toolID: "npm"}
	_, node, script := mainAuthorNativeTools(t)
	mainRelocationIsolateProvider(t, f.base, "")
	options := mainAuthorOptions(f.mainAuthorFixture, "npm")
	options["executable"], options["script"] = node, script
	p := mainAuthorPrepare(t, f.mainAuthorFixture, options)
	mainAuthorApply(t, f.mainAuthorFixture, p, "npm")
	cache := filepath.Join(f.data, "npm", "cache")
	if err := os.WriteFile(filepath.Join(cache, "offline-sentinel"), []byte("unique selected npm bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	command := map[string]any{"id": "selected-native-npm", "executable": node, "args": []string{script, "config", "get", "cache", "--logs-max=0", "--update-notifier=false"}, "cwd": ".", "timeoutMs": 15000, "outputLimitBytes": 2048}
	oldRoot, oldReady := mainRelocationCommandProject(t, f, "npm-current-control", command)
	mainRelocationRun(t, oldRoot, oldReady, cache)
	t.Log("qualified actual ordinary npm effective-cache consumer before relocation")
	before := mainRelocationSnapshot(t, f.data)
	ref := mainRelocationRef(t, f)
	newRoot := filepath.Join(f.base, "Selected npm ToolData")
	opts := map[string]any{"toolId": "npm", "profileSha256": ref["sha256"], "toolDataRoot": newRoot, "maxEntries": 1024, "maxBytes": 16 << 20}
	p = mainRelocationPrepare(t, f, opts)
	approval := mainRelocationApproval(p)
	approval["toolId"] = "npm"
	if r := mainRelocationCall(f, "apply", approval); !r.OK || r.Data["phase"] != "active" {
		t.Fatal("npm preserving apply", r)
	}
	if !reflect.DeepEqual(before, mainRelocationSnapshot(t, f.data)) {
		t.Fatal("npm original changed")
	}
	if !reflect.DeepEqual(mainRelocationSnapshot(t, filepath.Join(f.data, "npm")), mainRelocationSnapshot(t, filepath.Join(newRoot, "npm"))) {
		t.Fatal("npm copied bytes differ")
	}
	root, ready := mainRelocationCommandProject(t, f, "npm-new-generation", command)
	mainRelocationRun(t, root, ready, filepath.Join(newRoot, "npm", "cache"))
}

func TestMainToolAuthoringRelocationChild(t *testing.T) {
	child := false
	for _, arg := range os.Args {
		if arg == "relocation-use-child" {
			child = true
		}
	}
	if !child {
		t.Skip("registered native fixture child only")
	}
	if _, err := os.ReadFile(filepath.Join(os.Getenv("GOPATH"), "src", "selected-module", "retained.go")); err != nil {
		os.Exit(3)
	}
	fmt.Println(filepath.Join(os.Getenv("GOPATH"), "src", "selected-module"))
	if err := os.WriteFile("entered", []byte("selected store in use"), 0600); err != nil {
		os.Exit(4)
	}
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat("release"); err == nil {
			os.Exit(0)
		}
	}
	os.Exit(5)
}

type mainRelocationUse struct {
	root   string
	ready  Response
	result chan Response
	done   bool
}

func mainRelocationPair(t *testing.T, f mainRelocationFixture) []*mainRelocationUse {
	t.Helper()
	var uses []*mainRelocationUse
	for _, name := range []string{"active-project-a", "active-project-b"} {
		command := map[string]any{"id": "registered-store-use", "executable": os.Args[0], "args": []string{"-test.run=^TestMainToolAuthoringRelocationChild$", "relocation-use-child"}, "cwd": ".", "timeoutMs": 35000, "outputLimitBytes": 1024}
		root, r := mainRelocationCommandProject(t, f, name, command)
		uses = append(uses, &mainRelocationUse{root: root, ready: r, result: make(chan Response, 1)})
	}
	t.Cleanup(func() {
		for _, use := range uses {
			mainRelocationFinish(t, use)
		}
	})
	for _, use := range uses {
		go func(use *mainRelocationUse) {
			use.result <- request(use.root, "run", "worker", use.ready.Revision, map[string]any{})
		}(use)
	}
	for _, use := range uses {
		observed := false
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			if _, err := os.Stat(filepath.Join(use.root, "entered")); err == nil {
				observed = true
				break
			}
		}
		if !observed {
			t.Fatal("actual independent child did not reach selected-store barrier")
		}
	}
	return uses
}

func mainRelocationFinish(t *testing.T, use *mainRelocationUse) {
	t.Helper()
	if use.done {
		return
	}
	if err := os.WriteFile(filepath.Join(use.root, "release"), []byte("release own child"), 0600); err != nil {
		t.Error(err)
		return
	}
	select {
	case r := <-use.result:
		use.done = true
		if !r.OK || r.Data["operationState"] != "succeeded" {
			t.Error("owned child did not close successfully", r)
		}
	case <-time.After(40 * time.Second):
		t.Error("owned native fixture did not reach bounded stopped closure")
	}
}

func mainRelocationStoreUses(t *testing.T, root string) int {
	t.Helper()
	db, err := openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT count(*) FROM resource_use u JOIN resource_ledger r ON r.id=u.resource_id WHERE u.active=1 AND r.id LIKE 'ocmi-tool-storage-%'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestMainToolAuthoringRelocationConcurrentFixture(t *testing.T) {
	f := mainRelocationSetup(t)
	uses := mainRelocationPair(t, f)
	if mainRelocationStoreUses(t, uses[0].root) != 4 || mainRelocationStoreUses(t, uses[1].root) != 4 {
		t.Fatal("two actual native users are not independently recorded")
	}
	mainRelocationFinish(t, uses[0])
	if mainRelocationStoreUses(t, uses[0].root) != 0 || mainRelocationStoreUses(t, uses[1].root) != 4 {
		t.Fatal("closing A affected B or released no usage")
	}
	mainRelocationFinish(t, uses[1])
	if mainRelocationStoreUses(t, uses[1].root) != 0 {
		t.Fatal("B did not release after stopped proof")
	}
}

func TestMainToolAuthoringRelocationSharedBarrier(t *testing.T) {
	f := mainRelocationSetup(t)
	uses := mainRelocationPair(t, f)
	inspect := func(want string) {
		r := mainRelocationCall(f, "inspect", map[string]any{"toolId": "go"})
		if !r.OK || fmt.Sprint(r.Data["activeUses"]) != want || fmt.Sprint(r.Data["pendingUses"]) != "0" {
			t.Fatal("native cross-project use summary", want, r)
		}
	}
	refuse := func() {
		opts := mainRelocationOptions(t, f)
		r := mainRelocationCall(f, "prepare", opts)
		if r.OK || r.Code != "TOOL_STORAGE_IN_USE" {
			t.Fatal("transition ignored actual shared use", r)
		}
		if _, err := os.Lstat(opts["toolDataRoot"].(string)); !os.IsNotExist(err) {
			t.Fatal("busy transition created destination", err)
		}
	}
	inspect("2")
	refuse()
	mainRelocationFinish(t, uses[0])
	inspect("1")
	refuse()
	mainRelocationFinish(t, uses[1])
	inspect("0")
	mainRelocationPrepare(t, f, mainRelocationOptions(t, f))
}

func TestMainToolAuthoringRelocationRegistryAuthority(t *testing.T) {
	f := mainRelocationSetup(t)
	policy, _ := mainAuthorProfile(t, f.mainAuthorFixture, "go")
	identity, _ := policy["useRootIdentity"].(string)
	registry := filepath.Join(f.home, "local", "bindings", "tool-storage", "uses", "go")
	if policy["schemaVersion"] != float64(2) || identity == "" || identity != mainToolID(t, registry) {
		t.Fatal("lifecycle profile lacks immutable native registry authority")
	}
	uses := mainRelocationPair(t, f)
	retained := filepath.Join(f.base, "retained-known-registry")
	if err := os.Rename(registry, retained); err != nil {
		t.Fatal("native registry-loss premise", err)
	}
	replaced := false
	defer func() {
		if replaced {
			if err := os.Remove(registry); err != nil {
				t.Error(err)
			}
		}
		if err := os.Rename(retained, registry); err != nil {
			t.Error("restore known fixture registry before closing children", err)
		}
	}()
	for _, scenario := range []string{"missing", "same-path-new-directory"} {
		if scenario != "missing" {
			if err := os.Mkdir(registry, 0700); err != nil {
				t.Fatal(err)
			}
			replaced = true
		}
		opts := mainRelocationOptions(t, f)
		r := mainRelocationCall(f, "prepare", opts)
		if r.OK {
			t.Fatal("lost registry was silently interpreted as zero consumers", scenario, r)
		}
		if _, err := os.Lstat(opts["toolDataRoot"].(string)); !os.IsNotExist(err) {
			t.Fatal("unknown use created destination", err)
		}
	}
	_ = uses // cleanup waits for verified closure after the deferred registry restoration.
}

func TestMainToolAuthoringRelocationBoundaries(t *testing.T) {
	f := mainRelocationSetup(t)
	before := mainRelocationSnapshot(t, f.data)
	_, profileBefore := mainAuthorProfile(t, f.mainAuthorFixture, "go")
	for _, scenario := range []string{"wrong-sha", "upper-tool", "custom-tool", "source-overlap", "home-overlap", "missing-parent", "zero-entries", "entry-overflow", "zero-bytes", "byte-overflow", "insufficient-entry-budget", "insufficient-byte-budget", "extra-key", "case-key", "duplicate-key", "wrong-actor"} {
		t.Run(scenario, func(t *testing.T) {
			opts := mainRelocationOptions(t, f)
			switch scenario {
			case "wrong-sha":
				opts["profileSha256"] = strings.Repeat("0", 64)
			case "upper-tool":
				opts["toolId"] = "Go"
			case "custom-tool":
				opts["toolId"] = "unknown-tool"
			case "source-overlap":
				opts["toolDataRoot"] = f.data
			case "home-overlap":
				opts["toolDataRoot"] = filepath.Join(f.home, "local", "new-tools")
			case "missing-parent":
				opts["toolDataRoot"] = filepath.Join(f.base, "missing-parent", "new-tools")
			case "zero-entries":
				opts["maxEntries"] = 0
			case "entry-overflow":
				opts["maxEntries"] = 1025
			case "zero-bytes":
				opts["maxBytes"] = 0
			case "byte-overflow":
				opts["maxBytes"] = (16 << 20) + 1
			case "insufficient-entry-budget":
				opts["maxEntries"] = 1
			case "insufficient-byte-budget":
				opts["maxBytes"] = 1
			case "extra-key":
				opts["execute"] = "unapproved"
			case "case-key":
				opts["ToolId"] = "go"
			}
			raw, _ := json.Marshal(opts)
			if scenario == "duplicate-key" {
				raw = append(raw[:len(raw)-1], []byte(",\"toolId\":\"go\"}")...)
			}
			actor := "coordinator"
			if scenario == "wrong-actor" {
				actor = "worker"
			}
			r := Handle(Request{SchemaVersion: 1, Action: "tool-storage.relocate.prepare", ProjectRoot: f.home, RootKind: "installation", Actor: actor, RequestID: "fictional-relocate-invalid-" + scenario, Payload: raw})
			if r.OK {
				t.Fatal("invalid transition admitted", scenario, r)
			}
			if r.Data["reason"] == "" || r.Data["nextAction"] == "" {
				t.Fatal("unactionable transition refusal", r)
			}
			if !reflect.DeepEqual(before, mainRelocationSnapshot(t, f.data)) {
				t.Fatal("invalid preparation changed source")
			}
			_, profileAfter := mainAuthorProfile(t, f.mainAuthorFixture, "go")
			if string(profileBefore) != string(profileAfter) {
				t.Fatal("invalid preparation changed active profile")
			}
		})
	}
	if _, err := os.Lstat(filepath.Join(f.base, "New ToolData Ω")); !os.IsNotExist(err) {
		t.Fatal("refused preparation created external destination", err)
	}
}

func TestMainToolAuthoringRelocationSourceDrift(t *testing.T) {
	for _, scenario := range []string{"changed-bytes", "same-byte-file-replacement", "wrong-consent"} {
		t.Run(scenario, func(t *testing.T) {
			f := mainRelocationSetup(t)
			opts := mainRelocationOptions(t, f)
			p := mainRelocationPrepare(t, f, opts)
			approval := mainRelocationApproval(p)
			source := filepath.Join(f.packageDir, "retained.go")
			if scenario == "wrong-consent" {
				approval["consentReference"] = ""
			} else {
				raw, err := os.ReadFile(source)
				if err != nil {
					t.Fatal(err)
				}
				if scenario == "same-byte-file-replacement" {
					if err := os.Rename(source, filepath.Join(f.base, "retained-source-original")); err != nil {
						t.Fatal(err)
					}
				} else {
					raw = append(raw, []byte("// changed selected source\n")...)
				}
				if err := os.WriteFile(source, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if r := mainRelocationCall(f, "apply", approval); r.OK {
				t.Fatal("changed source/approval accepted", scenario, r)
			}
			if _, err := os.Lstat(opts["toolDataRoot"].(string)); !os.IsNotExist(err) {
				t.Fatal("stale approval created destination before refusal", err)
			}
		})
	}
}

func TestMainToolAuthoringRelocationUnknownReferences(t *testing.T) {
	for _, scenario := range []string{"corrupt", "link", "overflow"} {
		t.Run(scenario, func(t *testing.T) {
			f := mainRelocationSetup(t)
			registry := filepath.Join(f.home, "local", "bindings", "tool-storage", "uses", "go")
			if _, err := os.Stat(registry); err != nil {
				t.Fatal("lifecycle registry absent", err)
			}
			if scenario == "link" {
				target := filepath.Join(f.base, "foreign-use-proof")
				if err := os.WriteFile(target, []byte("foreign unknown proof"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, filepath.Join(registry, "unknown.json")); err != nil {
					t.Skip("native symlink privilege unavailable")
				}
			} else {
				count := 1
				if scenario == "overflow" {
					count = 65
				}
				for i := 0; i < count; i++ {
					if err := os.WriteFile(filepath.Join(registry, fmt.Sprintf("unknown-%02d.json", i)), []byte("{unattributable"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			before := mainRelocationSnapshot(t, f.data)
			opts := mainRelocationOptions(t, f)
			r := mainRelocationCall(f, "prepare", opts)
			if r.OK {
				t.Fatal("unknown native use was interpreted as idle", scenario, r)
			}
			if _, err := os.Lstat(opts["toolDataRoot"].(string)); !os.IsNotExist(err) {
				t.Fatal("unknown use caused destination effects", err)
			}
			if !reflect.DeepEqual(before, mainRelocationSnapshot(t, f.data)) {
				t.Fatal("unknown use changed retained source")
			}
		})
	}
}

func TestMainToolAuthoringRelocationStoppedInspection(t *testing.T) {
	f := mainRelocationSetup(t)
	c, code := resolveLayout(f.home, "installation")
	if code != "" {
		t.Fatal(code)
	}
	db, err := openLayout(c)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("UPDATE project SET stopped=1")
	closeErr := db.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	opts := mainRelocationOptions(t, f)
	r := mainRelocationCall(f, "prepare", opts)
	if r.OK || r.Code != "TOOL_STORAGE_STOPPED" {
		t.Fatal("STOP admitted transition", r)
	}
	if _, err := os.Lstat(opts["toolDataRoot"].(string)); !os.IsNotExist(err) {
		t.Fatal("STOP had destination effects", err)
	}
	r = mainRelocationCall(f, "inspect", map[string]any{"toolId": "go"})
	if !r.OK {
		t.Fatal("STOP blocked bounded read-only diagnosis", r)
	}
}

func TestMainToolAuthoringRelocationCrashChild(t *testing.T) {
	home := os.Getenv("OCMI_RELOCATE_CRASH_HOME")
	if home == "" {
		t.Skip("physical relocation crash helper only")
	}
	var approval map[string]any
	if json.Unmarshal([]byte(os.Getenv("OCMI_RELOCATE_CRASH_APPROVAL")), &approval) != nil {
		t.Fatal("fixture approval")
	}
	phase := os.Getenv("OCMI_RELOCATE_CRASH_PHASE")
	toolStorageAuthoringTestPhase = func(observed string) {
		if observed == phase {
			os.Exit(89)
		}
	}
	r := mainRelocationCall(mainRelocationFixture{mainAuthorFixture: mainAuthorFixture{home: home}}, "apply", approval)
	t.Fatal("durable relocation crash boundary not reached", phase, r)
}

func TestMainToolAuthoringRelocationCrashRecovery(t *testing.T) {
	for _, phase := range []string{"relocate.intent", "relocate.copied", "relocate.staged", "relocate.active"} {
		t.Run(phase, func(t *testing.T) {
			f := mainRelocationSetup(t)
			opts := mainRelocationOptions(t, f)
			boundRoot, _ := mainRelocationProject(t, f, "pending-consumer-control", f.packageDir)
			controlCommand, _ := json.Marshal(map[string]any{"executable": f.goExe, "toolStorage": mainRelocationRef(t, f)})
			if _, err := toolStorageFromCommand(boundRoot, controlCommand); err != nil {
				t.Fatal("positive bound native profile premise", err)
			}
			p := mainRelocationPrepare(t, f, opts)
			approval := mainRelocationApproval(p)
			before := mainRelocationSnapshot(t, f.data)
			profile := filepath.Join(f.home, "local", "bindings", "tool-storage", "go.json")
			profileID := mainToolID(t, profile)
			_, profileBytes := mainAuthorProfile(t, f.mainAuthorFixture, "go")
			raw, _ := json.Marshal(approval)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMainToolAuthoringRelocationCrashChild$")
			child.Env = append(os.Environ(), "OCMI_RELOCATE_CRASH_HOME="+f.home,
				"OCMI_RELOCATE_CRASH_PHASE="+phase, "OCMI_RELOCATE_CRASH_APPROVAL="+string(raw))
			out, err := child.CombinedOutput()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 89 {
				t.Fatalf("physical relocation interruption missing: %v %.600s", err, out)
			}
			if !reflect.DeepEqual(before, mainRelocationSnapshot(t, f.data)) {
				t.Fatal("interruption changed original source/foreign sibling")
			}
			if phase != "relocate.active" {
				_, after := mainAuthorProfile(t, f.mainAuthorFixture, "go")
				if string(after) != string(profileBytes) || mainToolID(t, profile) != profileID {
					t.Fatal("pre-publication crash replaced usable old authority")
				}
			}
			lock := filepath.Join(f.home, "local", "bindings", ".setup-lock")
			lockID := mainToolID(t, lock)
			wrong := mainRelocationApproval(p)
			wrong["planSha256"] = strings.Repeat("0", 64)
			previousProbe := environmentRunOverride
			replayedProbes := 0
			environmentRunOverride = func(context.Context, commandSpec) (commandResult, error) {
				replayedProbes++
				return commandResult{}, fmt.Errorf("unexpected recovery provider invocation")
			}
			t.Cleanup(func() { environmentRunOverride = previousProbe })
			if r := mainRelocationCall(f, "recover", wrong); r.OK {
				t.Fatal("wrong generation recovered", r)
			}
			if mainToolID(t, lock) != lockID {
				t.Fatal("wrong recovery changed admission")
			}
			// A durable pending transition must not authorize an ordinary launch.
			ref := mainRelocationRef(t, f)
			command := map[string]any{"executable": f.goExe, "toolStorage": ref}
			commandRaw, _ := json.Marshal(command)
			if _, err := toolStorageFromCommand(boundRoot, commandRaw); err == nil {
				t.Fatal("pending transition admitted bound native profile")
			}
			r := mainRelocationCall(f, "recover", approval)
			if replayedProbes != 0 {
				t.Fatal("recovery attempted an unproved provider replay", replayedProbes)
			}
			if phase == "relocate.active" {
				if !r.OK || r.Data["phase"] != "active" {
					t.Fatal("fully proven publication failed metadata-only closure", r)
				}
				newPackage := filepath.Join(opts["toolDataRoot"].(string), "go", "workspace", "src", "selected-module")
				root, ready := mainRelocationReadOnlyProject(t, f, "after-crash-closure")
				mainRelocationRun(t, root, ready, newPackage)
			} else if r.OK && r.Data["phase"] == "active" {
				t.Fatal("unproved interrupted probes/publication silently replayed", r)
			}
			if !reflect.DeepEqual(before, mainRelocationSnapshot(t, f.data)) {
				t.Fatal("recovery changed original selected source")
			}
			if phase != "relocate.intent" {
				newTool := filepath.Join(opts["toolDataRoot"].(string), "go")
				if !reflect.DeepEqual(mainRelocationSnapshot(t, filepath.Join(f.data, "go")), mainRelocationSnapshot(t, newTool)) {
					t.Fatal("recovery discarded or altered preserved copied bytes")
				}
			}
		})
	}
}

func TestMainToolAuthoringRelocationDivergence(t *testing.T) {
	f := mainRelocationSetup(t)
	opts := mainRelocationOptions(t, f)
	p := mainRelocationPrepare(t, f, opts)
	if r := mainRelocationCall(f, "apply", mainRelocationApproval(p)); !r.OK {
		t.Fatal(r)
	}
	target := filepath.Join(opts["toolDataRoot"].(string), "go", "workspace", "src", "selected-module", "retained.go")
	newWork := []byte("package retained\nconst NewWork=\"preserve new unique bytes\"\n")
	if err := os.WriteFile(target, newWork, 0600); err != nil {
		t.Fatal(err)
	}
	if r := mainRelocationCall(f, "rollback", mainRelocationApproval(p)); r.OK {
		t.Fatal("rollback hid divergent unique work", r)
	}
	if raw, err := os.ReadFile(target); err != nil || string(raw) != string(newWork) {
		t.Fatal("rollback lost new work", err)
	}
	if _, err := os.Stat(filepath.Join(f.packageDir, "retained.go")); err != nil {
		t.Fatal("rollback deleted original source", err)
	}
}

// Alter only reviewed fictional public build-info and its authoritative checksum.
func mainRelocationFactoryFormat(t *testing.T, home string, value any) {
	t.Helper()
	path := filepath.Join(home, "system", "build-info.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var info map[string]any
	if json.Unmarshal(raw, &info) != nil {
		t.Fatal("fixture build-info")
	}
	if value == nil {
		delete(info, "toolStorageProfileFormat")
	} else {
		info["toolStorageProfileFormat"] = value
	}
	raw, err = json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	sumsPath := filepath.Join(home, "SHA256SUMS")
	sums, err := os.ReadFile(sumsPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(sums), "\n")
	found := 0
	for i, line := range lines {
		if strings.HasSuffix(line, "  system/build-info.json") {
			lines[i] = mainToolHash(raw) + "  system/build-info.json"
			found++
		}
	}
	if found != 1 {
		t.Fatal("fixture inventory must have one build-info")
	}
	if err := os.WriteFile(sumsPath, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := verifyFactoryHome(home); err != nil {
		t.Fatal("resealed fictional factory", err)
	}
}

func TestMainToolAuthoringRelocationUpdateChild(t *testing.T) {
	home := os.Getenv("OCMI_RELOCATE_UPDATE_HOME")
	if home == "" {
		t.Skip("candidate-entry update helper only")
	}
	r := UpdateHome(os.Getenv("OCMI_RELOCATE_UPDATE_ACTION"), home, os.Getenv("OCMI_RELOCATE_UPDATE_CANDIDATE"),
		os.Getenv("OCMI_RELOCATE_UPDATE_SOURCE"), os.Getenv("OCMI_RELOCATE_UPDATE_NEXT"))
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("RELOCATION_UPDATE_RESULT=" + string(raw))
}

func mainRelocationUpdate(t *testing.T, entry, action, home, candidate, source, next string) Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, entry, "-test.run=^TestMainToolAuthoringRelocationUpdateChild$", "-test.v")
	cmd.Env = append(os.Environ(), "OCMI_RELOCATE_UPDATE_HOME="+home, "OCMI_RELOCATE_UPDATE_ACTION="+action,
		"OCMI_RELOCATE_UPDATE_CANDIDATE="+candidate, "OCMI_RELOCATE_UPDATE_SOURCE="+source, "OCMI_RELOCATE_UPDATE_NEXT="+next)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("candidate entry fixture: %v %.800s", err, out)
	}
	var result Response
	found := 0
	for _, line := range strings.Split(string(out), "\n") {
		if raw, ok := strings.CutPrefix(strings.TrimSpace(line), "RELOCATION_UPDATE_RESULT="); ok {
			if json.Unmarshal([]byte(raw), &result) != nil {
				t.Fatal("candidate fixture response")
			}
			found++
		}
	}
	if found != 1 {
		t.Fatalf("candidate fixture produced %d responses: %.800s", found, out)
	}
	return result
}

func TestMainToolAuthoringRelocationReaderCompatibility(t *testing.T) {
	for _, scenario := range []string{"absent", "invalid", "compatible", "reverse-without-reader"} {
		t.Run(scenario, func(t *testing.T) {
			f := mainRelocationSetup(t)
			if scenario == "compatible" {
				mainRelocationFactoryFormat(t, f.home, 2)
			}
			candidate := filepath.Join(f.base, "candidate")
			entry := acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
			switch scenario {
			case "invalid":
				mainRelocationFactoryFormat(t, candidate, "2")
			case "compatible", "reverse-without-reader":
				mainRelocationFactoryFormat(t, candidate, 2)
			}
			before := acceptancePublicSnapshot(t, f.home)
			profile := filepath.Join(f.home, "local", "bindings", "tool-storage", "go.json")
			profileID := mainToolID(t, profile)
			_, profileBytes := mainAuthorProfile(t, f.mainAuthorFixture, "go")
			r := mainRelocationUpdate(t, entry, "apply", f.home, candidate, strings.Repeat("a", 40), strings.Repeat("b", 40))
			if scenario == "absent" || scenario == "invalid" {
				if r.OK || r.Data["reason"] != "tool_storage_reader_capability" {
					t.Fatal("unsupported reader selected", scenario, r)
				}
				if !reflect.DeepEqual(before, acceptancePublicSnapshot(t, f.home)) {
					t.Fatal("reader refusal changed public generation")
				}
				for _, name := range []string{"update.json", ".setup-lock"} {
					if _, err := os.Lstat(filepath.Join(f.home, "local", "bindings", name)); !os.IsNotExist(err) {
						t.Fatal("reader refusal had admission effects", name, err)
					}
				}
				return
			}
			if !r.OK {
				t.Fatal("compatible reader update", r)
			}
			if mainToolID(t, profile) != profileID {
				t.Fatal("update replaced native profile authority")
			}
			_, after := mainAuthorProfile(t, f.mainAuthorFixture, "go")
			if string(after) != string(profileBytes) {
				t.Fatal("update rewrote schema2 binding")
			}
			newPublic := acceptancePublicSnapshot(t, f.home)
			r = mainRelocationUpdate(t, entry, "rollback", f.home, "", strings.Repeat("b", 40), strings.Repeat("a", 40))
			if scenario == "reverse-without-reader" {
				if r.OK || r.Data["reason"] != "tool_storage_reader_capability" {
					t.Fatal("rollback selected unsupported reader", r)
				}
				if !reflect.DeepEqual(newPublic, acceptancePublicSnapshot(t, f.home)) {
					t.Fatal("rollback reader refusal changed active system")
				}
			} else if !r.OK {
				t.Fatal("compatible reader reverse transition", r)
			}
		})
	}
}

func TestMainToolAuthoringRelocationUpdateAdapterControl(t *testing.T) {
	f := mainAuthorSetup(t)
	candidate := filepath.Join(f.base, "candidate-control")
	entry := acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
	mainRelocationFactoryFormat(t, candidate, 2)
	before := acceptancePublicSnapshot(t, f.home)
	r := mainRelocationUpdate(t, entry, "apply", f.home, candidate, strings.Repeat("a", 40), strings.Repeat("b", 40))
	if !r.OK {
		t.Fatal("actual candidate-entry control apply", r)
	}
	r = mainRelocationUpdate(t, entry, "rollback", f.home, "", strings.Repeat("b", 40), strings.Repeat("a", 40))
	if !r.OK || !reflect.DeepEqual(before, acceptancePublicSnapshot(t, f.home)) {
		t.Fatal("actual candidate-entry control reverse", r)
	}
}
