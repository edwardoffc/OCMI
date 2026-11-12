package main

import (
	"github.com/edwardoffc/OCMI/runtime/internal"
	"strings"
)

const lifecycleActions = "init status layout.inspect context.inspect task.bind task.revise task.ready task.history outcome.reassess outcome.review outcome.decide stop run submit review integrate resource.close resource.intent resource.apply resource.write resource.use resource.release resource.hold resource.dispose resource.cancel resource.reconcile resource.rebind resource.inspect resource.list resource.maintain.inspect operation.inspect operation.output operation.reconcile state.upgrade handover.import handover.export handover.cancel handover.ack handover.rebind"

func help(args []string) internal.Response {
	action := ""
	if len(args) > 1 {
		return setupUsage()
	}
	if len(args) == 1 {
		action = args[0]
	}
	if action == "unpack" {
		return internal.Response{SchemaVersion: 1, OK: true, Code: "HELP", Data: map[string]any{
			"usage":       "unpack --archive ABS --destination ABS --expected-sha256 HEX64",
			"inspect":     "unpack --destination ABS --inspect",
			"constraints": "Use a separately verified matching native launcher and trusted archive SHA256. Destination must be absent; the launcher creates it. Existing paths, including empty directories, are preserved. Inspection is read-only; it does not grant ownership or initialize an installation.",
			"guides":      []string{"docs/install-system.md"},
		}}
	}
	if action != "" && action != "call" && action != "tool-storage.inspect" && !strings.Contains(" "+lifecycleActions+" ", " "+action+" ") {
		return internal.Response{SchemaVersion: 1, Code: "UNKNOWN_ACTION", Data: map[string]any{}}
	}
	examples := map[string]any{
		"resource.intent":           map[string]any{"resourceId": "prepared-checks", "provider": "local", "kind": "directory", "mode": "create", "locator": "prepared-checks", "purpose": "selected temporary checks", "lifetime": "until owner review", "disposition": "remove", "ownerRef": "declared owner"},
		"resource.apply":            map[string]any{"resourceId": "prepared-checks"},
		"resource.write":            map[string]any{"resourceId": "prepared-checks", "path": "check.txt", "contentBase64": "eA=="},
		"resource.inspect":          map[string]any{"resourceId": "RESOURCE_ID"},
		"resource.list":             map[string]any{"taskId": "TASK_ID", "limit": 100, "cursor": 0},
		"resource.dispose":          map[string]any{"resourceId": "OWNED_RESOURCE_ID", "disposition": "remove", "reason": "reviewed unchanged owned output"},
		"resource.release":          map[string]any{"resourceId": "RESOURCE_ID", "reason": "use finished"},
		"resource.maintain.inspect": map[string]any{"resourceId": "SELECTED_RESOURCE_ID", "sizeLimitBytes": 1048576, "ageLimitDays": 7, "maxEntries": 256, "maxBytes": 67108864},
		"resource.rebind":           map[string]any{"resourceId": "RESOURCE_ID", "locator": "ABSOLUTE_EXISTING_PATH", "ownerRef": "declared owner", "reason": "explicit external use binding"},
		"resource.reconcile":        map[string]any{"resourceId": "ORIGINAL_MANAGED_WORKSPACE_ID", "inventorySHA256": "REVIEWED_SNAPSHOT_SHA256", "reason": "reviewed exact crash diagnostics"},
		"tool-storage.inspect":      map[string]any{"toolId": "go"},
		"task.bind":                 map[string]any{"taskId": "task", "outcomeId": "outcome", "taskRevision": 1, "completionScope": "target", "worker": "worker", "reviewer": "owner", "contract": "contract.md", "inputs": []string{"input.txt"}, "command": map[string]any{"id": "check", "executable": "ABSOLUTE_EXECUTABLE", "args": []string{}, "cwd": ".", "timeoutMs": 5000, "outputLimitBytes": 4096, "outputPolicy": "private"}},
		"operation.output":          map[string]any{"operationId": "REGISTERED_OPERATION_ID", "limitBytes": 4096},
		"init":                      map[string]any{"projectId": "project", "deviceId": "device", "copyId": "copy", "owner": "owner"},
		"task.ready":                map[string]any{}, "run": map[string]any{}, "submit": map[string]any{}, "review": map[string]any{"accept": true, "reason": "reviewed evidence"}, "integrate": map[string]any{"candidate": "reviewed candidate", "reason": "accepted evidence"}, "resource.close": map[string]any{"disposition": "keep", "reason": "retained evidence"},
	}
	data := map[string]any{"action": action, "usage": "call --root ABS --actor ACTOR --action ACTION [--revision N] [--payload JSON] [--request-id ID] [--root-kind project|installation]", "required": []string{"--root", "--actor", "--action", "--revision for mutations"}, "guides": []string{"docs/runtime.md", "docs/continuation.md"}, "constraints": "Read observations default revision 0. Mutations require explicit revision; no retry or refresh. Retain the original call: automatic mutation IDs are reproducible from that invocation, including revision; explicit requestId remains supported."}
	if action == "" {
		data["actions"] = strings.Fields(lifecycleActions)
		data["setup"] = []string{"help unpack", "unpack", "install", "coordinator", "update"}
		data["examples"] = examples
	} else if action != "call" {
		data["invocation"] = "call --root ABS --actor ACTOR --action " + action + " [--revision N] --payload JSON"
		if example, ok := examples[action]; ok {
			data["example"] = example
		} else {
			data["schemaGuidance"] = "No embedded payload schema for this action; consult docs/runtime.md and the action-specific guide before calling."
		}
	}
	if action == "operation.output" {
		data["outputPolicy"] = "Owner must deliberately opt in with outputPolicy shareable in the frozen command after reviewing disclosure. Private is the starter default. This is not a DLP boundary; complete describes transcript only. limitBytes defaults to 4096, maximum 65536."
	}
	switch action {
	case "resource.intent", "resource.apply", "resource.write":
		data["guides"] = []string{"docs/runtime-resources.md"}
		data["preparationAttribution"] = "Intent/apply do not require a bound task; without one they record empty taskId/revision 0. Write requires loadable current task, even for owner. A closed integrated task remains current until replaced: new resource events inherit that task/revision, not a future assignment. These calls do not bind/reopen tasks or grant a preparation scope."
		data["pathAndContentConstraints"] = "Create uses a fresh plain relative locator with existing plain parents; no overwrite/adoption, absolute paths, backslashes, '..', .git or .ocmi. Installation-home creates stay under OCMI_Sync/. Directory writes exclusively create a plain relative child and refresh group inventory; no ancestor mkdir. Standard Base64 content defaults to empty; encoded length at most 2 MiB. Ordinary inventory/provenance/STOP gates remain."
		switch action {
		case "resource.intent":
			data["requiredPayloadFields"] = []string{"resourceId", "provider", "kind", "mode", "locator", "purpose", "lifetime", "disposition", "ownerRef"}
			data["optionalPayloadFields"] = map[string]any{"dependsOn": "At most 32 available resource IDs; no self-dependency."}
			data["fileExample"] = map[string]any{"resourceId": "prepared-file", "provider": "local", "kind": "file", "mode": "create", "locator": "prepared-check.txt", "purpose": "selected temporary check", "lifetime": "until owner review", "disposition": "remove", "ownerRef": "declared owner"}
			data["role"] = "Owner only; STOP blocks intent. Examples declare new local owned resources; ownerRef is metadata, not an actor grant. Resource IDs beginning ocmi- are reserved. Intent precedes apply; other providers do not gain a supported local effect."
		case "resource.apply":
			data["requiredPayloadFields"] = []string{"resourceId"}
			data["optionalPayloadFields"] = map[string]any{"contentBase64": "Initial standalone-file bytes; omitted/empty creates an empty file. Directory apply creates the directory and ownership marker."}
			data["fileExample"] = map[string]any{"resourceId": "prepared-file", "contentBase64": "eA=="}
			data["role"] = "Owner only; STOP blocks apply. Requires an unissued local intent. Native create is exclusive; inspect the result before writing or disposition. Preserve an uncertain/issued effect instead of resetting it or retrying with a new ID."
		case "resource.write":
			data["requiredPayloadFields"] = []string{"resourceId", "path"}
			data["optionalPayloadFields"] = map[string]any{"contentBase64": "Bytes for the new child; omitted/empty creates an empty file. eA== encodes the synthetic byte x."}
			data["role"] = "Current task must be loadable; actor must be its worker or project owner. No ready/running state is required. STOP blocks write. Requires an active original local directory with unchanged native identity/inventory; this is not a standalone-file overwrite API."
		}
	case "task.bind", "task.revise":
		data["optionalCommandFields"] = map[string]any{"toolStorage": map[string]any{"home": "ABSOLUTE_APPROVED_INSTALLATION_HOME", "toolId": "go", "sha256": "APPROVED_PROFILE_SHA256"}}
		data["toolStorageGuidance"] = "Optional strict reference: exactly home, toolId, sha256. The active receipt's profileSha256 maps to reference sha256. Reuse an explicitly supplied approved reference; a project actor cannot inspect installation coordinator state or repair profile authority."
		data["guides"] = []string{"docs/runtime.md", "docs/runtime-tool-storage.md"}
	case "context.inspect":
		data["textFormat"] = "For read-only compact/start only: add --format text to call or request. Refusals and unsafe terminal text use full JSON. Default is JSON."
		data["startExample"] = map[string]any{"mode": "start", "taskId": "TASK_ID", "taskRevision": 1, "worker": "ASSIGNED_WORKER"}
		data["compactExample"] = map[string]any{"mode": "compact", "files": []string{"rules.md"}, "requiredFiles": []string{"input.txt"}, "budgetBytes": 4096}
		data["optionalCompactFields"] = map[string]any{"receipt": "Opaque base64url receipt returned by core; supply only for context retained in this chat. It is metadata, not authority.", "expectedBinding": "data.bindingRef from the explicitly delivered START; core checks the fresh ready binding."}
		data["schemaGuidance"] = "Read-only modes: summary, selected, start, compact. start requires taskId/taskRevision/worker and separately lists data.metadataOnlyFiles. compact accepts only mode/files/requiredFiles/budgetBytes/receipt/expectedBinding; no terms. Whole active Contract and requiredFiles are delivered only within allowed text scope; reusable files may be retained via receipt after fresh verification. Exact bound project-root .gitignore/.gitattributes are metadata only. Explicit text requests refuse with CONTEXT_METADATA_ONLY: omit them from text only if the semantic Contract does not require their contents; otherwise obtain an agreed readable original. A metadata-only active Contract also refuses safely. Budget defaults to 4096 UTF-8 bytes, range 128..65536; insufficient budget refuses without truncation. State, provenance and binding are observations, not task or permission grants."
	case "resource.inspect", "resource.list":
		data["role"] = "Read-only observation; explicit actor and selected root. list taskId optional; limit defaults/caps at 100; cursor is nonnegative."
		data["guides"] = []string{"docs/runtime-resources.md"}
	case "resource.rebind":
		data["role"] = "Owner only; explicit current revision. Existing local object becomes external use only, never deletion authority."
		data["guides"] = []string{"docs/runtime-resources.md"}
	case "tool-storage.inspect":
		data["role"] = "Read-only verified installation home; toolId is the selected tool slug."
		data["guides"] = []string{"docs/runtime-tool-storage.md"}
	case "resource.reconcile":
		data["supported"] = true
		data["supportedScope"] = "original-managed-crash-workspace-only"
		data["genericProviderSupported"] = false
		data["schemaGuidance"] = "Owner only, current revision: original automatic local crash-workspace recovery with resourceId, reason and inventorySHA256 from reviewed resource.inspect recovery entries. Rechecks native provenance, registered process/group absence, uses/holds and exact inventory. Retains diagnostics; separate resource.dispose deletes them. Other providers or ambiguous/imported/use-only objects are not adopted."
		data["guides"] = []string{"docs/runtime-resources.md"}
	case "resource.maintain.inspect":
		data["role"] = "Read-only bounded metadata for one original owned local group or protected internal state. Thresholds request review, never deletion or ownership. Unsupported/changed/over-bound stays incomplete."
		data["limits"] = map[string]any{"maxEntries": 1024, "maxBytes": 1 << 50, "ageBasis": "oldest file modification time in whole days, not disuse or TTL", "mutationSupported": false}
		data["guides"] = []string{"docs/runtime-resources.md", "docs/runtime-tool-storage.md"}
	case "resource.dispose":
		data["role"] = "Owner only; disposition is required. Removal retains existing native/inventory/use/hold gates. Inspection thresholds confer no deletion authority."
	case "resource.release":
		data["role"] = "Release the caller's own task use. Owner takeover of another actor's use requires the recorded stopped-operation proof; it is not an arbitrary lease reset."
	}
	return internal.Response{SchemaVersion: 1, OK: true, Code: "HELP", Data: data}
}
