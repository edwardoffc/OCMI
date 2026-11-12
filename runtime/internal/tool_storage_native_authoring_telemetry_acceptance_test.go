package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Independent native premise: the documented off command is safe to run before
// the ordinary query in a new private configuration. No host configuration is read.
func TestMainToolAuthoringTelemetryOffPremise(t *testing.T) {
	goExe, _, _ := mainAuthorNativeTools(t)
	f := mainAuthorSetup(t)
	if err := os.MkdirAll(filepath.Join(f.home, "local", "runs"), 0700); err != nil {
		t.Fatal(err)
	}
	locator := "local/runs/11111111-1111-4111-8111-111111111111"
	proof, err := createLocalResource(f.home, locator, "directory", "native-off-premise", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	scratch := filepath.Join(f.home, filepath.FromSlash(locator))
	outside := filepath.Join(f.base, "outside-config-sentinel")
	if err := os.WriteFile(outside, []byte("preserve host-independent sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	env := []string{"HOME=" + scratch, "APPDATA=" + scratch, "XDG_CONFIG_HOME=" + scratch,
		"TMP=" + scratch, "TEMP=" + scratch, "TMPDIR=" + scratch, "GOTMPDIR=" + scratch,
		"GOENV=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off"}
	registered := 0
	call := func(args []string) commandResult {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		r, err := runOwned(ctx, commandSpec{Executable: goExe, Args: args, Cwd: scratch, Env: env, OutputLimit: 8192,
			Registered: func(int, int) error { registered++; return nil }})
		if err != nil || r.ExitCode != 0 || r.TimedOut || r.Truncated || r.Pending || r.Unresolved || !r.StoppedVerified {
			t.Fatalf("isolated documented Go command failed: exit=%d pending=%t stopped=%t err=%v", r.ExitCode, r.Pending, r.StoppedVerified, err)
		}
		return r
	}
	call([]string{"telemetry", "off"})
	q := call([]string{"env", "-json", "GOTELEMETRY", "GOTELEMETRYDIR"})
	var values map[string]string
	if json.Unmarshal([]byte(q.Output), &values) != nil || values["GOTELEMETRY"] != "off" || !environmentWithin(scratch, values["GOTELEMETRYDIR"]) {
		t.Fatal("fresh native Go did not observe private off mode")
	}
	if registered != 2 {
		t.Fatal("both native commands must be supervised")
	}
	if raw, err := os.ReadFile(outside); err != nil || string(raw) != "preserve host-independent sentinel" {
		t.Fatal("unselected config changed")
	}
	final, err := captureLocalResource(f.home, locator, "directory", proof.NativeIdentity, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := removeLocalResource(f.home, locator, "directory", proof.NativeIdentity, final.Inventory, true); err != nil {
		t.Fatal("native off fixture closure", err)
	}
}

// The baseline fails before launching any unconfigured provider. This protects
// the pre-effect route, not a lucky successful cleanup after a detached sidecar.
func TestMainToolAuthoringGoTelemetryScoped(t *testing.T) {
	goExe, _, _ := mainAuthorNativeTools(t)
	f := mainAuthorSetup(t)
	options := mainAuthorOptions(f, "go")
	options["executable"] = goExe
	p := mainAuthorPrepare(t, f, options)
	previous := environmentRunOverride
	defer func() { environmentRunOverride = previous }()
	calls := 0
	problem := ""
	environmentRunOverride = func(ctx context.Context, spec commandSpec) (commandResult, error) {
		calls++
		settings := map[string]string{}
		for _, entry := range spec.Env {
			key, value, ok := strings.Cut(entry, "=")
			if ok {
				settings[strings.ToUpper(key)] = value
			}
		}
		if calls == 1 && (len(spec.Args) != 2 || spec.Args[0] != "telemetry" || spec.Args[1] != "off") {
			problem = "first Go effect must be documented telemetry off"
		}
		for _, key := range []string{"HOME", "APPDATA", "XDG_CONFIG_HOME"} {
			if settings[key] != spec.Cwd {
				problem = "configuration must be scoped before first Go effect"
			}
		}
		if spec.Registered == nil {
			problem = "missing registered child"
		}
		if problem != "" {
			return commandResult{StoppedVerified: true}, errors.New("unqualified scoped Go preparation")
		}
		r, err := runOwned(ctx, spec)
		if calls == 2 {
			var values map[string]string
			if json.Unmarshal([]byte(r.Output), &values) != nil || values["GOTELEMETRY"] != "off" || !environmentWithin(spec.Cwd, values["GOTELEMETRYDIR"]) {
				problem = "fresh path query did not verify private off mode"
			}
		}
		return r, err
	}
	r := mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, p, "go"))
	if problem != "" {
		t.Fatal(problem)
	}
	if !r.OK || calls != 2 {
		t.Fatal("scoped Go off then query must activate exactly once", calls, r)
	}
	if r.Data["verification"] != "effective-paths-verified" {
		t.Fatal("missing native path proof", r)
	}
	if r := mainAuthorCall(f, "inspect", "coordinator", map[string]any{"toolId": "go"}); !r.OK || r.Data["phase"] != "active" {
		t.Fatal(r)
	}
}

func TestMainToolAuthoringProbeFailureTyped(t *testing.T) {
	f := mainAuthorSetup(t)
	_, node, _ := mainAuthorNativeTools(t)
	options := mainAuthorOptions(f, "go")
	options["executable"] = node
	p := mainAuthorPrepare(t, f, options)
	r := mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, p, "go"))
	stage, _ := r.Data["stage"].(string)
	reason, _ := r.Data["reason"].(string)
	if r.OK || r.Code != "TOOL_STORAGE_PROBE_FAILED" || !strings.HasPrefix(stage, "probe.") || reason == "" || reason == "refused" || r.Data["nextAction"] == "" {
		t.Fatal("provider failure must identify finite probe stage/reason", r)
	}
}

func TestMainToolAuthoringProbeStageDiagnostics(t *testing.T) {
	for _, scenario := range []string{"control", "execution", "response", "identity", "inventory", "cleanup"} {
		t.Run(scenario, func(t *testing.T) {
			f := mainAuthorSetup(t)
			goExe, _, _ := mainAuthorNativeTools(t)
			options := mainAuthorOptions(f, "go")
			options["executable"] = goExe
			p := mainAuthorPrepare(t, f, options)
			previous := environmentRunOverride
			defer func() { environmentRunOverride = previous }()
			environmentRunOverride = func(_ context.Context, spec commandSpec) (commandResult, error) {
				if len(spec.Args) == 2 && spec.Args[0] == "telemetry" && spec.Args[1] == "off" {
					return commandResult{ExitCode: 0, StoppedVerified: true}, nil
				}
				values := map[string]string{"GOTELEMETRY": "off", "GOTELEMETRYDIR": filepath.Join(spec.Cwd, "go", "telemetry")}
				for _, entry := range spec.Env {
					key, value, ok := strings.Cut(entry, "=")
					if ok && (key == "GOCACHE" || key == "GOMODCACHE" || key == "GOPATH" || key == "GOBIN" || key == "GOTMPDIR") {
						values[key] = value
					}
				}
				switch scenario {
				case "execution":
					return commandResult{ExitCode: 1, StoppedVerified: true}, errors.New("private-provider-error-marker")
				case "response":
					values["GOCACHE"] += "-wrong"
				case "identity":
					if err := os.Rename(spec.Cwd, spec.Cwd+".moved"); err != nil {
						t.Fatal("native identity fault premise", err)
					}
					if err := os.Mkdir(spec.Cwd, 0700); err != nil {
						t.Fatal(err)
					}
				case "inventory":
					for i := 0; i < 257; i++ {
						if err := os.WriteFile(filepath.Join(spec.Cwd, fmt.Sprintf("fixture-%03d", i)), []byte("fictional"), 0600); err != nil {
							t.Fatal(err)
						}
					}
				case "cleanup":
					if err := os.Mkdir(filepath.Join(spec.Cwd, ".git"), 0700); err != nil {
						t.Fatal(err)
					}
				}
				raw, _ := json.Marshal(values)
				return commandResult{ExitCode: 0, StoppedVerified: true, Output: string(raw)}, nil
			}
			r := mainAuthorCall(f, "apply", "coordinator", mainAuthorApproval(t, p, "go"))
			if scenario == "control" {
				if !r.OK || r.Data["phase"] != "active" {
					t.Fatal("qualified fault harness control", r)
				}
				return
			}
			if r.OK || r.Code != "TOOL_STORAGE_PROBE_FAILED" || r.Data["stage"] != "probe."+scenario || r.Data["reason"] == "" || r.Data["reason"] == "refused" {
				t.Fatal("probe stage was hidden or mislabeled", scenario, r)
			}
			raw, _ := json.Marshal(r)
			if strings.Contains(string(raw), "private-provider-error-marker") {
				t.Fatal("raw provider detail escaped typed diagnostic")
			}
			if _, err := os.Lstat(filepath.Join(f.home, "local", "bindings", "tool-storage", "go.json")); !os.IsNotExist(err) {
				t.Fatal("failed native proof published profile", err)
			}
		})
	}
}
