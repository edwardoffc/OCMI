# Using OCMI

**OCMI 1.3.4, rules revision 2026-10-13**

Memory lives in the project; chats can be replaced after saving results and remaining work.
Three roles: the orchestrator plans and accepts, workers execute assignments, and the consultant is read-only.

## Starting and everyday work

For the main chat:

> You are this project's orchestrator. Read AGENTS.md and your role's inputs, restore the handoff and current queue. My goal is: … Prepare a bounded assignment and acceptance checks before handoff; review and integrate the submitted result.

Ask the orchestrator for separate prompts for the required worker slots and consultant.
Worker initialization:

> You are this project's worker. Read your role's rules. Without a separate START, only confirm readiness: do not choose a task, change files, or run checks.

For the consultant:

> You are a read-only consultant. Answer using relevant documents and code with references. Do not change files, start processes, or assign tasks. My question is: …

The orchestrator prepares [START](../memory/tasks/START-TEMPLATE.md).
For manual handoff, forward it to the worker, then give the orchestrator the task ID and HANDOFF path
containing the completed Result. A different working copy also requires changes and new files.
The orchestrator records Acceptance, integrates accepted work, and checks the combined result.
Passing tests alone are not acceptance.

## Small tasks and sufficient checks in OCMI 1.3.4

Ask for the [compact card](../memory/tasks/SMALL-TEMPLATE.md) when scope is bounded, reversible and low-risk.
The orchestrator selects the format; sensitive or uncertain tasks keep the full contract.
Existing proven checks cover criteria first; new tests fill gaps. Scope, protected inputs and mandatory gates remain.
Run relevant cheap checks before expensive CI. Preserve unexplained failures; repeated rework needs a cause/hypothesis and a new diagnostic step.
Within verified retained context, read changed records and relevant sources; a new chat still restores its required inputs.

## What changed in 1.2

Agents read short shared rules and their own role procedure. Full acceptance procedures remain with the orchestrator.
Continuation verifies project/role/task/revision/copy and current START.
The map identifies verification sources for each area; an old note or incomplete search does not confirm a fact.
A task lesson becomes a local rule after verification; startup does not load all memory.
The environment profile distinguishes configuration from a verified mechanism; manual continuation remains available.

Before assignment, the orchestrator checks actual formats and one end-to-end consumer path,
then necessary failure cases, exposing incompatibilities before integration.
The first review is complete and consolidated. After fixes, review changes and related risks;
before another substantive return, investigate the cause instead of automatically issuing another revision.
Rerun checks based on changed dependencies; the issued checker and safety checks remain mandatory.
Do not copy history into every message; update only changed memory facts.

## Task completion in 1.2.1

After integration, the orchestrator accounts for temporary resources and records cleanup in Acceptance.
R1/R2 usually reuse the same copy; a separate branch is based on task needs, not every edit.
The worker reports resources in Result. Pending cleanup records reason, owner, and next step;
it does not undo acceptance or require rerunning all tests. Retained resources have a purpose and event-based retention.
Full closure requires integration and cleanup: done; no automatic deletion or new authority is granted.

## New chats and project upgrades

Give a new orchestrator its role, AGENTS.md, and the orchestrator handoff path.
Give a new worker the latest assignment, START/revision, copy, and current Result with remaining work.
Give the consultant its role and question; its conclusions become decisions after passing to the orchestrator.
Unsaved chat history is not restored automatically.

For an installed project upgrade, ask its orchestrator to compare local rules with the new revision
and preserve current work. Do not rewrite old packages; needed changes require an explicit new revision.
Models, chat counts, automatic messaging, and external permissions are chosen separately.
Updating the base template does not change projects or create chats.

Assess value across several accepted and integrated tasks: returns, repeated checks, available time and cost.
Acceptance already records this; a new collector is unnecessary.
File size does not prove token savings. Total account usage is not one task's cost.
[Process](../memory/ORCHESTRATION.md) · [Memory rules](../memory/RULES.md) · [Protocol](../memory/TASK-PROTOCOL.md).
