package internal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	checkpointSchema   = 1
	checkpointMaxFiles = 512
	checkpointMaxFile  = int64(8 << 20)
	checkpointMaxTotal = int64(32 << 20)
	checkpointMaxMeta  = 1 << 20
)

// Test-only seams. Requests cannot select them.
var checkpointCaptureBeforeVerify func()
var checkpointRestoreBeforeActivation func() error

type checkpointReference struct {
	ID         string `json:"id"`
	Required   bool   `json:"required"`
	Status     string `json:"status"`
	NextAction string `json:"nextAction"`
}

type checkpointFile struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Size       int64  `json:"size"`
	Executable bool   `json:"executable"`
}

type checkpointManifest struct {
	SchemaVersion  int                   `json:"schemaVersion"`
	Kind           string                `json:"kind"`
	CheckpointID   string                `json:"checkpointId"`
	SystemVersion  string                `json:"systemVersion"`
	SourceIdentity string                `json:"sourceIdentity"`
	Owner          string                `json:"owner"`
	Purpose        string                `json:"purpose"`
	RetainUntil    string                `json:"retainUntil"`
	NextAction     string                `json:"nextAction"`
	Files          []checkpointFile      `json:"files"`
	References     []checkpointReference `json:"references,omitempty"`
}

type checkpointCapture struct {
	PackagePath    string                `json:"packagePath"`
	CheckpointID   string                `json:"checkpointId"`
	SystemVersion  string                `json:"systemVersion"`
	SourceIdentity string                `json:"sourceIdentity"`
	Owner          string                `json:"owner"`
	Purpose        string                `json:"purpose"`
	RetainUntil    string                `json:"retainUntil"`
	NextAction     string                `json:"nextAction"`
	Files          []string              `json:"files"`
	References     []checkpointReference `json:"references"`
}

func checkpoint(r Request) Response {
	if r.Actor != "main" || r.ExpectedRevision != 0 {
		return bad("CHECKPOINT_REFUSED", 0)
	}
	// Lexical normalization keeps selected-root aliases equivalent without
	// resolving links (which must remain a refusal at the boundary).
	r.ProjectRoot = filepath.Clean(r.ProjectRoot)
	switch r.Action {
	case "checkpoint.capture":
		return checkpointCaptureAction(r)
	case "checkpoint.inspect":
		return checkpointInspectAction(r)
	case "checkpoint.restore":
		return checkpointRestoreAction(r)
	case "checkpoint.git.capture":
		return checkpointGitCaptureAction(r)
	case "checkpoint.git.inspect":
		return checkpointGitInspectAction(r)
	case "checkpoint.git.restore":
		return checkpointGitRestoreAction(r)
	default:
		return bad("UNKNOWN_ACTION", 0)
	}
}

func checkpointCaptureAction(r Request) (out Response) {
	var input checkpointCapture
	if json.Unmarshal(r.Payload, &input) != nil || !checkpointText(input.PackagePath, 4096) {
		return bad("INVALID_CHECKPOINT", 0)
	}
	if err := checkpointSourceRoot(r.ProjectRoot); err != nil {
		return bad("CHECKPOINT_SOURCE_REFUSED", 0)
	}
	if err := checkpointOutputPath(input.PackagePath); err != nil {
		return bad("CHECKPOINT_DESTINATION_REFUSED", 0)
	}
	manifest, sources, err := checkpointManifestForCapture(r.ProjectRoot, input)
	if err != nil {
		return bad("INVALID_CHECKPOINT", 0)
	}
	identities := make([]string, len(sources))
	for i, source := range sources {
		identities[i], err = localNativeID(source)
		if err != nil {
			return bad("CHECKPOINT_SOURCE_REFUSED", 0)
		}
	}
	stage, err := os.MkdirTemp(filepath.Dir(input.PackagePath), ".ocmi-checkpoint-")
	if err != nil {
		return bad("CHECKPOINT_UNAVAILABLE", 0)
	}
	defer func() {
		if stage != "" {
			if cleanupErr := os.RemoveAll(stage); cleanupErr != nil {
				primaryCode := out.Code
				out = bad("CHECKPOINT_CLEANUP_REQUIRED", 0)
				out.Data = map[string]any{"primaryCode": primaryCode, "ownedStage": stage, "nextAction": "remove the declared owned checkpoint stage"}
			}
		}
	}()
	if err = os.Chmod(stage, 0700); err == nil {
		err = checkpointStageMarker(stage, manifest)
	}
	if err == nil {
		err = checkpointWritePackage(stage, manifest, r.ProjectRoot)
	}
	if err == nil && checkpointCaptureBeforeVerify != nil {
		checkpointCaptureBeforeVerify()
	}
	if err == nil {
		for i, source := range sources {
			identity, identityErr := localNativeID(source)
			if size, digest, _, checkErr := checkpointDigestFromRoot(r.ProjectRoot, manifest.Files[i].Path); identityErr != nil || checkErr != nil || identity != identities[i] || size != manifest.Files[i].Size || digest != manifest.Files[i].SHA256 {
				err = errors.New("source drift")
				break
			}
		}
	}
	if err == nil {
		_, _, err = checkpointReadPackage(stage)
	}
	if err == nil {
		err = checkpointOutputPath(input.PackagePath)
	}
	if err != nil {
		return bad("CHECKPOINT_CAPTURE_REFUSED", 0)
	}
	if err = os.Rename(stage, input.PackagePath); err != nil {
		return bad("CHECKPOINT_DESTINATION_REFUSED", 0)
	}
	stage = ""
	return good("CHECKPOINT_CAPTURED", 0, checkpointData(manifest, checkpointManifestDigest(manifest)))
}

func checkpointInspectAction(r Request) Response {
	path, ok := checkpointPathPayload(r.Payload)
	if !ok {
		return bad("INVALID_CHECKPOINT", 0)
	}
	manifest, digest, err := checkpointReadPackage(path)
	if err != nil {
		return bad("CHECKPOINT_INVALID", 0)
	}
	return good("CHECKPOINT_INSPECTED", 0, checkpointData(manifest, digest))
}

func checkpointRestoreAction(r Request) (out Response) {
	path, ok := checkpointPathPayload(r.Payload)
	if !ok {
		return bad("INVALID_CHECKPOINT", 0)
	}
	manifest, digest, err := checkpointReadPackage(path)
	if err != nil {
		return bad("CHECKPOINT_INVALID", 0)
	}
	if err = emptyImportRoot(r.ProjectRoot); err != nil {
		return bad("CHECKPOINT_DESTINATION_REFUSED", 0)
	}
	parent := filepath.Dir(r.ProjectRoot)
	if err = checkpointPlainDirectory(parent); err != nil {
		return bad("CHECKPOINT_DESTINATION_REFUSED", 0)
	}
	stage, err := os.MkdirTemp(parent, ".ocmi-checkpoint-restore-")
	if err != nil {
		return bad("CHECKPOINT_UNAVAILABLE", 0)
	}
	defer func() {
		if stage != "" {
			if cleanupErr := os.RemoveAll(stage); cleanupErr != nil {
				primaryCode := out.Code
				out = bad("CHECKPOINT_CLEANUP_REQUIRED", 0)
				out.Data = map[string]any{"primaryCode": primaryCode, "ownedStage": stage, "nextAction": "remove the declared owned checkpoint stage"}
			}
		}
	}()
	if err = os.Chmod(stage, 0700); err == nil {
		err = checkpointStageMarker(stage, manifest)
	}
	if err == nil {
		err = checkpointCopyPackage(path, stage, manifest)
	}
	if err != nil {
		return bad("CHECKPOINT_RESTORE_REFUSED", 0)
	}
	if checkpointRestoreBeforeActivation != nil {
		if err = checkpointRestoreBeforeActivation(); err != nil {
			return bad("CHECKPOINT_RESTORE_REFUSED", 0)
		}
	}
	if _, verifyDigest, verifyErr := checkpointReadPackage(path); verifyErr != nil || verifyDigest != digest {
		return bad("CHECKPOINT_RESTORE_REFUSED", 0)
	}
	if err = checkpointVerifyPayload(stage, manifest); err != nil {
		return bad("CHECKPOINT_RESTORE_REFUSED", 0)
	}
	if err = os.Remove(filepath.Join(stage, ".checkpoint-stage.json")); err != nil {
		return bad("CHECKPOINT_CLEANUP_REQUIRED", 0)
	}
	if err = emptyImportRoot(r.ProjectRoot); err != nil || checkpointPlainDirectory(parent) != nil {
		return bad("CHECKPOINT_DESTINATION_REFUSED", 0)
	}
	backup := stage + "-empty-root"
	if err = os.Rename(r.ProjectRoot, backup); err != nil {
		return bad("CHECKPOINT_DESTINATION_REFUSED", 0)
	}
	if err = os.Rename(stage, r.ProjectRoot); err != nil {
		_ = os.Rename(backup, r.ProjectRoot)
		return bad("CHECKPOINT_DESTINATION_REFUSED", 0)
	}
	stage = ""
	if err = os.Remove(backup); err != nil {
		return bad("CHECKPOINT_DESTINATION_REFUSED", 0)
	}
	return good("CHECKPOINT_RESTORED", 0, checkpointData(manifest, digest))
}

func checkpointPathPayload(payload json.RawMessage) (string, bool) {
	var p struct{ PackagePath string }
	if json.Unmarshal(payload, &p) != nil || !filepath.IsAbs(p.PackagePath) || !checkpointText(p.PackagePath, 4096) {
		return "", false
	}
	return filepath.Clean(p.PackagePath), true
}

func checkpointManifestForCapture(root string, input checkpointCapture) (checkpointManifest, []string, error) {
	if !checkpointText(input.CheckpointID, 256) || !checkpointText(input.SystemVersion, 256) || !checkpointText(input.SourceIdentity, 256) || !checkpointText(input.Owner, 4096) || !checkpointText(input.Purpose, 4096) || !checkpointText(input.RetainUntil, 4096) || !checkpointText(input.NextAction, 4096) || len(input.Files) == 0 || len(input.Files) > checkpointMaxFiles || len(input.References) > 128 || !checkpointReferences(input.References) {
		return checkpointManifest{}, nil, errors.New("metadata")
	}
	files := make([]checkpointFile, 0, len(input.Files))
	sources := make([]string, 0, len(input.Files))
	seen := map[string]string{}
	var total int64
	for _, name := range input.Files {
		if !checkpointName(name) || checkpointCollision(seen, name) {
			return checkpointManifest{}, nil, errors.New("name")
		}
		path, err := checkpointRootFile(root, name)
		if err != nil {
			return checkpointManifest{}, nil, err
		}
		size, digest, executable, err := checkpointDigestFromRoot(root, name)
		if err != nil || size > checkpointMaxFile || total+size > checkpointMaxTotal {
			return checkpointManifest{}, nil, errors.New("size")
		}
		total += size
		files = append(files, checkpointFile{Path: name, SHA256: digest, Size: size, Executable: executable})
		sources = append(sources, path)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	// Sources are re-resolved from the sorted manifest to make drift verification exact.
	sources = sources[:0]
	for _, file := range files {
		path, err := checkpointRootFile(root, file.Path)
		if err != nil {
			return checkpointManifest{}, nil, err
		}
		sources = append(sources, path)
	}
	return checkpointManifest{SchemaVersion: checkpointSchema, Kind: "files", CheckpointID: input.CheckpointID, SystemVersion: input.SystemVersion, SourceIdentity: input.SourceIdentity, Owner: input.Owner, Purpose: input.Purpose, RetainUntil: input.RetainUntil, NextAction: input.NextAction, Files: files, References: input.References}, sources, nil
}

func checkpointWritePackage(stage string, manifest checkpointManifest, sourceRoot string) error {
	data := filepath.Join(stage, "data")
	if err := os.Mkdir(data, 0700); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(stage, ".checkpoint-stage.json")); err != nil {
		return err
	}
	for _, file := range manifest.Files {
		target := filepath.Join(data, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		if err := checkpointCopyFileFromRoot(sourceRoot, file.Path, target, file.Executable); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(manifest)
	if err != nil || len(raw) > checkpointMaxMeta {
		return errors.New("manifest")
	}
	if err = os.WriteFile(filepath.Join(stage, "manifest.json"), raw, 0600); err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	complete, _ := json.Marshal(struct {
		SchemaVersion  int    `json:"schemaVersion"`
		ManifestSHA256 string `json:"manifestSha256"`
	}{checkpointSchema, hex.EncodeToString(digest[:])})
	return os.WriteFile(filepath.Join(stage, "COMPLETE"), complete, 0600)
}

func checkpointReadPackage(path string) (checkpointManifest, string, error) {
	if !filepath.IsAbs(path) || checkpointPlainDirectory(path) != nil {
		return checkpointManifest{}, "", errors.New("package")
	}
	raw, err := checkpointReadBoundedRegular(path, "manifest.json", checkpointMaxMeta)
	if err != nil || len(raw) == 0 {
		return checkpointManifest{}, "", errors.New("manifest")
	}
	var manifest checkpointManifest
	if !checkpointStrictJSON(raw, &manifest) || !checkpointManifestValid(manifest) {
		return checkpointManifest{}, "", errors.New("manifest")
	}
	digest := sha256.Sum256(raw)
	var complete struct {
		SchemaVersion  int    `json:"schemaVersion"`
		ManifestSHA256 string `json:"manifestSha256"`
	}
	completeRaw, err := checkpointReadBoundedRegular(path, "COMPLETE", checkpointMaxMeta)
	if err != nil || !checkpointStrictJSON(completeRaw, &complete) || complete.SchemaVersion != checkpointSchema || !strings.EqualFold(complete.ManifestSHA256, hex.EncodeToString(digest[:])) {
		return checkpointManifest{}, "", errors.New("complete")
	}
	expected := map[string]bool{"manifest.json": true, "COMPLETE": true}
	expectedDirs := map[string]bool{"data": true}
	for _, file := range manifest.Files {
		rel := "data/" + file.Path
		expected[rel] = true
		for parent := filepath.ToSlash(filepath.Dir(rel)); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
			expectedDirs[parent] = true
		}
	}
	// Validate links and exact inventory before opening any payload data.
	err = checkpointValidateClosedInventory(path, expected, expectedDirs)
	if err != nil {
		return checkpointManifest{}, "", err
	}
	for _, file := range manifest.Files {
		rel := "data/" + file.Path
		size, sum, executable, err := checkpointDigestFromRoot(path, rel)
		if err != nil || size != file.Size || sum != file.SHA256 || executable != file.Executable {
			return checkpointManifest{}, "", errors.New("data")
		}
	}
	return manifest, hex.EncodeToString(digest[:]), nil
}

func checkpointValidateClosedInventory(path string, expected map[string]bool, expectedDirs map[string]bool) error {
	return filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == path {
			return nil
		}
		rel, err := filepath.Rel(path, current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("link")
		}
		if entry.IsDir() && expectedDirs[rel] {
			return nil
		}
		if !entry.Type().IsRegular() || !expected[rel] {
			return errors.New("inventory")
		}
		return nil
	})
}

func checkpointCopyPackage(packagePath, target string, manifest checkpointManifest) error {
	for _, file := range manifest.Files {
		destination := filepath.Join(target, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			return err
		}
		if err := checkpointCopyFileFromRoot(packagePath, filepath.ToSlash(filepath.Join("data", file.Path)), destination, file.Executable); err != nil {
			return err
		}
	}
	return nil
}

func checkpointVerifyPayload(root string, manifest checkpointManifest) error {
	expected := map[string]bool{}
	expectedDirs := map[string]bool{}
	for _, file := range manifest.Files {
		expected[file.Path] = true
		for parent := filepath.ToSlash(filepath.Dir(file.Path)); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
			expectedDirs[parent] = true
		}
	}
	if _, markerErr := os.Lstat(filepath.Join(root, ".checkpoint-stage.json")); markerErr == nil {
		expected[".checkpoint-stage.json"] = true
	}
	if err := checkpointValidateClosedInventory(root, expected, expectedDirs); err != nil {
		return err
	}
	for _, file := range manifest.Files {
		size, digest, executable, err := checkpointDigestFromRoot(root, file.Path)
		if err != nil || size != file.Size || digest != file.SHA256 || executable != file.Executable {
			return errors.New("staged payload changed")
		}
	}
	return nil
}

func checkpointDigestFromRoot(rootPath, name string) (int64, string, bool, error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return 0, "", false, err
	}
	defer root.Close()
	f, err := root.Open(filepath.FromSlash(name))
	if err != nil {
		return 0, "", false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > checkpointMaxFile {
		return 0, "", false, errors.New("payload")
	}
	h := sha256.New()
	size, err := io.Copy(h, io.LimitReader(f, checkpointMaxFile+1))
	if err != nil || size != info.Size() {
		return 0, "", false, errors.New("payload")
	}
	return size, hex.EncodeToString(h.Sum(nil)), info.Mode()&0111 != 0, nil
}

func checkpointData(m checkpointManifest, digest string) map[string]any {
	return map[string]any{"checkpointId": m.CheckpointID, "manifestSha256": digest, "fileCount": len(m.Files), "nextAction": m.NextAction, "references": m.References, "readyToResume": false, "state": "requires_rebind"}
}

func checkpointManifestDigest(m checkpointManifest) string {
	raw, _ := json.Marshal(m)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func checkpointText(s string, max int) bool { return s != "" && len(s) <= max }

func checkpointReferences(refs []checkpointReference) bool {
	seen := map[string]bool{}
	for _, r := range refs {
		if !checkpointText(r.ID, 256) || !checkpointText(r.NextAction, 4096) || seen[r.ID] || (r.Status != "available" && r.Status != "missing" && r.Status != "unverified") {
			return false
		}
		seen[r.ID] = true
	}
	return true
}

func checkpointManifestValid(m checkpointManifest) bool {
	if m.SchemaVersion != checkpointSchema || m.Kind != "files" || !checkpointText(m.CheckpointID, 256) || !checkpointText(m.SystemVersion, 256) || !checkpointText(m.SourceIdentity, 256) || !checkpointText(m.Owner, 4096) || !checkpointText(m.Purpose, 4096) || !checkpointText(m.RetainUntil, 4096) || !checkpointText(m.NextAction, 4096) || len(m.Files) == 0 || len(m.Files) > checkpointMaxFiles || len(m.References) > 128 || !checkpointReferences(m.References) {
		return false
	}
	seen := map[string]string{}
	var total int64
	for _, f := range m.Files {
		if !checkpointName(f.Path) || checkpointCollision(seen, f.Path) || len(f.SHA256) != 64 || f.Size < 0 || f.Size > checkpointMaxFile || total+f.Size > checkpointMaxTotal {
			return false
		}
		total += f.Size
	}
	return true
}

func checkpointName(name string) bool {
	if !portableName(name) || strings.EqualFold(name, "COMPLETE") || strings.EqualFold(name, "manifest.json") {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(name), "/") {
		if strings.EqualFold(part, ".codex") {
			return false
		}
	}
	return true
}
func checkpointCollision(seen map[string]string, name string) bool {
	key := strings.ToLower(name)
	for old := range seen {
		if key == old || strings.HasPrefix(key, old+"/") || strings.HasPrefix(old, key+"/") {
			return true
		}
	}
	seen[key] = name
	return false
}

func checkpointSourceRoot(root string) error {
	if checkpointPlainDirectory(root) != nil {
		return errors.New("root")
	}
	if _, err := os.Lstat(filepath.Join(root, ".git")); err == nil {
		return errors.New("git")
	}
	return nil
}
func checkpointPlainDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("directory")
	}
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err = os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("link")
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return nil
}
func checkpointOutputPath(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("path")
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return errors.New("exists")
	}
	return checkpointPlainDirectory(filepath.Dir(path))
}
func checkpointRootFile(root, name string) (string, error) {
	if !checkpointName(name) {
		return "", errors.New("name")
	}
	root = filepath.Clean(root)
	path := filepath.Join(root, filepath.FromSlash(name))
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("link")
		}
		if current == root {
			break
		}
		if filepath.Dir(current) == current {
			return "", errors.New("path escaped root")
		}
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("file")
	}
	return path, nil
}
func checkpointDigest(path string) (int64, string, bool, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return 0, "", false, errors.New("file")
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, "", false, err
	}
	defer f.Close()
	h := sha256.New()
	size, err := io.Copy(h, io.LimitReader(f, checkpointMaxFile+1))
	if err != nil || size > checkpointMaxFile {
		return 0, "", false, errors.New("size")
	}
	return size, hex.EncodeToString(h.Sum(nil)), info.Mode()&0111 != 0, nil
}

func checkpointReadBoundedRegular(root, name string, max int) ([]byte, error) {
	path := filepath.Join(root, filepath.FromSlash(name))
	if name != "manifest.json" && name != "COMPLETE" {
		return nil, errors.New("metadata name")
	}
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("metadata link")
		}
		if current == root {
			break
		}
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > int64(max) {
		return nil, errors.New("metadata size")
	}
	before, err := localNativeID(path)
	if err != nil {
		return nil, err
	}
	held, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer held.Close()
	f, err := held.Open(filepath.FromSlash(name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, int64(max)+1))
	if err != nil || len(data) > max {
		return nil, errors.New("metadata read")
	}
	after, err := localNativeID(path)
	if err != nil || after != before {
		return nil, errors.New("metadata changed")
	}
	return data, nil
}

func checkpointStageMarker(stage string, manifest checkpointManifest) error {
	marker, err := json.Marshal(struct{ Owner, Purpose, RetainUntil string }{manifest.Owner, manifest.Purpose, manifest.RetainUntil})
	if err != nil || len(marker) > checkpointMaxMeta {
		return errors.New("stage marker")
	}
	return os.WriteFile(filepath.Join(stage, ".checkpoint-stage.json"), marker, 0600)
}
func checkpointCopyFile(source, target string, executable bool) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, io.LimitReader(in, checkpointMaxFile+1))
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if executable {
		return os.Chmod(target, 0700)
	}
	return nil
}

// Payload bytes are read through an opened root, not a re-resolved absolute
// path. This confines component traversal even if an ancestor is swapped
// between validation and the actual copy.
func checkpointCopyFileFromRoot(rootPath, name, target string, executable bool) error {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	in, err := root.Open(filepath.FromSlash(name))
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > checkpointMaxFile {
		return errors.New("payload unavailable")
	}
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, io.LimitReader(in, checkpointMaxFile+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	if executable {
		return os.Chmod(target, 0700)
	}
	return nil
}
func checkpointStrictJSON(raw []byte, target any) bool {
	// encoding/json deliberately accepts duplicate object names; checkpoint
	// manifests are integrity-digested exact bytes, so accepting an ambiguous semantic
	// view would let a later decoder disagree with the verifier.
	tokens := json.NewDecoder(bytes.NewReader(raw))
	if !checkpointJSONValue(tokens) {
		return false
	}
	if _, err := tokens.Token(); err != io.EOF {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}

func checkpointJSONValue(decoder *json.Decoder) bool {
	token, err := decoder.Token()
	if err != nil {
		return false
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return true
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			key, err := decoder.Token()
			name, ok := key.(string)
			if err != nil || !ok || seen[name] {
				return false
			}
			seen[name] = true
			if !checkpointJSONValue(decoder) {
				return false
			}
		}
		end, err := decoder.Token()
		return err == nil && end == json.Delim('}')
	case '[':
		for decoder.More() {
			if !checkpointJSONValue(decoder) {
				return false
			}
		}
		end, err := decoder.Token()
		return err == nil && end == json.Delim(']')
	default:
		return false
	}
}
