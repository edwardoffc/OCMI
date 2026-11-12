//go:build !windows

package internal

import (
	"fmt"
	"syscall"
	"testing"
)

func TestUnixObservationErrorClasses(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{nil, "none"},
		{fmt.Errorf("wrapped: %w", syscall.ESRCH), "not_found"},
		{fmt.Errorf("wrapped: %w", syscall.EPERM), "permission"},
		{fmt.Errorf("wrapped: %w", syscall.EACCES), "permission"},
		{fmt.Errorf("other"), "other"},
	} {
		if got := unixObservationErrorClass(tc.err); got != tc.want {
			t.Fatalf("class(%v)=%q, want %q", tc.err, got, tc.want)
		}
	}
}

func TestNativeErrnoExtractionAndResponseBounds(t *testing.T) {
	if value := unixNativeErrno(fmt.Errorf("wrapped: %w", syscall.EIO)); value == nil || *value != int(syscall.EIO) {
		t.Fatalf("EIO extraction=%v", value)
	}
	if value := unixNativeErrno(fmt.Errorf("non-native")); value != nil {
		t.Fatalf("non-native errno=%v", *value)
	}
}

func TestOutputPumpObservationDoesNotConsumeCompletion(t *testing.T) {
	state := &outputPumpState{}
	if got := state.observation(); got != "open" {
		t.Fatalf("before completion=%q", got)
	}
	state.finish(nil)
	if got := state.observation(); got != "eof" {
		t.Fatalf("successful completion=%q", got)
	}
	state = &outputPumpState{}
	state.finish(fmt.Errorf("fixture"))
	if got := state.observation(); got != "error" {
		t.Fatalf("failed completion=%q", got)
	}
}
