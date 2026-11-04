package internal

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResourceLocalIntentApplyWriteDisposeAndRedactedReceipt(t *testing.T) {
	root, rev := setup(t, "0", 5000, 4096)
	intent := request(root, "resource.intent", "main", rev, map[string]any{"resourceId": "scratch", "provider": "local", "kind": "directory", "mode": "create", "locator": "scratch", "purpose": "test scratch", "lifetime": "task", "disposition": "remove", "ownerRef": "main"})
	if !intent.OK {
		t.Fatal(intent)
	}
	apply := request(root, "resource.apply", "main", intent.Revision, map[string]any{"resourceId": "scratch", "contentBase64": ""})
	if !apply.OK || apply.Code != "RESOURCE_APPLIED" {
		t.Fatal(apply)
	}
	if info, err := os.Stat(filepath.Join(root, "scratch")); err != nil || !info.IsDir() {
		t.Fatalf("scratch=%v err=%v", info, err)
	}
	secret := base64.StdEncoding.EncodeToString([]byte("private resource bytes"))
	write := request(root, "resource.write", "worker", apply.Revision, map[string]any{"resourceId": "scratch", "path": "result.txt", "contentBase64": secret})
	if !write.OK || write.Code != "RESOURCE_WRITTEN" {
		t.Fatal(write)
	}
	db, err := openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	var fingerprint string
	if err = db.QueryRow("SELECT fingerprint FROM request_log WHERE id=(SELECT id FROM request_log ORDER BY rowid DESC LIMIT 1)").Scan(&fingerprint); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	if strings.Contains(fingerprint, secret) || strings.Contains(fingerprint, "private resource bytes") {
		t.Fatalf("raw content in receipt: %s", fingerprint)
	}
	dispose := request(root, "resource.dispose", "main", write.Revision, map[string]any{"resourceId": "scratch", "disposition": "remove", "reason": "done"})
	if !dispose.OK || dispose.Code != "RESOURCE_REMOVED" {
		t.Fatal(dispose)
	}
	if _, err := os.Stat(filepath.Join(root, "scratch")); !os.IsNotExist(err) {
		t.Fatalf("scratch remains: %v", err)
	}
	if inspected := request(root, "resource.inspect", "main", 0, map[string]any{"resourceId": "scratch"}); !inspected.OK || inspected.Data["observedState"] != "absent" {
		t.Fatal(inspected)
	}
	if status := request(root, "status", "main", 0, map[string]any{}); !status.OK || status.Data["resourceSummary"].(map[string]any)["total"] != 2 {
		t.Fatal(status)
	}
}

func TestIssuedResourceIntentCannotBeCancelledOnReplay(t *testing.T) {
	root, rev := setup(t, "0", 5000, 4096)
	intent := request(root, "resource.intent", "main", rev, map[string]any{"resourceId": "ambiguous", "provider": "local", "kind": "directory", "mode": "create", "locator": "ambiguous", "purpose": "fault", "lifetime": "task", "disposition": "remove", "ownerRef": "main"})
	if !intent.OK {
		t.Fatal(intent)
	}
	resourceEffectFault = func(string) error { return os.ErrDeadlineExceeded }
	apply := request(root, "resource.apply", "main", intent.Revision, map[string]any{"resourceId": "ambiguous"})
	resourceEffectFault = nil
	if apply.OK || apply.Code != "RESOURCE_PENDING" {
		t.Fatal(apply)
	}
	cancel := request(root, "resource.cancel", "main", apply.Revision, map[string]any{"resourceId": "ambiguous", "reason": "cannot cancel issued"})
	if cancel.OK || cancel.Code != "RESOURCE_CANCEL_REFUSED" {
		t.Fatal(cancel)
	}
}

func TestKeepAndTransferAreMetadataOnlyAndHoldAfterCreateWorks(t *testing.T) {
	root, rev := setup(t, "0", 5000, 4096)
	create := func(id, locator string, rev int64) Response {
		intent := request(root, "resource.intent", "main", rev, map[string]any{"resourceId": id, "provider": "local", "kind": "directory", "mode": "create", "locator": locator, "purpose": "retention", "lifetime": "task", "disposition": "remove", "ownerRef": "main"})
		if !intent.OK {
			t.Fatal(intent)
		}
		applied := request(root, "resource.apply", "main", intent.Revision, map[string]any{"resourceId": id})
		if !applied.OK {
			t.Fatal(applied)
		}
		return applied
	}
	kept := create("kept", "kept", rev)
	hold := request(root, "resource.hold", "main", kept.Revision, map[string]any{"resourceId": "kept", "enabled": true, "reason": "retain", "until": "review"})
	if !hold.OK {
		t.Fatal(hold)
	}
	keep := request(root, "resource.dispose", "main", hold.Revision, map[string]any{"resourceId": "kept", "disposition": "keep", "reason": "evidence", "until": "review"})
	if !keep.OK || keep.Code != "RESOURCE_KEPT" {
		t.Fatal(keep)
	}
	if info, err := os.Stat(filepath.Join(root, "kept")); err != nil || !info.IsDir() {
		t.Fatalf("keep deleted path: %v", err)
	}
	transferred := create("transfer", "transfer", keep.Revision)
	transfer := request(root, "resource.dispose", "main", transferred.Revision, map[string]any{"resourceId": "transfer", "disposition": "transfer", "reason": "handoff", "until": "ack", "transferId": "xfer"})
	if !transfer.OK || transfer.Code != "RESOURCE_TRANSFER_PENDING" {
		t.Fatal(transfer)
	}
	if info, err := os.Stat(filepath.Join(root, "transfer")); err != nil || !info.IsDir() {
		t.Fatalf("transfer deleted path: %v", err)
	}
}

func TestV3UpgradeCreatesResourceLedgerWithUnknownLegacyCoverage(t *testing.T) {
	root, rev := setup(t, "0", 5000, 4096)
	db, err := openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("DROP TABLE resource_event; DROP TABLE resource_use; DROP TABLE resource_ledger; DROP TABLE resource_meta; UPDATE meta SET schema_version=3")
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	if status := request(root, "status", "main", 0, map[string]any{}); status.OK || status.Code != "UPGRADE_REQUIRED" {
		t.Fatal(status)
	}
	upgraded := request(root, "state.upgrade", "main", rev, map[string]any{})
	if !upgraded.OK || upgraded.Code != "UPGRADED" {
		t.Fatal(upgraded)
	}
	db, err = openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version, legacy int
	if err = db.QueryRow("SELECT schema_version FROM meta").Scan(&version); err != nil || version != stateSchemaVersion {
		t.Fatalf("version=%d err=%v", version, err)
	}
	if err = db.QueryRow("SELECT legacy_coverage_unknown FROM resource_meta").Scan(&legacy); err != nil || legacy != 1 {
		t.Fatalf("legacy=%d err=%v", legacy, err)
	}
	if _, err = os.Stat(storePath(root) + ".v3-backup"); err != nil {
		t.Fatal(err)
	}
}
