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

[Usage](usage.md) · [Maintenance](maintaining.md) · [Localization](localization.md).
