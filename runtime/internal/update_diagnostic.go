package internal

import (
	"errors"
	"syscall"
)

// Proof observations travel with the original error. They never trigger a
// second filesystem evaluation and are not part of the persisted journal.
type updateObservation struct {
	cause                                 error
	validator, predicate, kind, operation string
	osCode                                uint64
	probes                                []error
}

func (e *updateObservation) Error() string { return e.cause.Error() }
func (e *updateObservation) Unwrap() error { return e.cause }

// Only this fixed vocabulary can cross the response boundary. In particular,
// arbitrary errors are never classified by parsing their text.
var updateDiagnosticPredicates = map[string]string{
	"root content": "root_content", "root mode": "root_mode", "root member identity": "root_identity",
	"root placement": "root_placement", "root progress": "root_progress",
	"root identity": "root_identity", "private root": "private_root", "root member": "root_inventory", "missing root member": "root_inventory",
	"activation phase": "activation_phase", "stage path": "stage_path", "old system phase": "system_placement", "new system phase": "system_placement",
	"system content": "system_content", "system mode": "system_mode", "system member identity": "system_member_identity",
	"system identity": "system_identity", "system placement": "system_placement",
	"metadata changed": "metadata_content", "metadata unavailable": "metadata_read", "unknown metadata": "metadata_content",
	"stage identity": "stage_identity", "home identity": "home_identity", "slot identity": "slot_identity", "new slot identity": "new_slot_identity",
	"stage inventory": "stage_inventory", "stage member": "stage_member", "root inventory mismatch": "root_inventory",
	"root inventories": "root_inventory", "unknown root placement": "root_placement",
	"unknown root member": "root_inventory", "unsafe root directory": "root_directory", "unsafe root member": "root_member",
	"occupied moved root slot": "root_slot", "unexpected local coordinator": "coordinator_presence", "unknown coordinator": "coordinator_presence",
	"unsafe stage": "stage_path", "unsafe slot": "slot_path", "unsafe system": "system_path",
	"missing stage proof": "stage_proof", "missing staged proof": "stage_proof",
	"system link": "system_path", "system directory": "system_inventory", "system member": "system_inventory",
	"missing system member": "system_inventory", "empty system directory": "system_inventory",
	"unknown home system": "system_placement", "unknown parked system": "system_placement", "unknown staged system": "system_placement",
	"unknown stage member": "stage_inventory", "unsafe stage member": "stage_member", "unsafe private root": "private_root",
	"recovery admission": "admission", "unsafe admission": "admission_path", "admission identity": "admission_identity", "admission contents": "admission_inventory",
	"occupied new system slot": "system_slot", "occupied home system": "system_slot", "occupied new slot": "root_slot", "old backup changed": "backup_content",
	"public content": "public_content", "public identity": "public_identity", "public mode": "public_mode", "public inventory": "public_inventory",
	"recovery rename": "recovery_rename", "recovery write": "recovery_write", "recovery journal": "recovery_journal",
	"metadata not restored": "metadata_content", "invalid recovery phase": "recovery_phase", "unattributed stage": "stage_presence",
}

func updateObserved(message string, cause error, operation string) error {
	if cause == nil {
		cause = errors.New(message)
		return &updateObservation{cause: cause, predicate: updateDiagnosticPredicates[message], kind: "mismatch"}
	}
	e := &updateObservation{cause: cause, predicate: updateDiagnosticPredicates[message], kind: "unknown"}
	var code syscall.Errno
	if errors.As(cause, &code) && code != 0 {
		e.kind, e.operation, e.osCode = "io_error", operation, uint64(code)
	}
	return e
}

func updateMismatch(message string) error { return updateObserved(message, nil, "") }

func updateValidator(err error, validator string) error {
	if err == nil {
		return nil
	}
	var observed *updateObservation
	if errors.As(err, &observed) {
		if observed.validator != "" {
			return err
		}
		copy := *observed
		copy.cause = err
		copy.validator = validator
		return &copy
	}
	return &updateObservation{cause: err, validator: validator, kind: "unknown"}
}

// Negative placement probes are normal. Only a failed aggregate consumes their
// observations. An absent alternative is not promoted to an IO failure.
func updatePlacementFailure(message string, probes ...error) error {
	e := updateMismatch(message).(*updateObservation)
	e.probes = probes
	return e
}

func updateProvenanceFailure(boundary string, j updateProofJournal, err error) Response {
	d := map[string]any{"boundary": "unknown", "validator": "unknown", "predicate": "unknown", "kind": "unknown", "phase": "unknown", "recoveryStep": j.RecoveryStep, "rootProgress": j.RootProgress, "metadataProgress": j.MetadataProgress}
	switch boundary {
	case "before_admission", "recover_public":
		d["boundary"] = boundary
	}
	switch j.Phase {
	case "intent", "staged", "old_system", "new_system", "root_switch", "roots", "metadata_switch", "metadata", "applied", "recovering", "rolled_back":
		d["phase"] = j.Phase
	}
	var e *updateObservation
	if errors.As(err, &e) {
		switch e.validator {
		case "recovery_start.root_placement", "recovery_start.metadata_placement", "recovery_start", "recovery_state", "recover_public":
			d["validator"] = e.validator
		}
		for _, predicate := range updateDiagnosticPredicates {
			if predicate == e.predicate {
				d["predicate"] = predicate
				break
			}
		}
		if d["predicate"] != "unknown" {
			switch e.kind {
			case "mismatch", "io_error":
				d["kind"] = e.kind
			}
		}
		if d["kind"] == "io_error" {
			switch e.operation {
			case "read", "stat", "native_id", "read_dir", "walk", "rename", "write", "path_check":
				d["operation"] = e.operation
			}
			if e.osCode > 0 {
				d["osCode"] = e.osCode
			}
		}
		if len(e.probes) > 0 {
			observations := []any{}
			for i, probe := range e.probes {
				if i >= 5 {
					break
				}
				if probe == nil {
					observations = append(observations, map[string]any{"probe": i, "kind": "match"})
					continue
				}
				child := updateProvenanceFailure(boundary, j, probe).Data["diagnostic"].(map[string]any)
				observation := map[string]any{"probe": i, "predicate": child["predicate"], "kind": child["kind"]}
				if v, ok := child["operation"]; ok {
					observation["operation"] = v
				}
				if v, ok := child["osCode"]; ok {
					observation["osCode"] = v
				}
				observations = append(observations, observation)
			}
			d["observations"] = observations
		}
	}
	r := updateFailure("provenance", "recovery", "inspect recorded update")
	r.Data["diagnostic"] = d
	return r
}
