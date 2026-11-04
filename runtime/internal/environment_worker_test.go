package internal

import (
	"os/exec"
	"testing"
)

func TestEnvironmentProbeUsesInstalledGoVersionCommand(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	op, err := newEnvironmentOperation(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer op.cancel()
	version, err := op.run(goTool, "go")
	if err != nil || version == "" {
		t.Fatalf("installed Go version probe: version=%q err=%v", version, err)
	}
}
