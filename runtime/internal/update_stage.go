package internal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Test-only observer of a successfully copied member; no CLI or environment
// switch can alter production behaviour.
var updateStageTestStep func(string, int)
var updateStageBeforeCheckpointTest func(string, int)

const updateStageBatchFiles = 32
const updateStageBatchBytes = 8 << 20
const updateStageSyncWorkers = 8

// verifyUpdateProofInputs binds the semantic claims to bytes in the captured
// public inventories, before any stage or journal is created.
func verifyUpdateProofInputs(j updateProofJournal, oldRoot, newRoot string) error {
	for _, side := range []struct {
		root, source, template string
		proof                  updatePublicProof
		private                bool
	}{{oldRoot, j.OldSource, j.Metadata.OldTemplate, j.OldPublic, true}, {newRoot, j.NewSource, j.Metadata.NewTemplate, j.NewPublic, false}} {
		if err := verifyUpdatePublicProof(side.root, side.proof, side.private, true); err != nil {
			return err
		}
		raw, err := installationRead(side.root, "system/build-info.json", 1<<20)
		if err != nil {
			return err
		}
		var info struct {
			SourceCommit string `json:"sourceCommit"`
		}
		if json.Unmarshal(raw, &info) != nil || info.SourceCommit != side.source {
			return errors.New("source proof mismatch")
		}
		if side.template != "" {
			var binding coordinatorBinding
			if !checkpointStrictJSON(j.Metadata.BindingBefore, &binding) {
				return errors.New("binding proof")
			}
			text, err := readHomeText(side.root, binding.Language)
			if err != nil || text.Startup != side.template {
				return errors.New("template proof mismatch")
			}
		}
	}
	return nil
}

func persistUpdateProofJournal(context layoutContext, j updateProofJournal) error {
	raw, err := encodeUpdateProofJournal(j)
	if err != nil {
		return err
	}
	return installationWrite(context.Root, context.Paths["bindings"]+"/update.json", raw)
}

// createUpdateStage records each acquired native identity before the next
// operation. Unrecorded objects after an interruption are never adopted.
func createUpdateStage(context layoutContext, j *updateProofJournal, oldRoot, newRoot string) error {
	if j == nil || j.Phase != "intent" || j.StageID != "" || j.OldSlotID != "" || j.NewSlotID != "" {
		return errors.New("invalid stage intent")
	}
	if _, err := encodeUpdateProofJournal(*j); err != nil {
		return err
	}
	if err := verifyUpdateMetadataBefore(context, j.Metadata); err != nil {
		return err
	}
	lockPath := filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"]), ".setup-lock")
	if environmentPlainAncestors(lockPath) != nil || plainPath(lockPath, true) != nil {
		return errors.New("unsafe admission")
	}
	lockID, err := localNativeID(lockPath)
	if err != nil || lockID != j.AdmissionID {
		return errors.New("admission identity")
	}
	if err := verifyUpdateProofInputs(*j, oldRoot, newRoot); err != nil {
		return err
	}
	if err := persistUpdateProofJournal(context, *j); err != nil {
		return err
	}
	stage := filepath.Join(context.Root, filepath.FromSlash(j.Stage))
	held, err := os.OpenRoot(context.Root)
	if err != nil {
		return err
	}
	err = installationEnsureParent(held, context.Paths["runs"])
	_ = held.Close()
	if err != nil || plainComponents(context.Root, filepath.FromSlash(context.Paths["runs"]), false) != nil {
		return errors.New("unsafe runs")
	}
	if err := os.Mkdir(stage, 0700); err != nil {
		return err
	}
	j.StageID, err = localNativeID(stage)
	if err != nil {
		return err
	}
	if err = persistUpdateProofJournal(context, *j); err != nil {
		return err
	}
	for _, slot := range []struct {
		name string
		id   *string
	}{{"old", &j.OldSlotID}, {"new", &j.NewSlotID}} {
		path := filepath.Join(stage, slot.name)
		if err = os.Mkdir(path, 0700); err != nil {
			return err
		}
		*slot.id, err = localNativeID(path)
		if err != nil {
			return err
		}
		if err = persistUpdateProofJournal(context, *j); err != nil {
			return err
		}
	}
	return nil
}

type updateStageIO struct {
	read  func(string, string, int) ([]byte, error)
	write func(*os.File, []byte) (int, error)
	sync  func(*os.File) error
}

type updateStageMember struct {
	entry    updatePublicEntry
	index    int
	nativeID string
	file     *os.File
}

// syncUpdateStageMembers drains every handle before returning, even if a sync
// fails or panics. At most updateStageSyncWorkers Sync calls run concurrently.
func syncUpdateStageMembers(members []*updateStageMember, syncFile func(*os.File) error) error {
	sem := make(chan struct{}, updateStageSyncWorkers)
	errs := make([]error, len(members))
	var wg sync.WaitGroup
	for i, m := range members {
		if m.file == nil {
			continue
		}
		wg.Add(1)
		go func(i int, f *os.File) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() {
				if recover() != nil {
					errs[i] = errors.New("stage sync panic")
				}
				if err := f.Close(); errs[i] == nil && err != nil {
					errs[i] = err
				}
				<-sem
			}()
			errs[i] = syncFile(f)
		}(i, m.file)
	}
	wg.Wait()
	for _, m := range members {
		m.file = nil
	}
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func copyUpdateStage(context layoutContext, j *updateProofJournal, oldRoot, newRoot string) error {
	return copyUpdateStageWithIO(context, j, oldRoot, newRoot, updateStageIO{read: installationRead, write: (*os.File).Write, sync: (*os.File).Sync})
}

func copyUpdateStageWithIO(context layoutContext, j *updateProofJournal, oldRoot, newRoot string, ioOps updateStageIO) error {
	if j == nil || j.Phase != "intent" {
		return errors.New("invalid stage phase")
	}
	if ioOps.read == nil || ioOps.write == nil || ioOps.sync == nil {
		return errors.New("invalid stage IO")
	}
	if err := verifyUpdateProofInputs(*j, oldRoot, newRoot); err != nil {
		return err
	}
	if err := verifyUpdateStage(context, *j, true); err != nil {
		return err
	}
	for _, side := range []struct {
		root, slot string
		proof      updatePublicProof
		progress   *int
	}{{oldRoot, "old", j.OldPublic, &j.OldCopied}, {newRoot, "new", j.NewPublic, &j.NewCopied}} {
		var pending []*updateStageMember
		var batchBytes int64
		// Also drain on an error or panic while a later member is prepared.
		defer func() { _ = syncUpdateStageMembers(pending, ioOps.sync) }()
		destRoot := filepath.Join(context.Root, filepath.FromSlash(j.Stage), side.slot)
		flush := func() error {
			if len(pending) == 0 {
				return nil
			}
			if err := syncUpdateStageMembers(pending, ioOps.sync); err != nil {
				return err
			}
			for _, member := range pending {
				entry := member.entry
				written, err := ioOps.read(destRoot, entry.Path, 32<<20)
				if err != nil || int64(len(written)) != entry.Size || updateDigest(written) != entry.Digest {
					return errors.New("staged file changed")
				}
				path := filepath.Join(destRoot, filepath.FromSlash(entry.Path))
				info, err := os.Lstat(path)
				if err != nil || uint32(info.Mode().Perm()) != entry.Mode {
					return errors.New("staged mode changed")
				}
				id, err := localNativeID(path)
				if err != nil || id != member.nativeID {
					return errors.New("staged identity changed")
				}
			}
			last := pending[len(pending)-1].index + 1
			if updateStageBeforeCheckpointTest != nil {
				updateStageBeforeCheckpointTest(side.slot, last)
			}
			*side.progress = last
			if err := persistUpdateProofJournal(context, *j); err != nil {
				return err
			}
			for _, member := range pending {
				if updateStageTestStep != nil {
					updateStageTestStep(side.slot, member.index+1)
				}
			}
			pending = nil
			batchBytes = 0
			return nil
		}
		for i, entry := range side.proof.Files {
			if i < *side.progress {
				continue
			}
			if len(pending) > 0 && (len(pending) >= updateStageBatchFiles || batchBytes+entry.Size > updateStageBatchBytes) {
				if err := flush(); err != nil {
					return err
				}
			}
			// A file can be durable before its journal count. The preceding
			// subset validation checked its bytes and mode against the proof.
			if _, err := os.Lstat(filepath.Join(destRoot, filepath.FromSlash(entry.Path))); err == nil {
				err = func() error {
					id, err := localNativeID(filepath.Join(destRoot, filepath.FromSlash(entry.Path)))
					if err != nil {
						return err
					}
					held, err := os.OpenRoot(destRoot)
					if err != nil {
						return err
					}
					defer held.Close()
					f, err := held.OpenFile(filepath.FromSlash(entry.Path), os.O_WRONLY, 0)
					if err != nil {
						return err
					}
					member := &updateStageMember{entry: entry, index: i, nativeID: id, file: f}
					pending = append(pending, member)
					openedID, err := localNativeIDFile(f)
					if err != nil || openedID != id {
						return errors.New("staged identity changed")
					}
					return nil
				}()
				if err != nil {
					return err
				}
				batchBytes += entry.Size
				if len(pending) >= updateStageBatchFiles || batchBytes >= updateStageBatchBytes {
					if err = flush(); err != nil {
						return err
					}
				}
				continue
			} else if !os.IsNotExist(err) {
				return err
			}
			src := filepath.Join(side.root, filepath.FromSlash(entry.Path))
			id, err := localNativeID(src)
			if err != nil || id != entry.NativeID {
				return errors.New("source identity changed")
			}
			raw, err := ioOps.read(side.root, entry.Path, 32<<20)
			if err != nil || int64(len(raw)) != entry.Size || updateDigest(raw) != entry.Digest {
				return errors.New("source changed")
			}
			sourceID, err := localNativeID(src)
			if err != nil || sourceID != entry.NativeID {
				return errors.New("source identity changed")
			}
			info, err := os.Lstat(src)
			if err != nil || uint32(info.Mode().Perm()) != entry.Mode {
				return errors.New("source mode changed")
			}
			err = func() error {
				held, err := os.OpenRoot(destRoot)
				if err != nil {
					return err
				}
				defer held.Close()
				parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(entry.Path)))
				if err = installationEnsureParent(held, parent); err != nil {
					return err
				}
				f, err := held.OpenFile(filepath.FromSlash(entry.Path), os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(entry.Mode))
				if err != nil {
					return err
				}
				member := &updateStageMember{entry: entry, index: i, file: f}
				pending = append(pending, member)
				member.nativeID, err = localNativeIDFile(f)
				if err != nil {
					return err
				}
				n, err := ioOps.write(f, raw)
				if err != nil {
					return err
				}
				if n != len(raw) {
					return errors.New("short stage write")
				}
				return f.Chmod(os.FileMode(entry.Mode))
			}()
			if err != nil {
				return err
			}
			batchBytes += entry.Size
			if len(pending) >= updateStageBatchFiles || batchBytes >= updateStageBatchBytes {
				if err = flush(); err != nil {
					return err
				}
			}
		}
		if err := flush(); err != nil {
			return err
		}
	}
	if err := verifyUpdateStage(context, *j, false); err != nil {
		return err
	}
	stage := filepath.Join(context.Root, filepath.FromSlash(j.Stage))
	oldProof, err := scanUpdatePublic(filepath.Join(stage, "old"), false)
	if err != nil {
		return err
	}
	newProof, err := scanUpdatePublic(filepath.Join(stage, "new"), false)
	if err != nil {
		return err
	}
	j.OldStaged, j.NewStaged = &oldProof, &newProof
	j.Phase = "staged"
	return persistUpdateProofJournal(context, *j)
}

// Partial validation accepts only an exact subset of captured members, including
// a valid file written immediately before its progress record became durable.
func verifyUpdateStage(context layoutContext, j updateProofJournal, partial bool) error {
	if _, err := encodeUpdateProofJournal(j); err != nil {
		return err
	}
	stage := filepath.Join(context.Root, filepath.FromSlash(j.Stage))
	if environmentPlainAncestors(stage) != nil {
		return errors.New("unsafe stage")
	}
	id, err := localNativeID(stage)
	if err != nil || id != j.StageID {
		return errors.New("stage identity")
	}
	entries, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	if len(entries) != 2 {
		return errors.New("stage inventory")
	}
	for _, side := range []struct {
		name, id string
		proof    updatePublicProof
		progress int
	}{{"old", j.OldSlotID, j.OldPublic, j.OldCopied}, {"new", j.NewSlotID, j.NewPublic, j.NewCopied}} {
		root := filepath.Join(stage, side.name)
		if plainPath(root, true) != nil || environmentPlainAncestors(root) != nil {
			return errors.New("unsafe slot")
		}
		id, err := localNativeID(root)
		if err != nil || id != side.id {
			return errors.New("slot identity")
		}
		want := map[string]updatePublicEntry{}
		dirs := map[string]bool{}
		for _, e := range side.proof.Files {
			want[e.Path] = e
			p := filepath.ToSlash(filepath.Dir(filepath.FromSlash(e.Path)))
			for p != "." {
				dirs[p] = true
				p = filepath.ToSlash(filepath.Dir(filepath.FromSlash(p)))
			}
		}
		seen := map[string]bool{}
		used := map[string]bool{}
		err = filepath.WalkDir(root, func(path string, e os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if rel == "." {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if e.Type()&os.ModeSymlink != 0 || e.Type()&os.ModeIrregular != 0 || plainComponents(root, filepath.FromSlash(rel), false) != nil {
				return errors.New("stage link")
			}
			if e.IsDir() {
				if !dirs[rel] {
					return errors.New("extra directory")
				}
				return nil
			}
			w, ok := want[rel]
			if !ok || !e.Type().IsRegular() {
				return errors.New("extra file")
			}
			raw, err := installationRead(root, rel, 32<<20)
			if err != nil || int64(len(raw)) != w.Size || updateDigest(raw) != w.Digest {
				return errors.New("stage content")
			}
			info, err := os.Lstat(path)
			if err != nil || uint32(info.Mode().Perm()) != w.Mode {
				return errors.New("stage mode")
			}
			seen[rel] = true
			p := filepath.ToSlash(filepath.Dir(filepath.FromSlash(rel)))
			for p != "." {
				used[p] = true
				p = filepath.ToSlash(filepath.Dir(filepath.FromSlash(p)))
			}
			return nil
		})
		if err != nil {
			return err
		}
		for i, e := range side.proof.Files {
			if i < side.progress && !seen[e.Path] {
				return errors.New("missing recorded member")
			}
			if !partial && !seen[e.Path] {
				return errors.New("missing member")
			}
		}
		for d := range dirs {
			if !used[d] {
				if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(d))); err == nil {
					return errors.New("empty directory")
				} else if !os.IsNotExist(err) {
					return err
				}
			}
		}
	}
	for _, e := range entries {
		if e.Name() != "old" && e.Name() != "new" || !e.IsDir() || strings.Contains(e.Name(), "/") {
			return errors.New("stage member")
		}
	}
	return nil
}
