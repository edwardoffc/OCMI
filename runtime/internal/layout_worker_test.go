package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func workerLayoutRequest(t *testing.T, root, action string, revision int64, payload any) Response {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return Handle(Request{SchemaVersion: 1, Action: action, ProjectRoot: root, Actor: "main", RequestID: "worker-layout-" + action, ExpectedRevision: revision, Payload: raw})
}

func TestWorkerLayoutRejectsStoreLinkIntoSync(t *testing.T) {
	root := t.TempDir()
	declaration, err := strictDeclaration(installationLayoutJSON)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(declaration)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, homeMarker), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(root, "local", "state"), 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "OCMI_Sync", "store.sqlite")
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(target, []byte("foreign state"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(target, filepath.Join(root, "local", "state", "state.sqlite")); err != nil {
		t.Skipf("file link creation unavailable: %v", err)
	}
	layout, code := resolveLayout(root, "installation")
	if code != "" {
		t.Fatal(code)
	}
	if err = confinedStore(layout); err == nil {
		t.Fatal("store link into OCMI_Sync was accepted")
	}
}

func TestWorkerProjectLocalRunsUserDirectoryIsNotAutomaticWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "local", "runs"), 0700); err != nil {
		t.Fatal(err)
	}
	if response := workerLayoutRequest(t, root, "init", 0, map[string]string{"projectId": "project", "deviceId": "device", "copyId": "copy", "owner": "main"}); !response.OK {
		t.Fatal(response)
	}
	intent := workerLayoutRequest(t, root, "resource.intent", 0, map[string]any{"resourceId": "ordinary-directory", "provider": "local", "kind": "directory", "mode": "create", "locator": "local/runs/user-directory", "purpose": "ordinary project data", "lifetime": "task", "disposition": "remove", "ownerRef": "main"})
	if !intent.OK {
		t.Fatal(intent)
	}
	applied := workerLayoutRequest(t, root, "resource.apply", intent.Revision, map[string]any{"resourceId": "ordinary-directory"})
	if !applied.OK {
		t.Fatal(applied)
	}
	observed := workerLayoutRequest(t, root, "resource.inspect", 0, map[string]any{"resourceId": "ordinary-directory"})
	if !observed.OK || observed.Data["observedState"] != "present" || observed.Data["pending"] != false {
		t.Fatal(observed)
	}
}
