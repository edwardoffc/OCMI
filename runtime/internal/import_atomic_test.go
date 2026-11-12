package internal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func exportedImportPackage(t *testing.T) json.RawMessage {
	t.Helper()
	source, revision := setup(t, "0", 5000, 4096)
	run := request(source, "run", "worker", revision, map[string]any{})
	if !run.OK {
		t.Fatal(run)
	}
	exported := request(source, "handover.export", "main", run.Revision, map[string]any{"transferId": "atomic-import", "files": []string{"contract.md", "input.txt"}})
	if !exported.OK {
		t.Fatal(exported)
	}
	raw, err := json.Marshal(exported.Data["package"])
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func importRequest(root string, raw json.RawMessage) Request {
	return Request{SchemaVersion: 1, Action: "handover.import", ProjectRoot: root, Actor: "main", RequestID: "atomic-import-request", ExpectedRevision: 0, Payload: mustJSON(map[string]any{"package": raw, "deviceId": "atomic-device", "copyId": "atomic-copy"})}
}

func TestPortableImportStagesAtomicallyAndRetries(t *testing.T) {
	raw := exportedImportPackage(t)
	destination := t.TempDir()
	previousFault := importFault
	importFault = func(point string) error {
		if point == "before-activation" {
			return errors.New("controlled staging failure")
		}
		return nil
	}
	defer func() { importFault = previousFault }()
	request := importRequest(destination, raw)
	if failed := Handle(request); failed.OK || failed.Code != "DESTINATION_UNAVAILABLE" {
		t.Fatalf("controlled staging failure=%+v", failed)
	}
	entries, err := os.ReadDir(destination)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failure published destination entries=%v err=%v", entries, err)
	}
	if _, err = os.Stat(storePath(destination)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failure published state err=%v", err)
	}

	importFault = nil
	imported := Handle(request)
	if !imported.OK || imported.Code != "IMPORTED" {
		t.Fatalf("retry=%+v", imported)
	}
	for _, name := range []string{"contract.md", "input.txt"} {
		if _, err := os.Stat(filepath.Join(destination, name)); err != nil {
			t.Fatalf("retry omitted %s: %v", name, err)
		}
	}
	if _, err := os.Stat(storePath(destination)); err != nil {
		t.Fatalf("retry omitted state: %v", err)
	}
	if replay := Handle(request); !replay.OK || replay.Code != "IMPORTED" {
		t.Fatalf("completed replay=%+v", replay)
	}
	rollbackDestination := t.TempDir()
	importFault = func(point string) error {
		if point == "after-empty-root-rename" {
			return errors.New("controlled activation rollback")
		}
		return nil
	}
	if failed := Handle(importRequest(rollbackDestination, raw)); failed.OK || failed.Code != "DESTINATION_UNAVAILABLE" {
		t.Fatalf("activation rollback failure=%+v", failed)
	}
	if entries, err := os.ReadDir(rollbackDestination); err != nil || len(entries) != 0 {
		t.Fatalf("activation rollback changed destination=%v err=%v", entries, err)
	}
	importFault = nil
	if retried := Handle(importRequest(rollbackDestination, raw)); !retried.OK || retried.Code != "IMPORTED" {
		t.Fatalf("activation rollback retry=%+v", retried)
	}

	foreign := t.TempDir()
	foreignFile := filepath.Join(foreign, "foreign.txt")
	if err := os.WriteFile(foreignFile, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if conflict := Handle(importRequest(foreign, raw)); conflict.OK || conflict.Code != "DESTINATION_CONFLICT" {
		t.Fatalf("nonempty destination=%+v", conflict)
	}
	if content, err := os.ReadFile(foreignFile); err != nil || string(content) != "preserve" {
		t.Fatalf("foreign file changed=%q err=%v", content, err)
	}
}

func TestPortableImportRejectsPrivateTraversalAndSymlinkDestination(t *testing.T) {
	raw := exportedImportPackage(t)
	var pkg portablePackage
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatal(err)
	}
	pkg.Files[".ocmi/private.txt"] = portableFile{Content: "eA==", SHA256: "2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881", Size: 1}
	pkg.Digest = packageDigest(pkg)
	privateRaw, _ := json.Marshal(pkg)
	privateRoot := t.TempDir()
	if rejected := Handle(importRequest(privateRoot, privateRaw)); rejected.OK || rejected.Code != "INVALID_PACKAGE_PATH" {
		t.Fatalf("private path=%+v", rejected)
	}
	if entries, err := os.ReadDir(privateRoot); err != nil || len(entries) != 0 {
		t.Fatalf("private rejection wrote destination=%v err=%v", entries, err)
	}

	traversal := pkg
	traversal.Files = map[string]portableFile{"../escape.txt": {Content: "eA==", SHA256: "2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881", Size: 1}}
	traversal.Digest = packageDigest(traversal)
	traversalRaw, _ := json.Marshal(traversal)
	traversalRoot := t.TempDir()
	if rejected := Handle(importRequest(traversalRoot, traversalRaw)); rejected.OK || rejected.Code != "INVALID_PACKAGE_PATH" {
		t.Fatalf("traversal path=%+v", rejected)
	}
	if entries, err := os.ReadDir(traversalRoot); err != nil || len(entries) != 0 {
		t.Fatalf("traversal rejection wrote destination=%v err=%v", entries, err)
	}

}

func TestPortableImportRejectsSymlinkDestination(t *testing.T) {
	raw := exportedImportPackage(t)
	parent := t.TempDir()
	target := t.TempDir()
	linked := filepath.Join(parent, "linked-destination")
	if err := os.Symlink(target, linked); err != nil {
		t.Skipf("host cannot create an owned symlink fixture: %v", err)
	}
	if rejected := Handle(importRequest(linked, raw)); rejected.OK || rejected.Code != "DESTINATION_CONFLICT" {
		t.Fatalf("symlink destination=%+v", rejected)
	}
}
