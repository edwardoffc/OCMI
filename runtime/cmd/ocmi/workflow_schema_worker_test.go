package main

import "testing"

func TestWorkerWorkflowSchemaOptionalProfile(t *testing.T) {
	bound := help([]string{"task.bind"})
	command := bound.Data["example"].(map[string]any)["command"].(map[string]any)
	if _, exists := command["toolStorage"]; exists {
		t.Fatal("default command unexpectedly opts into a profile")
	}
	for _, action := range []string{"task.bind", "task.revise"} {
		r := help([]string{action})
		if r.Data["toolStorageGuidance"] == nil || r.Data["optionalCommandFields"] == nil {
			t.Fatal("optional profile lacks authority guidance", action)
		}
	}
	start := help([]string{"context.inspect"}).Data["startExample"].(map[string]any)
	if len(start) != 4 || start["mode"] != "start" || start["taskRevision"] != 1 {
		t.Fatal("start payload contains implicit or unsupported fields", start)
	}
}
