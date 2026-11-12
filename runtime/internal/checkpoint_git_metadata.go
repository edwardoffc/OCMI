package internal

import "strings"

// checkpointGitManifestSemantics validates the closed manifest relations that
// can be established without opening its package payload. Payload hashes and
// Git object availability are verified by the package reader/importer.
func checkpointGitManifestSemantics(m checkpointGitManifest) bool {
	g := m.Git
	if m.SchemaVersion != gitCheckpointSchema || m.Kind != "git-wip" ||
		!checkpointText(m.CheckpointID, 256) || !checkpointText(m.SystemVersion, 256) ||
		!checkpointText(m.SourceIdentity, 256) || !checkpointText(m.Owner, 4096) ||
		!checkpointText(m.Purpose, 4096) || !checkpointText(m.RetainUntil, 4096) ||
		!checkpointText(m.NextAction, 4096) || len(m.References) > 128 ||
		!checkpointReferences(m.References) || g.ObjectFormat != "sha1" ||
		!checkpointGitOIDCanonical(g.HeadOID) || !checkpointGitRefName(g.AttachedRef) ||
		!checkpointGitBundleSemantics(g.Bundle) || len(g.Refs) == 0 || len(g.Refs) > 32 ||
		len(g.Files) == 0 || len(g.Files) > gitMaxFiles || len(g.Index) > gitMaxFiles ||
		len(g.ExtraFiles) > gitMaxFiles || len(g.Blobs) > gitMaxFiles {
		return false
	}

	refs := make(map[string]string, len(g.Refs))
	refNames := make(map[string]string, len(g.Refs))
	for i, ref := range g.Refs {
		if !checkpointGitRefName(ref.Name) || !checkpointGitOIDCanonical(ref.OID) ||
			!checkpointGitRefCompatible(refNames, ref.Name) || (i > 0 && g.Refs[i-1].Name >= ref.Name) {
			return false
		}
		refs[ref.Name] = ref.OID
	}
	if refs[g.AttachedRef] != g.HeadOID {
		return false
	}

	paths := make(map[string]string, len(g.Files)+len(g.Index)+len(g.ExtraFiles))
	files := make(map[string]checkpointGitFile, len(g.Files))
	for i, file := range g.Files {
		if _, exists := files[file.Path]; exists {
			return false
		}
		if !checkpointGitPathCompatible(paths, file.Path) || (i > 0 && g.Files[i-1].Path >= file.Path) ||
			(file.State != "present" && file.State != "absent") ||
			(file.State == "present" && !checkpointGitSHA256Canonical(file.SHA256)) ||
			(file.State == "absent" && (file.SHA256 != "" || file.Executable)) {
			return false
		}
		files[file.Path] = file
	}
	indexPaths := make(map[string]bool, len(g.Index))
	for i, entry := range g.Index {
		if indexPaths[entry.Path] || !checkpointGitPathCompatible(paths, entry.Path) || (i > 0 && g.Index[i-1].Path >= entry.Path) ||
			(entry.Mode != "100644" && entry.Mode != "100755") || !checkpointGitOIDCanonical(entry.OID) ||
			files[entry.Path].Path == "" {
			return false
		}
		indexPaths[entry.Path] = true
	}
	extraPaths := make(map[string]bool, len(g.ExtraFiles))
	for i, path := range g.ExtraFiles {
		file, ok := files[path]
		if !ok || file.State != "present" || extraPaths[path] || !checkpointGitPathCompatible(paths, path) ||
			(i > 0 && g.ExtraFiles[i-1] >= path) {
			return false
		}
		if indexPaths[path] {
			return false
		}
		extraPaths[path] = true
	}

	blobs := make(map[string]checkpointGitBlob, len(g.Blobs))
	oids := make(map[string]bool, len(g.Blobs))
	var total int64
	for i, blob := range g.Blobs {
		if !checkpointGitSHA256Canonical(blob.SHA256) || !checkpointGitOIDCanonical(blob.OID) ||
			blob.Size < 0 || blob.Size > gitMaxBlob || total > gitMaxTotal-blob.Size ||
			(i > 0 && g.Blobs[i-1].SHA256 >= blob.SHA256) || oids[blob.OID] {
			return false
		}
		total += blob.Size
		blobs[blob.SHA256] = blob
		oids[blob.OID] = true
	}

	used := make(map[string]bool, len(g.Blobs))
	for _, entry := range g.Index {
		found := ""
		for sha, blob := range blobs {
			if blob.OID == entry.OID {
				found = sha
				break
			}
		}
		if found == "" {
			return false
		}
		used[found] = true
	}
	for _, file := range g.Files {
		if file.State == "present" {
			if _, ok := blobs[file.SHA256]; !ok {
				return false
			}
			used[file.SHA256] = true
		}
	}
	return len(used) == len(blobs)
}

func checkpointGitBundleSemantics(b checkpointGitBundle) bool {
	return b.Path == "history.bundle" && b.Size > 0 && b.Size <= gitMaxBundle && checkpointGitSHA256Canonical(b.SHA256)
}

func checkpointGitOIDCanonical(s string) bool    { return checkpointGitHexCanonical(s, 40) }
func checkpointGitSHA256Canonical(s string) bool { return checkpointGitHexCanonical(s, 64) }

func checkpointGitHexCanonical(s string, length int) bool {
	if len(s) != length || strings.ToLower(s) != s {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func checkpointGitRefName(name string) bool {
	if !strings.HasPrefix(name, "refs/heads/") || len(name) == len("refs/heads/") || strings.ContainsAny(name, " ~^:?*[\\") ||
		strings.Contains(name, "..") || strings.Contains(name, "@{") || strings.HasSuffix(name, ".") || strings.Contains(name, "//") {
		return false
	}
	for _, part := range strings.Split(strings.TrimPrefix(name, "refs/heads/"), "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	for _, c := range name {
		if c <= 0x20 || c == 0x7f {
			return false
		}
	}
	return true
}

func checkpointGitRefCompatible(refs map[string]string, name string) bool {
	key := strings.ToLower(name)
	for oldKey := range refs {
		if key == oldKey || strings.HasPrefix(key, oldKey+"/") || strings.HasPrefix(oldKey, key+"/") {
			return false
		}
	}
	refs[key] = name
	return true
}

func checkpointGitPathCompatible(paths map[string]string, path string) bool {
	if !checkpointGitName(path) {
		return false
	}
	key := strings.ToLower(path)
	for oldKey, oldPath := range paths {
		if key == oldKey {
			return oldPath == path
		}
		if strings.HasPrefix(key, oldKey+"/") || strings.HasPrefix(oldKey, key+"/") {
			return false
		}
	}
	paths[key] = path
	return true
}
