package internal

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// checkpointGitOperation is deliberately per request. Its journal is a local
// owned artifact beside staging, never an entry in the closed checkpoint.
type checkpointGitOperation struct {
	ctx                               context.Context
	cancel                            context.CancelFunc
	journalRoot, artifactRoot         string
	owner, purpose, retention, action string
	mu                                sync.Mutex
	sequence                          int
	lastPID, lastGroup                int
	unknown                           bool
	launched, stopped                 bool
	sourceIDs                         map[string]string
}

func newCheckpointGitOperation(parent, owner, purpose, retention, action string) (*checkpointGitOperation, error) {
	root, err := os.MkdirTemp(parent, ".ocmi-git-operation-")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	o := &checkpointGitOperation{ctx: ctx, cancel: cancel, journalRoot: root, artifactRoot: filepath.Join(root, "artifacts"), owner: owner, purpose: purpose, retention: retention, action: action}
	if err = o.record("intent", 0, 0, ""); err != nil {
		return o, err
	}
	if err = os.Mkdir(o.artifactRoot, 0700); err != nil {
		return o, err
	}
	return o, nil
}

func (o *checkpointGitOperation) record(state string, pid, group int, result string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sequence++
	if pid > 0 {
		o.lastPID, o.lastGroup = pid, group
	}
	raw, err := json.Marshal(map[string]any{"owner": o.owner, "purpose": o.purpose, "retention": o.retention, "action": o.action, "sequence": o.sequence, "state": state, "pid": o.lastPID, "group": o.lastGroup, "result": result})
	if err != nil {
		return err
	}
	target := filepath.Join(o.journalRoot, "journal-"+strconv.Itoa(o.sequence)+".json")
	temp := target + ".tmp"
	f, err := os.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(temp)
		return err
	}
	if err = os.Rename(temp, target); err != nil {
		return err
	}
	return nil
}

// run admits one owned Git child. It delegates all lifecycle mechanics to the
// existing runner; an unresolved/lost-stop result freezes this operation.
func (o *checkpointGitOperation) run(cwd string, input *os.File, spec commandSpec) (commandResult, error) {
	o.mu.Lock()
	blocked := o.unknown
	o.mu.Unlock()
	if blocked {
		return commandResult{Unresolved: true}, errors.New("git operation unresolved")
	}
	if err := o.ctx.Err(); err != nil {
		o.freeze("operation context expired")
		return commandResult{Unresolved: true}, err
	}
	child, cancel := context.WithTimeout(o.ctx, 30*time.Second)
	defer cancel()
	if err := o.record("beforelaunch", 0, 0, ""); err != nil {
		o.freeze("beforelaunch journal failed")
		return commandResult{Unresolved: true}, err
	}
	prior := spec.Registered
	spec.Cwd, spec.Stdin = cwd, input
	spec.Registered = func(pid, group int) error {
		if err := o.record("registered", pid, group, ""); err != nil {
			o.freeze("registered journal failed")
			return err
		}
		if prior != nil {
			return prior(pid, group)
		}
		return nil
	}
	var result commandResult
	var err error
	o.mu.Lock()
	o.launched = true
	o.mu.Unlock()
	if checkpointGitRunOverride != nil {
		result, err = checkpointGitRunOverride(child, spec)
	} else {
		result, err = runOwned(child, spec)
	}
	o.mu.Lock()
	journalUnknown := o.unknown
	o.mu.Unlock()
	if journalUnknown || result.Unresolved || !result.StoppedVerified {
		o.freeze("lost stop evidence")
		return result, err
	}
	o.mu.Lock()
	o.stopped = true
	o.mu.Unlock()
	resultState := "exit=" + strconv.Itoa(result.ExitCode) + ",truncated=" + strconv.FormatBool(result.Truncated) + ",timedOut=" + strconv.FormatBool(result.TimedOut)
	if recordErr := o.record("stopped", 0, 0, resultState); recordErr != nil {
		o.freeze("stopped journal failed")
		return result, recordErr
	}
	return result, err
}

func (o *checkpointGitOperation) freeze(reason string) {
	o.mu.Lock()
	o.unknown = true
	o.mu.Unlock()
	_ = o.record("unknown", 0, 0, reason)
}

// finish owns only this operation root.  Worker 2 calls it from the action
// defer once artifact/stage ownership is wired; it never cleans uncertainty.
func (o *checkpointGitOperation) finish(out *Response) {
	o.cancel()
	o.mu.Lock()
	unknown := o.unknown || (o.launched && !o.stopped)
	o.mu.Unlock()
	if unknown {
		if out != nil {
			primary := out.Code
			*out = bad("CHECKPOINT_UNRESOLVED", out.Revision)
			out.Data = map[string]any{"primaryCode": primary, "ownedStage": o.journalRoot, "nextAction": "inspect retained Git helper evidence"}
		}
		return
	}
	if out == nil {
		return
	}
	if err := os.RemoveAll(o.journalRoot); err != nil {
		_ = o.record("cleanup_required", 0, 0, "remove owned operation root")
		primary := out.Code
		*out = bad("CHECKPOINT_CLEANUP_REQUIRED", out.Revision)
		out.Data = map[string]any{"primaryCode": primary, "ownedStage": o.journalRoot, "nextAction": "remove only the retained owned Git operation after inspection"}
	}
}

func (o *checkpointGitOperation) close(remove bool) error {
	o.cancel()
	o.mu.Lock()
	unknown := o.unknown
	o.mu.Unlock()
	if !remove || unknown {
		return nil
	}
	if err := os.RemoveAll(o.journalRoot); err != nil {
		_ = o.record("cleanup_required", 0, 0, "remove owned journal")
		return err
	}
	return nil
}
