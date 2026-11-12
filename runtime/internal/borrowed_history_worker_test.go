package internal

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWorkerBorrowedHistoryHeldDrift(t *testing.T) {
	for _, condition := range []string{"known", "replaced", "absent"} {
		root, path, rev := historyLoanFixture(t, true, true)
		r := request(root, "resource.hold", "main", rev, map[string]any{"resourceId": "loan", "enabled": true, "reason": "retained evidence", "until": "review"})
		if !r.OK {
			t.Fatal(r)
		}
		if condition == "absent" {
			if e := os.Remove(path); e != nil {
				t.Fatal(e)
			}
		} else if condition == "replaced" {
			historyReplace(t, path)
		}
		for _, action := range []string{"resource.inspect", "resource.list", "status"} {
			r = request(root, action, "main", 0, map[string]any{"resourceId": "loan"})
			if !r.OK {
				t.Fatal(r)
			}
			if action == "resource.inspect" {
				if r.Data["pending"] != (condition != "known") || r.Data["historicalReleased"] == true {
					t.Fatal(r)
				}
			} else {
				key := "resourceSummary"
				if action == "resource.list" {
					key = "summary"
				}
				if (r.Data[key].(map[string]any)["pending"].(int) == 0) != (condition == "known") {
					t.Fatal(r)
				}
			}
		}
	}
}

func TestWorkerBorrowedHistoryManagedObservationVsDisposition(t *testing.T) {
	for _, kind := range []string{"file", "directory"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			r := request(root, "init", "main", 0, map[string]any{"projectId": "p", "deviceId": "d", "copyId": "c", "owner": "main"})
			if !r.OK {
				t.Fatal(r)
			}
			r = request(root, "resource.intent", "main", r.Revision, map[string]any{"resourceId": "managed", "provider": "local", "kind": kind, "mode": "create", "locator": "managed", "purpose": "ordinary data", "lifetime": "task", "disposition": "remove", "ownerRef": "main"})
			if !r.OK {
				t.Fatal(r)
			}
			r = request(root, "resource.apply", "main", r.Revision, map[string]any{"resourceId": "managed"})
			if !r.OK {
				t.Fatal(r)
			}
			r = request(root, "resource.inspect", "main", 0, map[string]any{"resourceId": "managed"})
			if !r.OK || r.Data["observedState"] != "present" || r.Data["pending"] != false || r.Data["historicalReleased"] == true {
				t.Fatal(r)
			}
			r = request(root, "resource.list", "main", 0, map[string]any{})
			if !r.OK || r.Data["summary"].(map[string]any)["pending"].(int) == 0 {
				t.Fatal(r)
			}
			found := false
			for _, item := range r.Data["resources"].([]map[string]any) {
				if item["resourceId"] == "managed" {
					found = true
					if item["pending"] != false || item["dispositionPending"] != true {
						t.Fatal(item)
					}
				}
			}
			if !found {
				t.Fatal("missing managed item")
			}
		})
	}
}

// One synthetic retained history, no timing threshold or benchmark machinery.
func TestWorkerBorrowedHistoryAccumulated(t *testing.T) {
	root, path, _ := historyLoanFixture(t, true, false)
	db, e := openExisting(root)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	rows, e := db.Query("PRAGMA table_info(resource_ledger)")
	if e != nil {
		t.Fatal(e)
	}
	var columns, projection []string
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var def any
		if e = rows.Scan(&cid, &name, &typ, &notnull, &def, &pk); e != nil {
			t.Fatal(e)
		}
		columns = append(columns, name)
		if name == "id" {
			projection = append(projection, "'history-'||n")
		} else {
			projection = append(projection, "l."+name)
		}
	}
	if e = rows.Err(); e != nil {
		t.Fatal(e)
	}
	rows.Close()
	const count = 256
	tx, e := db.Begin()
	if e != nil {
		t.Fatal(e)
	}
	query := "WITH RECURSIVE seq(n) AS (VALUES(1) UNION ALL SELECT n+1 FROM seq WHERE n<256) INSERT INTO resource_ledger(" + strings.Join(columns, ",") + ") SELECT " + strings.Join(projection, ",") + " FROM resource_ledger l CROSS JOIN seq WHERE l.id='loan'"
	if _, e = tx.Exec(query); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec("INSERT INTO resource_use(resource_id,task_id,task_revision,actor,access,reason,active) SELECT id,task_id,task_revision,'worker','read','synthetic released history',0 FROM resource_ledger WHERE id LIKE 'history-%'"); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec("INSERT INTO resource_event(resource_id,event,actor,request_id,task_id,task_revision,observed_state,proof_class,created_at) SELECT id,'release','worker','synthetic-history',task_id,task_revision,'active','declared','fixture' FROM resource_ledger WHERE id LIKE 'history-%'"); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	historyReplace(t, path)
	started := time.Now()
	total, pending, uses, e := resourceSummary(db, root, "")
	duration := time.Since(started)
	if e != nil || total < count || pending != 0 || uses != 0 {
		t.Fatal(total, pending, uses, e)
	}
	started = time.Now()
	r := request(root, "resource.list", "main", 0, map[string]any{"limit": 100})
	listDuration := time.Since(started)
	if !r.OK || len(r.Data["resources"].([]map[string]any)) != 100 || r.Data["truncated"] != true || r.Data["summary"].(map[string]any)["pending"] != 0 {
		t.Fatal(r)
	}
	t.Logf("HISTORY_COST clones=%d total=%d summary=%s list100=%s MaxOpenConns=1; one shared SQL projection per read, summary uses two Go queries; synthetic local file loans share a replaced locator; no timing threshold", count, total, duration, listDuration)
}

func historyStored(t *testing.T, db *sql.DB) string {
	t.Helper()
	result := ""
	for _, table := range []string{"project", "resource_ledger", "resource_use", "resource_event", "operation", "operation_owner", "operation_revision", "resource", "resource_revision"} {
		rows, e := db.Query("SELECT * FROM " + table + " ORDER BY rowid")
		if e != nil {
			t.Fatal(e)
		}
		columns, e := rows.Columns()
		if e != nil {
			t.Fatal(e)
		}
		for rows.Next() {
			values := make([]any, len(columns))
			ptrs := make([]any, len(columns))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if e = rows.Scan(ptrs...); e != nil {
				t.Fatal(e)
			}
			result += fmt.Sprint(values)
		}
		if e = rows.Err(); e != nil {
			t.Fatal(e)
		}
		rows.Close()
	}
	return result
}

func TestWorkerBorrowedHistoryAbsentStable(t *testing.T) {
	root, path, rev := historyLoanFixture(t, true, false)
	if e := os.Remove(path); e != nil {
		t.Fatal(e)
	}
	db, e := openExisting(root)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	before := historyStored(t, db)
	for _, action := range []string{"resource.inspect", "resource.list", "status"} {
		r := request(root, action, "main", 0, map[string]any{"resourceId": "loan"})
		if !r.OK || r.Revision != rev {
			t.Fatal(r)
		}
		if action == "resource.inspect" && (r.Data["historicalReleased"] != true || r.Data["observedState"] != "absent" || r.Data["pending"] != false) {
			t.Fatal(r)
		}
		if action == "resource.list" {
			for _, item := range r.Data["resources"].([]map[string]any) {
				if item["pending"] == true {
					t.Errorf("unexpected pending item: %v", item)
				}
			}
		}
	}
	if after := historyStored(t, db); after != before {
		t.Fatal("read views changed stored history")
	}
	r := request(root, "resource.use", "worker", rev, map[string]any{"resourceId": "loan", "access": "read", "reason": "absent binding"})
	if r.OK {
		t.Fatal(r)
	}
}

func TestWorkerBorrowedHistoryQualification(t *testing.T) {
	for _, tc := range []struct{ name, sql string }{
		{"import", "UPDATE resource_ledger SET imported=1 WHERE id='loan'"},
		{"provider", "UPDATE resource_ledger SET provider='unknown-provider' WHERE id='loan'"},
		{"revision", "UPDATE resource_ledger SET task_revision=99 WHERE id='loan'"},
		{"unfinished", "UPDATE operation SET state='unknown'"},
		{"inflight", "UPDATE resource_ledger SET applying=1 WHERE id='loan'"},
		{"stored-unknown", "UPDATE resource_ledger SET observed_state='unknown' WHERE id='loan'"},
		{"no-release", "DELETE FROM resource_event WHERE event='release'"},
		{"invalid-native", "UPDATE resource_ledger SET native_identity='not-native' WHERE id='loan'"},
		{"open-close", "UPDATE resource SET closed=0"},
		{"foreign-use", "INSERT INTO resource_use(resource_id,task_id,task_revision,actor,access,reason,active) VALUES('loan','other',8,'other','read','current',1)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, path, _ := historyLoanFixture(t, true, false)
			historyReplace(t, path)
			db, e := openExisting(root)
			if e != nil {
				t.Fatal(e)
			}
			_, e = db.Exec(tc.sql)
			db.Close()
			if e != nil {
				t.Fatal(e)
			}
			r := request(root, "resource.inspect", "main", 0, map[string]any{"resourceId": "loan"})
			if !r.OK || r.Data["historicalReleased"] == true || r.Data["pending"] != true {
				t.Fatal(r)
			}
			r = request(root, "status", "main", 0, map[string]any{})
			if !r.OK || r.Data["resourceSummary"].(map[string]any)["pending"].(int) == 0 {
				t.Fatal(r)
			}
		})
	}
}
