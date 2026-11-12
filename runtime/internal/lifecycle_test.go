package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOwnerDecisionRequiresFreshEvidenceAndGrantsOneAttempt(t *testing.T) {
	root, rev := setup(t, "7", 5000, 4096)
	run := func() {
		r := request(root, "run", "worker", rev, map[string]any{})
		if !r.OK {
			t.Fatal(r)
		}
		rev = r.Revision
	}
	ready := func() {
		r := request(root, "task.ready", "main", rev, map[string]any{})
		if !r.OK {
			t.Fatal(r)
		}
		rev = r.Revision
	}
	revise := func(version int64) {
		r := request(root, "task.revise", "main", rev, map[string]any{"taskId": "t", "outcomeId": "o", "taskRevision": version, "worker": "worker", "reviewer": "main", "completionScope": "target", "contract": "contract.md", "inputs": []string{"input.txt"}, "reason": "new diagnostic attempt", "command": map[string]any{"id": "test", "executable": os.Args[0], "args": []string{"--ocmi-cli-test-helper", "7", root, "7"}, "cwd": ".", "timeoutMs": 5000, "outputLimitBytes": 4096}})
		if !r.OK {
			t.Fatal(r)
		}
		rev = r.Revision
	}
	run()
	revise(2)
	ready()
	run()
	reassess := request(root, "outcome.reassess", "main", rev, map[string]any{"outcomeId": "o", "reason": "diagnosis", "changedApproach": "change", "nextEvidence": "result"})
	if !reassess.OK {
		t.Fatal(reassess)
	}
	rev = reassess.Revision
	revise(3)
	ready()
	run()
	review := request(root, "outcome.review", "main", rev, map[string]any{"outcomeId": "o", "decision": "continue", "findings": "failure", "changedApproach": "change again", "nextEvidence": "result"})
	if !review.OK {
		t.Fatal(review)
	}
	rev = review.Revision
	revise(4)
	ready()
	run()
	if err := os.WriteFile(filepath.Join(root, "diagnosis.txt"), []byte("fresh evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	hash, err := digest(filepath.Join(root, "diagnosis.txt"))
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"outcomeId": "o", "decision": "continue", "basis": "owner diagnosis", "changedApproach": "targeted change", "nextEvidence": "new result", "evidence": []map[string]string{{"path": "diagnosis.txt", "sha256": hash}}}
	decision := request(root, "outcome.decide", "main", rev, payload)
	if !decision.OK || decision.Code != "OUTCOME_DECISION_GRANTED" {
		t.Fatal(decision)
	}
	rev = decision.Revision
	revise(5)
	ready()
	run()
	if again := request(root, "outcome.decide", "main", rev, payload); again.OK || again.Code != "OUTCOME_DECISION_REUSED" {
		t.Fatal(again)
	}
	if history := request(root, "task.history", "main", 0, map[string]any{}); !history.OK || len(history.Data["decisions"].([]map[string]any)) < 3 {
		t.Fatal(history)
	}
}

func TestUpgradeRejectsInterleavedLegacyRevision(t *testing.T) {
	root, rev := setup(t, "0", 5000, 4096)
	db, err := openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("PRAGMA foreign_keys=OFF; DROP TABLE outcome_authorization; DROP TABLE outcome_decision; DROP TABLE outcome_policy; DROP TABLE resource_revision; DROP TABLE integration_revision; DROP TABLE review_revision; DROP TABLE operation_revision; DROP TABLE task_revision; DROP TABLE active_task; UPDATE meta SET schema_version=1")
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	upgradeBeforeTransaction = func() {
		other, e := openExisting(root)
		if e == nil {
			_, _ = other.Exec("UPDATE project SET revision=revision+1")
			other.Close()
		}
	}
	defer func() { upgradeBeforeTransaction = nil }()
	r := request(root, "state.upgrade", "main", rev, map[string]any{})
	if r.OK || r.Code != "STALE_REVISION" {
		t.Fatal(r)
	}
	db, err = openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	var current int64
	if db.QueryRow("SELECT schema_version FROM meta").Scan(&version) != nil || db.QueryRow("SELECT revision FROM project").Scan(&current) != nil || version != 1 || current != rev+1 {
		t.Fatalf("version=%d revision=%d", version, current)
	}
}

func TestV2ReadPathsRequireExplicitUpgrade(t *testing.T) {
	root, rev := setup(t, "0", 5000, 4096)
	db, err := openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE meta SET schema_version=2"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	for _, tc := range []struct {
		name, action string
		payload      map[string]any
	}{
		{"status", "status", map[string]any{}},
		{"inspect", "operation.inspect", map[string]any{"operationId": "missing"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := request(root, tc.action, "main", rev, tc.payload)
			if response.OK || response.Code != "UPGRADE_REQUIRED" || response.Data["nextAction"] != "state.upgrade" {
				t.Fatal(response)
			}
		})
	}
	db, err = openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRow("SELECT schema_version FROM meta").Scan(&version); err != nil || version != 2 {
		t.Fatalf("version=%d err=%v", version, err)
	}
}

func TestGateForConsumedReassessmentRequiresOutcomeReview(t *testing.T) {
	if got := gateForFailures(2, 1, 0); got != "outcome_review_required" {
		t.Fatalf("gate=%q", got)
	}
}

func TestSubmitIgnoresForeignRevisionOperation(t *testing.T) {
	root, rev := setup(t, "7", 5000, 4096)
	run := request(root, "run", "worker", rev, map[string]any{})
	if !run.OK {
		t.Fatal(run)
	}
	db, err := openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO operation(id,task_id,state,output,truncated,before_fingerprint,after_fingerprint,created_at) VALUES('foreign-future','t','succeeded','',0,'same','same','9999-01-01'); INSERT INTO operation_revision(operation_id,task_id,task_revision,outcome_id) VALUES('foreign-future','other',1,'other')")
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	if submitted := request(root, "submit", "worker", run.Revision, map[string]any{}); submitted.OK || submitted.Code != "EVIDENCE_INVALID" {
		t.Fatal(submitted)
	}
}

func TestPreparationAcceptanceKeepsOutcomePolicyAndHistoryScope(t *testing.T) {
	root, rev := setupWithScope(t, "0", 5000, 4096, "preparation")
	run := request(root, "run", "worker", rev, map[string]any{})
	if !run.OK {
		t.Fatal(run)
	}
	submitted := request(root, "submit", "worker", run.Revision, map[string]any{})
	if !submitted.OK {
		t.Fatal(submitted)
	}
	db, err := openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	var before struct {
		failures, reassess, reviewed, allowance int
		gate                                    string
	}
	if err := db.QueryRow("SELECT failure_count,reassess_used,review_used,allowance,gate FROM outcome_policy WHERE outcome_id='o'").Scan(&before.failures, &before.reassess, &before.reviewed, &before.allowance, &before.gate); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	review := request(root, "review", "main", submitted.Revision, map[string]any{"accept": true, "reason": "preparation accepted"})
	if !review.OK || review.Code != "ACCEPTED" {
		t.Fatal(review)
	}
	db, err = openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	var after struct {
		failures, reassess, reviewed, allowance int
		gate                                    string
	}
	err = db.QueryRow("SELECT failure_count,reassess_used,review_used,allowance,gate FROM outcome_policy WHERE outcome_id='o'").Scan(&after.failures, &after.reassess, &after.reviewed, &after.allowance, &after.gate)
	db.Close()
	if err != nil || before != after {
		t.Fatalf("policy before=%+v after=%+v err=%v", before, after, err)
	}
	history := request(root, "task.history", "main", 0, map[string]any{"taskId": "t"})
	if !history.OK || history.Data["tasks"].([]map[string]any)[0]["completionScope"] != "preparation" || history.Data["reviews"].([]map[string]any)[0]["completionScope"] != "preparation" {
		t.Fatal(history)
	}
}
