package internal

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func compactFixture(t *testing.T) (string, int64) {
	t.Helper()
	root := t.TempDir()
	r := request(root, "init", "main", 0, map[string]any{"projectId": "project", "deviceId": "device", "copyId": "copy", "owner": "main"})
	if !r.OK {
		t.Fatal(r)
	}
	for name, body := range map[string]string{"criteria.md": "SEMANTIC-COMPACT-CANARY", "input.txt": "BOUND-COMPACT-CANARY", ".gitignore": "CONFIG-COMPACT-PRIVATE-CANARY", "rules.md": "RULE-COMPACT-CANARY"} {
		if e := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	r = request(root, "task.bind", "main", r.Revision, map[string]any{"taskId": "small", "taskRevision": 1, "outcomeId": "outcome", "completionScope": "target", "worker": "worker", "reviewer": "main", "contract": "criteria.md", "inputs": []string{"input.txt", ".gitignore"}, "command": map[string]any{"id": "bounded-check", "executable": os.Args[0], "args": []string{"--ocmi-cli-test-helper", "0", root, "0"}, "cwd": ".", "timeoutMs": 5000, "outputLimitBytes": 1024}})
	if !r.OK {
		t.Fatal(r)
	}
	r = request(root, "task.ready", "main", r.Revision, map[string]any{})
	if !r.OK {
		t.Fatal(r)
	}
	return root, r.Revision
}

func compactPayload() map[string]any {
	return map[string]any{"mode": "compact", "files": []string{"rules.md"}, "requiredFiles": []string{"input.txt"}, "budgetBytes": 4096}
}

func TestMainCompactPremise(t *testing.T) {
	root, rev := compactFixture(t)
	if r := request(root, "context.inspect", "main", rev, map[string]any{"mode": "summary"}); !r.OK {
		t.Fatal(r)
	}
	if r := request(root, "context.inspect", "main", rev, map[string]any{"mode": "selected", "requiredFiles": []string{"input.txt"}}); r.OK {
		t.Fatal("old hidden disclosure boundary changed", r)
	}
	plain, rev := workflowFixture(t)
	if r := request(plain, "context.inspect", "main", rev, map[string]any{"mode": "selected", "requiredFiles": []string{"input.txt"}}); !r.OK {
		t.Fatal("ordinary selected control", r)
	}
}

func TestMainCompactReceipt(t *testing.T) {
	root, rev := compactFixture(t)
	before := request(root, "status", "main", 0, map[string]any{})
	p := compactPayload()
	r := request(root, "context.inspect", "worker", rev, p)
	if !r.OK || r.Code != "CONTEXT_COMPACT" || r.Data["complete"] != true || r.Data["inputFreshness"] != "verified" {
		t.Fatal("compact retrieval", r)
	}
	text, _ := r.Data["context"].(string)
	for _, s := range []string{"SEMANTIC-COMPACT-CANARY", "BOUND-COMPACT-CANARY", "RULE-COMPACT-CANARY"} {
		if !strings.Contains(text, s) {
			t.Fatal("missing whole original", s, r)
		}
	}
	if strings.Contains(string(workflowJSON(t, r)), "CONFIG-COMPACT-PRIVATE-CANARY") || strings.Contains(text, "--ocmi-cli-test-helper") {
		t.Fatal("raw configuration/argv disclosure", r)
	}
	receipt, _ := r.Data["receipt"].(string)
	decoded, e := base64.RawURLEncoding.DecodeString(receipt)
	if e != nil || !json.Valid(decoded) || len(receipt) > 8192 || len(decoded) > 6144 || strings.Contains(string(decoded), "CANARY") {
		t.Fatal("not a bounded metadata-only receipt", e)
	}
	p["receipt"] = receipt
	r = request(root, "context.inspect", "worker", rev, p)
	text, _ = r.Data["context"].(string)
	if !r.OK || strings.Contains(text, "RULE-COMPACT-CANARY") || !strings.Contains(text, "SEMANTIC-COMPACT-CANARY") || !strings.Contains(text, "BOUND-COMPACT-CANARY") || r.Data["receiptUnchanged"] != true || r.Data["receipt"] != nil || !strings.Contains(string(workflowJSON(t, r.Data["retainedFiles"])), "rules.md") {
		t.Fatal("no useful retained-context reuse", r)
	}
	if e = os.WriteFile(filepath.Join(root, "rules.md"), []byte("CHANGED-RULE-CANARY"), 0600); e != nil {
		t.Fatal(e)
	}
	r = request(root, "context.inspect", "worker", rev, p)
	if !r.OK || !strings.Contains(r.Data["context"].(string), "CHANGED-RULE-CANARY") || r.Data["receipt"] == receipt {
		t.Fatal("changed rule was hidden", r)
	}
	after := request(root, "status", "main", 0, map[string]any{})
	if string(workflowJSON(t, before)) != string(workflowJSON(t, after)) {
		t.Fatal("view mutated state")
	}
}

func TestMainCompactBindingAndScope(t *testing.T) {
	root, rev := compactFixture(t)
	start := request(root, "context.inspect", "main", rev, workflowStartPayload())
	ref, _ := start.Data["bindingRef"].(string)
	if !start.OK || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(ref) || len([]byte(start.Data["start"].(string))) > 900 {
		t.Fatal("no thin pinned START", start)
	}
	p := compactPayload()
	p["expectedBinding"] = ref
	if r := request(root, "context.inspect", "worker", rev, p); !r.OK || r.Data["bindingRef"] != ref {
		t.Fatal("delivered binding failed", r)
	}
	for _, kind := range []string{"foreign-actor", "wrong-reference", "private", "config-text", "bad-receipt", "budget"} {
		t.Run(kind, func(t *testing.T) {
			p := compactPayload()
			actor := "worker"
			switch kind {
			case "foreign-actor":
				actor = "other"
			case "wrong-reference":
				p["expectedBinding"] = strings.Repeat("0", 64)
			case "private":
				p["files"] = []string{".env"}
			case "config-text":
				p["requiredFiles"] = []string{".gitignore"}
			case "bad-receipt":
				p["receipt"] = "invalid!!"
			case "budget":
				p["budgetBytes"] = 128
				p["files"] = []string{"rules.md", "input.txt"}
			}
			if kind == "budget" {
				if e := os.WriteFile(filepath.Join(root, "rules.md"), []byte(strings.Repeat("safe rule ", 100)), 0600); e != nil {
					t.Fatal(e)
				}
			}
			r := request(root, "context.inspect", actor, rev, p)
			if r.OK || r.Data["context"] != nil || strings.Contains(string(workflowJSON(t, r)), "CANARY") {
				t.Fatal("scope/freshness guard escaped", r)
			}
		})
	}
	r := request(root, "stop", "main", rev, map[string]any{"enabled": true})
	p = compactPayload()
	p["expectedBinding"] = ref
	if r = request(root, "context.inspect", "worker", r.Revision, p); r.OK {
		t.Fatal("START proof ignored STOP", r)
	}
}

func TestMainCompactNativeCLI(t *testing.T) {
	bin := os.Getenv("OCMI_ROOT_BINARY")
	if bin == "" {
		t.Skip("actual native core supplied after build")
	}
	root, rev := compactFixture(t)
	invoke := func(p map[string]any) Response {
		out, e := exec.Command(bin, "call", "--root", root, "--actor", "worker", "--action", "context.inspect", "--payload", string(workflowJSON(t, p))).CombinedOutput()
		var r Response
		if e != nil || json.Unmarshal(out, &r) != nil || !r.OK || r.Revision != rev {
			t.Fatal("actual compact CLI", e, string(out))
		}
		return r
	}
	p := compactPayload()
	first := invoke(p)
	p["receipt"] = first.Data["receipt"]
	second := invoke(p)
	if second.Data["receiptUnchanged"] != true || strings.Contains(second.Data["context"].(string), "RULE-COMPACT-CANARY") {
		t.Fatal("CLI receipt did not survive separate processes", second)
	}
	out, e := exec.Command(bin, "help", "context.inspect").CombinedOutput()
	var help Response
	if e != nil || json.Unmarshal(out, &help) != nil || help.Data["compactExample"] == nil {
		t.Fatal("missing discoverable compact schema", e, string(out))
	}
}
