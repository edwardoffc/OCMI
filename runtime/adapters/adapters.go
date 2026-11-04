// Package adapters exposes bounded transports for the OCMI runtime service.
package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/edwardoffc/OCMI/runtime/internal"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxHookInput = 1 << 20
const maxRequest = 8 << 20

// Config is fixed at process startup. MCP callers cannot replace its root or actor.
type Config struct {
	ProjectRoot  string
	Actor        string
	TransferRoot string
}

func (c Config) valid() bool {
	return filepath.IsAbs(c.ProjectRoot) && c.Actor != ""
}

func canonicalConfig(c Config) (Config, error) {
	if !c.valid() {
		return Config{}, errors.New("requires an absolute project root and non-empty actor")
	}
	root, err := filepath.EvalSymlinks(c.ProjectRoot)
	if err != nil {
		return Config{}, err
	}
	c.ProjectRoot = root
	if c.TransferRoot != "" {
		if !filepath.IsAbs(c.TransferRoot) {
			return Config{}, errors.New("transfer root must be absolute")
		}
		c.TransferRoot = filepath.Clean(c.TransferRoot)
	}
	return c, nil
}

type requestInput struct {
	Action           string `json:"action"`
	RequestID        string `json:"requestId"`
	ExpectedRevision int64  `json:"expectedRevision"`
	Payload          any    `json:"payload"`
	ProjectRoot      string `json:"projectRoot,omitempty"`
	Actor            string `json:"actor,omitempty"`
}

// ServeMCP runs one stdio MCP server bound to c. It never opens a listener.
func ServeMCP(ctx context.Context, c Config) error {
	var err error
	c, err = canonicalConfig(c)
	if err != nil {
		return fmt.Errorf("mcp %w", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "ocmi", Version: "v1"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ocmi_request",
		Description: "Send a bounded request to the OCMI service bound when this server started.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in requestInput) (*mcp.CallToolResult, any, error) {
		_ = ctx
		response := request(c, in)
		encoded, err := json.Marshal(response)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: string(encoded)}},
			StructuredContent: response,
			IsError:           !response.OK,
		}, response, nil
	})
	return server.Run(ctx, &mcp.StdioTransport{})
}

func request(c Config, in requestInput) internal.Response {
	payload, err := json.Marshal(in.Payload)
	whole, wholeErr := json.Marshal(in)
	if in.Action == "" || len(in.Action) > 256 || in.RequestID == "" || len(in.RequestID) > 512 || in.ExpectedRevision < 0 || err != nil || wholeErr != nil || len(whole) > maxRequest || len(payload) == 0 || string(payload) == "null" {
		return rejected("INVALID_REQUEST")
	}
	if (in.ProjectRoot != "" && filepath.Clean(in.ProjectRoot) != c.ProjectRoot) || (in.Actor != "" && in.Actor != c.Actor) {
		return rejected("BOUND_IDENTITY_DENIED")
	}
	if strings.HasPrefix(in.Action, "checkpoint.") {
		return rejected("CHECKPOINT_CLI_ONLY")
	}
	if strings.HasPrefix(in.Action, "environment.") {
		return rejected("ENVIRONMENT_CLI_ONLY")
	}
	if err := validateTransfer(c, in.Action, payload); err != nil {
		return rejected("TRANSFER_PATH_DENIED")
	}
	return internal.Handle(internal.Request{
		SchemaVersion: 1, Action: in.Action, ProjectRoot: c.ProjectRoot, Actor: c.Actor,
		RequestID: in.RequestID, ExpectedRevision: in.ExpectedRevision, Payload: payload,
	})
}

func rejected(code string) internal.Response {
	return internal.Response{SchemaVersion: 1, OK: false, Code: code, Revision: 0, Data: map[string]any{}}
}

func validateTransfer(c Config, action string, payload json.RawMessage) error {
	if action != "handover.export" && action != "handover.import" && action != "handover.ack" && action != "handover.cancel" {
		return nil
	}
	var value map[string]json.RawMessage
	if json.Unmarshal(payload, &value) != nil {
		return errors.New("bad payload")
	}
	for _, field := range []string{"transferPath", "packagePath"} {
		raw, selected := value[field]
		if !selected {
			continue
		}
		if c.TransferRoot == "" {
			return errors.New("no transfer root")
		}
		var path string
		if json.Unmarshal(raw, &path) != nil || !within(c.TransferRoot, path) {
			return errors.New("transfer path escape")
		}
	}
	return nil
}

func within(root, candidate string) bool {
	if !filepath.IsAbs(candidate) {
		return false
	}
	rootClean, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	candidateClean, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		parent, parentErr := filepath.EvalSymlinks(filepath.Dir(candidate))
		if parentErr != nil {
			return false
		}
		candidateClean = filepath.Join(parent, filepath.Base(candidate))
	}
	rel, err := filepath.Rel(rootClean, candidateClean)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// RunHook reads one documented Codex hook event and writes a bounded response.
// It is a guardrail for the named OCMI MCP call, not an installation or enforcement claim.
func RunHook(c Config, in io.Reader, out io.Writer) error {
	var err error
	c, err = canonicalConfig(c)
	if err != nil {
		return fmt.Errorf("hook %w", err)
	}
	b, err := io.ReadAll(io.LimitReader(in, maxHookInput+1))
	if err != nil {
		return err
	}
	if len(b) > maxHookInput {
		return errors.New("hook input too large")
	}
	var event struct {
		Event string `json:"hook_event_name"`
		Tool  string `json:"tool_name"`
		Input struct {
			Action  string          `json:"action"`
			Payload json.RawMessage `json:"payload"`
		} `json:"tool_input"`
	}
	if err := json.Unmarshal(b, &event); err != nil {
		return err
	}
	var response any = map[string]any{"ocmi": "no_coverage"}
	switch event.Event {
	case "SessionStart":
		response = sessionContext(c)
	case "PreToolUse":
		if event.Tool == "mcp__ocmi__ocmi_request" {
			response = guardCall(c, event.Input.Action, event.Input.Payload)
		}
	}
	return json.NewEncoder(out).Encode(response)
}

func sessionContext(c Config) any {
	status := internal.Handle(internal.Request{SchemaVersion: 1, Action: "status", ProjectRoot: c.ProjectRoot, Actor: c.Actor, RequestID: "hook-session-status", ExpectedRevision: 0, Payload: json.RawMessage(`{}`)})
	context := "OCMI state is unavailable; no installation or restoration is assumed."
	if status.OK {
		context = fmt.Sprintf("OCMI task %v is %v; next action: %v; STOP: %v.", status.Data["taskId"], status.Data["taskState"], status.Data["nextAction"], status.Data["stopped"])
	} else if status.Code == "UPGRADE_REQUIRED" {
		context = fmt.Sprintf("OCMI state requires upgrade; next action: %v.", status.Data["nextAction"])
	} else if status.Code == "TASK_UNBOUND" {
		context = "OCMI state has no bound task; no task restoration is available."
	}
	return map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "SessionStart", "additionalContext": context}}
}

func guardCall(c Config, action string, payload json.RawMessage) any {
	if preservationAction(action) || (action == "stop" && explicitStop(payload)) {
		return map[string]any{"ocmi": "safe_selected_call"}
	}
	if action == "stop" {
		return deny("STOP updates require an explicit enabled boolean.")
	}
	if !consequential(action) {
		return deny("Unknown OCMI action is not covered by this hook.")
	}
	status := internal.Handle(internal.Request{SchemaVersion: 1, Action: "status", ProjectRoot: c.ProjectRoot, Actor: c.Actor, RequestID: "hook-guard-status", ExpectedRevision: 0, Payload: json.RawMessage(`{}`)})
	if !status.OK || status.Data["stopped"] == true || status.Data["execution"] == "unknown/unverified" {
		reason := "OCMI state is unavailable."
		if status.OK && status.Data["stopped"] == true {
			reason = "OCMI STOP is active."
		}
		return deny(reason)
	}
	return map[string]any{"ocmi": "state_allows_selected_call"}
}

func explicitStop(payload json.RawMessage) bool {
	var p struct {
		Enabled *bool `json:"enabled"`
	}
	return json.Unmarshal(payload, &p) == nil && p.Enabled != nil
}

func consequential(action string) bool {
	switch action {
	case "task.bind", "task.ready", "task.revise", "run", "submit", "review", "integrate", "resource.intent", "resource.apply", "resource.write", "resource.use", "resource.release", "resource.hold", "resource.dispose", "resource.cancel", "resource.reconcile", "resource.rebind", "handover.export", "handover.import", "handover.ack", "handover.cancel", "handover.rebind", "outcome.reassess", "outcome.review", "outcome.decide":
		return true
	}
	return false
}

func preservationAction(action string) bool {
	switch action {
	case "status", "init", "task.history", "operation.inspect", "resource.inspect", "resource.list", "resource.close", "state.upgrade", "operation.reconcile":
		return true
	}
	return false
}

func deny(reason string) any {
	return map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "PreToolUse", "permissionDecision": "deny", "permissionDecisionReason": reason}}
}
