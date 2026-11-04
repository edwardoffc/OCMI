//go:build !windows

package internal

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"
)

// Regression at the real native process boundary: registration must not depend
// on a short-lived group leader still being queryable after successful Start.
func TestUnixQuickExitRecordsOwnedTerminalResult(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name string
		exit int
	}{{"true", 0}, {"false", 1}} {
		executable, err := exec.LookPath(tc.name)
		if err != nil {
			t.Fatal(err)
		}
		for attempt := 0; attempt < 64; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			calls, pid, group := 0, 0, 0
			result, runErr := runOwned(ctx, commandSpec{
				Executable: executable, Cwd: root, OutputLimit: 32,
				Registered: func(childPID, processGroup int) error {
					calls++
					pid, group = childPID, processGroup
					if childPID <= 1 || processGroup != childPID {
						return fmt.Errorf("invalid owned identity")
					}
					return nil
				},
			})
			cancel()
			if calls != 1 || pid <= 1 || group != pid || result.Unresolved || result.Pending || result.TimedOut || result.ExitCode != tc.exit || (runErr == nil) != (tc.exit == 0) {
				t.Fatalf("%s attempt%d: result=%+v err=%v calls=%d pid=%d group=%d", tc.name, attempt, result, runErr, calls, pid, group)
			}
		}
	}
}
