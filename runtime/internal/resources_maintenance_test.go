package internal

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func maintenanceFixture(t *testing.T, kind string) (string, int64) {
	t.Helper()
	root, rev := setup(t, "0", 5000, 4096)
	r := request(root, "resource.intent", "main", rev, map[string]any{"resourceId": "artifacts", "provider": "local", "kind": kind, "mode": "create", "locator": "artifacts", "purpose": "retained test evidence", "lifetime": "owner review", "disposition": "remove", "ownerRef": "main"})
	if !r.OK {
		t.Fatal(r)
	}
	r = request(root, "resource.apply", "main", r.Revision, map[string]any{"resourceId": "artifacts", "contentBase64": base64.StdEncoding.EncodeToString([]byte("PRIVATE-CONTENT-CANARY"))})
	if !r.OK {
		t.Fatal(r)
	}
	if kind == "directory" {
		r = request(root, "resource.write", "worker", r.Revision, map[string]any{"resourceId": "artifacts", "path": "report.log", "contentBase64": base64.StdEncoding.EncodeToString([]byte("PRIVATE-CONTENT-CANARY"))})
		if !r.OK {
			t.Fatal(r)
		}
	}
	return root, r.Revision
}

func maintenancePayload(id string) map[string]any {
	return map[string]any{"resourceId": id, "sizeLimitBytes": 1, "ageLimitDays": 1, "maxEntries": 64, "maxBytes": 16 << 20}
}

func maintenanceCall(t *testing.T, root string, payload map[string]any) Response {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return resourceMaintenanceRead(Request{ProjectRoot: root, Actor: "main", Payload: raw})
}

func TestResourceMaintenanceMetadataAndProtection(t *testing.T) {
	for _, kind := range []string{"file", "directory"} {
		t.Run(kind, func(t *testing.T) {
			root, rev := maintenanceFixture(t, kind)
			path := filepath.Join(root, "artifacts")
			if kind == "directory" {
				path = filepath.Join(path, "report.log")
			}
			old := time.Now().Add(-4 * 24 * time.Hour)
			if err := os.Chtimes(path, old, old); err != nil {
				t.Fatal(err)
			}
			rawBefore, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var expectedBytes int64
			expectedEntries := 1
			if kind == "directory" {
				entries, err := os.ReadDir(filepath.Dir(path))
				if err != nil {
					t.Fatal(err)
				}
				expectedEntries = len(entries)
				for _, entry := range entries {
					info, err := entry.Info()
					if err != nil {
						t.Fatal(err)
					}
					expectedBytes += info.Size()
				}
			} else {
				expectedBytes = int64(len(rawBefore))
			}
			check := func(active int, held bool, revision int64) {
				t.Helper()
				r := maintenanceCall(t, root, maintenancePayload("artifacts"))
				if !r.OK || r.Code != "RESOURCE_MAINTENANCE_INSPECTED" || r.Revision != revision || r.Data["complete"] != true || r.Data["mutationSupported"] != false || r.Data["observedBytes"] != expectedBytes || r.Data["observedEntries"] != expectedEntries || r.Data["oldestAgeDays"].(int64) < 3 || r.Data["sizeExceeded"] != true || r.Data["ageExceeded"] != true || r.Data["activeUses"] != active || r.Data["held"] != held || r.Data["protected"] != (held || active > 0) {
					t.Fatal(r)
				}
				encoded, _ := json.Marshal(r.Data)
				if strings.Contains(string(encoded), root) || strings.Contains(string(encoded), "PRIVATE-CONTENT") || strings.Contains(string(encoded), "nativeIdentity") {
					t.Fatal("maintenance leaked private observation", string(encoded))
				}
			}
			check(0, false, rev)
			r := request(root, "resource.use", "worker", rev, map[string]any{"resourceId": "artifacts", "access": "read", "reason": "known active use"})
			if !r.OK {
				t.Fatal(r)
			}
			r = request(root, "resource.hold", "main", r.Revision, map[string]any{"resourceId": "artifacts", "enabled": true, "reason": "keep evidence", "until": "owner review"})
			if !r.OK {
				t.Fatal(r)
			}
			check(1, true, r.Revision)
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(rawBefore) {
				t.Fatal("inspection changed bytes", err)
			}
			info, err := os.Stat(path)
			if err != nil || !info.ModTime().Equal(old) {
				t.Fatal("inspection changed mtime", err)
			}
		})
	}
}

func TestResourceMaintenanceStrictBoundsAndUnsupported(t *testing.T) {
	root, rev := maintenanceFixture(t, "directory")
	for _, tc := range []struct {
		key string
		val any
	}{
		{"extra", true}, {"ResourceId", "artifacts"}, {"sizeLimitBytes", 0}, {"sizeLimitBytes", int64(1<<50) + 1},
		{"ageLimitDays", 0}, {"ageLimitDays", 36501}, {"maxEntries", 0}, {"maxEntries", 1025},
		{"maxBytes", 0}, {"maxBytes", int64(1<<50) + 1}, {"maxBytes", 1}, {"maxEntries", 1},
		{"resourceId", "unknown"},
	} {
		payload := maintenancePayload("artifacts")
		payload[tc.key] = tc.val
		r := maintenanceCall(t, root, payload)
		if r.OK || r.Revision != rev || r.Data["complete"] != false || r.Data["mutationSupported"] != false || r.Data["nextAction"] == "" {
			t.Fatal(tc, r)
		}
		if _, exists := r.Data["observedBytes"]; exists {
			t.Fatal("incomplete observation presented byte total", r)
		}
	}
	encoded, _ := json.Marshal(maintenancePayload("artifacts"))
	encoded = append(encoded[:len(encoded)-1], []byte(`,"resourceId":"artifacts"}`)...)
	if r := resourceMaintenanceRead(Request{ProjectRoot: root, Actor: "main", Payload: encoded}); r.OK {
		t.Fatal("duplicate key accepted", r)
	}
	foreign := t.TempDir()
	canary := filepath.Join(foreign, "keep")
	if err := os.WriteFile(canary, []byte("foreign unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	r := request(root, "resource.intent", "main", rev, map[string]any{"resourceId": "external", "provider": "local", "kind": "directory", "mode": "use", "locator": foreign, "purpose": "borrowed", "lifetime": "task", "disposition": "release", "ownerRef": "external"})
	if !r.OK {
		t.Fatal(r)
	}
	r = request(root, "resource.apply", "main", r.Revision, map[string]any{"resourceId": "external"})
	if !r.OK {
		t.Fatal(r)
	}
	if r := maintenanceCall(t, root, maintenancePayload("external")); r.OK || r.Data["complete"] != false {
		t.Fatal("borrowed root measured", r)
	}
	if raw, err := os.ReadFile(canary); err != nil || string(raw) != "foreign unchanged" {
		t.Fatal("borrowed root changed", err)
	}
	path := filepath.Join(root, "artifacts")
	if err := os.Rename(path, path+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if r := maintenanceCall(t, root, maintenancePayload("artifacts")); r.OK || r.Data["complete"] != false {
		t.Fatal("replacement adopted", r)
	}
}

func TestResourceMaintenanceStateAndMetadataRace(t *testing.T) {
	root, rev := maintenanceFixture(t, "file")
	r := maintenanceCall(t, root, maintenancePayload("ocmi-state"))
	if !r.OK || r.Revision != rev || r.Data["protected"] != true || r.Data["mutationSupported"] != false || r.Data["observedBytes"].(int64) <= 0 || r.Data["maintenanceRequired"] != true {
		t.Fatal("live state measurement", r)
	}
	if status := request(root, "status", "main", 0, map[string]any{}); !status.OK || status.Revision != rev {
		t.Fatal("measurement mutated semantic revision", status)
	}
	group := t.TempDir()
	if err := os.WriteFile(filepath.Join(group, "member"), []byte("known"), 0600); err != nil {
		t.Fatal(err)
	}
	held, err := os.OpenRoot(group)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	m := resourceMaintenanceMeasurement{input: resourceMaintenanceInput{MaxEntries: 4, MaxBytes: 64}, now: time.Now()}
	if err := m.walk(held, "."); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(group, "member")); err != nil {
		t.Fatal(err)
	}
	if m.verify(held) == nil {
		t.Fatal("lost entry accepted as complete")
	}
	m = resourceMaintenanceMeasurement{input: resourceMaintenanceInput{MaxEntries: 4, MaxBytes: 64}, now: time.Now()}
	if err := m.file(held, "optional", true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(group, "optional"), []byte("arrived"), 0600); err != nil {
		t.Fatal(err)
	}
	if m.verify(held) == nil {
		t.Fatal("sidecar arrival accepted as stable absence")
	}
}

func TestResourceMaintenanceRefusesLinkedEntry(t *testing.T) {
	root, _ := maintenanceFixture(t, "directory")
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "artifacts", "linked")); err != nil {
		t.Skip("native link creation unavailable")
	}
	if r := maintenanceCall(t, root, maintenancePayload("artifacts")); r.OK || r.Data["complete"] != false {
		t.Fatal("linked entry accepted", r)
	}
}

func TestResourceMaintenanceUnknownProvenanceAndFutureAge(t *testing.T) {
	root, rev := maintenanceFixture(t, "file")
	future := time.Now().Add(48 * time.Hour)
	if err := os.Chtimes(filepath.Join(root, "artifacts"), future, future); err != nil {
		t.Fatal(err)
	}
	r := maintenanceCall(t, root, maintenancePayload("artifacts"))
	if !r.OK || r.Data["oldestAgeDays"] != int64(0) || r.Data["ageExceeded"] != false {
		t.Fatal("future mtime was treated as elapsed age", r)
	}
	db, err := openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, sql := range []string{
		"UPDATE resource_ledger SET imported=1 WHERE id='artifacts'",
		"UPDATE resource_ledger SET imported=0,applying=1 WHERE id='artifacts'",
		"UPDATE resource_ledger SET applying=0,observed_state='unknown' WHERE id='artifacts'",
		"UPDATE resource_ledger SET observed_state='active',provider='unverified' WHERE id='artifacts'",
		"UPDATE resource_ledger SET provider='local',purpose=printf('%2048s','private') WHERE id='artifacts'",
	} {
		if _, err := db.Exec(sql); err != nil {
			t.Fatal(err)
		}
		r := maintenanceCall(t, root, maintenancePayload("artifacts"))
		if r.OK || r.Revision != rev || r.Data["complete"] != false || r.Data["mutationSupported"] != false {
			t.Fatal("unknown/pending provenance was measured", r)
		}
		if raw, _ := json.Marshal(r.Data); len(raw) > 1024 {
			t.Fatal("unbounded ownership metadata returned")
		}
	}
	if _, err := db.Exec("UPDATE resource_ledger SET imported=1 WHERE id='ocmi-state'"); err != nil {
		t.Fatal(err)
	}
	if r := maintenanceCall(t, root, maintenancePayload("ocmi-state")); r.OK || r.Data["complete"] != false {
		t.Fatal("imported state record granted original-state scope", r)
	}
}

func TestResourceMaintenanceLargeSparseMetadata(t *testing.T) {
	root, rev := maintenanceFixture(t, "file")
	path := filepath.Join(root, "artifacts")
	const size = int64(32 << 20)
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := maintenanceMarkSparse(f); err != nil {
		f.Close()
		t.Skipf("native sparse file unavailable: %v", err)
	}
	truncateErr := f.Truncate(size)
	closeErr := f.Close()
	if truncateErr != nil || closeErr != nil {
		t.Fatal("sparse fixture preparation", truncateErr, closeErr)
	}
	payload := maintenancePayload("artifacts")
	payload["maxBytes"] = int64(1 << 50)
	r := maintenanceCall(t, root, payload)
	if !r.OK || r.Revision != rev || r.Data["complete"] != true || r.Data["observedBytes"] != size || r.Data["observedEntries"] != 1 || r.Data["mutationSupported"] != false {
		t.Fatal("large metadata measurement inherited content proof bound", r)
	}
	payload["maxBytes"] = size - 1
	r = maintenanceCall(t, root, payload)
	if r.OK || r.Data["complete"] != false || r.Data["mutationSupported"] != false {
		t.Fatal("configured large-file byte bound ignored", r)
	}
	if _, ok := r.Data["observedBytes"]; ok {
		t.Fatal("overflow exposed a complete-looking partial total", r)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() != size {
		t.Fatal("large file mutated", err)
	}
}

func TestResourceMaintenanceLegacyRequiresUpgradeWithoutMutation(t *testing.T) {
	root, rev := setup(t, "0", 5000, 4096)
	db, err := openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Reuse the existing v3 upgrade consumer's legacy schema premise.
	if _, err := db.Exec("DROP TABLE resource_event; DROP TABLE resource_use; DROP TABLE resource_ledger; DROP TABLE resource_meta; UPDATE meta SET schema_version=3"); err != nil {
		t.Fatal(err)
	}
	r := maintenanceCall(t, root, maintenancePayload("ocmi-state"))
	if r.OK || r.Code != "UPGRADE_REQUIRED" || r.Data["complete"] != false || r.Data["mutationSupported"] != false {
		t.Fatal("valid legacy state mislabeled or upgraded", r)
	}
	var version int
	var current int64
	if err := db.QueryRow("SELECT schema_version FROM meta").Scan(&version); err != nil || version != 3 {
		t.Fatal("inspection changed old schema", version, err)
	}
	if err := db.QueryRow("SELECT revision FROM project").Scan(&current); err != nil || current != rev {
		t.Fatal("inspection changed legacy revision", current, err)
	}
	if _, err := os.Stat(storePath(root) + ".v3-backup"); !os.IsNotExist(err) {
		t.Fatal("read-only inspection initiated upgrade backup", err)
	}
}
