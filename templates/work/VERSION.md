# OCMI 1.2.2

**OCMI 1.2.2, rules revision 2026-10-11**

Orchestration, Context Memory & Indexing.
Compatible security update to work profile 1.2.1. The lightweight profile is now 1.2.1.

## Changes from 1.2.1

Sanitize logs, shared memory, fixtures and archives before persistence/publication.
Necessary originals require explicit restricted storage and retention ownership.
Preserve task fields and protected packages; handle exposed credentials as incidents.

## Retained changes from 1.2

- One primary work context per task; R1/R2 reuse a suitable existing copy.
  Create a branch/worktree when the task requires it, not for every small change.
- Temporary resources have an owner, completion condition, and disposition in the existing task record.
- After integration, the orchestrator checks WIP, processes, preservation, dependencies, authority, and links.
  The worker reports resource state; cleanup alone does not require a full test rerun.
- Acceptance has a separate cleanup: done/pending status. Pending cleanup does not undo accepted implementation,
  but remaining work must stay visible with reason, owner, and next action.
- Preserve unique changes and evidence; do not automatically archive reproducible builds/caches.
  Shared caches are allowed; the main copy must not depend on a temporary copy being removed.
- A safe deletion refusal does not authorize force. A hash without accessible content is not a backup.
  Completion details live in the orchestrator procedure; shared startup reading stays short.

## Compatibility

Preserve the 1.2 core/routes, Contract/Result/Acceptance sections and exporter fields,
protected tests, fixed baseline, WIP, consolidated review, reuse, and integration.
Issued packages/checkers remain immutable. New rules apply to new assignments/explicit revisions.
Missing cleanup in an old record is not done; account for remaining resources separately
in current handoff/Acceptance, linking to the original without rewriting the protected package.
Project permissions are not expanded; there is no automatic deletion or background cleanup.
Installed projects do not update automatically; see [INSTALL](INSTALL.md).
Version, rules date, and task revision r1/r2 are different labels.
Use: OCMI 1.2.2, rules revision 2026-10-11.

Structural and archive checks do not prove model behavior or token savings.

Installer policy: OCMI system 1.3.2. New installations are performed by the initialized coordinator.
Runtime protocol remains 1.2.2; this source update does not migrate installed projects.
