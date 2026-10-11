# Installing file-based memory

**OCMI 1.2, rules revision 2026-10-11**

This directory is a portable neutral template. It contains no user data or other project information.
An agent installs it at the user's request; no program installation is required.

## Agent procedure

1. Determine target directory, scope (work/personal), and goal from the current request. The target must differ from the template directory. Clarify only genuinely missing scope/goal information. For an ongoing topic, describe what the user wants to discuss/track instead of a final deliverable.
2. Read only this template and the target area. No shared directory, profile, or neighboring projects are needed. Follow target instructions; check existing `AGENTS.md`, `AGENTS.override.md`, documents, and memory. In Git, first note the existing changes to preserve others' work.
3. Prepare the four files below. **Do not overwrite existing files with templates:** reconcile rules with current instructions and preserve content. If working memory already exists, use its paths and single current entry point instead of creating a second system just for these names.
4. Fill the brief and first task with confirmed information. Do not present assumed architecture, commands, or wishes as facts. Mark unknowns. Link to original documents rather than duplicating them.
5. Create additional files as needed: decisions, code map, significant-session log. The initial map lists verified entry points/code areas for the current task; it does not require reading the entire repository. Record source and verification date for each used map area.
6. Check relative links, the initial handoff, and instruction discovery when the client opens the target directory. Respect existing `AGENTS.override.md` precedence. Documentation checks do not require running the whole application. Do not claim new-chat recovery was tested unless it was.
7. Report memory location, how to resume, unknowns, and completed checks. Installing in one directory does not configure other projects, global automatic memory, or filesystem permissions.

Continuation verifies topic/task/directory, source, and remaining work.
Separate historical/canceled information from current state.
The lightweight profile does not require an orchestrator, work task records, LESSONS, or hooks.

## Minimum contents

Destination paths are relative to the selected project/topic root.

| Template source | Destination |
| --- | --- |
| [AGENTS.md](AGENTS.md) | `AGENTS.md` — agent instructions |
| [INDEX.md](memory/INDEX.md) | `memory/INDEX.md` — navigation and tasks |
| [BRIEF.md](memory/BRIEF.md) | `memory/BRIEF.md` — goal, scope, initial facts |
| [HANDOFF.md](memory/tasks/main/HANDOFF.md) | `memory/tasks/main/HANDOFF.md` — first task state |

Copying INSTALL.md is optional. The other files form a self-contained system without a link to the source setup store.

## Existing projects

- Preserve human documentation and uncommitted changes. Memory links to documentation instead of competing with it. Further development may require normal documentation updates.
- Locate existing AI notes within the project. If another directory is mentioned but its path is unknown, ask for it; do not scan the disk.
- Prepare `memory/MIGRATION.md`: old source → new location/link → checks → remaining migration. Preserve the origin of important decisions. Separate history from current facts.
- Keep originals available while verifying migration. Old entry points must clearly route to the single current state. Do not delete sources merely because a new handoff exists.
- If GSD, Serena, or another workflow is already used, extend its active memory/navigation. Do not create two independently updated accounts of current state.

## Other clients and access boundaries

The client must receive instructions in a supported format. Codex uses AGENTS.md;
discovery depends on project root and current directory.
Claude Code can use the documented `@AGENTS.md` import in CLAUDE.md to keep one rule source.
For other clients, consult their current documentation.

Directory separation and instructions control context selection, not access permissions.
For technical isolation, separately configure environment restrictions, connected sources,
and shared automatic memory. Inspect settings without opening others' private material to test access.

If the agent cannot read/write files, manually provide the index, brief, and relevant handoff,
then save updated files back. Do not describe that exchange as automatic filesystem installation.
