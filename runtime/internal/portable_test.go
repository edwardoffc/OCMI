package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPortableSelfContainedImportFreezeRebindAndAck(t *testing.T) {
	source, rev := setup(t, "0", 5000, 4096)
	run := request(source, "run", "worker", rev, map[string]any{})
	if !run.OK {
		t.Fatal(run)
	}
	rev = run.Revision
	exported := request(source, "handover.export", "main", rev, map[string]any{"transferId": "xfer-1", "files": []string{"contract.md", "input.txt"}})
	if !exported.OK || exported.Code != "EXPORTED" {
		t.Fatal(exported)
	}
	pkg, err := json.Marshal(exported.Data["package"])
	if err != nil {
		t.Fatal(err)
	}
	if state := request(source, "status", "main", 0, map[string]any{}); !state.OK || state.Data["stopped"] != true {
		t.Fatalf("source was not frozen: %+v", state)
	}
	if frozen := request(source, "stop", "main", exported.Revision, map[string]any{"enabled": false}); frozen.OK || frozen.Code != "SOURCE_TRANSFERRED" {
		t.Fatalf("exported source reactivated: %+v", frozen)
	}
	destination := t.TempDir()
	imported := request(destination, "handover.import", "main", 0, map[string]any{"package": json.RawMessage(pkg), "deviceId": "dest-device", "copyId": "dest-copy"})
	if !imported.OK || imported.Code != "IMPORTED" {
		t.Fatal(imported)
	}
	for _, name := range []string{"contract.md", "input.txt"} {
		if _, err := os.Stat(filepath.Join(destination, name)); err != nil {
			t.Fatalf("restored %s: %v", name, err)
		}
	}
	status := request(destination, "status", "main", 0, map[string]any{})
	if !status.OK || status.Data["taskState"] != "draft" || status.Data["stopped"] != true {
		t.Fatalf("quarantine=%+v", status)
	}
	clear := request(destination, "stop", "main", status.Revision, map[string]any{"enabled": false})
	if !clear.OK {
		t.Fatal(clear)
	}
	command, _ := json.Marshal(map[string]any{"id": "fresh", "executable": os.Args[0], "args": []string{"--ocmi-cli-test-helper", "0", destination, "0"}, "cwd": ".", "timeoutMs": 5000, "outputLimitBytes": 4096})
	rebound := request(destination, "handover.rebind", "main", clear.Revision, map[string]any{"command": json.RawMessage(command)})
	if !rebound.OK {
		t.Fatal(rebound)
	}
	ready := request(destination, "task.ready", "main", rebound.Revision, map[string]any{})
	if !ready.OK {
		t.Fatal(ready)
	}
	ack := request(source, "handover.ack", "main", exported.Revision, map[string]any{"transferId": "xfer-1", "generation": exported.Revision})
	if !ack.OK {
		t.Fatal(ack)
	}
	if replay := request(source, "handover.ack", "main", ack.Revision, map[string]any{"transferId": "xfer-1", "generation": exported.Revision}); replay.OK {
		t.Fatal("ack replay accepted")
	}
}

func TestPortableRejectsBadPackageAndUnknownReconcile(t *testing.T) {
	root, rev := setup(t, "0", 5000, 4096)
	if r := request(root, "operation.reconcile", "main", rev, map[string]any{"operationId": "missing", "disposition": "abandon", "reason": "guess"}); r.OK {
		t.Fatal(r)
	}
	bad := request(t.TempDir(), "handover.import", "main", 0, map[string]any{"package": map[string]any{"schemaVersion": 1, "system": "ocmi-runtime-preview", "transferId": "x", "generation": 1, "source": map[string]any{"projectId": "p"}, "files": map[string]any{"../escape": map[string]any{"content": "eA==", "sha256": "00", "size": 1}}, "task": map[string]any{"id": "t"}}, "deviceId": "d", "copyId": "c"})
	if bad.OK || (bad.Code != "INVALID_PACKAGE" && bad.Code != "INVALID_PACKAGE_PATH" && bad.Code != "PACKAGE_CHECKSUM") {
		t.Fatal(bad)
	}
}

func TestPortableV3RejectsMissingOrInconsistentCompletionScopeBeforeWrites(t *testing.T) {
	source, rev := setup(t, "0", 5000, 4096)
	run := request(source, "run", "worker", rev, map[string]any{})
	if !run.OK {
		t.Fatal(run)
	}
	exported := request(source, "handover.export", "main", run.Revision, map[string]any{"transferId": "scope-xfer", "files": []string{"contract.md", "input.txt"}})
	if !exported.OK {
		t.Fatal(exported)
	}
	original, err := json.Marshal(exported.Data["package"])
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*portablePackage)
	}{
		{"missing active scope", func(p *portablePackage) { p.Task.CompletionScope = "" }},
		{"inconsistent revision scope", func(p *portablePackage) { p.Revisions[0].CompletionScope = "preparation" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var pkg portablePackage
			if err := json.Unmarshal(original, &pkg); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&pkg)
			pkg.Digest = packageDigest(pkg)
			destination := t.TempDir()
			response := request(destination, "handover.import", "main", 0, map[string]any{"package": pkg, "deviceId": "dest", "copyId": "copy"})
			if response.OK || response.Code != "INVALID_PACKAGE" {
				t.Fatal(response)
			}
			entries, err := os.ReadDir(destination)
			if err != nil || len(entries) != 0 {
				t.Fatalf("destination entries=%v err=%v", entries, err)
			}
		})
	}
}

func TestPortableLegacySchemaRejectsCompletionScopeInjectionBeforeWrites(t *testing.T) {
	source, rev := setup(t, "0", 5000, 4096)
	run := request(source, "run", "worker", rev, map[string]any{})
	if !run.OK {
		t.Fatal(run)
	}
	exported := request(source, "handover.export", "main", run.Revision, map[string]any{"transferId": "legacy-scope-xfer", "files": []string{"contract.md", "input.txt"}})
	if !exported.OK {
		t.Fatal(exported)
	}
	raw, err := json.Marshal(exported.Data["package"])
	if err != nil {
		t.Fatal(err)
	}
	var pkg portablePackage
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatal(err)
	}
	pkg.SchemaVersion = 2
	pkg.Digest = packageDigest(pkg)
	destination := t.TempDir()
	response := request(destination, "handover.import", "main", 0, map[string]any{"package": pkg, "deviceId": "dest", "copyId": "copy"})
	if response.OK || response.Code != "INVALID_PACKAGE" {
		t.Fatal(response)
	}
	entries, err := os.ReadDir(destination)
	if err != nil || len(entries) != 0 {
		t.Fatalf("destination entries=%v err=%v", entries, err)
	}
}
