# OCMI

**English** · [Русский](README.ru.md)

**Orchestration, Context Memory & Indexing** — a file-based workflow for AI agents.

OCMI helps you work on a project across multiple chats and resume it in new ones.
Roles, decisions, task assignments, and next steps live alongside the project in Markdown.
A code map points agents to the relevant parts of the codebase, while task records connect
requirements, results, and verification.

**Work template: [1.2.1](templates/work/VERSION.md)** ·
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

1. Download the [work template](releases/ocmi-work-1.2.1-en-2026-10-11.zip) and extract it
   outside the target project, or clone this repository.
2. Open the target project in an AI client with file access and give the agent this prompt:

   > This is a work project. Set up OCMI using INSTALL.md from the template I provide.
   > Preserve existing instructions, documentation, active tasks, and unfinished work.
   > Populate project memory with verified facts and check the links.
   > Explain the setup and communicate with me in English.

   Provide the actual path to the extracted `INSTALL.md` or `templates/work/INSTALL.md`.
3. Ask the orchestrator to prepare a plan and prompts for the roles you want to use.
   Choose the number of chats and models separately for your project.

Once installed, the project's workflow is self-contained. Routine work does not require
reading the OCMI source repository. The [user guide](docs/usage.md) covers new projects,
existing projects, upgrades, and resuming work.

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
