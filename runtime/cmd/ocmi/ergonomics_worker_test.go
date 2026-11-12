package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWorkerErgonomicsSelectedHelp(t *testing.T) {
	for _, action := range strings.Fields(lifecycleActions) {
		r := help([]string{action})
		if !r.OK || r.Data["actions"] != nil || r.Data["examples"] != nil {
			t.Fatal(r)
		}
		if r.Data["example"] == nil && r.Data["schemaGuidance"] == nil {
			t.Fatal(r)
		}
	}
	r := help(nil)
	bind := r.Data["examples"].(map[string]any)["task.bind"].(map[string]any)
	if bind["command"].(map[string]any)["outputPolicy"] != "private" {
		t.Fatal(r)
	}
	for _, action := range []string{"task.bind", "integrate", "resource.close", "operation.output"} {
		r = help([]string{action})
		raw, _ := json.Marshal(r)
		if strings.Contains(string(raw), "REGISTERED_OPERATION_ID") && action != "operation.output" {
			t.Fatal(string(raw))
		}
	}
	if r = help([]string{"operation.output"}); r.Data["outputPolicy"] == nil {
		t.Fatal(r)
	}
}

func TestWorkerErgonomicsFlags(t *testing.T) {
	base := []string{"--root", t.TempDir(), "--actor", "owner", "--action", "submit"}
	for _, extra := range [][]string{nil, {"--revision", "-1"}, {"--revision", "0", "--revision", "1"}, {"--unknown", "x"}, {"--revision", "0", "--payload", "{"}} {
		args := append(append([]string{}, base...), extra...)
		if r := call(args); r.OK || r.Code != "USAGE" {
			t.Fatal(r)
		}
	}
	for _, args := range [][]string{nil, {"call"}, {"task.bind"}, {"operation.output"}} {
		if r := help(args); !r.OK || r.Code != "HELP" {
			t.Fatal(r)
		}
	}
	if r := help([]string{"imaginary"}); r.OK {
		t.Fatal(r)
	}
}
