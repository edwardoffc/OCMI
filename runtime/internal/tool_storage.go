package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

const toolStorageLimit = 16 << 10

var toolStorageSlug = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
var toolStorageKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)

type toolStorageRef struct {
	Home   string `json:"home"`
	ToolID string `json:"toolId"`
	SHA256 string `json:"sha256"`
}
type toolStorageStore struct {
	Key            string `json:"key"`
	Path           string `json:"path"`
	NativeIdentity string `json:"nativeIdentity"`
	Kind           string `json:"kind"`
}
type toolStoragePolicy struct {
	SchemaVersion    int                `json:"schemaVersion"`
	ToolID           string             `json:"toolId"`
	HomeIdentity     string             `json:"homeIdentity"`
	ToolDataRoot     string             `json:"toolDataRoot"`
	ToolDataIdentity string             `json:"toolDataIdentity"`
	Owner            string             `json:"owner"`
	Purpose          string             `json:"purpose"`
	Retention        string             `json:"retention"`
	ConsentReference string             `json:"consentReference"`
	MaxBytes         int64              `json:"maxBytes"`
	MaxAgeDays       int                `json:"maxAgeDays"`
	Stores           []toolStorageStore `json:"stores"`
	ScratchEnv       []string           `json:"scratchEnv"`
	UseRootIdentity  string             `json:"useRootIdentity,omitempty"`
}
type toolStorageBinding struct {
	ref                    toolStorageRef
	policy                 toolStoragePolicy
	profileID, profileHash string
}

func toolStorageFromCommand(project string, raw json.RawMessage) (*toolStorageBinding, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return nil, errors.New("command")
	}
	for key := range fields {
		if strings.EqualFold(key, "toolStorage") && key != "toolStorage" {
			return nil, errors.New("profile")
		}
	}
	v, ok := fields["toolStorage"]
	if !ok {
		return nil, nil
	}
	if !checkpointStrictJSON(raw, &fields) {
		return nil, errors.New("profile")
	}
	v = fields["toolStorage"]
	if string(v) == "null" {
		return nil, errors.New("profile")
	}
	var ref toolStorageRef
	if !toolStorageExactKeys(v, "home", "toolId", "sha256") || !strictToolStorageJSON(v, &ref) || !toolStorageSlug.MatchString(ref.ToolID) || !filepath.IsAbs(ref.Home) || !toolStorageText(ref.Home, 1024) || !toolStorageHash(ref.SHA256) {
		return nil, errors.New("profile")
	}
	if _, code := resolveLayout(ref.Home, "installation"); code != "" {
		return nil, errors.New("profile")
	}
	if environmentPlainAncestors(ref.Home) != nil {
		return nil, errors.New("profile")
	}
	profile := filepath.Join(ref.Home, "local", "bindings", "tool-storage", ref.ToolID+".json")
	if environmentPlainAncestors(filepath.Dir(profile)) != nil {
		return nil, errors.New("profile")
	}
	id, err := localNativeID(profile)
	if err != nil {
		return nil, errors.New("profile")
	}
	relative := "local/bindings/tool-storage/" + ref.ToolID + ".json"
	rawPolicy, err := installationRead(ref.Home, relative, toolStorageLimit)
	if err != nil {
		return nil, errors.New("profile")
	}
	h := sha256.Sum256(rawPolicy)
	if hex.EncodeToString(h[:]) != ref.SHA256 {
		return nil, errors.New("profile")
	}
	var policy toolStoragePolicy
	if !toolStoragePolicyShape(rawPolicy) || !strictToolStorageJSON(rawPolicy, &policy) || !toolStoragePolicyValid(project, ref, policy) {
		return nil, errors.New("profile")
	}
	if current, currentErr := localNativeID(profile); currentErr != nil || current != id {
		return nil, errors.New("profile")
	}
	profileHash := ref.SHA256
	if strings.HasPrefix(policy.ConsentReference, "ocmi-tool-storage:") {
		receiptProof, receiptErr := toolStorageAuthoringReceiptProof(ref, policy, id)
		if receiptErr != nil {
			return nil, errors.New("profile")
		}
		profileHash += ":" + receiptProof
	}
	return &toolStorageBinding{ref: ref, policy: policy, profileID: id, profileHash: profileHash}, nil
}
func toolStoragePolicyShape(raw []byte) bool {
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return false
	}
	keys := []string{"schemaVersion", "toolId", "homeIdentity", "toolDataRoot", "toolDataIdentity", "owner", "purpose", "retention", "consentReference", "maxBytes", "maxAgeDays", "stores", "scratchEnv"}
	var version int
	if !checkpointStrictJSON(top["schemaVersion"], &version) {
		return false
	}
	if version == 2 {
		keys = append(keys, "useRootIdentity")
	}
	if !toolStorageExactKeys(raw, keys...) {
		return false
	}
	var stores []json.RawMessage
	if json.Unmarshal(top["stores"], &stores) != nil {
		return false
	}
	for _, store := range stores {
		if !toolStorageExactKeys(store, "key", "path", "nativeIdentity", "kind") {
			return false
		}
	}
	return true
}
func strictToolStorageJSON(raw []byte, target any) bool {
	return checkpointStrictJSON(raw, target)
}
func toolStorageExactKeys(raw []byte, allowed ...string) bool {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	want := map[string]bool{}
	for _, k := range allowed {
		want[k] = true
	}
	for k := range m {
		if !want[k] {
			return false
		}
	}
	return len(m) == len(want)
}
func toolStorageHash(s string) bool {
	return len(s) == 64 && strings.ToLower(s) == s && regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(s)
}
func toolStorageText(s string, max int) bool {
	return strings.TrimSpace(s) != "" && len(s) <= max && utf8.ValidString(s)
}
func toolStoragePolicyValid(project string, ref toolStorageRef, p toolStoragePolicy) bool {
	if (p.SchemaVersion != 1 && p.SchemaVersion != 2) || p.ToolID != ref.ToolID || !toolStorageText(p.HomeIdentity, 1024) || !toolStorageText(p.ToolDataRoot, 1024) || !filepath.IsAbs(p.ToolDataRoot) || !toolStorageText(p.ToolDataIdentity, 1024) || !toolStorageText(p.Owner, 1024) || !toolStorageText(p.Purpose, 1024) || !toolStorageText(p.Retention, 1024) || !toolStorageText(p.ConsentReference, 1024) || p.MaxBytes < 1 || p.MaxBytes > (1<<50) || p.MaxAgeDays < 1 || p.MaxAgeDays > 36500 || len(p.Stores) < 1 || len(p.Stores) > 16 || len(p.ScratchEnv) > 8 {
		return false
	}
	if p.SchemaVersion == 2 {
		uses := filepath.Join(ref.Home, "local", "bindings", "tool-storage", "uses", p.ToolID)
		id, err := localNativeID(uses)
		if err != nil || id != p.UseRootIdentity || p.UseRootIdentity == "" {
			return false
		}
	}
	homeID, e := localNativeID(ref.Home)
	if e != nil || homeID != p.HomeIdentity {
		return false
	}
	dataInfo, dataErr := os.Lstat(p.ToolDataRoot)
	if dataErr != nil || !dataInfo.IsDir() || dataInfo.Mode()&os.ModeSymlink != 0 || filepath.Dir(filepath.Clean(p.ToolDataRoot)) == filepath.Clean(p.ToolDataRoot) {
		return false
	}
	dataID, e := localNativeID(p.ToolDataRoot)
	if e != nil || dataID != p.ToolDataIdentity || environmentPlainAncestors(p.ToolDataRoot) != nil || environmentWithin(project, p.ToolDataRoot) || environmentWithin(p.ToolDataRoot, project) || environmentWithin(ref.Home, p.ToolDataRoot) || environmentWithin(p.ToolDataRoot, ref.Home) {
		return false
	}
	seen := map[string]bool{}
	for _, s := range p.Stores {
		if !toolStorageKey.MatchString(s.Key) || !toolStorageText(s.Path, 1024) || !toolStorageText(s.NativeIdentity, 1024) || (s.Kind != "cache" && s.Kind != "data") || !toolStorageRelative(s.Path) {
			return false
		}
		key := strings.ToUpper(s.Key)
		if seen[key] || toolStorageReserved(key, ref.ToolID) || !toolStorageStoreKey(ref.ToolID, key) {
			return false
		}
		seen[key] = true
		path := filepath.Join(p.ToolDataRoot, filepath.FromSlash(s.Path))
		info, statErr := os.Lstat(path)
		if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		id, e := localNativeID(path)
		if e != nil || id != s.NativeIdentity || environmentPlainAncestors(path) != nil {
			return false
		}
	}
	for _, k := range p.ScratchEnv {
		if !toolStorageKey.MatchString(k) {
			return false
		}
		key := strings.ToUpper(k)
		if seen[key] || toolStorageReserved(key, ref.ToolID) || !toolStorageScratchKey(ref.ToolID, key) {
			return false
		}
		seen[key] = true
	}
	return true
}
func toolStorageRelative(p string) bool {
	if p == "" || filepath.ToSlash(filepath.Clean(filepath.FromSlash(p))) != p || filepath.IsAbs(filepath.FromSlash(p)) || strings.ContainsAny(p, "\\:") {
		return false
	}
	for _, x := range strings.Split(p, "/") {
		if x == "" || x == "." || x == ".." || strings.HasPrefix(x, ".") {
			return false
		}
	}
	return true
}
func toolStorageStoreKey(tool, key string) bool {
	if tool == "go" {
		return key == "GOCACHE" || key == "GOMODCACHE" || key == "GOPATH" || key == "GOBIN"
	}
	if tool == "npm" {
		return key == "NPM_CONFIG_CACHE"
	}
	return strings.HasPrefix(key, strings.ToUpper(strings.ReplaceAll(tool, "-", "_"))+"_")
}
func toolStorageScratchKey(tool, key string) bool {
	if tool == "go" {
		return key == "GOTMPDIR"
	}
	return strings.HasPrefix(key, strings.ToUpper(strings.ReplaceAll(tool, "-", "_"))+"_")
}
func toolStorageReserved(key, tool string) bool {
	if key == "PATH" || key == "HOME" || key == "USERPROFILE" || key == "APPDATA" || key == "LOCALAPPDATA" || key == "ENV" || key == "BASH_ENV" || key == "NODE_OPTIONS" || key == "PYTHONPATH" || key == "GOENV" || key == "GOFLAGS" || key == "GOTOOLCHAIN" || key == "TMP" || key == "TEMP" || key == "TMPDIR" || strings.HasPrefix(key, "XDG_") || strings.HasPrefix(key, "LD_") || strings.HasPrefix(key, "DYLD_") {
		return true
	}
	return false
}
func (b *toolStorageBinding) env(tmp string) []string {
	out := []string{}
	for _, s := range b.policy.Stores {
		out = append(out, s.Key+"="+filepath.Join(b.policy.ToolDataRoot, filepath.FromSlash(s.Path)))
	}
	for _, k := range b.policy.ScratchEnv {
		out = append(out, k+"="+tmp)
	}
	return out
}
