package adapters

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMaintenanceAcceptanceNativeMCP(t *testing.T) {
	bin := os.Getenv("OCMI_ROOT_BINARY")
	if bin == "" {
		t.Skip("actual native binary supplied by acceptance runner")
	}
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "maintenance-native-check", Version: "v1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: exec.Command(bin, "mcp", "--project-root", root, "--actor", "main")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	call := func(action, key string, payload map[string]any) {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "ocmi_request", Arguments: map[string]any{"action": action, "requestId": key, "expectedRevision": 0, "payload": payload}})
		if err != nil || result.IsError {
			t.Fatalf("%s: %v %+v", action, err, result)
		}
		r := decodeResponse(t, result)
		if action == "resource.maintain.inspect" && (!r.OK || r.Code != "RESOURCE_MAINTENANCE_INSPECTED" || r.Revision != 0 || r.Data["mutationSupported"] != false || r.Data["protected"] != true || r.Data["complete"] != true) {
			t.Fatalf("read-only selected state measurement lost: %+v", r)
		}
	}
	call("init", "native-init", map[string]any{"projectId": "maintenance", "deviceId": "device", "copyId": "copy", "owner": "main"})
	call("resource.maintain.inspect", "native-read", map[string]any{"resourceId": "ocmi-state", "sizeLimitBytes": 1, "ageLimitDays": 1, "maxEntries": 64, "maxBytes": 16 << 20})
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	t.Log("MAINTENANCE_MCP_NATIVE_OK selected protected state, no revision mutation, native session closed")
}
