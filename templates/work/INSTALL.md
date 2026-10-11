# Installing the OCMI work profile

**OCMI 1.2.1, rules revision 2026-10-11**

[Version and changes](VERSION.md). This is a neutral template, not an active project.
"This is a work project", referring to a specific directory, selects this profile;
it does not authorize creating chats, messaging, background tasks, or external actions.
Personal topics are not needed for installation.

## New or existing project

1. Read local instructions/overrides and inspect Git state, plan, memory, tests, and human documentation.
   Preserve current work; do not reset untracked files/WIP.
2. For a new project, install AGENTS.md, docs/memory, and docs/guides/agent-workflow.md;
   retain VERSION.md as the installed version description. Copying INSTALL.md is optional.
   Fill facts from sources, leave unknowns unknown, and do not present a template as a completed project.
3. For an existing project, merge rules rather than overwriting them with templates.
   Map old sources to current ones; retain one detailed PLAN and one STATE.
   Keep existing alternative paths and adapt links; do not duplicate human documentation.
4. Follow file responsibilities in [RULES](docs/memory/RULES.md).
   The orchestrator owns shared memory; the worker writes its Result and authorized implementation;
   the consultant is read-only. About 80 lines for BRIEF/handoff is a compactness guideline, not a reason to remove risks.
   Before shortening, preserve history and verify completeness/hashes/links; retain finding IDs/WIP/evidence.
5. Adapt the [process](docs/memory/ORCHESTRATION.md) and [task record](docs/memory/tasks/TEMPLATE.md)
   to actual commands, resources, and local permissions. Do not import models, slot counts,
   budgets, permissions, or chat IDs from the source. Handoff is manual by default.
6. Prepare the [guide](docs/guides/agent-workflow.md) and separate initialization prompts for workers/consultants.
   Initialization is not START; a record is not an assignment until ready.
   Submit into the Result section of tasks/<id>/HANDOFF.md, not a separate Result.md.
7. Check links, the single plan, instruction conflicts, and actual startup context per role:
   lines/bytes, duplication, stale statuses, and required input availability.
   A map/archive being in the package does not require reading all of it.
   Run existing documentation checks, not production/build for text; bytes are not tokens.
8. Select the first real task and prepare it according to the process: consumer/formats, full scope,
   external prerequisites, evidence matrix, protected tests BEFORE assignment, verified baseline/commands,
   one end-to-end path and material failures, reuse conditions, and a final integration check.
   An unmet mandatory prerequisite prevents ready status; do not invent tasks to fill slots.
9. Complete a small real task through acceptance and integration. Distinguish "template installed"
   from "workflow verified in use"; assess savings over several results.

Populate CODEMAP only with verified paths and a verification source per area.
Keep the environment profile in STATE or an existing runbook; configured/available/verified are distinct.
Do not create empty LESSONS: candidates stay in Result/Acceptance; accepted knowledge has one local home.
Exporter and technical protection scripts are not part of the template:
use verified project tooling or standard snapshot/Git with hashes.
A package includes the actual required tests/inputs, not only descriptions.
Do not claim automatic protection without verification.

## Upgrade OCMI 1 or 1.1 → 1.2

When installing current 1.2.1, also apply the resource-completion section below.

Contract/Result/Acceptance and exporter fields remain compatible.
Install the three role procedures with the new ORCHESTRATION router; check links before switching AGENTS.
Do not reduce mandatory inputs in active old packages; their fixed revision still applies.
Verify continuation identity and record statuses; check CODEMAP for affected areas.
Clarify the environment profile and check recovery using fictional data.
Scenarios/structural checkers do not prove model behavior in a new chat. Updating the base does not update projects.
On a separate owner request, merge new rules with local ones while preserving current assignments.
Prioritize: an end-to-end consumer/protocol scenario before assignment; accessible pinned links
instead of history copies; delta review after full review; dependency-based reuse; updates to changed facts only.
Current label: OCMI 1.2.1, rules revision 2026-10-11. Preserve historical labels.

Issued packages/checkers are immutable. Changing an active contract/tests/baseline/check requires
an agreed explicit revision preserving WIP, finding IDs, and evidence, not bypassing the checker.
Before another assignment, check links, record compatibility, environment, and minimum context.
Projects may have local refinements; do not inherit another project's permissions.
Record measurements in existing Acceptance through integration, without a new collector or promises of savings.

## Upgrade 1.2 → 1.2.1

Compare RULES, TASK-PROTOCOL, coordinator/worker procedures, and the task template with local rules.
Keep the short ORCHESTRATION router and three task headings; new fields stay inside these sections.
Record one primary work context and owners of temporary resources for new tasks.
At closure, the orchestrator checks remaining work, preservation/dependencies, and links, then records cleanup.
Do not rewrite old packages; missing cleanup is not completion.
Record old remaining work in current handoff/Acceptance, linking to the original.
Changes to an active contract require explicit revision.
Verify local permissions: new text does not authorize deletion. The template contains no automatic cleanup.
Check "no resources", "pending with owner/next action", "WIP or open finding",
and "main copy depends on temporary copy" scenarios; do not delete real data to test an instruction.
The lightweight profile remains 1.2; do not add branches/worktrees or work-profile acceptance to it.

Start: [AGENTS](AGENTS.md), [guide](docs/guides/agent-workflow.md), [process](docs/memory/ORCHESTRATION.md).
