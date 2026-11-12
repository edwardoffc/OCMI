package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestWorkerUpdateProofMetadata(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		t.Run(map[bool]string{false: "init-only", true: "prepared"}[prepared], func(t *testing.T) {
			base := t.TempDir()
			home := filepath.Join(base, "home")
			next := filepath.Join(base, "next")
			acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
			acceptanceUpdateFactory(t, next, strings.Repeat("b", 40))
			if r := InstallHome(home, false); !r.OK {
				t.Fatalf("install: %+v", r)
			}
			if prepared {
				if r := CoordinatorPrepare(home, "en", "Fictional project"); !r.OK {
					t.Fatalf("prepare: %+v", r)
				}
			}
			context, info, err := verifyFactoryHome(home)
			if err != nil {
				t.Fatal(err)
			}
			p, err := captureUpdateMetadata(context, info, strings.Repeat("b", 40), next)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := encodeUpdateMetadataProof(p)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := decodeUpdateMetadataProof(raw)
			if err != nil || !reflect.DeepEqual(p, decoded) {
				t.Fatalf("roundtrip: %v", err)
			}
			if _, err := decodeUpdateMetadataProof(append(raw, []byte(" garbage")...)); err == nil {
				t.Fatal("accepted trailing data")
			}
			oldPublic, err := captureUpdatePublicProof(home, true)
			if err != nil {
				t.Fatal(err)
			}
			newPublic, err := captureUpdatePublicProof(next, false)
			if err != nil {
				t.Fatal(err)
			}
			id := uuid.NewString()
			journal := updateProofJournal{SchemaVersion: 1, ID: id, Phase: "intent", Metadata: p, OldSource: p.OldSource, NewSource: p.NewSource, OldPublic: oldPublic, NewPublic: newPublic, Stage: "local/runs/update-" + id, OldSlot: "old", NewSlot: "new", AdmissionID: "fixture-admission"}
			journalRaw, err := encodeUpdateProofJournal(journal)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := decodeUpdateProofJournal(journalRaw); err != nil || !reflect.DeepEqual(got, journal) {
				t.Fatalf("journal roundtrip: %v", err)
			}
			mutations := []func(*updateProofJournal){func(j *updateProofJournal) { j.SchemaVersion = 2 }, func(j *updateProofJournal) { j.OldSource = strings.Repeat("c", 40) }, func(j *updateProofJournal) { j.Stage = "local/runs/foreign" }, func(j *updateProofJournal) { j.OldPublic.RootID = "" }, func(j *updateProofJournal) { j.Phase = "unknown" }, func(j *updateProofJournal) { j.Phase = "staged" }, func(j *updateProofJournal) { j.OldCopied = 1 }, func(j *updateProofJournal) { j.NewCopied = len(j.NewPublic.Files) + 1 }}
			for i, mutate := range mutations {
				bad := journal
				mutate(&bad)
				raw, err := json.Marshal(bad)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = decodeUpdateProofJournal(raw); err == nil {
					t.Fatalf("accepted mutation %d", i)
				}
			}
			if _, err := decodeUpdateProofJournal(append(journalRaw, []byte(" trailing")...)); err == nil {
				t.Fatal("accepted trailing journal data")
			}
			if err := verifyUpdateMetadataBefore(context, p); err != nil {
				t.Fatal(err)
			}
			if prepared {
				if len(p.PromptBefore) == 0 || bytes.Equal(p.PromptBefore, p.PromptAfter) {
					t.Fatal("missing source-dependent prompt")
				}
				path := filepath.Join(home, "local", "bindings", "coordinator", "STARTUP.md")
				if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := captureUpdateMetadata(context, info, strings.Repeat("b", 40), next); err == nil {
					t.Fatal("accepted changed prompt")
				}
				if err := verifyUpdateMetadataBefore(context, p); err == nil {
					t.Fatal("accepted changed prompt proof")
				}
				altered := p
				altered.PromptAfter = []byte("forged")
				if _, err := encodeUpdateMetadataProof(altered); err == nil {
					t.Fatal("accepted forged predicted prompt")
				}
			} else if len(p.BindingBefore) != 0 {
				t.Fatal("unexpected binding")
			} else {
				if err := os.MkdirAll(filepath.Join(home, "OCMI_Sync", "coordinator"), 0700); err != nil {
					t.Fatal(err)
				}
				if _, err := captureUpdateMetadata(context, info, strings.Repeat("b", 40), next); err != nil {
					t.Fatalf("opaque restored memory: %v", err)
				}
			}
		})
	}
}

func TestWorkerUpdatePublicProof(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "factory")
	acceptanceUpdateFactory(t, root, strings.Repeat("a", 40))
	p, err := captureUpdatePublicProof(root, false)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := encodeUpdatePublicProof(p)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeUpdatePublicProof(raw)
	if err != nil || !reflect.DeepEqual(p, decoded) {
		t.Fatalf("roundtrip: %v", err)
	}
	if _, err := decodeUpdatePublicProof(make([]byte, 2<<20+1)); err == nil {
		t.Fatal("accepted oversized proof")
	}
	if err := verifyUpdatePublicProof(root, p, false, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "empty"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := verifyUpdatePublicProof(root, p, false, true); err == nil {
		t.Fatal("accepted empty directory")
	}
	if err := os.Remove(filepath.Join(root, "empty")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("resealed"), 0700); err != nil {
		t.Fatal(err)
	}
	sums, err := os.ReadFile(filepath.Join(root, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(sums)), "\n")
	for i, line := range lines {
		if strings.HasSuffix(line, "  README.md") {
			lines[i] = fmt.Sprintf("%s  README.md", updateDigest([]byte("resealed")))
		}
	}
	if err := os.WriteFile(filepath.Join(root, "SHA256SUMS"), []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := verifyFactoryHome(root); err != nil {
		t.Fatalf("resealed control: %v", err)
	}
	if err := verifyUpdatePublicProof(root, p, false, false); err == nil {
		t.Fatal("accepted resealed bytes")
	}
	other := filepath.Join(base, "replacement")
	acceptanceUpdateFactory(t, other, strings.Repeat("a", 40))
	if err := verifyUpdatePublicProof(other, p, false, false); err != nil {
		t.Fatalf("copy comparison: %v", err)
	}
	if err := verifyUpdatePublicProof(other, p, false, true); err == nil {
		t.Fatal("accepted replaced root")
	}
}
