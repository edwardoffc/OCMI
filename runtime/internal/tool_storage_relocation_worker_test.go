package internal

import "testing"

func TestWorkerToolStorageAuthoringRelocationPrepareReplay(t *testing.T) {
	f := mainRelocationSetup(t)
	opts := mainRelocationOptions(t, f)
	first := mainRelocationPrepare(t, f, opts)
	second := mainRelocationPrepare(t, f, opts)
	if first.Data["planId"] != second.Data["planId"] || first.Data["planSha256"] != second.Data["planSha256"] {
		t.Fatal("unchanged prepare did not reuse approved plan", first, second)
	}
}
