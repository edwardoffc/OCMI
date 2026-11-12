package internal

// C1 relocation is deliberately narrow: it moves the selected native Go/npm
// subtree only, retains both generations, and uses the profile's per-operation
// directory as the transition barrier.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const toolStorageRelocationLimit = 16 << 20
const toolStorageUseCap = 64

var errToolStorageReaderCapability = errors.New("tool storage reader capability")

func toolStorageSchema2Present(c layoutContext) (bool, error) {
	dir := toolStorageBase(c)
	f, err := os.Open(dir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("profiles")
	}
	defer f.Close()
	if plainPath(dir, true) != nil || environmentPlainAncestors(dir) != nil {
		return false, errors.New("profiles")
	}
	dirID, err := localNativeID(dir)
	if err != nil {
		return false, err
	}
	openedID, err := localNativeIDFile(f)
	if err != nil || openedID != dirID {
		return false, errors.New("profiles identity")
	}
	entries, err := f.ReadDir(33)
	if err != nil && err != io.EOF || len(entries) > 32 {
		return false, errors.New("profiles")
	}
	active := false
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 || e.Type()&os.ModeIrregular != 0 {
			return false, errors.New("profiles")
		}
		if e.IsDir() {
			if e.Name() != "plans" && e.Name() != "receipts" && e.Name() != "uses" && e.Name() != "relocation" {
				return false, errors.New("profiles")
			}
			continue
		}
		if !strings.HasSuffix(e.Name(), ".json") || e.Name() == "transaction.json" || e.Name() == "relocation.json" {
			if e.Name() != "transaction.json" && e.Name() != "relocation.json" {
				return false, errors.New("profiles")
			}
			continue
		}
		tool := strings.TrimSuffix(e.Name(), ".json")
		if !toolStorageSlug.MatchString(tool) {
			return false, errors.New("profiles")
		}
		raw, err := installationRead(c.Root, toolStorageRel(c, filepath.Join(dir, e.Name())), toolStorageLimit)
		if err != nil {
			return false, err
		}
		var p toolStoragePolicy
		if !toolStoragePolicyShape(raw) || !checkpointStrictJSON(raw, &p) {
			return false, errors.New("profile")
		}
		if p.SchemaVersion == 2 {
			h := sha256.Sum256(raw)
			if _, _, _, _, err := toolStorageRelocationProfile(c, tool, hex.EncodeToString(h[:])); err != nil {
				return false, err
			}
			active = true
			continue
		}
		if p.SchemaVersion != 1 {
			return false, errors.New("profile")
		}
	}
	if id, e := localNativeID(dir); e != nil || id != dirID {
		return false, errors.New("profiles drift")
	}
	return active, nil
}

func toolStorageReaderCapability(root string) (int, error) {
	raw, err := installationRead(root, "system/build-info.json", 1<<20)
	if err != nil {
		return 0, err
	}
	var v map[string]json.RawMessage
	if !checkpointStrictJSON(raw, &v) {
		return 0, errors.New("build info")
	}
	format := v["toolStorageProfileFormat"]
	if len(format) == 0 {
		return 0, nil
	}
	var n int
	if !checkpointStrictJSON(format, &n) || n != 2 {
		return 0, errToolStorageReaderCapability
	}
	return n, nil
}
func toolStorageReaderGuard(c layoutContext, candidateRoot string) error {
	active, err := toolStorageSchema2Present(c)
	if err != nil {
		return errToolStorageReaderCapability
	}
	if !active {
		return nil
	}
	n, err := toolStorageReaderCapability(candidateRoot)
	if err != nil || n < 2 {
		return errToolStorageReaderCapability
	}
	return nil
}

func toolStorageReaderGuardSystem(c layoutContext, system string) error {
	active, err := toolStorageSchema2Present(c)
	if err != nil || !active {
		if err != nil {
			return errToolStorageReaderCapability
		}
		return nil
	}
	raw, err := installationRead(system, "build-info.json", 1<<20)
	if err != nil {
		return errToolStorageReaderCapability
	}
	var fields map[string]json.RawMessage
	if !checkpointStrictJSON(raw, &fields) {
		return errToolStorageReaderCapability
	}
	format := fields["toolStorageProfileFormat"]
	var n int
	if len(format) == 0 || !checkpointStrictJSON(format, &n) || n != 2 {
		return errToolStorageReaderCapability
	}
	return nil
}

type toolStorageRelocationInput struct {
	ToolID        string `json:"toolId"`
	ProfileSHA256 string `json:"profileSha256"`
	ToolDataRoot  string `json:"toolDataRoot"`
	MaxEntries    int    `json:"maxEntries"`
	MaxBytes      int64  `json:"maxBytes"`
}
type toolStorageRelocationApproval struct {
	ToolID           string `json:"toolId"`
	PlanID           string `json:"planId"`
	PlanSHA256       string `json:"planSha256"`
	ConsentReference string `json:"consentReference"`
}
type toolStorageRelocationEntry struct {
	Path           string `json:"path"`
	Directory      bool   `json:"directory"`
	NativeIdentity string `json:"nativeIdentity"`
	Size           int64  `json:"size"`
	SHA256         string `json:"sha256"`
	Mode           uint32 `json:"mode"`
}
type toolStorageRelocationPlan struct {
	SchemaVersion        int                          `json:"schemaVersion"`
	Phase                string                       `json:"phase"`
	PlanID               string                       `json:"planId"`
	ApprovedPlanSHA      string                       `json:"approvedPlanSha256,omitempty"`
	ToolID               string                       `json:"toolId"`
	ProfileSHA256        string                       `json:"profileSha256"`
	ConsentReference     string                       `json:"consentReference,omitempty"`
	SourceRoot           string                       `json:"sourceRoot"`
	SourceIdentity       string                       `json:"sourceIdentity"`
	TargetRoot           string                       `json:"targetRoot"`
	TargetParentIdentity string                       `json:"targetParentIdentity"`
	TargetDataBefore     string                       `json:"targetDataBefore,omitempty"`
	MaxEntries           int                          `json:"maxEntries"`
	MaxBytes             int64                        `json:"maxBytes"`
	Entries              []toolStorageRelocationEntry `json:"entries"`
	OldProfile           json.RawMessage              `json:"oldProfile,omitempty"`
	OldReceipt           json.RawMessage              `json:"oldReceipt,omitempty"`
	SourceReceiptID      string                       `json:"sourceReceiptId"`
	SourceReceiptSHA     string                       `json:"sourceReceiptSha256"`
	SourceProfileID      string                       `json:"sourceProfileId"`
	SourceAuthorPlanID   string                       `json:"sourceAuthorPlanId"`
	SourceAuthorPlanSHA  string                       `json:"sourceAuthorPlanSha256"`
	HomeIdentity         string                       `json:"homeIdentity"`
	InstallationID       string                       `json:"installationId"`
	DeviceID             string                       `json:"deviceId"`
	CopyID               string                       `json:"copyId"`
	NewProfile           json.RawMessage              `json:"newProfile,omitempty"`
	NewReceipt           json.RawMessage              `json:"newReceipt,omitempty"`
	Probe                *toolStorageTransaction      `json:"probe,omitempty"`
	ProbeCount           int                          `json:"probeCount,omitempty"`
	ProbeProofs          []string                     `json:"probeProofs,omitempty"`
	MetadataStage        string                       `json:"metadataStage,omitempty"`
	MetadataStageID      string                       `json:"metadataStageId,omitempty"`
	OldProfileID         string                       `json:"oldProfileId,omitempty"`
	OldReceiptID         string                       `json:"oldReceiptId,omitempty"`
	NewProfileID         string                       `json:"newProfileId,omitempty"`
	NewReceiptID         string                       `json:"newReceiptId,omitempty"`
	TargetIdentity       string                       `json:"targetIdentity,omitempty"`
	TargetDataIdentity   string                       `json:"targetDataIdentity,omitempty"`
	TargetFiles          map[string]string            `json:"targetFiles,omitempty"`
	Closed               bool                         `json:"closed,omitempty"`
	LockID               string                       `json:"lockId,omitempty"`
	PublicationStep      int                          `json:"publicationStep,omitempty"`
}
type toolStorageUseRecord struct {
	SchemaVersion   int                `json:"schemaVersion"`
	OperationID     string             `json:"operationId"`
	State           string             `json:"state"`
	ProfileSHA256   string             `json:"profileSha256"`
	UseRootIdentity string             `json:"useRootIdentity"`
	ProjectID       string             `json:"projectId"`
	ProjectIdentity string             `json:"projectIdentity"`
	TaskID          string             `json:"taskId"`
	TaskRevision    int64              `json:"taskRevision"`
	CopyID          string             `json:"copyId"`
	DeviceID        string             `json:"deviceId"`
	ProfileIdentity string             `json:"profileIdentity"`
	ReceiptProof    string             `json:"receiptProof"`
	NativeIdentity  string             `json:"nativeIdentity"`
	Stores          []toolStorageStore `json:"stores"`
}

func toolStorageUseAcquire(b *toolStorageBinding, operationID, projectRoot, projectID, taskID, copyID, deviceID string, taskRevision int64) error {
	if b == nil || b.policy.SchemaVersion != 2 || !isUUID(operationID) {
		return nil
	}
	c, code := resolveLayout(b.ref.Home, "installation")
	if code != "" {
		return errors.New("use")
	}
	var unlock func()
	var err error
	deadline := time.Now().Add(2 * time.Second)
	for {
		unlock, err = installationAdmission(c)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer unlock()
	if toolStoragePending(c) || updatePending(c) {
		return errors.New("transition")
	}
	if _, _, _, _, err := toolStorageRelocationProfile(c, b.ref.ToolID, b.ref.SHA256); err != nil {
		return err
	}
	if count, err := toolStorageRelocationUses(c, b.policy); err != nil || count >= toolStorageUseCap {
		return errors.New("use capacity")
	}
	root := toolStorageUseRoot(c, b.ref.ToolID)
	id, err := localNativeID(root)
	if err != nil || id != b.policy.UseRootIdentity {
		return errors.New("use root")
	}
	projectNative, err := localNativeID(projectRoot)
	if err != nil || projectID == "" || taskID == "" || copyID == "" || deviceID == "" {
		return errors.New("use provenance")
	}
	f, err := os.OpenFile(filepath.Join(root, operationID+".json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	leafID, err := localNativeIDFile(f)
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(toolStorageUseRecord{SchemaVersion: 1, OperationID: operationID, State: "active", ProfileSHA256: b.ref.SHA256, UseRootIdentity: id, ProjectID: projectID, ProjectIdentity: projectNative, TaskID: taskID, TaskRevision: taskRevision, CopyID: copyID, DeviceID: deviceID, ProfileIdentity: b.profileID, ReceiptProof: b.profileHash, NativeIdentity: leafID, Stores: b.policy.Stores})
	if _, err = f.Write(raw); err != nil {
		return err
	}
	return f.Sync()
}

func toolStorageUseVerify(b *toolStorageBinding, operationID, projectRoot string) error {
	if b == nil || b.policy.SchemaVersion != 2 {
		return nil
	}
	c, code := resolveLayout(b.ref.Home, "installation")
	if code != "" || toolStoragePending(c) {
		return errors.New("use")
	}
	root := toolStorageUseRoot(c, b.ref.ToolID)
	id, err := localNativeID(root)
	if err != nil || id != b.policy.UseRootIdentity {
		return errors.New("use root")
	}
	path := filepath.Join(root, operationID+".json")
	raw, err := installationRead(c.Root, toolStorageRel(c, path), toolStorageAuthoringLimit)
	if err != nil {
		return err
	}
	var u toolStorageUseRecord
	projectID, err := localNativeID(projectRoot)
	leafID, leafErr := localNativeID(path)
	if err != nil || leafErr != nil || !checkpointStrictJSON(raw, &u) || u.NativeIdentity != leafID || u.SchemaVersion != 1 || u.OperationID != operationID || u.State != "active" || u.UseRootIdentity != id || u.ProfileSHA256 != b.ref.SHA256 || u.ProfileIdentity != b.profileID || u.ReceiptProof != b.profileHash || u.ProjectIdentity != projectID || !reflect.DeepEqual(u.Stores, b.policy.Stores) {
		return errors.New("use proof")
	}
	db, err := openExisting(projectRoot)
	if err != nil {
		return err
	}
	defer db.Close()
	var project, copyID, device, taskID string
	var revision int64
	if db.QueryRow("SELECT p.id,p.copy_id,p.device_id,m.task_id,m.task_revision FROM project p JOIN operation_revision m ON m.operation_id=?", operationID).Scan(&project, &copyID, &device, &taskID, &revision) != nil || project != u.ProjectID || copyID != u.CopyID || device != u.DeviceID || taskID != u.TaskID || revision != u.TaskRevision {
		return errors.New("use provenance")
	}
	return nil
}

func toolStorageUseRelease(b *toolStorageBinding, operationID string) error {
	if b == nil || b.policy.SchemaVersion != 2 {
		return nil
	}
	c, code := resolveLayout(b.ref.Home, "installation")
	if code != "" {
		return errors.New("use")
	}
	root := toolStorageUseRoot(c, b.ref.ToolID)
	id, err := localNativeID(root)
	if err != nil || id != b.policy.UseRootIdentity {
		return errors.New("use root")
	}
	path := filepath.Join(root, operationID+".json")
	fileID, err := localNativeID(path)
	if err != nil {
		return err
	}
	raw, err := installationRead(c.Root, toolStorageRel(c, path), toolStorageAuthoringLimit)
	if err != nil {
		return err
	}
	var u toolStorageUseRecord
	if !checkpointStrictJSON(raw, &u) || u.NativeIdentity != fileID || u.OperationID != operationID || u.State != "active" || u.UseRootIdentity != id || u.ProfileSHA256 != b.ref.SHA256 || u.ProfileIdentity != b.profileID || u.ReceiptProof != b.profileHash {
		return errors.New("use proof")
	}
	if current, err := localNativeID(path); err != nil || current != fileID {
		return errors.New("use identity")
	}
	return os.Remove(path)
}

func toolStorageRelocationPlanPath(c layoutContext, tool string) string {
	return filepath.Join(toolStorageBase(c), "relocation", tool+".json")
}
func toolStorageRelocationStatePath(c layoutContext) string {
	return filepath.Join(toolStorageBase(c), "relocation.json")
}

func toolStorageRelocationTerminalValid(c layoutContext, p toolStorageRelocationPlan) bool {
	if p.SchemaVersion != 1 || !isUUID(p.PlanID) || !toolStorageHash(p.ApprovedPlanSHA) || (p.ToolID != "go" && p.ToolID != "npm") || !toolStorageHash(p.ProfileSHA256) || p.SourceReceiptID == "" || !toolStorageHash(p.SourceReceiptSHA) || p.SourceProfileID == "" || p.SourceAuthorPlanID == "" || !toolStorageHash(p.SourceAuthorPlanSHA) || p.HomeIdentity == "" || p.InstallationID == "" || p.DeviceID == "" || p.CopyID == "" || p.MetadataStageID == "" || p.TargetIdentity == "" || p.ProbeCount != 2 || p.PublicationStep != 4 || !toolStorageText(p.ConsentReference, 1024) {
		return false
	}
	if p.Phase != "active" && p.Phase != "rolled_back" || p.Phase == "active" && !p.Closed {
		return false
	}
	if len(p.ProbeProofs) != 2 || p.ProbeProofs[0] == "" || p.ProbeProofs[1] == "" {
		return false
	}
	if p.MetadataStage != filepath.Join(c.Runs, "relocation-"+p.PlanID) || !filepath.IsAbs(p.SourceRoot) || !filepath.IsAbs(p.TargetRoot) || environmentPlainAncestors(p.MetadataStage) != nil {
		return false
	}
	approved, _, err := toolStorageRelocationReadPlan(c, toolStorageRelocationApproval{ToolID: p.ToolID, PlanID: p.PlanID, PlanSHA256: p.ApprovedPlanSHA})
	if err != nil || approved.SourceRoot != p.SourceRoot || approved.SourceIdentity != p.SourceIdentity || approved.TargetRoot != p.TargetRoot || approved.TargetParentIdentity != p.TargetParentIdentity || approved.TargetDataBefore != p.TargetDataBefore || approved.SourceReceiptID != p.SourceReceiptID || approved.SourceReceiptSHA != p.SourceReceiptSHA || approved.SourceProfileID != p.SourceProfileID || approved.SourceAuthorPlanID != p.SourceAuthorPlanID || approved.SourceAuthorPlanSHA != p.SourceAuthorPlanSHA || approved.HomeIdentity != p.HomeIdentity || approved.InstallationID != p.InstallationID || approved.DeviceID != p.DeviceID || approved.CopyID != p.CopyID || approved.ProfileSHA256 != p.ProfileSHA256 || approved.MaxEntries != p.MaxEntries || approved.MaxBytes != p.MaxBytes || !reflect.DeepEqual(approved.Entries, p.Entries) || !bytes.Equal(approved.OldProfile, p.OldProfile) || !bytes.Equal(approved.OldReceipt, p.OldReceipt) {
		return false
	}
	home, err := readInstallationReceiptContext(c)
	if err != nil || home.HomeID != p.HomeIdentity || home.InstallationID != p.InstallationID || home.DeviceID != p.DeviceID || home.CopyID != p.CopyID {
		return false
	}
	if id, err := localNativeID(p.MetadataStage); err != nil || id != p.MetadataStageID {
		return false
	}
	if id, err := localNativeID(p.TargetRoot); err != nil || id != p.TargetIdentity {
		return false
	}
	profileID, receiptID, profile, receipt := p.NewProfileID, p.NewReceiptID, p.NewProfile, p.NewReceipt
	if p.Phase == "rolled_back" {
		profileID, receiptID, profile, receipt = p.OldProfileID, p.OldReceiptID, p.OldProfile, p.OldReceipt
	}
	profilePath := filepath.Join(toolStorageBase(c), p.ToolID+".json")
	receiptPath := toolStorageReceiptPath(c, p.ToolID)
	if id, err := localNativeID(profilePath); err != nil || id != profileID {
		return false
	}
	if id, err := localNativeID(receiptPath); err != nil || id != receiptID {
		return false
	}
	profileRaw, err := installationRead(c.Root, toolStorageRel(c, profilePath), toolStorageLimit)
	if err != nil || !bytes.Equal(profileRaw, profile) {
		return false
	}
	receiptRaw, err := installationRead(c.Root, toolStorageRel(c, receiptPath), toolStorageAuthoringLimit)
	return err == nil && bytes.Equal(receiptRaw, receipt)
}
func toolStorageUseRoot(c layoutContext, tool string) string {
	return filepath.Join(toolStorageBase(c), "uses", tool)
}
func toolStorageRelocationHook(phase string) { toolStorageAuthoringHook("relocate." + phase) }

func toolStorageRelocationFailure(reason, stage string) Response {
	return installationResponse("TOOL_STORAGE_REFUSED", false, map[string]any{"reason": reason, "stage": stage, "nextAction": "inspect retained relocation state"})
}

func toolStorageRelocation(c layoutContext, home installationReceipt, r Request) Response {
	if r.RootKind != "installation" || r.Actor != "coordinator" || r.ExpectedRevision != 0 {
		return toolStorageRelocationFailure("authority", "admission")
	}
	switch r.Action {
	case "tool-storage.relocate.prepare":
		return toolStorageRelocationPrepare(c, home, r)
	case "tool-storage.relocate.apply", "tool-storage.relocate.recover", "tool-storage.relocate.rollback":
		return toolStorageRelocationApply(c, home, r)
	case "tool-storage.relocate.inspect":
		return toolStorageRelocationInspect(c, r)
	default:
		return bad("UNKNOWN_ACTION", 0)
	}
}

func toolStorageRelocationExact(raw json.RawMessage, target any, keys ...string) bool {
	return toolStorageExact(raw, target, keys...)
}
func toolStorageRelocationDigest(raw []byte, id string) string {
	h := sha256.Sum256(append(append([]byte{}, raw...), []byte(":"+id)...))
	return hex.EncodeToString(h[:])
}

func toolStorageRelocationProfile(c layoutContext, tool, want string) (toolStoragePolicy, []byte, string, toolStorageReceipt, error) {
	if tool != "go" && tool != "npm" || !toolStorageHash(want) {
		return toolStoragePolicy{}, nil, "", toolStorageReceipt{}, errors.New("tool")
	}
	path := filepath.Join(toolStorageBase(c), tool+".json")
	raw, err := installationRead(c.Root, toolStorageRel(c, path), toolStorageLimit)
	if err != nil {
		return toolStoragePolicy{}, nil, "", toolStorageReceipt{}, err
	}
	h := sha256.Sum256(raw)
	if hex.EncodeToString(h[:]) != want {
		return toolStoragePolicy{}, nil, "", toolStorageReceipt{}, errors.New("hash")
	}
	id, err := localNativeID(path)
	if err != nil {
		return toolStoragePolicy{}, nil, "", toolStorageReceipt{}, err
	}
	var p toolStoragePolicy
	if !toolStoragePolicyShape(raw) || !checkpointStrictJSON(raw, &p) || p.SchemaVersion != 2 || !toolStoragePolicyValid(c.Root, toolStorageRef{Home: c.Root, ToolID: tool, SHA256: want}, p) {
		return toolStoragePolicy{}, nil, "", toolStorageReceipt{}, errors.New("profile")
	}
	rx, err := toolStorageReadReceipt(c, tool)
	if err != nil || rx.ProfileSHA256 != want || rx.ProfileNativeIdentity != id || rx.UseRootIdentity != p.UseRootIdentity {
		return toolStoragePolicy{}, nil, "", toolStorageReceipt{}, errors.New("receipt")
	}
	return p, raw, id, rx, nil
}

func toolStorageRelocationInventory(root string, maxEntries int, maxBytes int64) ([]toolStorageRelocationEntry, error) {
	if maxEntries < 1 || maxEntries > 1024 || maxBytes < 1 || maxBytes > toolStorageRelocationLimit {
		return nil, errors.New("limits")
	}
	entries := []toolStorageRelocationEntry{}
	total := int64(0)
	count := 0
	held, rootID, err := openHeldResourceRoot(root, ".", "")
	if err != nil {
		return nil, err
	}
	defer held.Close()
	seen := map[string]bool{}
	var walk func(string) error
	walk = func(dir string) error {
		f, err := held.Open(dir)
		if err != nil {
			return err
		}
		defer f.Close()
		for {
			batch, readErr := f.ReadDir(1)
			if readErr == io.EOF {
				return nil
			}
			if readErr != nil {
				return readErr
			}
			d := batch[0]
			rel := filepath.Join(dir, d.Name())
			path := filepath.Join(root, rel)
			fold := strings.ToLower(filepath.ToSlash(rel))
			if seen[fold] {
				return errors.New("case alias")
			}
			seen[fold] = true
			count++
			if count > maxEntries {
				return errors.New("limit")
			}
			if d.Type()&os.ModeSymlink != 0 || d.Type()&os.ModeIrregular != 0 {
				return errors.New("link")
			}
			if d.IsDir() {
				info, e := held.Lstat(rel)
				if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
					return errors.New("directory")
				}
				id, e := localNativeID(path)
				if e != nil {
					return e
				}
				child, e := held.Open(rel)
				if e != nil {
					return e
				}
				opened, e := localNativeIDFile(child)
				child.Close()
				if e != nil || id != opened {
					return errors.New("directory identity")
				}
				entries = append(entries, toolStorageRelocationEntry{Path: filepath.ToSlash(rel), Directory: true, NativeIdentity: id, Mode: uint32(info.Mode().Perm())})
				if e = walk(rel); e != nil {
					return e
				}
				after, e := localNativeID(path)
				if e != nil || after != id {
					return errors.New("directory drift")
				}
				continue
			}
			if !d.Type().IsRegular() {
				return errors.New("type")
			}
			rel = filepath.ToSlash(rel)
			info, err := held.Lstat(filepath.FromSlash(rel))
			if err != nil || info.Size() < 0 {
				return errors.New("info")
			}
			total += info.Size()
			if total > maxBytes {
				return errors.New("limit")
			}
			id, err := localNativeID(path)
			if err != nil {
				return err
			}
			f, err := held.Open(filepath.FromSlash(rel))
			if err != nil {
				return err
			}
			opened, err := localNativeIDFile(f)
			if err != nil || opened != id {
				f.Close()
				return errors.New("identity")
			}
			h := sha256.New()
			n, err := io.Copy(h, io.LimitReader(f, maxBytes+1))
			closeErr := f.Close()
			if err != nil || closeErr != nil || n != info.Size() {
				return errors.New("read")
			}
			after, err := localNativeID(path)
			if err != nil || after != id {
				return errors.New("drift")
			}
			entries = append(entries, toolStorageRelocationEntry{Path: rel, NativeIdentity: id, Size: n, SHA256: hex.EncodeToString(h.Sum(nil)), Mode: uint32(info.Mode().Perm())})
		}
	}
	err = walk(".")
	if err != nil {
		return nil, err
	}
	if id, e := localNativeID(root); e != nil || id != rootID {
		return nil, errors.New("root drift")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func toolStorageRelocationUses(c layoutContext, p toolStoragePolicy) (int, error) {
	root := toolStorageUseRoot(c, p.ToolID)
	id, err := localNativeID(root)
	if err != nil || id != p.UseRootIdentity {
		return 0, errors.New("use root")
	}
	dir, err := os.Open(root)
	if err != nil {
		return 0, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(toolStorageUseCap + 1)
	if err != nil && err != io.EOF {
		return 0, err
	}
	if len(entries) > toolStorageUseCap {
		return 0, errors.New("use overflow")
	}
	active := 0
	for _, e := range entries {
		if e.IsDir() || e.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(e.Name(), ".json") || !isUUID(strings.TrimSuffix(e.Name(), ".json")) {
			return 0, errors.New("use entry")
		}
		path := filepath.Join(root, e.Name())
		raw, err := installationRead(c.Root, toolStorageRel(c, path), toolStorageAuthoringLimit)
		if err != nil {
			return 0, err
		}
		var u toolStorageUseRecord
		leafID, leafErr := localNativeID(path)
		if leafErr != nil || !checkpointStrictJSON(raw, &u) || u.NativeIdentity != leafID || u.SchemaVersion != 1 || u.OperationID != strings.TrimSuffix(e.Name(), ".json") || u.State != "active" || u.UseRootIdentity != id || !toolStorageHash(u.ProfileSHA256) || u.ProjectID == "" || u.ProjectIdentity == "" || u.TaskID == "" || u.TaskRevision < 1 || u.CopyID == "" || u.DeviceID == "" || u.ProfileIdentity == "" || u.ReceiptProof == "" || len(u.Stores) == 0 {
			return 0, errors.New("use proof")
		}
		active++
	}
	return active, nil
}

func toolStorageRelocationPrepare(c layoutContext, home installationReceipt, r Request) Response {
	var in toolStorageRelocationInput
	if !toolStorageRelocationExact(r.Payload, &in, "toolId", "profileSha256", "toolDataRoot", "maxEntries", "maxBytes") || in.ToolID != "go" && in.ToolID != "npm" || !toolStorageHash(in.ProfileSHA256) || !filepath.IsAbs(in.ToolDataRoot) || in.MaxEntries < 1 || in.MaxEntries > 1024 || in.MaxBytes < 1 || in.MaxBytes > toolStorageRelocationLimit {
		return toolStorageRelocationFailure("input", "preflight")
	}
	if toolStorageStopped(c) {
		return bad("TOOL_STORAGE_STOPPED", 0)
	}
	if unlock, err := installationAdmission(c); err != nil {
		return bad("TOOL_STORAGE_BUSY", 0)
	} else {
		defer unlock()
	}
	writer, err := acquireUpdateRecoveryWriter(c)
	if err != nil {
		return bad("TOOL_STORAGE_BUSY", 0)
	}
	defer writer.Release()
	if toolStoragePending(c) || updatePending(c) {
		return bad("TOOL_STORAGE_PENDING", 0)
	}
	p, oldProfile, sourceProfileID, oldReceipt, err := toolStorageRelocationProfile(c, in.ToolID, in.ProfileSHA256)
	if err != nil {
		return toolStorageRelocationFailure("profile", "preflight")
	}
	if toolStorageActiveCurrent(c, oldReceipt) != nil {
		return toolStorageRelocationFailure("lineage", "preflight")
	}
	receiptRaw, err := installationRead(c.Root, toolStorageRel(c, toolStorageReceiptPath(c, in.ToolID)), toolStorageAuthoringLimit)
	if err != nil {
		return toolStorageRelocationFailure("receipt", "preflight")
	}
	receiptID, err := localNativeID(toolStorageReceiptPath(c, in.ToolID))
	if err != nil {
		return toolStorageRelocationFailure("receipt", "preflight")
	}
	authorRaw, err := installationRead(c.Root, toolStorageRel(c, toolStoragePlanPath(c, in.ToolID)), toolStorageAuthoringLimit)
	if err != nil {
		return toolStorageRelocationFailure("plan", "preflight")
	}
	authorID, err := localNativeID(toolStoragePlanPath(c, in.ToolID))
	if err != nil {
		return toolStorageRelocationFailure("plan", "preflight")
	}
	if uses, err := toolStorageRelocationUses(c, p); err != nil {
		return toolStorageRelocationFailure("use_registry", "preflight")
	} else if uses != 0 {
		return installationResponse("TOOL_STORAGE_IN_USE", false, map[string]any{"reason": "active_uses", "stage": "preflight", "activeUses": uses, "nextAction": "wait for verified operation closure"})
	}
	parent := filepath.Dir(in.ToolDataRoot)
	if plainPath(parent, true) != nil || environmentPlainAncestors(parent) != nil || environmentWithin(c.Root, in.ToolDataRoot) || environmentWithin(in.ToolDataRoot, c.Root) || environmentWithin(p.ToolDataRoot, in.ToolDataRoot) || environmentWithin(in.ToolDataRoot, p.ToolDataRoot) {
		return toolStorageRelocationFailure("destination", "preflight")
	}
	parentID, err := localNativeID(parent)
	if err != nil {
		return toolStorageRelocationFailure("destination", "preflight")
	}
	target := filepath.Join(in.ToolDataRoot, in.ToolID)
	if _, err = os.Lstat(target); !os.IsNotExist(err) {
		return toolStorageRelocationFailure("destination", "preflight")
	}
	targetDataBefore := ""
	if info, e := os.Lstat(in.ToolDataRoot); e == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || environmentPlainAncestors(in.ToolDataRoot) != nil {
			return toolStorageRelocationFailure("destination", "preflight")
		}
		targetDataBefore, e = localNativeID(in.ToolDataRoot)
		if e != nil {
			return toolStorageRelocationFailure("destination", "preflight")
		}
	} else if !os.IsNotExist(e) {
		return toolStorageRelocationFailure("destination", "preflight")
	}
	source := filepath.Join(p.ToolDataRoot, p.ToolID)
	sourceID, err := localNativeID(source)
	if err != nil {
		return toolStorageRelocationFailure("source", "preflight")
	}
	entries, err := toolStorageRelocationInventory(source, in.MaxEntries, in.MaxBytes)
	if err != nil {
		return toolStorageRelocationFailure("inventory", "preflight")
	}
	plan := toolStorageRelocationPlan{SchemaVersion: 1, Phase: "prepared", PlanID: uuid.NewString(), ToolID: in.ToolID, ProfileSHA256: in.ProfileSHA256, SourceRoot: source, SourceIdentity: sourceID, TargetRoot: target, TargetParentIdentity: parentID, TargetDataBefore: targetDataBefore, MaxEntries: in.MaxEntries, MaxBytes: in.MaxBytes, Entries: entries, OldProfile: oldProfile}
	plan.OldReceipt = receiptRaw
	plan.SourceReceiptID, plan.SourceReceiptSHA, plan.SourceProfileID = receiptID, updateDigest(receiptRaw), sourceProfileID
	plan.SourceAuthorPlanID, plan.SourceAuthorPlanSHA = authorID, updateDigest(authorRaw)
	plan.HomeIdentity, plan.InstallationID, plan.DeviceID, plan.CopyID = home.HomeID, home.InstallationID, home.DeviceID, home.CopyID
	raw, _ := json.Marshal(plan)
	path := toolStorageRelocationPlanPath(c, in.ToolID)
	if existing, readErr := installationRead(c.Root, toolStorageRel(c, path), toolStorageRelocationLimit); readErr == nil {
		var prior toolStorageRelocationPlan
		if !checkpointStrictJSON(existing, &prior) {
			return toolStorageRelocationFailure("plan", "preflight")
		}
		candidate := plan
		candidate.PlanID = prior.PlanID
		if !reflect.DeepEqual(candidate, prior) {
			return toolStorageRelocationFailure("plan_conflict", "preflight")
		}
		id, e := localNativeID(path)
		if e != nil {
			return toolStorageRelocationFailure("plan", "preflight")
		}
		return installationResponse("TOOL_STORAGE_RELOCATION_PREPARED", true, map[string]any{"phase": "prepared", "toolId": in.ToolID, "planId": prior.PlanID, "planSha256": toolStorageRelocationDigest(existing, id), "nextAction": "confirm current owner approval"})
	} else if !os.IsNotExist(readErr) {
		return toolStorageRelocationFailure("plan", "preflight")
	}
	if err = installationWrite(c.Root, toolStorageRel(c, path), raw); err != nil {
		return toolStorageRelocationFailure("plan", "preflight")
	}
	id, err := localNativeID(path)
	if err != nil {
		return toolStorageRelocationFailure("plan", "preflight")
	}
	return installationResponse("TOOL_STORAGE_RELOCATION_PREPARED", true, map[string]any{"phase": "prepared", "toolId": in.ToolID, "planId": plan.PlanID, "planSha256": toolStorageRelocationDigest(raw, id), "nextAction": "confirm current owner approval"})
}

func toolStorageRelocationReadPlan(c layoutContext, a toolStorageRelocationApproval) (toolStorageRelocationPlan, []byte, error) {
	path := toolStorageRelocationPlanPath(c, a.ToolID)
	raw, err := installationRead(c.Root, toolStorageRel(c, path), toolStorageRelocationLimit)
	if err != nil {
		return toolStorageRelocationPlan{}, nil, err
	}
	var p toolStorageRelocationPlan
	if !checkpointStrictJSON(raw, &p) || p.SchemaVersion != 1 || p.ToolID != a.ToolID || p.PlanID != a.PlanID {
		return p, nil, errors.New("plan")
	}
	id, err := localNativeID(path)
	if err != nil || toolStorageRelocationDigest(raw, id) != a.PlanSHA256 {
		return p, nil, errors.New("plan")
	}
	return p, raw, nil
}

func toolStorageRelocationCopy(c layoutContext, p *toolStorageRelocationPlan) error {
	source, sourceID, err := openHeldResourceRoot(p.SourceRoot, ".", p.SourceIdentity)
	if err != nil || sourceID != p.SourceIdentity {
		return errors.New("source root")
	}
	defer source.Close()
	if _, err := os.Stat(p.TargetRoot); !os.IsNotExist(err) {
		return errors.New("target")
	}
	dataRoot := filepath.Dir(p.TargetRoot)
	parent := filepath.Dir(dataRoot)
	record := func() error {
		raw, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return installationWrite(c.Root, toolStorageRel(c, toolStorageRelocationStatePath(c)), raw)
	}
	if parentID, err := localNativeID(parent); err != nil || parentID != p.TargetParentIdentity || environmentPlainAncestors(parent) != nil {
		return errors.New("destination parent")
	}
	if info, err := os.Lstat(dataRoot); os.IsNotExist(err) {
		if p.TargetDataBefore != "" {
			return errors.New("destination root missing")
		}
		if err = os.Mkdir(dataRoot, 0700); err != nil {
			return err
		}
	} else if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || environmentPlainAncestors(dataRoot) != nil {
		return errors.New("destination root")
	} else if id, err := localNativeID(dataRoot); err != nil || p.TargetDataBefore == "" || id != p.TargetDataBefore {
		return errors.New("destination root changed")
	}
	p.TargetDataIdentity, err = localNativeID(dataRoot)
	if err != nil {
		return err
	}
	if err = record(); err != nil {
		return err
	}
	if err := os.Mkdir(p.TargetRoot, 0700); err != nil {
		return err
	}
	target, targetID, err := openHeldResourceRoot(p.TargetRoot, ".", "")
	if err != nil {
		return err
	}
	defer target.Close()
	p.TargetIdentity = targetID
	p.TargetFiles = map[string]string{}
	if err := record(); err != nil {
		return err
	}
	for _, e := range p.Entries {
		src := filepath.Join(p.SourceRoot, filepath.FromSlash(e.Path))
		id, err := localNativeID(src)
		if err != nil || id != e.NativeIdentity {
			return errors.New("source drift")
		}
		if e.Directory {
			if err := target.Mkdir(filepath.FromSlash(e.Path), os.FileMode(e.Mode)); err != nil {
				return err
			}
			did, err := localNativeID(filepath.Join(p.TargetRoot, filepath.FromSlash(e.Path)))
			if err != nil || did == id {
				return errors.New("target directory identity")
			}
			p.TargetFiles[e.Path] = did
			if err := record(); err != nil {
				return err
			}
			continue
		}
		in, err := source.Open(filepath.FromSlash(e.Path))
		if err != nil {
			return err
		}
		opened, err := localNativeIDFile(in)
		if err != nil || opened != id {
			in.Close()
			return errors.New("source identity")
		}
		dst := filepath.Join(p.TargetRoot, filepath.FromSlash(e.Path))
		f, err := target.OpenFile(filepath.FromSlash(e.Path), os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(e.Mode))
		if err != nil {
			in.Close()
			return err
		}
		did, err := localNativeIDFile(f)
		if err != nil || did == id {
			in.Close()
			f.Close()
			return errors.New("target identity")
		}
		p.TargetFiles[e.Path] = did
		if err := record(); err != nil {
			in.Close()
			f.Close()
			return err
		}
		h := sha256.New()
		n, werr := io.Copy(io.MultiWriter(f, h), io.LimitReader(in, e.Size+1))
		inErr := in.Close()
		serr := f.Sync()
		cerr := f.Close()
		if werr != nil || inErr != nil || serr != nil || cerr != nil || n != e.Size || hex.EncodeToString(h.Sum(nil)) != e.SHA256 {
			return errors.New("copy")
		}
		if err = os.Chmod(dst, os.FileMode(e.Mode)); err != nil {
			return err
		}
		if after, err := localNativeID(dst); err != nil || after != did {
			return errors.New("target identity")
		}
		if after, err := localNativeID(src); err != nil || after != id {
			return errors.New("source drift")
		}
	}
	if id, err := localNativeID(p.TargetRoot); err != nil || id != targetID {
		return errors.New("target root drift")
	}
	return nil
}

func toolStorageRelocationApply(c layoutContext, home installationReceipt, r Request) Response {
	var a toolStorageRelocationApproval
	if !toolStorageRelocationExact(r.Payload, &a, "toolId", "planId", "planSha256", "consentReference") || !toolStorageSlug.MatchString(a.ToolID) || !isUUID(a.PlanID) || !toolStorageHash(a.PlanSHA256) || !toolStorageText(a.ConsentReference, 1024) {
		return toolStorageRelocationFailure("approval", "admission")
	}
	p, _, err := toolStorageRelocationReadPlan(c, a)
	if err != nil {
		return toolStorageRelocationFailure("approval_or_plan", "admission")
	}
	statePath := toolStorageRelocationStatePath(c)
	var retained toolStorageRelocationPlan
	if r.Action == "tool-storage.relocate.recover" {
		raw, e := installationRead(c.Root, toolStorageRel(c, statePath), toolStorageRelocationLimit)
		if e != nil || !checkpointStrictJSON(raw, &retained) || retained.PlanID != p.PlanID || retained.ApprovedPlanSHA != a.PlanSHA256 || retained.ToolID != a.ToolID || retained.ConsentReference != a.ConsentReference || retained.LockID == "" {
			return toolStorageRelocationFailure("approval_or_state", "recovery")
		}
		proof := retained
		proof.Closed = true
		if retained.Phase != "active" || !toolStorageRelocationTerminalValid(c, proof) {
			return toolStorageRelocationFailure("unproved_state", "recovery")
		}
		if current, e := toolStorageRelocationInventory(retained.SourceRoot, retained.MaxEntries, retained.MaxBytes); e != nil || !toolStorageRelocationSameEntries(current, retained.Entries) {
			return toolStorageRelocationFailure("source_drift", "recovery")
		}
		if current, e := toolStorageRelocationInventory(retained.TargetRoot, retained.MaxEntries, retained.MaxBytes); e != nil || !toolStorageRelocationSameContent(current, retained.Entries) || !toolStorageRelocationTargetIdentities(current, retained.TargetFiles) {
			return toolStorageRelocationFailure("target_drift", "recovery")
		}
		writer, e := acquireUpdateRecoveryWriter(c)
		if e != nil {
			return bad("TOOL_STORAGE_BUSY", 0)
		}
		oldLock := ownedSetupLock{path: filepath.Join(c.Root, filepath.FromSlash(c.Paths["bindings"]), ".setup-lock"), nativeID: retained.LockID}
		e = oldLock.Release()
		writer.Release()
		if e != nil {
			return toolStorageRelocationFailure("admission", "recovery")
		}
	}
	lock, err := setupLockOwned(filepath.Join(c.Root, filepath.FromSlash(c.Paths["bindings"])))
	if err != nil {
		return bad("TOOL_STORAGE_BUSY", 0)
	}
	defer lock.Release()
	writer, err := acquireUpdateRecoveryWriter(c)
	if err != nil {
		return bad("TOOL_STORAGE_BUSY", 0)
	}
	defer writer.Release()
	if r.Action == "tool-storage.relocate.recover" {
		raw, err := installationRead(c.Root, toolStorageRel(c, statePath), toolStorageRelocationLimit)
		if err != nil {
			return toolStorageRelocationFailure("state", "recovery")
		}
		var saved toolStorageRelocationPlan
		if !checkpointStrictJSON(raw, &saved) || saved.PlanID != p.PlanID || saved.ToolID != a.ToolID || saved.ConsentReference != a.ConsentReference || saved.Phase != "active" {
			return toolStorageRelocationFailure("state", "recovery")
		}
		profileID, e1 := localNativeID(filepath.Join(toolStorageBase(c), a.ToolID+".json"))
		receiptID, e2 := localNativeID(toolStorageReceiptPath(c, a.ToolID))
		stageID, e3 := localNativeID(saved.MetadataStage)
		if e1 != nil || e2 != nil || e3 != nil || profileID != saved.NewProfileID || receiptID != saved.NewReceiptID || stageID != saved.MetadataStageID {
			return toolStorageRelocationFailure("identity", "recovery")
		}
		profileRaw, e1 := installationRead(c.Root, toolStorageRel(c, filepath.Join(toolStorageBase(c), a.ToolID+".json")), toolStorageLimit)
		receiptRaw, e2 := installationRead(c.Root, toolStorageRel(c, toolStorageReceiptPath(c, a.ToolID)), toolStorageAuthoringLimit)
		if e1 != nil || e2 != nil || !bytes.Equal(profileRaw, saved.NewProfile) || !bytes.Equal(receiptRaw, saved.NewReceipt) || saved.PublicationStep != 4 || saved.ProbeCount != 2 || len(saved.ProbeProofs) != 2 || saved.ProbeProofs[0] == "" || saved.ProbeProofs[1] == "" {
			return toolStorageRelocationFailure("proof", "recovery")
		}
		if current, e := toolStorageRelocationInventory(saved.SourceRoot, saved.MaxEntries, saved.MaxBytes); e != nil || !toolStorageRelocationSameEntries(current, saved.Entries) {
			return toolStorageRelocationFailure("source_drift", "recovery")
		}
		if current, e := toolStorageRelocationInventory(saved.TargetRoot, saved.MaxEntries, saved.MaxBytes); e != nil || !toolStorageRelocationSameContent(current, saved.Entries) || !toolStorageRelocationTargetIdentities(current, saved.TargetFiles) {
			return toolStorageRelocationFailure("target_drift", "recovery")
		}
		saved.Closed = true
		closedRaw, err := json.Marshal(saved)
		if err != nil || installationWrite(c.Root, toolStorageRel(c, statePath), closedRaw) != nil {
			return toolStorageRelocationFailure("state", "recovery")
		}
		return installationResponse("TOOL_STORAGE_ACTIVE", true, map[string]any{"phase": "active"})
	}
	if r.Action == "tool-storage.relocate.rollback" {
		return toolStorageRelocationRollback(c, p, a)
	}
	if toolStorageStopped(c) || toolStoragePending(c) {
		return bad("TOOL_STORAGE_PENDING", 0)
	}
	policy, _, _, _, err := toolStorageRelocationProfile(c, a.ToolID, p.ProfileSHA256)
	if err != nil {
		return toolStorageRelocationFailure("profile", "admission")
	}
	if p.HomeIdentity != home.HomeID || p.InstallationID != home.InstallationID || p.DeviceID != home.DeviceID || p.CopyID != home.CopyID {
		return toolStorageRelocationFailure("home_drift", "preflight")
	}
	for _, check := range []struct {
		path, id, sha string
		limit         int
	}{{filepath.Join(toolStorageBase(c), a.ToolID+".json"), p.SourceProfileID, p.ProfileSHA256, toolStorageLimit}, {toolStorageReceiptPath(c, a.ToolID), p.SourceReceiptID, p.SourceReceiptSHA, toolStorageAuthoringLimit}, {toolStoragePlanPath(c, a.ToolID), p.SourceAuthorPlanID, p.SourceAuthorPlanSHA, toolStorageAuthoringLimit}} {
		id, e := localNativeID(check.path)
		raw, x := installationRead(c.Root, toolStorageRel(c, check.path), check.limit)
		if e != nil || x != nil || id != check.id || updateDigest(raw) != check.sha {
			return toolStorageRelocationFailure("lineage_drift", "preflight")
		}
	}
	if receipt, e := toolStorageReadReceipt(c, a.ToolID); e != nil || toolStorageActiveCurrent(c, receipt) != nil {
		return toolStorageRelocationFailure("lineage_drift", "preflight")
	}
	if uses, err := toolStorageRelocationUses(c, policy); err != nil || uses != 0 {
		return toolStorageRelocationFailure("use_registry", "admission")
	}
	current, err := toolStorageRelocationInventory(p.SourceRoot, p.MaxEntries, p.MaxBytes)
	if err != nil || !toolStorageRelocationSameEntries(current, p.Entries) {
		return toolStorageRelocationFailure("source_drift", "preflight")
	}
	dataRoot := filepath.Dir(p.TargetRoot)
	if id, e := localNativeID(filepath.Dir(dataRoot)); e != nil || id != p.TargetParentIdentity {
		return toolStorageRelocationFailure("destination_drift", "preflight")
	}
	if id, e := localNativeID(dataRoot); p.TargetDataBefore == "" {
		if !os.IsNotExist(e) {
			return toolStorageRelocationFailure("destination_drift", "preflight")
		}
	} else if e != nil || id != p.TargetDataBefore {
		return toolStorageRelocationFailure("destination_drift", "preflight")
	}
	if _, e := os.Lstat(p.TargetRoot); !os.IsNotExist(e) {
		return toolStorageRelocationFailure("destination_drift", "preflight")
	}
	p.ConsentReference = a.ConsentReference
	p.ApprovedPlanSHA = a.PlanSHA256
	p.LockID = lock.nativeID
	p.Phase = "intent"
	raw, _ := json.Marshal(p)
	if err := installationWrite(c.Root, toolStorageRel(c, statePath), raw); err != nil {
		return toolStorageRelocationFailure("state", "intent")
	}
	toolStorageRelocationHook("intent")
	if err := toolStorageRelocationCopy(c, &p); err != nil {
		return toolStorageRelocationFailure("copy", "copy")
	}
	p.Phase = "copied"
	raw, _ = json.Marshal(p)
	if err := installationWrite(c.Root, toolStorageRel(c, statePath), raw); err != nil {
		return toolStorageRelocationFailure("state", "copy")
	}
	toolStorageRelocationHook("copied")
	newPolicy := policy
	newPolicy.ToolDataRoot = filepath.Dir(p.TargetRoot)
	newPolicy.ToolDataIdentity, _ = localNativeID(newPolicy.ToolDataRoot)
	for i := range newPolicy.Stores {
		path := filepath.Join(newPolicy.ToolDataRoot, filepath.FromSlash(newPolicy.Stores[i].Path))
		if err := plainPath(path, true); err != nil {
			return toolStorageRelocationFailure("copy", "stores")
		}
		newPolicy.Stores[i].NativeIdentity, err = localNativeID(path)
		if err != nil {
			return toolStorageRelocationFailure("copy", "stores")
		}
	}
	planRaw, err := installationRead(c.Root, toolStorageRel(c, toolStoragePlanPath(c, a.ToolID)), toolStorageAuthoringLimit)
	var authorPlan toolStoragePlan
	if err != nil || !checkpointStrictJSON(planRaw, &authorPlan) || authorPlan.ToolID != a.ToolID {
		return toolStorageRelocationFailure("provider_plan", "probe")
	}
	if got, id, sum, err := environmentExecutable(c.Root, authorPlan.Executable); err != nil || got != authorPlan.Executable || id != authorPlan.ExecutableIdentity || sum != authorPlan.ExecutableSHA {
		return toolStorageRelocationFailure("provider_changed", "probe")
	}
	if a.ToolID == "npm" {
		if id, sum, err := toolStorageIdentityFile(authorPlan.Script); err != nil || id != authorPlan.ScriptIdentity || sum != authorPlan.ScriptSHA {
			return toolStorageRelocationFailure("provider_changed", "probe")
		}
	}
	authorPlan.ToolDataRoot = newPolicy.ToolDataRoot
	for i := 0; i < 2; i++ {
		authorPlan.PlanID = uuid.NewString()
		probe := &toolStorageTransaction{SchemaVersion: 1, ToolID: a.ToolID, PlanID: authorPlan.PlanID, PlanSHA256: a.PlanSHA256}
		p.Probe = probe
		record := func() error {
			b, err := json.Marshal(p)
			if err != nil {
				return err
			}
			return installationWrite(c.Root, toolStorageRel(c, statePath), b)
		}
		if err := record(); err != nil {
			return toolStorageRelocationFailure("state", "probe")
		}
		if err := toolStorageProbeRecorded(c, authorPlan, newPolicy.Stores, probe, record); err != nil {
			return toolStorageRelocationFailure("provider_probe_failed", "probe")
		}
		if !strings.HasPrefix(probe.ProbeState, "stopped:") || probe.StageID == "" {
			return toolStorageRelocationFailure("provider_probe_failed", "probe")
		}
		p.ProbeProofs = append(p.ProbeProofs, authorPlan.PlanID+":"+probe.StageID+":"+probe.ProbeState)
		p.ProbeCount = i + 1
		if err := record(); err != nil {
			return toolStorageRelocationFailure("state", "probe")
		}
	}
	if current, err := toolStorageRelocationInventory(p.SourceRoot, p.MaxEntries, p.MaxBytes); err != nil || !toolStorageRelocationSameEntries(current, p.Entries) {
		return toolStorageRelocationFailure("source_drift", "stage")
	}
	if current, err := toolStorageRelocationInventory(p.TargetRoot, p.MaxEntries, p.MaxBytes); err != nil || !toolStorageRelocationSameContent(current, p.Entries) || !toolStorageRelocationTargetIdentities(current, p.TargetFiles) {
		return toolStorageRelocationFailure("target_drift", "stage")
	}
	newProfile, _ := json.Marshal(newPolicy)
	ph := sha256.Sum256(newProfile)
	newReceipt := toolStorageReceipt{}
	_ = json.Unmarshal(p.OldReceipt, &newReceipt)
	newReceipt.ToolDataRoot = newPolicy.ToolDataRoot
	newReceipt.ToolDataIdentity = newPolicy.ToolDataIdentity
	newReceipt.Stores = newPolicy.Stores
	newReceipt.ProfileSHA256 = hex.EncodeToString(ph[:])
	p.NewProfile = newProfile
	profilePath := filepath.Join(toolStorageBase(c), a.ToolID+".json")
	receiptPath := toolStorageReceiptPath(c, a.ToolID)
	p.OldProfileID, err = localNativeID(profilePath)
	if err != nil {
		return toolStorageRelocationFailure("publication", "stage")
	}
	p.OldReceiptID, err = localNativeID(receiptPath)
	if err != nil {
		return toolStorageRelocationFailure("publication", "stage")
	}
	p.MetadataStage = filepath.Join(c.Runs, "relocation-"+p.PlanID)
	if err := os.MkdirAll(c.Runs, 0700); err != nil {
		return toolStorageRelocationFailure("publication", "stage")
	}
	if err := os.Mkdir(p.MetadataStage, 0700); err != nil {
		return toolStorageRelocationFailure("publication", "stage")
	}
	p.MetadataStageID, err = localNativeID(p.MetadataStage)
	if err != nil {
		return toolStorageRelocationFailure("publication", "stage")
	}
	recordStage := func() error {
		b, e := json.Marshal(p)
		if e != nil {
			return e
		}
		return installationWrite(c.Root, toolStorageRel(c, statePath), b)
	}
	if err := recordStage(); err != nil {
		return toolStorageRelocationFailure("state", "stage")
	}
	stageFile := func(name string, data []byte) (string, error) {
		f, err := os.OpenFile(filepath.Join(p.MetadataStage, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return "", err
		}
		id, err := localNativeIDFile(f)
		if err == nil {
			if name == "new-profile" {
				p.NewProfileID = id
			} else {
				p.NewReceiptID = id
			}
			err = recordStage()
		}
		if err == nil {
			_, err = f.Write(data)
		}
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		return id, err
	}
	p.NewProfileID, err = stageFile("new-profile", newProfile)
	if err != nil {
		return toolStorageRelocationFailure("publication", "stage")
	}
	newReceipt.ProfileNativeIdentity = p.NewProfileID
	p.NewReceipt, _ = json.Marshal(newReceipt)
	p.NewReceiptID, err = stageFile("new-receipt", p.NewReceipt)
	if err != nil {
		return toolStorageRelocationFailure("publication", "stage")
	}
	p.Phase = "staged"
	raw, _ = json.Marshal(p)
	if err := installationWrite(c.Root, toolStorageRel(c, statePath), raw); err != nil {
		return toolStorageRelocationFailure("state", "stage")
	}
	toolStorageRelocationHook("staged")
	for _, item := range []struct{ live, old, next, oldID, newID string }{
		{profilePath, "old-profile", "new-profile", p.OldProfileID, p.NewProfileID},
		{receiptPath, "old-receipt", "new-receipt", p.OldReceiptID, p.NewReceiptID},
	} {
		stageID, e := localNativeID(p.MetadataStage)
		if e != nil || stageID != p.MetadataStageID {
			return toolStorageRelocationFailure("publication", "identity")
		}
		oldID, e := localNativeID(item.live)
		if e != nil || oldID != item.oldID {
			return toolStorageRelocationFailure("publication", "identity")
		}
		newID, e := localNativeID(filepath.Join(p.MetadataStage, item.next))
		if e != nil || newID != item.newID {
			return toolStorageRelocationFailure("publication", "identity")
		}
		if _, e = os.Lstat(filepath.Join(p.MetadataStage, item.old)); !os.IsNotExist(e) {
			return toolStorageRelocationFailure("publication", "occupied")
		}
		if e = os.Rename(item.live, filepath.Join(p.MetadataStage, item.old)); e != nil {
			return toolStorageRelocationFailure("publication", "park")
		}
		p.PublicationStep++
		if e = recordStage(); e != nil {
			return toolStorageRelocationFailure("state", "park")
		}
		if e = os.Rename(filepath.Join(p.MetadataStage, item.next), item.live); e != nil {
			return toolStorageRelocationFailure("publication", "activate")
		}
		p.PublicationStep++
		if e = recordStage(); e != nil {
			return toolStorageRelocationFailure("state", "activate")
		}
	}
	p.Phase = "active"
	raw, _ = json.Marshal(p)
	if err := installationWrite(c.Root, toolStorageRel(c, statePath), raw); err != nil {
		return toolStorageRelocationFailure("state", "active")
	}
	toolStorageRelocationHook("active")
	p.Closed = true
	raw, _ = json.Marshal(p)
	if err := installationWrite(c.Root, toolStorageRel(c, statePath), raw); err != nil {
		return toolStorageRelocationFailure("state", "active")
	}
	return installationResponse("TOOL_STORAGE_ACTIVE", true, map[string]any{"phase": "active"})
}

func toolStorageRelocationSameEntries(a, b []toolStorageRelocationEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func toolStorageRelocationSameContent(a, b []toolStorageRelocationEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Path != b[i].Path || a[i].Directory != b[i].Directory || a[i].Size != b[i].Size || a[i].SHA256 != b[i].SHA256 || a[i].Mode != b[i].Mode {
			return false
		}
	}
	return true
}
func toolStorageRelocationRollback(c layoutContext, p toolStorageRelocationPlan, a toolStorageRelocationApproval) Response {
	statePath := toolStorageRelocationStatePath(c)
	if raw, err := installationRead(c.Root, toolStorageRel(c, statePath), toolStorageRelocationLimit); err == nil {
		var saved toolStorageRelocationPlan
		if checkpointStrictJSON(raw, &saved) && saved.PlanID == p.PlanID && saved.Phase == "active" {
			p = saved
		}
	}
	if len(p.NewProfile) == 0 || len(p.NewReceipt) == 0 {
		return toolStorageRelocationFailure("not_active", "rollback")
	}
	if p.ConsentReference != a.ConsentReference {
		return toolStorageRelocationFailure("approval", "rollback")
	}
	if id, err := localNativeID(p.TargetRoot); err != nil || id != p.TargetIdentity {
		return toolStorageRelocationFailure("target_identity", "rollback")
	}
	if current, err := toolStorageRelocationInventory(p.SourceRoot, p.MaxEntries, p.MaxBytes); err != nil || !toolStorageRelocationSameEntries(current, p.Entries) {
		return toolStorageRelocationFailure("source_drift", "rollback")
	}
	var activePolicy toolStoragePolicy
	if !checkpointStrictJSON(p.NewProfile, &activePolicy) {
		return toolStorageRelocationFailure("profile", "rollback")
	}
	if uses, err := toolStorageRelocationUses(c, activePolicy); err != nil || uses != 0 {
		return toolStorageRelocationFailure("use_registry", "rollback")
	}
	if current, err := toolStorageRelocationInventory(p.TargetRoot, p.MaxEntries, p.MaxBytes); err != nil || !toolStorageRelocationSameContent(current, p.Entries) || !toolStorageRelocationTargetIdentities(current, p.TargetFiles) {
		return toolStorageRelocationFailure("destination_diverged", "rollback")
	}
	p.Phase = "rolling_back"
	p.Closed = false
	p.PublicationStep = 0
	rawState, err := json.Marshal(p)
	if err != nil || installationWrite(c.Root, toolStorageRel(c, statePath), rawState) != nil {
		return toolStorageRelocationFailure("state", "rollback")
	}
	for _, item := range []struct{ live, old, next, oldID, newID string }{
		{filepath.Join(toolStorageBase(c), a.ToolID+".json"), "old-profile", "new-profile", p.OldProfileID, p.NewProfileID},
		{toolStorageReceiptPath(c, a.ToolID), "old-receipt", "new-receipt", p.OldReceiptID, p.NewReceiptID},
	} {
		stageID, err := localNativeID(p.MetadataStage)
		if err != nil || stageID != p.MetadataStageID {
			return toolStorageRelocationFailure("stage", "rollback")
		}
		id, err := localNativeID(item.live)
		if err != nil || id != item.newID {
			return toolStorageRelocationFailure("active_identity", "rollback")
		}
		id, err = localNativeID(filepath.Join(p.MetadataStage, item.old))
		if err != nil || id != item.oldID {
			return toolStorageRelocationFailure("old_identity", "rollback")
		}
		if _, err = os.Lstat(filepath.Join(p.MetadataStage, item.next)); !os.IsNotExist(err) {
			return toolStorageRelocationFailure("publication", "rollback")
		}
		if err = os.Rename(item.live, filepath.Join(p.MetadataStage, item.next)); err != nil {
			return toolStorageRelocationFailure("publication", "rollback")
		}
		p.PublicationStep++
		rawState, err = json.Marshal(p)
		if err != nil || installationWrite(c.Root, toolStorageRel(c, statePath), rawState) != nil {
			return toolStorageRelocationFailure("state", "rollback")
		}
		if err = os.Rename(filepath.Join(p.MetadataStage, item.old), item.live); err != nil {
			return toolStorageRelocationFailure("publication", "rollback")
		}
		p.PublicationStep++
		rawState, err = json.Marshal(p)
		if err != nil || installationWrite(c.Root, toolStorageRel(c, statePath), rawState) != nil {
			return toolStorageRelocationFailure("state", "rollback")
		}
	}
	p.Phase = "rolled_back"
	rawState, err = json.Marshal(p)
	if err != nil || installationWrite(c.Root, toolStorageRel(c, statePath), rawState) != nil {
		return toolStorageRelocationFailure("state", "rollback")
	}
	return installationResponse("TOOL_STORAGE_ACTIVE", true, map[string]any{"phase": "rolled_back"})
}
func toolStorageRelocationTargetIdentities(entries []toolStorageRelocationEntry, expected map[string]string) bool {
	if len(entries) != len(expected) {
		return false
	}
	for _, entry := range entries {
		if expected[entry.Path] == "" || expected[entry.Path] != entry.NativeIdentity {
			return false
		}
	}
	return true
}
func toolStorageRelocationInspect(c layoutContext, r Request) Response {
	var q struct {
		ToolID string `json:"toolId"`
	}
	if !toolStorageRelocationExact(r.Payload, &q, "toolId") || !toolStorageSlug.MatchString(q.ToolID) {
		return toolStorageRelocationFailure("input", "inspect")
	}
	stateRaw, stateErr := installationRead(c.Root, toolStorageRel(c, toolStorageRelocationStatePath(c)), toolStorageRelocationLimit)
	if stateErr == nil {
		var state toolStorageRelocationPlan
		if !checkpointStrictJSON(stateRaw, &state) || state.SchemaVersion != 1 || !isUUID(state.PlanID) || state.ToolID == "" {
			return toolStorageRelocationFailure("unknown_transaction", "inspect")
		}
		if state.ToolID == q.ToolID {
			var old toolStoragePolicy
			if !toolStoragePolicyShape(state.OldProfile) || !checkpointStrictJSON(state.OldProfile, &old) {
				return toolStorageRelocationFailure("unknown_transaction", "inspect")
			}
			active, usesErr := toolStorageRelocationUses(c, old)
			data := map[string]any{"phase": state.Phase, "toolId": q.ToolID, "owner": old.Owner, "purpose": old.Purpose, "retention": old.Retention, "oldLocation": state.SourceRoot, "newLocation": state.TargetRoot, "nextAction": "inspect retained transition"}
			if usesErr != nil {
				data["pendingUses"] = "unknown"
			} else {
				data["activeUses"] = active
				data["pendingUses"] = 0
			}
			return installationResponse("TOOL_STORAGE_INSPECTED", true, data)
		}
	} else if !os.IsNotExist(stateErr) {
		return toolStorageRelocationFailure("unknown_transaction", "inspect")
	}
	raw, err := installationRead(c.Root, toolStorageRel(c, filepath.Join(toolStorageBase(c), q.ToolID+".json")), toolStorageLimit)
	if err != nil {
		return bad("TOOL_STORAGE_NOT_FOUND", 0)
	}
	h := sha256.Sum256(raw)
	p, _, _, _, err := toolStorageRelocationProfile(c, q.ToolID, hex.EncodeToString(h[:]))
	if err != nil {
		return toolStorageRelocationFailure("profile", "inspect")
	}
	uses, err := toolStorageRelocationUses(c, p)
	if err != nil {
		return toolStorageRelocationFailure("use_registry", "inspect")
	}
	return installationResponse("TOOL_STORAGE_INSPECTED", true, map[string]any{"phase": "active", "toolId": q.ToolID, "activeUses": uses, "pendingUses": 0, "owner": p.Owner, "purpose": p.Purpose, "retention": p.Retention, "oldLocation": filepath.Join(p.ToolDataRoot, p.ToolID), "nextAction": "prepare relocation"})
}
