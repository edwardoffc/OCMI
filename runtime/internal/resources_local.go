package internal

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

const localMarker = ".ocmi-resource-token"

type localResourceProof struct {
	NativeIdentity, Inventory, ObservedState string
	EffectStarted                            bool
}

// localName is deliberately stricter than os.Root. Root confines traversal,
// but may follow a link which remains inside root; that is not ownership.
func localName(locator string, internal bool) (string, error) {
	n := filepath.Clean(filepath.FromSlash(locator))
	if n == "." || filepath.IsAbs(n) || n == ".." || strings.HasPrefix(n, ".."+string(os.PathSeparator)) {
		return "", errors.New("unsafe locator")
	}
	parts := strings.Split(n, string(os.PathSeparator))
	for _, p := range parts {
		switch strings.ToLower(p) {
		case ".git", ".ocmi":
			if !internal {
				return "", errors.New("reserved locator")
			}
		}
	}
	projectRun := len(parts) == 4 && strings.EqualFold(parts[0], ".ocmi") && strings.EqualFold(parts[1], "local") && strings.EqualFold(parts[2], "runs") && isUUID(parts[3])
	homeRun := len(parts) == 3 && strings.EqualFold(parts[0], "local") && strings.EqualFold(parts[1], "runs") && isUUID(parts[2])
	if internal && !projectRun && !homeRun {
		return "", errors.New("invalid internal locator")
	}
	return n, nil
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return false
			}
			continue
		}
		if !unicode.Is(unicode.ASCII_Hex_Digit, r) {
			return false
		}
	}
	return true
}

// plainComponents rejects every existing link/reparse point in a managed
// placement. The leaf may be absent only for an exclusive create.
func plainComponents(root, name string, allowMissingLeaf bool) error {
	cur := root
	parts := strings.Split(name, string(os.PathSeparator))
	for i, part := range parts {
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if err != nil {
			if os.IsNotExist(err) && allowMissingLeaf && i == len(parts)-1 {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 {
			return errors.New("linked placement")
		}
		if i < len(parts)-1 && !info.IsDir() {
			return errors.New("non-directory component")
		}
	}
	return nil
}

func inspectLocalResource(root, locator, kind string, external bool) (localResourceProof, error) {
	if external {
		// Selected external paths are observation only: absolute paths and links
		// are valid here and never become deletion/write capability.
		path := filepath.FromSlash(locator)
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		f, err := os.Open(path)
		if err != nil {
			return localResourceProof{ObservedState: "absent"}, err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return localResourceProof{ObservedState: "unknown"}, err
		}
		if (kind == "file" && !info.Mode().IsRegular()) || (kind == "directory" && !info.IsDir()) || (kind != "file" && kind != "directory") {
			return localResourceProof{ObservedState: "unknown"}, errors.New("external kind mismatch")
		}
		id, err := localNativeIDFile(f)
		if err != nil {
			return localResourceProof{ObservedState: "unknown"}, err
		}
		return localResourceProof{NativeIdentity: id, ObservedState: "external"}, nil
	}
	return captureLocalResource(root, locator, kind, "", false)
}

func createLocalResource(root, locator, kind, token string, content []byte, internal bool) (localResourceProof, error) {
	n, err := localName(locator, internal)
	if err != nil {
		return localResourceProof{}, err
	}
	if err = plainComponents(root, n, true); err != nil {
		return localResourceProof{}, err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return localResourceProof{}, err
	}
	defer r.Close()
	p := localResourceProof{}
	switch kind {
	case "file":
		f, e := r.OpenFile(n, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return p, e
		} // O_EXCL proves this call had no effect.
		p.EffectStarted = true
		_, e = f.Write(content)
		closeErr := f.Close()
		if e != nil {
			return p, e
		}
		if closeErr != nil {
			return p, closeErr
		}
	case "directory":
		if err = r.Mkdir(n, 0700); err != nil {
			return p, err
		}
		p.EffectStarted = true
		marker, e := r.OpenFile(filepath.Join(n, localMarker), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return p, e
		}
		_, e = marker.Write([]byte(token))
		closeErr := marker.Close()
		if e != nil {
			return p, e
		}
		if closeErr != nil {
			return p, closeErr
		}
	default:
		return p, errors.New("unsupported local kind")
	}
	proof, err := captureLocalResource(root, n, kind, "", internal)
	if err != nil {
		p.EffectStarted = true
		return p, err
	}
	proof.EffectStarted = true
	return proof, nil
}

func writeLocalResource(root, locator, kind, expectedIdentity, expectedInventory, path string, content []byte, internal bool) (localResourceProof, error) {
	if kind != "directory" {
		return localResourceProof{}, errors.New("writes require directory")
	}
	p, err := captureLocalResource(root, locator, kind, expectedIdentity, internal)
	if err != nil || p.Inventory != expectedInventory {
		return localResourceProof{}, errors.New("resource changed")
	}
	child, err := localName(path, false)
	if err != nil {
		return localResourceProof{}, err
	}
	if err = plainComponents(filepath.Join(root, locator), child, true); err != nil {
		return localResourceProof{}, err
	}
	held, heldID, err := openHeldResourceRoot(root, locator, expectedIdentity)
	if err != nil || heldID != p.NativeIdentity {
		return localResourceProof{}, errors.New("resource changed")
	}
	defer held.Close()
	f, err := held.OpenFile(child, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return localResourceProof{}, err
	}
	started := localResourceProof{EffectStarted: true}
	_, e := f.Write(content)
	closeErr := f.Close()
	if e != nil {
		return started, e
	}
	if closeErr != nil {
		return started, closeErr
	}
	proof, err := captureLocalResource(root, locator, kind, expectedIdentity, internal)
	if err != nil {
		return started, err
	}
	proof.EffectStarted = true
	return proof, nil
}

func removeLocalResource(root, locator, kind, expectedIdentity, expectedInventory string, internal bool) error {
	n, err := localName(locator, internal)
	if err != nil {
		return err
	}
	p, err := captureLocalResource(root, n, kind, expectedIdentity, internal)
	if err != nil || p.Inventory != expectedInventory {
		return errors.New("resource changed")
	}
	if err = plainComponents(root, n, false); err != nil {
		return err
	}
	q, qerr := captureLocalResource(root, n, kind, expectedIdentity, internal)
	if qerr != nil || q.Inventory != expectedInventory {
		return errors.New("resource changed")
	}
	if kind == "file" {
		r, e := os.OpenRoot(root)
		if e != nil {
			return e
		}
		defer r.Close()
		if err = r.Remove(n); err != nil {
			return err
		}
		if _, err = os.Lstat(filepath.Join(root, n)); !os.IsNotExist(err) {
			return errors.New("absence not verified")
		}
		return nil
	}
	paths, err := inventoryPaths(p.Inventory)
	if err != nil {
		return err
	}
	held, heldID, err := openHeldResourceRoot(root, n, expectedIdentity)
	if err != nil || heldID != expectedIdentity {
		return errors.New("resource changed")
	}
	for i := len(paths) - 1; i >= 0; i-- {
		if err = held.Remove(paths[i]); err != nil {
			held.Close()
			return err
		}
	}
	if err = held.Close(); err != nil {
		return err
	}
	parent, err := os.OpenRoot(filepath.Dir(filepath.Join(root, n)))
	if err != nil {
		return err
	}
	defer parent.Close()
	base := filepath.Base(n)
	if now, e := localNativeID(filepath.Join(root, n)); e != nil || now != expectedIdentity {
		return errors.New("resource changed")
	}
	if err = parent.Remove(base); err != nil {
		return err
	}
	if _, err = os.Lstat(filepath.Join(root, n)); !os.IsNotExist(err) {
		return errors.New("absence not verified")
	}
	return nil
}

func inventoryPaths(inventory string) ([]string, error) {
	if inventory == "" {
		return nil, nil
	}
	parts := strings.Split(inventory, "|")
	paths := make([]string, 0, len(parts))
	seen := make(map[string]bool)
	for _, item := range parts {
		fields := strings.Split(item, ",")
		if len(fields) != 4 || (fields[0] != "f" && fields[0] != "d") {
			return nil, errors.New("invalid inventory")
		}
		decoded, err := base64.RawURLEncoding.DecodeString(fields[1])
		if err != nil {
			return nil, errors.New("invalid inventory")
		}
		name, err := localName(string(decoded), false)
		if err != nil {
			return nil, errors.New("invalid inventory")
		}
		paths = append(paths, name)
		seen[name] = true
	}
	sort.Slice(paths, func(i, j int) bool {
		di, dj := strings.Count(filepath.ToSlash(paths[i]), "/"), strings.Count(filepath.ToSlash(paths[j]), "/")
		if di != dj {
			return di < dj
		}
		return paths[i] < paths[j]
	})
	return paths, nil
}

func captureLocalResource(root, locator, kind, expectedIdentity string, internal bool) (localResourceProof, error) {
	n, err := localName(locator, internal)
	if err != nil {
		return localResourceProof{}, err
	}
	if err = plainComponents(root, n, false); err != nil {
		return localResourceProof{}, err
	}
	path := filepath.Join(root, n)
	id, err := localNativeID(path)
	if err != nil {
		return localResourceProof{}, err
	}
	if expectedIdentity != "" && expectedIdentity != id {
		return localResourceProof{}, errors.New("identity changed")
	}
	if kind == "file" {
		digest, size, e := localDigest(path)
		if e != nil {
			return localResourceProof{}, e
		}
		return localResourceProof{NativeIdentity: id, Inventory: fmt.Sprintf("file:%s:%d", digest, size), ObservedState: "present"}, nil
	}
	if kind != "directory" {
		return localResourceProof{}, errors.New("unsupported local kind")
	}
	held, heldID, err := openHeldResourceRoot(root, n, expectedIdentity)
	if err != nil {
		return localResourceProof{}, err
	}
	defer held.Close()
	if id != heldID {
		return localResourceProof{}, errors.New("held identity changed")
	}
	entries, total := make([]string, 0), int64(0)
	if err = walkLocalInventory(held, ".", "", &entries, &total); err != nil {
		return localResourceProof{}, err
	}
	if !containsMarker(entries) {
		return localResourceProof{}, errors.New("missing provenance marker")
	}
	sort.Strings(entries)
	return localResourceProof{NativeIdentity: id, Inventory: strings.Join(entries, "|"), ObservedState: "present"}, nil
}

func containsMarker(entries []string) bool {
	for _, e := range entries {
		parts := strings.Split(e, ",")
		if len(parts) == 4 && parts[0] == "f" {
			name, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err == nil && string(name) == localMarker {
				return true
			}
		}
	}
	return false
}

func openHeldResourceRoot(root, name, expected string) (*os.Root, string, error) {
	r, err := os.OpenRoot(filepath.Join(root, name))
	if err != nil {
		return nil, "", err
	}
	f, err := r.Open(".")
	if err != nil {
		r.Close()
		return nil, "", err
	}
	id, err := localNativeIDFile(f)
	f.Close()
	if err != nil {
		r.Close()
		return nil, "", err
	}
	if expected != "" && expected != id {
		r.Close()
		return nil, "", errors.New("identity changed")
	}
	return r, id, nil
}

func walkLocalInventory(root *os.Root, openName, rel string, entries *[]string, total *int64) error {
	dir, err := root.Open(openName)
	if err != nil {
		return err
	}
	defer dir.Close()
	list, err := dir.ReadDir(-1)
	if err != nil {
		return err
	}
	for _, ent := range list {
		child := ent.Name()
		if openName != "." {
			child = filepath.Join(openName, child)
		}
		info, e := root.Lstat(child)
		if e != nil {
			return e
		}
		if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 {
			return errors.New("unverified entry")
		}
		next := ent.Name()
		if rel != "" {
			next = filepath.Join(rel, next)
		}
		if info.IsDir() {
			if len(*entries) >= 256 {
				return errors.New("inventory overflow")
			}
			opened, e := root.Open(child)
			if e != nil {
				return e
			}
			id, e := localNativeIDFile(opened)
			opened.Close()
			if e != nil {
				return e
			}
			*entries = append(*entries, inventoryItem("d", next, id, "0"))
			if e = walkLocalInventory(root, child, next, entries, total); e != nil {
				return e
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return errors.New("unverified entry")
		}
		if len(*entries) >= 256 {
			return errors.New("inventory overflow")
		}
		*total += info.Size()
		if *total > 16<<20 {
			return errors.New("inventory overflow")
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
		digest, _, e := digestOpenFile(opened)
		opened.Close()
		if e != nil {
			return e
		}
		*entries = append(*entries, inventoryItem("f", next, id, digest+":"+fmt.Sprint(info.Size())))
	}
	return nil
}

func inventoryItem(kind, path, id, tail string) string {
	return kind + "," + base64.RawURLEncoding.EncodeToString([]byte(filepath.ToSlash(path))) + "," + id + "," + tail
}

func digestOpenFile(f *os.File) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, (16<<20)+1))
	if err != nil {
		return "", 0, err
	}
	if n > 16<<20 {
		return "", 0, errors.New("inventory overflow")
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func localDigest(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, (16<<20)+1))
	if err != nil {
		return "", 0, err
	}
	if n > 16<<20 {
		return "", 0, errors.New("inventory overflow")
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
