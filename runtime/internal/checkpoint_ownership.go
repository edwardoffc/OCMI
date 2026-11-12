package internal

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// These are private, request-local producer groups, not portable checkpoint
// metadata. Authority starts with an empty original directory, before effects.
// Inventory is bounded producer evidence within that authority; it does not
// establish isolation from an arbitrary concurrent same-user writer.
type checkpointOwnership struct {
	Path, ParentID, RootID, Layout string
	Paths                          map[string]bool
	Issued                         map[string]bool
	root, parent                   *os.Root
	active                         bool
}

func checkpointEmptyDirectory(path string) error {
	if err := checkpointPlainDirectory(path); err != nil {
		return err
	}
	r, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	defer r.Close()
	f, err := r.Open(".")
	if err != nil {
		return err
	}
	defer f.Close()
	entries, err := f.ReadDir(1)
	if err != nil && err != io.EOF {
		return err
	}
	if len(entries) != 0 {
		return errors.New("destination is occupied")
	}
	return nil
}

func checkpointHold(path, layout string, paths map[string]bool) (*checkpointOwnership, error) {
	g := &checkpointOwnership{Path: path, Layout: layout, Paths: paths, active: true}
	var err error
	defer func() {
		if err != nil {
			g.closeHandles()
		}
	}()
	if err = checkpointPlainDirectory(path); err != nil {
		return g, err
	}
	g.ParentID, err = localNativeID(filepath.Dir(path))
	if err == nil {
		g.RootID, err = localNativeID(path)
	}
	if err == nil {
		g.parent, err = os.OpenRoot(filepath.Dir(path))
	}
	if err == nil {
		g.root, err = os.OpenRoot(path)
	}
	if err == nil {
		err = g.identities()
	}
	if err == nil {
		var f *os.File
		f, err = g.root.Open(".")
		if err == nil {
			var entries []os.DirEntry
			entries, err = f.ReadDir(1)
			f.Close()
			if err == io.EOF {
				err = nil
			}
			if err == nil && len(entries) != 0 {
				err = errors.New("group is not initially empty")
			}
		}
	}
	// Native directory readers can deny DELETE sharing on Windows. Keep the
	// original IDs durable, and acquire identity-checked rooted handles for
	// bounded checks/effects rather than pinning handles across producer seams.
	g.closeHandles()
	return g, err
}

func (g *checkpointOwnership) openHandles() error {
	if err := g.identities(); err != nil {
		return err
	}
	g.closeHandles()
	var err error
	g.parent, err = os.OpenRoot(filepath.Dir(g.Path))
	if err == nil {
		g.root, err = os.OpenRoot(g.Path)
	}
	if err == nil {
		err = g.identities()
	}
	if err != nil {
		g.closeHandles()
	}
	return err
}

func (g *checkpointOwnership) identities() error {
	if !g.active {
		return nil
	}
	for path, expected := range map[string]string{filepath.Dir(g.Path): g.ParentID, g.Path: g.RootID} {
		if expected == "" || checkpointPlainDirectory(path) != nil || checkpointGitReparse(path) {
			return errors.New("group type changed")
		}
		id, err := localNativeID(path)
		if err != nil || id != expected {
			return errors.New("group identity changed")
		}
	}
	for root, expected := range map[*os.Root]string{g.parent: g.ParentID, g.root: g.RootID} {
		if root == nil {
			continue
		}
		f, err := root.Open(".")
		if err != nil {
			return err
		}
		id, err := localNativeIDFile(f)
		f.Close()
		if err != nil || id != expected {
			return errors.New("held group identity changed")
		}
	}
	return nil
}

func (g *checkpointOwnership) closeHandles() {
	if g.root != nil {
		g.root.Close()
		g.root = nil
	}
	if g.parent != nil {
		g.parent.Close()
		g.parent = nil
	}
}

func checkpointFileLayout(m checkpointManifest, capture bool) map[string]bool {
	paths := map[string]bool{".checkpoint-stage.json": false}
	if capture {
		paths["manifest.json"] = false
		paths["COMPLETE"] = false
		paths["data"] = true
	}
	for _, f := range m.Files {
		name := f.Path
		if capture {
			name = "data/" + name
		}
		paths[name] = false
		for p := filepath.ToSlash(filepath.Dir(name)); p != "."; p = filepath.ToSlash(filepath.Dir(p)) {
			paths[p] = true
		}
	}
	return paths
}

func checkpointGitRestoreLayout(m checkpointGitManifest) map[string]bool {
	paths := map[string]bool{".git": true, ".empty-template": true, "history.bundle": false}
	for _, f := range m.Git.Files {
		if f.State != "present" {
			continue
		}
		paths[f.Path] = false
		for p := filepath.ToSlash(filepath.Dir(f.Path)); p != "."; p = filepath.ToSlash(filepath.Dir(p)) {
			paths[p] = true
		}
	}
	return paths
}

// Only Git's template-free metadata layout is admitted, never arbitrary .git
// contents. Object/pack names remain bounded by the group inventory limits.
func checkpointGitProduced(name string, dir bool) bool {
	p := strings.Split(name, "/")
	if len(p) == 1 {
		if dir {
			return name == "objects" || name == "refs" || name == "logs" || name == "hooks" || name == "info" || name == "branches"
		}
		return name == "HEAD" || name == "config" || name == "description" || name == "index" || name == "packed-refs" || name == "ORIG_HEAD" || name == "FETCH_HEAD" || name == "shallow" || strings.HasSuffix(name, ".lock") && checkpointGitProduced(strings.TrimSuffix(name, ".lock"), false)
	}
	switch p[0] {
	case "logs":
		if name == "logs/HEAD" {
			return !dir
		}
		if name == "logs/refs" {
			return dir
		}
		if strings.HasPrefix(name, "logs/refs/") {
			return checkpointGitProduced(strings.TrimPrefix(name, "logs/"), dir)
		}
		return false
	case "refs":
		return (p[1] == "heads" || p[1] == "tags") && (dir || len(p) > 2 && checkpointGitRefName("refs/"+strings.Join(p[1:], "/")))
	case "objects":
		if len(p) == 2 && dir {
			return p[1] == "info" || p[1] == "pack" || checkpointGitHexCanonical(p[1], 2)
		}
		if len(p) == 3 && !dir {
			if checkpointGitHexCanonical(p[1], 2) {
				return checkpointGitHexCanonical(p[2], 38)
			}
			if p[1] == "pack" {
				for _, suffix := range []string{".pack", ".idx", ".rev", ".keep"} {
					if strings.HasPrefix(p[2], "pack-") && strings.HasSuffix(p[2], suffix) {
						return checkpointGitOIDCanonical(strings.TrimSuffix(strings.TrimPrefix(p[2], "pack-"), suffix))
					}
				}
			}
		}
	}
	return false
}

func (o *checkpointGitOperation) groupAllowed(g *checkpointOwnership, name string, dir bool) bool {
	if !dir && g.Issued[name] {
		return true
	}
	if want, ok := g.Paths[name]; ok {
		return want == dir
	}
	switch g.Layout {
	case "journal", "artifacts":
		if dir {
			for _, child := range o.groups {
				if child.active && child != g && child.Path == filepath.Join(g.Path, filepath.FromSlash(name)) {
					return true
				}
			}
		}
	case "git-capture":
		if name == "data" || name == "data/blobs" {
			return dir
		}
		if strings.HasPrefix(name, "data/blobs/") {
			return !dir && checkpointGitSHA256Canonical(strings.TrimPrefix(name, "data/blobs/"))
		}
		if !dir && (name == "history.bundle" || name == "manifest.json" || name == "COMPLETE") {
			return true
		}
	case "git-restore", "git-verify":
		if name == ".git" {
			return dir
		}
		if strings.HasPrefix(name, ".git/") {
			return checkpointGitProduced(strings.TrimPrefix(name, ".git/"), dir)
		}
		if g.Layout == "git-verify" && name == "history.bundle" {
			return !dir
		}
	case "git-hash-input":
		return !dir && checkpointGitSHA256Canonical(name)
	}
	return false
}

func (o *checkpointGitOperation) groupAt(path string) *checkpointOwnership {
	for _, g := range o.groups {
		if g.active && g.Path == path {
			return g
		}
	}
	return nil
}

// Read directories in small batches so even an over-bound foreign inventory
// cannot cause an unbounded ReadDir allocation. Return only verified members.
func (o *checkpointGitOperation) inventory(g *checkpointOwnership) ([]string, error) {
	if err := g.identities(); err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(g.Path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	id, err := unpackRootID(r)
	if err != nil || id != g.RootID {
		return nil, errors.New("opened group identity changed")
	}
	parent, err := os.OpenRoot(filepath.Dir(g.Path))
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	id, err = unpackRootID(parent)
	if err != nil || id != g.ParentID {
		return nil, errors.New("opened group parent identity changed")
	}
	var names []string
	var total int64
	var walk func(string) error
	walk = func(name string) error {
		if strings.Count(name, "/") > 32 || len(name) > 4096 {
			return errors.New("group depth bound")
		}
		f, err := r.Open(filepath.FromSlash(name))
		if err != nil {
			return err
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
				if len(names) >= 32768 {
					return errors.New("group member bound")
				}
				info, err := r.Lstat(filepath.FromSlash(rel))
				if err != nil || info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) || checkpointGitReparse(filepath.Join(g.Path, filepath.FromSlash(rel))) || !o.groupAllowed(g, rel, info.IsDir()) {
					return errors.New("unknown group member")
				}
				total += info.Size()
				if total > 2<<30 {
					return errors.New("group byte bound")
				}
				names = append(names, rel)
				delegated := false
				for _, child := range o.groups {
					if child != g && child.active && child.Path == filepath.Join(g.Path, filepath.FromSlash(rel)) {
						delegated = true
						break
					}
				}
				if info.IsDir() && !delegated {
					if err := walk(rel); err != nil {
						return err
					}
				}
			}
			if readErr == io.EOF {
				break
			}
		}
		return nil
	}
	err = walk(".")
	if err == nil {
		err = g.identities()
	}
	return names, err
}

func (o *checkpointGitOperation) guard() error {
	if o.unknown {
		return errors.New("ownership uncertainty retained")
	}
	for _, g := range o.groups {
		if !g.active {
			continue
		}
		if _, err := o.inventory(g); err != nil {
			o.unknown = true
			return err
		}
	}
	return nil
}

func (o *checkpointGitOperation) declareGroup(path, layout string, paths map[string]bool) (*checkpointOwnership, error) {
	g, err := checkpointHold(path, layout, paths)
	if g != nil {
		o.groups = append(o.groups, g)
	}
	if err == nil {
		err = o.record("group-intent", 0, 0, path)
	}
	if err != nil {
		o.unknown = true
	}
	return g, err
}

func (o *checkpointGitOperation) newGroup(parent, prefix, layout string, paths map[string]bool) (*checkpointOwnership, error) {
	if err := o.guard(); err != nil {
		return nil, err
	}
	path, err := os.MkdirTemp(parent, prefix)
	if err != nil {
		return nil, err
	}
	return o.declareGroup(path, layout, paths)
}

func (o *checkpointGitOperation) fixedGroup(path, layout string) error {
	if err := o.guard(); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0700); err != nil {
		return err
	}
	_, err := o.declareGroup(path, layout, nil)
	return err
}

func (o *checkpointGitOperation) cleanupGroups() error {
	if err := o.guard(); err != nil {
		return err
	}
	for i := len(o.groups) - 1; i >= 0; i-- {
		g := o.groups[i]
		if !g.active {
			continue
		}
		if err := g.openHandles(); err != nil {
			return err
		}
		names, err := o.inventory(g)
		if err != nil {
			return err
		}
		sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
		for _, name := range names {
			if err = g.identities(); err != nil {
				return err
			}
			if err = g.root.Remove(filepath.FromSlash(name)); err != nil {
				return err
			}
			delete(g.Issued, name)
		}
		if err = g.identities(); err != nil {
			return err
		}
		g.root.Close()
		g.root = nil
		if err = g.parent.Remove(filepath.Base(g.Path)); err != nil {
			return err
		}
		g.active = false
		g.closeHandles()
	}
	return nil
}

func (o *checkpointGitOperation) publish(g *checkpointOwnership, destination string) error {
	if err := o.guard(); err != nil {
		return err
	}
	if err := o.record("before-publication", 0, 0, destination); err != nil {
		o.unknown = true
		return err
	}
	parent := filepath.Dir(destination)
	pid := o.parentID
	source, err := filepath.Rel(parent, g.Path)
	if err != nil {
		return err
	}
	held, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer held.Close()
	g.closeHandles()
	err = unpackPublish(held, source, filepath.Base(destination), pid, g.RootID)
	if err == nil {
		g.active = false
	}
	return err
}

// Existing-empty destinations are captured before staging. The original is
// reserved under a unique no-replace leaf; rollback never replaces an arrival.
func (o *checkpointGitOperation) activate(g, destination *checkpointOwnership) error {
	if err := destination.identities(); err != nil {
		return err
	}
	if err := checkpointEmptyDirectory(destination.Path); err != nil {
		return err
	}
	if err := o.guard(); err != nil {
		return err
	}
	if err := o.record("before-reservation", 0, 0, destination.RootID); err != nil {
		o.unknown = true
		return err
	}
	backup := filepath.Base(o.journalRoot) + "-empty-root"
	destination.closeHandles()
	held, err := os.OpenRoot(filepath.Dir(destination.Path))
	if err != nil {
		return err
	}
	defer held.Close()
	if err = destination.identities(); err != nil {
		return err
	}
	if err = unpackPublish(held, filepath.Base(destination.Path), backup, destination.ParentID, destination.RootID); err != nil {
		return err
	}
	originalPath := destination.Path
	destination.Path = filepath.Join(filepath.Dir(originalPath), backup)
	// Keep the reservation separate from recursive group cleanup. A failed
	// rollback deliberately retains both roots with the private phase journal.
	if err = destination.identities(); err == nil {
		err = checkpointEmptyDirectory(destination.Path)
	}
	if err == nil {
		err = o.publish(g, originalPath)
	}
	if err != nil {
		if destination.identities() != nil || checkpointEmptyDirectory(destination.Path) != nil || unpackPublish(held, backup, filepath.Base(originalPath), destination.ParentID, destination.RootID) != nil {
			o.unknown = true
		} else {
			destination.Path = originalPath
		}
		return err
	}
	if err = destination.identities(); err == nil {
		err = checkpointEmptyDirectory(destination.Path)
	}
	if err == nil {
		err = held.Remove(backup)
	}
	if err != nil {
		o.unknown = true
	}
	destination.active = false
	return err
}

func (o *checkpointGitOperation) recordDestination(g *checkpointOwnership) error {
	o.destination = g
	if err := o.record("original-empty-destination", 0, 0, g.Path); err != nil {
		o.unknown = true
		return err
	}
	return nil
}
