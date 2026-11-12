package internal

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMainToolAuthoringRelocationDirectoryPremise(t *testing.T) {
	root := t.TempDir()
	selected := filepath.Join(root, "selected-empty")
	retained := filepath.Join(root, "retained-original")
	if err := os.Mkdir(selected, 0700); err != nil {
		t.Fatal(err)
	}
	old := mainToolID(t, selected)
	if err := os.Rename(selected, retained); err != nil {
		t.Fatal(err)
	}
	if mainToolID(t, retained) != old {
		t.Fatal("directory rename changed native identity")
	}
	if err := os.Mkdir(selected, 0700); err != nil {
		t.Fatal(err)
	}
	if mainToolID(t, selected) == old {
		t.Fatal("empty directory replacement control did not change native identity")
	}
}

func TestMainToolAuthoringRelocationEmptyDirectories(t *testing.T) {
	f := mainRelocationSetup(t)
	rel := filepath.Join("workspace", "src", "retained-empty", "offline")
	old := filepath.Join(f.data, "go", rel)
	if err := os.MkdirAll(old, 0700); err != nil {
		t.Fatal(err)
	}
	id := mainToolID(t, old)
	opts := mainRelocationOptions(t, f)
	p := mainRelocationPrepare(t, f, opts)
	if r := mainRelocationCall(f, "apply", mainRelocationApproval(p)); !r.OK {
		t.Fatal(r)
	}
	newPath := filepath.Join(opts["toolDataRoot"].(string), "go", rel)
	info, err := os.Lstat(newPath)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("preserving copy lost plain empty directory", err)
	}
	if mainToolID(t, old) != id || mainToolID(t, newPath) == id {
		t.Fatal("copy altered/adopted original directory identity")
	}
}

func TestMainToolAuthoringRelocationDirectoryDrift(t *testing.T) {
	f := mainRelocationSetup(t)
	path := filepath.Join(f.packageDir, "empty-before-plan")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	id := mainToolID(t, path)
	opts := mainRelocationOptions(t, f)
	p := mainRelocationPrepare(t, f, opts)
	retained := filepath.Join(f.base, "retained-original-empty")
	if err := os.Rename(path, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if mainToolID(t, path) == id {
		t.Fatal("qualified directory replacement premise failed")
	}
	if r := mainRelocationCall(f, "apply", mainRelocationApproval(p)); r.OK {
		t.Fatal("same-empty directory replacement silently adopted", r)
	}
	if _, err := os.Lstat(opts["toolDataRoot"].(string)); !os.IsNotExist(err) {
		t.Fatal("stale source directory caused destination effects", err)
	}
	if mainToolID(t, retained) != id {
		t.Fatal("refused transition changed retained original")
	}
}

// Go list may populate the selected build cache. This never grants permission
// to discard changed target data; unchanged-target positive reverse uses the
// independently qualified read-only native child instead.
func TestMainToolAuthoringRelocationNativeGoUsePolicy(t *testing.T) {
	f := mainRelocationSetup(t)
	opts := mainRelocationOptions(t, f)
	p := mainRelocationPrepare(t, f, opts)
	if r := mainRelocationCall(f, "apply", mainRelocationApproval(p)); !r.OK {
		t.Fatal(r)
	}
	target := filepath.Join(opts["toolDataRoot"].(string), "go")
	before := mainRelocationSnapshot(t, target)
	old := mainRelocationSnapshot(t, f.data)
	newPackage := filepath.Join(target, "workspace", "src", "selected-module")
	root, ready := mainRelocationProject(t, f, "native-go-use", newPackage)
	mainRelocationRun(t, root, ready, newPackage)
	after := mainRelocationSnapshot(t, target)
	r := mainRelocationCall(f, "rollback", mainRelocationApproval(p))
	if !reflect.DeepEqual(before, after) {
		if r.OK {
			t.Fatal("native Go cache changes were discarded/hidden by rollback", r)
		}
		if !reflect.DeepEqual(after, mainRelocationSnapshot(t, target)) {
			t.Fatal("refused rollback changed target")
		}
		t.Log("native Go changed target; strict rollback correctly refused")
	} else if !r.OK {
		t.Fatal("unchanged native Go target could not reverse", r)
	}
	if !reflect.DeepEqual(old, mainRelocationSnapshot(t, f.data)) {
		t.Fatal("native Go use/reverse changed retained old source")
	}
}

func TestMainToolAuthoringRelocationReadOnlyConsumerPremise(t *testing.T) {
	f := mainRelocationSetup(t)
	before := mainRelocationSnapshot(t, f.data)
	root, ready := mainRelocationReadOnlyProject(t, f, "native-read-only-control")
	mainRelocationRun(t, root, ready, f.packageDir)
	if !reflect.DeepEqual(before, mainRelocationSnapshot(t, f.data)) {
		t.Fatal("read-only native child changed selected stores")
	}
}
