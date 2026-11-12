package internal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
)

const toolStorageAuthoringLimit = 64 << 10

type toolStorageAuthoringInput struct {
	ToolID       string                      `json:"toolId"`
	ToolDataRoot string                      `json:"toolDataRoot"`
	Owner        string                      `json:"owner"`
	Purpose      string                      `json:"purpose"`
	Retention    string                      `json:"retention"`
	Executable   string                      `json:"executable"`
	Script       string                      `json:"script"`
	MaxBytes     int64                       `json:"maxBytes"`
	MaxAgeDays   int                         `json:"maxAgeDays"`
	Stores       []toolStorageAuthoringStore `json:"stores"`
	ScratchEnv   []string                    `json:"scratchEnv"`
}
type toolStorageAuthoringStore struct {
	Key  string `json:"key"`
	Path string `json:"path"`
	Kind string `json:"kind"`
}
type toolStoragePlan struct {
	SchemaVersion int    `json:"schemaVersion"`
	PlanID        string `json:"planId"`
	toolStorageAuthoringInput
	RootIdentity       string `json:"rootIdentity"`
	ParentIdentity     string `json:"parentIdentity"`
	ExecutableIdentity string `json:"executableIdentity"`
	ExecutableSHA      string `json:"executableSha256"`
	ScriptIdentity     string `json:"scriptIdentity"`
	ScriptSHA          string `json:"scriptSha256"`
}
type toolStorageApproval struct {
	ToolID           string `json:"toolId"`
	PlanID           string `json:"planId"`
	PlanSHA256       string `json:"planSha256"`
	ConsentReference string `json:"consentReference"`
}
type toolStorageReceipt struct {
	SchemaVersion         int                `json:"schemaVersion"`
	Phase                 string             `json:"phase"`
	PlanID                string             `json:"planId"`
	PlanSHA256            string             `json:"planSha256"`
	ConsentReference      string             `json:"consentReference"`
	Home                  string             `json:"home"`
	HomeIdentity          string             `json:"homeIdentity"`
	InstallationID        string             `json:"installationId"`
	DeviceID              string             `json:"deviceId"`
	CopyID                string             `json:"copyId"`
	ToolID                string             `json:"toolId"`
	ToolDataRoot          string             `json:"toolDataRoot"`
	ToolDataIdentity      string             `json:"toolDataIdentity"`
	ProfileSHA256         string             `json:"profileSha256"`
	ProfileNativeIdentity string             `json:"profileNativeIdentity"`
	ExecutableIdentity    string             `json:"executableIdentity"`
	ExecutableSHA         string             `json:"executableSha256"`
	ScriptIdentity        string             `json:"scriptIdentity"`
	ScriptSHA             string             `json:"scriptSha256"`
	Verification          string             `json:"verification"`
	Stores                []toolStorageStore `json:"stores"`
	UseRootIdentity       string             `json:"useRootIdentity,omitempty"`
}
type toolStorageTransaction struct {
	SchemaVersion int    `json:"schemaVersion"`
	Phase         string `json:"phase"`
	ToolID        string `json:"toolId"`
	PlanID        string `json:"planId"`
	PlanSHA256    string `json:"planSha256"`
	LockID        string `json:"lockId"`
	Stage         string `json:"stage,omitempty"`
	StageID       string `json:"stageId,omitempty"`
	ConsentDigest string `json:"consentDigest,omitempty"`
	ProbeState    string `json:"probeState,omitempty"`
	ProbePID      int    `json:"probePid,omitempty"`
	UseRootID     string `json:"useRootId,omitempty"`
}
type toolStorageProbeFailure string

func (e toolStorageProbeFailure) Error() string { return string(e) }

// Failure observations contain only finite labels, flags and numeric sizes.
// The wrapped cause preserves existing typed stage selection, never its text.
type toolStorageProbeDiagnosticError struct {
	cause       error
	observation map[string]any
}

func (e *toolStorageProbeDiagnosticError) Error() string { return "provider probe failed" }
func (e *toolStorageProbeDiagnosticError) Unwrap() error { return e.cause }

func toolStorageProbeObservation(tool, step string, run commandResult, err error, recordFailed bool) map[string]any {
	if tool != "go" && tool != "npm" {
		tool = "unknown"
	}
	switch step {
	case "stage.create", "stage.record", "config.write", "telemetry", "telemetry.record", "execute", "identity", "inventory", "stopped.record", "response", "cleanup":
	default:
		step = "unknown"
	}
	return map[string]any{"tool": tool, "step": step, "exitCode": run.ExitCode, "errorPresent": err != nil,
		"timedOut": run.TimedOut, "truncated": run.Truncated, "pending": run.Pending,
		"unresolved": run.Unresolved, "stoppedVerified": run.StoppedVerified,
		"outputBytes": len(run.Output), "registrationRecordFailed": recordFailed}
}

func toolStorageAuthoring(r Request) (out Response) {
	defer func() {
		if !out.OK {
			if out.Data == nil {
				out.Data = map[string]any{}
			}
			if _, ok := out.Data["reason"]; !ok {
				out.Data["reason"] = "refused"
			}
			if _, ok := out.Data["stage"]; !ok {
				out.Data["stage"] = "admission"
			}
			if _, ok := out.Data["nextAction"]; !ok {
				out.Data["nextAction"] = "inspect tool storage state"
			}
		}
	}()
	if r.RootKind != "installation" || r.Actor != "coordinator" || r.ExpectedRevision != 0 {
		return toolStorageFailure("authority", "admission", "use the confirmed installation coordinator")
	}
	context, info, err := verifyFactoryHome(r.ProjectRoot)
	if err != nil {
		return bad("TOOL_STORAGE_REFUSED", 0)
	}
	receipt, err := currentInstallation(context, info)
	if err != nil {
		return bad("TOOL_STORAGE_REFUSED", 0)
	}
	binding, err := readCoordinatorBindingContext(context)
	if err != nil || binding.Phase != "confirmed" || !currentCoordinator(context, receipt, binding) {
		return bad("TOOL_STORAGE_REFUSED", 0)
	}
	if toolStorageStopped(context) && r.Action != "tool-storage.inspect" && r.Action != "tool-storage.recover" && r.Action != "tool-storage.maintain.inspect" && r.Action != "tool-storage.relocate.inspect" && r.Action != "tool-storage.relocate.recover" {
		return bad("TOOL_STORAGE_STOPPED", 0)
	}
	switch r.Action {
	case "tool-storage.prepare":
		return toolStoragePrepare(context, receipt, r)
	case "tool-storage.apply", "tool-storage.recover":
		return toolStorageApply(context, receipt, r, r.Action == "tool-storage.recover")
	case "tool-storage.cancel":
		return toolStorageCancel(context, r)
	case "tool-storage.inspect":
		return toolStorageInspect(context, r)
	case "tool-storage.maintain.inspect":
		return toolStorageMaintenanceInspect(context, r)
	case "tool-storage.relocate.prepare", "tool-storage.relocate.apply", "tool-storage.relocate.recover", "tool-storage.relocate.rollback", "tool-storage.relocate.inspect":
		return toolStorageRelocation(context, receipt, r)
	default:
		return bad("UNKNOWN_ACTION", 0)
	}
}
func toolStorageStopped(context layoutContext) bool {
	db, e := openLayout(context)
	if e != nil {
		return true
	}
	defer db.Close()
	var n int
	return db.QueryRow("SELECT stopped FROM project").Scan(&n) != nil || n != 0
}
func toolStorageBase(c layoutContext) string {
	return filepath.Join(c.Root, filepath.FromSlash(c.Paths["bindings"]), "tool-storage")
}
func toolStoragePlanPath(c layoutContext, tool string) string {
	return filepath.Join(toolStorageBase(c), "plans", tool+".json")
}
func toolStorageReceiptPath(c layoutContext, tool string) string {
	return filepath.Join(toolStorageBase(c), "receipts", tool+".json")
}
func toolStorageTransactionPath(c layoutContext) string {
	return filepath.Join(toolStorageBase(c), "transaction.json")
}
func toolStorageRel(c layoutContext, path string) string {
	rel, _ := filepath.Rel(c.Root, path)
	return filepath.ToSlash(rel)
}
func toolStorageExact(raw json.RawMessage, target any, keys ...string) bool {
	return toolStorageExactKeys(raw, keys...) && checkpointStrictJSON(raw, target)
}
func toolStorageInputValid(p toolStorageAuthoringInput) bool {
	if !toolStorageSlug.MatchString(p.ToolID) || !toolStorageText(p.ToolDataRoot, 1024) || !filepath.IsAbs(p.ToolDataRoot) || !toolStorageText(p.Owner, 1024) || !toolStorageText(p.Purpose, 1024) || !toolStorageText(p.Retention, 1024) || p.MaxBytes < 1 || p.MaxBytes > (1<<50) || p.MaxAgeDays < 1 || p.MaxAgeDays > 36500 || len(p.Stores) < 1 || len(p.Stores) > 16 || len(p.ScratchEnv) > 8 {
		return false
	}
	seen := map[string]bool{}
	for _, s := range p.Stores {
		k := strings.ToUpper(s.Key)
		if !toolStorageKey.MatchString(s.Key) || !toolStorageRelative(s.Path) || (s.Kind != "cache" && s.Kind != "data") || seen[k] || toolStorageReserved(k, p.ToolID) || !toolStorageStoreKey(p.ToolID, k) {
			return false
		}
		seen[k] = true
	}
	for _, s := range p.ScratchEnv {
		k := strings.ToUpper(s)
		if !toolStorageKey.MatchString(s) || seen[k] || toolStorageReserved(k, p.ToolID) || !toolStorageScratchKey(p.ToolID, k) {
			return false
		}
		seen[k] = true
	}
	return true
}
func toolStorageDefaults(p *toolStorageAuthoringInput) bool {
	if p.ToolID == "go" {
		if len(p.Stores) != 0 || len(p.ScratchEnv) != 0 || p.Script != "" || p.Executable == "" {
			return false
		}
		p.Stores = []toolStorageAuthoringStore{{"GOCACHE", "build-cache", "cache"}, {"GOMODCACHE", "modules", "cache"}, {"GOPATH", "workspace", "data"}, {"GOBIN", "bin", "data"}}
		p.ScratchEnv = []string{"GOTMPDIR"}
		return true
	}
	if p.ToolID == "npm" {
		if len(p.Stores) != 0 || len(p.ScratchEnv) != 0 || p.Executable == "" || p.Script == "" {
			return false
		}
		p.Stores = []toolStorageAuthoringStore{{"npm_config_cache", "cache", "cache"}}
		return true
	}
	return p.Executable == "" && p.Script == ""
}
func toolStorageIdentityFile(path string) (string, string, error) {
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 0 || info.Size() > environmentExecutableLimit || environmentPlainAncestors(path) != nil {
		return "", "", errors.New("file")
	}
	id, e := localNativeID(path)
	if e != nil {
		return "", "", e
	}
	f, e := os.Open(path)
	if e != nil {
		return "", "", errors.New("file")
	}
	defer f.Close()
	openedID, e := localNativeIDFile(f)
	if e != nil || openedID != id {
		return "", "", errors.New("file")
	}
	opened, e := f.Stat()
	if e != nil || !opened.Mode().IsRegular() || opened.Size() < 0 || opened.Size() > environmentExecutableLimit {
		return "", "", errors.New("file")
	}
	raw, e := io.ReadAll(io.LimitReader(f, environmentExecutableLimit+1))
	if e != nil || len(raw) > environmentExecutableLimit {
		return "", "", errors.New("file")
	}
	if after, e := localNativeID(path); e != nil || after != id {
		return "", "", errors.New("file")
	}
	h := sha256.Sum256(raw)
	return id, hex.EncodeToString(h[:]), nil
}
func toolStoragePrepare(c layoutContext, home installationReceipt, r Request) Response {
	var in toolStorageAuthoringInput
	keys := []string{"toolId", "toolDataRoot", "owner", "purpose", "retention", "maxBytes", "maxAgeDays", "executable", "script", "stores", "scratchEnv"}
	if !toolStorageExact(r.Payload, &in, keys...) || !toolStorageInputShape(r.Payload) || !toolStorageDefaults(&in) || !toolStorageInputValid(in) || environmentPlainAncestors(filepath.Dir(in.ToolDataRoot)) != nil || environmentWithin(c.Root, in.ToolDataRoot) || environmentWithin(in.ToolDataRoot, c.Root) {
		return bad("TOOL_STORAGE_INVALID", 0)
	}
	parent := filepath.Dir(in.ToolDataRoot)
	if plainPath(parent, true) != nil {
		return bad("TOOL_STORAGE_INVALID", 0)
	}
	if existing, e := os.Lstat(filepath.Join(in.ToolDataRoot, in.ToolID)); e == nil || !os.IsNotExist(e) {
		_ = existing
		return bad("TOOL_STORAGE_CONFLICT", 0)
	}
	if _, e := os.Lstat(filepath.Join(toolStorageBase(c), in.ToolID+".json")); e == nil || !os.IsNotExist(e) {
		return bad("TOOL_STORAGE_CONFLICT", 0)
	}
	if _, e := os.Lstat(toolStorageReceiptPath(c, in.ToolID)); e == nil || !os.IsNotExist(e) {
		return bad("TOOL_STORAGE_CONFLICT", 0)
	}
	if in.ToolID == "go" || in.ToolID == "npm" {
		if _, e := os.Lstat(toolStorageUseRoot(c, in.ToolID)); !os.IsNotExist(e) {
			return bad("TOOL_STORAGE_CONFLICT", 0)
		}
	}
	p := toolStoragePlan{SchemaVersion: 1, PlanID: uuid.NewString(), toolStorageAuthoringInput: in}
	if info, e := os.Lstat(in.ToolDataRoot); e == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return bad("TOOL_STORAGE_INVALID", 0)
		}
		p.RootIdentity, _ = localNativeID(in.ToolDataRoot)
	} else if !os.IsNotExist(e) {
		return bad("TOOL_STORAGE_INVALID", 0)
	}
	p.ParentIdentity, _ = localNativeID(parent)
	if in.ToolID == "go" || in.ToolID == "npm" {
		var e error
		p.Executable, p.ExecutableIdentity, p.ExecutableSHA, e = environmentExecutable(c.Root, in.Executable)
		if e != nil {
			return bad("TOOL_STORAGE_INVALID", 0)
		}
		if in.ToolID == "npm" {
			p.ScriptIdentity, p.ScriptSHA, e = toolStorageIdentityFile(in.Script)
			if e != nil {
				return bad("TOOL_STORAGE_INVALID", 0)
			}
		}
	}
	if unlock, e := installationAdmission(c); e != nil {
		return bad("TOOL_STORAGE_BUSY", 0)
	} else {
		defer unlock()
	}
	writer, e := acquireUpdateRecoveryWriter(c)
	if e != nil {
		return bad("TOOL_STORAGE_BUSY", 0)
	}
	defer writer.Release()
	if toolStorageStopped(c) {
		return bad("TOOL_STORAGE_STOPPED", 0)
	}
	if toolStoragePending(c) {
		return bad("TOOL_STORAGE_PENDING", 0)
	}
	path := toolStoragePlanPath(c, in.ToolID)
	if raw, e := installationRead(c.Root, toolStorageRel(c, path), toolStorageAuthoringLimit); e == nil {
		var old toolStoragePlan
		if !checkpointStrictJSON(raw, &old) || !toolStoragePlanEquivalent(old, p) {
			return bad("TOOL_STORAGE_CONFLICT", 0)
		}
		id, e := localNativeID(path)
		if e != nil {
			return bad("TOOL_STORAGE_PENDING", 0)
		}
		return good("TOOL_STORAGE_PREPARED", 0, toolStoragePlanData(old, raw, id))
	}
	raw, _ := json.Marshal(p)
	if e := installationWrite(c.Root, toolStorageRel(c, path), raw); e != nil {
		return bad("TOOL_STORAGE_PENDING", 0)
	}
	id, e := localNativeID(path)
	if e != nil {
		return bad("TOOL_STORAGE_PENDING", 0)
	}
	return good("TOOL_STORAGE_PREPARED", 0, toolStoragePlanData(p, raw, id))
}
func toolStorageInputShape(raw json.RawMessage) bool {
	var v struct {
		Stores []json.RawMessage `json:"stores"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return false
	}
	for _, store := range v.Stores {
		if !toolStorageExactKeys(store, "key", "path", "kind") {
			return false
		}
	}
	return true
}
func toolStoragePlanEquivalent(a, b toolStoragePlan) bool {
	a.PlanID = b.PlanID
	return reflect.DeepEqual(a, b)
}
func toolStoragePlanData(p toolStoragePlan, raw []byte, id string) map[string]any {
	h := sha256.Sum256(append(append([]byte{}, raw...), []byte(":"+id)...))
	stores := make([]string, 0, len(p.Stores))
	for _, store := range p.Stores {
		stores = append(stores, filepath.Join(p.ToolDataRoot, p.ToolID, filepath.FromSlash(store.Path)))
	}
	return map[string]any{"phase": "prepared", "planId": p.PlanID, "planSha256": hex.EncodeToString(h[:]), "toolId": p.ToolID, "toolDataRoot": p.ToolDataRoot, "stores": stores, "executable": p.Executable, "script": p.Script, "nextAction": "confirm current owner approval"}
}
func toolStorageFailure(reason, stage, next string) Response {
	return installationResponse("TOOL_STORAGE_REFUSED", false, map[string]any{"reason": reason, "stage": stage, "nextAction": next})
}
func toolStoragePending(c layoutContext) bool {
	_, e := os.Lstat(toolStorageTransactionPath(c))
	if e == nil || !os.IsNotExist(e) {
		return true
	}
	raw, e := installationRead(c.Root, toolStorageRel(c, toolStorageRelocationStatePath(c)), toolStorageRelocationLimit)
	if os.IsNotExist(e) {
		return false
	}
	var state toolStorageRelocationPlan
	return e != nil || !checkpointStrictJSON(raw, &state) || !toolStorageRelocationTerminalValid(c, state)
}
func toolStorageWriteTransaction(c layoutContext, tx toolStorageTransaction) error {
	raw, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	return installationWrite(c.Root, toolStorageRel(c, toolStorageTransactionPath(c)), raw)
}
func toolStorageReadTransaction(c layoutContext) (toolStorageTransaction, error) {
	raw, e := installationRead(c.Root, toolStorageRel(c, toolStorageTransactionPath(c)), toolStorageAuthoringLimit)
	if e != nil {
		return toolStorageTransaction{}, e
	}
	var x toolStorageTransaction
	if !checkpointStrictJSON(raw, &x) || x.SchemaVersion != 1 || !toolStorageSlug.MatchString(x.ToolID) || !isUUID(x.PlanID) || !toolStorageHash(x.PlanSHA256) {
		return toolStorageTransaction{}, errors.New("transaction")
	}
	return x, nil
}
func toolStorageReadPlan(c layoutContext, a toolStorageApproval) (toolStoragePlan, []byte, string, error) {
	path := toolStoragePlanPath(c, a.ToolID)
	raw, e := installationRead(c.Root, toolStorageRel(c, path), toolStorageAuthoringLimit)
	if e != nil {
		return toolStoragePlan{}, nil, "", e
	}
	var p toolStoragePlan
	if !checkpointStrictJSON(raw, &p) || p.ToolID != a.ToolID || p.PlanID != a.PlanID {
		return p, nil, "", errors.New("plan")
	}
	id, e := localNativeID(path)
	if e != nil {
		return p, nil, "", e
	}
	d := toolStoragePlanData(p, raw, id)
	if d["planSha256"] != a.PlanSHA256 {
		return p, nil, "", errors.New("plan")
	}
	return p, raw, id, nil
}
func toolStorageApprovalValid(raw json.RawMessage, a *toolStorageApproval, consent bool) bool {
	keys := []string{"toolId", "planId", "planSha256"}
	if consent {
		keys = append(keys, "consentReference")
	}
	return toolStorageExact(raw, a, keys...) && toolStorageSlug.MatchString(a.ToolID) && isUUID(a.PlanID) && toolStorageHash(a.PlanSHA256) && (!consent || toolStorageText(a.ConsentReference, 1024))
}
func toolStorageApply(c layoutContext, home installationReceipt, r Request, recover bool) Response {
	var a toolStorageApproval
	if !toolStorageApprovalValid(r.Payload, &a, true) {
		return bad("TOOL_STORAGE_INVALID", 0)
	}
	writer, e := acquireUpdateRecoveryWriter(c)
	if e != nil {
		return bad("TOOL_STORAGE_BUSY", 0)
	}
	defer writer.Release()
	p, raw, planID, e := toolStorageReadPlan(c, a)
	if e != nil {
		return toolStorageFailure("approval_or_plan", "admission", "inspect the prepared plan and submit its current approval")
	}
	if recover {
		recorded, e := toolStorageReadTransaction(c)
		consent := sha256.Sum256([]byte(a.ConsentReference))
		if e != nil || recorded.PlanID != a.PlanID || recorded.ToolID != a.ToolID || recorded.PlanSHA256 != a.PlanSHA256 || recorded.ConsentDigest != hex.EncodeToString(consent[:]) || recorded.LockID == "" {
			return bad("TOOL_STORAGE_UNRESOLVED", 0)
		}
		lock := ownedSetupLock{path: filepath.Join(c.Root, filepath.FromSlash(c.Paths["bindings"]), ".setup-lock"), nativeID: recorded.LockID}
		if e = lock.Release(); e != nil {
			return bad("TOOL_STORAGE_UNRESOLVED", 0)
		}
	}
	lock, e := setupLockOwned(filepath.Join(c.Root, filepath.FromSlash(c.Paths["bindings"])))
	if e != nil {
		return bad("TOOL_STORAGE_BUSY", 0)
	}
	defer lock.Release()
	if updatePending(c) {
		return bad("TOOL_STORAGE_PENDING", 0)
	}
	if toolStorageStopped(c) && !recover {
		return bad("TOOL_STORAGE_STOPPED", 0)
	}
	_ = raw
	if active, e := toolStorageReadReceipt(c, a.ToolID); e == nil {
		if active.Phase == "active" && active.PlanID == a.PlanID && active.PlanSHA256 == a.PlanSHA256 && active.ConsentReference == a.ConsentReference {
			if toolStorageActiveCurrent(c, active) != nil {
				return toolStorageFailure("active_proof", "replay", "inspect retained tool storage state")
			}
			if recover {
				recorded, txErr := toolStorageReadTransaction(c)
				if txErr != nil || recorded.Phase != "active" || recorded.PlanID != a.PlanID || os.Remove(toolStorageTransactionPath(c)) != nil {
					return bad("TOOL_STORAGE_UNRESOLVED", 0)
				}
			}
			return good("TOOL_STORAGE_ACTIVE", 0, map[string]any{"phase": "active", "verification": active.Verification})
		}
		return bad("TOOL_STORAGE_CONFLICT", 0)
	}
	if recover {
		recoveredTx, e := toolStorageReadTransaction(c)
		if e != nil || recoveredTx.PlanID != a.PlanID {
			return bad("TOOL_STORAGE_UNRESOLVED", 0)
		}
		if recoveredTx.Phase != "active" {
			return bad("TOOL_STORAGE_UNRESOLVED", 0)
		}
	}
	if toolStoragePending(c) && !recover {
		return bad("TOOL_STORAGE_PENDING", 0)
	}
	if !recover && toolStoragePlanCurrent(c, p) != nil {
		return bad("TOOL_STORAGE_REFUSED", 0)
	}
	if !recover && (p.ToolID == "go" || p.ToolID == "npm") {
		if _, e := os.Lstat(toolStorageUseRoot(c, p.ToolID)); !os.IsNotExist(e) {
			return bad("TOOL_STORAGE_CONFLICT", 0)
		}
	}
	consent := sha256.Sum256([]byte(a.ConsentReference))
	tx := toolStorageTransaction{SchemaVersion: 1, Phase: "intent", ToolID: a.ToolID, PlanID: a.PlanID, PlanSHA256: a.PlanSHA256, LockID: lock.nativeID, ConsentDigest: hex.EncodeToString(consent[:])}
	if e := toolStorageWriteTransaction(c, tx); e != nil {
		return bad("TOOL_STORAGE_PENDING", 0)
	}
	toolStorageAuthoringHook("intent")
	if e := os.MkdirAll(c.Runs, 0700); e != nil {
		return bad("TOOL_STORAGE_UNRESOLVED", 0)
	}
	if e := toolStorageCreateDirectories(p); e != nil {
		return bad("TOOL_STORAGE_UNRESOLVED", 0)
	}
	tx.Phase = "directories"
	if e := toolStorageWriteTransaction(c, tx); e != nil {
		return bad("TOOL_STORAGE_UNRESOLVED", 0)
	}
	toolStorageAuthoringHook("directories")
	stores, rootID, e := toolStorageAuthoringStores(p)
	if e != nil {
		return bad("TOOL_STORAGE_UNRESOLVED", 0)
	}
	verification := "path-variables-only"
	if p.ToolID == "go" || p.ToolID == "npm" {
		if e := toolStorageProbe(c, p, stores, &tx); e != nil {
			stage := "probe.execution"
			var typed toolStorageProbeFailure
			if errors.As(e, &typed) {
				stage = string(typed)
			}
			data := map[string]any{"reason": "provider_probe_failed", "stage": stage, "nextAction": "inspect retained probe state"}
			var diagnostic *toolStorageProbeDiagnosticError
			if errors.As(e, &diagnostic) {
				data["diagnostic"] = diagnostic.observation
			}
			return installationResponse("TOOL_STORAGE_PROBE_FAILED", false, data)
		}
		verification = "effective-paths-verified"
	}
	profile := toolStoragePolicy{SchemaVersion: 1, ToolID: p.ToolID, HomeIdentity: home.HomeID, ToolDataRoot: p.ToolDataRoot, ToolDataIdentity: rootID, Owner: p.Owner, Purpose: p.Purpose, Retention: p.Retention, ConsentReference: "ocmi-tool-storage:" + p.PlanID, MaxBytes: p.MaxBytes, MaxAgeDays: p.MaxAgeDays, Stores: stores, ScratchEnv: p.ScratchEnv}
	if p.ToolID == "go" || p.ToolID == "npm" {
		useRoot := filepath.Join(toolStorageBase(c), "uses", p.ToolID)
		if err := os.MkdirAll(filepath.Dir(useRoot), 0700); err != nil {
			return bad("TOOL_STORAGE_UNRESOLVED", 0)
		}
		if err := os.Mkdir(useRoot, 0700); err != nil {
			return bad("TOOL_STORAGE_UNRESOLVED", 0)
		}
		id, err := localNativeID(useRoot)
		if err != nil {
			return bad("TOOL_STORAGE_UNRESOLVED", 0)
		}
		profile.SchemaVersion, profile.UseRootIdentity = 2, id
		tx.UseRootID = id
		if err := toolStorageWriteTransaction(c, tx); err != nil {
			return bad("TOOL_STORAGE_UNRESOLVED", 0)
		}
	}
	profileRaw, _ := json.Marshal(profile)
	profilePath := filepath.Join(toolStorageBase(c), p.ToolID+".json")
	if e := installationWrite(c.Root, toolStorageRel(c, profilePath), profileRaw); e != nil {
		return bad("TOOL_STORAGE_UNRESOLVED", 0)
	}
	profileID, e := localNativeID(profilePath)
	if e != nil {
		return bad("TOOL_STORAGE_UNRESOLVED", 0)
	}
	h := sha256.Sum256(profileRaw)
	tx.Phase = "profile"
	if e := toolStorageWriteTransaction(c, tx); e != nil {
		return bad("TOOL_STORAGE_UNRESOLVED", 0)
	}
	toolStorageAuthoringHook("profile")
	rc := toolStorageReceipt{SchemaVersion: 1, Phase: "active", PlanID: p.PlanID, PlanSHA256: a.PlanSHA256, ConsentReference: a.ConsentReference, Home: c.Root, HomeIdentity: home.HomeID, InstallationID: home.InstallationID, DeviceID: home.DeviceID, CopyID: home.CopyID, ToolID: p.ToolID, ToolDataRoot: p.ToolDataRoot, ToolDataIdentity: rootID, ProfileSHA256: hex.EncodeToString(h[:]), ProfileNativeIdentity: profileID, ExecutableIdentity: p.ExecutableIdentity, ExecutableSHA: p.ExecutableSHA, ScriptIdentity: p.ScriptIdentity, ScriptSHA: p.ScriptSHA, Verification: verification, Stores: stores, UseRootIdentity: profile.UseRootIdentity}
	rb, _ := json.Marshal(rc)
	if e := installationWrite(c.Root, toolStorageRel(c, toolStorageReceiptPath(c, p.ToolID)), rb); e != nil {
		return bad("TOOL_STORAGE_UNRESOLVED", 0)
	}
	tx.Phase = "active"
	if e := toolStorageWriteTransaction(c, tx); e != nil {
		return bad("TOOL_STORAGE_UNRESOLVED", 0)
	}
	toolStorageAuthoringHook("active")
	if e := os.Remove(toolStorageTransactionPath(c)); e != nil {
		return bad("TOOL_STORAGE_UNRESOLVED", 0)
	}
	_ = planID
	return good("TOOL_STORAGE_ACTIVE", 0, map[string]any{"phase": "active", "verification": verification})
}
func toolStoragePlanCurrent(c layoutContext, p toolStoragePlan) error {
	parent := filepath.Dir(p.ToolDataRoot)
	parentID, e := localNativeID(parent)
	if e != nil || parentID != p.ParentIdentity {
		return errors.New("parent")
	}
	if p.RootIdentity == "" {
		if _, e = os.Lstat(p.ToolDataRoot); !os.IsNotExist(e) {
			return errors.New("root")
		}
	} else {
		id, x := localNativeID(p.ToolDataRoot)
		if x != nil || id != p.RootIdentity {
			return errors.New("root")
		}
	}
	if _, e = os.Lstat(filepath.Join(p.ToolDataRoot, p.ToolID)); !os.IsNotExist(e) {
		return errors.New("tool")
	}
	for _, path := range []string{filepath.Join(toolStorageBase(c), p.ToolID+".json"), toolStorageReceiptPath(c, p.ToolID)} {
		if _, e = os.Lstat(path); !os.IsNotExist(e) {
			return errors.New("metadata")
		}
	}
	if p.ToolID == "go" || p.ToolID == "npm" {
		got, id, sha, x := environmentExecutable(c.Root, p.Executable)
		if x != nil || got != p.Executable || id != p.ExecutableIdentity || sha != p.ExecutableSHA {
			return errors.New("executable")
		}
		if p.ToolID == "npm" {
			id, sha, x := toolStorageIdentityFile(p.Script)
			if x != nil || id != p.ScriptIdentity || sha != p.ScriptSHA {
				return errors.New("script")
			}
		}
	}
	return nil
}
func toolStorageAuthoringHook(phase string) {
	if toolStorageAuthoringTestPhase != nil {
		toolStorageAuthoringTestPhase(phase)
	}
}
func toolStorageCreateDirectories(p toolStoragePlan) error {
	if p.RootIdentity == "" {
		if e := os.Mkdir(p.ToolDataRoot, 0700); e != nil {
			return e
		}
	}
	tool := filepath.Join(p.ToolDataRoot, p.ToolID)
	if e := os.Mkdir(tool, 0700); e != nil {
		return e
	}
	for _, s := range p.Stores {
		if e := os.MkdirAll(filepath.Join(tool, filepath.FromSlash(s.Path)), 0700); e != nil {
			return e
		}
	}
	return nil
}
func toolStorageAuthoringStores(p toolStoragePlan) ([]toolStorageStore, string, error) {
	id, e := localNativeID(p.ToolDataRoot)
	if e != nil {
		return nil, "", e
	}
	out := make([]toolStorageStore, 0, len(p.Stores))
	for _, s := range p.Stores {
		path := filepath.Join(p.ToolDataRoot, p.ToolID, filepath.FromSlash(s.Path))
		sid, e := localNativeID(path)
		if e != nil {
			return nil, "", e
		}
		out = append(out, toolStorageStore{Key: s.Key, Path: filepath.ToSlash(filepath.Join(p.ToolID, s.Path)), NativeIdentity: sid, Kind: s.Kind})
	}
	return out, id, nil
}
func toolStorageProbe(c layoutContext, p toolStoragePlan, stores []toolStorageStore, tx *toolStorageTransaction) error {
	return toolStorageProbeRecorded(c, p, stores, tx, func() error { return toolStorageWriteTransaction(c, *tx) })
}
func toolStorageProbeRecorded(c layoutContext, p toolStoragePlan, stores []toolStorageStore, tx *toolStorageTransaction, record func() error) (failure error) {
	locator := filepath.ToSlash(filepath.Join("local", "runs", p.PlanID))
	scratch := filepath.Join(c.Root, filepath.FromSlash(locator))
	step := "stage.create"
	var observed commandResult
	var runErr error
	runnerObserved, recordFailed := false, false
	defer func() {
		if failure == nil {
			return
		}
		d := toolStorageProbeObservation(p.ToolID, step, observed, runErr, recordFailed)
		d["runnerObserved"] = runnerObserved
		d["cwdChars"], d["tmpChars"] = len([]rune(scratch)), len([]rune(scratch))
		if p.ToolID == "npm" {
			d["userConfigChars"] = len([]rune(filepath.Join(scratch, "npm-userconfig")))
			d["globalConfigChars"] = len([]rune(filepath.Join(scratch, "npm-globalconfig")))
		}
		for _, store := range stores {
			if store.Key == "GOCACHE" || store.Key == "npm_config_cache" {
				d["cacheChars"] = len([]rune(filepath.Join(p.ToolDataRoot, filepath.FromSlash(store.Path))))
			}
		}
		failure = &toolStorageProbeDiagnosticError{cause: failure, observation: d}
	}()
	stage, e := createLocalResource(c.Root, locator, "directory", "tool-storage-probe-"+p.PlanID, nil, true)
	if e != nil {
		return e
	}
	stageID := stage.NativeIdentity
	tx.Stage, tx.StageID, tx.ProbeState = scratch, stageID, "prepared"
	step = "stage.record"
	if e := record(); e != nil {
		return e
	}
	if p.ToolID == "npm" {
		step = "config.write"
		stage, e = writeLocalResource(c.Root, locator, "directory", stageID, stage.Inventory, "npm-userconfig", []byte{}, true)
		if e != nil {
			return e
		}
		stage, e = writeLocalResource(c.Root, locator, "directory", stageID, stage.Inventory, "npm-globalconfig", []byte{}, true)
		if e != nil {
			return e
		}
	}
	env := []string{"HOME=" + scratch, "TMP=" + scratch, "TEMP=" + scratch, "TMPDIR=" + scratch}
	for _, s := range stores {
		env = append(env, s.Key+"="+filepath.Join(p.ToolDataRoot, filepath.FromSlash(s.Path)))
	}
	args := []string{}
	exe := p.Executable
	if p.ToolID == "go" {
		args = []string{"env", "-json", "GOCACHE", "GOMODCACHE", "GOPATH", "GOBIN", "GOTMPDIR", "GOTELEMETRY", "GOTELEMETRYDIR"}
		env = append(env, "APPDATA="+scratch, "XDG_CONFIG_HOME="+scratch, "GOENV=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off", "GOTMPDIR="+scratch)
	} else {
		args = []string{p.Script, "config", "get", "cache"}
		env = append(env, "npm_config_userconfig="+filepath.Join(scratch, "npm-userconfig"), "npm_config_globalconfig="+filepath.Join(scratch, "npm-globalconfig"), "npm_config_cache="+filepath.Join(p.ToolDataRoot, filepath.FromSlash(stores[0].Path)), "npm_config_update_notifier=false", "npm_config_loglevel=silent", "npm_config_logs_max=0")
	}
	runProbe := func(callArgs []string) (commandResult, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		observed, runErr = toolStorageAuthoringRun(ctx, commandSpec{Executable: exe, Args: callArgs, Cwd: scratch, OutputLimit: 8192, Env: env, Registered: func(pid, group int) error {
			tx.ProbePID = pid
			tx.ProbeState = "registered"
			callbackErr := record()
			recordFailed = callbackErr != nil
			return callbackErr
		}})
		runnerObserved = true
		return observed, runErr
	}
	if p.ToolID == "go" {
		step = "telemetry"
		off, offErr := runProbe([]string{"telemetry", "off"})
		if offErr != nil || off.ExitCode != 0 || off.TimedOut || off.Truncated || off.Pending || off.Unresolved || !off.StoppedVerified {
			return errors.New("probe.telemetry")
		}
		tx.ProbeState = "telemetry-off"
		step = "telemetry.record"
		if e := record(); e != nil {
			return e
		}
	}
	step = "execute"
	run, e := runProbe(args)
	if e != nil || run.ExitCode != 0 || run.TimedOut || run.Truncated || run.Pending || run.Unresolved || !run.StoppedVerified {
		return errors.New("probe")
	}
	step = "identity"
	if current, idErr := localNativeID(scratch); idErr != nil || current != stageID {
		return toolStorageProbeFailure("probe.identity")
	}
	step = "inventory"
	stage, e = captureLocalResource(c.Root, locator, "directory", stageID, true)
	if e != nil {
		return toolStorageProbeFailure("probe.inventory")
	}
	tx.ProbeState = "stopped:" + stage.Inventory
	step = "stopped.record"
	if e := record(); e != nil {
		return e
	}
	step = "response"
	if p.ToolID == "go" {
		var got map[string]string
		if json.Unmarshal([]byte(run.Output), &got) != nil {
			return toolStorageProbeFailure("probe.response")
		}
		for _, s := range stores {
			if filepath.Clean(got[s.Key]) != filepath.Clean(filepath.Join(p.ToolDataRoot, filepath.FromSlash(s.Path))) {
				return toolStorageProbeFailure("probe.response")
			}
		}
		if filepath.Clean(got["GOTMPDIR"]) != filepath.Clean(scratch) || got["GOTELEMETRY"] != "off" || !environmentWithin(scratch, got["GOTELEMETRYDIR"]) {
			return toolStorageProbeFailure("probe.response")
		}
		step = "cleanup"
		if e := removeLocalResource(c.Root, locator, "directory", stageID, stage.Inventory, true); e != nil {
			return toolStorageProbeFailure("probe.cleanup")
		}
		return nil
	}
	if filepath.Clean(strings.TrimSpace(run.Output)) != filepath.Clean(filepath.Join(p.ToolDataRoot, filepath.FromSlash(stores[0].Path))) {
		return toolStorageProbeFailure("probe.response")
	}
	step = "cleanup"
	if e := removeLocalResource(c.Root, locator, "directory", stageID, stage.Inventory, true); e != nil {
		return toolStorageProbeFailure("probe.cleanup")
	}
	return nil
}
func toolStorageAuthoringRun(ctx context.Context, spec commandSpec) (commandResult, error) {
	if environmentRunOverride != nil {
		return environmentRunOverride(ctx, spec)
	}
	return runOwned(ctx, spec)
}
func toolStorageReadReceipt(c layoutContext, tool string) (toolStorageReceipt, error) {
	raw, e := installationRead(c.Root, toolStorageRel(c, toolStorageReceiptPath(c, tool)), toolStorageAuthoringLimit)
	if e != nil {
		return toolStorageReceipt{}, e
	}
	var x toolStorageReceipt
	if !toolStorageReceiptShape(raw) || !checkpointStrictJSON(raw, &x) || x.SchemaVersion != 1 || x.ToolID != tool || (x.Verification != "path-variables-only" && x.Verification != "effective-paths-verified") {
		return toolStorageReceipt{}, errors.New("receipt")
	}
	return x, nil
}
func toolStorageReceiptShape(raw []byte) bool {
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return false
	}
	keys := []string{"schemaVersion", "phase", "planId", "planSha256", "consentReference", "home", "homeIdentity", "installationId", "deviceId", "copyId", "toolId", "toolDataRoot", "toolDataIdentity", "profileSha256", "profileNativeIdentity", "executableIdentity", "executableSha256", "scriptIdentity", "scriptSha256", "verification", "stores"}
	if _, ok := top["useRootIdentity"]; ok {
		keys = append(keys, "useRootIdentity")
	}
	if !toolStorageExactKeys(raw, keys...) {
		return false
	}
	var v struct {
		Stores []json.RawMessage `json:"stores"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return false
	}
	for _, store := range v.Stores {
		if !toolStorageExactKeys(store, "key", "path", "nativeIdentity", "kind") {
			return false
		}
	}
	return true
}
func toolStorageActiveCurrent(c layoutContext, x toolStorageReceipt) error {
	if x.Phase != "active" || x.Home != c.Root || x.ToolID == "" || x.ProfileSHA256 == "" || x.ProfileNativeIdentity == "" {
		return errors.New("receipt")
	}
	profilePath := filepath.Join(toolStorageBase(c), x.ToolID+".json")
	raw, e := installationRead(c.Root, toolStorageRel(c, profilePath), toolStorageAuthoringLimit)
	if e != nil {
		return e
	}
	h := sha256.Sum256(raw)
	profileID, e := localNativeID(profilePath)
	if e != nil || hex.EncodeToString(h[:]) != x.ProfileSHA256 || profileID != x.ProfileNativeIdentity {
		return errors.New("profile")
	}
	planRaw, e := installationRead(c.Root, toolStorageRel(c, toolStoragePlanPath(c, x.ToolID)), toolStorageAuthoringLimit)
	if e != nil {
		return e
	}
	var p toolStoragePlan
	if !checkpointStrictJSON(planRaw, &p) || p.PlanID != x.PlanID || p.ToolID != x.ToolID || (p.ToolID == "go" || p.ToolID == "npm") && (p.ExecutableIdentity != x.ExecutableIdentity || p.ExecutableSHA != x.ExecutableSHA) {
		return errors.New("plan")
	}
	if p.ToolID == "go" || p.ToolID == "npm" {
		got, id, sha, e := environmentExecutable(c.Root, p.Executable)
		if e != nil || got != p.Executable || id != p.ExecutableIdentity || sha != p.ExecutableSHA {
			return errors.New("provider")
		}
		if p.ToolID == "npm" {
			id, sha, e = toolStorageIdentityFile(p.Script)
			if e != nil || id != p.ScriptIdentity || sha != p.ScriptSHA || id != x.ScriptIdentity || sha != x.ScriptSHA {
				return errors.New("script")
			}
		}
	}
	return nil
}
func toolStorageAuthoringReceiptProof(ref toolStorageRef, policy toolStoragePolicy, profileID string) (string, error) {
	c, code := resolveLayout(ref.Home, "installation")
	if code != "" || toolStoragePending(c) {
		return "", errors.New("receipt")
	}
	raw, e := installationRead(c.Root, toolStorageRel(c, toolStorageReceiptPath(c, ref.ToolID)), toolStorageAuthoringLimit)
	if e != nil {
		return "", e
	}
	id, e := localNativeID(toolStorageReceiptPath(c, ref.ToolID))
	if e != nil {
		return "", e
	}
	x, receiptErr := toolStorageReadReceipt(c, ref.ToolID)
	planID := strings.TrimPrefix(policy.ConsentReference, "ocmi-tool-storage:")
	homeReceipt, homeErr := readInstallationReceiptContext(c)
	if receiptErr != nil || homeErr != nil || x.Phase != "active" || x.PlanID != planID || x.ToolID != ref.ToolID || x.Home != ref.Home || x.HomeIdentity != policy.HomeIdentity || x.InstallationID != homeReceipt.InstallationID || x.DeviceID != homeReceipt.DeviceID || x.CopyID != homeReceipt.CopyID || x.ToolDataRoot != policy.ToolDataRoot || x.ToolDataIdentity != policy.ToolDataIdentity || x.ProfileSHA256 != ref.SHA256 || x.ProfileNativeIdentity != profileID || len(x.Stores) != len(policy.Stores) {
		return "", errors.New("receipt")
	}
	for i := range x.Stores {
		if x.Stores[i] != policy.Stores[i] {
			return "", errors.New("receipt")
		}
	}
	if toolStorageActiveCurrent(c, x) != nil {
		return "", errors.New("receipt")
	}
	h := sha256.Sum256(raw)
	return id + ":" + hex.EncodeToString(h[:]), nil
}
func toolStorageInspect(c layoutContext, r Request) Response {
	var q struct {
		ToolID string `json:"toolId"`
	}
	if !toolStorageExact(r.Payload, &q, "toolId") || !toolStorageSlug.MatchString(q.ToolID) {
		return bad("TOOL_STORAGE_INVALID", 0)
	}
	if tx, e := toolStorageReadTransaction(c); e == nil && tx.ToolID == q.ToolID {
		owner, purpose, lifetime := "coordinator", "tool storage", "until recovery"
		if raw, readErr := installationRead(c.Root, toolStorageRel(c, toolStoragePlanPath(c, q.ToolID)), toolStorageAuthoringLimit); readErr == nil {
			var p toolStoragePlan
			if checkpointStrictJSON(raw, &p) && p.ToolID == q.ToolID {
				owner, purpose, lifetime = p.Owner, p.Purpose, p.Retention
			}
		}
		return installationResponse("TOOL_STORAGE_PENDING", false, map[string]any{"phase": "pending", "reason": "retained_transaction", "stage": tx.Phase, "nextAction": "run tool storage recover", "toolId": q.ToolID, "owner": owner, "purpose": purpose, "lifetime": lifetime})
	}
	x, e := toolStorageReadReceipt(c, q.ToolID)
	if e != nil {
		path := toolStoragePlanPath(c, q.ToolID)
		raw, readErr := installationRead(c.Root, toolStorageRel(c, path), toolStorageAuthoringLimit)
		var p toolStoragePlan
		if readErr == nil && checkpointStrictJSON(raw, &p) && p.ToolID == q.ToolID {
			id, idErr := localNativeID(path)
			if idErr == nil {
				return good("TOOL_STORAGE_INSPECTED", 0, toolStoragePlanData(p, raw, id))
			}
		}
		return bad("TOOL_STORAGE_NOT_FOUND", 0)
	}
	if toolStorageActiveCurrent(c, x) != nil {
		return toolStorageFailure("active_proof", "inspect", "inspect retained tool storage state")
	}
	return good("TOOL_STORAGE_INSPECTED", 0, map[string]any{"phase": x.Phase, "verification": x.Verification, "toolId": x.ToolID})
}
func toolStorageCancel(c layoutContext, r Request) Response {
	var a toolStorageApproval
	if !toolStorageApprovalValid(r.Payload, &a, false) {
		return bad("TOOL_STORAGE_INVALID", 0)
	}
	if unlock, e := installationAdmission(c); e != nil {
		return bad("TOOL_STORAGE_BUSY", 0)
	} else {
		defer unlock()
	}
	writer, e := acquireUpdateRecoveryWriter(c)
	if e != nil {
		return bad("TOOL_STORAGE_BUSY", 0)
	}
	defer writer.Release()
	if toolStoragePending(c) {
		return bad("TOOL_STORAGE_PENDING", 0)
	}
	for _, path := range []string{toolStorageReceiptPath(c, a.ToolID), filepath.Join(toolStorageBase(c), a.ToolID+".json")} {
		if _, e := os.Lstat(path); e == nil || !os.IsNotExist(e) {
			return toolStorageFailure("not_prepared", "cancel", "inspect retained tool storage state")
		}
	}
	if _, e := toolStorageReadReceipt(c, a.ToolID); e == nil {
		return toolStorageFailure("active", "cancel", "inspect active tool storage")
	}
	if _, _, _, e := toolStorageReadPlan(c, a); e != nil {
		return bad("TOOL_STORAGE_REFUSED", 0)
	}
	if e := os.Remove(toolStoragePlanPath(c, a.ToolID)); e != nil {
		return bad("TOOL_STORAGE_PENDING", 0)
	}
	return good("TOOL_STORAGE_CANCELLED", 0, map[string]any{"phase": "cancelled"})
}
