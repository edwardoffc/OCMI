package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func boundsFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	op := "8c43d039-8893-4b53-87f7-a90d29b2f91a"
	loc := ".ocmi/local/runs/" + op
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(loc))), 0700); err != nil {
		t.Fatal(err)
	}
	p, err := createLocalResource(root, loc, "directory", "ocmi-"+op, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	return root, loc, p.NativeIdentity
}
func boundsFile(t *testing.T, path string, size int64) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = f.Truncate(size)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
}

func TestWorkerWorkspaceBoundsLargePolicy(t *testing.T) {
	root, loc, id := boundsFixture(t)
	group := filepath.Join(root, filepath.FromSlash(loc))
	boundsFile(t, filepath.Join(group, "artifact.bin"), 21<<20)
	for i := 0; i < 292; i++ {
		boundsFile(t, filepath.Join(group, fmt.Sprintf("small-%03d", i)), 1)
	}
	if _, err := captureLocalResource(root, loc, "directory", id, true); err == nil {
		t.Fatal("internal flag enlarged ordinary tool-stage policy")
	}
	p, err := captureAutomaticWorkspace(root, loc, id)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := recoveryProjection(p.Inventory, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(projection)
	if projection["entryCount"] != 294 || projection["fileCount"] != 294 || projection["totalBytes"] != int64((21<<20)+292+41) || projection["entriesComplete"] != false || projection["omittedEntries"] != 286 || len(raw) > 8192 {
		t.Fatal(projection)
	}
	if strings.Contains(string(raw), "nativeIdentity") || strings.Contains(string(raw), "proof") {
		t.Fatal("full private proof leaked")
	}
	if err := removeAutomaticWorkspace(root, loc, id, p.Inventory); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(group); !os.IsNotExist(err) {
		t.Fatal("workspace not removed", err)
	}
}

func TestWorkerWorkspaceBoundsLimits(t *testing.T) {
	for _, which := range []string{"nodes", "file", "total", "depth", "path"} {
		t.Run(which, func(t *testing.T) {
			root, loc, id := boundsFixture(t)
			group := filepath.Join(root, filepath.FromSlash(loc))
			switch which {
			case "nodes":
				for i := 0; i < 1024; i++ {
					boundsFile(t, filepath.Join(group, fmt.Sprintf("f%04d", i)), 0)
				}
			case "file":
				boundsFile(t, filepath.Join(group, "large"), (64<<20)+1)
			case "total":
				boundsFile(t, filepath.Join(group, "one"), 64<<20)
				boundsFile(t, filepath.Join(group, "two"), 64<<20)
			case "depth":
				if err := os.MkdirAll(filepath.Join(group, strings.Repeat("d/", 33)), 0700); err != nil {
					t.Fatal(err)
				}
			case "path":
				if runtime.GOOS == "darwin" {
					// Darwin's absolute path ceiling prevents creating a relative
					// 1025-byte path beneath this nonempty fixture prefix. Exercise
					// the actual walker at an explicit smaller private boundary.
					boundsNativePathBudget(t, root, loc, id)
					return
				}
				if err := os.MkdirAll(filepath.Join(group, strings.Repeat(strings.Repeat("a", 200)+"/", 6)), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := captureAutomaticWorkspace(root, loc, id); err == nil {
				t.Fatal("accepted overflow", which)
			}
			if _, err := os.Stat(filepath.Join(group, localMarker)); err != nil {
				t.Fatal("overflow removed original", err)
			}
		})
	}
	root, loc, id := boundsFixture(t)
	group := filepath.Join(root, filepath.FromSlash(loc))
	for i := 0; i < 256; i++ {
		boundsFile(t, filepath.Join(group, fmt.Sprintf("n%04d", i)), 0)
	}
	if _, err := captureLocalResource(root, loc, "directory", id, true); err == nil {
		t.Fatal("ordinary node limit enlarged")
	}
	if _, err := captureAutomaticWorkspace(root, loc, id); err != nil {
		t.Fatal("wide bounded enumeration", err)
	}
}

func boundsNativePathBudget(t *testing.T, root, loc, id string) {
	t.Helper()
	policy := automaticInventoryPolicy
	policy.pathBytes = 64 // Private test value; production policy stays 1024.
	group := filepath.Join(root, filepath.FromSlash(loc))
	boundsFile(t, filepath.Join(group, strings.Repeat("a", 64)), 1)
	if _, err := captureLocalResourcePolicy(root, loc, "directory", id, true, policy); err != nil {
		t.Fatal("actual native path at private boundary refused", err)
	}
	boundsFile(t, filepath.Join(group, strings.Repeat("b", 65)), 1)
	if _, err := captureLocalResourcePolicy(root, loc, "directory", id, true, policy); err == nil || !strings.Contains(err.Error(), "inventory path overflow") {
		t.Fatal("native over-bound path did not reach walker path guard", err)
	}
	if _, err := os.Stat(filepath.Join(group, localMarker)); err != nil {
		t.Fatal("path refusal removed original marker", err)
	}
}

func TestWorkerWorkspaceBoundsPortablePathBoundaries(t *testing.T) {
	// Encoding controls use valid relative components on every host without
	// attempting to construct an OS-impossible absolute path on Darwin.
	exact := strings.Repeat(strings.Repeat("a", 199)+"/", 5) + strings.Repeat("b", 24)
	if len(exact) != 1024 || automaticInventoryPolicy.pathBytes != 1024 {
		t.Fatal("production boundary premise changed")
	}
	item := func(path string) string {
		return inventoryItem("f", path, "fictional-id", strings.Repeat("a", 64)+":1")
	}
	paths, err := inventoryPathsPolicy(item(exact), automaticInventoryPolicy)
	if err != nil || len(paths) != 1 || filepath.ToSlash(paths[0]) != exact {
		t.Fatal("production parser refused exact 1024-byte path", err)
	}
	if _, err := inventoryPathsPolicy(item(exact+"b"), automaticInventoryPolicy); err == nil {
		t.Fatal("production parser accepted 1025-byte path")
	}
	// Run this control on all hosts, so the Darwin fixture construction and
	// actual walker assertions also receive local focused qualification.
	root, loc, id := boundsFixture(t)
	boundsNativePathBudget(t, root, loc, id)
}

func TestWorkerWorkspaceBoundsEncodingAndStream(t *testing.T) {
	if _, err := inventoryPaths(strings.Repeat("x", (16<<20)+1)); err == nil {
		t.Fatal("ordinary parser unbounded")
	}
	if _, err := inventoryPathsPolicy(strings.Repeat("x", (1<<20)+1), automaticInventoryPolicy); err == nil {
		t.Fatal("workspace parser unbounded")
	}
	w := localInventoryWalk{policy: automaticInventoryPolicy}
	w.encoded = w.policy.encodedBytes - 1
	if err := w.append("f", "name", "id", strings.Repeat("a", 64)+":0"); err == nil || len(w.entries) != 0 {
		t.Fatal("encoding allocated appended overflow")
	}
	file := filepath.Join(t.TempDir(), "stream")
	boundsFile(t, file, 17)
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, n, err := digestInventoryFile(f, 16); err == nil || n != 17 {
		t.Fatal("stream sentinel failed", n, err)
	}
	a, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	boundsFile(t, file, 18)
	b, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if stableInventoryInfo(a, b) {
		t.Fatal("changed metadata accepted")
	}
	long := strings.Repeat("界", 90)
	inv := inventoryItem("f", long, "private-native-id", strings.Repeat("a", 64)+":1")
	p, err := recoveryProjection(inv, strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	entry := p["entries"].([]map[string]any)[0]
	if len(entry["path"].(string)) > 128 || entry["truncated"] != true || p["entriesComplete"] != false {
		t.Fatal(p)
	}
}

func TestWorkerWorkspaceBoundsExactAndAssociation(t *testing.T) {
	root, loc, id := boundsFixture(t)
	group := filepath.Join(root, filepath.FromSlash(loc))
	boundsFile(t, filepath.Join(group, "one"), 64<<20)
	boundsFile(t, filepath.Join(group, "two"), (64<<20)-41)
	if _, err := captureAutomaticWorkspace(root, loc, id); err != nil {
		t.Fatal("exact streamed budgets refused", err)
	}
	boundsFile(t, filepath.Join(group, "over"), 1)
	if _, err := captureAutomaticWorkspace(root, loc, id); err == nil {
		t.Fatal("aggregate plus one accepted")
	}
	project, _ := setup(t, "0", 5000, 4096)
	op := "8c43d039-8893-4b53-87f7-a90d29b2f91a"
	db := insertRecoveryOwner(t, project, op, "prepared", "launching")
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	task, err := loadTask(tx)
	if err != nil {
		t.Fatal(err)
	}
	if err = automaticRunResources(tx, Request{ProjectRoot: project, Actor: "worker", RequestID: "original-run"}, task, op, "c", "d"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var qualified bool
	query := "SELECT " + automaticWorkspaceProjection + " FROM resource_ledger WHERE id=?"
	if err = db.QueryRow(query, "ocmi-workspace-"+op).Scan(&qualified); err != nil || !qualified {
		t.Fatal("original association refused", err)
	}
	// Keep the original automatic_intent intact while changing one context
	// condition at a time. Roll back each negative premise independently.
	for _, mutation := range []string{
		"UPDATE resource_ledger SET transfer_id='fictional-transfer' WHERE id='ocmi-workspace-" + op + "'",
		"UPDATE operation_owner SET copy_id='different-copy' WHERE operation_id='" + op + "'",
		"UPDATE operation_owner SET device_id='different-device' WHERE operation_id='" + op + "'",
	} {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(mutation); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		err = tx.QueryRow(query, "ocmi-workspace-"+op).Scan(&qualified)
		rollbackErr := tx.Rollback()
		if err != nil || rollbackErr != nil || qualified {
			t.Fatal("ambiguous association gained enlarged policy", err, rollbackErr)
		}
	}
	if err = db.QueryRow(query, "ocmi-workspace-"+op).Scan(&qualified); err != nil || !qualified {
		t.Fatal("original association not restored", err)
	}
	if _, err = db.Exec("DELETE FROM resource_event WHERE resource_id=? AND event='automatic_intent'", "ocmi-workspace-"+op); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(query, "ocmi-workspace-"+op).Scan(&qualified); err != nil || qualified {
		t.Fatal("locator without original event gained policy", err)
	}
}

func TestWorkerWorkspaceBoundsDirectoryReopenIdentity(t *testing.T) {
	base := t.TempDir()
	child := filepath.Join(base, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	boundsFile(t, filepath.Join(child, "original-file"), 1)
	opened, err := os.Open(child)
	if err != nil {
		t.Fatal(err)
	}
	originalID, err := localNativeIDFile(opened)
	closeErr := opened.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	held, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	original := localInventoryWalk{policy: automaticInventoryPolicy}
	if err := original.walk(held, "child", "child", 1, originalID); err != nil {
		t.Fatal("original descendant refused", err)
	}
	// Deterministically replace after the parent's descriptor observation and
	// before recursive reopen. No sleeps, scheduling race or production hook.
	if err := os.Rename(child, filepath.Join(base, "parked-original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	boundsFile(t, filepath.Join(child, "replacement-file"), 1)
	replacementID, err := localNativeID(child)
	if err != nil || replacementID == originalID {
		t.Fatal("replacement premise", err)
	}
	replaced := localInventoryWalk{policy: automaticInventoryPolicy}
	if err := replaced.walk(held, "child", "child", 1, originalID); err == nil || replaced.nodes != 0 || len(replaced.entries) != 0 {
		t.Fatal("replacement was enumerated under original ID", err)
	}
	if err := os.Rename(child, filepath.Join(base, "parked-replacement")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(base, "parked-original"), child); err != nil {
		t.Fatal(err)
	}
	restored := localInventoryWalk{policy: automaticInventoryPolicy}
	if err := restored.walk(held, "child", "child", 1, originalID); err != nil {
		t.Fatal("restored original descendant refused", err)
	}
}
