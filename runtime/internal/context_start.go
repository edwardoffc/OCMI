package internal

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

func contextStart(r Request) Response {
	fail := func(code string, rev int64) Response {
		return contextBad(code, rev, "start", "ready assignment cannot be verified", "inspect")
	}
	if len(r.Payload) > 4096 {
		return fail("INVALID_PAYLOAD", 0)
	}
	fields, ok := outputObject(r.Payload)
	if !ok || len(fields) != 4 {
		return fail("INVALID_PAYLOAD", 0)
	}
	for k := range fields {
		if k != "mode" && k != "taskId" && k != "taskRevision" && k != "worker" {
			return fail("INVALID_PAYLOAD", 0)
		}
	}
	var p struct {
		Mode         string `json:"mode"`
		TaskID       string `json:"taskId"`
		TaskRevision int64  `json:"taskRevision"`
		Worker       string `json:"worker"`
	}
	if json.Unmarshal(r.Payload, &p) != nil || p.Mode != "start" || !contextBounded(p.TaskID, 256) || !contextBounded(p.Worker, 256) || p.TaskRevision < 1 {
		return fail("INVALID_PAYLOAD", 0)
	}
	state := status(r)
	if !state.OK {
		return fail(state.Code, state.Revision)
	}
	if r.ExpectedRevision > 0 && r.ExpectedRevision != state.Revision {
		return fail("STALE_REVISION", state.Revision)
	}
	snap, e := contextTaskSnapshot(r)
	if e != nil || snap.revision != state.Revision || !contextSnapshotBounded(snap) {
		return fail("CONTEXT_INCOMPLETE", state.Revision)
	}
	t := snap.task
	if t.ID != p.TaskID || t.Revision != p.TaskRevision || t.Worker != p.Worker || t.State != "ready" || state.Data["stopped"] != false || state.Data["nextAction"] != "run" {
		return fail("START_NOT_READY", state.Revision)
	}
	db, e := openExistingFor(r)
	if e != nil {
		return fail("STATE_UNAVAILABLE", state.Revision)
	}
	var owner string
	var unresolved int
	e = db.QueryRow("SELECT owner FROM project").Scan(&owner)
	if e == nil {
		e = db.QueryRow("SELECT count(*) FROM operation WHERE state IN ('prepared','running','unknown')").Scan(&unresolved)
	}
	if e == nil && unresolved == 0 {
		e = db.QueryRow("SELECT count(*) FROM resource_ledger WHERE applying<>0 OR observed_state IN ('intent','unknown','remove_pending','transfer_pending')").Scan(&unresolved)
	}
	db.Close()
	if e != nil || unresolved != 0 || (r.Actor != owner && r.Actor != t.Worker) {
		return fail("START_NOT_READY", state.Revision)
	}
	if contextMetadataOnly(r.layout, t.Contract) {
		return contextMetadataRefusal(state.Revision, []string{t.Contract}, true)
	}
	assignment, ok := contextAssignment(t)
	if !ok {
		return fail("CONTEXT_INPUTS_CHANGED", state.Revision)
	}
	var inputs []string
	if json.Unmarshal([]byte(t.Inputs), &inputs) != nil {
		return fail("CONTEXT_INPUTS_CHANGED", state.Revision)
	}
	bound := append([]string{t.Contract}, inputs...)
	paths, required, e := contextStartPaths(r.layout, bound)
	if e != nil {
		return fail("CONTEXT_SCOPE_REFUSED", state.Revision)
	}
	set, omitted, notices, unsafe := contextPreflight(r.layout, paths, required)
	if unsafe || len(omitted) > 0 || len(notices) > 0 || !contextBindingCurrent(t, set, bound) {
		return fail("CONTEXT_INPUTS_CHANGED", state.Revision)
	}
	cmd, e := commandFingerprint(r.ProjectRoot, json.RawMessage(t.Command))
	if e != nil || cmd != t.CommandFingerprint {
		return fail("CONTEXT_INPUTS_CHANGED", state.Revision)
	}
	var checker struct {
		ID string `json:"id"`
	}
	if json.Unmarshal([]byte(t.Command), &checker) != nil || !contextBounded(checker.ID, 256) {
		return fail("CONTEXT_INCOMPLETE", state.Revision)
	}
	if contextTestAfterRead != nil {
		contextTestAfterRead()
	}
	if !contextRecheck(r, state, snap, set) {
		return fail("CONTEXT_CHANGED", state.Revision)
	}
	cmd, e = commandFingerprint(r.ProjectRoot, json.RawMessage(t.Command))
	if e != nil || cmd != t.CommandFingerprint || !contextStateRecheck(r, state, snap) {
		return fail("CONTEXT_CHANGED", state.Revision)
	}
	refs := assignment["requiredInputs"].([]map[string]any)
	bindingRef := compactBinding(snap, set, bound)
	var b strings.Builder
	fmt.Fprintf(&b, "Ready START (verified view; main must deliver).\nTask %s/%d worker %s; observed global revision %d\nRoot %s\nContract %s\nRequired sources:", strconv.Quote(t.ID), t.Revision, strconv.Quote(t.Worker), state.Revision, startRootText(filepath.Clean(r.layout.Root)), strconv.Quote(filepath.ToSlash(t.Contract)))
	metadata := []string{}
	for _, ref := range refs {
		path := ref["path"].(string)
		if path == filepath.ToSlash(t.Contract) {
			continue
		}
		if contextMetadataOnly(r.layout, path) {
			metadata = append(metadata, path)
			continue
		}
		fmt.Fprintf(&b, " %s", strconv.Quote(path))
	}
	if len(metadata) > 0 {
		b.WriteString("\nMetadata-only (omit text only if Contract needs none):")
		for _, path := range metadata {
			fmt.Fprintf(&b, " %s", strconv.Quote(path))
		}
	}
	fmt.Fprintf(&b, "\nChecker %s. Before writes: verify identity/state with compact expectedBinding=%s; read whole Contract and required originals; run only bound checker.", strconv.Quote(checker.ID), bindingRef)
	if b.Len() > 3000 {
		return fail("CONTEXT_INCOMPLETE", state.Revision)
	}
	return good("CONTEXT_START", state.Revision, map[string]any{"start": b.String(), "bindingRef": bindingRef, "deliveryRequired": true, "inputFreshness": "verified", "metadataOnlyFiles": metadata})
}

func contextMetadataOnly(layout layoutContext, path string) bool {
	return layout.Kind == "project" && (path == ".gitignore" || path == ".gitattributes")
}

func contextMetadataRefusal(rev int64, paths []string, contract bool) Response {
	reason := "Metadata-only sources can be omitted from text retrieval only if the semantic Contract does not require their contents."
	next := "Review semantic criteria and omit only unneeded metadata-only text."
	if contract {
		reason = "An active metadata-only Contract cannot be delivered as text."
		next = "Use an approved readable Contract in a task revision."
	}
	return Response{SchemaVersion: 1, Code: "CONTEXT_METADATA_ONLY", Revision: rev, Data: map[string]any{"stage": "scope", "reason": reason, "nextAction": next, "metadataOnlyFiles": paths}}
}

// START emits only path/digest metadata for these exact explicitly bound
// project-root files. Other modes and installation homes retain their scope.
func contextStartPaths(layout layoutContext, bound []string) ([]string, map[string]bool, error) {
	paths := make([]string, 0, len(bound))
	required := map[string]bool{}
	for _, path := range bound {
		allowedConfig := contextMetadataOnly(layout, path)
		plain := filepath.ToSlash(path)
		if !allowedConfig {
			selected, _, err := contextPaths(layout, []string{path}, nil)
			if err != nil {
				return nil, nil, err
			}
			plain = selected[0]
		}
		if !required[plain] {
			paths = append(paths, plain)
		}
		required[plain] = true
	}
	return paths, required, nil
}

func startRootText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\\' {
			b.WriteRune(r)
		} else if r < 0x20 || r == 0x7f || r == '"' {
			fmt.Fprintf(&b, "\\u%04x", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
