# Start using OCMI

**English** · [Русский](locales/ru/BOOTSTRAP.md)

OCMI 1.3.1. This procedure belongs to the first, temporary installation chat.

## Brief orientation

Read this file, [VERSION](VERSION.md), and [README](README.md) only.
Do not load docs/memory, private stores, neighboring projects or client history.
A request such as “study this repository” selects explanation, not configuration.

Explain in a few sentences: OCMI saves decisions, tasks and continuation in files.
New chats restore the saved state; a code map guides targeted reading.
Work projects can use an orchestrator, workers and a read-only consultant.
Unsaved conversation cannot be restored. Then offer to install OCMI.
Keep the first response brief; leave advanced setup options to the permanent coordinator.

## Two-stage installation

1. Wait for the user's selection before any writes, downloads, clones or target scans.
   An explicit installation request is sufficient; do not ask again for the same authorization.
   Confirm missing locations before writing. Opening a link or attaching a folder is not consent.
2. After consent, a local system copy is required. Suggest `00_OCMI` or `OCMI`;
   names are examples. An existing verified download can fulfill this requirement.
   Use [system installation](docs/install-system.md) to verify the source and prepare handover.
3. Agree on a private coordinator memory location outside the source checkout and public Git.
   This service memory stores setup state, not personal topics; personal storage is optional.
   Preserve an existing coordinator location rather than creating competing records.
4. Save a fully filled startup prompt in private coordinator memory.
   Give the same prompt to the user with the exact system and memory paths.
5. Ask the user to create a local project OCMI attached to the system folder and a separate
   coordinator chat inside it. If projects are unavailable, use a separate chat with file access.
   The user sends the prepared prompt there; do not create or message chats automatically.
6. Keep the first chat until the coordinator reads the saved inputs and confirms initialization.
   Only then may the user delete or archive the first chat. Do not delete it automatically.
   Continuation must not depend on the first chat's history.

Do not configure personal storage, rename folders or integrate projects during core installation.
The permanent coordinator offers these optional actions after initialization.
Client settings, models, background work, publication and production need separate authorization.

## Starting from a URL

Repository: [edwardoffc/OCMI](https://github.com/edwardoffc/OCMI).
Read the orientation files from one identifiable revision. Record the commit/tag, or the
access date and verification limit if unavailable. A moving branch alone is not a pinned revision.
A URL provides source access, not local file access. If browsing or writing is unavailable,
give manual download/save steps and the exact remaining actions; do not claim installation.
Do not execute remote scripts merely to explain or install the file-based system.

[Permanent coordinator](COORDINATOR.md) · [System installation](docs/install-system.md).
