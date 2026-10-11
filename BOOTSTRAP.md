# Start using OCMI

**English** · [Русский](locales/ru/BOOTSTRAP.md)

OCMI 1.3.2. This procedure belongs to the first, temporary installation chat.

## First request

Read this file, [VERSION](VERSION.md), and [README](README.md) only.
Do not load docs/memory, private stores, neighboring projects or client history.
Route both “study OCMI” and a first request to integrate OCMI into a project through core setup.
A question selects explanation; an integration request does not authorize unspecified core targets.
Preserve an explicit project-integration request as deferred scope, with only minimal target-instruction reading to distinguish an operational installation; defer all target changes.
If a configured coordinator already exists and is explicitly identified, verify and resume it.
An existing operational target uses its own memory for routine work and requested upgrades;
do not force coordinator setup for that target. The core gate applies to new installations.

Keep the first response brief; leave advanced setup options to the permanent coordinator.
Explain in one or two sentences that OCMI restores saved file context, not unsaved conversation.
Then give the required outcome and concrete user steps:
local system folder → project bound to that folder → separate coordinator chat → filled startup prompt.
Offer to complete this setup and state the exact next missing step.
Do not just say “install OCMI” or assume the user knows how to bind a folder.
An explanation-only user may decline; do not configure anything from a question alone.

## One installation outcome

Windows and macOS use this same procedure. Resolve native absolute paths in private setup records;
stable profile paths remain unchanged. [Platform support](docs/platforms.md).

1. Wait for the user's selection before any writes, downloads, clones or target scans.
   An explicit core installation request is sufficient; do not ask again for the same authorization.
   Resolve missing core targets before writes. Core prerequisites do not grant access to work projects.
2. After consent, a local system copy is required. Suggest `00_OCMI` or `OCMI`.
   Reuse a verified downloaded copy; do not clone again or force a rename.
   Read [system installation](docs/install-system.md) for source verification and exact handover steps.
3. Use .local/coordinator inside the selected system folder for private coordinator memory;
   exclude .local from Git and public distributions. Personal storage is optional.
   Verify ignore/export exclusions before saving user values; never force-add local files.
   Preserve existing coordinator paths/records. Save core state and any still-current deferred request.
4. Save a fully filled startup prompt in private coordinator memory.
   Deliver identical prompt text plus the exact system path, service path and intended client project.
5. Tell the user to create a local project OCMI attached to the system folder and a separate
   coordinator chat inside it. Reuse an existing correctly bound project; do not create a duplicate.
   Give an ordered checklist: create/reuse project, attach exact folder, open coordinator inside it,
   grant required service-directory access, send the supplied prompt.
   The user performs these client actions; do not create or message chats automatically.
6. Core setup is incomplete without the bound client project and initialized coordinator.
   A client's equivalent folder-bound workspace is acceptable; a loose chat is not a completed setup.
   If binding or file access is unavailable, give remaining steps and report setup pending.
7. Keep the first chat until the coordinator reads the saved inputs and confirms initialization,
   including its client project and system-folder association.
   Only then may the user delete/archive the first chat. Do not delete it automatically.
   Continuation must not depend on the first chat's history.

Do not integrate a work project before core setup is confirmed.
The work and lightweight INSTALL entry checks apply even when a component ZIP is supplied directly.
The coordinator then explains practical usage and offers personal storage, organization and integration.
A still-current explicit deferred request can continue without repeated permission for the same action.
Unselected options remain inactive. Existing installed projects keep their own memory and routine workflow.
Client settings, models, background work, publication and production need relevant authority.

## Starting from a URL

Repository: [edwardoffc/OCMI](https://github.com/edwardoffc/OCMI).
Read orientation from one identifiable revision; record commit/tag or date and verification limit.
A moving branch alone is not a pinned revision.
A URL provides source access, not local file access. If browsing/writing is unavailable,
provide exact manual download/save/binding steps; do not claim installation.
Do not execute remote scripts merely to explain or install the file-based system.

[Permanent coordinator](COORDINATOR.md) · [System installation](docs/install-system.md).
