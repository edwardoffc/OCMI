package internal

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

// These are controlled failures, not simulated provider success qualification.
func TestToolStorageProbeDiagnosticFailureDistinction(t *testing.T) {
	canary := "PRIVATE-PROBE-ERROR-OUTPUT-CANARY"
	for _, tc := range []struct {
		name  string
		run   commandResult
		err   error
		field string
		value any
	}{
		{"exit", commandResult{ExitCode: 7, StoppedVerified: true}, nil, "exitCode", 7},
		{"timeout", commandResult{TimedOut: true, Pending: true, StoppedVerified: true}, errors.New(canary), "timedOut", true},
		{"lost-stop", commandResult{Unresolved: true}, nil, "unresolved", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := mainAuthorSetup(t)
			options := mainAuthorOptions(f, "go")
			// Only identity admission is real here; no external tool is launched.
			options["executable"] = os.Args[0]
			plan := mainAuthorPrepare(t, f, options)
			previous := environmentRunOverride
			defer func() { environmentRunOverride = previous }()
			calls := 0
			environmentRunOverride = func(_ context.Context, spec commandSpec) (commandResult, error) {
				calls++
				if spec.Registered == nil {
					t.Fatal("registration callback missing")
				}
				if err := spec.Registered(123, 0); err != nil {
					t.Fatal("fixture registration failed")
				}
				if len(spec.Args) > 0 && spec.Args[0] == "telemetry" {
					return commandResult{StoppedVerified: true}, nil
				}
				r := tc.run
				r.Output = canary
				return r, tc.err
			}
			result := mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, plan, "go"))
			if result.OK || result.Code != "TOOL_STORAGE_PROBE_FAILED" || result.Data["stage"] != "probe.execution" || calls != 2 {
				t.Fatalf("failure gate changed: code=%s calls=%d", result.Code, calls)
			}
			d, ok := result.Data["diagnostic"].(map[string]any)
			if !ok || d[tc.field] != tc.value || d["step"] != "execute" || d["runnerObserved"] != true || d["errorPresent"] != (tc.err != nil) || d["outputBytes"] != len(canary) {
				t.Fatal("safe failure distinction missing")
			}
			raw, err := json.Marshal(result)
			if err != nil || strings.Contains(string(raw), canary) || strings.Contains(string(raw), f.home) || strings.Contains(string(raw), f.data) {
				t.Fatal("failure response disclosed private data")
			}
		})
	}
}

func TestToolStorageProbeDiagnosticRegistrationRecordFailure(t *testing.T) {
	f := mainAuthorSetup(t)
	c, code := resolveLayout(f.home, "installation")
	if code != "" {
		t.Fatal("fixture layout unavailable")
	}
	// Match toolStorageApply's prerequisite before its direct probe call.
	// createLocalResource exclusively creates the leaf, not missing parents.
	if err := os.MkdirAll(c.Runs, 0700); err != nil {
		t.Fatal("fixture probe parent unavailable")
	}
	p := toolStoragePlan{PlanID: "11111111-1111-4111-8111-111111111111",
		toolStorageAuthoringInput: toolStorageAuthoringInput{ToolID: "go", Executable: os.Args[0]}}
	previous := environmentRunOverride
	defer func() { environmentRunOverride = previous }()
	environmentRunOverride = func(_ context.Context, spec commandSpec) (commandResult, error) {
		err := spec.Registered(123, 0)
		return commandResult{StoppedVerified: true}, err
	}
	records := 0
	err := toolStorageProbeRecorded(c, p, nil, &toolStorageTransaction{}, func() error {
		records++
		if records == 2 {
			return errors.New("PRIVATE-REGISTRATION-ERROR-CANARY")
		}
		return nil
	})
	var diagnostic *toolStorageProbeDiagnosticError
	if !errors.As(err, &diagnostic) || records != 2 || diagnostic.observation["registrationRecordFailed"] != true || diagnostic.observation["errorPresent"] != true || diagnostic.observation["step"] != "telemetry" {
		if diagnostic != nil {
			t.Logf("safe registration control records=%d diagnostic=%v", records, diagnostic.observation)
		} else {
			t.Logf("safe registration control records=%d typedDiagnostic=false", records)
		}
		t.Fatal("actual registration record failure not distinguished")
	}
	raw, marshalErr := json.Marshal(diagnostic.observation)
	if marshalErr != nil || strings.Contains(string(raw), "PRIVATE-") || strings.Contains(string(raw), f.home) {
		t.Fatal("registration observation disclosed private data")
	}
}

func TestToolStorageProbeDiagnosticLabelsAndTypedCause(t *testing.T) {
	canary := "PRIVATE-LABEL-CANARY"
	d := toolStorageProbeObservation(canary, canary, commandResult{Output: canary}, errors.New(canary), true)
	e := &toolStorageProbeDiagnosticError{cause: toolStorageProbeFailure("probe.identity"), observation: d}
	var typed toolStorageProbeFailure
	if !errors.As(e, &typed) || typed != "probe.identity" || d["registrationRecordFailed"] != true {
		t.Fatal("typed cause or registration distinction lost")
	}
	raw, err := json.Marshal(d)
	if err != nil || strings.Contains(string(raw), canary) || strings.Contains(e.Error(), canary) || d["tool"] != "unknown" || d["step"] != "unknown" {
		t.Fatal("unbounded diagnostic label")
	}
}
