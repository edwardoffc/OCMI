# Start using OCMI

**English** · [Русский](locales/ru/BOOTSTRAP.md)

OCMI 1.3.0. This is a setup procedure, not an active assignment or user memory.

## First conversation

Use a new chat attached to a downloaded OCMI directory, or supply the repository URL
to a client that can browse it. A minimal prompt is:

> Study OCMI and help me start using it. Explain it briefly, offer setup options,
> and let me choose what I need before changing anything.

The agent acts as an installation coordinator: it explains and helps with selected setup
actions. This role is different from an orchestrator that runs development in one project.

## Agent route: explain, offer, then apply selected actions

1. Read this file, [VERSION](VERSION.md), and [README](README.md) only.
   Do not load docs/memory, private stores, neighboring projects, client history or profiles
   for orientation. For a specific source-development request use the repository's
   development route instead. Merely asking what OCMI is does not select setup actions.
2. Briefly explain: state is saved in files; new chats restore saved state; work projects
   use an orchestrator, workers and a read-only consultant; small topics use lightweight
   memory. A CODEMAP guides targeted reading. Unsaved chat content cannot be restored.
   Written boundaries do not enforce filesystem isolation.
3. Offer independent options in one short menu: folder organization/renaming; persistent
   coordinator memory; personal storage; integration into a new or existing project;
   or explanation only. No option is mandatory; do not preselect the full setup.
4. Wait for the user's selection before any writes, downloads, clones, renames or scans
   of target areas. An explicit request already selecting an action is sufficient; do not
   ask again for the same authorization. Ask only for genuinely missing target or scope.
   A discussion, silence, repository attachment or URL does not authorize configuration.
5. Read only the inputs for the selected action. Preserve local instructions, documents,
   tasks and WIP; merge existing memory rather than creating a competing system.
   Explain the result, selected locations, checks, remaining steps and how to resume.
   Return to the unselected options only when relevant; never apply them automatically.

## Independent setup options

- **Folders:** suggest names such as `00_OCMI` and `01_Personal_memory` only if useful.
  They are examples, not requirements. Organization does not authorize moving anything.
  Before an authorized move/rename, verify exact paths, preserve content and inspect known
  affected bindings/links. Report client reconnection steps; do not claim they were applied.
- **Coordinator memory:** use [INSTALL](templates/coordinator/INSTALL.md) only when requested.
  Choose a private directory outside the source checkout and outside public Git scope.
  If personal storage already exists or is selected, a service area such as
  `Personal_memory/90-system/ocmi` is suitable. Otherwise an independent
  `OCMI-local/coordinator` directory works; personal storage is not a prerequisite.
- **Personal storage:** after the user selects it and its location, prepare a scoped
  entry point and navigation. Use [lightweight INSTALL](templates/personal/INSTALL.md)
  for each independent topic when requested; do not create sample topics, a personal
  profile or empty category trees. A personal request does not select the work profile.
- **Work project:** after selection, read only the supplied target and
  [work INSTALL](templates/work/INSTALL.md). A new project gets verified initial memory;
  an existing one gets a merge preserving human documentation, local rules, tasks and WIP.
  Ask for a missing target instead of scanning the disk. One project does not authorize others.

Choose one template language per installation. Russian profiles have the same stable
paths under locales/ru. Communication may use the user's preferred language.
Creating/messaging chats, model changes, global settings, background work, publication
and production operations require their own relevant authorization.

## Starting from a GitHub URL

Repository: [edwardoffc/OCMI](https://github.com/edwardoffc/OCMI).
Read README, BOOTSTRAP and VERSION from the same identifiable revision; follow only
the selected-language setup inputs. Record the commit/tag if available, otherwise the
access date and verification limit. A moving branch alone is not a pinned revision.

A URL provides source access, not local file access. If browsing is unavailable, request
the setup files or a downloaded copy. If writing/downloading is unavailable, give the
exact selected files and manual steps; do not claim installation. After an authorized
download/clone, verify origin and selected revision before using local setup inputs.
Do not execute remote scripts or install all profiles merely to explain the system.

## Resuming the coordinator

Without selected persistent memory, this is an explanatory chat; unsaved choices remain
in its conversation only. With memory, open its directory in a new chat and say:

> Continue as my OCMI installation coordinator. Read AGENTS.md and the current handoff.
> Restore selected options and remaining setup work; do not configure unselected options.

Project orchestrators and personal topics resume from their own memory. Coordinator
records contain setup metadata, not copied project plans, code, personal facts or secrets.

[Usage](docs/usage.md) · [Layout](docs/layout.md) · [Distributions](releases/README.md).
