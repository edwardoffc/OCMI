# OCMI

**English** · [Русский](README.ru.md)

**Orchestration, Context Memory & Indexing** — a file-based workflow for AI agents.

OCMI helps you work on a project across multiple chats and resume it in new ones.
Roles, decisions, task assignments, and next steps live alongside the project in Markdown.
A code map points agents to the relevant parts of the codebase, while task records connect
requirements, results, and verification.

**OCMI system: [1.3.2](VERSION.md)** · [Start here](BOOTSTRAP.md) ·
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

Windows and macOS follow the same steps. Ordinary file-based use needs no PowerShell;
source checks use PowerShell 7. See [platform support](docs/platforms.md).

Both routes reach the same result: **local system → bound client project →
coordinator chat with saved service memory**.

### From a URL

Give [the repository URL](https://github.com/edwardoffc/OCMI) to an ordinary chat and ask:

> Study OCMI and help me start using it.

The first chat briefly explains OCMI, gives the folder/project/coordinator steps and offers setup.
After selection it helps obtain the local system in `00_OCMI` or `OCMI`.

### From a downloaded copy

Unpack the full repository into your chosen folder. Create a local OCMI project in the AI client,
attach that folder, open a first chat inside it and send the same request.
The chat verifies and reuses the download and existing project; no duplicates are needed.

### Complete either route

1. The first chat prepares private service memory and a filled startup prompt,
   then gives user actions with concrete paths and names.
2. Create/select the OCMI project bound to the system folder.
   Inside it create a separate “OCMI Coordinator” chat and send the prompt.
   Allow access to the exact private service directory if needed.
3. The coordinator verifies files, client project/binding and confirms startup.
   Until then setup is incomplete; keep the first chat.
4. The coordinator now explains usage and offers optional personal storage,
   folder organization and work-project integration. You may retire the first chat.

A first request to “integrate OCMI into my project” also passes through core setup.
Preserve it as deferred scope; change the target only after coordinator confirmation.
Already configured projects continue using their own memory.

A question or URL alone installs nothing. If the client cannot bind a project/equivalent workspace
or access required files, the chat explains remaining steps and reports setup pending.
Private service memory lives in `.local/coordinator` inside the system folder;
`.local` is excluded from Git and public distributions. Personal storage is optional.
[Full installation guide](docs/install-system.md) · [Installer route](BOOTSTRAP.md).

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
