package internal

// Independent E2 failure controls. The two nil-default hooks only substitute
// external runner/journal IO in tests, as in the existing checkpoint engine.
import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func environmentAcceptanceRequest(t *testing.T, secondTool bool) Request {
	t.Helper()
	project, scratch := t.TempDir(), t.TempDir()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	files := []environmentSource{{Kind: "go-mod", Path: "go.mod"}}
	tools := []environmentTool{{Kind: "go", Executable: goTool}}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module fixture\ngo 1.20\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if secondTool {
		if err := os.WriteFile(filepath.Join(project, "package.json"), []byte(`{"engines":{"node":">=20"}}`), 0600); err != nil {
			t.Fatal(err)
		}
		files = append(files, environmentSource{Kind: "node-package", Path: "package.json"})
		// The runner is substituted below; this binding never runs Go as Node.
		tools = append(tools, environmentTool{Kind: "node", Executable: goTool})
	}
	raw, err := json.Marshal(environmentInput{Sources: files, Tools: tools, ScratchRoot: scratch})
	if err != nil {
		t.Fatal(err)
	}
	return Request{SchemaVersion: 1, Action: "environment.inspect", ProjectRoot: project, Actor: "main", ExpectedRevision: 0, RequestID: "synthetic-lifecycle", Payload: raw}
}

func TestEnvironmentAcceptanceUnknownStopsInspection(t *testing.T) {
	request := environmentAcceptanceRequest(t, true)
	count := 0
	previous := environmentRunOverride
	environmentRunOverride = func(ctx context.Context, spec commandSpec) (commandResult, error) {
		count++
		if spec.Registered != nil {
			if err := spec.Registered(123456, 123456); err != nil {
				return commandResult{StoppedVerified: true}, err
			}
		}
		return commandResult{Unresolved: true}, nil
	}
	t.Cleanup(func() { environmentRunOverride = previous })
	r := Handle(request)
	if r.OK || !strings.Contains(r.Code, "UNRESOLVED") || count != 1 || r.Data["nextAction"] == nil {
		t.Fatalf("unknown child became success/continued: count=%d %+v", count, r)
	}
	if _, ok := r.Data["ownedStage"].(string); !ok {
		t.Fatalf("unresolved ownership location lost: %+v", r)
	}
}

func TestEnvironmentAcceptanceJournalFailureStopsInspection(t *testing.T) {
	for _, phase := range []string{"beforelaunch", "registered", "stopped"} {
		t.Run(phase, func(t *testing.T) {
			request := environmentAcceptanceRequest(t, true)
			count := 0
			oldRun, oldRecord := environmentRunOverride, environmentRecordOverride
			environmentRecordOverride = func(state string) error {
				if state == phase {
					return os.ErrPermission
				}
				return nil
			}
			environmentRunOverride = func(ctx context.Context, spec commandSpec) (commandResult, error) {
				count++
				if err := spec.Registered(123456, 123456); err != nil {
					return commandResult{StoppedVerified: true}, err
				}
				return commandResult{StoppedVerified: true, Output: "go version go1.26.2 windows/amd64", ExitCode: 0}, nil
			}
			t.Cleanup(func() { environmentRunOverride, environmentRecordOverride = oldRun, oldRecord })
			r := Handle(request)
			want := 1
			if phase == "beforelaunch" {
				want = 0
			}
			if r.OK || count != want || r.Data["nextAction"] == nil {
				t.Fatalf("journal %s failure lost: count=%d %+v", phase, count, r)
			}
		})
	}
}

func TestEnvironmentAcceptanceExpiredAndReplacedOperation(t *testing.T) {
	for _, mode := range []string{"deadline", "replacement"} {
		t.Run(mode, func(t *testing.T) {
			op, err := newEnvironmentOperation(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer op.cancel()
			if mode == "deadline" {
				op.cancel()
			} else {
				if err := os.Rename(op.root, op.root+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(op.root, 0700); err != nil {
					t.Fatal(err)
				}
			}
			count := 0
			previous := environmentRunOverride
			environmentRunOverride = func(context.Context, commandSpec) (commandResult, error) {
				count++
				return commandResult{}, errors.New("must not execute")
			}
			t.Cleanup(func() { environmentRunOverride = previous })
			if _, err := op.run("not-executed", "go"); err == nil || count != 0 {
				t.Fatalf("%s operation launched: %d %v", mode, count, err)
			}
		})
	}
}

func TestEnvironmentAcceptanceJournalDoesNotWriteReplacedRoot(t *testing.T) {
	r := environmentAcceptanceRequest(t, false)
	previous := environmentRunOverride
	foreign := ""
	environmentRunOverride = func(ctx context.Context, spec commandSpec) (commandResult, error) {
		foreign = spec.Cwd
		if err := os.Rename(foreign, foreign+"-owned-original"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(foreign, 0700); err != nil {
			t.Fatal(err)
		}
		if err := spec.Registered(43210, 54321); err != nil {
			return commandResult{StoppedVerified: true}, err
		}
		return commandResult{StoppedVerified: true, Output: "go version go1.26.2 windows/amd64"}, nil
	}
	t.Cleanup(func() { environmentRunOverride = previous })
	response := Handle(r)
	if response.OK || foreign == "" {
		t.Fatalf("replaced root became success: %+v", response)
	}
	entries, err := os.ReadDir(foreign)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("journal wrote into a replacement/foreign directory: %+v", entries)
	}
}

func TestEnvironmentAcceptanceProbeFailureCannotMatch(t *testing.T) {
	for _, mode := range []string{"timeout", "truncation"} {
		t.Run(mode, func(t *testing.T) {
			r := environmentAcceptanceRequest(t, false)
			previous := environmentRunOverride
			environmentRunOverride = func(ctx context.Context, spec commandSpec) (commandResult, error) {
				if err := spec.Registered(43210, 54321); err != nil {
					return commandResult{}, err
				}
				return commandResult{StoppedVerified: true, TimedOut: mode == "timeout", Truncated: mode == "truncation", Output: "go version go1.26.2 windows/amd64"}, nil
			}
			t.Cleanup(func() { environmentRunOverride = previous })
			response := Handle(r)
			if response.OK {
				t.Fatalf("%s probe accepted its output as valid: %+v", mode, response)
			}
			if response.Data["nextAction"] == nil || response.Data["ownedStage"] == nil {
				t.Fatalf("%s lost recovery evidence: %+v", mode, response)
			}
		})
	}
}
