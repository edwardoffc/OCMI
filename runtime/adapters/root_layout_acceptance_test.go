package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func acceptanceAdapterHome(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	marker := map[string]any{"schemaVersion": 1, "layoutVersion": 1, "kind": "installation", "paths": map[string]string{
		"system": "system", "bin": "system/bin", "sync": "OCMI_Sync", "coordinator": "OCMI_Sync/coordinator",
		"personal": "OCMI_Sync/personal", "recovery": "OCMI_Sync/recovery", "local": "local", "state": "local/state",
		"store": "local/state/state.sqlite", "bindings": "local/bindings", "cache": "local/cache", "runs": "local/runs",
	}}
	raw, _ := json.Marshal(marker)
	if err := os.WriteFile(filepath.Join(root, ".ocmi-home.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRootLayoutAcceptanceFixedAdapterAndHook(t *testing.T) {
	root := acceptanceAdapterHome(t)
	raw, _ := json.Marshal(map[string]string{"ProjectRoot": root, "Actor": "main", "RootKind": "installation"})
	var config Config
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	config, err := canonicalConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	var input requestInput
	raw = []byte(`{"action":"init","requestId":"adapter-init","payload":{"projectId":"home","deviceId":"device","copyId":"copy","owner":"main"}}`)
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	if response := request(config, input); !response.OK {
		t.Fatal(response)
	}
	if _, err := os.Stat(filepath.Join(root, "local", "state", "state.sqlite")); err != nil {
		t.Fatalf("adapter did not retain its installation profile: %v", err)
	}
	scope := filepath.Join(root, "OCMI_Sync", "coordinator")
	if err := os.MkdirAll(scope, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"contract.md", "input.txt"} {
		if err := os.WriteFile(filepath.Join(scope, name), []byte("Fictional selected coordinator input."), 0600); err != nil {
			t.Fatal(err)
		}
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(map[string]any{
		"action": "task.bind", "requestId": "adapter-bind", "expectedRevision": 0,
		"payload": map[string]any{
			"taskId": "adapter-task", "outcomeId": "adapter-outcome", "taskRevision": 1, "completionScope": "target",
			"worker": "worker-1", "reviewer": "main", "contract": "OCMI_Sync/coordinator/contract.md",
			"inputs":  []string{"OCMI_Sync/coordinator/input.txt"},
			"command": map[string]any{"id": "check", "executable": executable, "args": []string{"-test.run=^$"}, "cwd": "OCMI_Sync/coordinator", "timeoutMs": 3000, "outputLimitBytes": 1024},
		},
	})
	input = requestInput{}
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	if response := request(config, input); !response.OK {
		t.Fatal(response)
	}
	input = requestInput{}
	raw = []byte(`{"action":"status","requestId":"override","rootKind":"project","payload":{}}`)
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	if response := request(config, input); response.OK {
		t.Fatal("caller changed the fixed root profile", response)
	}
	var output bytes.Buffer
	if err := RunHook(config, strings.NewReader(`{"hook_event_name":"SessionStart"}`), &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "unavailable") {
		t.Fatal("hook did not inspect the configured home ledger", output.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".ocmi")); !os.IsNotExist(err) {
		t.Fatal("parallel project state appeared", err)
	}
}

func TestRootLayoutAcceptanceNativeMCP(t *testing.T) {
	binary := os.Getenv("OCMI_ROOT_BINARY")
	if binary == "" {
		t.Skip("actual compiled CLI is provided by the acceptance runner after build")
	}
	root := acceptanceAdapterHome(t)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "root-layout-check", Version: "v1"}, nil)
	command := exec.Command(binary, "mcp", "--project-root", root, "--root-kind", "installation", "--actor", "main")
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "ocmi_request", Arguments: map[string]any{
		"action": "init", "requestId": "native-init", "expectedRevision": 0,
		"payload": map[string]any{"projectId": "native-home", "deviceId": "device", "copyId": "copy", "owner": "main"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("native home init: result=%+v err=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(root, "local", "state", "state.sqlite")); err != nil {
		t.Fatal(err)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "ocmi_request", Arguments: map[string]any{
		"action": "layout.inspect", "requestId": "native-inspect", "expectedRevision": 0, "payload": map[string]any{},
	}})
	if err != nil || result.IsError {
		t.Fatalf("native selected-root read: result=%+v err=%v", result, err)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "ocmi_request", Arguments: map[string]any{
		"action": "status", "rootKind": "project", "requestId": "override", "expectedRevision": 0, "payload": map[string]any{},
	}})
	if err != nil || !result.IsError {
		t.Fatalf("native root profile override admitted: result=%+v err=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".ocmi")); !os.IsNotExist(err) {
		t.Fatal("native MCP created parallel project state", err)
	}
}
