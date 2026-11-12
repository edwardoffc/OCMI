package internal

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the installed executable, never a test-only renderer or wrapper.
func TestMainCompactTextNativeCLI(t *testing.T) {
	bin := os.Getenv("OCMI_ROOT_BINARY")
	if bin == "" {
		t.Skip("actual packaged core required")
	}
	root, rev := compactFixture(t)
	before := request(root, "status", "main", 0, map[string]any{})
	whole := "first \"quoted\" line\n\tsecond \\ path 🙂\n" + strings.Repeat("original-line\n", 12)
	if e := os.WriteFile(filepath.Join(root, "note.txt"), []byte(whole), 0600); e != nil {
		t.Fatal(e)
	}
	invoke := func(actor string, payload map[string]any, text bool, more ...string) ([]byte, error) {
		a := []string{"call", "--root", root, "--actor", actor, "--action", "context.inspect", "--payload", string(workflowJSON(t, payload))}
		if text {
			a = append(a, "--format", "text")
		}
		return exec.Command(bin, append(a, more...)...).CombinedOutput()
	}
	p := compactPayload()
	p["requiredFiles"] = []string{"input.txt", "note.txt"}
	raw, e := invoke("worker", p, false)
	var canonical Response
	if e != nil || json.Unmarshal(raw, &canonical) != nil || !canonical.OK || canonical.Revision != rev {
		t.Fatal("JSON premise", e, string(raw))
	}
	text, e := invoke("worker", p, true)
	if e != nil || json.Valid(text) || !bytes.Contains(text, []byte(canonical.Data["context"].(string))) || !bytes.Contains(text, []byte(canonical.Data["receipt"].(string))) {
		t.Fatal("actual complete text/receipt missing", e, string(text))
	}
	for _, field := range []string{"verified", "complete", "ready", "false"} {
		if !strings.Contains(strings.ToLower(string(text)), field) {
			t.Fatal("missing text state/freshness", field, string(text))
		}
	}
	if len(text) >= len(raw) || bytes.Contains(text, []byte("CONFIG-COMPACT-PRIVATE-CANARY")) || bytes.Contains(text, []byte("--ocmi-cli-test-helper")) {
		t.Fatal("not smaller or leaked source", string(text))
	}
	p["receipt"] = canonical.Data["receipt"]
	text, e = invoke("worker", p, true)
	if e != nil || bytes.Contains(text, []byte("RULE-COMPACT-CANARY")) || !bytes.Contains(text, []byte("rules.md")) || !strings.Contains(strings.ToLower(string(text)), "retained") || !bytes.Contains(text, []byte("SEMANTIC-COMPACT-CANARY")) || !bytes.Contains(text, []byte(whole)) {
		t.Fatal("retained text lost semantics", e, string(text))
	}
	if e = os.WriteFile(filepath.Join(root, "rules.md"), []byte("CHANGED-TEXT-RULE"), 0600); e != nil {
		t.Fatal(e)
	}
	text, e = invoke("worker", p, true)
	if e != nil || !bytes.Contains(text, []byte("CHANGED-TEXT-RULE")) || bytes.Contains(text, []byte(canonical.Data["receipt"].(string))) {
		t.Fatal("changed rule/receipt not delivered", e, string(text))
	}
	startPayload := workflowStartPayload()
	start := request(root, "context.inspect", "main", rev, startPayload)
	if !start.OK {
		t.Fatal(start)
	}
	text, e = invoke("main", startPayload, true)
	if e != nil || strings.TrimSpace(string(text)) != strings.TrimSpace(start.Data["start"].(string)) {
		t.Fatal("text START differs from core", e, string(text))
	}
	if strings.Count(string(text), `"criteria.md"`) != 1 || !bytes.Contains(text, []byte(start.Data["bindingRef"].(string))) {
		t.Fatal("duplicated semantic reference or lost binding", string(text))
	}
	p["expectedBinding"] = start.Data["bindingRef"]
	text, e = invoke("worker", p, true, "--request-id", "--format")
	if e != nil || !bytes.Contains(text, []byte("SEMANTIC-COMPACT-CANARY")) || (!strings.Contains(strings.ToLower(string(text)), "matched") && !bytes.Contains(text, []byte(start.Data["bindingRef"].(string)))) {
		t.Fatal("format flag confused with value/binding", e, string(text))
	}
	stdin := Request{SchemaVersion: 1, RequestID: "text-stdin-control", ProjectRoot: root, Actor: "worker", Action: "context.inspect", Payload: workflowJSON(t, p)}
	jsonCmd := exec.Command(bin, "request")
	jsonCmd.Stdin = bytes.NewReader(workflowJSON(t, stdin))
	control, controlErr := jsonCmd.CombinedOutput()
	var stdinControl Response
	if controlErr != nil || json.Unmarshal(control, &stdinControl) != nil || !stdinControl.OK {
		t.Fatal("stdin JSON premise", controlErr, string(control))
	}
	cmd := exec.Command(bin, "request", "--format", "text")
	cmd.Stdin = bytes.NewReader(workflowJSON(t, stdin))
	text, e = cmd.CombinedOutput()
	if e != nil || !bytes.Contains(text, []byte(whole)) || json.Valid(text) {
		t.Fatal("request text route", e, string(text))
	}
	for _, bad := range []map[string]any{
		{"mode": "compact", "requiredFiles": []string{".gitignore"}},
		{"mode": "compact", "requiredFiles": []string{"../outside.txt"}},
		{"mode": "compact", "expectedBinding": strings.Repeat("0", 64)},
		{"mode": "compact", "requiredFiles": []string{"note.txt"}, "budgetBytes": 128},
	} {
		out, err := invoke("worker", bad, true)
		var refusal Response
		if err == nil || json.Unmarshal(out, &refusal) != nil || refusal.OK || bytes.Contains(out, []byte("CANARY")) {
			t.Fatal("text hid refusal or disclosed originals", err, string(out))
		}
	}
	after := request(root, "status", "main", 0, map[string]any{})
	if !bytes.Equal(workflowJSON(t, before), workflowJSON(t, after)) {
		t.Fatal("text observations mutated state")
	}
	stop := request(root, "stop", "main", rev, map[string]any{"enabled": true})
	if !stop.OK {
		t.Fatal(stop)
	}
	text, e = invoke("worker", p, true)
	var refused Response
	if e == nil || json.Unmarshal(text, &refused) != nil || refused.OK {
		t.Fatal("expected binding bypassed STOP", e, string(text))
	}
	delete(p, "expectedBinding")
	text, e = invoke("worker", p, true)
	if e != nil || !strings.Contains(strings.ToLower(string(text)), "stop") || !strings.Contains(strings.ToLower(string(text)), "true") {
		t.Fatal("observation concealed STOP", e, string(text))
	}
	// C1 is valid UTF-8 accepted by the canonical context reader, but must
	// never reach a terminal raw. ESC is already refused by that reader.
	unsafe := "prefix\u009b2Jterminal control\n"
	if e = os.WriteFile(filepath.Join(root, "note.txt"), []byte(unsafe), 0600); e != nil {
		t.Fatal(e)
	}
	control, controlErr = invoke("worker", p, false)
	if controlErr != nil || json.Unmarshal(control, &stdinControl) != nil || !stdinControl.OK || !strings.Contains(stdinControl.Data["context"].(string), unsafe) {
		t.Fatal("canonical UTF-8 control premise", controlErr, string(control))
	}
	text, e = invoke("worker", p, true)
	var fallback Response
	if e != nil || bytes.ContainsRune(text, '\x1b') || bytes.ContainsRune(text, '\u009b') || json.Unmarshal(text, &fallback) != nil || !fallback.OK || !strings.Contains(fallback.Data["context"].(string), unsafe) {
		t.Fatal("unsafe terminal bytes or incomplete fallback", e, string(text))
	}
}

func TestMainCompactTextNoEffectsNativeCLI(t *testing.T) {
	bin := os.Getenv("OCMI_ROOT_BINARY")
	if bin == "" {
		t.Skip("actual packaged core required")
	}
	initPayload := string(workflowJSON(t, map[string]any{"projectId": "p", "deviceId": "d", "copyId": "c", "owner": "main"}))
	controlRoot := t.TempDir()
	control, err := exec.Command(bin, "call", "--root", controlRoot, "--actor", "main", "--action", "init", "--revision", "0", "--payload", initPayload).CombinedOutput()
	var positive Response
	if err != nil || json.Unmarshal(control, &positive) != nil || !positive.OK {
		t.Fatal("valid init premise", err, string(control))
	}
	for _, args := range [][]string{
		{"--action", "init", "--revision", "0", "--payload", initPayload, "--format", "text"},
		{"--action", "init", "--revision", "0", "--payload", initPayload, "--format", "yaml"},
		{"--action", "context.inspect", "--payload", `{"mode":"compact"}`, "--format", "text", "--format", "text"},
		{"--action", "status", "--format", "text"},
	} {
		root := t.TempDir()
		out, e := exec.Command(bin, append([]string{"call", "--root", root, "--actor", "main"}, args...)...).CombinedOutput()
		var r Response
		if e == nil || json.Unmarshal(out, &r) != nil || r.OK || r.Code != "USAGE" {
			t.Fatal("unsupported format accepted", e, string(out))
		}
		if _, e = os.Lstat(filepath.Join(root, ".ocmi")); !os.IsNotExist(e) {
			t.Fatal("format refusal had effects", e)
		}
	}
	root := t.TempDir()
	cmd := exec.Command(bin, "request", "--format", "text")
	cmd.Stdin = bytes.NewReader(workflowJSON(t, Request{SchemaVersion: 1, RequestID: "text-no-effects-control", ProjectRoot: root, Actor: "main", Action: "init", Payload: json.RawMessage(initPayload)}))
	out, e := cmd.CombinedOutput()
	var r Response
	if e == nil || json.Unmarshal(out, &r) != nil || r.OK || r.Code != "USAGE" {
		t.Fatal("stdin mutation executed", e, string(out))
	}
	if _, e = os.Lstat(filepath.Join(root, ".ocmi")); !os.IsNotExist(e) {
		t.Fatal("stdin format refusal had effects", e)
	}
}
