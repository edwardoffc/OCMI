package internal

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Consume the published schema with a fictional approved receipt, then execute
// the ordinary registered checker with the actual packaged/native executable.
func TestMainWorkflowProfileNativeCLI(t *testing.T) {
	bin := os.Getenv("OCMI_ROOT_BINARY")
	if bin == "" {
		t.Skip("actual compiled consumer supplied after build")
	}
	out, e := exec.Command(bin, "help", "task.bind").CombinedOutput()
	var help Response
	if e != nil || json.Unmarshal(out, &help) != nil || !help.OK {
		t.Fatal(e, string(out))
	}
	fields, ok := help.Data["optionalCommandFields"].(map[string]any)
	if !ok {
		t.Fatal(help)
	}
	shape, ok := fields["toolStorage"].(map[string]any)
	if !ok || len(shape) != 3 {
		t.Fatal(help)
	}
	f := mainToolSetup(t)
	approved := f.ref(t)
	receipt := map[string]any{"home": approved["home"], "toolId": approved["toolId"], "profileSha256": approved["sha256"]}
	ref := map[string]any{}
	for key := range shape {
		value := receipt[key]
		if key == "sha256" {
			value = receipt["profileSha256"]
		}
		if value == nil {
			t.Fatal("published key cannot be consumed", key)
		}
		ref[key] = value
	}
	root, r := f.project(t, "schema-consumer", ref, true)
	r = mainToolReady(t, root, r)
	payload := `{"mode":"start","taskId":"tool-task","taskRevision":1,"worker":"worker"}`
	out, e = exec.Command(bin, "call", "--root", root, "--actor", "main", "--action", "context.inspect", "--payload", payload).CombinedOutput()
	var result Response
	if e != nil || json.Unmarshal(out, &result) != nil || !result.OK || result.Data["start"] == nil {
		t.Fatal("profile-bound ready view", e, string(out))
	}
	requestRaw := workflowJSON(t, map[string]any{"schemaVersion": 1, "action": "run", "projectRoot": root, "actor": "worker", "requestId": "schema-native-run", "expectedRevision": r.Revision, "payload": map[string]any{}})
	cmd := exec.Command(bin, "request")
	cmd.Stdin = strings.NewReader(string(requestRaw))
	out, e = cmd.CombinedOutput()
	if e != nil || json.Unmarshal(out, &result) != nil || !result.OK || result.Data["operationState"] != "succeeded" {
		t.Fatal("published reference run failed", e, string(out))
	}
	observed := mainToolObserved(t, root)
	if observed["cache"] != f.cache || observed["scratch"] != observed["tmp"] {
		t.Fatal("published reference didn't apply to child", observed)
	}
}
