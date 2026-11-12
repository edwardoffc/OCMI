package internal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strings"
)

// Codecs perform no filesystem writes. The sink must consume its entire reader.
type unpackEntry struct {
	Name   string
	Size   int64
	Mode   os.FileMode
	SHA256 string
}
type unpackArchiveInfo struct {
	Entries      []unpackEntry
	SourceCommit string
	Target       string
}

const unpackFileLimit int64 = 32 << 20
const unpackTotalLimit int64 = 256 << 20
const unpackWireLimit int64 = 272 << 20

var errUnpackArchive = errors.New("unsupported or invalid bounded ReadyKit archive")

func validateUnpackArchive(f *os.File, expectedSHA string) (unpackArchiveInfo, error) {
	return unpackPass(f, expectedSHA, nil, nil)
}
func streamUnpackArchive(f *os.File, expectedSHA string, expected unpackArchiveInfo, sink func(unpackEntry, io.Reader) error) error {
	if sink == nil {
		return errUnpackArchive
	}
	actual, err := unpackPass(f, expectedSHA, &expected, sink)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, expected) {
		return errUnpackArchive
	}
	return nil
}
func unpackDigest(f *os.File, expected string) (os.FileInfo, error) {
	if len(expected) != 64 || strings.Trim(expected, "0123456789abcdef") != "" {
		return nil, errUnpackArchive
	}
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() < 1 || st.Size() > unpackWireLimit {
		return nil, errUnpackArchive
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, unpackWireLimit+1))
	if err != nil || n != st.Size() || hex.EncodeToString(h.Sum(nil)) != expected {
		return nil, errUnpackArchive
	}
	return st, nil
}
func unpackPass(f *os.File, expectedSHA string, expected *unpackArchiveInfo, sink func(unpackEntry, io.Reader) error) (unpackArchiveInfo, error) {
	var empty unpackArchiveInfo
	before, err := unpackDigest(f, expectedSHA)
	if err != nil {
		return empty, err
	}
	p := unpackCollector{metadata: map[string][]byte{}, names: map[string]bool{}, expected: map[string]unpackEntry{}, sink: sink}
	if expected != nil {
		for _, e := range expected.Entries {
			if _, ok := p.expected[e.Name]; ok {
				return empty, errUnpackArchive
			}
			p.expected[e.Name] = e
		}
	}
	var sig [4]byte
	if _, err = f.ReadAt(sig[:], 0); err != nil {
		return empty, err
	}
	if bytes.Equal(sig[:], []byte{'P', 'K', 3, 4}) {
		err = unpackZIP(f, before.Size(), &p)
	} else if sig[0] == 31 && sig[1] == 139 {
		err = unpackTAR(f, before.Size(), &p)
	} else {
		err = errUnpackArchive
	}
	if err != nil {
		return empty, err
	}
	info, err := p.finish()
	if err != nil {
		return empty, err
	}
	after, err := unpackDigest(f, expectedSHA)
	if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return empty, errUnpackArchive
	}
	_, err = f.Seek(0, io.SeekStart)
	return info, err
}
func unpackName(n string) bool {
	return unpackCanonicalName(n) && (n == "SHA256SUMS" || factoryName(n))
}
func unpackCanonicalName(n string) bool {
	if n == "" || len(n) > 1024 || len(strings.Split(n, "/")) > 32 {
		return false
	}
	for _, c := range []byte(n) {
		if c < 32 || c > 126 || strings.ContainsRune("\\:*?\"<>|", rune(c)) {
			return false
		}
	}
	for _, part := range strings.Split(n, "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return false
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CLOCK$" || len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9' {
			return false
		}
	}
	return true
}

type unpackCollector struct {
	entries  []unpackEntry
	metadata map[string][]byte
	names    map[string]bool
	parents  map[string]string
	expected map[string]unpackEntry
	total    int64
	sink     func(unpackEntry, io.Reader) error
}

func (p *unpackCollector) file(n string, size int64, mode os.FileMode, r io.Reader) error {
	if !unpackName(n) || size < 0 || size > unpackFileLimit || size > unpackTotalLimit-p.total || len(p.entries) >= 4096 {
		return errUnpackArchive
	}
	key := strings.ToLower(n)
	if p.names[key] {
		return errUnpackArchive
	}
	if p.parents == nil {
		p.parents = map[string]string{}
	}
	if _, exists := p.parents[key]; exists {
		return errUnpackArchive
	}
	parts := strings.Split(n, "/")
	for i := 1; i < len(parts); i++ {
		parent := strings.Join(parts[:i], "/")
		fold := strings.ToLower(parent)
		if p.names[fold] {
			return errUnpackArchive
		}
		if old, ok := p.parents[fold]; ok && old != parent {
			return errUnpackArchive
		}
		p.parents[fold] = parent
	}
	p.names[key] = true
	meta := n == "SHA256SUMS" || n == homeMarker || n == "system/build-info.json" || n == "system/VERSION.md"
	if meta && size > 1<<20 {
		return errUnpackArchive
	}
	e := unpackEntry{Name: n, Size: size, Mode: mode}
	h := sha256.New()
	var b bytes.Buffer
	out := io.Writer(h)
	if meta {
		out = io.MultiWriter(h, &b)
	}
	limited := &io.LimitedReader{R: r, N: size}
	reader := io.TeeReader(limited, out)
	if p.sink != nil {
		want, ok := p.expected[n]
		if !ok || want.Size != size || want.Mode != mode {
			return errUnpackArchive
		}
		if err := p.sink(want, reader); err != nil {
			return err
		}
	} else {
		if _, err := io.Copy(io.Discard, reader); err != nil {
			return errUnpackArchive
		}
	}
	if limited.N != 0 {
		return errUnpackArchive
	}
	var last [1]byte
	if n, err := r.Read(last[:]); n != 0 || err != io.EOF {
		return errUnpackArchive
	}
	e.SHA256 = hex.EncodeToString(h.Sum(nil))
	if p.sink != nil && p.expected[e.Name].SHA256 != e.SHA256 {
		return errUnpackArchive
	}
	p.entries = append(p.entries, e)
	p.total += size
	if meta {
		p.metadata[n] = b.Bytes()
	}
	return nil
}
func (p *unpackCollector) finish() (unpackArchiveInfo, error) {
	bad := unpackArchiveInfo{}
	sums := p.metadata["SHA256SUMS"]
	if len(sums) == 0 {
		return bad, errUnpackArchive
	}
	wanted := map[string]string{}
	fold := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(string(sums), "\n"), "\n") {
		line = strings.TrimSuffix(line, "\r")
		a := strings.SplitN(line, "  ", 2)
		if len(a) != 2 || len(a[0]) != 64 || strings.Trim(a[0], "0123456789abcdef") != "" || !unpackName(a[1]) || a[1] == "SHA256SUMS" || fold[strings.ToLower(a[1])] {
			return bad, errUnpackArchive
		}
		wanted[a[1]] = a[0]
		fold[strings.ToLower(a[1])] = true
	}
	if len(wanted)+1 != len(p.entries) {
		return bad, errUnpackArchive
	}
	for _, e := range p.entries {
		if e.Name != "SHA256SUMS" && wanted[e.Name] != e.SHA256 {
			return bad, errUnpackArchive
		}
	}
	for _, n := range []string{homeMarker, "README.md", "README.ru.md", "AGENTS.md", ".gitignore", ".gitattributes", "system/build-info.json", "system/VERSION.md"} {
		if wanted[n] == "" {
			return bad, errUnpackArchive
		}
	}
	declaration, err := strictDeclaration(p.metadata[homeMarker])
	canonical, _ := strictDeclaration(installationLayoutJSON)
	if err != nil || !sameLayout(declaration, canonical) {
		return bad, errUnpackArchive
	}
	var info map[string]any
	if !checkpointStrictJSON(p.metadata["system/build-info.json"], &info) {
		return bad, errUnpackArchive
	}
	source, _ := info["sourceCommit"].(string)
	target, _ := info["target"].(string)
	binary, _ := info["binaryPath"].(string)
	version, _ := info["systemVersion"].(string)
	exe := "system/bin/ocmi"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if info["distributionKind"] != "ready-kit" || info["layoutVersion"] != float64(1) || info["layoutDeclaration"] != homeMarker || info["sourceDirty"] != false || len(source) != 40 || strings.Trim(source, "0123456789abcdef") != "" || target != runtime.GOOS+"-"+runtime.GOARCH || binary != exe || wanted[exe] == "" || version == "" || !factoryVersion(string(p.metadata["system/VERSION.md"]), version) {
		return bad, errUnpackArchive
	}
	for _, e := range p.entries {
		mode := os.FileMode(0644)
		if e.Name == exe {
			mode = 0755
		}
		if e.Mode != mode {
			return bad, errUnpackArchive
		}
	}
	sort.Slice(p.entries, func(i, j int) bool { return p.entries[i].Name < p.entries[j].Name })
	return unpackArchiveInfo{Entries: p.entries, SourceCommit: source, Target: target}, nil
}
