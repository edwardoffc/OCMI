package internal

import "testing"

func TestRunnerRecoveryResponseNativeErrnoBounds(t *testing.T) {
	valid := 5
	response := runnerRecoveryResponse(3, "fixture-op", commandResult{Phase: "child_state", ErrorClass: "other", GroupObservation: "present", PumpObservation: "open", NativeErrno: &valid, Output: "DO_NOT_EXPORT_NATIVE_TEXT"})
	if response.Data["nativeErrno"] != valid {
		t.Fatalf("valid errno=%v", response.Data["nativeErrno"])
	}
	invalid := 4096
	response = runnerRecoveryResponse(3, "fixture-op", commandResult{NativeErrno: &invalid, Output: "DO_NOT_EXPORT_NATIVE_TEXT"})
	if response.Data["nativeErrno"] != nil {
		t.Fatalf("out-of-range errno=%v", response.Data["nativeErrno"])
	}
	if response.Data["phase"] != "unknown" || response.Data["errorClass"] != "unknown" || response.Data["groupObservation"] != "unknown" || response.Data["pumpObservation"] != "unknown" {
		t.Fatalf("unexpected response mapping=%v", response.Data)
	}
}
