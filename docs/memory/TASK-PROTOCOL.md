# OCMI 1.3.3 — task and handoff format

**OCMI 1.3.3, rules revision 2026-10-14**

Compatible update to the OCMI 1 process; the task schema and existing exporter fields are preserved.
[ORCHESTRATION](ORCHESTRATION.md) routes readiness/checks/review; [RULES](RULES.md) defines memory.
Project commands, models, and permissions are local; an installed copy is self-contained.

## One record per task

Path: docs/memory/tasks/<id>/HANDOFF.md. Exact headings:

- `## Contract` — requirements/baseline/revision/scope/checks/assignment; edited by the orchestrator.
- `## Result` — outcome, evidence, and continuation; edited by the assigned worker.
- `## Acceptance` — review, decision, and integration; edited by the orchestrator.

Exporter JSON before Contract is protected; do not rename its fields.
Keep a large contract in the package and link to it. Do not create a separate Result.md.
A new task record contains relevant facts, the matrix, and differences from the previous revision;
do not copy permanent rules or past history. Links must lead to accessible pinned inputs;
"same as before" is insufficient for a new chat.

## Task format selection

The orchestrator chooses full or compact before START. Compact work must be bounded, reversible and low risk,
with one observable result, known dependencies and sufficient checks. Line count does not establish risk.
Use [SMALL-TEMPLATE](tasks/SMALL-TEMPLATE.md); keep the same Contract/Result/Acceptance and one HANDOFF.
Full format is required for financial behavior, authorization/security, persistent data or migration,
recovery guarantees, external contracts, release/deployment, shared CI/toolchain changes or broad impact.
Unknown risk or an unresolved mandatory prerequisite requires full format and prevents ready status
until resolved. A documentation correction about a sensitive area can be compact only if behavior,
permissions, operational commands and release conditions do not change.

Compact Contract still fixes task/revision/owner/copy, result/criterion/consumer, scope/AllowedPaths,
baseline/WIP, protected checks/hashes, criterion coverage, commands/environment/evidence reuse,
accessible inputs, permissions and resources/cleanup. Inline criterion-to-check mapping is enough;
an extra matrix, packet or separate report is unnecessary unless needed to preserve fixed inputs.
Existing project gates and the issued checker remain mandatory. Compact does not mean automatic
acceptance, skipped safety checks or permission to modify protected inputs. Scope/risk changes need
an explicit full-contract revision with WIP/evidence preserved; issued packages are not shortened retroactively.

## START, package, and continuation

Initialization assigns a role/slot: before START, confirm readiness only, with no implementation, checks, or Goal.
A request for prompts does not authorize creating chats, sending messages, or changing models.
START specifies ID/revision, exact working copy, current HANDOFF, baseline, inputs, and commands.
Continuation verifies project/role/task/revision/copy and the current ready START against
the orchestrator's assignment; contradictory input does not assign work.
Running means waiting for the same process, not starting a duplicate.
The package is an immutable original; the working record is the current submission, with only its Result body editable.
Changing the contract/tests/checker/baseline requires a new explicit revision preserving WIP and evidence.
An old START does not assign new work. Do not erase the current Result when starting a new chat.

## Submission and acceptance

Result includes status running/review/blocked and date, baseline/full diff with added/deleted files,
criterion coverage, commands/exit codes/pass-fail-skip, environment/dependencies/logs, protected hashes,
reuse/reasons for reruns, risks/unverified items, remaining work/processes, code-map proposals,
and a lesson candidate with evidence/scope/limitations.
Model/effort, time, and cost must be observed values with source/scope, or unknown.
Keep full sanitized logs at authorized accessible paths under RULES; a brief chat summary does not replace evidence.

DONE means ready for review. The orchestrator checks protected sections before editing Acceptance;
recording Acceptance completes that review revision without granting the worker permission to edit it.
Record accepted/rework/blocked, findings, accepted version, and integrated version separately.
Accounting covers the integrated result/release and dependent repairs across task IDs; apply the orchestrator's
repeated-failure rule to that whole chain. Release requires its own authorization. Report format does not prove behavior.
The orchestrator accepts lessons after verification; only then may they become local rules or LESSONS.
Retain source and status; do not automatically promote local experience to global rules.
In a shared directory, provide the ID and HANDOFF path; in another copy, also provide the full diff/new files and copy path.
Synchronization between copies is not assumed. The consultant passes proposals through the owner.

## Closure and cleanup

Acceptance, integration, and resource completion are separate states. After integration,
the orchestrator performs the short completion check in its procedure and records in Acceptance:

- `cleanup: done` — resources removed within authorized boundaries or recorded as keep/archive/transfer
  with reason, owner, retention condition, and verified access to necessary data.
- `cleanup: pending` — specific remainder, reason, owner, next action, and condition for rechecking.

If no temporary resources were created: `cleanup: done — no temporary resources were created`.
Unknown remaining work cannot be marked done. Pending does not undo accepted implementation/integration
or start another testing cycle; assess risks to the result separately.
Full task closure requires integration and done. Accepted tasks with pending cleanup remain visible
in the current handoff/PLAN through a link to Acceptance. No separate cleanup log is needed.
The worker reports resources/WIP/processes in Result; the orchestrator owns completion within its permissions.
Cleanup that changes the environment or config requires the affected check; formatting alone does not.

## Compatibility

New rules apply to new assignments/explicit revisions. The issued checker remains mandatory:
simpler reports or proven reuse do not permit bypassing existing requirements.
Preserve old packages and version labels; updating the template does not reinstall projects.
Missing cleanup in an old record means unknown status, not done; do not append to the protected package.
For remaining resources, record the orchestrator's current decision in current handoff/Acceptance,
linking to the old result; contract changes require an explicit revision.
