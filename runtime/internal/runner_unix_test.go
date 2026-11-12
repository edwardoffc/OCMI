//go:build !windows

package internal

import (
	"context"
	"os"
	"testing"
)

func TestUnixRunnerLimitsLargeChildOutput(t *testing.T) {
	r, err := runOwned(context.Background(), commandSpec{Executable: os.Args[0], Args: []string{"--ocmi-cli-test-helper", "large", t.TempDir(), "large"}, Cwd: ".", OutputLimit: 17})
	if err != nil || !r.Truncated || len(r.Output) != 17 {
		t.Fatalf("result=%+v err=%v", r, err)
	}
}
