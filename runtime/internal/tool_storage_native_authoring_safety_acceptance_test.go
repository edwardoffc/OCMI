package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMainToolAuthoringForeignProfileAppeared(t *testing.T) {
	f := mainAuthorSetup(t)
	plan := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
	profile := filepath.Join(f.home, "local", "bindings", "tool-storage", "widget.json")
	foreign := []byte("{\"foreign\":\"pre-existing unowned bytes\"}")
	if e := os.WriteFile(profile, foreign, 0600); e != nil {
		t.Fatal(e)
	}
	result := mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, plan, "widget"))
	raw, e := os.ReadFile(profile)
	if result.OK || e != nil || string(raw) != string(foreign) {
		t.Fatal("foreign metadata overwritten after approval", result, e)
	}
	if _, e = os.Lstat(f.data); !os.IsNotExist(e) {
		t.Fatal("external effect before metadata absence recheck", e)
	}
}

func TestMainToolAuthoringLiveAdmission(t *testing.T) {
	f := mainAuthorSetup(t)
	plan := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan Response, 1)
	old := toolStorageAuthoringTestPhase
	toolStorageAuthoringTestPhase = func(phase string) {
		if phase != "intent" {
			return
		}
		close(entered)
		select {
		case <-release:
		case <-time.After(20 * time.Second):
		}
	}
	released := false
	defer func() {
		if !released {
			close(release)
		}
		select {
		case <-done:
		case <-time.After(25 * time.Second):
		}
		toolStorageAuthoringTestPhase = old
	}()
	go func() { done <- mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, plan, "widget")) }()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("native setup never reached durable held phase")
	}
	c, _ := resolveLayout(f.home, "installation")
	lock := filepath.Join(f.home, "local", "bindings", ".setup-lock")
	lockID := mainToolID(t, lock)
	db, e := openLayout(c)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("PRAGMA busy_timeout=50"); e != nil {
		db.Close()
		t.Fatal(e)
	}
	tx, e := db.Begin()
	if e != nil {
		db.Close()
		t.Fatal(e)
	}
	_, blocked := tx.Exec("UPDATE project SET revision=revision+1")
	tx.Rollback()
	db.Close()
	if blocked == nil || (!strings.Contains(strings.ToLower(blocked.Error()), "locked") && !strings.Contains(strings.ToLower(blocked.Error()), "busy")) {
		t.Error("setup has no actual SQLite writer admission; STOP/state writer could commit during effect")
	}
	takeover := mainAuthorCall(f, "recover", "coordinator", mainAuthorApproval(t, plan, "widget"))
	if takeover.OK {
		t.Error("recovery stole live setup", takeover)
	}
	if current, e := localNativeID(lock); e != nil || current != lockID {
		t.Error("competing recovery removed/replaced live native admission", e)
	}
	close(release)
	released = true
	select {
	case result := <-done:
		if !result.OK {
			t.Error("first setup damaged by competing recovery", result)
		}
		// Supply the terminal value to the deferred join rather than a second wait.
		done <- result
	case <-time.After(10 * time.Second):
		t.Fatal("first setup did not finish after release")
	}
}
func TestMainToolAuthoringApprovedRootDrift(t *testing.T) {
	f := mainAuthorSetup(t)
	if e := os.Mkdir(f.data, 0700); e != nil {
		t.Fatal(e)
	}
	plan := mainAuthorPrepare(t, f, mainAuthorOptions(f, "widget"))
	if e := os.Rename(f.data, f.data+".saved"); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(f.data, 0700); e != nil {
		t.Fatal(e)
	}
	marker := filepath.Join(f.data, "foreign.txt")
	if e := os.WriteFile(marker, []byte("unknown replacement root"), 0600); e != nil {
		t.Fatal(e)
	}
	result := mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, plan, "widget"))
	if result.OK {
		t.Error("stale approved root identity adopted replacement data")
	}
	if _, e := os.Lstat(filepath.Join(f.data, "widget")); !os.IsNotExist(e) {
		t.Error("external effect in replacement root", e)
	}
	raw, e := os.ReadFile(marker)
	if e != nil || string(raw) != "unknown replacement root" {
		t.Fatal("foreign data changed")
	}
}
func TestMainToolAuthoringExecutableDriftBeforeEffect(t *testing.T) {
	f := mainAuthorSetup(t)
	_, node, _ := mainAuthorNativeTools(t)
	raw, e := os.ReadFile(node)
	if e != nil {
		t.Fatal(e)
	}
	executable := filepath.Join(f.base, "selected-native")
	if strings.HasSuffix(strings.ToLower(node), ".exe") {
		executable += ".exe"
	}
	if e = os.WriteFile(executable, raw, 0700); e != nil {
		t.Fatal(e)
	}
	options := mainAuthorOptions(f, "go")
	options["executable"] = executable
	plan := mainAuthorPrepare(t, f, options)
	if e = os.Rename(executable, executable+".saved"); e != nil {
		t.Fatal(e)
	}
	// Intentionally invalid image: a broken implementation cannot recursively
	// execute Go tests or arbitrary fixture code while demonstrating this defect.
	if e = os.WriteFile(executable, []byte("not an executable image"), 0700); e != nil {
		t.Fatal(e)
	}
	result := mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, plan, "go"))
	if result.OK {
		t.Fatal("changed native image configured")
	}
	if _, e = os.Lstat(f.data); !os.IsNotExist(e) {
		t.Fatal("external directories created before executable drift refusal", e)
	}
}
