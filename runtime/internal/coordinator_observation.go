package internal

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"path/filepath"
)

// observeRestoredCoordinator records a use-only observation of restored semantic
// memory. It deliberately neither writes nor acquires ownership of that memory.
func observeRestoredCoordinator(context layoutContext, coordinatorID string) error {
	if coordinatorID == "" || context.Paths["coordinator"] == "" {
		return errors.New("invalid restored coordinator observation")
	}
	id := "restored-coordinator-use-" + coordinatorID
	rev, err := resourceRevision(context)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"resourceId": id, "provider": "local", "kind": "directory", "mode": "use",
		"locator": filepath.Join(context.Root, filepath.FromSlash(context.Paths["coordinator"])), "purpose": "Use selected restored coordinator memory without ownership",
		"lifetime": "installation", "disposition": "release", "ownerRef": "coordinator",
	})
	if err != nil {
		return err
	}
	intent := Handle(Request{SchemaVersion: 1, Action: "resource.intent", ProjectRoot: context.Root, RootKind: "installation", Actor: "coordinator", RequestID: id + "-intent-" + uuid.NewString(), ExpectedRevision: rev, Payload: payload})
	if !intent.OK {
		return errors.New(intent.Code)
	}
	applyPayload, err := json.Marshal(map[string]any{"resourceId": id})
	if err != nil {
		return err
	}
	apply := Handle(Request{SchemaVersion: 1, Action: "resource.apply", ProjectRoot: context.Root, RootKind: "installation", Actor: "coordinator", RequestID: id + "-apply-" + uuid.NewString(), ExpectedRevision: intent.Revision, Payload: applyPayload})
	if !apply.OK {
		return errors.New(apply.Code)
	}
	keepPayload, err := json.Marshal(map[string]any{"resourceId": id, "disposition": "keep", "reason": "Selected restored coordinator memory remains externally owned", "until": "until explicit coordinator retirement"})
	if err != nil {
		return err
	}
	keep := Handle(Request{SchemaVersion: 1, Action: "resource.dispose", ProjectRoot: context.Root, RootKind: "installation", Actor: "coordinator", RequestID: id + "-keep-" + uuid.NewString(), ExpectedRevision: apply.Revision, Payload: keepPayload})
	if !keep.OK {
		return errors.New(keep.Code)
	}
	return nil
}
