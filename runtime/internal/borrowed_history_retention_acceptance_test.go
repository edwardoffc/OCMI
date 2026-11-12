package internal

import "testing"

// An explicit hold protects retained evidence; it is not itself unfinished cleanup.
func TestMainBorrowedHistoryKnownHeldRetention(t *testing.T) {
	root, _, rev := historyLoanFixture(t, true, true)
	r := request(root, "resource.hold", "main", rev, map[string]any{"resourceId": "loan", "enabled": true, "reason": "retain accepted evidence", "until": "owner review"})
	if !r.OK {
		t.Fatal(r)
	}
	for _, action := range []string{"resource.inspect", "resource.list", "status"} {
		r = request(root, action, "main", 0, map[string]any{"resourceId": "loan"})
		if !r.OK {
			t.Fatal(r)
		}
		if action == "resource.inspect" && (r.Data["pending"] != false || r.Data["historicalReleased"] == true) {
			t.Fatal(r)
		}
		key := "resourceSummary"
		if action == "resource.list" {
			key = "summary"
		}
		if action != "resource.inspect" && r.Data[key].(map[string]any)["pending"].(int) != 0 {
			t.Fatal(r)
		}
	}
}
