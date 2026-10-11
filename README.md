# OCMI

**English** · [Русский](README.ru.md)

**Orchestration, Context Memory & Indexing** — a file-based workflow for AI agents.

OCMI helps you work on a project across multiple chats and resume it in new ones.
Roles, decisions, task assignments, and next steps live alongside the project in Markdown.
A code map points agents to the relevant parts of the codebase, while task records connect
requirements, results, and verification.

**OCMI system: [1.3.1](VERSION.md)** · [Start here](BOOTSTRAP.md) ·
[Installation](docs/install-system.md) · [User guide](docs/usage.md) ·
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

Open a new chat and give it [the repository URL](https://github.com/edwardoffc/OCMI)
or a downloaded copy. Ask, for example:

> Study OCMI and briefly explain what it is.

1. The first chat gives a short introduction and offers installation.
2. If you choose to install, it prepares a verified local system copy in `00_OCMI`
   or `OCMI`, private coordinator service memory, and a filled startup prompt.
3. Create a local project OCMI attached to the system folder, then a separate
   “OCMI Coordinator” chat inside it. Send the prepared prompt.
   Without projects, use a separate chat with access to the required files.
4. The coordinator verifies saved state and confirms startup. It then offers optional
   personal storage, folder organization and integration into your work projects.
5. After confirmation, you may delete/archive the first chat.
   New coordinator chats resume from the saved startup prompt and current handoff.

Providing a URL alone installs nothing and grants no local file access.
If tools cannot download or write, the first chat provides manual steps and reports what remains.
Private service memory belongs outside the source checkout/public Git.
See [the two-stage procedure](BOOTSTRAP.md) and [installation details](docs/install-system.md).

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
