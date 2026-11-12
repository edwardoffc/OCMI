# Orchestrator procedure

**OCMI 1.3.5, rules revision 2026-10-21**

Read INDEX, BRIEF, current STATE/handoff, and the relevant PLAN section.
The orchestrator owns shared memory, prepares protected assignments/checks, accepts results, and integrates.
Do not replace the worker by implementing its task. Specify submission format in Contract using TASK-PROTOCOL.
One task has one owner. Parallel work requires independent results, separate paths, baselines,
and resources: lock files, installations, ports, databases/artifacts are isolated or have one owner.
Test resources need an owner and cleanup; a separate copy does not isolate processes/secrets.
One task has one primary work context; create a new branch/worktree only when the task needs it.
Before creating one, check for a suitable available copy and its owner; do not take another owner's active resources.
R1/R2 continue in the same copy by default. Moving preserves WIP, evidence, and assignment identity;
changing the baseline/contract requires an explicit revision. A small edit does not require a new branch.
In Contract, record temporary task resources: path/branch/process, owner, completion event,
keep/archive/remove/transfer, and the basis for already-authorized cleanup. A date does not authorize deletion.

## Readiness before START

For a real task gap, read skills/catalog.json metadata and selected skill bodies only.
Use docs/guides/skills.md when an optional specialist contribution is needed.
List selected paths/revisions in existing START inputs; no extra role/chat or bulk skill loading.
Optional advice stays within current authority; one orchestrator retains checks and acceptance.

Choose full or compact using [format selection](../TASK-PROTOCOL.md). For compact work retain the
same readiness safeguards; record criterion coverage inline and only relevant details, not repeated rules.
Before assignment, the orchestrator verifies in [Contract](../tasks/TEMPLATE.md):

1. One observable integrable result, its consumer, criteria, and dependencies from PLAN.
2. Actual inputs/outputs of supported protocols and modes: versions, IDs, units,
   compatibility with old data, stop/restart, and material failure cases according to risk.
3. The full scope chain: callers, schema, config, CI, related files, and environment fit
   AllowedPaths and do not conflict with ProtectedPaths or other assignments.
   Name external access, quotas, and decisions in advance; mocks do not prove their availability.
4. The matrix "requirement → behavior → test/fixture/runner → sufficiency".
   The orchestrator selects and validates sufficient existing acceptance checks BEFORE assignment;
   it adds new tests only for uncovered requirements or risks. Freeze independent acceptance checks/inputs.
   Permit worker-owned development tests/fixtures in AllowedPaths; these do not replace independent acceptance.
   A baseline failure must be caused by the requirement, not a broken test/environment;
   validate risky nonstandard setup/concurrency/transaction/recovery premises on the actual engine/harness.
   Link the relevant known-correct control and targeted wrong behavior or justified oracle in this matrix;
   static advice or an unrelated passing control is insufficient. Unverified material premises prevent START.
   Reuse valid controls under their dependency/condition rules; preserved behavior may already pass.
5. Verified commands/cwd/environment/skips, Result format, and the existing checker.
   Each check's dependencies, evidence reuse conditions, and an overall integration check.
6. Exact code/test baseline, protected SHA256 (tests, fixtures, runner/config,
   Contract/Acceptance), unaccepted WIP, worker, copy, resources, and stop conditions.

For behavioral changes, first select and validate a check of one end-to-end path through real callers,
substituting external I/O when needed; then cover material negative scenarios. Use existing coverage when sufficient.
This orders coverage verification; it does not require the orchestrator to implement the task first.
A premise probe answers one named uncertainty; prefer reusable code over a parallel feature implementation.
Before freezing a new executable fixture/adapter, validate actual calls/formats and a relevant control within authority.
Syntax or a stub is insufficient; unavailable validation stays design-only with its missing dependency, not executable readiness.
Before ready, verify actual formats and sufficiency of all required checks; one successful path is insufficient.
Give a component a consumer and the next joint check. An unavailable mandatory prerequisite
prevents ready status; independent work may proceed as a separate task.
A small reversible text edit needs criteria and documentation checks;
research needs sources and verifiable conclusions. Artificial tests are unnecessary.

For long waits follow RULES waiting/continuation; an elapsed wait does not authorize retries or new START.

## Checks and review

Additional tests cover gaps; justify oracles/mutations by specific risk and sufficiency.
Order relevant checks from cheap to expensive. Before costly CI, require a valid preflight for the
changed scope; existing evidence may be reused only under its dependency/condition rules.
Verify affected runner/config consumers in the actual CI mode/tool versions and isolated fixtures;
record missing prerequisites rather than treating a different local mode as equivalent.
For behavioral changes include a real consumer path in isolated conditions, substituting external I/O
where required. Do not require a product end-to-end run for a documentation-only task.
Rerun when runtime, tests, fixtures, schema, config, toolchain, callers/environment change,
when evidence is unreliable, or for integration. Test-only work usually needs the changed test
and affected checks; broad impact needs corresponding regression coverage.
The orchestrator does not duplicate the entire worker run without a reason.
Reuse is allowed after confirming unchanged relevant dependencies/conditions: preserve version/hashes,
command, exit code, log, and check scope. Unknown conditions require rerunning the affected check.
Result-only edits do not invalidate runtime evidence: extract counters from logs, not another suite run.
The issued checker is mandatory even with a stricter fingerprint; exceptions require a new explicit revision.

A successful retry alone does not resolve the original failure. Preserve the failed log and conditions;
classify implementation, check/contract, environment or flakiness and record the causal evidence.
For unexplained setup/barrier failures before behavioral assertions, require a discriminating diagnostic
before speculative product changes or repeats; keep observation/hypothesis/next check in existing Result/Acceptance.
A confirmed transient infrastructure failure can close without a code change when the evidence and
project policy support that conclusion; an unexplained green rerun is not proof of a fix.
Unexplained mandatory safety failures remain blockers according to the project gates and risk.

The first review is complete: matrix, diff, callers, protected hashes, and evidence sufficiency.
Return one consolidated list of ID/reproduction/impact/required outcome/check, classified as:

- Defect in agreed behavior/evidence: fix within the current result.
- Assignment/test/environment defect: the orchestrator fixes the cause; a changed contract/checker requires revision.
- New independent scope, including cosmetics: a separate PLAN item, not a hidden acceptance criterion.
- New critical security, financial, or data-integrity risk: a blocker even late; report immediately.

After rework, check changes, previous findings, and affected connections; expand review for new risk.
At existing Result/review checkpoints, progress means a verified agreed criterion or evidence resolving a named uncertainty;
activity, file/test counts and report edits alone are not progress. Research/documentation use their agreed criteria.
From the second unexpected failed delivery/review/CI cycle for one integrated result or release,
or two consecutive preparation/review checkpoints without progress, reassess the assignment itself
BEFORE another substantive return or costly run: preserve the failure log and WIP; briefly record
the cause category, hypothesis, changed approach, next diagnostic and expected new evidence,
the sufficient check set and reason for any broad rerun in existing Acceptance or the release's orchestration handoff.
Check criteria/oracle, fixture/consumer fit, permissions, duplicated preparation and the next integrable result.
Include prerequisite repairs; new task IDs, revisions and green intermediate runs do not reset this history.
Allow one bounded changed attempt justified by that evidence; another failure or no-progress checkpoint stops equivalent returns.
Then use ocmi-review once for a bounded process review; use authorized independent advice if available,
otherwise label self-review honestly. Choose evidence-backed continuation/revision, a checkable slice or an unresolved dependency.
Do not repeat that review without new evidence; preserve WIP and stop affected work if no justified route remains.
Escalate only missing authority/input/owner decisions; routine safe continuation needs no renewed permission.
Do not automatically repeat a full run or add revisions; broaden only for dependencies/integration/risk. No separate tracker is needed.
This does not justify accepting a defect, changing model/budget, bypassing STOP, or setting a Goal to blocked/paused.
For reporting inaccuracies supported by verified facts, the worker corrects Result or the orchestrator
annotates Acceptance while preserving the author's Result; formatting does not need another implementation cycle.
After acceptance, integrate the accepted diff and perform the joint check; return implementation conflicts to the worker.
Dependent work follows integration or an explicitly recorded prerequisite, not DONE alone.

## Resource completion

After integration, the orchestrator checks completion once; the worker reports resource state in Result.
Repeat when the basis changes or the recorded condition occurs, not after every message.
Make a short pass over task resources without scanning neighboring projects:

1. Confirm accepted/integrated version and remaining WIP, including tracked, untracked, and ignored.
   Preserve unique changes, reports, necessary fixtures, and evidence for open findings.
2. Check processes, owners, dependent tasks, and consumers of temporary paths.
   The main copy must not depend on the closing copy; a shared cache does not belong to one task.
3. For keep/transfer, record reason, owner, and retention/transfer condition;
   for archive, verify saved content and the recovery path. A SHA alone is not a backup.
4. Within existing authority, stop only your own unnecessary processes, close unneeded copies,
   and delete integrated temporary branches after verifying preservation. Before deletion, verify the exact
   absolute path/name, owner, and authorized cleanup boundaries. Close managed worktrees with their supported tool.
   Preserve main, pinned, shared, or active copies. A safe deletion refusal does not authorize force.
   After squash/cherry-pick, verify actual integration and availability of source history/artifacts;
   --merged or a SHA alone is insufficient. Unknown files/dependencies or insufficient authority mean pending.
5. At acceptance/integration, update only changed PLAN/STATE/INDEX/CODEMAP/handoff facts;
   reconcile affected IDs/status/next action across current entries; explicitly supersede conflicting instructions.
   at closure, check affected links. Current links point to the main source; historical links point to
   an accessible pinned version with a recovery method. Do not rewrite issued packages:
   record a missing route in current Acceptance. Mark old results historical; retain open findings and next steps.
6. Record cleanup according to the [protocol](../TASK-PROTOCOL.md). Keep pending visible in current handoff/PLAN
   via Acceptance, without a second queue. Do not declare full closure until remaining resources are accounted for.

A cleanup record does not require another implementation cycle or full test run.
If resolving a dependency changes the environment/config, rerun the affected check; expand according to risk.
Contract records user/project permissions; it does not grant them.
Already-authorized bounded cleanup needs no repeated approval; actions outside scope remain pending
with the required owner decision.
These rules do not authorize deploy, production, external/financial actions, or deleting remote branches.

## Outcomes and costs

In existing Acceptance through integration, record the useful result, accepted/integrated versions,
returns/reasons, repeated checks/reasons, and available time/cost with source and scope.
Unknown values remain unknown; total account usage is not the cost of one parallel task.
Budgets/stop conditions come only from current authority; do not inherit old percentages/limits.
An unavailable meter does not prove remaining budget; reaching a limit does not mean successful acceptance.
Across several completed tasks, compare outcomes and costs of the whole chain through integration.
Do not create a separate collector, validator, or Goal for measurement or rerun tests for statistics.

## Code map and verified lessons

After an accepted change, verify affected paths/symbols and the verification source in CODEMAP.
A rename, interface, schema/config, caller, or check change makes related entries require verification;
do not recheck the entire unaffected map. Record risks/gaps in the existing task record.
Lesson candidate: cause → reproducible evidence → scope/condition → action/limitations.
Save an accepted lesson in one local location: the existing guide or optional docs/memory/LESSONS.md.
Format: ID; scope; condition; action; evidence; limitations; status active/superseded/rejected and replacement link.
Create LESSONS only for an accepted useful lesson; add a targeted INDEX link rather than reading everything at startup.
Repetition/confidence does not replace verification; do not make a rule global without an appropriate scope.
The candidate and decision remain in Result/Acceptance; preserve provenance/history when replacing a rule.
