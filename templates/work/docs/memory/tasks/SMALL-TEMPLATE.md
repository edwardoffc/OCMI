# TASK-ID: one bounded result

**OCMI 1.3.4, rules revision 2026-10-13**

Format: compact
Risk: low
Status: draft

Template, not a START. Use only after the orchestrator verifies low risk under
[format selection](../TASK-PROTOCOL.md). One HANDOFF; retain protected exporter fields if present.

## Contract

- Task / revision / owner / working copy: exact identity, current START and PLAN item.
- Result / criterion / consumer: one observable result; why bounded, reversible and low risk.
- Scope / AllowedPaths / baseline / WIP: include relevant callers; fixed commit/snapshot and preserved work.
- ProtectedPaths / hashes: checks and fixed inputs, or verified none; issued checker remains mandatory.
- Checks / evidence / sufficiency: each criterion → existing check or necessary gap test; cwd/commands,
  environment and preflight; valid evidence reuse, final integration check and material limits.
- Inputs / permissions / resources: exact accessible inputs, authority and dependencies; resources none
  or owner/completion/disposition. Unknown risk or a prerequisite gap requires the full format before START.

## Result

Worker: status/date, diff, check commands/exit codes/logs or verified reuse, risks and remaining WIP/processes.
Preserve failed-run evidence; a successful retry alone is not a fix. Do not edit Contract or Acceptance.

## Acceptance

Orchestrator: consolidated decision/findings, accepted and integrated versions/evidence;
repeat-failure cause/hypothesis/next diagnostic if relevant; cleanup: done/pending with reason/owner/next action.
Record observed time/cost or unknown. Project gates and separate release authority still apply.
