package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

type localInventoryPolicy struct {
	nodes                          int
	fileBytes, totalBytes          int64
	encodedBytes, pathBytes, depth int
}

var ordinaryInventoryPolicy = localInventoryPolicy{256, 16 << 20, 16 << 20, 16 << 20, 0, 0}
var automaticInventoryPolicy = localInventoryPolicy{1024, 64 << 20, 128 << 20, 1 << 20, 1024, 32}

// Correlated with the original ledger row, including its pre-effect event.
// A reserved-looking locator or the internal path flag alone is not authority.
const automaticWorkspaceProjection = `(provider='local' AND kind='directory' AND mode='create' AND ownership='project' AND owner_ref='ocmi-runtime' AND imported=0 AND transfer_id='' AND id='ocmi-workspace-'||operation_id AND EXISTS(SELECT 1 FROM operation o JOIN operation_revision v ON v.operation_id=o.id AND v.task_id=o.task_id JOIN operation_owner ow ON ow.operation_id=o.id JOIN project p ON p.id=resource_ledger.project_id AND p.copy_id=resource_ledger.copy_id AND p.device_id=resource_ledger.device_id WHERE ow.copy_id=p.copy_id AND ow.device_id=p.device_id AND o.id=resource_ledger.operation_id AND o.task_id=resource_ledger.task_id AND v.task_revision=resource_ledger.task_revision AND EXISTS(SELECT 1 FROM resource_event e WHERE e.resource_id=resource_ledger.id AND e.event='automatic_intent' AND e.proof_class='pre_effect' AND e.request_id=ow.origin_request_id AND e.task_id=resource_ledger.task_id AND e.task_revision=resource_ledger.task_revision)))`

func captureAutomaticWorkspace(root, locator, native string) (localResourceProof, error) {
	if native == "" {
		return localResourceProof{}, errors.New("missing workspace identity")
	}
	return captureLocalResourcePolicy(root, locator, "directory", native, true, automaticInventoryPolicy)
}
func removeAutomaticWorkspace(root, locator, native, inventory string) error {
	if native == "" {
		return errors.New("missing workspace identity")
	}
	return removeLocalResourcePolicy(root, locator, "directory", native, inventory, true, automaticInventoryPolicy)
}

type localInventoryWalk struct {
	policy         localInventoryPolicy
	nodes, encoded int
	bytes          int64
	entries        []string
}

func (w *localInventoryWalk) append(kind, path, id, tail string) error {
	// Check the base64 expansion before allocating the encoded item.
	size := len(kind) + 1 + (len(filepath.ToSlash(path))*8+5)/6 + 1 + len(id) + 1 + len(tail)
	if len(w.entries) > 0 {
		size++
	}
	if size > w.policy.encodedBytes-w.encoded {
		return errors.New("inventory encoding overflow")
	}
	w.entries = append(w.entries, inventoryItem(kind, path, id, tail))
	w.encoded += size
	return nil
}

func stableInventoryInfo(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func (w *localInventoryWalk) walk(root *os.Root, name, rel string, depth int, expectedID string) error {
	dir, err := root.Open(name)
	if err != nil {
		return err
	}
	defer dir.Close()
	reopenedID, err := localNativeIDFile(dir)
	if err != nil || expectedID == "" || reopenedID != expectedID {
		return errors.New("inventory directory identity changed")
	}
	before, err := dir.Stat()
	if err != nil || !before.IsDir() {
		return errors.New("inventory directory changed")
	}
	for {
		limit := w.policy.nodes - w.nodes + 1
		if limit > 32 {
			limit = 32
		}
		list, readErr := dir.ReadDir(limit)
		w.nodes += len(list)
		if w.nodes > w.policy.nodes {
			return errors.New("inventory overflow")
		}
		if readErr != nil && readErr != io.EOF {
			return readErr
		}
		for _, entry := range list {
			child := filepath.Join(name, entry.Name())
			next := filepath.Join(rel, entry.Name())
			if w.policy.pathBytes > 0 && (len(filepath.ToSlash(next)) > w.policy.pathBytes || depth+1 > w.policy.depth || !utf8.ValidString(next)) {
				return errors.New("inventory path overflow")
			}
			info, e := root.Lstat(child)
			if e != nil {
				return e
			}
			if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
				return errors.New("unverified entry")
			}
			opened, e := root.Open(child)
			if e != nil {
				return e
			}
			id, e := localNativeIDFile(opened)
			if e != nil {
				opened.Close()
				return e
			}
			openedInfo, e := opened.Stat()
			if e != nil || !stableInventoryInfo(info, openedInfo) {
				opened.Close()
				return errors.New("inventory identity changed")
			}
			if info.IsDir() {
				opened.Close()
				if e = w.append("d", next, id, "0"); e != nil {
					return e
				}
				if e = w.walk(root, child, next, depth+1, id); e != nil {
					return e
				}
			} else {
				limit := w.policy.totalBytes - w.bytes
				if limit > w.policy.fileBytes {
					limit = w.policy.fileBytes
				}
				if info.Size() < 0 || info.Size() > limit {
					opened.Close()
					return errors.New("inventory overflow")
				}
				digest, n, hashErr := digestInventoryFile(opened, limit)
				after, statErr := opened.Stat()
				afterID, idErr := localNativeIDFile(opened)
				opened.Close()
				if hashErr != nil {
					return hashErr
				}
				if statErr != nil || idErr != nil || id != afterID || n != info.Size() || !stableInventoryInfo(info, after) {
					return errors.New("inventory file changed")
				}
				w.bytes += n
				if e = w.append("f", next, id, digest+":"+fmt.Sprint(n)); e != nil {
					return e
				}
			}
			after, e := root.Lstat(child)
			if e != nil || !stableInventoryInfo(info, after) {
				return errors.New("inventory entry changed")
			}
		}
		if readErr == io.EOF {
			break
		}
	}
	after, err := dir.Stat()
	if err != nil || !stableInventoryInfo(before, after) {
		return errors.New("inventory directory changed")
	}
	return nil
}

func digestInventoryFile(f *os.File, limit int64) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, limit+1))
	if err != nil {
		return "", n, err
	}
	if n > limit {
		return "", n, errors.New("inventory overflow")
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func recoveryProjection(inventory, digest string) (map[string]any, error) {
	if len(inventory) > automaticInventoryPolicy.encodedBytes || strings.Count(inventory, "|")+1 > automaticInventoryPolicy.nodes {
		return nil, errors.New("inventory overflow")
	}
	entries := []map[string]any{}
	count, files := 0, 0
	var total int64
	complete := true
	for _, item := range strings.Split(inventory, "|") {
		fields := strings.Split(item, ",")
		if len(fields) != 4 {
			return nil, errors.New("invalid inventory")
		}
		// inventoryPaths validates names; decode only one already bounded item.
		paths, err := inventoryPathsPolicy(item, automaticInventoryPolicy)
		if err != nil || len(paths) != 1 {
			return nil, errors.New("invalid inventory")
		}
		count++
		if fields[0] == "f" {
			tail := strings.LastIndex(fields[3], ":")
			if tail < 0 {
				return nil, errors.New("invalid inventory")
			}
			size, err := strconv.ParseInt(fields[3][tail+1:], 10, 64)
			if err != nil || size < 0 || size > automaticInventoryPolicy.fileBytes || size > automaticInventoryPolicy.totalBytes-total {
				return nil, errors.New("invalid inventory")
			}
			total += size
			files++
		}
		if len(entries) < 8 {
			path := paths[0]
			truncated := false
			if len(path) > 128 {
				path = path[:128]
				for !utf8.ValidString(path) {
					path = path[:len(path)-1]
				}
				truncated = true
				complete = false
			}
			entries = append(entries, map[string]any{"kind": fields[0], "path": path, "truncated": truncated})
		} else {
			complete = false
		}
	}
	out := map[string]any{"eligible": true, "inventorySHA256": digest, "entryCount": count, "fileCount": files, "totalBytes": total, "entries": entries, "entriesComplete": complete, "omittedEntries": count - len(entries), "previewOnly": true}
	raw, err := json.Marshal(out)
	if err != nil || len(raw) > 8192 {
		return nil, errors.New("recovery projection overflow")
	}
	return out, nil
}
