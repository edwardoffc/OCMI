package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Independent consumer oracle: retrieve registered evidence, never read SQL.
func outputFixture(t *testing.T, policy, helper string, captureLimit int) (string, Response) {
	t.Helper()
	root := t.TempDir()
	r := request(root, "init", "main", 0, map[string]any{"projectId": "output-fixture", "deviceId": "device", "copyId": "copy", "owner": "main"})
	if !r.OK {
		t.Fatal(r)
	}
	for _, name := range []string{"contract.md", "input.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	command := map[string]any{"id": "output-control", "executable": os.Args[0], "args": []string{"--ocmi-cli-test-helper", helper, root, helper}, "cwd": ".", "timeoutMs": 5000, "outputLimitBytes": captureLimit}
	if policy != "" {
		command["outputPolicy"] = policy
	}
	r = request(root, "task.bind", "main", r.Revision, map[string]any{"taskId": "output-task", "outcomeId": "output-outcome", "taskRevision": 1, "completionScope": "target", "worker": "worker", "reviewer": "main", "contract": "contract.md", "inputs": []string{"input.txt"}, "command": command})
	if !r.OK {
		t.Fatal(r)
	}
	r = request(root, "task.ready", "main", r.Revision, map[string]any{})
	if !r.OK {
		t.Fatal(r)
	}
	r = request(root, "run", "worker", r.Revision, map[string]any{})
	if !r.OK {
		t.Fatal(r)
	}
	return root, r
}

func TestMainOperationOutputConsumer(t *testing.T) {
	root, run := outputFixture(t, "shareable", "large", 2048)
	op := run.Data["operationId"]
	for _, actor := range []string{"main", "worker"} {
		r := request(root, "operation.output", actor, run.Revision, map[string]any{"operationId": op, "limitBytes": 2048})
		if !r.OK || r.Code != "OPERATION_OUTPUT" || r.Revision != run.Revision || r.Data["output"] != strings.Repeat("x", 1024) || r.Data["complete"] != true || r.Data["capturedTruncated"] != false || r.Data["returnedTruncated"] != false || r.Data["operationId"] != op || r.Data["taskId"] != "output-task" {
			t.Fatal("registered full evidence unavailable or incorrectly qualified", r)
		}
	}
	r := request(root, "operation.output", "worker", 0, map[string]any{"operationId": op, "limitBytes": 128})
	if !r.OK || len(r.Data["output"].(string)) != 128 || r.Data["complete"] != false || r.Data["returnedTruncated"] != true || r.Data["capturedTruncated"] != false {
		t.Fatal("response bound hidden", r)
	}
	for _, payload := range []map[string]any{{"operationId": op, "limitBytes": 0}, {"operationId": op, "limitBytes": 65537}, {"operationId": op, "limitBytes": 1.5}, {"operationId": "unknown"}} {
		if r := request(root, "operation.output", "main", 0, payload); r.OK || r.Data["output"] != nil {
			t.Fatal("invalid evidence request leaked text", r)
		}
	}
	if r := request(root, "operation.output", "stranger", 0, map[string]any{"operationId": op}); r.OK || r.Data["output"] != nil {
		t.Fatal("unassigned actor obtained output", r)
	}
	if r := request(root, "operation.output", "main", 1, map[string]any{"operationId": op}); r.OK || r.Code != "STALE_REVISION" || r.Data["output"] != nil {
		t.Fatal("stale read not refused", r)
	}
	if r := request(root, "status", "main", 0, map[string]any{}); !r.OK || r.Revision != run.Revision {
		t.Fatal("evidence observation mutated state", r)
	}
	// Ordinary inspection never discloses output, even for a shareable command.
	if r := request(root, "operation.inspect", "main", 0, map[string]any{"operationId": op}); !r.OK || r.Data["output"] != nil {
		t.Fatal("default observation changed", r)
	}
}

func TestMainOperationOutputPrivacyAndIncomplete(t *testing.T) {
	for _, policy := range []string{"", "private"} {
		t.Run("withheld-"+policy, func(t *testing.T) {
			root, run := outputFixture(t, policy, "large", 2048)
			r := request(root, "operation.output", "main", 0, map[string]any{"operationId": run.Data["operationId"], "outputPolicy": "shareable"})
			if r.OK || r.Data["output"] != nil {
				t.Fatal("request promoted unreviewed output", r)
			}
		})
	}
	root, run := outputFixture(t, "shareable", "7", 2048)
	r := request(root, "operation.output", "main", 0, map[string]any{"operationId": run.Data["operationId"]})
	if !r.OK || r.Data["operationState"] != "failed" || r.Data["exitCode"] != 7 || r.Data["output"] != "" {
		t.Fatal("failed command evidence lost", r)
	}
	root, run = outputFixture(t, "shareable", "large", 64)
	r = request(root, "operation.output", "worker", 0, map[string]any{"operationId": run.Data["operationId"]})
	if !r.OK || r.Data["capturedTruncated"] != true || r.Data["complete"] != false || len(r.Data["output"].(string)) > 64 {
		t.Fatal("captured truncation hidden", r)
	}
}

func TestMainErgonomicsNativeCLI(t *testing.T) {
	binary := os.Getenv("OCMI_ROOT_BINARY")
	if binary == "" {
		t.Skip("set after actual native CLI build")
	}
	for _, args := range [][]string{{"--help"}, {"help", "task.bind"}, {"help", "operation.output"}, {"help", "call"}} {
		out, err := exec.Command(binary, args...).CombinedOutput()
		var r Response
		if err != nil || json.Unmarshal(out, &r) != nil || !r.OK || r.Code != "HELP" {
			t.Fatal("native help", args, err, string(out))
		}
		if len(out) > 16384 {
			t.Fatal("unbounded help")
		}
		if args[len(args)-1] == "task.bind" && (!bytes.Contains(out, []byte("completionScope")) || !bytes.Contains(out, []byte("outputPolicy"))) {
			t.Fatal("bind example incomplete", string(out))
		}
	}
	root, run := outputFixture(t, "shareable", "large", 2048)
	payload, _ := json.Marshal(map[string]any{"operationId": run.Data["operationId"], "limitBytes": 128})
	args := []string{"call", "--root", root, "--actor", "worker", "--action", "operation.output", "--payload", string(payload)}
	out, err := exec.Command(binary, args...).CombinedOutput()
	var r Response
	if err != nil || json.Unmarshal(out, &r) != nil || !r.OK || len(r.Data["output"].(string)) != 128 {
		t.Fatal("same-core native output", err, string(out))
	}
	// Short call must not silently supply a mutation revision.
	out, err = exec.Command(binary, "call", "--root", root, "--actor", "worker", "--action", "submit").CombinedOutput()
	if err == nil || json.Unmarshal(out, &r) != nil || r.OK {
		t.Fatal("mutation revision silently inferred", string(out))
	}
	if state := request(root, "status", "main", 0, map[string]any{}); state.Revision != run.Revision {
		t.Fatal("help/call refusal changed state", state)
	}
	args = []string{"call", "--root", root, "--actor", "worker", "--action", "submit", "--revision", fmt.Sprint(run.Revision), "--request-id", "native-short-submit"}
	for range 2 {
		out, err = exec.Command(binary, args...).CombinedOutput()
		if err != nil || json.Unmarshal(out, &r) != nil || !r.OK || r.Code != "SUBMITTED" || r.Revision != run.Revision+1 {
			t.Fatal("short mutation bypassed same-core semantics or exact retry", err, string(out))
		}
	}
	if state := request(root, "status", "main", 0, map[string]any{}); state.Revision != run.Revision+1 {
		t.Fatal("exact request replay duplicated mutation", state)
	}
}
