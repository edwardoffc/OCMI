package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// updateMetadataProof is a bounded snapshot; it gives no authority to mutate a
// home until the transaction also verifies its public and stage proofs.
type updateMetadataProof struct {
	SchemaVersion int    `json:"schemaVersion"`
	Home          string `json:"home"`
	HomeID        string `json:"homeId"`
	OldSource     string `json:"oldSource"`
	NewSource     string `json:"newSource"`
	ReceiptBefore []byte `json:"receiptBefore"`
	ReceiptAfter  []byte `json:"receiptAfter"`
	BindingBefore []byte `json:"bindingBefore,omitempty"`
	BindingAfter  []byte `json:"bindingAfter,omitempty"`
	PromptBefore  []byte `json:"promptBefore,omitempty"`
	PromptAfter   []byte `json:"promptAfter,omitempty"`
	OldTemplate   string `json:"oldTemplate,omitempty"`
	NewTemplate   string `json:"newTemplate,omitempty"`
}

// updateProofJournal carries metadata evidence for the later transaction
// layer; it does not by itself authorize activation or recovery.
type updateProofJournal struct {
	SchemaVersion     int                   `json:"schemaVersion"`
	ID                string                `json:"id"`
	Phase             string                `json:"phase"`
	Metadata          updateMetadataProof   `json:"metadata"`
	OldSource         string                `json:"oldSource"`
	NewSource         string                `json:"newSource"`
	OldPublic         updatePublicProof     `json:"oldPublic"`
	NewPublic         updatePublicProof     `json:"newPublic"`
	Stage             string                `json:"stage"`
	StageID           string                `json:"stageId"`
	OldSlot           string                `json:"oldSlot"`
	OldSlotID         string                `json:"oldSlotId"`
	NewSlot           string                `json:"newSlot"`
	NewSlotID         string                `json:"newSlotId"`
	AdmissionID       string                `json:"admissionId"`
	OldCopied         int                   `json:"oldCopied"`
	NewCopied         int                   `json:"newCopied"`
	OldStaged         *updatePublicProof    `json:"oldStaged,omitempty"`
	NewStaged         *updatePublicProof    `json:"newStaged,omitempty"`
	RootProgress      int                   `json:"rootProgress"`
	MetadataProgress  int                   `json:"metadataProgress"`
	RecoveryFrom      string                `json:"recoveryFrom,omitempty"`
	RecoveryStep      int                   `json:"recoveryStep"`
	CleanupState      string                `json:"cleanupState,omitempty"`
	CleanupEntries    []updateCleanupEntry  `json:"cleanupEntries,omitempty"`
	Previous          *updateProofJournal   `json:"previous,omitempty"`
	RetirementState   string                `json:"retirementState,omitempty"`
	RetirementEntries []updateCleanupEntry  `json:"retirementEntries,omitempty"`
	LastFailure       *updateGenerationFact `json:"lastFailure,omitempty"`
	LastRetired       *updateGenerationFact `json:"lastRetired,omitempty"`
}

type updateGenerationFact struct {
	ID           string `json:"id"`
	OldSource    string `json:"oldSource"`
	NewSource    string `json:"newSource"`
	Phase        string `json:"phase"`
	RecoveryFrom string `json:"recoveryFrom,omitempty"`
}

func decodeUpdateProofJournal(raw []byte) (updateProofJournal, error) {
	var j updateProofJournal
	if len(raw) > 16<<20 || !checkpointStrictJSON(raw, &j) || j.SchemaVersion != 1 || !isUUID(j.ID) {
		return updateProofJournal{}, errors.New("invalid proof journal")
	}
	switch j.Phase {
	case "intent", "staged", "old_system", "new_system", "root_switch", "roots", "metadata_switch", "metadata", "applied", "recovering", "rolled_back":
	default:
		return updateProofJournal{}, errors.New("invalid proof journal phase")
	}
	if _, err := encodeUpdateMetadataProof(j.Metadata); err != nil {
		return updateProofJournal{}, err
	}
	if _, err := encodeUpdatePublicProof(j.OldPublic); err != nil {
		return updateProofJournal{}, err
	}
	if _, err := encodeUpdatePublicProof(j.NewPublic); err != nil {
		return updateProofJournal{}, err
	}
	if j.OldSource != j.Metadata.OldSource || j.NewSource != j.Metadata.NewSource || j.OldPublic.RootID != j.Metadata.HomeID || j.Stage != "local/runs/update-"+j.ID || j.OldSlot != "old" || j.NewSlot != "new" || j.AdmissionID == "" || len(j.AdmissionID) > 512 || len(j.StageID) > 512 || len(j.OldSlotID) > 512 || len(j.NewSlotID) > 512 || j.OldCopied < 0 || j.NewCopied < 0 || j.OldCopied > len(j.OldPublic.Files) || j.NewCopied > len(j.NewPublic.Files) {
		return updateProofJournal{}, errors.New("inconsistent proof journal")
	}
	if j.Phase != "intent" && !(j.Phase == "rolled_back" && j.RecoveryFrom == "intent") && (j.StageID == "" || j.OldSlotID == "" || j.NewSlotID == "" || j.OldCopied != len(j.OldPublic.Files) || j.NewCopied != len(j.NewPublic.Files)) {
		return updateProofJournal{}, errors.New("incomplete staged proof")
	}
	if (j.OldSlotID != "" || j.NewSlotID != "") && j.StageID == "" {
		return updateProofJournal{}, errors.New("orphan slot identity")
	}
	if (j.OldCopied > 0 && j.OldSlotID == "") || (j.NewCopied > 0 && j.NewSlotID == "") {
		return updateProofJournal{}, errors.New("orphan copy progress")
	}
	if (j.OldStaged == nil) != (j.NewStaged == nil) {
		return updateProofJournal{}, errors.New("incomplete staged native proof")
	}
	if j.OldStaged != nil {
		if _, err := encodeUpdatePublicProof(*j.OldStaged); err != nil {
			return updateProofJournal{}, err
		}
		if _, err := encodeUpdatePublicProof(*j.NewStaged); err != nil {
			return updateProofJournal{}, err
		}
		if j.OldStaged.RootID != j.OldSlotID || j.NewStaged.RootID != j.NewSlotID {
			return updateProofJournal{}, errors.New("staged slot proof identity")
		}
		if !sameUpdatePublicContent(j.OldPublic, *j.OldStaged) || !sameUpdatePublicContent(j.NewPublic, *j.NewStaged) {
			return updateProofJournal{}, errors.New("staged content differs from captured proof")
		}
	}
	if (j.Phase == "old_system" || j.Phase == "new_system" || j.Phase == "root_switch" || j.Phase == "roots" || j.Phase == "metadata_switch" || j.Phase == "metadata" || j.Phase == "applied") && j.OldStaged == nil {
		return updateProofJournal{}, errors.New("missing staged native proof")
	}
	if j.RecoveryStep < 0 || j.RecoveryStep > 2*len(j.OldPublic.Files)+16 {
		return updateProofJournal{}, errors.New("invalid recovery progress")
	}
	if err := validateUpdateCleanupJournal(j); err != nil {
		return updateProofJournal{}, err
	}
	for _, fact := range []*updateGenerationFact{j.LastFailure, j.LastRetired} {
		if fact != nil && (!isUUID(fact.ID) || !validCommit(fact.OldSource) || !validCommit(fact.NewSource) || (fact.Phase != "rolled_back" && fact.Phase != "retired")) {
			return updateProofJournal{}, errors.New("invalid generation fact")
		}
		if fact != nil && fact.RecoveryFrom != "" {
			switch fact.RecoveryFrom {
			case "intent", "staged", "old_system", "new_system", "root_switch", "roots", "metadata_switch", "metadata", "applied":
			default:
				return updateProofJournal{}, errors.New("invalid recovery fact")
			}
		}
	}
	if j.Previous != nil {
		p := j.Previous
		if p.Previous != nil || p.RetirementState != "" || len(p.RetirementEntries) != 0 || p.Phase != "applied" || p.CleanupState != "" || p.ID == j.ID || p.Metadata.Home != j.Metadata.Home || p.Metadata.HomeID != j.Metadata.HomeID || p.NewSource != j.OldSource || !sameUpdatePublicContent(p.NewPublic, j.OldPublic) {
			return updateProofJournal{}, errors.New("invalid previous generation")
		}
		rawPrevious, err := json.Marshal(p)
		if err != nil {
			return updateProofJournal{}, err
		}
		if _, err = decodeUpdateProofJournal(rawPrevious); err != nil {
			return updateProofJournal{}, err
		}
	}
	if err := validateRetirementJournal(j); err != nil {
		return updateProofJournal{}, err
	}
	if j.Phase == "recovering" {
		switch j.RecoveryFrom {
		case "staged", "old_system", "new_system", "root_switch", "roots", "metadata_switch", "metadata", "applied":
		default:
			return updateProofJournal{}, errors.New("invalid recovery origin")
		}
		if j.OldStaged == nil {
			return updateProofJournal{}, errors.New("missing recovery proof")
		}
	} else if j.RecoveryFrom != "" && j.Phase != "rolled_back" {
		return updateProofJournal{}, errors.New("unexpected recovery origin")
	}
	rootCount := 0
	if j.NewStaged != nil {
		for _, e := range j.NewStaged.Files {
			if !strings.HasPrefix(e.Path, "system/") {
				rootCount++
			}
		}
	}
	metadataCount := 1
	if len(j.Metadata.BindingBefore) > 0 {
		metadataCount = 3
	}
	if j.RootProgress < 0 || j.RootProgress > rootCount || j.MetadataProgress < 0 || j.MetadataProgress > metadataCount {
		return updateProofJournal{}, errors.New("invalid activation progress")
	}
	if (j.Phase == "roots" || j.Phase == "metadata_switch" || j.Phase == "metadata" || j.Phase == "applied") && j.RootProgress != rootCount {
		return updateProofJournal{}, errors.New("incomplete root progress")
	}
	if (j.Phase == "metadata" || j.Phase == "applied") && j.MetadataProgress != metadataCount {
		return updateProofJournal{}, errors.New("incomplete metadata progress")
	}
	return j, nil
}

func sameUpdatePublicContent(a, b updatePublicProof) bool {
	if len(a.Files) != len(b.Files) {
		return false
	}
	for i, x := range a.Files {
		y := b.Files[i]
		if x.Path != y.Path || x.Digest != y.Digest || x.Size != y.Size || x.Mode != y.Mode {
			return false
		}
	}
	return true
}

func encodeUpdateProofJournal(j updateProofJournal) ([]byte, error) {
	raw, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	if _, err = decodeUpdateProofJournal(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func verifyUpdateMetadataBefore(context layoutContext, p updateMetadataProof) error {
	id, err := localNativeID(context.Root)
	if err != nil || context.Root != p.Home || id != p.HomeID {
		return errors.New("home identity changed")
	}
	for rel, expected := range map[string][]byte{
		context.Paths["bindings"] + "/" + installationReceiptName:   p.ReceiptBefore,
		context.Paths["bindings"] + "/coordinator/coordinator.json": p.BindingBefore,
		context.Paths["bindings"] + "/coordinator/STARTUP.md":       p.PromptBefore,
	} {
		raw, err := installationRead(context.Root, rel, 1<<20)
		if len(expected) == 0 && os.IsNotExist(err) {
			continue
		}
		if err != nil || !bytes.Equal(raw, expected) {
			return errors.New("metadata changed")
		}
	}
	return nil
}

func captureUpdateMetadata(context layoutContext, info factoryInfo, nextSource, nextRoot string) (updateMetadataProof, error) {
	var p updateMetadataProof
	if !filepath.IsAbs(context.Root) || !validCommit(nextSource) {
		return p, errors.New("invalid proof input")
	}
	id, err := localNativeID(context.Root)
	if err != nil || id == "" {
		return p, errors.New("home identity")
	}
	receipt, err := currentInstallation(context, info)
	if err != nil {
		return p, err
	}
	db, err := openLayout(context)
	if err != nil {
		return p, err
	}
	defer db.Close()
	version, err := databaseVersion(db)
	if err != nil || version != stateSchemaVersion {
		return p, errors.New("state schema")
	}
	before, err := installationRead(context.Root, context.Paths["bindings"]+"/"+installationReceiptName, 1<<20)
	if err != nil {
		return p, err
	}
	receipt.SourceCommit = nextSource
	after, err := json.Marshal(receipt)
	if err != nil {
		return p, err
	}
	p = updateMetadataProof{SchemaVersion: 1, Home: context.Root, HomeID: id, OldSource: info.SourceCommit, NewSource: nextSource, ReceiptBefore: before, ReceiptAfter: after}
	binding, err := readCoordinatorBindingContext(context)
	if os.IsNotExist(err) {
		for _, rel := range []string{context.Paths["bindings"] + "/coordinator"} {
			if _, e := os.Lstat(filepath.Join(context.Root, filepath.FromSlash(rel))); e == nil || !os.IsNotExist(e) {
				return updateMetadataProof{}, errors.New("partial coordinator")
			}
		}
		return p, nil
	}
	if err != nil {
		return updateMetadataProof{}, err
	}
	oldReceipt, err := readInstallationReceiptContext(context)
	if err != nil || !currentCoordinator(context, oldReceipt, binding) {
		return updateMetadataProof{}, errors.New("coordinator context")
	}
	p.BindingBefore, err = installationRead(context.Root, context.Paths["bindings"]+"/coordinator/coordinator.json", 1<<20)
	if err != nil {
		return updateMetadataProof{}, err
	}
	p.PromptBefore, err = installationRead(context.Root, context.Paths["bindings"]+"/coordinator/STARTUP.md", 1<<20)
	if err != nil {
		return updateMetadataProof{}, err
	}
	oldText, err := readHomeText(context.Root, binding.Language)
	if err != nil {
		return updateMetadataProof{}, err
	}
	memory := filepath.Join(context.Root, filepath.FromSlash(context.Paths["coordinator"]))
	if !bytes.Equal(p.PromptBefore, renderStartup(oldText.Startup, context, binding, memory)) {
		return updateMetadataProof{}, errors.New("changed prompt")
	}
	newText, err := readHomeText(nextRoot, binding.Language)
	if err != nil {
		return updateMetadataProof{}, err
	}
	binding.SourceCommit = nextSource
	p.BindingAfter, err = json.Marshal(binding)
	if err != nil {
		return updateMetadataProof{}, err
	}
	p.PromptAfter = renderStartup(newText.Startup, context, binding, memory)
	p.OldTemplate, p.NewTemplate = oldText.Startup, newText.Startup
	return p, nil
}

func encodeUpdateMetadataProof(p updateMetadataProof) ([]byte, error) {
	raw, err := json.Marshal(p)
	if err != nil || len(raw) > 4<<20 {
		return nil, errors.New("proof size")
	}
	if _, err = decodeUpdateMetadataProof(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func decodeUpdateMetadataProof(raw []byte) (updateMetadataProof, error) {
	var p updateMetadataProof
	if len(raw) > 4<<20 || !checkpointStrictJSON(raw, &p) || p.SchemaVersion != 1 || !filepath.IsAbs(p.Home) || p.HomeID == "" || !validCommit(p.OldSource) || !validCommit(p.NewSource) || len(p.ReceiptBefore) == 0 || len(p.ReceiptAfter) == 0 || len(p.ReceiptBefore) > 1<<20 || len(p.ReceiptAfter) > 1<<20 {
		return updateMetadataProof{}, errors.New("invalid proof")
	}
	if (len(p.BindingBefore) == 0) != (len(p.BindingAfter) == 0) || (len(p.BindingBefore) == 0) != (len(p.PromptBefore) == 0) || (len(p.BindingBefore) == 0) != (len(p.PromptAfter) == 0) || len(p.BindingBefore) > 1<<20 || len(p.BindingAfter) > 1<<20 || len(p.PromptBefore) > 1<<20 || len(p.PromptAfter) > 1<<20 {
		return updateMetadataProof{}, errors.New("invalid coordinator proof")
	}
	var oldReceipt, newReceipt installationReceipt
	if !checkpointStrictJSON(p.ReceiptBefore, &oldReceipt) || !checkpointStrictJSON(p.ReceiptAfter, &newReceipt) || !receiptValid(oldReceipt) || !receiptValid(newReceipt) || oldReceipt.Home != p.Home || oldReceipt.HomeID != p.HomeID || oldReceipt.SourceCommit != p.OldSource || newReceipt.SourceCommit != p.NewSource {
		return updateMetadataProof{}, errors.New("invalid receipt proof")
	}
	oldReceipt.SourceCommit = p.NewSource
	if !reflect.DeepEqual(oldReceipt, newReceipt) {
		return updateMetadataProof{}, errors.New("receipt changed beyond source")
	}
	if len(p.BindingBefore) != 0 {
		var oldBinding, newBinding coordinatorBinding
		if !checkpointStrictJSON(p.BindingBefore, &oldBinding) || !checkpointStrictJSON(p.BindingAfter, &newBinding) || !coordinatorBindingValid(oldBinding) || !coordinatorBindingValid(newBinding) || oldBinding.Home != p.Home || oldBinding.SourceCommit != p.OldSource || newBinding.SourceCommit != p.NewSource || oldBinding.InstallationID != oldReceipt.InstallationID || oldBinding.DeviceID != oldReceipt.DeviceID || oldBinding.CopyID != oldReceipt.CopyID {
			return updateMetadataProof{}, errors.New("invalid binding proof")
		}
		oldBinding.SourceCommit = p.NewSource
		if !reflect.DeepEqual(oldBinding, newBinding) {
			return updateMetadataProof{}, errors.New("binding changed beyond source")
		}
		declaration, err := strictDeclaration(installationLayoutJSON)
		if err != nil || !bounded(p.OldTemplate, 32768) || !bounded(p.NewTemplate, 32768) {
			return updateMetadataProof{}, errors.New("invalid template proof")
		}
		context := layoutContext{Root: p.Home, Paths: declaration.Paths}
		memory := filepath.Join(p.Home, filepath.FromSlash(context.Paths["coordinator"]))
		oldBinding.SourceCommit = p.OldSource
		if !bytes.Equal(p.PromptBefore, renderStartup(p.OldTemplate, context, oldBinding, memory)) || !bytes.Equal(p.PromptAfter, renderStartup(p.NewTemplate, context, newBinding, memory)) {
			return updateMetadataProof{}, errors.New("invalid prompt proof")
		}
	} else if p.OldTemplate != "" || p.NewTemplate != "" {
		return updateMetadataProof{}, errors.New("unexpected template proof")
	}
	return p, nil
}
