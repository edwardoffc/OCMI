package internal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func workerBatchFactory(t *testing.T, root, source string, image []byte, extra int) {
	t.Helper()
	acceptanceUpdateFactory(t, root, source)
	binary := "system/bin/ocmi"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if len(image) > 0 {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(binary)), image, 0700); err != nil {
			t.Fatal(err)
		}
	}
	sumPath := filepath.Join(root, "SHA256SUMS")
	raw, err := os.ReadFile(sumPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(image) > 0 {
		for i, line := range lines {
			if strings.HasSuffix(line, "  "+binary) {
				lines[i] = fmt.Sprintf("%s  %s", updateDigest(image), binary)
			}
		}
	}
	dir := filepath.Join(root, "system", "batch-fixture")
	if err = os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < extra; i++ {
		name := fmt.Sprintf("%04d.dat", i)
		data := []byte(fmt.Sprintf("fictional batch member %04d\n", i))
		if err = os.WriteFile(filepath.Join(dir, name), data, 0700); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, fmt.Sprintf("%s  system/batch-fixture/%s", updateDigest(data), name))
	}
	if err = os.WriteFile(sumPath, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerUpdateStageBatch(t *testing.T) {
	for _, scenario := range []string{"recover", "resume-copy", "changed", "extra"} {
		t.Run(scenario, func(t *testing.T) {
			base := t.TempDir()
			home := filepath.Join(base, "home")
			candidate := filepath.Join(base, "candidate")
			workerBatchFactory(t, home, strings.Repeat("a", 40), nil, 90)
			workerBatchFactory(t, candidate, strings.Repeat("b", 40), nil, 90)
			if r := InstallHome(home, false); !r.OK {
				t.Fatal(r)
			}
			ctx, info, err := verifyFactoryHome(home)
			if err != nil {
				t.Fatal(err)
			}
			before := acceptancePublicSnapshot(t, home)
			meta, err := captureUpdateMetadata(ctx, info, strings.Repeat("b", 40), candidate)
			if err != nil {
				t.Fatal(err)
			}
			old, err := captureUpdatePublicProof(home, true)
			if err != nil {
				t.Fatal(err)
			}
			next, err := captureUpdatePublicProof(candidate, false)
			if err != nil {
				t.Fatal(err)
			}
			lock, err := setupLockOwned(filepath.Join(home, "local", "bindings"))
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Release()
			id := uuid.NewString()
			j := updateProofJournal{SchemaVersion: 1, ID: id, Phase: "intent", Metadata: meta, OldSource: meta.OldSource, NewSource: meta.NewSource, OldPublic: old, NewPublic: next, Stage: "local/runs/update-" + id, OldSlot: "old", NewSlot: "new", AdmissionID: lock.nativeID}
			if err = createUpdateStage(ctx, &j, home, candidate); err != nil {
				t.Fatal(err)
			}
			writer, err := acquireUpdateRecoveryWriter(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Release()
			updateStageBeforeCheckpointTest = func(side string, n int) {
				if side == "old" && n == 64 {
					panic("batch-interrupted")
				}
			}
			defer func() { updateStageBeforeCheckpointTest = nil }()
			interrupted := false
			func() {
				defer func() {
					if v := recover(); v != nil {
						if v != "batch-interrupted" {
							panic(v)
						}
						interrupted = true
					}
				}()
				if err = copyUpdateStage(ctx, &j, home, candidate); err != nil {
					t.Fatal(err)
				}
			}()
			updateStageBeforeCheckpointTest = nil
			if !interrupted {
				t.Fatal("copy did not reach interruption")
			}
			disk, err := readUpdateProofJournal(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if disk.OldCopied != 32 || disk.NewCopied != 0 {
				t.Fatalf("unexpected durable prefix: %d/%d", disk.OldCopied, disk.NewCopied)
			}
			stage := filepath.Join(home, filepath.FromSlash(disk.Stage))
			count := 0
			if err = filepath.WalkDir(filepath.Join(stage, "old"), func(_ string, e os.DirEntry, err error) error {
				if err == nil && !e.IsDir() {
					count++
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
			if count != 64 {
				t.Fatalf("expected 64 synced files, got %d", count)
			}
			if err = verifyUpdateStage(ctx, disk, true); err != nil {
				t.Fatal(err)
			}
			if scenario == "resume-copy" {
				if err = copyUpdateStage(ctx, &disk, home, candidate); err != nil {
					t.Fatal(err)
				}
				if disk.Phase != "staged" {
					t.Fatal("resume did not finish stage")
				}
			}
			if scenario == "changed" {
				path := filepath.Join(stage, "old", filepath.FromSlash(disk.OldPublic.Files[36].Path))
				if err = os.WriteFile(path, []byte("foreign"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "extra" {
				if err = os.WriteFile(filepath.Join(stage, "old", "foreign"), []byte("foreign"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			journalPath := ctx.Paths["bindings"] + "/update.json"
			journalBefore, err := installationRead(home, journalPath, 16<<20)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "changed" || scenario == "extra" {
				stageBefore := workerUpdateTreeSnapshot(t, stage)
				if err = recoverUpdatePublic(ctx, &disk, writer, lock); err == nil {
					t.Fatal("accepted altered stage")
				}
				after, err := installationRead(home, journalPath, 16<<20)
				if err != nil || !bytes.Equal(journalBefore, after) {
					t.Fatal("refusal changed journal")
				}
				if !reflect.DeepEqual(stageBefore, workerUpdateTreeSnapshot(t, stage)) {
					t.Fatal("refusal changed stage")
				}
				return
			}
			if err = recoverUpdatePublic(ctx, &disk, writer, lock); err != nil {
				t.Fatal(err)
			}
			if err = cleanupUpdateStage(ctx, &disk, writer, lock); err != nil {
				t.Fatal(err)
			}
			if disk.CleanupState != "done" || !updatePathAbsent(stage) {
				t.Fatal("partial stage not cleaned")
			}
			after := acceptancePublicSnapshot(t, home)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("recovery changed public payload")
			}
		})
	}
}
