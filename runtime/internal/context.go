package internal

// This file implements the native, deliberately disposable context view.  It
// never makes a task decision and never persists source material: all source
// bytes live only for the duration of one request.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	contextFileLimit    = 128 << 10
	contextTotalLimit   = 2 << 20
	contextLineLimit    = 16384
	contextSymbolLimit  = 512
	contextWindowLimit  = 512
	contextSummaryLimit = 4096
)

var contextTerm = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$-]{0,79}$`)

// contextTestAfterRead is deliberately test-only and has no environment or
// command-line route.  Protected controls use it to model a read-window edit.
var contextTestAfterRead func()

type contextPayload struct {
	Mode          string   `json:"mode"`
	Files         []string `json:"files"`
	RequiredFiles []string `json:"requiredFiles"`
	Terms         []string `json:"terms"`
	BudgetBytes   *int     `json:"budgetBytes"`
}

type contextFile struct {
	path, sha, native string
	raw               []byte
	provider          string
}

type contextCacheEntry struct {
	SHA, Native string
	Outline     []map[string]any
	Limited     bool
}
type contextCacheState struct {
	sync.Mutex
	root, rootID, projectID, deviceID, copyID, task string
	revision                                        int64
	entries                                         map[string]contextCacheEntry
}

var contextCache contextCacheState

func contextBad(code string, revision int64, stage, reason, nextAction string) Response {
	return Response{SchemaVersion: schemaVersion, Code: code, Revision: revision, Data: map[string]any{"stage": stage, "reason": reason, "nextAction": nextAction}}
}

func contextInspect(r Request) Response {
	var mode struct {
		Mode string `json:"mode"`
	}
	if json.Unmarshal(r.Payload, &mode) == nil && mode.Mode == "start" {
		return contextStart(r)
	}
	if json.Unmarshal(r.Payload, &mode) == nil && mode.Mode == "compact" {
		return contextCompact(r)
	}
	p, invalid := parseContextPayload(r.Payload)
	if invalid != "" {
		return contextBad("INVALID_PAYLOAD", 0, "payload", invalid, "inspect")
	}
	state := status(r)
	if !state.OK {
		return contextBad(state.Code, state.Revision, "state", "current task state is unavailable", "inspect")
	}
	if r.ExpectedRevision > 0 && r.ExpectedRevision != state.Revision {
		return contextBad("STALE_REVISION", state.Revision, "revision", "expected revision no longer matches", "inspect")
	}
	snap, err := contextTaskSnapshot(r)
	if err != nil {
		return contextBad("TASK_UNBOUND", state.Revision, "binding", "no current task binding", "task.bind")
	}
	if state.Revision != snap.revision {
		return contextBad("CONTEXT_CHANGED", state.Revision, "state", "state changed while observed", "inspect")
	}
	if !contextSnapshotBounded(snap) {
		return contextBad("CONTEXT_INCOMPLETE", state.Revision, "metadata", "derived identity metadata exceeds the bounded view", "inspect")
	}
	assignment, ok := contextAssignment(snap.task)
	if !ok {
		return contextBad("CONTEXT_INPUTS_CHANGED", state.Revision, "binding", "bound input fingerprints are unavailable", "task.revise")
	}
	base := contextBase(r, state, snap, assignment, p.Mode)
	if p.Mode == "summary" {
		if !contextStateRecheck(r, state, snap) {
			return contextBad("CONTEXT_CHANGED", state.Revision, "recheck", "root, task, or state changed while observed", "inspect")
		}
		return good("CONTEXT_SELECTED", state.Revision, base)
	}
	return contextSelected(r, p, state, snap, assignment, base)
}
func contextSnapshotBounded(s contextSnapshot) bool {
	for _, value := range []string{s.rootID, s.projectID, s.deviceID, s.copyID, s.task.ID, s.task.OutcomeID, s.task.State, s.task.Worker, s.task.Reviewer, s.task.Contract, s.task.CompletionScope} {
		if !contextBounded(value, 1024) {
			return false
		}
	}
	return true
}

type contextSnapshot struct {
	revision                            int64
	rootID, projectID, deviceID, copyID string
	task                                task
}

func contextTaskSnapshot(r Request) (contextSnapshot, error) {
	layout, code := selectedLayout(r)
	if code != "" {
		return contextSnapshot{}, errors.New(code)
	}
	rootID, err := localNativeID(layout.Root)
	if err != nil {
		return contextSnapshot{}, err
	}
	db, err := openExistingFor(r)
	if err != nil {
		return contextSnapshot{}, err
	}
	defer db.Close()
	var s contextSnapshot
	if err = db.QueryRow("SELECT id,device_id,copy_id,revision FROM project").Scan(&s.projectID, &s.deviceID, &s.copyID, &s.revision); err != nil {
		return contextSnapshot{}, err
	}
	if err = db.QueryRow("SELECT t.id,t.outcome_id,t.state,t.worker,t.reviewer,t.contract_path,t.inputs_json,t.command_json,t.task_revision,t.command_fingerprint,t.input_fingerprints,t.completion_scope FROM task t JOIN active_task a ON a.task_id=t.id LIMIT 1").Scan(&s.task.ID, &s.task.OutcomeID, &s.task.State, &s.task.Worker, &s.task.Reviewer, &s.task.Contract, &s.task.Inputs, &s.task.Command, &s.task.Revision, &s.task.CommandFingerprint, &s.task.BindingFingerprint, &s.task.CompletionScope); err != nil {
		return contextSnapshot{}, err
	}
	s.rootID = rootID
	return s, nil
}

func contextAssignment(t task) (map[string]any, bool) {
	var inputs []string
	if json.Unmarshal([]byte(t.Inputs), &inputs) != nil {
		return nil, false
	}
	var parts []string
	if json.Unmarshal([]byte(t.BindingFingerprint), &parts) != nil || len(parts) != len(inputs)+1 {
		return nil, false
	}
	if len(inputs) > 127 || !contextBounded(t.Worker, 256) || !contextBounded(t.Reviewer, 256) || !contextBounded(t.Contract, 1024) {
		return nil, false
	}
	required := make([]map[string]any, 0, len(parts))
	seen := map[string]bool{}
	for i, part := range parts {
		prefix, path := "contract:", t.Contract
		if i > 0 {
			prefix, path = inputs[i-1]+":", inputs[i-1]
		}
		if !strings.HasPrefix(part, prefix) || len(part) != len(prefix)+64 || !contextBounded(path, 1024) {
			return nil, false
		}
		path = filepath.ToSlash(path)
		if seen[path] {
			continue
		}
		seen[path] = true
		required = append(required, map[string]any{"path": path, "sha256": strings.TrimPrefix(part, prefix)})
	}
	return map[string]any{"worker": t.Worker, "reviewer": t.Reviewer, "contractPath": filepath.ToSlash(t.Contract), "requiredInputs": required}, true
}
func contextBounded(s string, limit int) bool {
	return s != "" && len(s) <= limit && utf8.ValidString(s)
}

func contextBase(r Request, state Response, snap contextSnapshot, assignment map[string]any, mode string) map[string]any {
	b, _ := json.Marshal(state.Data)
	h := sha256.Sum256(b)
	summary := "OCMI task " + snap.task.ID + " is " + snap.task.State + "; next action: " + stringValue(state.Data["nextAction"]) + "; STOP: " + stringValue(state.Data["stopped"]) + "."
	if !contextBounded(summary, contextSummaryLimit) {
		summary = "OCMI state is available; inspect the current task."
	}
	return map[string]any{
		"formatVersion": 1, "mode": mode, "summary": summary, "state": state.Data, "assignment": assignment,
		"provenance": map[string]any{"rootKind": r.layout.Kind, "rootId": snap.rootID, "projectId": snap.projectID, "deviceId": snap.deviceID, "copyId": snap.copyID, "revision": state.Revision, "taskId": snap.task.ID, "taskRevision": snap.task.Revision, "statusFingerprint": hex.EncodeToString(h[:])},
		"readiness":  "not-evaluated", "coverage": "explicit-files-only", "completeness": "not-established", "inputFreshness": "not-read",
		"files": []any{}, "selected": []any{}, "omitted": []any{}, "notices": []any{}, "outline": []any{},
		"cache":   map[string]any{"storage": "session-memory-only", "reused": 0, "parsed": 0, "dropped": 0},
		"metrics": map[string]any{"filesRead": 0, "sourceBytesRead": 0, "snippetBytes": 0, "budgetBytes": 0, "tokens": "unknown"},
	}
}
func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if b, ok := v.(bool); ok {
		if b {
			return "true"
		}
		return "false"
	}
	return "unknown"
}

func parseContextPayload(raw json.RawMessage) (contextPayload, string) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return contextPayload{}, "invalid object"
	}
	for k := range fields {
		if k != "mode" && k != "files" && k != "requiredFiles" && k != "terms" && k != "budgetBytes" {
			return contextPayload{}, "unknown field"
		}
	}
	for _, key := range []string{"mode", "files", "requiredFiles", "terms", "budgetBytes"} {
		if raw, ok := fields[key]; ok && string(raw) == "null" {
			return contextPayload{}, "null field"
		}
	}
	var p contextPayload
	if json.Unmarshal(raw, &p) != nil {
		return p, "invalid field"
	}
	if p.Mode == "" {
		p.Mode = "summary"
	}
	if p.Mode != "summary" && p.Mode != "selected" {
		return p, "invalid mode"
	}
	if p.Mode == "summary" && len(fields) != 0 && !(len(fields) == 1 && fields["mode"] != nil) {
		return p, "summary accepts only mode"
	}
	if p.Mode == "selected" {
		if len(p.Files)+len(p.RequiredFiles) > 128 || len(p.Terms) > 8 {
			return p, "selection limit"
		}
		for _, x := range p.Terms {
			if !contextTerm.MatchString(x) {
				return p, "invalid term"
			}
		}
		if len(p.Files) != 0 && len(p.Terms) == 0 {
			return p, "files require terms"
		}
		if p.BudgetBytes == nil {
			n := 16000
			p.BudgetBytes = &n
		}
		if *p.BudgetBytes < 128 || *p.BudgetBytes > 65536 {
			return p, "invalid budget"
		}
	}
	return p, ""
}

func contextSelected(r Request, p contextPayload, state Response, snap contextSnapshot, assignment, out map[string]any) Response {
	var bound []string
	if json.Unmarshal([]byte(snap.task.Inputs), &bound) != nil {
		return contextBad("CONTEXT_INPUTS_CHANGED", state.Revision, "binding", "bound input list changed", "task.revise")
	}
	boundPaths := append([]string{snap.task.Contract}, bound...)
	requestedSet, explicitRequired := map[string]bool{}, map[string]bool{}
	for _, path := range p.Files {
		requestedSet[filepath.ToSlash(path)] = true
	}
	for _, path := range p.RequiredFiles {
		explicitRequired[filepath.ToSlash(path)] = true
	}
	// The contract is always whole text.  A bound input remains whole text by
	// default, but an explicit files selection requests its bounded windows
	// unless the caller also explicitly requires the original in full.
	priorityRequired := []string{snap.task.Contract}
	for _, path := range bound {
		plain := filepath.ToSlash(path)
		if !requestedSet[plain] || explicitRequired[plain] {
			priorityRequired = append(priorityRequired, path)
		}
	}
	priorityRequired = append(priorityRequired, p.RequiredFiles...)
	requested := append(append([]string{}, p.Files...), boundPaths...)
	all, requiredSet, err := contextPaths(r.layout, priorityRequired, requested)
	if err != nil {
		return contextBad("CONTEXT_SCOPE_REFUSED", state.Revision, "scope", "selected path is outside allowed source scope", "inspect")
	}
	if len(all) > 128 {
		return contextBad("INVALID_PAYLOAD", state.Revision, "payload", "file limit", "inspect")
	}
	set, earlyOmitted, earlyNotices, unsafe := contextPreflight(r.layout, all, requiredSet)
	if unsafe {
		return contextBad("CONTEXT_SCOPE_REFUSED", state.Revision, "scope", "selected source is unsafe", "inspect")
	}
	if set == nil {
		return contextBad("CONTEXT_SCOPE_REFUSED", state.Revision, "read", "selected source is unsafe or unavailable", "inspect")
	}
	// The binding must be checked from bounded reads, not the legacy unbounded
	// fingerprint helper.  It is intentionally checked before excerpts exist.
	if !contextBindingCurrent(snap.task, set, boundPaths) {
		return contextBad("CONTEXT_INPUTS_CHANGED", state.Revision, "binding", "bound contract or input changed", "task.revise")
	}
	if contextTestAfterRead != nil {
		contextTestAfterRead()
	}
	selected, outlines, omitted, notices, cache := contextRender(set, requiredSet, p.Terms, *p.BudgetBytes, r.layout.Root, snap)
	omitted = append(earlyOmitted, omitted...)
	notices = append(earlyNotices, notices...)
	if !contextRecheck(r, state, snap, set) {
		contextCacheDrop()
		return contextBad("CONTEXT_CHANGED", state.Revision, "recheck", "root, state, or source changed during read", "inspect")
	}
	files := make([]any, 0, len(set))
	for _, f := range set {
		files = append(files, map[string]any{"path": f.path, "sha256": f.sha, "bytes": len(f.raw), "provider": f.provider})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].(map[string]any)["path"].(string) < files[j].(map[string]any)["path"].(string)
	})
	snippetBytes := 0
	for _, x := range selected {
		snippetBytes += len([]byte(x["text"].(string)))
	}
	out["inputFreshness"] = "verified"
	out["files"] = files
	out["selected"] = mapsAny(selected)
	out["outline"] = mapsAny(outlines)
	out["omitted"] = mapsAny(omitted)
	out["notices"] = mapsAny(notices)
	out["cache"] = cache
	out["metrics"] = map[string]any{"filesRead": len(set), "sourceBytesRead": contextRawBytes(set), "snippetBytes": snippetBytes, "budgetBytes": *p.BudgetBytes, "tokens": "unknown"}
	if len(omitted) > 0 || len(notices) > 0 {
		out["nextAction"] = "narrow retrieval or inspect missing selected input"
		out["stage"] = "selection"
		out["reason"] = "selected context is incomplete within fixed source or output limits"
		return Response{SchemaVersion: schemaVersion, OK: false, Code: "CONTEXT_INCOMPLETE", Revision: state.Revision, Data: out}
	}
	return good("CONTEXT_SELECTED", state.Revision, out)
}
func mapsAny(in []map[string]any) []any {
	out := make([]any, len(in))
	for i := range in {
		out[i] = in[i]
	}
	return out
}
func contextRawBytes(set []contextFile) int {
	n := 0
	for _, f := range set {
		n += len(f.raw)
	}
	return n
}

func contextPaths(layout layoutContext, required, requested []string) ([]string, map[string]bool, error) {
	seen := map[string]bool{}
	requiredSet := map[string]bool{}
	all := []string{}
	add := func(p string, required bool) error {
		p = filepath.ToSlash(p)
		if !contextPathAllowed(layout, p) {
			return errors.New("scope")
		}
		if !seen[p] {
			seen[p] = true
			all = append(all, p)
		}
		if required {
			requiredSet[p] = true
		}
		return nil
	}
	for _, p := range required {
		if err := add(p, true); err != nil {
			return nil, nil, err
		}
	}
	for _, p := range requested {
		if err := add(p, false); err != nil {
			return nil, nil, err
		}
	}
	return all, requiredSet, nil
}
func contextPathAllowed(layout layoutContext, p string) bool {
	if p == "" || filepath.IsAbs(filepath.FromSlash(p)) || strings.Contains(p, "\\") || strings.Contains(p, ":") || filepath.ToSlash(filepath.Clean(filepath.FromSlash(p))) != p || strings.HasSuffix(p, ".") || strings.HasSuffix(p, " ") || !portableName(p) {
		return false
	}
	parts := strings.Split(p, "/")
	for _, part := range parts {
		lower := strings.ToLower(part)
		if part == "" || part == "." || part == ".." || strings.HasPrefix(part, ".") || lower == "node_modules" || lower == "vendor" || lower == "dist" || lower == "build" || lower == "target" || lower == "bin" || lower == "obj" || lower == "backups" || lower == "outputs" || checkpointGitSensitiveName(part) {
			return false
		}
	}
	lower := strings.ToLower(p)
	if strings.HasSuffix(lower, ".db") || strings.HasSuffix(lower, ".sqlite") || strings.HasSuffix(lower, ".log") || strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tar") || strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".key") || strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".crt") || strings.HasSuffix(lower, ".cer") || strings.HasSuffix(lower, ".pfx") || strings.HasSuffix(lower, ".p12") {
		return false
	}
	if layout.Kind != "installation" {
		return true
	}
	if strings.HasPrefix(p, "system/") || strings.HasPrefix(p, "OCMI_Sync/coordinator/") {
		return true
	}
	if strings.Contains(p, "/") {
		return false
	}
	switch lower {
	case "readme.md", "readme.ru.md", "license", "notice", "version.md", "bootstrap.md", "agents.md", "changelog.md":
		return true
	}
	return false
}

// Preflight distinguishes a lexical/link escape (never read) from a legitimate
// absent or oversized input.  The latter is partial context, not authority to
// expand scope or to expose another file.
func contextPreflight(layout layoutContext, paths []string, required map[string]bool) ([]contextFile, []map[string]any, []map[string]any, bool) {
	read, omitted, notices := []contextFile{}, []map[string]any{}, []map[string]any{}
	if environmentPlainAncestors(layout.Root) != nil {
		return nil, nil, nil, true
	}
	root, err := os.OpenRoot(layout.Root)
	if err != nil {
		return nil, nil, nil, true
	}
	defer root.Close()
	total, exhausted := 0, false
	for _, p := range paths {
		if exhausted {
			omitted = append(omitted, map[string]any{"path": p, "kind": contextOmissionKind(required, p), "reason": "aggregate source byte limit reached"})
			continue
		}
		name := filepath.FromSlash(p)
		if contextPlainParents(root, layout.Root, name) != nil {
			return nil, nil, nil, true
		}
		info, err := root.Lstat(name)
		if err != nil {
			if os.IsNotExist(err) {
				omitted = append(omitted, map[string]any{"path": p, "kind": contextOmissionKind(required, p), "reason": "selected source is unavailable"})
				continue
			}
			return nil, nil, nil, true
		}
		if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 {
			return nil, nil, nil, true
		}
		if !info.Mode().IsRegular() {
			omitted = append(omitted, map[string]any{"path": p, "kind": contextOmissionKind(required, p), "reason": "selected source is not a regular text file"})
			continue
		}
		if info.Size() > contextFileLimit {
			notices = append(notices, map[string]any{"path": p, "reason": "selected source exceeds per-file byte limit"})
			omitted = append(omitted, map[string]any{"path": p, "kind": contextOmissionKind(required, p), "reason": "selected source exceeds per-file byte limit"})
			continue
		}
		f, err := contextReadOne(root, layout.Root, p)
		if err != nil {
			if err.Error() == "link" {
				return nil, nil, nil, true
			}
			notices = append(notices, map[string]any{"path": p, "reason": "selected source has unsupported text encoding or content"})
			omitted = append(omitted, map[string]any{"path": p, "kind": contextOmissionKind(required, p), "reason": "selected source is unsupported"})
			continue
		}
		if total+len(f.raw) > contextTotalLimit {
			exhausted = true
			notices = append(notices, map[string]any{"path": p, "reason": "aggregate source byte limit reached"})
			omitted = append(omitted, map[string]any{"path": p, "kind": contextOmissionKind(required, p), "reason": "aggregate source byte limit reached"})
			continue
		}
		total += len(f.raw)
		read = append(read, f)
	}
	return read, omitted, notices, false
}
func contextPlainParents(root *os.Root, base, name string) error {
	parts := strings.Split(name, string(os.PathSeparator))
	cur := ""
	for i, part := range parts {
		if i == len(parts)-1 {
			break
		}
		cur = filepath.Join(cur, part)
		info, err := root.Lstat(cur)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 || checkpointGitReparse(filepath.Join(base, cur)) {
			return errors.New("unsafe parent")
		}
	}
	return nil
}
func contextOmissionKind(required map[string]bool, p string) string {
	if required[p] {
		return "required"
	}
	return "candidate"
}

func contextReadSet(layout layoutContext, paths []string) ([]contextFile, bool) {
	rootID, err := localNativeID(layout.Root)
	if err != nil || environmentPlainAncestors(layout.Root) != nil {
		return nil, false
	}
	root, err := os.OpenRoot(layout.Root)
	if err != nil {
		return nil, false
	}
	defer root.Close()
	set := make([]contextFile, 0, len(paths))
	total := 0
	for _, p := range paths {
		f, err := contextReadOne(root, layout.Root, p)
		if err != nil {
			return nil, false
		}
		total += len(f.raw)
		if total > contextTotalLimit {
			return nil, false
		}
		set = append(set, f)
	}
	if id, e := localNativeID(layout.Root); e != nil || id != rootID {
		return nil, false
	}
	return set, true
}
func contextReadOne(root *os.Root, base, p string) (contextFile, error) {
	name := filepath.FromSlash(p)
	if environmentRootPathPlain(root, base, name) != nil {
		return contextFile{}, errors.New("link")
	}
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > contextFileLimit {
		return contextFile{}, errors.New("file")
	}
	f, err := root.Open(name)
	if err != nil {
		return contextFile{}, err
	}
	defer f.Close()
	id, err := localNativeIDFile(f)
	if err != nil {
		return contextFile{}, err
	}
	raw, err := io.ReadAll(io.LimitReader(f, contextFileLimit+1))
	if err != nil || len(raw) > contextFileLimit || !utf8.Valid(raw) || contextControl(raw) || bytes.IndexByte(raw, 0) >= 0 || contextLines(raw) > contextLineLimit {
		return contextFile{}, errors.New("content")
	}
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Size() != info.Size() || opened.Size() != int64(len(raw)) {
		return contextFile{}, errors.New("drift")
	}
	h := sha256.Sum256(raw)
	return contextFile{path: p, sha: hex.EncodeToString(h[:]), native: id, raw: raw, provider: "native-root"}, nil
}
func contextControl(raw []byte) bool {
	for _, b := range raw {
		if b < 0x09 || (b > 0x0d && b < 0x20) || b == 0x7f {
			return true
		}
	}
	return false
}
func contextLines(raw []byte) int { return bytes.Count(raw, []byte{'\n'}) + 1 }

func contextBindingCurrent(t task, set []contextFile, bound []string) bool {
	by := map[string]contextFile{}
	for _, f := range set {
		by[f.path] = f
	}
	parts := []string{}
	for i, p := range bound {
		f, ok := by[filepath.ToSlash(p)]
		if !ok {
			return false
		}
		if i == 0 {
			parts = append(parts, "contract:"+f.sha)
		} else {
			parts = append(parts, filepath.ToSlash(p)+":"+f.sha)
		}
	}
	b, _ := json.Marshal(parts)
	return string(b) == t.BindingFingerprint
}
func contextRecheck(r Request, state Response, snap contextSnapshot, set []contextFile) bool {
	fresh, ok := contextReadSet(r.layout, contextPathsOnly(set))
	if !ok || len(fresh) != len(set) {
		return false
	}
	for i := range set {
		if fresh[i].path != set[i].path || fresh[i].sha != set[i].sha || fresh[i].native != set[i].native {
			return false
		}
	}
	return contextStateRecheck(r, state, snap)
}
func contextStateRecheck(r Request, state Response, snap contextSnapshot) bool {
	now := status(r)
	if !now.OK || now.Revision != state.Revision {
		return false
	}
	b, _ := json.Marshal(now.Data)
	h := sha256.Sum256(b)
	old, _ := json.Marshal(state.Data)
	oh := sha256.Sum256(old)
	if h != oh {
		return false
	}
	current, err := contextTaskSnapshot(r)
	return err == nil && current.rootID == snap.rootID && current.revision == snap.revision && current.projectID == snap.projectID && current.deviceID == snap.deviceID && current.copyID == snap.copyID && contextTaskSame(current.task, snap.task)
}
func contextTaskSame(a, b task) bool {
	return a.ID == b.ID && a.OutcomeID == b.OutcomeID && a.State == b.State && a.Worker == b.Worker && a.Reviewer == b.Reviewer && a.Contract == b.Contract && a.Inputs == b.Inputs && a.Command == b.Command && a.CommandFingerprint == b.CommandFingerprint && a.BindingFingerprint == b.BindingFingerprint && a.Revision == b.Revision && a.CompletionScope == b.CompletionScope
}
func contextPathsOnly(set []contextFile) []string {
	out := make([]string, len(set))
	for i := range set {
		out[i] = set[i].path
	}
	return out
}

func contextCacheDrop() {
	contextCache.Lock()
	defer contextCache.Unlock()
	contextCache.root, contextCache.rootID, contextCache.projectID, contextCache.deviceID, contextCache.copyID, contextCache.task, contextCache.revision, contextCache.entries = "", "", "", "", "", "", 0, nil
}

func contextRender(set []contextFile, required map[string]bool, terms []string, budget int, root string, snap contextSnapshot) ([]map[string]any, []map[string]any, []map[string]any, []map[string]any, map[string]any) {
	selected, outlines, omitted, notices := []map[string]any{}, []map[string]any{}, []map[string]any{}, []map[string]any{}
	cache, reused, parsed, dropped := contextCacheFor(root, snap, set)
	remaining := budget
	definitions, references := []map[string]any{}, []map[string]any{}
	outlineLimited := false
	windowLimited := false
	for _, f := range set {
		if required[f.path] {
			text := contextText(f.raw)
			if len([]byte(text)) > remaining {
				omitted = append(omitted, map[string]any{"path": f.path, "kind": "required", "reason": "required input exceeds selected budget"})
				continue
			}
			selected = append(selected, map[string]any{"path": f.path, "sha256": f.sha, "startLine": 1, "endLine": contextLines(f.raw), "kind": "required", "text": text})
			remaining -= len([]byte(text))
			continue
		}
		outline := cache[f.path].Outline
		for _, item := range outline {
			if contextOutlineMatches(item, terms) {
				outlines = append(outlines, item)
			}
		}
		if cache[f.path].Limited {
			outlineLimited = true
		}
		if len(terms) == 0 {
			continue
		}
		windows, limited := contextWindows(f, terms)
		windowLimited = windowLimited || limited
		if len(windows) == 0 {
			omitted = append(omitted, map[string]any{"path": f.path, "kind": "candidate", "reason": "no matching candidates"})
			continue
		}
		for _, window := range windows {
			if window["kind"] == "definition-candidate" {
				if len(definitions) < contextWindowLimit {
					definitions = append(definitions, window)
				} else {
					windowLimited = true
				}
			} else if len(references) < contextWindowLimit {
				references = append(references, window)
			} else {
				windowLimited = true
			}
		}
	}
	if len(outlines) > 128 {
		outlines = outlines[:128]
		outlineLimited = true
	}
	if outlineLimited {
		notices = append(notices, map[string]any{"path": "", "reason": "outline symbol or aggregate output limit reached"})
	}
	sortWindows(definitions)
	sortWindows(references)
	candidates := append(definitions, references...)
	if len(candidates) > contextWindowLimit {
		candidates = candidates[:contextWindowLimit]
		windowLimited = true
	}
	for _, window := range candidates {
		if len(selected) >= contextWindowLimit || len([]byte(window["text"].(string))) > remaining {
			omitted = contextOmit(omitted, window["path"].(string), "candidate", "snippet budget exhausted")
			continue
		}
		selected = append(selected, window)
		remaining -= len([]byte(window["text"].(string)))
	}
	if windowLimited {
		notices = append(notices, map[string]any{"path": "", "reason": "candidate window limit reached"})
	}
	if len(selected) == 0 && len(terms) > 0 {
		notices = append(notices, map[string]any{"path": "", "reason": "no selected candidates"})
	}
	return selected, outlines, omitted, notices, map[string]any{"storage": "session-memory-only", "reused": reused, "parsed": parsed, "dropped": dropped}
}
func sortWindows(values []map[string]any) {
	sort.SliceStable(values, func(i, j int) bool {
		pi, pj := values[i]["path"].(string), values[j]["path"].(string)
		if pi != pj {
			return pi < pj
		}
		return values[i]["startLine"].(int) < values[j]["startLine"].(int)
	})
}
func contextOmit(items []map[string]any, path, kind, reason string) []map[string]any {
	for _, item := range items {
		if item["path"] == path && item["kind"] == kind && item["reason"] == reason {
			return items
		}
	}
	return append(items, map[string]any{"path": path, "kind": kind, "reason": reason})
}
func contextOutlineMatches(item map[string]any, terms []string) bool {
	if len(terms) == 0 {
		return false
	}
	name, _ := item["name"].(string)
	for _, term := range terms {
		if name == term {
			return true
		}
	}
	return false
}

// Text results use portable LF newlines while hashes/freshness retain the raw
// bytes actually opened from the selected root.
func contextText(raw []byte) string {
	return strings.ReplaceAll(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\r", "\n")
}

// contextWindows returns small, line-stable lexical windows.  Identifier
// matching deliberately has word boundaries: `compute` must not select
// `precompute`.  Definitions sort before references at the same source line.
func contextWindows(f contextFile, terms []string) ([]map[string]any, bool) {
	lines := strings.Split(contextText(f.raw), "\n")
	definitions, references := []map[string]any{}, []map[string]any{}
	seen := map[string]bool{}
	limited := false
	for i, line := range lines {
		kind := ""
		for _, term := range terms {
			if !contextIdentifierLine(line, term) {
				continue
			}
			if contextDefinitionLine(line, term) {
				kind = "definition-candidate"
			} else {
				kind = "reference-candidate"
			}
			break
		}
		if kind == "" {
			continue
		}
		start, end := i, i
		if start > 0 {
			start--
		}
		if end+1 < len(lines) {
			end++
		}
		key := kind + ":" + strconv.Itoa(start) + ":" + strconv.Itoa(end)
		if seen[key] {
			continue
		}
		seen[key] = true
		text := strings.Join(lines[start:end+1], "\n")
		item := map[string]any{"path": f.path, "sha256": f.sha, "startLine": start + 1, "endLine": end + 1, "kind": kind, "text": text}
		if kind == "definition-candidate" {
			if len(definitions) < contextWindowLimit {
				definitions = append(definitions, item)
			} else {
				limited = true
			}
		} else if len(references) < contextWindowLimit {
			references = append(references, item)
		} else {
			limited = true
		}
	}
	result := append(definitions, references...)
	if len(result) > contextWindowLimit {
		result = result[:contextWindowLimit]
		limited = true
	}
	return result, limited
}
func contextIdentifierLine(line, term string) bool {
	for at := strings.Index(line, term); at >= 0; {
		left := at == 0 || !contextIdentifierByte(line[at-1])
		after := at + len(term)
		right := after == len(line) || !contextIdentifierByte(line[after])
		if left && right {
			return true
		}
		next := strings.Index(line[at+len(term):], term)
		if next < 0 {
			break
		}
		at += len(term) + next
	}
	return false
}
func contextIdentifierByte(b byte) bool {
	return b == '_' || b == '$' || b == '-' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}
func contextDefinitionLine(line, term string) bool {
	for _, prefix := range []string{"func ", "type ", "function ", "class ", "def ", "fn "} {
		if strings.Contains(line, prefix+term) && contextIdentifierLine(line, term) {
			return true
		}
	}
	return false
}
func contextCacheFor(root string, snap contextSnapshot, set []contextFile) (map[string]contextCacheEntry, int, int, int) {
	contextCache.Lock()
	defer contextCache.Unlock()
	old := contextCache.entries
	if contextCache.root != root || contextCache.rootID != snap.rootID || contextCache.projectID != snap.projectID || contextCache.deviceID != snap.deviceID || contextCache.copyID != snap.copyID || contextCache.task != snap.task.ID || contextCache.revision != snap.task.Revision {
		old = nil
	}
	next := map[string]contextCacheEntry{}
	reused, parsed, dropped := 0, 0, 0
	for _, f := range set {
		if e, ok := old[f.path]; ok && e.SHA == f.sha && e.Native == f.native {
			next[f.path] = e
			reused++
		} else {
			outline, limited := contextOutline(f)
			next[f.path] = contextCacheEntry{SHA: f.sha, Native: f.native, Outline: outline, Limited: limited}
			parsed++
			if ok {
				dropped++
			}
		}
	}
	for p := range old {
		if _, ok := next[p]; !ok {
			dropped++
		}
	}
	contextCache.root, contextCache.rootID, contextCache.projectID, contextCache.deviceID, contextCache.copyID, contextCache.task, contextCache.revision, contextCache.entries = root, snap.rootID, snap.projectID, snap.deviceID, snap.copyID, snap.task.ID, snap.task.Revision, next
	return next, reused, parsed, dropped
}
func contextOutline(f contextFile) ([]map[string]any, bool) {
	out := []map[string]any{}
	lines := strings.Split(string(f.raw), "\n")
	for i, line := range lines {
		name := ""
		for _, prefix := range []string{"func ", "type ", "func ", "function ", "class ", "def ", "function ", "fn "} {
			if at := strings.Index(line, prefix); at >= 0 {
				tail := strings.TrimSpace(line[at+len(prefix):])
				for j, r := range tail {
					if !(r == '_' || r == '$' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
						tail = tail[:j]
						break
					}
				}
				name = tail
				break
			}
		}
		if name != "" {
			out = append(out, map[string]any{"path": f.path, "name": name, "line": i + 1, "endLine": i + 1, "provider": "lexical"})
			if len(out) >= contextSymbolLimit {
				return out, true
			}
		}
	}
	return out, false
}
