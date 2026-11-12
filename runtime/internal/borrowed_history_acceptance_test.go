package internal

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func historyLoanFixture(t *testing.T, closeTask, owned bool) (string, string, int64) {
	t.Helper()
	root, rev := setup(t, "0", 5000, 4096)
	path := filepath.Join(t.TempDir(), "borrowed-tool")
	mode, disposition := "use", "release"
	if owned {
		path = "kept.txt"
		mode = "create"
		disposition = "keep"
	} else if err := os.WriteFile(path, []byte("fictional tool"), 0600); err != nil {
		t.Fatal(err)
	}
	r := request(root, "resource.intent", "main", rev, map[string]any{"resourceId": "loan", "provider": "local", "kind": "file", "mode": mode, "locator": path, "purpose": "fictional dependency", "lifetime": "task", "disposition": disposition, "ownerRef": "fixture-owner"})
	if !r.OK {
		t.Fatal(r)
	}
	r = request(root, "resource.apply", "main", r.Revision, map[string]any{"resourceId": "loan", "contentBase64": base64.StdEncoding.EncodeToString([]byte("fictional tool"))})
	if !r.OK {
		t.Fatal(r)
	}
	if !owned {
		r = request(root, "resource.use", "worker", r.Revision, map[string]any{"resourceId": "loan", "access": "read", "reason": "declared dependency"})
		if !r.OK {
			t.Fatal(r)
		}
	} else {
		r = request(root, "resource.dispose", "main", r.Revision, map[string]any{"resourceId": "loan", "disposition": "keep", "reason": "retain unique fixture evidence", "until": "fixture acceptance"})
		if !r.OK {
			t.Fatal(r)
		}
		path = filepath.Join(root, path)
	}
	r = request(root, "run", "worker", r.Revision, map[string]any{})
	if !r.OK {
		t.Fatal(r)
	}
	if !owned {
		r = request(root, "resource.release", "worker", r.Revision, map[string]any{"resourceId": "loan", "reason": "verified command stopped"})
		if !r.OK {
			t.Fatal(r)
		}
	}
	if closeTask {
		for _, step := range []struct {
			action, actor string
			payload       map[string]any
		}{
			{"submit", "worker", map[string]any{}},
			{"review", "main", map[string]any{"accept": true, "reason": "fixed fixture accepted"}},
			{"integrate", "main", map[string]any{"candidate": "fixture", "reason": "accepted fixture"}},
			{"resource.close", "main", map[string]any{"disposition": "keep", "reason": "retained fictional primary copy"}},
		} {
			r = request(root, step.action, step.actor, r.Revision, step.payload)
			if !r.OK {
				t.Fatal(r)
			}
		}
	}
	return root, path, r.Revision
}

func historyReplace(t *testing.T, path string) {
	t.Helper()
	if err := os.Rename(path, path+".previous"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fictional tool"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestMainBorrowedHistoryClosedReplacement(t *testing.T) {
	root, path, rev := historyLoanFixture(t, true, false)
	before := request(root, "resource.inspect", "main", 0, map[string]any{"resourceId": "loan"})
	if !before.OK || before.Data["pending"] != false {
		t.Fatal("fixture not closed", before)
	}
	historyReplace(t, path)
	r := request(root, "resource.inspect", "main", 0, map[string]any{"resourceId": "loan"})
	if !r.OK || r.Data["pending"] != false || r.Data["historicalReleased"] != true || r.Data["observedState"] != "unknown" || r.Data["nativeIdentity"] != before.Data["nativeIdentity"] {
		t.Fatal("closed loan drift blocks current work or hides observation", r)
	}
	s := request(root, "status", "main", 0, map[string]any{})
	if !s.OK || s.Revision != rev || s.Data["resourceSummary"].(map[string]any)["pending"] != 0 {
		t.Fatal("history blocks summary or observation wrote state", s)
	}
	l := request(root, "resource.list", "main", 0, map[string]any{})
	if !l.OK || l.Data["summary"].(map[string]any)["pending"] != 0 {
		t.Fatal(l)
	}
	var found bool
	for _, item := range l.Data["resources"].([]map[string]any) {
		if item["resourceId"] == "loan" {
			found = true
			if item["historicalReleased"] != true || item["observedState"] != "unknown" {
				t.Fatal(item)
			}
		}
	}
	if !found {
		t.Fatal("history disappeared")
	}
	if use := request(root, "resource.use", "worker", rev, map[string]any{"resourceId": "loan", "access": "read", "reason": "must revalidate real identity"}); use.OK {
		t.Fatal("historical observation granted stale reuse", use)
	}
}

func TestMainBorrowedHistorySafetyBoundaries(t *testing.T) {
	for _, kind := range []string{"open", "owned", "held", "active-use", "no-native-stop"} {
		t.Run(kind, func(t *testing.T) {
			root, path, rev := historyLoanFixture(t, kind != "open", kind == "owned")
			if kind == "held" {
				r := request(root, "resource.hold", "main", rev, map[string]any{"resourceId": "loan", "enabled": true, "reason": "open finding", "until": "explicit review"})
				if !r.OK {
					t.Fatal(r)
				}
			}
			if kind == "active-use" {
				r := request(root, "resource.use", "worker", rev, map[string]any{"resourceId": "loan", "access": "read", "reason": "new current use"})
				if !r.OK {
					t.Fatal(r)
				}
			}
			if kind == "no-native-stop" {
				db, e := openExisting(root)
				if e != nil {
					t.Fatal(e)
				}
				_, e = db.Exec("UPDATE resource_ledger SET observed_state='unknown' WHERE provider='supervised_process'")
				db.Close()
				if e != nil {
					t.Fatal(e)
				}
			}
			historyReplace(t, path)
			r := request(root, "resource.inspect", "main", 0, map[string]any{"resourceId": "loan"})
			if !r.OK || r.Data["pending"] != true || r.Data["historicalReleased"] == true {
				t.Fatal("unresolved/owned/live/held resource was hidden", kind, r)
			}
			s := request(root, "status", "main", 0, map[string]any{})
			if !s.OK || s.Data["resourceSummary"].(map[string]any)["pending"].(int) == 0 {
				t.Fatal("required pending disappeared", s)
			}
		})
	}
}

func TestMainBorrowedHistoryNativeCLI(t *testing.T) {
	binary := os.Getenv("OCMI_ROOT_BINARY")
	if binary == "" {
		t.Skip("set after actual native CLI build")
	}
	for _, action := range []string{"resource.inspect", "resource.list", "resource.rebind", "tool-storage.inspect", "resource.reconcile"} {
		out, err := exec.Command(binary, "help", action).CombinedOutput()
		var r Response
		if err != nil || json.Unmarshal(out, &r) != nil || !r.OK || r.Code != "HELP" {
			t.Fatal("missing public action help", action, err, string(out))
		}
		if action == "resource.reconcile" {
			if r.Data["supported"] != true || r.Data["supportedScope"] != "original-managed-crash-workspace-only" || r.Data["genericProviderSupported"] != false {
				t.Fatal("reconciliation help must distinguish qualified workspace recovery from unsupported generic providers", r)
			}
		} else if _, ok := r.Data["example"].(map[string]any); !ok {
			t.Fatal("missing actionable payload", action, r)
		}
	}
	root, path, _ := historyLoanFixture(t, true, false)
	historyReplace(t, path)
	out, err := exec.Command(binary, "call", "--root", root, "--actor", "main", "--action", "resource.inspect", "--payload", `{"resourceId":"loan"}`).CombinedOutput()
	var r Response
	if err != nil || json.Unmarshal(out, &r) != nil || !r.OK || r.Data["pending"] != false || r.Data["historicalReleased"] != true || r.Data["observedState"] != "unknown" {
		t.Fatal("native historical-loan consumer", err, string(out))
	}
}
