package internal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func workspaceReaderFixture(t *testing.T) (layoutContext, string, int64) {
	t.Helper()
	root, rev := setup(t, "7", 5000, 4096)
	r := request(root, "run", "worker", rev, map[string]any{})
	if !r.OK || r.Data["operationState"] != "failed" {
		t.Fatal("original registered failed-run premise", r)
	}
	c, code := resolveLayout(root, "project")
	if code != "" {
		t.Fatal(code)
	}
	return c, "ocmi-workspace-" + r.Data["operationId"].(string), r.Revision
}

func workspaceReaderDeclaration(t *testing.T, system, value string) {
	t.Helper()
	if err := os.MkdirAll(system, 0700); err != nil {
		t.Fatal(err)
	}
	raw := `{}`
	if value != "" {
		raw = `{"operationWorkspaceInventoryPolicy":` + value + `}`
	}
	if err := os.WriteFile(filepath.Join(system, "build-info.json"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceReaderWorkerDeclarationAndDisposedControl(t *testing.T) {
	c, resource, rev := workspaceReaderFixture(t)
	if needed, err := workspaceReaderRequired(c); err != nil || !needed {
		t.Fatal("original retained automatic workspace not recognized", needed, err)
	}
	system := filepath.Join(t.TempDir(), "system")
	path := filepath.Join(c.Runs, strings.TrimPrefix(resource, "ocmi-workspace-"))
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", `"1"`, "true", "null", "0", "2", "1.0", "1e0", `1,"operationWorkspaceInventoryPolicy":1`} {
		workspaceReaderDeclaration(t, system, value)
		if err := workspaceReaderGuardSystem(c, system); !errors.Is(err, errWorkspaceReaderCapability) {
			t.Fatal("unsupported declaration selected", value, err)
		}
	}
	workspaceReaderDeclaration(t, system, "1")
	if err := workspaceReaderGuardSystem(c, system); err != nil {
		t.Fatal("integer1 reader refused", err)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("guard changed retained root", err)
	}
	if s := request(c.Root, "status", "main", 0, map[string]any{}); !s.OK || s.Revision != rev {
		t.Fatal("guard changed semantic revision", s)
	}
	r := request(c.Root, "resource.dispose", "main", rev, map[string]any{"resourceId": resource, "disposition": "remove", "reason": "reviewed original small automatic output"})
	if !r.OK {
		t.Fatal("qualified original disposal control", r)
	}
	workspaceReaderDeclaration(t, system, "")
	if err := workspaceReaderGuardSystem(c, system); err != nil {
		t.Fatal("disposed workspace still requires reader", err)
	}
	if err := workspacePreviousReaderGuard(c, updateProofJournal{Stage: "local/runs/no-retained-previous", Phase: "applied"}, false); err != nil {
		t.Fatal("old rollback remained blocked after qualified workspace disposal", err)
	}
}

func TestWorkspaceReaderWorkerNoRetainedAndAmbiguousState(t *testing.T) {
	root, _ := setup(t, "0", 5000, 4096)
	c, _ := resolveLayout(root, "project")
	if err := workspaceReaderGuard(c, filepath.Join(t.TempDir(), "no-candidate-needed")); err != nil {
		t.Fatal("no retained workspace restricted old transition", err)
	}
	system := filepath.Join(t.TempDir(), "system")
	workspaceReaderDeclaration(t, system, "1")
	for _, mutation := range []string{
		"UPDATE resource_ledger SET imported=1 WHERE id=?",
		"UPDATE resource_ledger SET project_id='foreign' WHERE id=?",
		"UPDATE resource_ledger SET copy_id='foreign' WHERE id=?",
		"UPDATE resource_ledger SET device_id='foreign' WHERE id=?",
		"UPDATE resource_ledger SET task_revision=task_revision+1 WHERE id=?",
		"UPDATE resource_ledger SET provider='unverified' WHERE id=?",
		"UPDATE resource_ledger SET native_identity='' WHERE id=?",
		"UPDATE resource_ledger SET transfer_id='ambiguous' WHERE id=?",
		"UPDATE resource_ledger SET observed_state='unknown' WHERE id=?",
		"DELETE FROM resource_event WHERE resource_id=? AND event='automatic_intent'",
		"UPDATE operation_owner SET phase='launching' WHERE operation_id=(SELECT operation_id FROM resource_ledger WHERE id=?)",
		"UPDATE operation_owner SET root='foreign' WHERE operation_id=(SELECT operation_id FROM resource_ledger WHERE id=?)",
	} {
		t.Run(mutation, func(t *testing.T) {
			selected, resource, _ := workspaceReaderFixture(t)
			db, err := openLayout(selected)
			if err != nil {
				t.Fatal(err)
			}
			_, mutationErr := db.Exec(mutation, resource)
			closeErr := db.Close()
			if mutationErr != nil || closeErr != nil {
				t.Fatal("malformed-state test premise", mutationErr, closeErr)
			}
			if err := workspaceReaderGuardSystem(selected, system); !errors.Is(err, errWorkspaceReaderCapability) {
				t.Fatal("association ambiguity accepted as empty/original", err)
			}
		})
	}
}

func TestWorkspaceReaderWorkerPreviousSelection(t *testing.T) {
	c, _, _ := workspaceReaderFixture(t)
	j := updateProofJournal{Stage: "local/runs/fictional-update", Phase: "applied"}
	system := filepath.Join(c.Root, filepath.FromSlash(j.Stage), "old", "system")
	workspaceReaderDeclaration(t, system, "")
	if workspacePreviousReaderGuard(c, j, false) == nil {
		t.Fatal("old rollback reader selected")
	}
	if err := workspacePreviousReaderGuard(c, j, true); err != nil {
		t.Fatal("terminal applied recovery spuriously selects previous", err)
	}
	j.Phase = "intent"
	if err := workspacePreviousReaderGuard(c, j, true); err != nil {
		t.Fatal("pre-activation intent cancellation spuriously selects previous", err)
	}
	j.Phase = "metadata_switch"
	if workspacePreviousReaderGuard(c, j, true) == nil {
		t.Fatal("interrupted recover selected unsupported old reader")
	}
	workspaceReaderDeclaration(t, system, "1")
	if err := workspacePreviousReaderGuard(c, j, true); err != nil {
		t.Fatal("capable previous reader refused", err)
	}
	raw, _ := json.Marshal(workspaceReaderFailure("recovery"))
	if strings.Contains(string(raw), c.Root) || !strings.Contains(string(raw), "workspace_inventory_reader_capability") {
		t.Fatal("unsafe/uninformative refusal", string(raw))
	}
}

func TestWorkspaceReaderWorkerFallbackRequired(t *testing.T) {
	c, resource, rev := workspaceReaderFixture(t)
	candidate := t.TempDir()
	workspaceReaderDeclaration(t, filepath.Join(candidate, "system"), "1")
	workspaceReaderDeclaration(t, filepath.Join(c.Root, "system"), "")
	if err := workspaceReaderGuard(c, candidate); err != nil {
		t.Fatal("capable candidate premise", err)
	}
	if err := workspaceReaderTransitionGuard(c, candidate); !errors.Is(err, errWorkspaceReaderCapability) {
		t.Fatal("incapable current generation allowed as retained-work fallback", err)
	}
	workspaceReaderDeclaration(t, filepath.Join(c.Root, "system"), "1")
	if err := workspaceReaderTransitionGuard(c, candidate); err != nil {
		t.Fatal("capable source and candidate refused", err)
	}
	workspaceReaderDeclaration(t, filepath.Join(c.Root, "system"), "")
	r := request(c.Root, "resource.dispose", "main", rev, map[string]any{"resourceId": resource, "disposition": "remove", "reason": "qualified old small-workspace closure before update"})
	if !r.OK {
		t.Fatal("original small disposal premise", r)
	}
	if err := workspaceReaderTransitionGuard(c, candidate); err != nil {
		t.Fatal("normal old-source transition blocked after disposal", err)
	}
}
