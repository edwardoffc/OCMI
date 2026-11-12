package internal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func checkpointRequest(t *testing.T, root, action string, payload any) Response {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return Handle(Request{SchemaVersion: 1, Action: action, ProjectRoot: root, Actor: "main", RequestID: "checkpoint-" + action + "-" + time.Now().UTC().Format("150405.000000000"), Payload: b})
}

func checkpointPayload(path string) map[string]any {
	return map[string]any{"packagePath": path, "checkpointId": "fictional", "systemVersion": "1.3.5", "sourceIdentity": "fixture", "owner": "main", "purpose": "test", "retainUntil": "review", "nextAction": "rebind", "files": []string{"note.txt"}}
}

func TestCheckpointCaptureRejectsSourceDrift(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(t.TempDir(), "checkpoint")
	checkpointCaptureBeforeVerify = func() { _ = os.WriteFile(filepath.Join(root, "note.txt"), []byte("after"), 0600) }
	defer func() { checkpointCaptureBeforeVerify = nil }()
	if got := checkpointRequest(t, root, "checkpoint.capture", checkpointPayload(packagePath)); got.OK {
		t.Fatalf("capture accepted drift: %+v", got)
	}
	if _, err := os.Stat(packagePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("drift left package: %v", err)
	}
}

func TestCheckpointRestoreFailurePreservesEmptyDestination(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(t.TempDir(), "checkpoint")
	if got := checkpointRequest(t, root, "checkpoint.capture", checkpointPayload(packagePath)); !got.OK {
		t.Fatalf("capture=%+v", got)
	}
	destination := filepath.Join(t.TempDir(), "destination")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	checkpointRestoreBeforeActivation = func() error { return errors.New("fixture activation failure") }
	defer func() { checkpointRestoreBeforeActivation = nil }()
	if got := checkpointRequest(t, destination, "checkpoint.restore", map[string]any{"packagePath": packagePath}); got.OK {
		t.Fatalf("restore accepted fault: %+v", got)
	}
	entries, err := os.ReadDir(destination)
	if err != nil || len(entries) != 0 {
		t.Fatalf("destination after rollback entries=%v err=%v", entries, err)
	}
}

func TestCheckpointAcceptsLexicalSelectedRootAliases(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for index, alias := range []string{root + string(os.PathSeparator), filepath.Join(root, ".")} {
		payload := checkpointPayload(filepath.Join(t.TempDir(), "checkpoint-"+string(rune('a'+index))))
		if got := checkpointRequest(t, alias, "checkpoint.capture", payload); !got.OK {
			t.Fatalf("alias %q: %+v", alias, got)
		}
	}
}
