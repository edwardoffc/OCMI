package internal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type updatePublicEntry struct {
	Path     string `json:"path"`
	Digest   string `json:"digest"`
	Size     int64  `json:"size"`
	Mode     uint32 `json:"mode"`
	NativeID string `json:"nativeId"`
}
type updatePublicProof struct {
	SchemaVersion int                 `json:"schemaVersion"`
	RootID        string              `json:"rootId"`
	SystemID      string              `json:"systemId"`
	Files         []updatePublicEntry `json:"files"`
}

// captureUpdatePublicProof starts from a verified factory inventory. Private
// roots are opaque only for an installed home; factories must not contain them.
func captureUpdatePublicProof(root string, allowPrivate bool) (updatePublicProof, error) {
	if _, _, err := verifyFactoryHome(root); err != nil {
		return updatePublicProof{}, err
	}
	return scanUpdatePublic(root, allowPrivate)
}

func scanUpdatePublic(root string, allowPrivate bool) (updatePublicProof, error) {
	var p updatePublicProof
	if !filepath.IsAbs(root) || environmentPlainAncestors(root) != nil {
		return p, errors.New("unsafe public root")
	}
	var err error
	p.SchemaVersion = 1
	p.RootID, err = localNativeID(root)
	if err != nil {
		return p, err
	}
	p.SystemID, err = localNativeID(filepath.Join(root, "system"))
	if err != nil {
		return p, err
	}
	dirs := map[string]bool{}
	used := map[string]bool{}
	var total int64
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "local" || rel == "OCMI_Sync" {
			if allowPrivate && entry.IsDir() && plainPath(path, true) == nil {
				return filepath.SkipDir
			}
			return errors.New("private factory content")
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.Type()&os.ModeIrregular != 0 || plainComponents(root, filepath.FromSlash(rel), false) != nil {
			return errors.New("unsafe public member")
		}
		if entry.IsDir() {
			dirs[rel] = true
			return nil
		}
		if !entry.Type().IsRegular() || !factoryName(rel) && rel != "SHA256SUMS" {
			return errors.New("invalid public member")
		}
		raw, e := installationRead(root, rel, 32<<20)
		if e != nil {
			return updateObserved("public content", e, "read")
		}
		total += int64(len(raw))
		if total > 512<<20 {
			return errors.New("public payload limit")
		}
		info, e := os.Lstat(path)
		if e != nil {
			return updateObserved("public mode", e, "stat")
		}
		id, e := localNativeID(path)
		if e != nil {
			return updateObserved("public identity", e, "native_id")
		}
		p.Files = append(p.Files, updatePublicEntry{Path: rel, Digest: updateDigest(raw), Size: int64(len(raw)), Mode: uint32(info.Mode().Perm()), NativeID: id})
		if len(p.Files) > 4096 {
			return errors.New("public inventory limit")
		}
		parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(rel)))
		for parent != "." {
			used[parent] = true
			parent = filepath.ToSlash(filepath.Dir(filepath.FromSlash(parent)))
		}
		return nil
	})
	if err != nil {
		return updatePublicProof{}, err
	}
	for dir := range dirs {
		if !used[dir] {
			return updatePublicProof{}, errors.New("unknown empty directory")
		}
	}
	sort.Slice(p.Files, func(i, j int) bool { return p.Files[i].Path < p.Files[j].Path })
	return p, nil
}

func decodeUpdatePublicProof(raw []byte) (updatePublicProof, error) {
	var p updatePublicProof
	if len(raw) > 2<<20 || !checkpointStrictJSON(raw, &p) || p.SchemaVersion != 1 || p.RootID == "" || p.SystemID == "" || len(p.Files) == 0 || len(p.Files) > 4096 {
		return updatePublicProof{}, errors.New("invalid public proof")
	}
	last := ""
	for _, v := range p.Files {
		if (v.Path != "SHA256SUMS" && !factoryName(v.Path)) || v.Path <= last || len(v.Digest) != 64 || strings.Trim(v.Digest, "0123456789abcdef") != "" || v.Size < 0 || v.Size > 32<<20 || v.Mode > 0777 || v.NativeID == "" {
			return updatePublicProof{}, errors.New("invalid public entry")
		}
		last = v.Path
	}
	return p, nil
}
func encodeUpdatePublicProof(p updatePublicProof) ([]byte, error) {
	raw, e := json.Marshal(p)
	if e != nil {
		return nil, e
	}
	if _, e = decodeUpdatePublicProof(raw); e != nil {
		return nil, e
	}
	return raw, nil
}

func verifyUpdatePublicProof(root string, p updatePublicProof, allowPrivate, exactNative bool) error {
	if _, err := encodeUpdatePublicProof(p); err != nil {
		return err
	}
	actual, err := scanUpdatePublic(root, allowPrivate)
	if err != nil {
		return err
	}
	if exactNative && (actual.RootID != p.RootID || actual.SystemID != p.SystemID) {
		return updateMismatch("public identity")
	}
	if len(actual.Files) != len(p.Files) {
		return updateMismatch("public inventory")
	}
	for i, v := range actual.Files {
		want := p.Files[i]
		if v.Path != want.Path || v.Digest != want.Digest || v.Size != want.Size || v.Mode != want.Mode || exactNative && v.NativeID != want.NativeID {
			if v.Path != want.Path {
				return updateMismatch("public inventory")
			}
			if v.Digest != want.Digest || v.Size != want.Size {
				return updateMismatch("public content")
			}
			if v.Mode != want.Mode {
				return updateMismatch("public mode")
			}
			return updateMismatch("public identity")
		}
	}
	return nil
}
