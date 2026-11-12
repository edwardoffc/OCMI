package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// Main-owned independent native crash oracle. Fixture manifests are fictional;
// full source/notices/two-real-revision qualification belongs to the ReadyKit consumer.
func acceptanceUpdateFactory(t *testing.T, root, source string) string {
	t.Helper()
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	var declaration any
	if err := json.Unmarshal(installationLayoutJSON, &declaration); err != nil {
		t.Fatal(err)
	}
	marker, _ := json.Marshal(declaration)
	binary := "system/bin/ocmi"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	image, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	text := map[string]any{"schemaVersion": 1, "rootReadme": "# Fictional OCMI\n", "rootAgent": "Read selected guide.\n",
		"syncReadme": "Fictional selected semantic bytes.\n", "coordinatorAgent": "<GUIDE_RELATIVE>\n",
		"coordinatorIndex": "HANDOFF.md\n", "coordinatorHandoff": "Fictional continuation.\n",
		"startup": "<HOME> <MEMORY> <GUIDE> <PROJECT> <SOURCE> <LANGUAGE>\n"}
	textRaw, _ := json.Marshal(text)
	info, _ := json.Marshal(map[string]any{"distributionKind": "ready-kit", "systemVersion": "1.3.5", "sourceCommit": source,
		"sourceDirty": false, "layoutVersion": 1, "layoutDeclaration": ".ocmi-home.json", "binaryPath": binary,
		"target": runtime.GOOS + "-" + runtime.GOARCH, "updateCompatibility": map[string]int{"schemaVersion": 1,
			"layoutVersion": 1, "installationReceiptVersion": 1, "coordinatorMemoryVersion": 1, "coordinatorBindingVersion": 1, "stateSchemaVersion": 4}})
	files := map[string][]byte{".ocmi-home.json": marker, "README.md": []byte("# Fictional root\n"),
		"README.ru.md": []byte("# Fictional RU root\n"), "AGENTS.md": []byte("Selected fictional route.\n"),
		".gitignore": []byte("/OCMI_Sync/\n/local/\n"), ".gitattributes": []byte("/local export-ignore\n"),
		"system/VERSION.md":      []byte("# OCMI 1.3.5\n\nFictional update premise.\n"),
		"system/build-info.json": info, "system/packaging/home/text.json": textRaw, binary: image}
	names := make([]string, 0, len(files))
	for name, raw := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0700); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	var sums strings.Builder
	for _, name := range names {
		h := sha256.Sum256(files[name])
		fmt.Fprintf(&sums, "%x  %s\n", h, name)
	}
	if err := os.WriteFile(filepath.Join(root, "SHA256SUMS"), []byte(sums.String()), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := verifyFactoryHome(root); err != nil {
		t.Fatalf("independent factory invalid: %v", err)
	}
	return filepath.Join(root, filepath.FromSlash(binary))
}

func acceptancePublicSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			t.Fatal("fixture sums")
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(parts[1])))
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(content)
		out[parts[1]] = hex.EncodeToString(h[:])
	}
	h := sha256.Sum256(raw)
	out["SHA256SUMS"] = hex.EncodeToString(h[:])
	return out
}

func TestMainUpdateCrashChild(t *testing.T) {
	home := os.Getenv("OCMI_ACCEPT_UPDATE_HOME")
	if home == "" {
		t.Skip("subprocess-only actual crash helper")
	}
	phase := os.Getenv("OCMI_ACCEPT_UPDATE_PHASE")
	updateTestPhase = func(stage string) {
		if stage == phase {
			if stage == "old_system" {
				if _, err := os.Lstat(filepath.Join(home, "system")); !os.IsNotExist(err) {
					t.Fatal("old_system hook did not follow the actual old-system move")
				}
			}
			if stage == "new_system" {
				if _, _, err := verifyFactoryHome(home); err == nil {
					t.Fatal("new_system hook skipped the actual mixed system/root-inventory window")
				}
			}
			os.Exit(89)
		}
	}
	action := os.Getenv("OCMI_ACCEPT_UPDATE_ACTION")
	if action == "" {
		action = "apply"
	}
	if action != "apply" && action != "rollback" {
		t.Fatal("unsupported fixture action")
	}
	r := UpdateHome(action, home, os.Getenv("OCMI_ACCEPT_UPDATE_CANDIDATE"), strings.Repeat("a", 40), strings.Repeat("b", 40))
	t.Fatalf("declared durable phase %q was not reached: %+v", phase, r)
}

func TestMainUpdateCrashMatrix(t *testing.T) {
	for _, phase := range []string{"intent", "staged", "old_system", "new_system", "roots", "metadata"} {
		t.Run(phase, func(t *testing.T) {
			base := t.TempDir()
			home := filepath.Join(base, "installed home Ω")
			candidate := filepath.Join(base, "new candidate")
			acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
			entry := acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
			before := acceptancePublicSnapshot(t, home)
			if r := InstallHome(home, false); !r.OK {
				t.Fatalf("actual init: %+v", r)
			}
			ctx, code := resolveLayout(home, "installation")
			if code != "" {
				t.Fatal(code)
			}
			receipt, err := readInstallationReceiptContext(ctx)
			if err != nil {
				t.Fatal(err)
			}
			canary := filepath.Join(home, "OCMI_Sync", "personal", "topic.txt")
			if err := os.MkdirAll(filepath.Dir(canary), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(canary, []byte("Initial private fictional WIP\n"), 0600); err != nil {
				t.Fatal(err)
			}
			db, err := openLayout(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec("UPDATE project SET stopped=1"); err != nil {
				t.Fatal(err)
			}
			db.Close()
			child := exec.Command(entry, "-test.run=^TestMainUpdateCrashChild$")
			child.Env = append(os.Environ(), "OCMI_ACCEPT_UPDATE_HOME="+home, "OCMI_ACCEPT_UPDATE_CANDIDATE="+candidate, "OCMI_ACCEPT_UPDATE_PHASE="+phase)
			output, err := child.CombinedOutput()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 89 {
				t.Fatalf("actual phase crash not proved: %v %s", err, output)
			}
			pending := UpdateHome("inspect", home, "", "", "")
			if pending.OK || pending.Data["phase"] != "pending" {
				t.Fatalf("mixed/interrupted home appeared ready: %+v", pending)
			}
			mutation := Handle(Request{SchemaVersion: 1, Action: "stop", ProjectRoot: home, RootKind: "installation", Actor: "coordinator", RequestID: "must-not-clear-stop", Payload: json.RawMessage(`{"enabled":false}`)})
			if mutation.OK {
				t.Fatalf("pending home admitted mutation: %+v", mutation)
			}
			later := []byte("Later private fictional WIP after process death\n")
			if err := os.WriteFile(canary, later, 0600); err != nil {
				t.Fatal(err)
			}
			r := UpdateHome("recover", home, "", "", "")
			if !r.OK || r.Data["phase"] != "rolled_back" {
				t.Fatalf("owned crash recovery failed: %+v", r)
			}
			current, err := readInstallationReceiptContext(ctx)
			if err != nil || !reflect.DeepEqual(current, receipt) {
				t.Fatalf("receipt/native identity changed: %v %+v", err, current)
			}
			after := acceptancePublicSnapshot(t, home)
			if !reflect.DeepEqual(after, before) {
				t.Fatal("exact old public payload not restored")
			}
			raw, err := os.ReadFile(canary)
			if err != nil || string(raw) != string(later) {
				t.Fatal("recovery erased later private WIP")
			}
			db, err = openLayout(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var stopped int
			err = db.QueryRow("SELECT stopped FROM project").Scan(&stopped)
			db.Close()
			if err != nil || stopped != 1 {
				t.Fatal("recovery rolled back STOP/core data")
			}
			if r := InstallHome(home, true); !r.OK {
				t.Fatalf("recovered native entry not coherent: %+v", r)
			}
		})
	}
}

func TestMainUpdateCommittedAdmissionRecovery(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		t.Run(fmt.Sprintf("foreign=%t", foreign), func(t *testing.T) {
			base := t.TempDir()
			home, candidate := filepath.Join(base, "home"), filepath.Join(base, "candidate")
			acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
			entry := acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
			oldPublic := acceptancePublicSnapshot(t, home)
			if r := InstallHome(home, false); !r.OK {
				t.Fatal(r)
			}
			crash := func(action, phase string) {
				t.Helper()
				child := exec.Command(entry, "-test.run=^TestMainUpdateCrashChild$")
				child.Env = append(os.Environ(), "OCMI_ACCEPT_UPDATE_HOME="+home, "OCMI_ACCEPT_UPDATE_CANDIDATE="+candidate, "OCMI_ACCEPT_UPDATE_PHASE="+phase, "OCMI_ACCEPT_UPDATE_ACTION="+action)
				out, err := child.CombinedOutput()
				if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 89 {
					t.Fatalf("committed crash %s/%s: %v %s", action, phase, err, out)
				}
			}
			lockPath := filepath.Join(home, "local", "bindings", ".setup-lock")
			journalPath := filepath.Join(home, "local", "bindings", "update.json")
			assertRecordedLock := func(wantPhase, wantCleanup string) {
				t.Helper()
				raw, err := os.ReadFile(journalPath)
				if err != nil {
					t.Fatal(err)
				}
				var j struct {
					Phase       string `json:"phase"`
					Cleanup     string `json:"cleanupState"`
					AdmissionID string `json:"admissionId"`
				}
				if err := json.Unmarshal(raw, &j); err != nil || j.Phase != wantPhase || j.Cleanup != wantCleanup {
					t.Fatalf("terminal boundary not durable: %v %+v", err, j)
				}
				id, err := localNativeID(lockPath)
				if err != nil || id != j.AdmissionID {
					t.Fatalf("crash did not preserve recorded admission: %v", err)
				}
			}
			crash("apply", "applied")
			assertRecordedLock("applied", "")
			applied := acceptancePublicSnapshot(t, home)
			if reflect.DeepEqual(applied, oldPublic) {
				t.Fatal("applied boundary left old payload")
			}
			if foreign {
				if err := os.Rename(lockPath, filepath.Join(base, "original-admission")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(lockPath, 0700); err != nil {
					t.Fatal(err)
				}
				id, err := localNativeID(lockPath)
				if err != nil {
					t.Fatal(err)
				}
				raw, _ := os.ReadFile(journalPath)
				if r := UpdateHome("recover", home, "", "", ""); r.OK {
					t.Fatalf("foreign terminal admission appeared recovered: %+v", r)
				}
				afterID, err := localNativeID(lockPath)
				afterRaw, _ := os.ReadFile(journalPath)
				if err != nil || afterID != id || !reflect.DeepEqual(raw, afterRaw) || !reflect.DeepEqual(applied, acceptancePublicSnapshot(t, home)) {
					t.Fatal("foreign terminal admission refusal changed state")
				}
				return
			}
			for _, boundary := range []string{"applied", "rolled_back"} {
				if boundary == "rolled_back" {
					crash("rollback", "cleanup_done")
					assertRecordedLock("rolled_back", "done")
				}
				before := acceptancePublicSnapshot(t, home)
				r := UpdateHome("recover", home, "", "", "")
				if !r.OK || r.Data["phase"] != boundary {
					t.Fatalf("terminal recovery: %+v", r)
				}
				if _, err := os.Lstat(lockPath); !os.IsNotExist(err) {
					t.Fatal("terminal recovery left its own stale setup admission")
				}
				if !reflect.DeepEqual(before, acceptancePublicSnapshot(t, home)) {
					t.Fatal("terminal recovery rewrote committed payload")
				}
				if boundary == "rolled_back" && !reflect.DeepEqual(before, oldPublic) {
					t.Fatal("rollback boundary did not restore old payload")
				}
			}
		})
	}
}

func TestMainUpdateFixtureControl(t *testing.T) {
	root := filepath.Join(t.TempDir(), "known control")
	acceptanceUpdateFactory(t, root, strings.Repeat("a", 40))
	before := acceptancePublicSnapshot(t, root)
	if r := InstallHome(root, false); !r.OK {
		t.Fatalf("independent native fixture: %+v", r)
	}
	if !reflect.DeepEqual(before, acceptancePublicSnapshot(t, root)) {
		t.Fatal("init changed public fixture")
	}
	ctx, code := resolveLayout(root, "installation")
	if code != "" {
		t.Fatal(code)
	}
	db, err := openLayout(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE project SET stopped=1"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	clear := Handle(Request{SchemaVersion: 1, Action: "stop", ProjectRoot: root, RootKind: "installation", Actor: "coordinator", RequestID: "actual-control-clear-stop", Payload: json.RawMessage(`{"enabled":false}`)})
	if !clear.OK {
		t.Fatalf("pending mutation oracle has no valid control: %+v", clear)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("Wrong public replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(before, acceptancePublicSnapshot(t, root)) {
		t.Fatal("independent public oracle accepted targeted wrong bytes")
	}
}

func TestMainUpdateUnknownStageRefusal(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	candidate := filepath.Join(base, "candidate")
	acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
	entry := acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
	if r := InstallHome(home, false); !r.OK {
		t.Fatal(r)
	}
	child := exec.Command(entry, "-test.run=^TestMainUpdateCrashChild$")
	child.Env = append(os.Environ(), "OCMI_ACCEPT_UPDATE_HOME="+home, "OCMI_ACCEPT_UPDATE_CANDIDATE="+candidate, "OCMI_ACCEPT_UPDATE_PHASE=staged")
	output, err := child.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 89 {
		t.Fatalf("actual staged crash: %v %s", err, output)
	}
	entries, err := os.ReadDir(filepath.Join(home, "local", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	var stage string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "update-") {
			if stage != "" {
				t.Fatal("ambiguous update stages")
			}
			stage = filepath.Join(home, "local", "runs", e.Name())
		}
	}
	if stage == "" {
		t.Fatal("no attributable staged update")
	}
	foreign := filepath.Join(stage, "unclassified.txt")
	if err := os.WriteFile(foreign, []byte("Unknown fictional evidence; do not delete\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before := acceptancePublicSnapshot(t, home)
	r := UpdateHome("recover", home, "", "", "")
	if r.OK {
		t.Fatalf("unknown stage contents acquired deletion authority: %+v", r)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatal("recovery removed unknown file")
	}
	if !reflect.DeepEqual(before, acceptancePublicSnapshot(t, home)) {
		t.Fatal("unsafe stage recovery altered public payload")
	}
}

func TestMainUpdateHeldWriterChild(t *testing.T) {
	home := os.Getenv("OCMI_ACCEPT_UPDATE_HELD_HOME")
	if home == "" {
		t.Skip("subprocess-only actual writer admission control")
	}
	gate := os.Getenv("OCMI_ACCEPT_UPDATE_GATE")
	updateTestPhase = func(phase string) {
		if phase != "staged" {
			return
		}
		if err := os.WriteFile(filepath.Join(gate, "entered"), []byte("staged"), 0600); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(filepath.Join(gate, "release")); err == nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("bounded held-writer fixture deadline")
	}
	r := UpdateHome("apply", home, os.Getenv("OCMI_ACCEPT_UPDATE_CANDIDATE"), strings.Repeat("a", 40), strings.Repeat("b", 40))
	if !r.OK {
		t.Fatalf("actual first writer: %+v", r)
	}
}

func TestMainUpdateHoldsWriterAndRefusesLiveRecovery(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	candidate := filepath.Join(base, "candidate")
	acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
	entry := acceptanceUpdateFactory(t, candidate, strings.Repeat("b", 40))
	if r := InstallHome(home, false); !r.OK {
		t.Fatal(r)
	}
	child := exec.Command(entry, "-test.run=^TestMainUpdateHeldWriterChild$")
	child.Env = append(os.Environ(), "OCMI_ACCEPT_UPDATE_HELD_HOME="+home, "OCMI_ACCEPT_UPDATE_CANDIDATE="+candidate, "OCMI_ACCEPT_UPDATE_GATE="+base)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	finished := false
	defer func() {
		if !finished {
			_ = child.Process.Kill()
			<-done
		}
	}()
	deadline := time.Now().Add(10 * time.Second)
	entered := false
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(base, "entered")); err == nil {
			entered = true
			break
		}
		select {
		case err := <-done:
			finished = true
			t.Fatalf("first writer exited before real stage: %v", err)
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !entered {
		t.Fatal("actual updater never reached bounded held stage")
	}
	ctx, code := resolveLayout(home, "installation")
	if code != "" {
		t.Fatal(code)
	}
	db, err := openLayout(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("PRAGMA busy_timeout=50")
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	_, blocked := tx.Exec("UPDATE project SET revision=revision+1")
	tx.Rollback()
	db.Close()
	if blocked == nil || (!strings.Contains(strings.ToLower(blocked.Error()), "locked") && !strings.Contains(strings.ToLower(blocked.Error()), "busy")) {
		t.Fatalf("activation has no actual writer barrier: %v", blocked)
	}
	recovery := UpdateHome("recover", home, "", "", "")
	if recovery.OK {
		t.Fatalf("live writer was taken over by recovery: %+v", recovery)
	}
	if err := os.WriteFile(filepath.Join(base, "release"), []byte("release"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		finished = true
		if err != nil {
			t.Fatalf("first writer failed after competing recovery: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("first writer did not finish bounded operation")
	}
	result := UpdateHome("inspect", home, "", "", "")
	if !result.OK || result.Data["phase"] != "applied" {
		t.Fatalf("competing recovery damaged first writer: %+v", result)
	}
}
