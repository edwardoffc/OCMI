# Using OCMI

[English](usage.md) · [Русский](../locales/ru/docs/usage.md)

Choose the [English work template](../templates/work/INSTALL.md) or
[Russian work template](../locales/ru/templates/work/INSTALL.md).
Installed copies are independent; routine chats read their own project's memory.

## New project

Start with [BOOTSTRAP](../BOOTSTRAP.md): brief orientation, selected local installation,
then a separate permanent coordinator in a project attached to the system folder.
Use the filled private startup prompt; discard the first chat only after confirmed handover.
The coordinator offers optional personal storage, organization and project integration.
[System installation](install-system.md) explains paths, access and recovery.
Direct template adoption below remains available without a central coordinator.
An explicit installation request already selects its action; do not repeat authorization.

Open the project directory and give the agent the exact path to the selected INSTALL.md:

> This is a work project. Install OCMI from the supplied template.
> Fill the goal and initial state using verified facts only, and check links.

The agent prepares AGENTS.md, memory, and a short guide.
Then ask the orchestrator for a plan and prompts for workers and the consultant.
Creating chats and choosing models are separate actions; a START template is not an assignment.

## Existing project

> Adopt OCMI while preserving existing instructions, human documentation,
> active assignments, and unfinished changes. Show where the memory has moved.

The agent reconciles rules, retains one PLAN, and checks new routes.
Memory links to source documentation. Do not replace populated files with blank templates.

## Resuming in a new chat

Before ending a chat, save its result, unfinished work, and next step in the handoff.
In the new chat, specify project, role, and handoff path:

> Continue as the orchestrator. Read AGENTS.md, your role inputs, and the specified handoff.
> Verify the current state and continue from the recorded next step.

A worker also needs its current assignment, START/revision, and working copy.
It verifies their validity before implementation. An old START alone does not authorize work.
Unsaved conversations cannot be restored automatically.

## Everyday cycle

The orchestrator prepares criteria, checks, and START. The worker implements the task,
runs checks, and saves Result in the required format.
Give the orchestrator the ID and result path; copying the report between chats is unnecessary
when both can access the same files.

The orchestrator reviews, records Acceptance, integrates, and updates affected memory.
After integration it records cleanup: done or pending with reason, owner, and next action.
Questions can go to the consultant; pass accepted decisions to the orchestrator.

## Upgrading an installation or changing language

Specify the new template and ask to preserve local rules and current work.
INSTALL describes migration: 1.2 introduced role procedures; 1.2.1 added resource completion.
Changing only the version number is insufficient. Preserve active packages/checkers;
changed assignment terms require an explicit new revision.
A language change is also a merge, not permission to replace current memory with empty files.
Choose one template language for a project; human communication can use the user's preferred language.

## Other options

For a small independent topic use the [lightweight profile](../templates/personal/INSTALL.md):
entry point, index, brief, and handoff without mandatory orchestration.
Its historical directory name personal does not require a central personal store.

If the client does not discover AGENTS.md, provide instructions and relevant files explicitly.
File rules guide context selection; they do not establish technical access permissions.

[Layout](layout.md) · [Checks](../scripts/README.md).

Work 1.2.2 and lightweight 1.2.1 add sanitization before persistence/sharing. Merge those rules explicitly; do not replace active memory or protected packages. See the security policy before publishing logs.
