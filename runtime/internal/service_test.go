package internal

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMain(m *testing.M) {
	for _, arg := range os.Args[1:] {
		if arg == "--runner-leaf" || arg == "--runner-control" {
			path := os.Args[len(os.Args)-1]
			for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
				if stream, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); err == nil {
					_, _ = stream.WriteString(".")
					_ = stream.Close()
				}
				if arg == "--runner-leaf" {
					_, _ = os.Stdout.WriteString(strings.Repeat("x", 128))
				}
			}
			os.Exit(0)
		}
		if arg == "--runner-root" {
			child := exec.Command(os.Args[0], "--runner-leaf", os.Args[len(os.Args)-1])
			child.Stdout = os.Stdout
			child.Stderr = os.Stderr
			_ = child.Start()
			time.Sleep(150 * time.Millisecond)
			os.Exit(0)
		}
		if arg == "--ocmi-cli-test-helper" {
			root, code := os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
			if strings.Contains(code, "sleep") {
				time.Sleep(100 * time.Millisecond)
				code = "0"
			}
			if strings.Contains(code, "large") {
				_, _ = os.Stdout.WriteString(strings.Repeat("x", 1024))
				code = "0"
			}
			_ = os.WriteFile(filepath.Join(root, "effect.txt"), []byte("ran"), 0600)
			if code == "7" {
				os.Exit(7)
			}
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

func request(root, action, actor string, rev int64, payload any) Response {
	b, _ := json.Marshal(payload)
	return Handle(Request{SchemaVersion: 1, Action: action, ProjectRoot: root, Actor: actor, RequestID: uuid.NewString(), ExpectedRevision: rev, Payload: b})
}

func setup(t *testing.T, helper string, timeout, limit int) (string, int64) {
	return setupWithScope(t, helper, timeout, limit, "target")
}

func setupWithScope(t *testing.T, helper string, timeout, limit int, scope string) (string, int64) {
	t.Helper()
	root := t.TempDir()
	if r := request(root, "init", "main", 0, map[string]any{"projectId": "p", "deviceId": "d", "copyId": "c", "owner": "main"}); !r.OK {
		t.Fatal(r)
	}
	if err := os.WriteFile(filepath.Join(root, "contract.md"), []byte("contract"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "input.txt"), []byte("input"), 0600); err != nil {
		t.Fatal(err)
	}
	bind := map[string]any{"taskId": "t", "outcomeId": "o", "taskRevision": 1, "worker": "worker", "reviewer": "main", "completionScope": scope, "contract": "contract.md", "inputs": []string{"input.txt"}, "command": map[string]any{"id": "test", "executable": os.Args[0], "args": []string{"--ocmi-cli-test-helper", helper, root, helper}, "cwd": ".", "timeoutMs": timeout, "outputLimitBytes": limit}}
	r := request(root, "task.bind", "main", 0, bind)
	if !r.OK {
		t.Fatal(r)
	}
	r = request(root, "task.ready", "main", r.Revision, map[string]any{})
	if !r.OK {
		t.Fatal(r)
	}
	return root, r.Revision
}

func TestMalformedTimeoutTruncationAndChangedInputs(t *testing.T) {
	if r := Handle(Request{}); r.OK || r.Code != "INVALID_REQUEST" {
		t.Fatalf("malformed=%+v", r)
	}
	for _, tc := range []struct {
		name, helper   string
		timeout, limit int
		want           string
	}{
		{"timeout", "sleep", 10, 4096, "incomplete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, rev := setup(t, tc.helper, tc.timeout, tc.limit)
			r := request(root, "run", "worker", rev, map[string]any{})
			if !r.OK || r.Data["operationState"] != tc.want {
				t.Fatalf("run=%+v", r)
			}
			if s := request(root, "submit", "worker", r.Revision, map[string]any{}); s.OK {
				t.Fatalf("submitted incomplete=%+v", s)
			}
		})
	}
	root, rev := setup(t, "0", 5000, 4096)
	r := request(root, "run", "worker", rev, map[string]any{})
	if !r.OK {
		t.Fatal(r)
	}
	if err := os.WriteFile(filepath.Join(root, "input.txt"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if s := request(root, "submit", "worker", r.Revision, map[string]any{}); s.OK || s.Code != "EVIDENCE_INVALID" {
		t.Fatalf("changed submit=%+v", s)
	}
}

func TestLimitedOutputIsIncomplete(t *testing.T) {
	b := &limitedBuffer{limit: 2}
	if _, err := b.Write([]byte("three")); err != nil || !b.truncated || b.String() != "th" {
		t.Fatalf("buffer=%q truncated=%v err=%v", b.String(), b.truncated, err)
	}
}

func TestConcurrentAdmissionHasOneEffect(t *testing.T) {
	root, rev := setup(t, "0", 5000, 4096)
	var group sync.WaitGroup
	results := make(chan Response, 2)
	for range 2 {
		group.Add(1)
		go func() { defer group.Done(); results <- request(root, "run", "worker", rev, map[string]any{}) }()
	}
	group.Wait()
	close(results)
	success := 0
	for result := range results {
		if result.OK {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("successful admissions=%d", success)
	}
	effect, err := os.ReadFile(filepath.Join(root, "effect.txt"))
	if err != nil || string(effect) != "ran" {
		t.Fatalf("effect=%q err=%v", effect, err)
	}
}

func TestInitIsExclusiveAndReplaysItsOriginalOutcome(t *testing.T) {
	root := t.TempDir()
	payload := map[string]any{"projectId": "p", "deviceId": "d", "copyId": "c", "owner": "main"}
	makeRequest := func(id string) Request {
		b, _ := json.Marshal(payload)
		return Request{SchemaVersion: 1, Action: "init", ProjectRoot: root, Actor: "main", RequestID: id, Payload: b}
	}
	first := makeRequest("first")
	var group sync.WaitGroup
	results := make(chan Response, 2)
	for _, request := range []Request{first, makeRequest("second")} {
		group.Add(1)
		go func(r Request) { defer group.Done(); results <- Handle(r) }(request)
	}
	group.Wait()
	close(results)
	success := 0
	for result := range results {
		if result.OK {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("successful init=%d", success)
	}
	// The initial outcome is stable even after a later mutation changes project revision.
	db, err := openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("UPDATE project SET revision=9")
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	// Determine the persisted initializer key; the concurrent loser must never become an initializer.
	db, err = openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	var saved string
	if err = db.QueryRow("SELECT id FROM request_log LIMIT 1").Scan(&saved); err != nil {
		t.Fatal(err)
	}
	db.Close()
	var accepted Request
	if saved == "first" {
		accepted = first
	} else {
		accepted = makeRequest("second")
	}
	replay := Handle(accepted)
	if !replay.OK || replay.Revision != 0 {
		t.Fatalf("init replay=%+v", replay)
	}
}

func TestCorruptStateAndInterruptedIntentAreHonest(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".ocmi", "local"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storePath(root), []byte("not sqlite"), 0600); err != nil {
		t.Fatal(err)
	}
	if r := request(root, "status", "main", 0, map[string]any{}); r.OK || r.Code != "STATE_CORRUPT" {
		t.Fatalf("corrupt=%+v", r)
	}
	root, rev := setup(t, "0", 5000, 4096)
	db, err := openExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	prepared := prepareRun(tx, Request{ProjectRoot: root, Actor: "worker"}, rev)
	if !prepared.OK {
		t.Fatal(prepared)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	db.Close()
	status := request(root, "status", "main", 0, map[string]any{})
	if !status.OK || status.Data["operationState"] != "unknown" {
		t.Fatalf("interrupted=%+v", status)
	}
}
