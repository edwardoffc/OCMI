# OCMI layout

[English](layout.md) · [Русский](../locales/ru/docs/layout.md)

The system repository and projects using it can live in any separate directories.
No special folder name, shared drive, or central store is required.

~~~text
OCMI/
  README.md                 English entry point
  README.ru.md              Russian entry point
  templates/work/           canonical English work profile
  templates/personal/       canonical English lightweight profile
  templates/coordinator/    installation coordinator service profile
  BOOTSTRAP.md              temporary installer route
  COORDINATOR.md            permanent coordinator route
  VERSION.md                system version, separate from profile versions
  docs/                     English guides, reports, and development memory
  locales/ru/               Russian guides and self-contained templates
  localization/             translation manifest and validation data
  releases/                 immutable distributions and checksums
  scripts/                  validation and packaging tools

example-project/
  AGENTS.md                 agent entry point in the selected language
  docs/guides/              human documentation
  docs/memory/              one memory, role procedures, and tasks
  src/                      project code, when applicable
~~~

Adapt target paths during installation, preserving existing documentation and memory.
Each project has its own rules, queue, and tasks. Agents read role inputs and current task information;
neighboring projects are not automatically added to context.

The source template is needed only for installation or an explicit upgrade.
Updating OCMI does not automatically change installed projects.
This repository's docs/memory belongs to OCMI development and must not be copied instead of templates.

Do not add user data, client materials, secrets, or application-project chat logs to this repository.
Local reports/backups live in ignored directories. Before publication, review history and archives too.

After moving a project, verify the client's working directory.
Renaming a folder does not automatically update existing chat bindings.

## Optional user organization

[COORDINATOR](../COORDINATOR.md) offers organization after core handover; it does not apply it automatically.
Folder names and locations are user choices, and work projects may live elsewhere.

~~~text
Projects/
  00_OCMI/                         source checkout; no user installation records
  OCMI-local/coordinator/          private core setup state and filled STARTUP
  01_Personal_memory/              only if selected
    topics/<topic>/               independent lightweight memory when requested
  example-project/                 its own work memory
~~~

Guided installation prepares private service memory outside the source checkout/public Git scope.
Personal-topic storage remains optional. Keep an existing private coordinator path;
a service area in a selected personal store is also possible without moving existing records. It contains setup choices, nonsensitive installation metadata
and a setup handoff, not project plans or personal content. Registry pointers do not
authorize reading their targets. Existing setup paths are merged in place rather than
renamed just to match the example. A new coordinator chat in the system-bound project uses its filled STARTUP prompt and this private entry.
Core setup requires confirmed project binding and startup; reuse existing suitable folders/projects.
The work orchestrator remains responsible for its project's queue and acceptance.

[Usage](usage.md) · [Maintenance](maintaining.md) · [Localization](localization.md).
