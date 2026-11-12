package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// These controls use only the existing private phase seams. Replacement objects
// belong to the fixture, never to the checkpoint operation's cleanup authority.
func TestCheckpointOwnershipFileCapturePreservesReplacedStage(t *testing.T) {
	source, parent := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "note.txt"), []byte("expected"), 0600); err != nil {
		t.Fatal(err)
	}
	var replaced string
	checkpointCaptureBeforeVerify = func() {
		entries, err := os.ReadDir(parent)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), ".ocmi-checkpoint-") {
				if replaced != "" {
					t.Fatal("ambiguous selected stage fixture")
				}
				replaced = filepath.Join(parent, entry.Name())
			}
		}
		if replaced == "" {
			t.Fatal("checkpoint phase did not expose its selected stage")
		}
		if err := os.Rename(replaced, filepath.Join(parent, "retained-original-stage")); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(replaced, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(replaced, "foreign.txt"), []byte("preserve"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { checkpointCaptureBeforeVerify = nil })
	got := checkpointRequest(t, source, "checkpoint.capture", checkpointPayload(filepath.Join(parent, "closed-package")))
	if got.OK {
		t.Fatalf("replacement accepted: %+v", got)
	}
	if data, err := os.ReadFile(filepath.Join(replaced, "foreign.txt")); err != nil || string(data) != "preserve" {
		t.Fatalf("FOREIGN_STAGE_REMOVED: %q %v; response=%+v", data, err, got)
	}
	if got.Data["ownedStage"] == nil || got.Data["nextAction"] == nil {
		t.Fatalf("uncertain ownership has no actionable retained-stage receipt: %+v", got)
	}
}

func TestCheckpointOwnershipFileRestorePreservesReplacedDestination(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "note.txt"), []byte("expected"), 0600); err != nil {
		t.Fatal(err)
	}
	closed := filepath.Join(t.TempDir(), "checkpoint")
	if got := checkpointRequest(t, source, "checkpoint.capture", checkpointPayload(closed)); !got.OK {
		t.Fatal(got)
	}
	parent := t.TempDir()
	destination := filepath.Join(parent, "destination")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	originalID, err := localNativeID(destination)
	if err != nil {
		t.Fatal(err)
	}
	var replacementID string
	entered := false
	checkpointRestoreBeforeActivation = func() error {
		entered = true
		if err := os.Rename(destination, filepath.Join(parent, "retained-original-empty")); err != nil {
			return err
		}
		if err := os.Mkdir(destination, 0700); err != nil {
			return err
		}
		var err error
		replacementID, err = localNativeID(destination)
		return err
	}
	t.Cleanup(func() { checkpointRestoreBeforeActivation = nil })
	got := checkpointRequest(t, destination, "checkpoint.restore", map[string]any{"packagePath": closed})
	if got.OK {
		t.Fatalf("REPLACED_DESTINATION_ACCEPTED: %+v", got)
	}
	if !entered {
		t.Fatalf("activation seam not reached: %+v", got)
	}
	if replacementID == "" {
		// A held native directory may refuse the fixture's rename on Windows.
		// That is a refusal before replacement, never evidence of replacement cleanup.
		if entries, err := os.ReadDir(destination); err != nil || len(entries) != 0 {
			t.Fatalf("held original destination changed: %v %v", entries, err)
		}
		if current, err := localNativeID(destination); err != nil || current != originalID {
			t.Fatalf("failed replacement did not preserve original identity: %q %v", current, err)
		}
		t.Log("NATIVE_REPLACEMENT_BLOCKED; no claim that replacement cleanup was exercised")
		return
	}
	if current, err := localNativeID(destination); err != nil || current != replacementID {
		t.Fatalf("foreign replacement identity changed: %q %v", current, err)
	}
	if entries, err := os.ReadDir(destination); err != nil || len(entries) != 0 {
		t.Fatalf("foreign replacement contents changed: %v %v", entries, err)
	}
}

func TestCheckpointOwnershipGitFinishPreservesReplacement(t *testing.T) {
	op, err := newCheckpointGitOperation(t.TempDir(), "owner", "fictional recovery", "owner review", "inspect")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(op.journalRoot, op.journalRoot+"-retained-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(op.journalRoot, 0700); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(op.journalRoot, "foreign.txt")
	if err := os.WriteFile(foreign, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	out := good("CHECKPOINT_GIT_INSPECTED", 0, map[string]any{})
	op.finish(&out)
	if data, err := os.ReadFile(foreign); err != nil || string(data) != "preserve" {
		t.Fatalf("FOREIGN_GIT_OPERATION_REMOVED: %q %v; response=%+v", data, err, out)
	}
	if out.OK || out.Data["ownedStage"] == nil || out.Data["nextAction"] == nil {
		t.Fatalf("uncertain Git operation has no actionable refusal: %+v", out)
	}
}

func TestCheckpointOwnershipGitPersistsOriginalIdentityBeforeHelper(t *testing.T) {
	op, err := newCheckpointGitOperation(t.TempDir(), "owner", "fictional recovery", "owner review", "inspect")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(op.cancel)
	id, err := localNativeID(op.journalRoot)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(op.journalRoot)
	if err != nil {
		t.Fatal(err)
	}
	var contains func(any) bool
	contains = func(v any) bool {
		switch x := v.(type) {
		case string:
			return x == id
		case []any:
			for _, child := range x {
				if contains(child) {
					return true
				}
			}
		case map[string]any:
			for _, child := range x {
				if contains(child) {
					return true
				}
			}
		}
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(op.journalRoot, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var value any
		if json.Unmarshal(raw, &value) == nil && contains(value) {
			return
		}
	}
	t.Fatal("ORIGINAL_ID_NOT_PERSISTED: initial private journal lacks original directory identity before helper/payload effects")
}

// Qualify the already-shipped primitive's actual composition before relying on
// nested checkpoint staging and checked existing-empty reservation.
func TestCheckpointOwnershipNestedPublishControl(t *testing.T) {
	parent := t.TempDir()
	for _, name := range []string{"operation", "operation/artifacts", "operation/artifacts/payload", "destination"} {
		if err := os.Mkdir(filepath.Join(parent, filepath.FromSlash(name)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	parentID, err := localNativeID(parent)
	if err != nil {
		t.Fatal(err)
	}
	emptyID, err := localNativeID(filepath.Join(parent, "destination"))
	if err != nil {
		t.Fatal(err)
	}
	payloadID, err := localNativeID(filepath.Join(parent, "operation", "artifacts", "payload"))
	if err != nil {
		t.Fatal(err)
	}
	nestedSource := filepath.Join("operation", "artifacts", "payload")
	if err := unpackPublish(root, nestedSource, "destination", parentID, payloadID); err == nil {
		t.Fatal("occupied destination was replaced")
	}
	if current, err := localNativeID(filepath.Join(parent, "destination")); err != nil || current != emptyID {
		t.Fatalf("occupied original changed: %q %v", current, err)
	}
	if err := unpackPublish(root, "destination", "original-empty-backup", parentID, emptyID); err != nil {
		t.Fatalf("native original empty reservation: %v", err)
	}
	if err := unpackPublish(root, nestedSource, "destination", parentID, payloadID); err != nil {
		t.Fatalf("nested original publication: %v", err)
	}
	if current, err := localNativeID(filepath.Join(parent, "destination")); err != nil || current != payloadID {
		t.Fatalf("published original identity changed: %q %v", current, err)
	}
	if err := unpackPublish(root, "original-empty-backup", "destination", parentID, emptyID); err == nil {
		t.Fatal("rollback overwrote an occupied destination")
	}
	if current, err := localNativeID(filepath.Join(parent, "original-empty-backup")); err != nil || current != emptyID {
		t.Fatalf("refused rollback lost original backup: %q %v", current, err)
	}
	if current, err := localNativeID(filepath.Join(parent, "destination")); err != nil || current != payloadID {
		t.Fatalf("refused rollback changed arrival: %q %v", current, err)
	}
}

func TestCheckpointOwnershipUnissuedJournalPreserved(t *testing.T) {
	for _, name := range []string{"journal-unissued.json", "journal-9999.json", "journal-9999.json.tmp"} {
		t.Run(name, func(t *testing.T) {
			op, err := newCheckpointGitOperation(t.TempDir(), "owner", "fictional recovery", "owner review", "inspect")
			if err != nil {
				t.Fatal(err)
			}
			foreign := filepath.Join(op.journalRoot, name)
			if err := os.WriteFile(foreign, []byte("unissued foreign evidence"), 0600); err != nil {
				t.Fatal(err)
			}
			out := good("CHECKPOINT_GIT_INSPECTED", 0, map[string]any{})
			op.finish(&out)
			if data, err := os.ReadFile(foreign); err != nil || string(data) != "unissued foreign evidence" {
				t.Fatalf("FOREIGN_JOURNAL_REMOVED: %s %q %v; response=%+v", name, data, err, out)
			}
			if out.OK || out.Data["ownedStage"] == nil || out.Data["nextAction"] == nil {
				t.Fatalf("unissued journal has no actionable refusal: %+v", out)
			}
		})
	}
}

func TestCheckpointOwnershipUnissuedCoreInputPreserved(t *testing.T) {
	op, err := newCheckpointGitOperation(t.TempDir(), "owner", "fictional recovery", "owner review", "capture")
	if err != nil {
		t.Fatal(err)
	}
	g, err := op.newGroup(op.artifactRoot, "capture-", "git-capture", nil)
	if err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(g.Path, ".git-input-unissued")
	if err := os.WriteFile(foreign, []byte("unissued foreign input"), 0600); err != nil {
		t.Fatal(err)
	}
	out := good("CHECKPOINT_GIT_CAPTURED", 0, map[string]any{})
	op.finish(&out)
	if data, err := os.ReadFile(foreign); err != nil || string(data) != "unissued foreign input" {
		t.Fatalf("FOREIGN_CORE_INPUT_REMOVED: %q %v; response=%+v", data, err, out)
	}
	if out.OK || out.Data["ownedStage"] == nil || out.Data["nextAction"] == nil {
		t.Fatalf("unissued core input has no actionable refusal: %+v", out)
	}
}

func TestCheckpointOwnershipJournalTargetCollisionPreserved(t *testing.T) {
	op, err := newCheckpointGitOperation(t.TempDir(), "owner", "fictional recovery", "owner review", "inspect")
	if err != nil {
		t.Fatal(err)
	}
	defer op.cancel()
	foreign := filepath.Join(op.journalRoot, "journal-"+strconv.Itoa(op.sequence+1)+".json")
	if err := os.WriteFile(foreign, []byte("foreign next phase"), 0600); err != nil {
		t.Fatal(err)
	}
	err = op.record("next-phase", 0, 0, "")
	if data, readErr := os.ReadFile(foreign); readErr != nil || string(data) != "foreign next phase" {
		t.Fatalf("FOREIGN_JOURNAL_TARGET_REPLACED: %q %v; writeErr=%v", data, readErr, err)
	}
	if err == nil {
		t.Fatal("pre-existing unissued journal target admitted a phase")
	}
}
