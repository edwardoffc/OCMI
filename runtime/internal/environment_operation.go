package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type environmentOperation struct {
	ctx           context.Context
	cancel        context.CancelFunc
	root          string
	mu            sync.Mutex
	sequence      int
	rootID        string
	project, tool string
	group         int
}

var environmentRunOverride func(context.Context, commandSpec) (commandResult, error)
var environmentRecordOverride func(string) error
var errEnvironmentUnresolved = errors.New("environment unresolved")

func newEnvironmentOperation(scratch string, project ...string) (*environmentOperation, error) {
	root, e := os.MkdirTemp(scratch, ".ocmi-environment-")
	if e != nil {
		return nil, e
	}
	c, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	id, e := localNativeID(root)
	if e != nil {
		cancel()
		return &environmentOperation{ctx: c, cancel: cancel, root: root}, e
	}
	o := &environmentOperation{ctx: c, cancel: cancel, root: root, rootID: id}
	if len(project) == 1 {
		o.project = project[0]
	}
	if o.record("intent", 0) != nil {
		cancel()
		return o, os.ErrInvalid
	}
	return o, nil
}
func (o *environmentOperation) record(state string, pid int) error {
	info, err := os.Lstat(o.root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || checkpointGitReparse(o.root) || environmentPlainAncestors(o.root) != nil {
		return errEnvironmentUnresolved
	}
	id, err := localNativeID(o.root)
	if err != nil || id != o.rootID {
		return errEnvironmentUnresolved
	}
	if environmentRecordOverride != nil {
		if e := environmentRecordOverride(state); e != nil {
			return e
		}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sequence++
	b, err := json.Marshal(map[string]any{"owner": "ocmi-environment", "purpose": "environment inspection", "retention": "through recovery review", "state": state, "pid": pid, "group": o.group, "tool": o.tool, "projectRoot": o.project, "sequence": o.sequence, "operation": filepath.Base(o.root)})
	if err != nil {
		return err
	}
	name := filepath.Join(o.root, fmt.Sprintf("journal-%03d.json", o.sequence))
	tmp := name + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, name)
}
func (o *environmentOperation) finish(state string) error {
	if o.ctx.Err() != nil {
		return errEnvironmentUnresolved
	}
	if id, e := localNativeID(o.root); e != nil || id != o.rootID {
		return errEnvironmentUnresolved
	}
	return o.record(state, 0)
}
func (o *environmentOperation) run(path, kind string) (string, error) {
	o.tool = kind
	id, e := localNativeID(o.root)
	if o.ctx.Err() != nil || e != nil || id != o.rootID {
		return "", errEnvironmentUnresolved
	}
	args := []string{"--version"}
	if kind == "go" {
		args = []string{"version"}
	}
	if kind == "python" {
		args = []string{"-I", "-S", "--version"}
	}
	c, cancel := context.WithTimeout(o.ctx, 5*time.Second)
	defer cancel()
	if o.record("beforelaunch", 0) != nil {
		return "", errEnvironmentUnresolved
	}
	spec := commandSpec{Executable: path, Args: args, Cwd: o.root, OutputLimit: 8192, Env: effectiveEnv([]string{"PATH=" + filepath.Dir(path), "HOME=" + o.root, "TMP=" + o.root, "TEMP=" + o.root, "TMPDIR=" + o.root, "USERPROFILE=" + o.root, "APPDATA=" + o.root, "XDG_CONFIG_HOME=" + o.root, "GOTOOLCHAIN=local", "GOENV=off", "GOWORK=off", "GOPROXY=off", "NODE_OPTIONS=", "PYTHONPATH="}), Registered: func(pid, group int) error { o.group = group; return o.record("registered", pid) }}
	var r commandResult
	if environmentRunOverride != nil {
		r, e = environmentRunOverride(c, spec)
	} else {
		r, e = runOwned(c, spec)
	}
	if r.StoppedVerified {
		if o.record("stopped", 0) != nil {
			return "", errEnvironmentUnresolved
		}
	} else {
		_ = o.record("unknown", 0)
	}
	if e != nil || r.ExitCode != 0 || r.TimedOut || r.Truncated || r.Unresolved || !r.StoppedVerified {
		return "", errEnvironmentUnresolved
	}
	v, ok := environmentParseObserved(kind, r.Output)
	if !ok {
		return "", errEnvironmentUnresolved
	}
	return v, nil
}
