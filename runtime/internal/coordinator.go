package internal

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type coordinatorBinding struct {
	SchemaVersion  int    `json:"schemaVersion"`
	Phase          string `json:"phase"`
	CoordinatorID  string `json:"coordinatorId"`
	InstallationID string `json:"installationId"`
	DeviceID       string `json:"deviceId"`
	CopyID         string `json:"copyId"`
	Home           string `json:"home"`
	ClientProject  string `json:"clientProject"`
	Language       string `json:"language"`
	SourceCommit   string `json:"sourceCommit"`
	EvidenceSource string `json:"evidenceSource,omitempty"`
	Evidence       string `json:"evidence,omitempty"`
}
type coordinatorMemory struct {
	SchemaVersion int    `json:"schemaVersion"`
	CoordinatorID string `json:"coordinatorId"`
	Language      string `json:"language"`
}
type homeText struct {
	SchemaVersion      int    `json:"schemaVersion"`
	RootReadme         string `json:"rootReadme"`
	RootAgent          string `json:"rootAgent"`
	SyncReadme         string `json:"syncReadme"`
	CoordinatorAgent   string `json:"coordinatorAgent"`
	CoordinatorIndex   string `json:"coordinatorIndex"`
	CoordinatorHandoff string `json:"coordinatorHandoff"`
	Startup            string `json:"startup"`
}

func CoordinatorPrepare(home, language, project string) Response {
	context, info, err := verifyFactoryHome(home)
	if err != nil {
		return setupFailure("COORDINATOR_REFUSED", "factory_payload", "factory", "inspect factory home")
	}
	receipt, err := currentInstallation(context, info)
	if err != nil {
		return setupFailure("COORDINATOR_REFUSED", "receipt_or_core", "installation", "inspect local receipt")
	}
	if (language != "en" && language != "ru") || !bounded(project, 512) {
		return setupFailure("COORDINATOR_REFUSED", "invalid_options", "options", "review coordinator options")
	}
	unlock, lockErr := installationAdmission(context)
	if lockErr != nil {
		return installationResponse("COORDINATOR_PENDING", false, map[string]any{"phase": "handover_pending"})
	}
	defer unlock()
	if receipt, err = currentInstallation(context, info); err != nil {
		return installationResponse("COORDINATOR_REFUSED", false, map[string]any{"reason": "receipt_context", "stage": "admission", "nextAction": "inspect installation"})
	}
	binding, bindErr := readCoordinatorBindingContext(context)
	if bindErr != nil && !errors.Is(bindErr, os.ErrNotExist) {
		return setupFailure("COORDINATOR_REFUSED", "binding_metadata", "binding", "inspect local binding")
	}
	if errors.Is(bindErr, os.ErrNotExist) {
		localCoordinator := filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"]), "coordinator")
		if _, err := os.Lstat(localCoordinator); err == nil {
			return installationResponse("COORDINATOR_REFUSED", false, map[string]any{"stage": "foreign_binding", "nextAction": "inspect local metadata"})
		} else if !os.IsNotExist(err) {
			return installationResponse("COORDINATOR_REFUSED", false, map[string]any{})
		}
	}
	if bindErr == nil && (binding.ClientProject != project || binding.Language != language || binding.InstallationID != receipt.InstallationID) {
		return setupFailure("COORDINATOR_REFUSED", "binding_conflict", "binding", "review coordinator binding")
	}
	if bindErr == nil && !currentCoordinator(context, receipt, binding) {
		return installationResponse("COORDINATOR_PENDING", false, map[string]any{"phase": "handover_pending", "stage": "provisioning", "nextAction": "inspect provisioning"})
	}
	memoryPath := filepath.Join(context.Root, filepath.FromSlash(context.Paths["coordinator"]))
	if bindErr != nil {
		if _, e := os.Lstat(memoryPath); e == nil {
			memory, e := readCoordinatorMemoryContext(context)
			if e != nil {
				return setupFailure("COORDINATOR_REFUSED", "semantic_memory", "memory", "inspect restored memory")
			}
			if memory.Language != language {
				return setupFailure("COORDINATOR_REFUSED", "restored_language", "language", "select matching language")
			}
			if e = observeRestoredCoordinator(context, memory.CoordinatorID); e != nil {
				return installationResponse("COORDINATOR_PENDING", false, map[string]any{"reason": "restored_observation", "stage": "admission", "nextAction": "inspect restored memory"})
			}
			binding = coordinatorBinding{SchemaVersion: 1, Phase: "handover_pending", CoordinatorID: memory.CoordinatorID, InstallationID: receipt.InstallationID, DeviceID: receipt.DeviceID, CopyID: receipt.CopyID, Home: context.Root, ClientProject: project, Language: memory.Language, SourceCommit: info.SourceCommit}
		} else if !os.IsNotExist(e) {
			return setupFailure("COORDINATOR_REFUSED", "semantic_memory", "memory", "inspect restored memory")
		} else {
			binding = coordinatorBinding{SchemaVersion: 1, Phase: "handover_pending", CoordinatorID: uuid.NewString(), InstallationID: receipt.InstallationID, DeviceID: receipt.DeviceID, CopyID: receipt.CopyID, Home: context.Root, ClientProject: project, Language: language, SourceCommit: info.SourceCommit}
			text, e := readHomeText(context.Root, language)
			if e != nil {
				return setupFailure("COORDINATOR_REFUSED", "template_text", "factory", "verify matching kit")
			}
			if e = writeCoordinatorBinding(context, binding); e != nil {
				return installationResponse("COORDINATOR_PENDING", false, map[string]any{"stage": "intent", "nextAction": "inspect provisioning"})
			}
			if e = provisionCoordinator(context, binding, text); e != nil {
				return installationResponse("COORDINATOR_PENDING", false, map[string]any{"phase": "handover_pending", "stage": "provisioning", "nextAction": "inspect provisioning"})
			}
		}
	}
	if err := writeCoordinatorBinding(context, binding); err != nil {
		return installationResponse("COORDINATOR_PENDING", false, map[string]any{"phase": "handover_pending"})
	}
	promptPath := filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"]), "coordinator", "STARTUP.md")
	text, e := readHomeText(context.Root, binding.Language)
	if e != nil {
		return installationResponse("COORDINATOR_PENDING", false, map[string]any{"phase": "handover_pending"})
	}
	if e = writeStartup(promptPath, text.Startup, context, binding, memoryPath); e != nil {
		return installationResponse("COORDINATOR_PENDING", false, map[string]any{"phase": "handover_pending"})
	}
	return installationResponse("COORDINATOR_PREPARED", true, map[string]any{"phase": binding.Phase, "startupPromptPath": promptPath, "coordinatorMemoryPath": memoryPath})
}

func CoordinatorInspect(home string) Response {
	context, info, err := verifyFactoryHome(home)
	if err != nil {
		return setupFailure("COORDINATOR_REFUSED", "factory_payload", "factory", "inspect factory home")
	}
	receipt, currentErr := currentInstallation(context, info)
	if currentErr != nil {
		return installationResponse("COORDINATOR_REFUSED", false, map[string]any{})
	}
	binding, err := readCoordinatorBindingContext(context)
	if errors.Is(err, os.ErrNotExist) {
		return installationResponse("COORDINATOR_INSPECTED", true, map[string]any{"phase": "not_prepared"})
	}
	if err != nil {
		return installationResponse("COORDINATOR_REFUSED", false, map[string]any{})
	}
	if !currentCoordinator(context, receipt, binding) {
		return installationResponse("COORDINATOR_REFUSED", false, map[string]any{})
	}
	return installationResponse("COORDINATOR_INSPECTED", true, map[string]any{"phase": binding.Phase, "startupPromptPath": filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"]), "coordinator", "STARTUP.md"), "coordinatorMemoryPath": filepath.Join(context.Root, filepath.FromSlash(context.Paths["coordinator"]))})
}

func CoordinatorConfirm(home, project, source, evidence string) Response {
	context, info, err := verifyFactoryHome(home)
	if err != nil {
		return installationResponse("COORDINATOR_REFUSED", false, map[string]any{})
	}
	receipt, currentErr := currentInstallation(context, info)
	if currentErr != nil {
		return installationResponse("COORDINATOR_REFUSED", false, map[string]any{})
	}
	unlock, lockErr := installationAdmission(context)
	if lockErr != nil {
		return installationResponse("COORDINATOR_PENDING", false, map[string]any{})
	}
	defer unlock()
	if receipt, currentErr = currentInstallation(context, info); currentErr != nil {
		return installationResponse("COORDINATOR_REFUSED", false, map[string]any{"reason": "receipt_context", "stage": "admission", "nextAction": "inspect installation"})
	}
	binding, err := readCoordinatorBindingContext(context)
	if err != nil || binding.ClientProject != project || (source != "user-current" && source != "client-tool") || !bounded(evidence, 4096) {
		return installationResponse("COORDINATOR_REFUSED", false, map[string]any{})
	}
	if !currentCoordinator(context, receipt, binding) {
		return installationResponse("COORDINATOR_REFUSED", false, map[string]any{})
	}
	if binding.Phase == "confirmed" {
		if binding.EvidenceSource == source && binding.Evidence == evidence {
			return installationResponse("COORDINATOR_CONFIRMED", true, map[string]any{"phase": "confirmed"})
		}
		return installationResponse("COORDINATOR_REFUSED", false, map[string]any{})
	}
	if binding.Phase != "handover_pending" {
		return installationResponse("COORDINATOR_REFUSED", false, map[string]any{})
	}
	binding.Phase = "confirmed"
	binding.EvidenceSource = source
	binding.Evidence = evidence
	if err = writeCoordinatorBinding(context, binding); err != nil {
		return installationResponse("COORDINATOR_PENDING", false, map[string]any{})
	}
	return installationResponse("COORDINATOR_CONFIRMED", true, map[string]any{"phase": "confirmed"})
}

func bounded(s string, max int) bool { return strings.TrimSpace(s) != "" && len(s) <= max }
func currentInstallation(context layoutContext, info factoryInfo) (installationReceipt, error) {
	receipt, err := readInstallationReceiptContext(context)
	if err != nil || receipt.Phase != "installed" || receipt.SourceCommit != info.SourceCommit || !installationCoreMatches(context, receipt) {
		return installationReceipt{}, errors.New("installation")
	}
	return receipt, nil
}
func currentCoordinator(context layoutContext, receipt installationReceipt, binding coordinatorBinding) bool {
	if binding.InstallationID != receipt.InstallationID || binding.DeviceID != receipt.DeviceID || binding.CopyID != receipt.CopyID || binding.Home != context.Root || binding.SourceCommit != receipt.SourceCommit {
		return false
	}
	memory, err := readCoordinatorMemoryContext(context)
	if err != nil || memory.CoordinatorID != binding.CoordinatorID || memory.Language != binding.Language {
		return false
	}
	prompt := filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"]), "coordinator", "STARTUP.md")
	info, err := os.Lstat(prompt)
	return err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0
}
func readCoordinatorBinding(path string) (coordinatorBinding, error) {
	raw, e := os.ReadFile(path)
	if e != nil {
		return coordinatorBinding{}, e
	}
	var b coordinatorBinding
	if len(raw) > 1<<20 || !checkpointStrictJSON(raw, &b) || b.SchemaVersion != 1 || (b.Phase != "handover_pending" && b.Phase != "confirmed") || !isUUID(b.CoordinatorID) || !isUUID(b.InstallationID) || !isUUID(b.DeviceID) || !isUUID(b.CopyID) || !bounded(b.Home, 4096) || !bounded(b.ClientProject, 512) || (b.Language != "en" && b.Language != "ru") {
		return coordinatorBinding{}, errors.New("binding")
	}
	return b, nil
}
func readCoordinatorBindingContext(context layoutContext) (coordinatorBinding, error) {
	raw, e := installationRead(context.Root, context.Paths["bindings"]+"/coordinator/coordinator.json", 1<<20)
	if e != nil {
		return coordinatorBinding{}, e
	}
	var b coordinatorBinding
	if !checkpointStrictJSON(raw, &b) || !coordinatorBindingValid(b) {
		return coordinatorBinding{}, errors.New("binding")
	}
	return b, nil
}
func coordinatorBindingValid(b coordinatorBinding) bool {
	return b.SchemaVersion == 1 && (b.Phase == "handover_pending" || b.Phase == "confirmed") && isUUID(b.CoordinatorID) && isUUID(b.InstallationID) && isUUID(b.DeviceID) && isUUID(b.CopyID) && bounded(b.Home, 4096) && bounded(b.ClientProject, 512) && (b.Language == "en" || b.Language == "ru")
}
func writeCoordinatorBinding(context layoutContext, b coordinatorBinding) error {
	raw, _ := json.Marshal(b)
	return installationWrite(context.Root, context.Paths["bindings"]+"/coordinator/coordinator.json", raw)
}
func readCoordinatorMemory(path string) (coordinatorMemory, error) {
	raw, e := os.ReadFile(filepath.Join(path, "coordinator.json"))
	if e != nil {
		return coordinatorMemory{}, e
	}
	var m coordinatorMemory
	if len(raw) > 1<<20 || !checkpointStrictJSON(raw, &m) || m.SchemaVersion != 1 || !isUUID(m.CoordinatorID) || (m.Language != "en" && m.Language != "ru") {
		return coordinatorMemory{}, errors.New("memory")
	}
	for _, name := range []string{"AGENTS.md", "INDEX.md", "HANDOFF.md"} {
		if info, e := os.Lstat(filepath.Join(path, name)); e != nil || !info.Mode().IsRegular() {
			return coordinatorMemory{}, errors.New("memory")
		}
	}
	return m, nil
}
func readCoordinatorMemoryContext(context layoutContext) (coordinatorMemory, error) {
	raw, e := installationRead(context.Root, context.Paths["coordinator"]+"/coordinator.json", 1<<20)
	if e != nil {
		return coordinatorMemory{}, e
	}
	var m coordinatorMemory
	if !checkpointStrictJSON(raw, &m) || m.SchemaVersion != 1 || !isUUID(m.CoordinatorID) || (m.Language != "en" && m.Language != "ru") {
		return coordinatorMemory{}, errors.New("memory")
	}
	for _, name := range []string{"AGENTS.md", "INDEX.md", "HANDOFF.md"} {
		if _, e := installationRead(context.Root, context.Paths["coordinator"]+"/"+name, 1<<20); e != nil {
			return coordinatorMemory{}, errors.New("memory")
		}
	}
	return m, nil
}
func readHomeText(root, language string) (homeText, error) {
	path := "system/packaging/home/text.json"
	if language == "ru" {
		path = "system/locales/ru/packaging/home/text.json"
	}
	raw, e := installationRead(root, path, 1<<20)
	var t homeText
	if e != nil || !checkpointStrictJSON(raw, &t) || t.SchemaVersion != 1 || !bounded(t.RootReadme, 32768) || !bounded(t.RootAgent, 32768) || !bounded(t.SyncReadme, 32768) || !bounded(t.CoordinatorAgent, 32768) || !bounded(t.CoordinatorIndex, 32768) || !bounded(t.CoordinatorHandoff, 32768) || !bounded(t.Startup, 32768) || !strings.Contains(t.CoordinatorAgent, "<GUIDE_RELATIVE>") || !strings.Contains(t.Startup, "<HOME>") || !strings.Contains(t.Startup, "<MEMORY>") || !strings.Contains(t.Startup, "<GUIDE>") || !strings.Contains(t.Startup, "<PROJECT>") || !strings.Contains(t.Startup, "<SOURCE>") || !strings.Contains(t.Startup, "<LANGUAGE>") {
		return homeText{}, errors.New("text")
	}
	return t, nil
}
func provisionCoordinator(context layoutContext, b coordinatorBinding, t homeText) error {
	dir := context.Paths["coordinator"]
	if err := provisionResource(context, "coordinator-dir-"+b.CoordinatorID, "directory", dir, "", nil); err != nil {
		return err
	}
	memory, _ := json.Marshal(coordinatorMemory{SchemaVersion: 1, CoordinatorID: b.CoordinatorID, Language: b.Language})
	guide := "../../system/COORDINATOR.md"
	if b.Language == "ru" {
		guide = "../../system/locales/ru/COORDINATOR.md"
	}
	files := map[string][]byte{"coordinator.json": memory, "AGENTS.md": []byte(strings.ReplaceAll(t.CoordinatorAgent, "<GUIDE_RELATIVE>", guide)), "INDEX.md": []byte(t.CoordinatorIndex), "HANDOFF.md": []byte(t.CoordinatorHandoff)}
	for name, body := range files {
		if err := provisionResource(context, "coordinator-file-"+b.CoordinatorID+"-"+strings.TrimSuffix(name, ".md"), "file", dir+"/"+name, "coordinator-dir-"+b.CoordinatorID, body); err != nil {
			return err
		}
	}
	return nil
}
func provisionResource(context layoutContext, id, kind, locator, depends string, content []byte) error {
	rev, e := resourceRevision(context)
	if e != nil {
		return e
	}
	payload := map[string]any{"resourceId": id, "provider": "local", "kind": kind, "mode": "create", "locator": locator, "purpose": "coordinator bootstrap", "lifetime": "installation", "disposition": "keep", "ownerRef": "coordinator"}
	if depends != "" {
		payload["dependsOn"] = []string{depends}
	}
	raw, _ := json.Marshal(payload)
	intent := Handle(Request{SchemaVersion: 1, Action: "resource.intent", ProjectRoot: context.Root, RootKind: "installation", Actor: "coordinator", RequestID: id + "-intent", ExpectedRevision: rev, Payload: raw})
	if !intent.OK {
		return errors.New(intent.Code)
	}
	rev = intent.Revision
	raw, _ = json.Marshal(map[string]any{"resourceId": id, "contentBase64": base64.StdEncoding.EncodeToString(content)})
	apply := Handle(Request{SchemaVersion: 1, Action: "resource.apply", ProjectRoot: context.Root, RootKind: "installation", Actor: "coordinator", RequestID: id + "-apply", ExpectedRevision: rev, Payload: raw})
	if !apply.OK {
		return errors.New(apply.Code)
	}
	raw, _ = json.Marshal(map[string]any{"resourceId": id, "disposition": "keep", "reason": "installation bootstrap", "until": "until explicit coordinator retirement"})
	keep := Handle(Request{SchemaVersion: 1, Action: "resource.dispose", ProjectRoot: context.Root, RootKind: "installation", Actor: "coordinator", RequestID: id + "-keep", ExpectedRevision: apply.Revision, Payload: raw})
	if !keep.OK {
		return errors.New(keep.Code)
	}
	return nil
}
func resourceRevision(context layoutContext) (int64, error) {
	r := Handle(Request{SchemaVersion: 1, Action: "resource.list", ProjectRoot: context.Root, RootKind: "installation", Actor: "coordinator", RequestID: "coordinator-resource-list-" + uuid.NewString(), ExpectedRevision: 0, Payload: json.RawMessage(`{}`)})
	if !r.OK {
		return 0, errors.New(r.Code)
	}
	return r.Revision, nil
}
func writeStartup(path, template string, context layoutContext, b coordinatorBinding, memory string) error {
	guide := filepath.Join(context.Root, "system", "COORDINATOR.md")
	if b.Language == "ru" {
		guide = filepath.Join(context.Root, "system", "locales", "ru", "COORDINATOR.md")
	}
	replace := strings.NewReplacer("<HOME>", context.Root, "<MEMORY>", memory, "<GUIDE>", guide, "<PROJECT>", b.ClientProject, "<SOURCE>", b.SourceCommit, "<LANGUAGE>", b.Language)
	return installationWrite(context.Root, context.Paths["bindings"]+"/coordinator/STARTUP.md", []byte(replace.Replace(template)))
}
