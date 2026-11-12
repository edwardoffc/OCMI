package internal

// This file intentionally keeps the Git checkpoint separate from the v1 file
// package: a git-wip archive has a different closed inventory and must never
// be accepted by a v1 reader.

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	gitCheckpointSchema = 2
	gitMaxFiles         = 4096
	gitMaxBlob          = int64(32 << 20)
	gitMaxTotal         = int64(256 << 20)
	gitMaxBundle        = int64(512 << 20)
	gitMaxManifest      = 4 << 20
)

var checkpointGitCaptureBeforeVerify func()
var checkpointGitRestoreBeforeActivation func(string) error

// Test-only wrapper boundary for an owned Git helper. Nil uses runOwned.
var checkpointGitRunOverride func(context.Context, commandSpec) (commandResult, error)

func checkpointGitWriteJournal(stage, owner, purpose, state string, pid int) error {
	raw, err := json.Marshal(map[string]any{"owner": owner, "purpose": purpose, "state": state, "pid": pid})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stage, "git-operation.json"), raw, 0600)
}

type checkpointGitCapture struct {
	PackagePath, CheckpointID, SystemVersion, SourceIdentity, Owner, Purpose, RetainUntil, NextAction string
	References                                                                                        []checkpointReference `json:"references"`
	Refs                                                                                              []string              `json:"refs"`
	Files                                                                                             []string              `json:"files"`
}
type checkpointGitRef struct {
	Name string `json:"name"`
	OID  string `json:"oid"`
}
type checkpointGitBundle struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type checkpointGitIndex struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	OID  string `json:"oid"`
}
type checkpointGitFile struct {
	Path       string `json:"path"`
	State      string `json:"state"`
	SHA256     string `json:"sha256"`
	Executable bool   `json:"executable"`
}
type checkpointGitBlob struct {
	SHA256 string `json:"sha256"`
	OID    string `json:"oid"`
	Size   int64  `json:"size"`
}
type checkpointGitInfo struct {
	ObjectFormat string               `json:"objectFormat"`
	HeadOID      string               `json:"headOid"`
	AttachedRef  string               `json:"attachedRef"`
	Refs         []checkpointGitRef   `json:"refs"`
	Bundle       checkpointGitBundle  `json:"bundle"`
	Index        []checkpointGitIndex `json:"index"`
	Files        []checkpointGitFile  `json:"files"`
	ExtraFiles   []string             `json:"extraFiles"`
	Blobs        []checkpointGitBlob  `json:"blobs"`
}
type checkpointGitManifest struct {
	SchemaVersion  int                   `json:"schemaVersion"`
	Kind           string                `json:"kind"`
	CheckpointID   string                `json:"checkpointId"`
	SystemVersion  string                `json:"systemVersion"`
	SourceIdentity string                `json:"sourceIdentity"`
	Owner          string                `json:"owner"`
	Purpose        string                `json:"purpose"`
	RetainUntil    string                `json:"retainUntil"`
	NextAction     string                `json:"nextAction"`
	References     []checkpointReference `json:"references,omitempty"`
	Git            checkpointGitInfo     `json:"git"`
}

func checkpointGitCaptureAction(r Request) (out Response) {
	var in checkpointGitCapture
	if !checkpointGitNoCaseAlias(r.Payload) || !checkpointStrictJSON(r.Payload, &in) || !checkpointGitCaptureInputValid(in) || checkpointOutputPath(in.PackagePath) != nil {
		return bad("INVALID_CHECKPOINT", 0)
	}
	op, err := newCheckpointGitOperation(filepath.Dir(in.PackagePath), in.Owner, in.Purpose, in.RetainUntil, "capture")
	if op != nil {
		defer op.finish(&out)
	}
	if err != nil {
		return bad("CHECKPOINT_UNAVAILABLE", 0)
	}
	group, err := op.newGroup(op.artifactRoot, "capture-", "git-capture", nil)
	if err != nil {
		return bad("CHECKPOINT_UNAVAILABLE", 0)
	}
	stage := group.Path
	if err := checkpointGitSourceOK(op, r.ProjectRoot); err != nil {
		if op.unknown {
			stage = ""
			return Response{SchemaVersion: schemaVersion, OK: false, Code: "CHECKPOINT_UNRESOLVED", Revision: 0, Data: map[string]any{"ownedStage": op.journalRoot, "nextAction": "inspect retained Git helper evidence"}}
		}
		return bad("CHECKPOINT_SOURCE_REFUSED", 0)
	}
	m, err := checkpointGitBuild(op, stage, r.ProjectRoot, in)
	if err == nil && checkpointGitCaptureBeforeVerify != nil {
		checkpointGitCaptureBeforeVerify()
	}
	if err == nil {
		err = op.guard()
	}
	if err == nil {
		err = checkpointGitVerifySource(op, r.ProjectRoot, m)
	}
	if err == nil {
		_, _, err = checkpointGitReadPackage(op, stage)
	}
	if err != nil {
		if op.unknown {
			stage = ""
			return Response{SchemaVersion: schemaVersion, OK: false, Code: "CHECKPOINT_UNRESOLVED", Revision: 0, Data: map[string]any{"ownedStage": op.journalRoot, "nextAction": "inspect retained Git helper evidence"}}
		}
		return bad("CHECKPOINT_CAPTURE_REFUSED", 0)
	}
	if op.ctx.Err() != nil {
		return bad("CHECKPOINT_CAPTURE_REFUSED", 0)
	}
	if err = op.publish(group, in.PackagePath); err != nil {
		return bad("CHECKPOINT_DESTINATION_REFUSED", 0)
	}
	stage = ""
	return good("CHECKPOINT_GIT_CAPTURED", 0, checkpointGitData(m, checkpointGitManifestDigest(m)))
}

func checkpointGitInspectAction(r Request) (out Response) {
	op, err := newCheckpointGitOperation(filepath.Dir(r.ProjectRoot), "main", "Git checkpoint inspection", "inspect completion", "inspect")
	if op != nil {
		defer op.finish(&out)
	}
	if err != nil {
		return bad("CHECKPOINT_UNAVAILABLE", 0)
	}
	p, ok := checkpointGitPathPayload(r.Payload)
	if !ok {
		return bad("INVALID_CHECKPOINT", 0)
	}
	m, d, e := checkpointGitReadPackage(op, p)
	if e != nil {
		return bad("CHECKPOINT_INVALID", 0)
	}
	if e = checkpointGitValidateBundle(op, p, m); e != nil {
		return bad("CHECKPOINT_INVALID", 0)
	}
	if op.ctx.Err() != nil {
		return bad("CHECKPOINT_INVALID", 0)
	}
	return good("CHECKPOINT_GIT_INSPECTED", 0, checkpointGitData(m, d))
}

func checkpointGitRestoreAction(r Request) (out Response) {
	op, opErr := newCheckpointGitOperation(filepath.Dir(r.ProjectRoot), "main", "Git checkpoint restore", "restore completion", "restore")
	if op != nil {
		defer op.finish(&out)
	}
	if opErr != nil {
		return bad("CHECKPOINT_UNAVAILABLE", 0)
	}
	p, ok := checkpointGitPathPayload(r.Payload)
	if !ok {
		return bad("INVALID_CHECKPOINT", 0)
	}
	m, d, e := checkpointGitReadPackage(op, p)
	if e != nil {
		return bad("CHECKPOINT_INVALID", 0)
	}
	if checkpointEmptyDirectory(r.ProjectRoot) != nil || checkpointPlainDirectory(filepath.Dir(r.ProjectRoot)) != nil {
		return bad("CHECKPOINT_DESTINATION_REFUSED", 0)
	}
	destination, e := checkpointHold(r.ProjectRoot, "original-empty-destination", nil)
	if destination != nil {
		defer destination.closeHandles()
	}
	if e != nil {
		return bad("CHECKPOINT_DESTINATION_REFUSED", 0)
	}
	if e = op.recordDestination(destination); e != nil {
		return bad("CHECKPOINT_UNAVAILABLE", 0)
	}
	if e = checkpointGitValidateBundle(op, p, m); e != nil {
		return bad("CHECKPOINT_INVALID", 0)
	}
	group, e := op.newGroup(op.artifactRoot, "restore-", "git-restore", checkpointGitRestoreLayout(m))
	if e != nil {
		return bad("CHECKPOINT_UNAVAILABLE", 0)
	}
	stage := group.Path
	e = checkpointGitReconstruct(op, stage, p, m)
	if e == nil && checkpointGitRestoreBeforeActivation != nil {
		e = checkpointGitRestoreBeforeActivation(stage)
	}
	if e == nil {
		e = op.guard()
	}
	if e == nil {
		var check checkpointGitManifest
		var got string
		check, got, e = checkpointGitReadPackage(op, p)
		_ = check
		if got != d {
			e = errors.New("package drift")
		}
	}
	if e == nil {
		e = checkpointGitVerifyRestored(op, stage, m)
	}
	if e == nil {
		e = checkpointEmptyDirectory(r.ProjectRoot)
	}
	if e == nil {
		e = checkpointGitEmptyDestination(r.ProjectRoot)
	}
	if e == nil {
		e = checkpointPlainDirectory(filepath.Dir(r.ProjectRoot))
	}
	if e != nil {
		return bad("CHECKPOINT_RESTORE_REFUSED", 0)
	}
	if op.ctx.Err() != nil {
		return bad("CHECKPOINT_RESTORE_REFUSED", 0)
	}
	if e = op.activate(group, destination); e != nil {
		return bad("CHECKPOINT_DESTINATION_REFUSED", 0)
	}
	return good("CHECKPOINT_GIT_RESTORED", 0, checkpointGitData(m, d))
}

func checkpointGitPathPayload(raw json.RawMessage) (string, bool) {
	var p struct {
		PackagePath string `json:"packagePath"`
	}
	if !checkpointGitNoCaseAlias(raw) || !checkpointStrictJSON(raw, &p) || !filepath.IsAbs(p.PackagePath) || !checkpointText(p.PackagePath, 4096) {
		return "", false
	}
	return filepath.Clean(p.PackagePath), true
}

func checkpointGitEmptyDestination(path string) error {
	return checkpointEmptyDirectory(path)
}

// v1 keeps its compatibility decoder. Git format 2 is a closed, strict
// schema, so reject case-only aliases before encoding/json can fold them.
func checkpointGitNoCaseAlias(raw []byte) bool {
	canonical := []string{"packagePath", "checkpointId", "systemVersion", "sourceIdentity", "owner", "purpose", "retainUntil", "nextAction", "references", "refs", "files", "id", "required", "status", "schemaVersion", "kind", "git", "objectFormat", "headOid", "attachedRef", "name", "oid", "bundle", "path", "size", "sha256", "index", "mode", "state", "executable", "extraFiles", "blobs", "manifestSha256"}
	var walk func(*json.Decoder) bool
	walk = func(d *json.Decoder) bool {
		t, err := d.Token()
		if err != nil {
			return false
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return true
		}
		switch delim {
		case '{':
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return false
				}
				key, ok := k.(string)
				if !ok {
					return false
				}
				for _, exact := range canonical {
					if strings.EqualFold(key, exact) && key != exact {
						return false
					}
				}
				if !walk(d) {
					return false
				}
			}
			e, err := d.Token()
			return err == nil && e == json.Delim('}')
		case '[':
			for d.More() {
				if !walk(d) {
					return false
				}
			}
			e, err := d.Token()
			return err == nil && e == json.Delim(']')
		}
		return false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if !walk(d) {
		return false
	}
	_, err := d.Token()
	return err == io.EOF
}

func checkpointGitCaptureInputValid(in checkpointGitCapture) bool {
	return checkpointText(in.PackagePath, 4096) && checkpointText(in.CheckpointID, 256) && checkpointText(in.SystemVersion, 256) && checkpointText(in.SourceIdentity, 256) && checkpointText(in.Owner, 4096) && checkpointText(in.Purpose, 4096) && checkpointText(in.RetainUntil, 4096) && checkpointText(in.NextAction, 4096) && len(in.Refs) > 0 && len(in.Refs) <= 32 && len(in.Files) <= gitMaxFiles && checkpointReferences(in.References)
}
func checkpointGitData(m checkpointGitManifest, d string) map[string]any {
	return map[string]any{"checkpointId": m.CheckpointID, "manifestSha256": d, "nextAction": m.NextAction, "references": m.References, "state": "requires_rebind", "readyToResume": false, "kind": "git-wip", "headOid": m.Git.HeadOID, "attachedRef": m.Git.AttachedRef}
}
func checkpointGitManifestDigest(m checkpointGitManifest) string {
	b, _ := json.Marshal(m)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func checkpointGitBuild(o *checkpointGitOperation, stage, root string, in checkpointGitCapture) (checkpointGitManifest, error) {
	worktree, e := os.OpenRoot(root)
	if e != nil {
		return checkpointGitManifest{}, e
	}
	defer worktree.Close()
	head, e := gitText(o, root, "rev-parse", "HEAD")
	if e != nil {
		return checkpointGitManifest{}, e
	}
	attached, e := gitText(o, root, "symbolic-ref", "HEAD")
	if e != nil {
		return checkpointGitManifest{}, e
	}
	refs := make([]checkpointGitRef, 0, len(in.Refs))
	seen := map[string]bool{}
	for _, name := range in.Refs {
		if !strings.HasPrefix(name, "refs/heads/") || seen[name] {
			return checkpointGitManifest{}, errors.New("refs")
		}
		seen[name] = true
		oid, e := gitText(o, root, "rev-parse", name)
		if e != nil {
			return checkpointGitManifest{}, e
		}
		refs = append(refs, checkpointGitRef{name, oid})
	}
	if !seen[attached] {
		return checkpointGitManifest{}, errors.New("attached ref")
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Name < refs[j].Name })
	bundle := filepath.Join(stage, "history.bundle")
	args := []string{"bundle", "create", bundle, "HEAD"}
	for _, ref := range refs {
		args = append(args, ref.Name)
	}
	if e = gitRun(o, root, nil, args...); e != nil {
		return checkpointGitManifest{}, e
	}
	idx, e := gitBytes(o, root, nil, "ls-files", "--stage", "-z")
	if e != nil {
		return checkpointGitManifest{}, e
	}
	index, e := checkpointGitParseIndex(idx)
	if e != nil {
		return checkpointGitManifest{}, e
	}
	paths := map[string]bool{}
	tree, e := gitBytes(o, root, nil, "ls-tree", "-r", "-z", "HEAD")
	if e != nil {
		return checkpointGitManifest{}, e
	}
	for _, p := range checkpointGitTreePaths(tree) {
		paths[p] = true
	}
	for _, x := range index {
		paths[x.Path] = true
	}
	extras := map[string]bool{}
	for _, p := range in.Files {
		if !checkpointGitName(p) || paths[p] || extras[p] {
			return checkpointGitManifest{}, errors.New("file")
		}
		extras[p] = true
		paths[p] = true
	}
	blobs := map[string]checkpointGitBlob{}
	var blobTotal int64
	writeBlob := func(raw []byte) (string, error) {
		h := sha256.Sum256(raw)
		s := hex.EncodeToString(h[:])
		if _, ok := blobs[s]; ok {
			return s, nil
		}
		if int64(len(raw)) > gitMaxBlob {
			return "", errors.New("blob")
		}
		if blobTotal > gitMaxTotal-int64(len(raw)) {
			return "", errors.New("blob total")
		}
		oid := checkpointGitOID(raw)
		if e := os.MkdirAll(filepath.Join(stage, "data", "blobs"), 0700); e != nil {
			return "", e
		}
		if e := os.WriteFile(filepath.Join(stage, "data", "blobs", s), raw, 0600); e != nil {
			return "", e
		}
		blobs[s] = checkpointGitBlob{s, oid, int64(len(raw))}
		blobTotal += int64(len(raw))
		return s, nil
	}
	oids := make([]string, 0, len(index))
	seenOID := map[string]bool{}
	for _, x := range index {
		if !seenOID[x.OID] {
			seenOID[x.OID] = true
			oids = append(oids, x.OID)
		}
	}
	batchInput, e := checkpointGitBatchOIDInput(oids)
	if e != nil {
		return checkpointGitManifest{}, e
	}
	batchFile, e := checkpointGitTemp(o, stage, batchInput)
	if e != nil {
		return checkpointGitManifest{}, e
	}
	batchRaw, batchErr := gitBytesLimit(o, root, batchFile, gitMaxTotal+int64(len(oids))*checkpointGitBatchHeaderMax, "cat-file", "--batch")
	inputCloseErr := o.closeInput(batchFile)
	if batchErr != nil {
		return checkpointGitManifest{}, batchErr
	}
	if inputCloseErr != nil {
		return checkpointGitManifest{}, inputCloseErr
	}
	indexedBlobs, e := checkpointGitParseBatch(batchRaw, oids)
	if e != nil {
		return checkpointGitManifest{}, e
	}
	for _, x := range index {
		raw := indexedBlobs[x.OID]
		s, e := writeBlob(raw)
		if e != nil {
			return checkpointGitManifest{}, e
		}
		if blobs[s].OID != x.OID {
			return checkpointGitManifest{}, errors.New("index oid")
		}
	}
	files := make([]checkpointGitFile, 0, len(paths))
	keys := make([]string, 0, len(paths))
	for p := range paths {
		keys = append(keys, p)
	}
	sort.Strings(keys)
	for _, p := range keys {
		name := filepath.FromSlash(p)
		if checkpointGitRootPath(root, name) != nil {
			return checkpointGitManifest{}, errors.New("worktree link")
		}
		info, e := worktree.Lstat(name)
		if os.IsNotExist(e) && !extras[p] {
			files = append(files, checkpointGitFile{Path: p, State: "absent"})
			continue
		}
		if e != nil || !info.Mode().IsRegular() {
			return checkpointGitManifest{}, errors.New("worktree")
		}
		raw, e := checkpointGitReadFile(worktree, name, gitMaxBlob)
		if e != nil {
			return checkpointGitManifest{}, e
		}
		s, e := writeBlob(raw)
		if e != nil {
			return checkpointGitManifest{}, e
		}
		files = append(files, checkpointGitFile{p, "present", s, info.Mode()&0111 != 0})
	}
	stageRoot, e := os.OpenRoot(stage)
	if e != nil {
		return checkpointGitManifest{}, e
	}
	bs, bd, e := checkpointGitDigestFile(stageRoot, "history.bundle", gitMaxBundle)
	closeErr := stageRoot.Close()
	if e != nil || closeErr != nil {
		return checkpointGitManifest{}, errors.New("bundle")
	}
	list, e := gitBytes(o, root, nil, "bundle", "list-heads", bundle)
	if e != nil {
		return checkpointGitManifest{}, e
	}
	if !checkpointGitBundleClosure(list, head, refs) {
		return checkpointGitManifest{}, errors.New("closure")
	}
	if e = gitRun(o, root, nil, "bundle", "verify", bundle); e != nil {
		return checkpointGitManifest{}, e
	}
	listBlobs := make([]checkpointGitBlob, 0, len(blobs))
	for _, b := range blobs {
		listBlobs = append(listBlobs, b)
	}
	sort.Slice(listBlobs, func(i, j int) bool { return listBlobs[i].SHA256 < listBlobs[j].SHA256 })
	extraList := make([]string, 0, len(extras))
	for p := range extras {
		extraList = append(extraList, p)
	}
	sort.Strings(extraList)
	m := checkpointGitManifest{gitCheckpointSchema, "git-wip", in.CheckpointID, in.SystemVersion, in.SourceIdentity, in.Owner, in.Purpose, in.RetainUntil, in.NextAction, in.References, checkpointGitInfo{"sha1", head, attached, refs, checkpointGitBundle{"history.bundle", bs, bd}, index, files, extraList, listBlobs}}
	if e = checkpointGitSnapshotSource(o, root, m); e != nil {
		return checkpointGitManifest{}, e
	}
	if e = checkpointGitWriteManifest(stage, m); e != nil {
		return checkpointGitManifest{}, e
	}
	return m, nil
}

func checkpointGitVerifySource(o *checkpointGitOperation, root string, m checkpointGitManifest) error {
	if e := checkpointGitSourceOK(o, root); e != nil || checkpointGitVerifySnapshot(o, root) != nil {
		return errors.New("drift")
	}
	held, e := os.OpenRoot(root)
	if e != nil {
		return e
	}
	defer held.Close()
	h, e := gitText(o, root, "rev-parse", "HEAD")
	if e != nil || h != m.Git.HeadOID {
		return errors.New("drift")
	}
	for _, r := range m.Git.Refs {
		v, e := gitText(o, root, "rev-parse", r.Name)
		if e != nil || v != r.OID {
			return errors.New("drift")
		}
	}
	attached, e := gitText(o, root, "symbolic-ref", "HEAD")
	if e != nil || attached != m.Git.AttachedRef {
		return errors.New("drift")
	}
	idx, e := gitBytes(o, root, nil, "ls-files", "--stage", "-z")
	if e != nil || !bytes.Equal(idx, checkpointGitIndexBytes(m.Git.Index)) {
		return errors.New("drift")
	}
	for _, f := range m.Git.Files {
		name := filepath.FromSlash(f.Path)
		if checkpointGitRootPath(root, name) != nil {
			return errors.New("drift")
		}
		if f.State == "absent" {
			if _, e := held.Lstat(name); !os.IsNotExist(e) {
				return errors.New("drift")
			}
			continue
		}
		info, e := held.Lstat(name)
		raw, readErr := checkpointGitReadFile(held, name, gitMaxBlob)
		if e != nil || readErr != nil || !info.Mode().IsRegular() || checkpointGitSHA(raw) != f.SHA256 || (info.Mode()&0111 != 0) != f.Executable {
			return errors.New("drift")
		}
	}
	return nil
}

func checkpointGitSnapshotSource(o *checkpointGitOperation, root string, m checkpointGitManifest) error {
	ids := make(map[string]string, len(m.Git.Files)+5)
	for _, path := range []string{".", ".git", ".git/HEAD", ".git/config", ".git/index"} {
		id, err := localNativeID(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		ids[path] = id
	}
	for _, file := range m.Git.Files {
		if file.State != "present" || checkpointGitRootPath(root, filepath.FromSlash(file.Path)) != nil {
			continue
		}
		id, err := localNativeID(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			return err
		}
		ids["file:"+file.Path] = id
	}
	o.mu.Lock()
	o.sourceIDs = ids
	o.mu.Unlock()
	return nil
}

func checkpointGitVerifySnapshot(o *checkpointGitOperation, root string) error {
	o.mu.Lock()
	ids := make(map[string]string, len(o.sourceIDs))
	for key, value := range o.sourceIDs {
		ids[key] = value
	}
	o.mu.Unlock()
	if len(ids) == 0 {
		return nil
	}
	for key, want := range ids {
		name := key
		if strings.HasPrefix(name, "file:") {
			name = strings.TrimPrefix(name, "file:")
			if checkpointGitRootPath(root, filepath.FromSlash(name)) != nil {
				return errors.New("source link")
			}
		}
		got, err := localNativeID(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || got != want {
			return errors.New("source identity")
		}
	}
	return nil
}

func checkpointGitReconstruct(o *checkpointGitOperation, stage, p string, m checkpointGitManifest) error {
	payload, e := os.OpenRoot(p)
	if e != nil {
		return e
	}
	defer payload.Close()
	template := filepath.Join(stage, ".empty-template")
	if e := os.Mkdir(template, 0700); e != nil {
		return e
	}
	if e := gitRun(o, filepath.Dir(stage), nil, "init", "--template="+template, "--initial-branch=main", stage); e != nil {
		return e
	}
	if e := os.Remove(template); e != nil {
		return e
	}
	if e := checkpointGitCopyPayload(payload, "history.bundle", filepath.Join(stage, "history.bundle"), gitMaxBundle); e != nil {
		return e
	}
	stagePayload, e := os.OpenRoot(stage)
	if e != nil {
		return e
	}
	bundleSize, bundleHash, e := checkpointGitDigestFile(stagePayload, "history.bundle", gitMaxBundle)
	_ = stagePayload.Close()
	if e != nil || bundleSize != m.Git.Bundle.Size || bundleHash != m.Git.Bundle.SHA256 {
		return errors.New("bundle")
	}
	if e := gitRun(o, stage, nil, "bundle", "unbundle", filepath.Join(stage, "history.bundle")); e != nil {
		return e
	}
	for _, r := range m.Git.Refs {
		if e := gitRun(o, stage, nil, "update-ref", r.Name, r.OID); e != nil {
			return e
		}
	}
	if e := gitRun(o, stage, nil, "symbolic-ref", "HEAD", m.Git.AttachedRef); e != nil {
		return e
	}
	_ = os.Remove(filepath.Join(stage, "history.bundle"))
	batchRoot := filepath.Join(o.artifactRoot, "hash-input")
	if e := o.fixedGroup(batchRoot, "git-hash-input"); e != nil {
		return e
	}
	paths := make([]string, 0, len(m.Git.Blobs))
	for _, b := range m.Git.Blobs {
		copyPath := filepath.Join(batchRoot, b.SHA256)
		if e := checkpointGitCopyPayload(payload, "data/blobs/"+b.SHA256, copyPath, gitMaxBlob); e != nil {
			return e
		}
		paths = append(paths, copyPath)
	}
	batchInput, e := checkpointGitPathInput(paths)
	if e != nil {
		return e
	}
	batchFile, e := checkpointGitTemp(o, stage, batchInput)
	if e != nil {
		return e
	}
	hashed, hashErr := gitBytesLimit(o, stage, batchFile, int64(len(m.Git.Blobs))*41, "hash-object", "-w", "--no-filters", "--stdin-paths")
	inputCloseErr := o.closeInput(batchFile)
	if hashErr != nil || inputCloseErr != nil || !checkpointGitHashOutput(hashed, m.Git.Blobs) {
		return errors.New("blob")
	}
	if e := gitRun(o, stage, nil, "read-tree", "--empty"); e != nil {
		return e
	}
	stageRoot, e := os.OpenRoot(stage)
	if e != nil {
		return e
	}
	defer stageRoot.Close()
	var entries bytes.Buffer
	for _, x := range m.Git.Index {
		fmt.Fprintf(&entries, "%s %s 0\t%s\x00", x.Mode, x.OID, x.Path)
	}
	input, e := checkpointGitTemp(o, stage, entries.Bytes())
	if e != nil {
		return e
	}
	e = gitRun(o, stage, input, "update-index", "--add", "-z", "--index-info")
	inputCloseErr = o.closeInput(input)
	if e != nil {
		return e
	}
	if inputCloseErr != nil {
		return inputCloseErr
	}
	for _, f := range m.Git.Files {
		if f.State == "absent" {
			continue
		}
		raw, e := checkpointGitReadFile(payload, "data/blobs/"+f.SHA256, gitMaxBlob)
		if e != nil || checkpointGitSHA(raw) != f.SHA256 {
			return errors.New("blob")
		}
		name := filepath.FromSlash(f.Path)
		if e = stageRoot.MkdirAll(filepath.Dir(name), 0700); e != nil {
			return e
		}
		if e = stageRoot.WriteFile(name, raw, 0600); e != nil {
			return e
		}
		if f.Executable {
			if e = stageRoot.Chmod(name, 0700); e != nil {
				return e
			}
		}
	}
	return nil
}

// Validate actual pack/object availability in an isolated owned repository;
// list-heads/verify metadata alone is never package-payload proof.
func checkpointGitValidateBundle(o *checkpointGitOperation, p string, m checkpointGitManifest) error {
	repo := filepath.Join(o.artifactRoot, "bundle-verify")
	template := filepath.Join(o.artifactRoot, "empty-template")
	if err := o.fixedGroup(template, "empty-template"); err != nil {
		return err
	}
	if err := o.fixedGroup(repo, "git-verify"); err != nil {
		return err
	}
	if err := gitRun(o, o.artifactRoot, nil, "init", "--template="+template, "--initial-branch=main", repo); err != nil {
		return err
	}
	payload, err := os.OpenRoot(p)
	if err != nil {
		return err
	}
	defer payload.Close()
	bundle := filepath.Join(repo, "history.bundle")
	if err := checkpointGitCopyPayload(payload, "history.bundle", bundle, gitMaxBundle); err != nil {
		return err
	}
	owned, err := os.OpenRoot(repo)
	if err != nil {
		return err
	}
	size, digest, err := checkpointGitDigestFile(owned, "history.bundle", gitMaxBundle)
	_ = owned.Close()
	if err != nil || size != m.Git.Bundle.Size || digest != m.Git.Bundle.SHA256 {
		return errors.New("bundle")
	}
	if err := gitRun(o, repo, nil, "bundle", "verify", bundle); err != nil {
		return err
	}
	if err := gitRun(o, repo, nil, "bundle", "unbundle", bundle); err != nil {
		return err
	}
	if err := gitRun(o, repo, nil, "fsck", "--full", "--strict", "--no-reflogs"); err != nil {
		return err
	}
	for _, ref := range m.Git.Refs {
		if err := gitRun(o, repo, nil, "update-ref", ref.Name, ref.OID); err != nil {
			return err
		}
		typ, err := gitText(o, repo, "cat-file", "-t", ref.OID)
		if err != nil || typ != "commit" {
			return errors.New("bundle object")
		}
	}
	if err := gitRun(o, repo, nil, "update-ref", m.Git.AttachedRef, m.Git.HeadOID); err != nil {
		return err
	}
	typ, err := gitText(o, repo, "cat-file", "-t", m.Git.HeadOID)
	if err != nil || typ != "commit" {
		return errors.New("bundle head")
	}
	tree, err := gitBytes(o, repo, nil, "ls-tree", "-r", "-z", m.Git.HeadOID)
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, path := range checkpointGitTreePaths(tree) {
		known[path] = true
	}
	for _, entry := range m.Git.Index {
		known[entry.Path] = true
	}
	extras := map[string]bool{}
	for _, path := range m.Git.ExtraFiles {
		extras[path] = true
		known[path] = true
	}
	for _, file := range m.Git.Files {
		if !known[file.Path] {
			return errors.New("file union")
		}
		if extras[file.Path] && file.State != "present" {
			return errors.New("extra state")
		}
	}
	files := make(map[string]bool, len(m.Git.Files))
	for _, file := range m.Git.Files {
		files[file.Path] = true
	}
	for path := range known {
		if !files[path] {
			return errors.New("missing file union")
		}
	}
	return nil
}

func checkpointGitVerifyRestored(o *checkpointGitOperation, root string, m checkpointGitManifest) error {
	if e := checkpointGitVerifySource(o, root, m); e != nil {
		return e
	}
	if e := checkpointGitVerifyStageInventory(root, m); e != nil {
		return e
	}
	idx, e := gitBytes(o, root, nil, "ls-files", "--stage", "-z")
	if e != nil {
		return e
	}
	want := checkpointGitIndexBytes(m.Git.Index)
	if !bytes.Equal(idx, want) {
		return errors.New("index")
	}
	return nil
}

func checkpointGitVerifyStageInventory(root string, m checkpointGitManifest) error {
	expected := map[string]bool{}
	directories := map[string]bool{".": true}
	for _, f := range m.Git.Files {
		if f.State == "present" {
			expected[f.Path] = true
			for parent := filepath.ToSlash(filepath.Dir(f.Path)); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
				directories[parent] = true
			}
		}
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == filepath.Join(root, ".git") {
			return filepath.SkipDir
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return errors.New("inventory")
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if !directories[rel] {
				return errors.New("inventory")
			}
			return nil
		}
		if !expected[rel] {
			return errors.New("inventory")
		}
		delete(expected, rel)
		return nil
	})
	if err != nil || len(expected) != 0 {
		return errors.New("inventory")
	}
	return nil
}

func checkpointGitSourceOK(o *checkpointGitOperation, root string) error {
	if checkpointPlainDirectory(root) != nil {
		return errors.New("root")
	}
	gitDir := filepath.Join(root, ".git")
	info, e := os.Lstat(gitDir)
	if e != nil || !info.IsDir() || checkpointGitReparse(gitDir) {
		return errors.New("git")
	}
	gitRoot, e := os.OpenRoot(gitDir)
	if e != nil {
		return errors.New("git")
	}
	defer gitRoot.Close()
	entries, e := fs.ReadDir(gitRoot.FS(), ".")
	if e != nil {
		return errors.New("git metadata")
	}
	for _, entry := range entries {
		info, infoErr := entry.Info()
		if infoErr != nil || entry.Type()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) || checkpointGitReparse(filepath.Join(gitDir, entry.Name())) {
			return errors.New("git metadata")
		}
	}
	for _, name := range []string{"HEAD", "config", "index"} {
		metadata, metadataErr := gitRoot.Lstat(name)
		if metadataErr != nil || !metadata.Mode().IsRegular() || metadata.Mode()&os.ModeSymlink != 0 || checkpointGitReparse(filepath.Join(gitDir, name)) {
			return errors.New("git metadata")
		}
	}
	for _, name := range []string{"objects", "refs"} {
		metadata, metadataErr := gitRoot.Lstat(name)
		if metadataErr != nil || !metadata.IsDir() || metadata.Mode()&os.ModeSymlink != 0 || checkpointGitReparse(filepath.Join(gitDir, name)) || checkpointGitMetadataTree(o, gitRoot, gitDir, name) != nil {
			return errors.New("git metadata")
		}
	}
	if packed, packedErr := gitRoot.Lstat("packed-refs"); packedErr == nil && (!packed.Mode().IsRegular() || packed.Mode()&os.ModeSymlink != 0 || checkpointGitReparse(filepath.Join(gitDir, "packed-refs"))) {
		return errors.New("git metadata")
	} else if packedErr != nil && !errors.Is(packedErr, os.ErrNotExist) {
		return errors.New("git metadata")
	}
	configInfo, e := gitRoot.Lstat("config")
	if e != nil || !configInfo.Mode().IsRegular() {
		return errors.New("config")
	}
	raw, e := checkpointGitReadFile(gitRoot, "config", gitMaxManifest)
	if e != nil {
		return errors.New("config")
	}
	config := bytes.ToLower(raw)
	if checkpointGitUnsafeConfig(config) || bytes.Contains(config, []byte("promisor")) || bytes.Contains(config, []byte("partialclone")) {
		return errors.New("include")
	}
	for _, name := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "index.lock", "shallow", "commondir", "gitdir", "packed-refs.lock", "config.worktree", "refs/replace"} {
		if _, e := gitRoot.Lstat(filepath.FromSlash(name)); e == nil {
			return errors.New("unsupported git state")
		}
	}
	for _, name := range []string{"rebase-apply", "rebase-merge", "sequencer", "objects/info/alternates", "info/grafts"} {
		if _, e := gitRoot.Lstat(filepath.FromSlash(name)); e == nil {
			return errors.New("unsupported git state")
		}
	}
	if entries, readErr := fs.ReadDir(gitRoot.FS(), "objects/pack"); readErr == nil {
		for _, entry := range entries {
			if strings.HasSuffix(strings.ToLower(entry.Name()), ".promisor") {
				return errors.New("promisor")
			}
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return errors.New("pack")
	}
	format, e := gitText(o, root, "rev-parse", "--show-object-format")
	if e != nil || format != "sha1" {
		return errors.New("object format")
	}
	shallow, e := gitText(o, root, "rev-parse", "--is-shallow-repository")
	if e != nil || shallow != "false" {
		return errors.New("shallow")
	}
	if _, e = gitText(o, root, "symbolic-ref", "HEAD"); e != nil {
		return errors.New("detached")
	}
	if sparse, e := gitText(o, root, "config", "--bool", "--default=false", "core.sparseCheckout"); e != nil || sparse == "true" {
		return errors.New("sparse")
	}
	flags, e := gitBytes(o, root, nil, "ls-files", "-v", "-z")
	if e != nil {
		return e
	}
	for _, record := range bytes.Split(flags, []byte{0}) {
		if len(record) > 1 && record[1] == ' ' && ((record[0] >= 'a' && record[0] <= 'z') || record[0] == 'S' || record[0] == 'M' || record[0] == 'R') {
			return errors.New("index flags")
		}
	}
	debug, e := gitBytes(o, root, nil, "ls-files", "--debug", "-z")
	if e != nil {
		return e
	}
	if bytes.Contains(debug, []byte("flags: 2000")) || bytes.Contains(debug, []byte("flags: 4000")) || bytes.Contains(debug, []byte("flags: 8000")) {
		return errors.New("index flags")
	}
	tree, e := gitBytes(o, root, nil, "ls-tree", "-r", "-z", "HEAD")
	if e != nil {
		return e
	}
	for _, record := range bytes.Split(tree, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		fields := strings.Fields(string(bytes.SplitN(record, []byte{'\t'}, 2)[0]))
		if len(fields) < 2 || (fields[0] != "100644" && fields[0] != "100755") {
			return errors.New("tree mode")
		}
	}
	return nil
}

func checkpointGitMetadataTree(o *checkpointGitOperation, root *os.Root, rootPath, name string) error {
	return fs.WalkDir(root.FS(), name, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || o.ctx.Err() != nil {
			return errors.New("metadata")
		}
		if entry.Type()&os.ModeSymlink != 0 || checkpointGitReparse(filepath.Join(rootPath, filepath.FromSlash(path))) {
			return errors.New("metadata")
		}
		info, infoErr := entry.Info()
		if infoErr != nil || (!info.IsDir() && !info.Mode().IsRegular()) {
			return errors.New("metadata")
		}
		return nil
	})
}
func checkpointGitUnsafeConfig(raw []byte) bool {
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] != '[' {
			continue
		}
		end := bytes.IndexByte(line, ']')
		if end < 0 {
			continue
		}
		section := strings.TrimSpace(string(line[1:end]))
		if section == "include" || strings.HasPrefix(section, "includeif") {
			return true
		}
	}
	return false
}

func gitRun(o *checkpointGitOperation, root string, input *os.File, args ...string) error {
	tool, lookupErr := exec.LookPath("git")
	if lookupErr != nil {
		return lookupErr
	}
	spec := commandSpec{Executable: tool, Cwd: root, Args: checkpointGitArgs(root, args), OutputLimit: 1 << 20, Stdin: input, Env: []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_ATTR_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1"}}
	r, e := o.run(root, input, spec)
	if e != nil || r.ExitCode != 0 || r.Unresolved || r.TimedOut || r.Truncated {
		return errors.New("git")
	}
	return nil
}
func gitBytes(o *checkpointGitOperation, root string, input *os.File, args ...string) ([]byte, error) {
	return gitBytesLimit(o, root, input, 8<<20, args...)
}
func gitBytesLimit(o *checkpointGitOperation, root string, input *os.File, limit int64, args ...string) ([]byte, error) {
	tool, lookupErr := exec.LookPath("git")
	if lookupErr != nil {
		return nil, lookupErr
	}
	spec := commandSpec{Executable: tool, Cwd: root, Args: checkpointGitArgs(root, args), OutputLimit: int(limit), Stdin: input, Env: []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_ATTR_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1"}}
	r, e := o.run(root, input, spec)
	if e != nil || r.ExitCode != 0 || r.Unresolved || r.TimedOut || r.Truncated {
		return nil, errors.New("git")
	}
	return []byte(r.Output), nil
}
func checkpointGitArgs(root string, args []string) []string {
	prefix := []string{"--no-pager", "-c", "core.hooksPath=", "-c", "core.fsmonitor=false", "-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "core.longpaths=true", "-c", "core.attributesFile=" + os.DevNull, "-c", "protocol.file.allow=never"}
	if len(args) == 0 || args[0] != "init" {
		// Cwd is the already selected repository root. Relative arguments avoid
		// Git for Windows' separate GIT_DIR length limit without global config.
		prefix = append(prefix, "--git-dir=.git", "--work-tree=.")
	}
	return append(prefix, args...)
}

func checkpointGitRun(ctx context.Context, spec commandSpec) (commandResult, error) {
	if checkpointGitRunOverride != nil {
		return checkpointGitRunOverride(ctx, spec)
	}
	return runOwned(ctx, spec)
}
func gitText(o *checkpointGitOperation, root string, args ...string) (string, error) {
	b, e := gitBytes(o, root, nil, args...)
	return strings.TrimSpace(string(b)), e
}

func checkpointGitPathInput(paths []string) ([]byte, error) {
	if len(paths) > gitMaxFiles {
		return nil, errors.New("batch paths")
	}
	var input bytes.Buffer
	for _, path := range paths {
		if path == "" || strings.ContainsAny(path, "\x00\r\n") {
			return nil, errors.New("batch paths")
		}
		input.WriteString(path)
		input.WriteByte('\n')
	}
	return input.Bytes(), nil
}

func checkpointGitHashOutput(raw []byte, want []checkpointGitBlob) bool {
	if len(raw) != len(want)*41 {
		return false
	}
	for i, blob := range want {
		line := raw[i*41 : (i+1)*41]
		if line[40] != '\n' || string(line[:40]) != blob.OID {
			return false
		}
	}
	return true
}

func checkpointGitTemp(o *checkpointGitOperation, dir string, b []byte) (*os.File, error) {
	if err := o.guard(); err != nil {
		return nil, err
	}
	g := o.groupAt(dir)
	if g == nil || (g.Layout != "git-capture" && g.Layout != "git-restore") {
		o.freeze("input group unavailable")
		return nil, errors.New("input group unavailable")
	}
	f, e := os.CreateTemp(dir, ".git-input-")
	if e != nil {
		o.freeze("input creation failed")
		return nil, e
	}
	if g.Issued == nil {
		g.Issued = map[string]bool{}
	}
	g.Issued[filepath.Base(f.Name())] = true
	e = o.record("core-input-issued", 0, 0, f.Name())
	if e == nil {
		var n int
		n, e = f.Write(b)
		if e == nil && n != len(b) {
			e = io.ErrShortWrite
		}
	}
	if e == nil {
		e = f.Sync()
	}
	if e == nil {
		_, e = f.Seek(0, 0)
	}
	if e != nil {
		_ = f.Close()
		o.freeze("input receipt or write failed")
		return nil, e
	}
	return f, nil
}

func (o *checkpointGitOperation) closeInput(f *os.File) error {
	if err := f.Close(); err != nil {
		o.freeze("input close failed")
		return err
	}
	if err := o.guard(); err != nil {
		return err
	}
	g := o.groupAt(filepath.Dir(f.Name()))
	name := filepath.Base(f.Name())
	if g == nil || !g.Issued[name] {
		o.freeze("input issuance unavailable")
		return errors.New("input issuance unavailable")
	}
	if err := os.Remove(f.Name()); err != nil {
		o.freeze("input retirement failed")
		return err
	}
	delete(g.Issued, name)
	return o.record("core-input-closed", 0, 0, f.Name())
}
func checkpointGitSHA(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func checkpointGitOID(b []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(b))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
func checkpointGitName(p string) bool {
	if p == "" || !filepath.IsLocal(filepath.FromSlash(p)) || strings.ContainsAny(p, "\\\x00\t\r\n") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." || strings.TrimRight(part, ". ") != part || strings.EqualFold(part, ".git") || strings.EqualFold(part, ".ocmi") || strings.EqualFold(part, ".ocmi-checkpoint") || strings.HasPrefix(strings.ToLower(part), ".git-input-") || strings.EqualFold(part, ".empty-template") || strings.ContainsAny(part, `<>:"|?*`) || checkpointGitWindowsDevice(part) || checkpointGitSensitiveName(part) {
			return false
		}
		for _, r := range part {
			if r < 0x20 || r == 0x7f {
				return false
			}
		}
	}
	return true
}

func checkpointGitSensitiveName(part string) bool {
	lower := strings.ToLower(part)
	if strings.HasPrefix(lower, ".env") && lower != ".env.example" && lower != ".env.sample" && lower != ".env.template" {
		return true
	}
	for _, suffix := range []string{".db", ".sqlite", ".sqlite3", ".wal", ".shm", "-wal", "-shm", ".key", ".pem", ".pfx", ".p12"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func checkpointGitRootPath(root, name string) error {
	if !checkpointGitName(filepath.ToSlash(name)) {
		return errors.New("path")
	}
	path := filepath.Join(root, name)
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if os.IsNotExist(err) && current != root {
			continue
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || checkpointGitReparse(current) {
			return errors.New("link")
		}
		if current == root {
			return nil
		}
		if parent := filepath.Dir(current); parent == current {
			return errors.New("path")
		}
	}
}

func checkpointGitWindowsDevice(part string) bool {
	base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" {
		return true
	}
	return len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9'
}
func checkpointGitParseIndex(b []byte) ([]checkpointGitIndex, error) {
	var out []checkpointGitIndex
	seen := map[string]bool{}
	for _, r := range bytes.Split(b, []byte{0}) {
		if len(r) == 0 {
			continue
		}
		parts := bytes.SplitN(r, []byte{'\t'}, 2)
		if len(parts) != 2 {
			return nil, errors.New("index")
		}
		f := strings.Fields(string(parts[0]))
		if len(f) != 3 || f[2] != "0" || !checkpointGitName(string(parts[1])) || seen[string(parts[1])] {
			return nil, errors.New("index")
		}
		seen[string(parts[1])] = true
		out = append(out, checkpointGitIndex{string(parts[1]), f[0], f[1]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
func checkpointGitIndexBytes(x []checkpointGitIndex) []byte {
	var b bytes.Buffer
	for _, v := range x {
		fmt.Fprintf(&b, "%s %s 0\t%s\x00", v.Mode, v.OID, v.Path)
	}
	return b.Bytes()
}
func checkpointGitTreePaths(b []byte) []string {
	var out []string
	for _, r := range bytes.Split(b, []byte{0}) {
		p := bytes.SplitN(r, []byte{'\t'}, 2)
		if len(p) == 2 {
			out = append(out, string(p[1]))
		}
	}
	return out
}
func checkpointGitBundleClosure(b []byte, head string, refs []checkpointGitRef) bool {
	got := map[string]string{}
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		f := strings.SplitN(l, " ", 2)
		if len(f) == 2 {
			got[f[1]] = f[0]
		}
	}
	if len(got) != len(refs)+1 || got["HEAD"] != head {
		return false
	}
	for _, r := range refs {
		if got[r.Name] != r.OID {
			return false
		}
	}
	return true
}
func checkpointGitWriteManifest(stage string, m checkpointGitManifest) error {
	raw, e := json.Marshal(m)
	if e != nil || len(raw) > gitMaxManifest {
		return errors.New("manifest")
	}
	if e = os.WriteFile(filepath.Join(stage, "manifest.json"), raw, 0600); e != nil {
		return e
	}
	h := sha256.Sum256(raw)
	complete, _ := json.Marshal(struct {
		SchemaVersion  int    `json:"schemaVersion"`
		ManifestSHA256 string `json:"manifestSha256"`
	}{gitCheckpointSchema, hex.EncodeToString(h[:])})
	return os.WriteFile(filepath.Join(stage, "COMPLETE"), complete, 0600)
}
func checkpointGitReadPackage(o *checkpointGitOperation, path string) (checkpointGitManifest, string, error) {
	if !filepath.IsAbs(path) || checkpointPlainDirectory(path) != nil {
		return checkpointGitManifest{}, "", errors.New("package")
	}
	if o == nil {
		return checkpointGitManifest{}, "", errors.New("operation")
	}
	held, e := os.OpenRoot(path)
	if e != nil {
		return checkpointGitManifest{}, "", e
	}
	defer held.Close()
	raw, e := checkpointGitReadFile(held, "manifest.json", gitMaxManifest)
	if e != nil {
		return checkpointGitManifest{}, "", e
	}
	var m checkpointGitManifest
	if !checkpointGitNoCaseAlias(raw) || !checkpointStrictJSON(raw, &m) || !checkpointGitManifestValid(m) {
		return checkpointGitManifest{}, "", errors.New("manifest")
	}
	h := sha256.Sum256(raw)
	var c struct {
		SchemaVersion  int    `json:"schemaVersion"`
		ManifestSHA256 string `json:"manifestSha256"`
	}
	cr, e := checkpointGitReadFile(held, "COMPLETE", gitMaxManifest)
	if e != nil || !checkpointGitNoCaseAlias(cr) || !checkpointStrictJSON(cr, &c) || c.SchemaVersion != gitCheckpointSchema || !strings.EqualFold(c.ManifestSHA256, hex.EncodeToString(h[:])) {
		return checkpointGitManifest{}, "", errors.New("complete")
	}
	expected := map[string]bool{"manifest.json": true, "COMPLETE": true, "history.bundle": true}
	dirs := map[string]bool{"data": true, "data/blobs": true}
	for _, b := range m.Git.Blobs {
		expected["data/blobs/"+b.SHA256] = true
	}
	if checkpointValidateClosedInventory(path, expected, dirs) != nil {
		return checkpointGitManifest{}, "", errors.New("inventory")
	}
	for _, b := range m.Git.Blobs {
		raw, e := checkpointGitReadFile(held, "data/blobs/"+b.SHA256, gitMaxBlob)
		if e != nil || checkpointGitSHA(raw) != b.SHA256 || checkpointGitOID(raw) != b.OID || int64(len(raw)) != b.Size {
			return checkpointGitManifest{}, "", errors.New("blob")
		}
	}
	sz, d, e := checkpointGitDigestFile(held, "history.bundle", gitMaxBundle)
	if e != nil || sz != m.Git.Bundle.Size || d != m.Git.Bundle.SHA256 {
		return checkpointGitManifest{}, "", errors.New("bundle")
	}
	return m, hex.EncodeToString(h[:]), nil
}
func checkpointGitReadFile(held *os.Root, name string, max int64) ([]byte, error) {
	f, err := checkpointGitOpenPayload(held, name, max)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil || int64(len(b)) > max {
		return nil, errors.New("payload")
	}
	return b, nil
}

func checkpointGitDigestFile(held *os.Root, name string, max int64) (int64, string, error) {
	f, err := checkpointGitOpenPayload(held, name, max)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, max+1))
	if err != nil || n > max {
		return 0, "", errors.New("payload")
	}
	return n, hex.EncodeToString(h.Sum(nil)), nil
}

func checkpointGitOpenPayload(held *os.Root, name string, max int64) (*os.File, error) {
	info, err := held.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > max {
		return nil, errors.New("payload")
	}
	f, err := held.Open(name)
	if err != nil {
		return nil, err
	}
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Size() != info.Size() {
		_ = f.Close()
		return nil, errors.New("payload")
	}
	if _, err = localNativeIDFile(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func checkpointGitCopyPayload(held *os.Root, name, destination string, max int64) error {
	source, err := checkpointGitOpenPayload(held, name, max)
	if err != nil {
		return err
	}
	defer source.Close()
	dest, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(dest, io.LimitReader(source, max+1))
	closeErr := dest.Close()
	if copyErr != nil || closeErr != nil || n > max {
		_ = os.Remove(destination)
		return errors.New("payload")
	}
	return nil
}
func checkpointGitManifestValid(m checkpointGitManifest) bool {
	return checkpointGitManifestSemantics(m)
}
