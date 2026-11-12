package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Internal Go-only fault/interruption seam. It has no environment or CLI setter.
var unpackTestPhase func(phase, stage string) error

type unpackOwnership struct {
	SchemaVersion int    `json:"schemaVersion"`
	Token         string `json:"token"`
	Destination   string `json:"destination"`
	ParentID      string `json:"parentId"`
	StageID       string `json:"stageId"`
	PayloadID     string `json:"payloadId"`
	ArchiveID     string `json:"archiveId"`
	ArchiveSHA256 string `json:"archiveSHA256"`
	SourceCommit  string `json:"sourceCommit"`
	Phase         string `json:"phase"`
}
type unpackOwnedEntry struct {
	id, digest string
	size       int64
	mode       os.FileMode
	directory  bool
}
type unpackStage struct {
	parent, root, payload                          *os.Root
	parentPath, name, parentID, stageID, payloadID string
	owned                                          map[string]unpackOwnedEntry
	uncertain, published                           bool
}

func unpackFailure(reason, stage, cleanup string, u *unpackStage) Response {
	data := map[string]any{"reason": reason, "stage": stage, "cleanup": cleanup, "nextAction": "verify input and selected absent destination"}
	if cleanup == "pending" {
		data["nextAction"] = "preserve recorded stage; inspect destination; do not retry or delete unknown data"
		if u != nil {
			data["ownedStage"] = filepath.Join(u.parentPath, u.name)
			data["record"] = "unpack-owner.json"
		}
	}
	return installationResponse("UNPACK_REFUSED", false, data)
}

func unpackAbsolute(name string) bool { return filepath.IsAbs(name) && filepath.Clean(name) == name }
func unpackPublicationNames(source, destination string) bool {
	if !filepath.IsLocal(source) || filepath.Clean(source) != source || source == "." || destination == "" || destination == "." || destination == ".." || filepath.Base(destination) != destination || !utf8.ValidString(destination) || strings.ContainsAny(destination, "\\/:<>\"|?*\x00") || strings.HasSuffix(destination, ".") || strings.HasSuffix(destination, " ") {
		return false
	}
	for _, c := range destination {
		if c < 32 || c == 127 {
			return false
		}
	}
	base, _, _ := strings.Cut(strings.ToUpper(destination), ".")
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CLOCK$" || base == "CONIN$" || base == "CONOUT$" {
		return false
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9' {
		return false
	}
	return true
}
func unpackRootID(root *os.Root) (string, error) {
	f, e := root.Open(".")
	if e != nil {
		return "", e
	}
	defer f.Close()
	return localNativeIDFile(f)
}
func unpackPhase(phase string, u *unpackStage) error {
	if unpackTestPhase != nil {
		return unpackTestPhase(phase, filepath.Join(u.parentPath, u.name))
	}
	return nil
}

// UnpackHome never initializes local state or executes any delivered payload.
func UnpackHome(archive, destination, expectedSHA string, inspect bool) (result Response) {
	if !unpackAbsolute(destination) || filepath.Dir(destination) == destination {
		return unpackFailure("input", "admission", "done", nil)
	}
	if inspect {
		if archive != "" || expectedSHA != "" || environmentPlainAncestors(destination) != nil || plainPath(destination, true) != nil {
			return unpackFailure("inspection", "inspect", "done", nil)
		}
		id, e := localNativeID(destination)
		if e != nil {
			return unpackFailure("inspection", "inspect", "done", nil)
		}
		_, info, e := verifyFactoryHome(destination)
		now, idErr := localNativeID(destination)
		if e != nil || idErr != nil || id != now {
			return unpackFailure("inspection", "inspect", "done", nil)
		}
		return installationResponse("UNPACK_INSPECTED", true, map[string]any{"phase": "ready", "sourceCommit": info.SourceCommit, "cleanup": "done"})
	}
	if !unpackAbsolute(archive) || len(expectedSHA) != 64 || strings.Trim(expectedSHA, "0123456789abcdef") != "" {
		return unpackFailure("input", "admission", "done", nil)
	}
	parentPath, leaf := filepath.Dir(destination), filepath.Base(destination)
	if !unpackPublicationNames("stage", leaf) || environmentPlainAncestors(parentPath) != nil || plainPath(parentPath, true) != nil {
		return unpackFailure("parent", "admission", "done", nil)
	}
	parent, e := os.OpenRoot(parentPath)
	if e != nil {
		return unpackFailure("parent", "admission", "done", nil)
	}
	defer parent.Close()
	parentID, e := unpackRootID(parent)
	if e != nil {
		return unpackFailure("parent", "admission", "done", nil)
	}
	if _, e = parent.Lstat(leaf); !os.IsNotExist(e) {
		return unpackFailure("destination_exists", "admission", "done", nil)
	}
	if environmentPlainAncestors(archive) != nil || plainPath(archive, false) != nil {
		return unpackFailure("archive", "validation", "done", nil)
	}
	f, e := os.Open(archive)
	if e != nil {
		return unpackFailure("archive", "validation", "done", nil)
	}
	defer f.Close()
	archiveInfo, e := f.Stat()
	if e != nil || !archiveInfo.Mode().IsRegular() {
		return unpackFailure("archive", "validation", "done", nil)
	}
	archiveID, e := localNativeIDFile(f)
	if e != nil {
		return unpackFailure("archive", "validation", "done", nil)
	}
	if current, e := localNativeID(archive); e != nil || current != archiveID {
		return unpackFailure("archive_changed", "validation", "done", nil)
	}
	info, e := validateUnpackArchive(f, expectedSHA)
	if e != nil || info.Target != runtime.GOOS+"-"+runtime.GOARCH || !validCommit(info.SourceCommit) {
		return unpackFailure("archive", "validation", "done", nil)
	}
	if e = unpackArchiveUnchanged(f, archive, archiveID, archiveInfo); e != nil {
		return unpackFailure("archive_changed", "validation", "done", nil)
	}
	if environmentPlainAncestors(parentPath) != nil {
		return unpackFailure("parent_changed", "admission", "done", nil)
	}
	if current, e := localNativeID(parentPath); e != nil || current != parentID {
		return unpackFailure("parent_changed", "admission", "done", nil)
	}
	if _, e = parent.Lstat(leaf); !os.IsNotExist(e) {
		return unpackFailure("destination_exists", "admission", "done", nil)
	}
	u := &unpackStage{parent: parent, parentPath: parentPath, name: ".ocmi-unpack-" + uuid.NewString(), parentID: parentID, owned: map[string]unpackOwnedEntry{}}
	if e = parent.Mkdir(u.name, 0700); e != nil {
		return unpackFailure("stage", "staging", "done", nil)
	}
	// From this point, every exit either verifies and closes this exact owned
	// stage or explicitly reports the attributable retained record.
	defer func() {
		if u.payload != nil {
			u.payload.Close()
		}
		if u.root != nil {
			u.root.Close()
		}
		if e := u.cleanup(); e != nil {
			pending := unpackFailure("cleanup_pending", "cleanup", "pending", u)
			if !result.OK && result.Data != nil {
				pending.Data["reason"] = result.Data["reason"]
				pending.Data["stage"] = result.Data["stage"]
			}
			result = pending
		}
	}()
	u.root, e = parent.OpenRoot(u.name)
	if e != nil {
		u.uncertain = true
		return unpackFailure("stage", "staging", "pending", u)
	}
	u.stageID, e = unpackRootID(u.root)
	if e != nil {
		u.uncertain = true
		return unpackFailure("stage", "staging", "pending", u)
	}
	if e = u.root.Mkdir("home", 0700); e != nil {
		u.uncertain = true
		return unpackFailure("stage", "staging", "pending", u)
	}
	u.payload, e = u.root.OpenRoot("home")
	if e != nil {
		u.uncertain = true
		return unpackFailure("stage", "staging", "pending", u)
	}
	u.payloadID, e = unpackRootID(u.payload)
	if e != nil {
		u.uncertain = true
		return unpackFailure("stage", "staging", "pending", u)
	}
	u.owned["home"] = unpackOwnedEntry{id: u.payloadID, directory: true}
	record := unpackOwnership{1, strings.TrimPrefix(u.name, ".ocmi-unpack-"), leaf, parentID, u.stageID, u.payloadID, archiveID, expectedSHA, info.SourceCommit, "prepared"}
	raw, _ := json.Marshal(record)
	if e = u.writeFile("unpack-owner.json", raw); e != nil {
		u.uncertain = true
		return unpackFailure("record", "staging", "pending", u)
	}
	if e = unpackPhase("stage_created", u); e != nil {
		return unpackFailure("interrupted", "staging", "done", u)
	}
	written := map[string]bool{}
	expectedEntries := map[string]unpackEntry{}
	for _, entry := range info.Entries {
		expectedEntries[entry.Name] = entry
	}
	e = streamUnpackArchive(f, expectedSHA, info, func(entry unpackEntry, reader io.Reader) error {
		if written[entry.Name] || expectedEntries[entry.Name] != entry {
			return errors.New("stream inventory changed")
		}
		written[entry.Name] = true
		return u.writePayload(entry, reader)
	})
	if e != nil || len(written) != len(info.Entries) {
		return unpackFailure("archive_stream", "write", "done", u)
	}
	if e = unpackArchiveUnchanged(f, archive, archiveID, archiveInfo); e != nil {
		return unpackFailure("archive_changed", "verification", "done", u)
	}
	if e = unpackPhase("payload_written", u); e != nil {
		return unpackFailure("interrupted", "verification", "done", u)
	}
	payloadPath := filepath.Join(parentPath, u.name, "home")
	if e = u.identities(); e != nil {
		u.uncertain = true
		return unpackFailure("stage_changed", "verification", "pending", u)
	}
	_, factory, e := verifyFactoryHome(payloadPath)
	if e != nil || factory.SourceCommit != info.SourceCommit {
		return unpackFailure("factory", "verification", "done", u)
	}
	if e = unpackPhase("before_publish", u); e != nil {
		return unpackFailure("interrupted", "publication", "done", u)
	}
	if e = u.identities(); e != nil {
		u.uncertain = true
		return unpackFailure("stage_changed", "publication", "pending", u)
	}
	// Recheck the exact bytes/owned nodes after the final fault/race seam.
	if e = u.verifyOwned(); e != nil {
		u.uncertain = true
		return unpackFailure("stage_changed", "publication", "pending", u)
	}
	if e = unpackPublish(parent, filepath.Join(u.name, "home"), leaf, parentID, u.payloadID); e != nil {
		return unpackFailure("publication", "publication", "done", u)
	}
	u.published = true
	for name := range u.owned {
		if name == "home" || strings.HasPrefix(name, "home/") {
			delete(u.owned, name)
		}
	}
	if e = unpackPhase("published", u); e != nil {
		u.uncertain = true
		return unpackFailure("publication_unknown", "publication", "pending", u)
	}
	if now, e := localNativeID(destination); e != nil || now != u.payloadID {
		u.uncertain = true
		return unpackFailure("publication_unknown", "publication", "pending", u)
	}
	return installationResponse("UNPACKED", true, map[string]any{"phase": "ready", "sourceCommit": info.SourceCommit, "cleanup": "done"})
}

func unpackArchiveUnchanged(f *os.File, name, id string, before os.FileInfo) error {
	after, e := f.Stat()
	if e != nil || !stableInventoryInfo(before, after) {
		return errors.New("archive changed")
	}
	now, e := localNativeIDFile(f)
	if e != nil || now != id {
		return errors.New("archive identity")
	}
	if plainPath(name, false) != nil {
		return errors.New("archive path")
	}
	now, e = localNativeID(name)
	if e != nil || now != id {
		return errors.New("archive placement")
	}
	return nil
}

func (u *unpackStage) identities() error {
	if u.uncertain || u.stageID == "" || environmentPlainAncestors(u.parentPath) != nil {
		return errors.New("unqualified stage")
	}
	id, e := localNativeID(u.parentPath)
	if e != nil || id != u.parentID {
		return errors.New("parent changed")
	}
	for name, want := range map[string]string{u.name: u.stageID} {
		info, e := u.parent.Lstat(name)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || checkpointGitReparse(filepath.Join(u.parentPath, name)) {
			return errors.New("stage changed")
		}
		f, e := u.parent.Open(name)
		if e != nil {
			return e
		}
		id, e := localNativeIDFile(f)
		f.Close()
		if e != nil || id != want {
			return errors.New("stage identity")
		}
	}
	return nil
}

func (u *unpackStage) writeFile(name string, raw []byte) error {
	f, e := u.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	if _, e = f.Write(raw); e == nil {
		e = f.Sync()
	}
	id, idErr := localNativeIDFile(f)
	stat, statErr := f.Stat()
	closeErr := f.Close()
	if e != nil || idErr != nil || statErr != nil || closeErr != nil {
		return errors.New("record write")
	}
	h := sha256.Sum256(raw)
	u.owned[name] = unpackOwnedEntry{id: id, digest: hex.EncodeToString(h[:]), size: int64(len(raw)), mode: stat.Mode().Perm()}
	return nil
}

func (u *unpackStage) writePayload(entry unpackEntry, reader io.Reader) error {
	name := entry.Name
	if len(name) > 1024 || name == "." || (name != "SHA256SUMS" && !factoryName(name)) || path.Clean(name) != name || !filepath.IsLocal(name) || strings.ContainsAny(name, "\\:\x00") || strings.Count(name, "/") >= 32 || entry.Size < 0 || entry.Size > 32<<20 {
		return errors.New("invalid writer entry")
	}
	parts := strings.Split(name, "/")
	prefix := "home"
	for _, part := range parts[:len(parts)-1] {
		prefix += "/" + part
		if known, ok := u.owned[prefix]; ok {
			if !known.directory {
				return errors.New("file prefix")
			}
			f, e := u.root.Open(filepath.FromSlash(prefix))
			if e != nil {
				return e
			}
			id, e := localNativeIDFile(f)
			f.Close()
			if e != nil || id != known.id {
				return errors.New("directory changed")
			}
			continue
		}
		if len(u.owned) > 135172 {
			return errors.New("stage metadata bound")
		}
		if e := u.root.Mkdir(filepath.FromSlash(prefix), 0700); e != nil {
			return e
		}
		f, e := u.root.Open(filepath.FromSlash(prefix))
		if e != nil {
			u.uncertain = true
			return e
		}
		id, e := localNativeIDFile(f)
		f.Close()
		if e != nil {
			u.uncertain = true
			return e
		}
		u.owned[prefix] = unpackOwnedEntry{id: id, directory: true}
	}
	full := "home/" + name
	f, e := u.root.OpenFile(filepath.FromSlash(full), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	h := sha256.New()
	n, writeErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(reader, entry.Size+1))
	mode := os.FileMode(0644)
	if name == "system/bin/ocmi" || name == "system/bin/ocmi.exe" {
		mode = 0755
	}
	modeErr := f.Chmod(mode)
	id, idErr := localNativeIDFile(f)
	stat, statErr := f.Stat()
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil || modeErr != nil || idErr != nil || statErr != nil || syncErr != nil || closeErr != nil {
		u.uncertain = true
		return errors.New("stage write uncertain")
	}
	digest := hex.EncodeToString(h.Sum(nil))
	u.owned[full] = unpackOwnedEntry{id: id, digest: digest, size: n, mode: stat.Mode().Perm()}
	if n != entry.Size || digest != entry.SHA256 {
		return errors.New("writer content mismatch")
	}
	return nil
}

func (u *unpackStage) verifyOwned() error {
	if e := u.identities(); e != nil {
		return e
	}
	seen := 0
	var walk func(string) error
	walk = func(name string) error {
		f, e := u.root.Open(filepath.FromSlash(name))
		if e != nil {
			return e
		}
		defer f.Close()
		for {
			entries, readErr := f.ReadDir(32)
			if readErr != nil && readErr != io.EOF {
				return readErr
			}
			for _, entry := range entries {
				rel := entry.Name()
				if name != "." {
					rel = name + "/" + rel
				}
				seen++
				if seen > len(u.owned) {
					return errors.New("foreign stage member")
				}
				want, ok := u.owned[rel]
				if !ok {
					return errors.New("unknown stage member")
				}
				info, e := u.root.Lstat(filepath.FromSlash(rel))
				if e != nil || info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 || info.IsDir() != want.directory {
					return errors.New("stage member changed")
				}
				child, e := u.root.Open(filepath.FromSlash(rel))
				if e != nil {
					return e
				}
				id, idErr := localNativeIDFile(child)
				if idErr != nil || id != want.id {
					child.Close()
					return errors.New("stage member identity")
				}
				if !want.directory {
					h := sha256.New()
					n, err := io.Copy(h, io.LimitReader(child, want.size+1))
					child.Close()
					if err != nil || n != want.size || hex.EncodeToString(h.Sum(nil)) != want.digest || info.Mode().Perm() != want.mode {
						return errors.New("stage content changed")
					}
				} else {
					child.Close()
					if e = walk(rel); e != nil {
						return e
					}
				}
			}
			if readErr == io.EOF {
				break
			}
		}
		return nil
	}
	if e := walk("."); e != nil {
		return e
	}
	if seen != len(u.owned) {
		return errors.New("stage member missing")
	}
	return nil
}

func (u *unpackStage) cleanup() error {
	if u.root == nil || u.uncertain {
		return errors.New("stage pending")
	}
	// Reopen only after original roots/handles have been closed by the caller.
	r, e := u.parent.OpenRoot(u.name)
	if e != nil {
		return e
	}
	u.root = r
	defer r.Close()
	if e = u.verifyOwned(); e != nil {
		return e
	}
	names := make([]string, 0, len(u.owned))
	for name := range u.owned {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if strings.Count(names[i], "/") != strings.Count(names[j], "/") {
			return strings.Count(names[i], "/") > strings.Count(names[j], "/")
		}
		return names[i] > names[j]
	})
	for _, name := range names {
		if e = r.Remove(filepath.FromSlash(name)); e != nil {
			return e
		}
	}
	r.Close()
	if e = u.identities(); e != nil {
		return e
	}
	return u.parent.Remove(u.name)
}
