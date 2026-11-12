package internal

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type compactEntry struct {
	Path   string `json:"path"`
	Hash   string `json:"hash"`
	Native string `json:"native"`
}
type compactReceipt struct {
	Version int            `json:"version"`
	Root    string         `json:"root"`
	Project string         `json:"project"`
	Copy    string         `json:"copy"`
	Device  string         `json:"device"`
	Actor   string         `json:"actor"`
	Files   []compactEntry `json:"files"`
}

func compactBinding(s contextSnapshot, files []contextFile, bound []string) string {
	by := map[string]contextFile{}
	for _, f := range files {
		by[f.path] = f
	}
	refs := make([]compactEntry, 0, len(bound))
	for _, p := range bound {
		f := by[filepath.ToSlash(p)]
		refs = append(refs, compactEntry{f.path, f.sha, f.native})
	}
	raw, _ := json.Marshal([]any{s.rootID, s.projectID, s.copyID, s.deviceID, s.task.ID, s.task.OutcomeID, s.task.Revision, s.task.Worker, s.task.Reviewer, refs, s.task.CommandFingerprint})
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

func contextCompact(r Request) Response {
	fail := func(code string, rev int64) Response {
		return contextBad(code, rev, "compact", "context cannot be safely verified", "inspect")
	}
	if len(r.Payload) > 16384 {
		return fail("INVALID_PAYLOAD", 0)
	}
	fields, ok := outputObject(r.Payload)
	if !ok {
		return fail("INVALID_PAYLOAD", 0)
	}
	for k := range fields {
		switch k {
		case "mode", "files", "requiredFiles", "budgetBytes", "receipt", "expectedBinding":
		default:
			return fail("INVALID_PAYLOAD", 0)
		}
	}
	var p struct {
		Mode     string   `json:"mode"`
		Files    []string `json:"files"`
		Required []string `json:"requiredFiles"`
		Budget   *int     `json:"budgetBytes"`
		Receipt  string   `json:"receipt"`
		Expected string   `json:"expectedBinding"`
	}
	if json.Unmarshal(r.Payload, &p) != nil || p.Mode != "compact" || len(p.Files) > 32 || len(p.Required) > 128 {
		return fail("INVALID_PAYLOAD", 0)
	}
	for _, paths := range [][]string{p.Files, p.Required} {
		seen := map[string]bool{}
		for _, path := range paths {
			normalized := filepath.ToSlash(path)
			if seen[normalized] {
				return fail("INVALID_PAYLOAD", 0)
			}
			seen[normalized] = true
		}
	}
	for _, v := range fields {
		if string(v) == "null" {
			return fail("INVALID_PAYLOAD", 0)
		}
	}
	budget := 4096
	if p.Budget != nil {
		budget = *p.Budget
	}
	if budget < 128 || budget > 65536 {
		return fail("INVALID_PAYLOAD", 0)
	}
	if len(p.Receipt) > 8192 || (p.Expected != "" && !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(p.Expected)) {
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
	db, e := openExistingFor(r)
	if e != nil {
		return fail("STATE_UNAVAILABLE", state.Revision)
	}
	var owner string
	e = db.QueryRow("SELECT owner FROM project").Scan(&owner)
	db.Close()
	if e != nil || (r.Actor != owner && r.Actor != snap.task.Worker && r.Actor != snap.task.Reviewer) {
		return fail("ACCESS_DENIED", state.Revision)
	}
	if _, ok := contextAssignment(snap.task); !ok {
		return fail("CONTEXT_INPUTS_CHANGED", state.Revision)
	}
	var inputs []string
	if json.Unmarshal([]byte(snap.task.Inputs), &inputs) != nil {
		return fail("CONTEXT_INPUTS_CHANGED", state.Revision)
	}
	bound := append([]string{snap.task.Contract}, inputs...)
	boundPaths, _, e := contextStartPaths(r.layout, bound)
	if e != nil {
		return fail("CONTEXT_SCOPE_REFUSED", state.Revision)
	}
	closed := snap.task.State == "integrated" && state.Data["cleanup"] == "done"
	if closed {
		if rs, ok := state.Data["resourceSummary"].(map[string]any); !ok || rs["pending"] != 0 || rs["activeUses"] != 0 {
			closed = false
		}
	}
	boundSet := map[string]bool{}
	for _, path := range bound {
		boundSet[path] = true
	}
	metadata := []string{}
	metadataSeen := map[string]bool{}
	for _, path := range append(append([]string{}, p.Required...), p.Files...) {
		if contextMetadataOnly(r.layout, path) && boundSet[path] {
			if !metadataSeen[path] {
				metadata = append(metadata, path)
				metadataSeen[path] = true
			}
		} else if !contextPathAllowed(r.layout, filepath.ToSlash(path)) {
			return fail("CONTEXT_SCOPE_REFUSED", state.Revision)
		}
	}
	if !closed && contextMetadataOnly(r.layout, snap.task.Contract) {
		return contextMetadataRefusal(state.Revision, []string{snap.task.Contract}, true)
	}
	if len(metadata) > 0 {
		return contextMetadataRefusal(state.Revision, metadata, false)
	}
	requested, _, e := contextPaths(r.layout, p.Required, p.Files)
	if e != nil {
		return fail("CONTEXT_SCOPE_REFUSED", state.Revision)
	}
	seen := map[string]bool{}
	paths := []string{}
	req := map[string]bool{}
	for _, path := range append(boundPaths, requested...) {
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
		req[path] = true
	}
	if len(paths) > 128 {
		return fail("INVALID_PAYLOAD", state.Revision)
	}
	set, omitted, notices, unsafe := contextPreflight(r.layout, paths, req)
	if unsafe || len(omitted) > 0 || len(notices) > 0 || !contextBindingCurrent(snap.task, set, bound) {
		return fail("CONTEXT_INPUTS_CHANGED", state.Revision)
	}
	cmd, e := commandFingerprint(r.ProjectRoot, json.RawMessage(snap.task.Command))
	if e != nil || cmd != snap.task.CommandFingerprint {
		return fail("CONTEXT_INPUTS_CHANGED", state.Revision)
	}
	ref := compactBinding(snap, set, bound)
	if p.Expected != "" {
		raw, _ := json.Marshal(map[string]any{"mode": "start", "taskId": snap.task.ID, "taskRevision": snap.task.Revision, "worker": snap.task.Worker})
		sr := r
		sr.Payload = raw
		checked := contextStart(sr)
		if !checked.OK || checked.Data["bindingRef"] != p.Expected || ref != p.Expected {
			return fail("CONTEXT_CHANGED", state.Revision)
		}
	}
	old := compactReceipt{}
	if p.Receipt != "" {
		raw, e := base64.RawURLEncoding.DecodeString(p.Receipt)
		if e != nil || len(raw) > 6144 {
			return fail("INVALID_RECEIPT", state.Revision)
		}
		m, valid := outputObject(raw)
		if !valid || len(m) != 7 || json.Unmarshal(raw, &old) != nil || old.Version != 1 || old.Root != snap.rootID || old.Project != snap.projectID || old.Copy != snap.copyID || old.Device != snap.deviceID || old.Actor != r.Actor || len(old.Files) > 32 {
			return fail("INVALID_RECEIPT", state.Revision)
		}
		for k := range m {
			switch k {
			case "version", "root", "project", "copy", "device", "actor", "files":
			default:
				return fail("INVALID_RECEIPT", state.Revision)
			}
		}
		var elements []json.RawMessage
		if string(m["files"]) == "null" || json.Unmarshal(m["files"], &elements) != nil || len(elements) != len(old.Files) {
			return fail("INVALID_RECEIPT", state.Revision)
		}
		for _, element := range elements {
			item, valid := outputObject(element)
			if !valid || len(item) != 3 {
				return fail("INVALID_RECEIPT", state.Revision)
			}
			for k := range item {
				switch k {
				case "path", "hash", "native":
				default:
					return fail("INVALID_RECEIPT", state.Revision)
				}
			}
		}
		prior := map[string]bool{}
		for _, f := range old.Files {
			if f.Path == "" || prior[f.Path] || !contextPathAllowed(r.layout, f.Path) || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(f.Hash) || f.Native == "" {
				return fail("INVALID_RECEIPT", state.Revision)
			}
			prior[f.Path] = true
		}
	}
	oldBy := map[string]compactEntry{}
	for _, f := range old.Files {
		oldBy[f.Path] = f
	}
	by := map[string]contextFile{}
	for _, f := range set {
		by[f.path] = f
	}
	mandatory := map[string]bool{}
	for _, path := range p.Required {
		mandatory[filepath.ToSlash(path)] = true
	}
	contract := filepath.ToSlash(snap.task.Contract)
	if !closed {
		mandatory[contract] = true
	}
	var output strings.Builder
	delivered := map[string]bool{}
	retained := []string{}
	entries := []compactEntry{}
	emit := func(path string) {
		if delivered[path] {
			return
		}
		f := by[path]
		fmt.Fprintf(&output, "--- %s ---\n%s\n", path, contextText(f.raw))
		delivered[path] = true
	}
	for _, path := range requested {
		if mandatory[path] {
			emit(path)
		}
	}
	if mandatory[contract] {
		emit(contract)
	}
	for _, raw := range p.Files {
		path := filepath.ToSlash(raw)
		f := by[path]
		if mandatory[path] || path == contract {
			emit(path)
			continue
		}
		entry := compactEntry{path, f.sha, f.native}
		entries = append(entries, entry)
		if prior, exists := oldBy[path]; exists && prior == entry {
			retained = append(retained, path)
		} else {
			emit(path)
		}
	}
	if output.Len() > budget {
		return Response{SchemaVersion: 1, Code: "CONTEXT_BUDGET", Revision: state.Revision, Data: map[string]any{"requiredBytes": output.Len()}}
	}
	if contextTestAfterRead != nil {
		contextTestAfterRead()
	}
	if !contextRecheck(r, state, snap, set) {
		return fail("CONTEXT_CHANGED", state.Revision)
	}
	cmd, e = commandFingerprint(r.ProjectRoot, json.RawMessage(snap.task.Command))
	if e != nil || cmd != snap.task.CommandFingerprint || !contextStateRecheck(r, state, snap) {
		return fail("CONTEXT_CHANGED", state.Revision)
	}
	data := map[string]any{"context": output.String(), "complete": true, "inputFreshness": "verified", "bindingRef": ref, "state": map[string]any{"stopped": state.Data["stopped"], "taskState": state.Data["taskState"], "nextAction": state.Data["nextAction"]}, "provenance": map[string]any{"rootId": snap.rootID, "projectId": snap.projectID, "copyId": snap.copyID, "deviceId": snap.deviceID, "taskId": snap.task.ID, "taskRevision": snap.task.Revision}, "retainedFiles": retained}
	if len(entries) > 0 {
		next := compactReceipt{1, snap.rootID, snap.projectID, snap.copyID, snap.deviceID, r.Actor, entries}
		raw, e := json.Marshal(next)
		if e != nil || len(raw) > 6144 {
			return fail("CONTEXT_INCOMPLETE", state.Revision)
		}
		token := base64.RawURLEncoding.EncodeToString(raw)
		if len(token) > 8192 {
			return fail("CONTEXT_INCOMPLETE", state.Revision)
		}
		if token == p.Receipt {
			data["receiptUnchanged"] = true
		} else {
			data["receipt"] = token
		}
	}
	return good("CONTEXT_COMPACT", state.Revision, data)
}
