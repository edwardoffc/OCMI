package main

import "testing"

func TestWorkerHistoryHelp(t *testing.T) {
	for _, action := range []string{"resource.inspect", "resource.list", "resource.rebind", "tool-storage.inspect"} {
		r := help([]string{action})
		if !r.OK || r.Data["example"] == nil || r.Data["role"] == nil || r.Data["examples"] != nil {
			t.Fatal(r)
		}
	}
	r := help([]string{"resource.reconcile"})
	if !r.OK || r.Data["supported"] != true || r.Data["supportedScope"] != "original-managed-crash-workspace-only" || r.Data["genericProviderSupported"] != false || r.Data["example"] == nil {
		t.Fatal(r)
	}
}
