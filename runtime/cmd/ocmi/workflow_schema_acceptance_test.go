package main

import "testing"

func TestMainWorkflowSchema(t *testing.T) {
	for _, action := range []string{"task.bind", "task.revise"} {
		r := help([]string{action})
		if !r.OK {
			t.Fatal(r)
		}
		fields, ok := r.Data["optionalCommandFields"].(map[string]any)
		if !ok {
			t.Fatal("missing optional command schema", action, r)
		}
		ref, ok := fields["toolStorage"].(map[string]any)
		if !ok || len(ref) != 3 || ref["home"] == nil || ref["toolId"] == nil || ref["sha256"] == nil || ref["profileSha256"] != nil {
			t.Fatal("wrong strict tool reference", action, r)
		}
	}
	r := help([]string{"context.inspect"})
	example, ok := r.Data["startExample"].(map[string]any)
	if !r.OK || !ok || example["mode"] != "start" || example["taskId"] == nil || example["taskRevision"] == nil || example["worker"] == nil {
		t.Fatal("missing exact start-view payload", r)
	}
	r = help([]string{"tool-storage.inspect"})
	if !r.OK || r.Data["role"] == nil {
		t.Fatal(r)
	}
}
