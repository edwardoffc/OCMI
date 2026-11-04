package internal

import (
	"fmt"
	"strings"
	"testing"
)

func TestCheckpointGitManifestSemanticsWorkerAcceptsClosedMetadata(t *testing.T) {
	if !checkpointGitManifestSemantics(workerValidGitManifest()) {
		t.Fatal("valid closed Git metadata refused")
	}
}

func TestCheckpointGitManifestSemanticsWorkerRejectsRelationsAndBounds(t *testing.T) {
	tests := []struct {
		name string
		edit func(*checkpointGitManifest)
	}{
		{"missing owner", func(m *checkpointGitManifest) { m.Owner = "" }},
		{"unbound head", func(m *checkpointGitManifest) { m.Git.HeadOID = strings.Repeat("d", 40) }},
		{"registry orphan", func(m *checkpointGitManifest) {
			m.Git.Blobs = append(m.Git.Blobs, checkpointGitBlob{SHA256: strings.Repeat("d", 64), OID: strings.Repeat("d", 40), Size: 1})
		}},
		{"missing registry link", func(m *checkpointGitManifest) { m.Git.Files[0].SHA256 = strings.Repeat("e", 64) }},
		{"casefold path collision", func(m *checkpointGitManifest) {
			m.Git.Files = append(m.Git.Files, checkpointGitFile{Path: "Readme", State: "present", SHA256: strings.Repeat("a", 64)})
		}},
		{"prefix path collision", func(m *checkpointGitManifest) {
			m.Git.Files = append(m.Git.Files, checkpointGitFile{Path: "README/child", State: "present", SHA256: strings.Repeat("a", 64)})
		}},
		{"aggregate registry bound overflow", func(m *checkpointGitManifest) { *m = workerGitManifestWithBlobs(9) }},
		{"noncanonical hash", func(m *checkpointGitManifest) { m.Git.Bundle.SHA256 = strings.Repeat("A", 64) }},
		{"extra tracked index path", func(m *checkpointGitManifest) { m.Git.ExtraFiles = []string{"README"} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := workerValidGitManifest()
			tc.edit(&m)
			if checkpointGitManifestSemantics(m) {
				t.Fatal("invalid metadata accepted")
			}
		})
	}
}

func TestCheckpointGitManifestSemanticsWorkerAcceptsAggregateRegistryBoundary(t *testing.T) {
	if !checkpointGitManifestSemantics(workerGitManifestWithBlobs(8)) {
		t.Fatal("eight 32 MiB blobs must meet the 256 MiB aggregate bound")
	}
}

func TestCheckpointGitManifestSemanticsWorkerRejectsRefFormatAndPortableCollisions(t *testing.T) {
	tests := []struct {
		name string
		edit func(*checkpointGitManifest)
	}{
		{"control character", func(m *checkpointGitManifest) {
			m.Git.AttachedRef = "refs/heads/line\nbreak"
			m.Git.Refs[0].Name = m.Git.AttachedRef
		}},
		{"leading dot component", func(m *checkpointGitManifest) {
			m.Git.AttachedRef = "refs/heads/.hidden"
			m.Git.Refs[0].Name = m.Git.AttachedRef
		}},
		{"casefold collision", func(m *checkpointGitManifest) {
			m.Git.Refs = []checkpointGitRef{{Name: "refs/heads/Main", OID: m.Git.HeadOID}, {Name: "refs/heads/main", OID: m.Git.HeadOID}}
		}},
		{"prefix collision", func(m *checkpointGitManifest) {
			m.Git.AttachedRef = "refs/heads/foo"
			m.Git.Refs = []checkpointGitRef{{Name: "refs/heads/foo", OID: m.Git.HeadOID}, {Name: "refs/heads/foo/bar", OID: m.Git.HeadOID}}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := workerValidGitManifest()
			tc.edit(&m)
			if checkpointGitManifestSemantics(m) {
				t.Fatal("unsafe Git ref metadata accepted")
			}
		})
	}
}

func workerValidGitManifest() checkpointGitManifest {
	sha := strings.Repeat("a", 64)
	oid := strings.Repeat("b", 40)
	head := strings.Repeat("c", 40)
	return checkpointGitManifest{
		SchemaVersion: gitCheckpointSchema, Kind: "git-wip", CheckpointID: "checkpoint", SystemVersion: "1.3.5",
		SourceIdentity: "selected-source", Owner: "owner", Purpose: "recovery", RetainUntil: "review", NextAction: "rebind",
		Git: checkpointGitInfo{
			ObjectFormat: "sha1", HeadOID: head, AttachedRef: "refs/heads/main",
			Refs:   []checkpointGitRef{{Name: "refs/heads/main", OID: head}},
			Bundle: checkpointGitBundle{Path: "history.bundle", Size: 1, SHA256: strings.Repeat("f", 64)},
			Index:  []checkpointGitIndex{{Path: "README", Mode: "100644", OID: oid}},
			Files:  []checkpointGitFile{{Path: "README", State: "present", SHA256: sha}},
			Blobs:  []checkpointGitBlob{{SHA256: sha, OID: oid, Size: 1}},
		},
	}
}

func workerGitManifestWithBlobs(count int) checkpointGitManifest {
	m := workerValidGitManifest()
	m.Git.Index = nil
	m.Git.Files = nil
	m.Git.Blobs = nil
	for i := 1; i <= count; i++ {
		sha := fmt.Sprintf("%064x", i)
		oid := fmt.Sprintf("%040x", i)
		m.Git.Blobs = append(m.Git.Blobs, checkpointGitBlob{SHA256: sha, OID: oid, Size: gitMaxBlob})
		m.Git.Files = append(m.Git.Files, checkpointGitFile{Path: fmt.Sprintf("file-%02d", i), State: "present", SHA256: sha})
	}
	return m
}
