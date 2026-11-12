package internal

import (
	"os"
	"path/filepath"
	"time"
)

type toolStorageMaintenanceInput struct {
	ToolID        string `json:"toolId"`
	ProfileSHA256 string `json:"profileSha256"`
	StoreKey      string `json:"storeKey"`
	MaxEntries    int    `json:"maxEntries"`
	MaxBytes      int64  `json:"maxBytes"`
}

func toolStorageMaintenanceInspect(c layoutContext, r Request) Response {
	var in toolStorageMaintenanceInput
	if !toolStorageExact(r.Payload, &in, "toolId", "profileSha256", "storeKey", "maxEntries", "maxBytes") || (in.ToolID != "go" && in.ToolID != "npm") || !toolStorageHash(in.ProfileSHA256) || !toolStorageKey.MatchString(in.StoreKey) || in.MaxEntries < 1 || in.MaxEntries > 1024 || in.MaxBytes < 1 || in.MaxBytes > toolStorageRelocationLimit {
		return toolStorageFailure("input", "maintenance.inspect", "review selected cache input")
	}
	if raw, err := installationRead(c.Root, toolStorageRel(c, toolStorageRelocationStatePath(c)), toolStorageRelocationLimit); err == nil {
		var state toolStorageRelocationPlan
		if !checkpointStrictJSON(raw, &state) || !toolStorageRelocationTerminalValid(c, state) {
			return toolStorageFailure("pending_transition", "maintenance.inspect", "inspect or recover retained relocation")
		}
	} else if !os.IsNotExist(err) {
		return toolStorageFailure("transition_state", "maintenance.inspect", "inspect retained relocation")
	}
	p, _, profileID, _, err := toolStorageRelocationProfile(c, in.ToolID, in.ProfileSHA256)
	if err != nil || p.SchemaVersion != 2 {
		return toolStorageFailure("profile", "maintenance.inspect", "inspect current native profile")
	}
	if _, err = toolStorageAuthoringReceiptProof(toolStorageRef{Home: c.Root, ToolID: in.ToolID, SHA256: in.ProfileSHA256}, p, profileID); err != nil {
		return toolStorageFailure("receipt", "maintenance.inspect", "inspect current native receipt")
	}
	var selected *toolStorageStore
	for i := range p.Stores {
		if p.Stores[i].Key == in.StoreKey && p.Stores[i].Kind == "cache" {
			selected = &p.Stores[i]
			break
		}
	}
	if selected == nil {
		return toolStorageFailure("cache_store", "maintenance.inspect", "select a current cache store")
	}
	uses, err := toolStorageRelocationUses(c, p)
	if err != nil {
		return toolStorageFailure("use_registry", "maintenance.inspect", "inspect retained use registry")
	}
	root := filepath.Join(p.ToolDataRoot, filepath.FromSlash(selected.Path))
	entries, err := toolStorageRelocationInventory(root, in.MaxEntries, in.MaxBytes)
	if err != nil {
		return toolStorageFailure("inventory", "maintenance.inspect", "use a supported bounded provider or preserve the cache unchanged")
	}
	var bytes int64
	oldest := int64(0)
	now := time.Now()
	held, _, err := openHeldResourceRoot(root, ".", selected.NativeIdentity)
	if err != nil {
		return toolStorageFailure("identity", "maintenance.inspect", "inspect selected cache")
	}
	defer held.Close()
	for _, entry := range entries {
		if entry.Directory {
			continue
		}
		bytes += entry.Size
		f, e := held.Open(filepath.FromSlash(entry.Path))
		if e != nil {
			return toolStorageFailure("identity", "maintenance.inspect", "inspect selected cache")
		}
		id, idErr := localNativeIDFile(f)
		info, statErr := f.Stat()
		closeErr := f.Close()
		if idErr != nil || statErr != nil || closeErr != nil || id != entry.NativeIdentity || info.Size() != entry.Size {
			return toolStorageFailure("identity", "maintenance.inspect", "inspect selected cache")
		}
		age := int64(now.Sub(info.ModTime()).Hours() / 24)
		if age < 0 {
			age = 0
		}
		if age > oldest {
			oldest = age
		}
	}
	return installationResponse("TOOL_STORAGE_MAINTENANCE_INSPECTED", true, map[string]any{"toolId": in.ToolID, "storeKey": in.StoreKey, "owner": p.Owner, "purpose": p.Purpose, "retention": p.Retention, "observedEntries": len(entries), "observedBytes": bytes, "oldestAgeDays": oldest, "sizeLimitBytes": p.MaxBytes, "ageLimitDays": p.MaxAgeDays, "sizeExceeded": bytes > p.MaxBytes, "ageExceeded": oldest > int64(p.MaxAgeDays), "maintenanceRequired": bytes > p.MaxBytes || oldest > int64(p.MaxAgeDays), "activeUses": uses, "pendingUses": 0, "mutationSupported": false, "nextAction": "review preserving cache maintenance"})
}
