//go:build windows

package recoveryprobe

import "testing"

func TestUnixGatePremiseScope(t *testing.T) {
	t.Skip("Unix gate/PGID premise; existing spike TestWindowsJobClosesOnSupervisorExit qualifies Windows Job ownership")
}
