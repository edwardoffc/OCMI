package main

import "testing"

func TestWorkerWorkflowSchemaCompactHelp(t *testing.T) {
	r := help([]string{"context.inspect"})
	example := r.Data["compactExample"].(map[string]any)
	if len(example) != 4 || example["mode"] != "compact" || example["budgetBytes"] != 4096 || example["files"] == nil || example["requiredFiles"] == nil {
		t.Fatal("wrong compact example", example)
	}
	optional := r.Data["optionalCompactFields"].(map[string]any)
	if len(optional) != 2 || optional["receipt"] == nil || optional["expectedBinding"] == nil || example["terms"] != nil {
		t.Fatal("wrong compact optional fields", optional)
	}
	if r.Data["startExample"] == nil || !help([]string{"task.bind"}).OK {
		t.Fatal("existing help route lost")
	}
}
