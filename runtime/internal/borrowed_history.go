package internal

import "regexp"

// Correlated projection only: no mutation or nested query on an open Rows.
// Both manual release and native automatic release require a stopped operation.
const borrowedHistoryProjection = `(
 resource_ledger.provider='local' AND resource_ledger.kind IN ('file','directory')
 AND resource_ledger.mode='use' AND resource_ledger.ownership='external'
 AND resource_ledger.observed_state='active' AND resource_ledger.imported=0
 AND resource_ledger.held=0 AND resource_ledger.hold_reason='' AND resource_ledger.hold_until=''
 AND resource_ledger.applying=0
 AND resource_ledger.native_identity<>'' AND resource_ledger.actual_disposition=''
 AND NOT EXISTS(SELECT 1 FROM resource_use u WHERE u.resource_id=resource_ledger.id AND u.active<>0)
 AND NOT EXISTS(SELECT 1 FROM operation o JOIN operation_revision m ON m.operation_id=o.id
 WHERE m.task_id=resource_ledger.task_id AND m.task_revision=resource_ledger.task_revision
 AND o.state IN ('prepared','running','unknown'))
 AND EXISTS(SELECT 1 FROM task t JOIN resource_revision c ON c.task_id=t.id AND c.task_revision=t.task_revision
 JOIN resource rc ON rc.rowid=c.resource_id
 WHERE t.id=resource_ledger.task_id AND t.task_revision=resource_ledger.task_revision
 AND t.state='integrated' AND rc.closed=1
 AND EXISTS(SELECT 1 FROM integration_revision i WHERE i.task_id=t.id AND i.task_revision=t.task_revision AND i.outcome_id=t.outcome_id))
 AND EXISTS(SELECT 1 FROM resource_use u
 JOIN operation_revision m ON m.task_id=u.task_id AND m.task_revision=u.task_revision
 JOIN operation o ON o.id=m.operation_id AND o.task_id=m.task_id
 JOIN task_revision tr ON tr.task_id=m.task_id AND tr.task_revision=m.task_revision AND tr.outcome_id=m.outcome_id
 JOIN operation_owner ow ON ow.operation_id=o.id
 JOIN resource_ledger p ON p.id='ocmi-process-'||o.id AND p.operation_id=o.id
 JOIN project pr ON pr.id=p.project_id AND pr.copy_id=p.copy_id AND pr.device_id=p.device_id
 WHERE u.resource_id=resource_ledger.id AND u.active=0
 AND u.task_id=resource_ledger.task_id AND u.task_revision=resource_ledger.task_revision
 AND o.state IN ('succeeded','failed','incomplete') AND ow.phase='registered'
 AND ow.child_pid>0 AND p.provider='supervised_process' AND p.observed_state='stopped' AND p.imported=0
 AND ow.copy_id=pr.copy_id AND ow.device_id=pr.device_id
 AND resource_ledger.project_id=pr.id AND resource_ledger.copy_id=pr.copy_id AND resource_ledger.device_id=pr.device_id
 AND p.task_id=u.task_id AND p.task_revision=u.task_revision
 AND (resource_ledger.operation_id='' OR resource_ledger.operation_id=o.id)
 AND EXISTS(SELECT 1 FROM resource_event e WHERE e.resource_id=p.id AND e.event='process_terminated'
 AND e.task_id=u.task_id AND e.task_revision=u.task_revision
 AND e.observed_state='stopped' AND e.proof_class='runner_termination')
 AND (EXISTS(SELECT 1 FROM resource_event e WHERE e.resource_id=resource_ledger.id
 AND e.event='release' AND e.task_id=u.task_id AND e.task_revision=u.task_revision)
 OR (resource_ledger.operation_id=o.id AND EXISTS(SELECT 1 FROM resource_event e
 WHERE e.resource_id=resource_ledger.id AND e.event='automatic_use'
 AND e.task_id=u.task_id AND e.task_revision=u.task_revision AND e.proof_class='external_native'))))
)`

var borrowedNativeProof = regexp.MustCompile(`^[0-9a-f]{1,16}-[0-9a-f]{1,16}$`)

func historicalBorrowed(qualified bool, native string) bool {
	return qualified && borrowedNativeProof.MatchString(native)
}

// A hold prevents history qualification; it is not unfinished retention by itself.
func resourceObservationPending(provider, state, observed string, applying int, historical bool) bool {
	if historical {
		return false
	}
	return (provider != "local" && provider != "supervised_process" && provider != "internal") || applying != 0 || state == "intent" || state == "unknown" || state == "remove_pending" || state == "transfer_pending" || state == "needs_rebind" ||
		(provider == "local" && state != "removed" && state != "historical_stopped" && (observed == "unknown" || observed == "absent"))
}

// A present managed object is observable, while its active disposition is still
// unfinished task cleanup. Do not export that summary obligation as inspect.pending.
func resourceDispositionPending(provider, ownership, state, observed string, applying int, historical bool) bool {
	return resourceObservationPending(provider, state, observed, applying, historical) ||
		(ownership == "project" && state == "active")
}
