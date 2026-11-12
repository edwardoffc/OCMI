package internal

import (
	"os"
	"testing"
)

// An uncatalogued namespace uses the same real operation/ledger caller. The
// unknown remote effect remains pending even after its local command succeeds.
func TestMainToolStorageUncataloguedClosure(t *testing.T) {
	f := mainToolSetup(t)
	root, next := f.project(t, "uncatalogued-lifecycle", f.ref(t), true)
	next = mainToolReady(t, root, next)
	next = request(root, "resource.intent", "main", next.Revision, map[string]any{
		"resourceId": "selected-opaque-job", "provider": "fictional-remote", "kind": "job",
		"mode": "use", "locator": "fixture://selected-job/one", "purpose": "Selected unverified remote-like effect",
		"lifetime": "until provider observation", "disposition": "release", "ownerRef": "main",
	})
	if !next.OK {
		t.Fatal("pre-effect remote accounting", next)
	}
	blocked := request(root, "run", "worker", next.Revision, map[string]any{})
	if blocked.OK || blocked.Code != "RESOURCE_PENDING" {
		t.Fatal("unobserved pre-effect resource did not stop launch", blocked)
	}
	// No remote action was issued. Cancel only this unused declaration, then
	// perform the independently available local part through the normal caller.
	next = request(root, "resource.cancel", "main", next.Revision, map[string]any{
		"resourceId": "selected-opaque-job", "reason": "No effect issued; complete independent local component first.",
	})
	if !next.OK {
		t.Fatal("unused declaration cancellation", next)
	}
	run := mainToolRun(t, root, next.Revision)
	observed := mainToolObserved(t, root)
	if observed["cache"] != f.cache || observed["scratch"] != observed["tmp"] || observed["tmpObserved"] != "present" {
		t.Fatal("uncatalogued ordinary child placement", observed)
	}
	db, err := openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	var stopped int
	err = db.QueryRow("SELECT count(*) FROM resource_ledger WHERE id=? AND observed_state='stopped'", "ocmi-process-"+run.Data["operationId"].(string)).Scan(&stopped)
	db.Close()
	if err != nil || stopped != 1 {
		t.Fatal("actual local process closure missing", stopped, err)
	}
	// A selected external reference still needs provider observation. Recording
	// it claims no remote execution and may not erase the accepted local result.
	next = request(root, "resource.intent", "main", run.Revision, map[string]any{
		"resourceId": "retained-opaque-job", "provider": "fictional-remote", "kind": "job",
		"mode": "use", "locator": "fixture://selected-job/two", "purpose": "Selected external reference awaiting observation",
		"lifetime": "until provider observation", "disposition": "release", "ownerRef": "main",
	})
	if !next.OK {
		t.Fatal("retain selected external reference", next)
	}
	remote := request(root, "resource.inspect", "main", 0, map[string]any{"resourceId": "retained-opaque-job"})
	if !remote.OK || remote.Data["observedState"] != "intent" || remote.Data["pending"] != true || remote.Data["nextAction"] != "resource.reconcile" {
		t.Fatal("unknown provider observation became completion", remote)
	}
	next = request(root, "submit", "worker", next.Revision, map[string]any{})
	if !next.OK {
		t.Fatal(next)
	}
	next = request(root, "review", "main", next.Revision, map[string]any{"accept": true, "reason": "Verified local result; remote observation remains pending."})
	if !next.OK {
		t.Fatal(next)
	}
	next = request(root, "integrate", "main", next.Revision, map[string]any{"candidate": "fictional-generic-component", "reason": "Accepted local behavior with separately retained resource."})
	if !next.OK {
		t.Fatal(next)
	}
	closed := request(root, "resource.close", "main", next.Revision, map[string]any{"disposition": "keep", "reason": "Retain external store and unverified remote effect."})
	if closed.OK || closed.Code != "RESOURCE_PENDING" {
		t.Fatal("local success erased remote cleanup uncertainty", closed)
	}
	if _, err := os.Stat(f.cache); err != nil {
		t.Fatal("external shared cache lost", err)
	}
}
