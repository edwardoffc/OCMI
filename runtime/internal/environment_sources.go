package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

const environmentSourceLimit = 256 << 10
const environmentExecutableLimit = 256 << 20

type environmentFile struct {
	Kind, Path    string
	Raw           []byte
	SHA, NativeID string
}
type environmentReadSet struct {
	Files           []environmentFile
	project, rootID string
	sizes           []int64
}

func environmentReadSources(project string, selected []environmentSource) (*environmentReadSet, error) {
	if len(selected) == 0 || len(selected) > 32 || environmentPlainAncestors(project) != nil {
		return nil, errors.New("environment sources")
	}
	project = filepath.Clean(project)
	rootID, err := localNativeID(project)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(project)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	set := &environmentReadSet{project: project, rootID: rootID}
	seen := map[string]bool{}
	total := 0
	for _, source := range selected {
		if !environmentSourceKind(source.Kind) || !environmentSourcePath(source.Path) || seen[source.Path] {
			return nil, errors.New("environment source")
		}
		seen[source.Path] = true
		file, size, err := environmentReadSource(root, project, source)
		if err != nil || total+len(file.Raw) > 2<<20 {
			return nil, errors.New("environment source")
		}
		total += len(file.Raw)
		set.Files = append(set.Files, file)
		set.sizes = append(set.sizes, size)
	}
	return set, nil
}

func (s *environmentReadSet) Recheck() error {
	if s == nil || environmentPlainAncestors(s.project) != nil {
		return errors.New("environment root")
	}
	id, err := localNativeID(s.project)
	if err != nil || id != s.rootID {
		return errors.New("environment root drift")
	}
	root, err := os.OpenRoot(s.project)
	if err != nil {
		return errors.New("environment root drift")
	}
	defer root.Close()
	for i, expected := range s.Files {
		got, size, err := environmentReadSource(root, s.project, environmentSource{Kind: expected.Kind, Path: expected.Path})
		if err != nil || size != s.sizes[i] || got.NativeID != expected.NativeID || got.SHA != expected.SHA {
			return errors.New("environment source drift")
		}
	}
	return nil
}

func (s *environmentReadSet) Close() error {
	return nil
}

func environmentReadSource(root *os.Root, project string, source environmentSource) (environmentFile, int64, error) {
	name := filepath.FromSlash(source.Path)
	if environmentRootPathPlain(root, project, name) != nil {
		return environmentFile{}, 0, errors.New("environment source link")
	}
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > environmentSourceLimit {
		return environmentFile{}, 0, errors.New("environment source size")
	}
	f, err := root.Open(name)
	if err != nil {
		return environmentFile{}, 0, err
	}
	defer f.Close()
	id, err := localNativeIDFile(f)
	if err != nil {
		return environmentFile{}, 0, err
	}
	raw, err := io.ReadAll(io.LimitReader(f, environmentSourceLimit+1))
	if err != nil || len(raw) > environmentSourceLimit || !utf8.Valid(raw) || environmentControl(raw) {
		return environmentFile{}, 0, errors.New("environment source content")
	}
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Size() != info.Size() || opened.Size() != int64(len(raw)) {
		return environmentFile{}, 0, errors.New("environment source drift")
	}
	h := sha256.Sum256(raw)
	return environmentFile{Kind: source.Kind, Path: source.Path, Raw: raw, SHA: hex.EncodeToString(h[:]), NativeID: id}, opened.Size(), nil
}

func environmentExecutable(project, path string) (resolved, nativeID, sha string, err error) {
	if !filepath.IsAbs(path) || environmentWithin(project, path) || environmentSelectedExecutablePath(path) != nil {
		return "", "", "", errors.New("environment executable")
	}
	resolved, err = environmentCanonicalExecutable(path)
	if err != nil {
		return "", "", "", err
	}
	resolved = filepath.Clean(resolved)
	if environmentWithin(project, resolved) || environmentPrivateExecutablePath(resolved) || environmentPlainAncestors(resolved) != nil {
		return "", "", "", errors.New("environment executable")
	}
	pre, err := os.Lstat(resolved)
	if err != nil || !pre.Mode().IsRegular() || pre.Size() < 0 || pre.Size() > environmentExecutableLimit {
		return "", "", "", errors.New("environment executable")
	}
	f, err := os.Open(resolved)
	if err != nil {
		return "", "", "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != pre.Size() {
		return "", "", "", errors.New("environment executable")
	}
	nativeID, err = localNativeIDFile(f)
	if err != nil {
		return "", "", "", err
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, environmentExecutableLimit+1))
	if err != nil || n != info.Size() || n > environmentExecutableLimit {
		return "", "", "", errors.New("environment executable")
	}
	final, err := f.Stat()
	if err != nil || final.Size() != info.Size() {
		return "", "", "", errors.New("environment executable drift")
	}
	finalID, err := localNativeIDFile(f)
	if err != nil || finalID != nativeID {
		return "", "", "", errors.New("environment executable drift")
	}
	return resolved, nativeID, hex.EncodeToString(h.Sum(nil)), nil
}

func environmentExecutableUnchanged(project, resolved, nativeID, sha string) bool {
	got, id, digest, err := environmentExecutable(project, resolved)
	return err == nil && got == resolved && id == nativeID && digest == sha
}

func environmentScratchAllowed(project, scratch string) error {
	if !filepath.IsAbs(scratch) || environmentPlainAncestors(project) != nil || environmentPlainAncestors(scratch) != nil || environmentWithin(project, scratch) || environmentWithin(scratch, project) {
		return errors.New("environment scratch")
	}
	info, err := os.Stat(scratch)
	if err != nil || !info.IsDir() {
		return errors.New("environment scratch")
	}
	return nil
}

func environmentSourceKind(kind string) bool {
	return kind == "go-mod" || kind == "node-package" || kind == "python-project" || kind == "guide"
}
func environmentSourcePath(p string) bool {
	if p == "" || filepath.ToSlash(filepath.Clean(p)) != p || !portableName(p) || strings.ContainsAny(p, "\\\x00\r\n") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." || environmentPrivateName(part) {
			return false
		}
	}
	return true
}
func environmentPrivateName(name string) bool {
	lower := strings.ToLower(name)
	if lower == ".git" || lower == ".ocmi" || lower == ".local" || lower == ".codex" || lower == "node_modules" || strings.HasPrefix(lower, ".env") {
		return true
	}
	return checkpointGitSensitiveName(name)
}
func environmentPrivateExecutablePath(path string) bool {
	parts := strings.Split(filepath.Clean(path), string(os.PathSeparator))
	if len(parts) == 0 || checkpointGitSensitiveName(parts[len(parts)-1]) || strings.HasPrefix(strings.ToLower(parts[len(parts)-1]), ".env") {
		return true
	}
	for i, part := range parts {
		lower := strings.ToLower(part)
		if lower == ".ocmi" || lower == ".codex" {
			return true
		}
		if lower == ".local" && i+1 < len(parts) {
			next := strings.ToLower(parts[i+1])
			if next == "coordinator" || next == "sync" || next == "device" {
				return true
			}
		}
	}
	return false
}
func environmentControl(raw []byte) bool {
	for _, b := range raw {
		if b < 0x09 || (b > 0x0d && b < 0x20) || b == 0x7f {
			return true
		}
	}
	return false
}
func environmentRootPathPlain(root *os.Root, project, name string) error {
	cur := ""
	for _, part := range strings.Split(name, string(os.PathSeparator)) {
		cur = filepath.Join(cur, part)
		info, err := root.Lstat(cur)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 || checkpointGitReparse(filepath.Join(project, cur)) {
			return errors.New("environment link")
		}
	}
	return nil
}
func environmentPlainAncestors(path string) error {
	path = filepath.Clean(path)
	for cur := path; ; cur = filepath.Dir(cur) {
		info, err := os.Lstat(cur)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 || checkpointGitReparse(cur) {
			return errors.New("environment link")
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return nil
		}
	}
}
func environmentWithin(parent, child string) bool {
	p, c := filepath.Clean(parent), filepath.Clean(child)
	if runtime.GOOS == "windows" {
		p, c = strings.ToLower(p), strings.ToLower(c)
	}
	rel, err := filepath.Rel(p, c)
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))))
}
