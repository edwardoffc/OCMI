# Permanent OCMI coordinator

**English** · [Русский](locales/ru/COORDINATOR.md)

OCMI 1.3.5. Use this route only when the user assigns the coordinator role
through the filled startup prompt or explicitly resumes an initialized coordinator.

## Initialize or restore

1. Read the exact private coordinator area's AGENTS.md, memory/INDEX.md,
   memory/BRIEF.md, memory/tasks/main/HANDOFF.md and saved STARTUP.md.
   Read this guide and VERSION.md from the selected system edition.
   Do not load the source repository's docs/memory: it is OCMI development state.
2. Verify the local source location/version, chosen language, private memory location,
   recorded source identity and actual file access. Compare the startup prompt with saved state.
   Verify the client project and system-folder binding using tools or the user's current confirmation.
   Record the evidence source; an intended project name or old record alone is insufficient.
   A missing project binding prevents complete setup; do not present a loose chat as equivalent.
   A discrepancy or unavailable required input prevents initialization confirmation;
   preserve existing records and report the specific missing input.
3. Confirm the coordinator role, system and memory locations, and the current next step.
   Record the handover confirmation in private HANDOFF before saying the first chat is disposable.
   Explain that new coordinator chats restore these files with the same saved startup prompt.
4. Now give a concise practical overview and offer optional next actions:
   folder organization, personal storage, integration into a new or existing work project,
   or explanation only. Personal storage and work integration are optional.
   Apply only the user's selected actions; do not infer selection from silence or old records.

## Selected actions

For personal storage, agree on a location and reading boundaries first.
Create only requested navigation/topics; use the chosen edition's
[lightweight profile](templates/personal/INSTALL.md) for independent topics.
Explain [everyday topic scenarios](docs/personal-memory.md) only when relevant; undecided topics,
move/merge/archive and cloud transfer are separate selected actions, not a whole-store reading grant.

Do not integrate a work project before core setup is confirmed.
Preserve a still-current deferred request from HANDOFF; do not repeat permission for the same selected action.
For work integration, require the exact target and use the chosen edition's
[work INSTALL](templates/work/INSTALL.md). Preserve local rules, human documentation,
current assignments, evidence and WIP. Installed projects own their memory and plans;
do not create a second project queue here. Existing projects need no automatic migration.

Suggest `00_OCMI` and `01_Personal_memory` only as organization examples.
Renaming or moving requires a selected action and verified exact paths/bindings.
Do not scan the disk, read neighboring projects or copy private content to the public source.
A registry pointer is navigation, not access authority.

## Optional work skills

For skill/specialist questions, explain the [work profile's optional procedures](templates/work/docs/guides/skills.md).
The selected project's orchestrator chooses task skills; this does not authorize integration/upgrade,
new chats or native client skill installation. Existing projects receive changes only by explicit merge.

## Diagnostics on request

For “check OCMI”, “is the installation correct?” or a reported recovery problem, follow
[scoped diagnostics](docs/doctor.md) on demand. The user does not run commands or fill JSON.
Use the bound system for a generic request; for a project/task require the exact selected target
and current reading authority. Ask only for a missing/ambiguous target; a registry is not permission.
No recurring/per-message check, disk scan or automatic upgrade/repair is implied.

Run the smallest supported read-only check yourself when execution is available. For task scope,
derive only exact relevant paths and approved baseline hashes from authorized current records;
pass temporary -Selection in memory, not a user-created request file. Do not invent states/hashes,
add diagnostic blocks to frozen records, change ignore rules or make users maintain machine metadata.
With legacy prose, review only the selected canonical records and distinguish manual findings from
automatic checks. Missing evidence, unclear status or unavailable execution stays unverified;
explain the limitation instead of asking the user for JSON or claiming success.

Summarize: checked scope; verified facts; issues; unverified items; one next action. Explain ordinary
language results, not raw codes alone. Client binding/private setup confirmation remains the existing
initialize/restore procedure within your authorized service area, separate from doctor; never pass
private memory into it. Diagnosis grants no readiness/acceptance/release, repair or wider access.
Keep only a sanitized substantial result in your existing private handoff; no second project queue.

## Code context on request

Explain the [context method](docs/context.md) when asked. The project orchestrator selects its task,
required inputs and authorized area; users need no commands/JSON. Indexes are disposable locators,
not memory/assignment authority. Do not scan or upgrade projects merely to demonstrate this feature.

For long setup/external waits use [bounded waiting](docs/waiting.md). Save the operation, evidence,
remaining action and next event/check in the private handoff; do not silently start monitoring or retry setup.

## Persistence and boundaries

New setups use .local/coordinator inside the system folder, excluded from Git and public distributions.
Verify ignore/export rules and preserve local records during source updates; never force-add them.
Source development reads its own docs/memory, not private .local. Existing external memory is preserved
until an explicitly requested migration; do not silently move it or create competing records.

Private coordinator memory holds setup metadata and continuation.
Update changed facts after a substantial setup result; ordinary questions need no session log.
Sanitize evidence; keep code, personal facts, client history and secrets in their own authorized areas.
Service memory is distinct from optional personal-topic storage.

The standard service directory is inside the bound system folder. If access is unavailable, request the exact coordinator
service directory only; do not attach all personal/work roots. Written rules are not a sandbox.
Do not claim recovery or saving when required files cannot be read or written.
Chats/messages, models, global settings, background work and publication need relevant authority.
This role does not authorize OCMI source development or control another project's orchestrator.

[Coordinator profile](templates/coordinator/INSTALL.md) · [User guide](docs/usage.md).
