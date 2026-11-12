package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/uuid"
)

const installationReceiptName = "installation.json"

type installationReceipt struct {
	SchemaVersion  int      `json:"schemaVersion"`
	Phase          string   `json:"phase"`
	InstallationID string   `json:"installationId"`
	DeviceID       string   `json:"deviceId"`
	CopyID         string   `json:"copyId"`
	SourceCommit   string   `json:"sourceCommit"`
	LayoutVersion  int      `json:"layoutVersion"`
	Infrastructure []string `json:"infrastructure"`
	Home           string   `json:"home"`
	HomeID         string   `json:"homeId"`
}

func installationResponse(code string, ok bool, data map[string]any) Response {
	if !ok {
		if data == nil {
			data = map[string]any{}
		}
		reason, stage, next := "factory_payload", "factory", "inspect factory home"
		if strings.HasPrefix(code, "COORDINATOR_") {
			reason, stage, next = "binding_or_options", "coordinator", "inspect coordinator inputs"
		}
		if code == "COORDINATOR_PENDING" || code == "INSTALLATION_PENDING" {
			reason, stage, next = "unfinished_provisioning", "provisioning", "inspect pending provisioning"
		}
		if _, exists := data["reason"]; !exists {
			data["reason"] = reason
		}
		if _, exists := data["stage"]; !exists {
			data["stage"] = stage
		}
		if _, exists := data["nextAction"]; !exists {
			data["nextAction"] = next
		}
	}
	return Response{SchemaVersion: schemaVersion, OK: ok, Code: code, Revision: 0, Data: data}
}

func setupFailure(code, reason, stage, next string) Response {
	return installationResponse(code, false, map[string]any{"reason": reason, "stage": stage, "nextAction": next})
}

// InstallHome validates an already unpacked ready-kit and creates only local bootstrap state.
func InstallHome(home string, inspect bool) Response {
	context, info, err := verifyFactoryHome(home)
	if err != nil {
		return setupFailure("INSTALLATION_REFUSED", "factory_payload", "factory", "inspect factory home")
	}
	receiptPath := filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"]), installationReceiptName)
	receipt, receiptErr := readInstallationReceiptContext(context)
	if receiptErr == nil {
		if receipt.Phase != "installed" || receipt.SourceCommit != info.SourceCommit || !installationCoreMatches(context, receipt) {
			return setupFailure("INSTALLATION_PENDING", "receipt_or_core", "installation", "inspect local receipt")
		}
		return installationResponse("INSTALLATION_INSPECTED", true, map[string]any{"phase": "installed", "installationId": receipt.InstallationID})
	}
	if receiptErr != nil && !errors.Is(receiptErr, os.ErrNotExist) {
		return setupFailure("INSTALLATION_REFUSED", "receipt_metadata", "installation", "inspect local receipt")
	}
	if privateHomeOccupied(context) {
		return installationResponse("INSTALLATION_REFUSED", false, map[string]any{})
	}
	if inspect {
		return installationResponse("INSTALLATION_INSPECTED", true, map[string]any{"phase": "uninitialized"})
	}
	unlock, lockErr := installationAdmission(context)
	if lockErr != nil {
		return installationResponse("INSTALLATION_PENDING", false, map[string]any{"phase": "pending"})
	}
	defer unlock()
	if _, err := os.Lstat(receiptPath); err == nil {
		return installationResponse("INSTALLATION_PENDING", false, map[string]any{"phase": "pending"})
	} else if !os.IsNotExist(err) {
		return installationResponse("INSTALLATION_REFUSED", false, map[string]any{})
	}
	homeID, identityErr := localNativeID(context.Root)
	if identityErr != nil {
		return installationResponse("INSTALLATION_REFUSED", false, map[string]any{})
	}
	receipt = installationReceipt{SchemaVersion: 1, Phase: "pending", InstallationID: uuid.NewString(), DeviceID: uuid.NewString(), CopyID: uuid.NewString(), SourceCommit: info.SourceCommit, LayoutVersion: 1, Infrastructure: []string{"local/state/state.sqlite", "local/bindings/installation.json", "OCMI_Sync/README.md"}, Home: context.Root, HomeID: homeID}
	if err := writeInstallationReceipt(context, receipt); err != nil {
		return installationResponse("INSTALLATION_PENDING", false, map[string]any{"phase": "pending"})
	}
	payload, _ := json.Marshal(map[string]string{"projectId": receipt.InstallationID, "deviceId": receipt.DeviceID, "copyId": receipt.CopyID, "owner": "coordinator"})
	init := Handle(Request{SchemaVersion: 1, Action: "init", ProjectRoot: context.Root, RootKind: "installation", Actor: "coordinator", RequestID: "installation-init-" + receipt.InstallationID, Payload: payload})
	if !init.OK && init.Code != "INITIALIZED" {
		return installationResponse("INSTALLATION_PENDING", false, map[string]any{"phase": "pending"})
	}
	if !installationCoreMatches(context, receipt) {
		return installationResponse("INSTALLATION_PENDING", false, map[string]any{"phase": "pending"})
	}
	text, textErr := readHomeText(context.Root, "en")
	if textErr != nil || ensureSyncReadme(context, text.SyncReadme) != nil {
		return installationResponse("INSTALLATION_PENDING", false, map[string]any{"phase": "pending"})
	}
	receipt.Phase = "installed"
	if err := writeInstallationReceipt(context, receipt); err != nil {
		return installationResponse("INSTALLATION_PENDING", false, map[string]any{"phase": "pending"})
	}
	return installationResponse("INSTALLED", true, map[string]any{"phase": "installed", "installationId": receipt.InstallationID})
}

type factoryInfo struct{ SourceCommit string }

func verifyFactoryHome(home string) (layoutContext, factoryInfo, error) {
	if _, err := installationRead(home, homeMarker, 1<<20); err != nil {
		return layoutContext{}, factoryInfo{}, errors.New("marker")
	}
	context, code := resolveLayout(home, "installation")
	if code != "" {
		return layoutContext{}, factoryInfo{}, errors.New(code)
	}
	if err := verifyPublicInventory(context.Root); err != nil {
		return layoutContext{}, factoryInfo{}, err
	}
	raw, err := installationRead(context.Root, "system/build-info.json", 1<<20)
	if err != nil {
		return layoutContext{}, factoryInfo{}, errors.New("build info")
	}
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil || value["distributionKind"] != "ready-kit" || value["layoutVersion"] != float64(1) || value["layoutDeclaration"] != ".ocmi-home.json" || value["sourceDirty"] != false {
		return layoutContext{}, factoryInfo{}, errors.New("build info")
	}
	commit, _ := value["sourceCommit"].(string)
	target, _ := value["target"].(string)
	binary, _ := value["binaryPath"].(string)
	version, _ := value["systemVersion"].(string)
	expectedBinary := "system/bin/ocmi"
	if runtime.GOOS == "windows" {
		expectedBinary += ".exe"
	}
	versionRaw, versionErr := installationRead(context.Root, "system/VERSION.md", 1<<20)
	if len(commit) != 40 || strings.Trim(commit, "0123456789abcdef") != "" || version == "" || versionErr != nil || !factoryVersion(string(versionRaw), version) || target != runtime.GOOS+"-"+runtime.GOARCH || binary != expectedBinary {
		return layoutContext{}, factoryInfo{}, errors.New("build info")
	}
	return context, factoryInfo{SourceCommit: commit}, nil
}

func factoryVersion(raw, want string) bool {
	first, _, _ := strings.Cut(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	return first == "# OCMI "+want
}

func verifyPublicInventory(root string) error {
	raw, err := installationRead(root, "SHA256SUMS", 1<<20)
	if err != nil {
		return errors.New("sums")
	}
	expected := map[string]string{}
	casefold := map[string]bool{}
	total := int64(0)
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		line = strings.TrimSuffix(line, "\r")
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			return errors.New("sums")
		}
		fold := strings.ToLower(parts[1])
		if len(parts[0]) != 64 || strings.Trim(parts[0], "0123456789abcdef") != "" || !factoryName(parts[1]) || expected[parts[1]] != "" || casefold[fold] {
			return errors.New("sums")
		}
		expected[parts[1]] = parts[0]
		casefold[fold] = true
	}
	if len(expected) == 0 || len(expected) > 4096 {
		return errors.New("sums")
	}
	for _, required := range []string{".ocmi-home.json", "README.md", "README.ru.md", "AGENTS.md", ".gitignore", ".gitattributes", "system/build-info.json", "system/VERSION.md"} {
		if _, ok := expected[required]; !ok {
			return errors.New("required")
		}
	}
	seen := map[string]bool{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.Type()&os.ModeIrregular != 0 {
			return errors.New("link")
		}
		if rel == "OCMI_Sync" || rel == "local" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return errors.New("private")
		}
		if strings.HasPrefix(rel, "OCMI_Sync/") || strings.HasPrefix(rel, "local/") || rel == ".ocmi" || strings.HasPrefix(rel, ".ocmi/") {
			return errors.New("private")
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return errors.New("regular")
		}
		if rel == "SHA256SUMS" {
			return nil
		}
		want, ok := expected[rel]
		if !ok {
			return errors.New("unknown")
		}
		info, e := entry.Info()
		if e != nil || info.Size() > 32<<20 {
			return errors.New("size")
		}
		total += info.Size()
		if total > 256<<20 {
			return errors.New("total")
		}
		file, e := os.Open(path)
		if e != nil {
			return e
		}
		h := sha256.New()
		_, e = io.Copy(h, io.LimitReader(file, (32<<20)+1))
		closeErr := file.Close()
		if e != nil || closeErr != nil || hex.EncodeToString(h.Sum(nil)) != want {
			return errors.New("digest")
		}
		seen[rel] = true
		return nil
	})
	if err != nil || len(seen) != len(expected) {
		return errors.New("inventory")
	}
	for name := range expected {
		if !seen[name] {
			return errors.New("missing")
		}
	}
	return nil
}

func factoryName(name string) bool {
	if name == "" || name == "SHA256SUMS" || !portableName(name) || filepath.IsAbs(filepath.FromSlash(name)) || filepath.ToSlash(filepath.Clean(filepath.FromSlash(name))) != name || strings.HasPrefix(name, "../") || strings.Contains(name, "\\") {
		return false
	}
	if strings.HasPrefix(name, "system/") {
		source := strings.ToLower(strings.TrimPrefix(name, "system/"))
		for _, part := range strings.Split(source, "/") {
			if part == ".git" || part == ".local" || part == ".codex" || part == ".ocmi" || part == "__pycache__" || part == "node_modules" {
				return false
			}
		}
		for _, prefix := range []string{"ocmi_sync/", "local/", "docs/memory/", "docs/research/", "locales/ru/docs/research/", "diagnostics/"} {
			if strings.HasPrefix(source, prefix) {
				return false
			}
		}
		return true
	}
	for _, fixed := range []string{".ocmi-home.json", "README.md", "README.ru.md", "AGENTS.md", ".gitignore", ".gitattributes"} {
		if name == fixed {
			return true
		}
	}
	return false
}

func receiptValid(r installationReceipt) bool {
	return r.SchemaVersion == 1 && (r.Phase == "pending" || r.Phase == "installed") && isUUID(r.InstallationID) && isUUID(r.DeviceID) && isUUID(r.CopyID) && len(r.SourceCommit) == 40 && r.LayoutVersion == 1 && r.Home != "" && r.HomeID != ""
}
func readInstallationReceipt(path string) (installationReceipt, error) {
	raw, e := os.ReadFile(path)
	if e != nil {
		return installationReceipt{}, e
	}
	var r installationReceipt
	if len(raw) > 1<<20 || !checkpointStrictJSON(raw, &r) || !receiptValid(r) {
		return installationReceipt{}, errors.New("receipt")
	}
	return r, nil
}
func readInstallationReceiptContext(context layoutContext) (installationReceipt, error) {
	raw, e := installationRead(context.Root, context.Paths["bindings"]+"/"+installationReceiptName, 1<<20)
	if e != nil {
		return installationReceipt{}, e
	}
	var r installationReceipt
	if !checkpointStrictJSON(raw, &r) || !receiptValid(r) {
		return installationReceipt{}, errors.New("receipt")
	}
	return r, nil
}
func writeInstallationReceipt(context layoutContext, receipt installationReceipt) error {
	if !receiptValid(receipt) {
		return errors.New("receipt")
	}
	raw, _ := json.Marshal(receipt)
	return installationWrite(context.Root, context.Paths["bindings"]+"/"+installationReceiptName, raw)
}
func privateHomeOccupied(context layoutContext) bool {
	for _, name := range []string{context.Paths["local"], context.Paths["bindings"]} {
		if _, e := os.Lstat(filepath.Join(context.Root, filepath.FromSlash(name))); e == nil {
			return true
		} else if !os.IsNotExist(e) {
			return true
		}
	}
	return false
}
func installationCoreMatches(context layoutContext, receipt installationReceipt) bool {
	if receipt.Home != context.Root {
		return false
	}
	if id, err := localNativeID(context.Root); err != nil || id != receipt.HomeID {
		return false
	}
	db, e := openLayout(context)
	if e != nil {
		return false
	}
	defer db.Close()
	var project, device, copyID, owner string
	return db.QueryRow("SELECT id,device_id,copy_id,owner FROM project").Scan(&project, &device, &copyID, &owner) == nil && project == receipt.InstallationID && device == receipt.DeviceID && copyID == receipt.CopyID && owner == "coordinator"
}
func ensureSyncReadme(context layoutContext, content string) error {
	path := filepath.Join(context.Root, filepath.FromSlash(context.Paths["sync"]), "README.md")
	if _, e := os.Lstat(path); e == nil {
		return nil
	} else if !os.IsNotExist(e) {
		return e
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return installationWrite(context.Root, context.Paths["sync"]+"/README.md", []byte(content))
}
func setupLock(parent string) (func(), error) {
	if info, err := os.Lstat(parent); err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return nil, errors.New("unsafe setup parent")
	}
	if err := os.MkdirAll(parent, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(parent, ".setup-lock")
	if err := os.Mkdir(path, 0700); err != nil {
		return nil, err
	}
	return func() { _ = os.Remove(path) }, nil
}
