package internal

import "testing"

func TestStatusHintsTerminalAndSuccessfulOperationsWithoutMutation(t *testing.T) {
	for _, tc := range []struct {
		name, helper string
		timeout      int
		want         string
	}{
		{name: "success", helper: "0", timeout: 5000, want: "submit"},
		{name: "failed", helper: "7", timeout: 5000, want: "diagnose_and_revise"},
		{name: "incomplete", helper: "sleep", timeout: 10, want: "diagnose_and_revise"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, rev := setup(t, tc.helper, tc.timeout, 4096)
			run := request(root, "run", "worker", rev, map[string]any{})
			if !run.OK {
				t.Fatal(run)
			}
			status := request(root, "status", "main", 0, map[string]any{})
			if !status.OK || status.Revision != run.Revision || status.Data["nextAction"] != tc.want {
				t.Fatalf("status=%+v run=%+v", status, run)
			}
		})
	}
}

func TestStatusDoesNotPromoteUnknownOperationStateToSubmission(t *testing.T) {
	root, rev := setup(t, "0", 5000, 4096)
	run := request(root, "run", "worker", rev, map[string]any{})
	if !run.OK {
		t.Fatal(run)
	}
	db, err := openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("UPDATE operation SET state='unexpected_fixture_state' WHERE id=?", run.Data["operationId"])
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	status := request(root, "status", "main", 0, map[string]any{})
	if !status.OK || status.Revision != run.Revision || status.Data["nextAction"] != "diagnose_and_revise" {
		t.Fatalf("status=%+v", status)
	}
}
