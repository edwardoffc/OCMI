//go:build windows

package internal

import (
	"errors"
	"testing"
)

func TestWindowsObserverRejectsInvalidRecordedPID(t *testing.T) {
	live, err := windowsProcessLive(0)
	if err == nil || live {
		t.Fatalf("invalid PID was treated as absence: live=%v err=%v", live, err)
	}
}

func TestWindowsNativeErrorsNeverProveAbsence(t *testing.T) {
	if windowsAbsenceProven(false, errors.New("synthetic native denial"), false, nil) {
		t.Fatal("native lookup error was treated as absence")
	}
}
