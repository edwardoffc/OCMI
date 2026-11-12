package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkerWorkflowStartMetadataAndSameByteReplacement(t *testing.T) {
	root, rev := workflowFixture(t)
	db, e := openExisting(root)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	snapshot := func() string {
		var taskFields, revisions, requests string
		if e := db.QueryRow("SELECT quote(id)||quote(outcome_id)||quote(state)||quote(worker)||quote(reviewer)||quote(contract_path)||quote(inputs_json)||quote(command_json)||quote(task_revision)||quote(command_fingerprint)||quote(input_fingerprints)||quote(completion_scope) FROM task").Scan(&taskFields); e != nil {
			t.Fatal(e)
		}
		if e := db.QueryRow("SELECT coalesce(group_concat(quote(task_id)||quote(task_revision)||quote(outcome_id)||quote(command_json)||quote(input_fingerprints)), '') FROM task_revision").Scan(&revisions); e != nil {
			t.Fatal(e)
		}
		if e := db.QueryRow("SELECT coalesce(group_concat(quote(id)||quote(fingerprint)||quote(response_json)), '') FROM request_log").Scan(&requests); e != nil {
			t.Fatal(e)
		}
		return historyStored(t, db) + taskFields + revisions + requests
	}
	before := snapshot()
	var fingerprint string
	if e = db.QueryRow("SELECT command_fingerprint FROM task").Scan(&fingerprint); e != nil {
		t.Fatal(e)
	}
	r := request(root, "context.inspect", "main", rev, workflowStartPayload())
	start, _ := r.Data["start"].(string)
	if !r.OK || strings.Contains(start, fingerprint) || r.Data["bindingRef"] == "" || !strings.Contains(start, fmt.Sprintf("observed global revision %d", rev)) {
		t.Fatal(r)
	}
	if snapshot() != before {
		t.Fatal("START changed stored metadata")
	}
	path := filepath.Join(root, "input.txt")
	contextTestAfterRead = func() {
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.Rename(path, path+".old"); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
	defer func() { contextTestAfterRead = nil }()
	r = request(root, "context.inspect", "worker", rev, workflowStartPayload())
	if r.OK || r.Code != "CONTEXT_CHANGED" || r.Data["start"] != nil {
		t.Fatal(r)
	}
	if snapshot() != before {
		t.Fatal("refused START changed stored metadata")
	}
}

func TestWorkerWorkflowStartHomeScope(t *testing.T) {
	root := t.TempDir()
	if e := os.WriteFile(filepath.Join(root, homeMarker), installationLayoutJSON, 0600); e != nil {
		t.Fatal(e)
	}
	invoke := func(action string, rev int64, p any) Response {
		raw, _ := json.Marshal(p)
		return Handle(Request{SchemaVersion: 1, ProjectRoot: root, RootKind: "installation", Action: action, Actor: "main", RequestID: "start-home-" + action, ExpectedRevision: rev, Payload: raw})
	}
	r := invoke("init", 0, map[string]any{"projectId": "home-start", "deviceId": "device", "copyId": "copy", "owner": "main"})
	if !r.OK {
		t.Fatal(r)
	}
	scope := filepath.Join(root, "OCMI_Sync", "coordinator")
	if e := os.MkdirAll(scope, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(scope, "contract.md"), []byte("criteria"), 0600); e != nil {
		t.Fatal(e)
	}
	private := filepath.Join(root, "private.md")
	if e := os.WriteFile(private, []byte("PRIVATE-HOME-CANARY"), 0600); e != nil {
		t.Fatal(e)
	}
	r = invoke("task.bind", 0, map[string]any{"taskId": "home-task", "outcomeId": "home-outcome", "taskRevision": 1, "completionScope": "target", "worker": "worker", "reviewer": "main", "contract": "OCMI_Sync/coordinator/contract.md", "inputs": []string{"private.md"}, "command": map[string]any{"id": "check", "executable": os.Args[0], "args": []string{"-test.run=^$"}, "cwd": "OCMI_Sync/coordinator", "timeoutMs": 3000, "outputLimitBytes": 1024}})
	if !r.OK {
		t.Fatal("home-scope fixture bind", r)
	}
	r = invoke("task.ready", r.Revision, map[string]any{})
	if !r.OK {
		t.Fatal(r)
	}
	r = invoke("context.inspect", r.Revision, map[string]any{"mode": "start", "taskId": "home-task", "taskRevision": 1, "worker": "worker"})
	raw, _ := json.Marshal(r)
	if r.OK || r.Data["start"] != nil || strings.Contains(string(raw), "PRIVATE-HOME-CANARY") {
		t.Fatal(r)
	}
}

func TestWorkerWorkflowBoundConfigIdentityAndHome(t *testing.T) {
	for _, kind := range []string{"project", "installation"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			if kind == "installation" {
				if e := os.WriteFile(filepath.Join(root, homeMarker), installationLayoutJSON, 0600); e != nil {
					t.Fatal(e)
				}
			}
			invoke := func(action string, rev int64, p any) Response {
				raw, _ := json.Marshal(p)
				return Handle(Request{SchemaVersion: 1, ProjectRoot: root, RootKind: kind, Action: action, Actor: "main", RequestID: "bound-config-" + action, ExpectedRevision: rev, Payload: raw})
			}
			r := invoke("init", 0, map[string]any{"projectId": "config-test", "deviceId": "device", "copyId": "copy", "owner": "main"})
			if !r.OK {
				t.Fatal(r)
			}
			contract := "criteria.md"
			cwd := "."
			if kind == "installation" {
				contract = "OCMI_Sync/coordinator/criteria.md"
				cwd = "OCMI_Sync/coordinator"
			}
			if e := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(contract))), 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(root, filepath.FromSlash(contract)), []byte("criteria"), 0600); e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(root, ".gitignore")
			if e := os.WriteFile(path, []byte("SECRET-BOUND-IDENTITY\n"), 0600); e != nil {
				t.Fatal(e)
			}
			r = invoke("task.bind", r.Revision, map[string]any{"taskId": "small", "outcomeId": "outcome", "taskRevision": 1, "completionScope": "target", "worker": "worker", "reviewer": "main", "contract": contract, "inputs": []string{".gitignore"}, "command": map[string]any{"id": "check", "executable": os.Args[0], "args": []string{"-test.run=^$"}, "cwd": cwd, "timeoutMs": 3000, "outputLimitBytes": 1024}})
			if !r.OK {
				t.Fatal("bind premise", r)
			}
			r = invoke("task.ready", r.Revision, map[string]any{})
			if !r.OK {
				t.Fatal(r)
			}
			if kind == "project" {
				contextTestAfterRead = func() {
					raw, e := os.ReadFile(path)
					if e != nil {
						t.Fatal(e)
					}
					if e = os.Rename(path, path+".saved"); e != nil {
						t.Fatal(e)
					}
					if e = os.WriteFile(path, raw, 0600); e != nil {
						t.Fatal(e)
					}
				}
				defer func() { contextTestAfterRead = nil }()
			}
			r = invoke("context.inspect", r.Revision, workflowStartPayload())
			raw, _ := json.Marshal(r)
			if r.OK || r.Data["start"] != nil || strings.Contains(string(raw), "SECRET-BOUND-IDENTITY") {
				t.Fatal(r)
			}
			if kind == "project" && r.Code != "CONTEXT_CHANGED" {
				t.Fatal(r)
			}
			if kind == "installation" && r.Code != "CONTEXT_SCOPE_REFUSED" {
				t.Fatal(r)
			}
		})
	}
}

func TestWorkerWorkflowStartSourceFailures(t *testing.T) {
	for _, kind := range []string{"oversize", "utf8", "control", "race", "command"} {
		t.Run(kind, func(t *testing.T) {
			root, rev := workflowFixture(t)
			file := filepath.Join(root, "input.txt")
			switch kind {
			case "oversize":
				if e := os.WriteFile(file, []byte(strings.Repeat("x", contextFileLimit+1)), 0600); e != nil {
					t.Fatal(e)
				}
			case "utf8":
				if e := os.WriteFile(file, []byte{0xff}, 0600); e != nil {
					t.Fatal(e)
				}
			case "control":
				if e := os.WriteFile(file, []byte{'x', 0, 'y'}, 0600); e != nil {
					t.Fatal(e)
				}
			case "race":
				contextTestAfterRead = func() { _ = os.WriteFile(file, []byte("changed"), 0600) }
				defer func() { contextTestAfterRead = nil }()
			case "command":
				db, e := openExisting(root)
				if e != nil {
					t.Fatal(e)
				}
				_, e = db.Exec("UPDATE task SET command_fingerprint='changed'")
				db.Close()
				if e != nil {
					t.Fatal(e)
				}
			}
			r := request(root, "context.inspect", "worker", rev, workflowStartPayload())
			if r.OK || r.Data["start"] != nil {
				t.Fatal(r)
			}
		})
	}
}
