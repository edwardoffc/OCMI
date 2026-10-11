# OCMI

**English** · [Русский](README.ru.md)

**Orchestration, Context Memory & Indexing** — a file-based workflow for AI agents.

OCMI helps you work on a project across multiple chats and resume it in new ones.
Roles, decisions, task assignments, and next steps live alongside the project in Markdown.
A code map points agents to the relevant parts of the codebase, while task records connect
requirements, results, and verification.

**OCMI system: [1.3.0](VERSION.md)** · [Start here](BOOTSTRAP.md) ·
**Work template: [1.2.2](templates/work/VERSION.md)** ·
[Installation](templates/work/INSTALL.md) · [User guide](docs/usage.md) ·
[Changelog](CHANGELOG.md) · [MIT](LICENSE)

English is the canonical language for guides, templates, and development instructions.
The [Russian edition](README.ru.md) includes complete guides and installable templates.
See the [localization policy](docs/localization.md) for supported paths and maintenance.

## How it works

| Role | Responsibility |
| --- | --- |
| Orchestrator | Maintains the plan, prepares assignments and acceptance checks, reviews results, and integrates changes |
| Worker | Completes an assigned task within its agreed scope, runs checks, and records the result |
| Consultant | Reads the project and answers questions without changing files |

The cycle: **plan → assign → implement → verify → accept → integrate → update memory**.
After integration, the orchestrator accounts for unfinished work and temporary task resources.
A small change does not require a separate branch or a new chat just to follow the process.

## Quick start

Open a downloaded copy of this repository in a new AI chat, or give a browsing-capable
client [the repository URL](https://github.com/edwardoffc/OCMI), and say:

> Study OCMI and help me start using it. Explain it briefly, offer setup options,
> and let me choose what I need before changing anything.

The [installation coordinator](BOOTSTRAP.md) explains the system and offers independent
options: folder organization, persistent coordinator memory, personal storage, and
integration into a new or existing work project. Explanation only is a valid choice.
Opening the repository or providing a URL does not install anything or rename folders.
Persistent memory is optional and belongs outside the source repository/public Git scope.

A URL alone does not provide local file access. Clients without browsing or writing need
the supplied setup files or manual saving; the coordinator explains the remaining steps.

### Direct project installation

If you already know the target, use [work INSTALL](templates/work/INSTALL.md) or its
[complete ZIP](releases/ocmi-work-1.2.2-en-2026-10-11.zip), and ask:

> This is a work project. Set up OCMI using INSTALL.md from the template I provide.
> Preserve existing instructions, documentation, active tasks, and unfinished work.
> Populate project memory with verified facts and check the links.

Provide the exact template and target paths. Then ask the project orchestrator for the
plan and role prompts you need. Select chats/models separately. Installed projects are
self-contained; routine work reads their own memory. See the [user guide](docs/usage.md).

## What lives in your project

| File | Purpose |
| --- | --- |
| `AGENTS.md` | Entry point for the agent and a route to its role-specific instructions |
| `docs/memory/INDEX.md` | Navigation through project memory |
| `BRIEF.md`, `STATE.md`, and `PLAN.md` in the memory directory | Goals, verified current state, and one shared work queue |
| `docs/memory/CODEMAP.md` | Verified code entry points and their links to tests |
| A task record and its handoff | Agreed scope, results, acceptance, and the exact next step |

To resume in a new chat, specify the project directory, the agent's role, and the current
task handoff. The agent restores the saved state and checks that the assignment is still active.
For example:

> Continue as the orchestrator. Read AGENTS.md, the inputs for your role, and the handoff
> I specify. Verify the current state and continue from the recorded next step.

A worker also needs its current assignment, START revision, and working copy.
Documentation for people remains separate; project memory links to it instead of duplicating it.

## Capabilities and limits

OCMI provides instructions and templates, not a server or an autonomous agent manager.
Setup requires no database, plugins, or separate service subscription.
Your client must be able to read files and receive instructions; automatic instruction discovery
depends on the client.

Indexing here means an agent-maintained code map, not an automatic repository-wide search engine.
Unsaved chat history cannot be restored. Written rules do not enforce technical access controls.
Token savings and context recovery quality have not yet been measured. Completed structural checks
are separate from the [planned behavioral evaluations](docs/evals/context-recovery.md).

## Development

Workflow sources are in `templates/work`, guides in `docs`, frozen releases in `releases`,
and validation scripts in `scripts`.
[Repository layout](docs/layout.md) · [Contributing](CONTRIBUTING.md) ·
[Checks](scripts/README.md) · [Optional lightweight profile](templates/personal/INSTALL.md).

[Security policy](SECURITY.md). Sanitize evidence before saving or publishing it.
