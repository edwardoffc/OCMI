# OCMI current state

Snapshot: 2026-10-11.
Work protocol: 1.2.1. Optional lightweight protocol: 1.2.
English is canonical; Russian guides/templates are maintained under locales/ru.
docs/memory is the single development memory, not installed project data.

Language editions add LICENSE to each installable profile: work has 19 files, lightweight 6.
Original Russian archives (18 and 5 files for 1.2/1.2.1) remain unchanged.
[Historical verification](../releases/1.2.1.md) · [Distribution index](../../releases/README.md).

## Environment

| Capability / scope | Requirement | State | Evidence |
| --- | --- | --- | --- |
| PowerShell checks / repository root | Windows PowerShell 5.1 and .NET ZIP support | verified for the recorded checks | [Current handoff](tasks/orchestration/HANDOFF.md) |
| Git / repository root | Git for Windows | available | Diff and publication checks recorded at acceptance |
| Model behavior and task costs | Separate OCMI-EVAL-001 | unknown | Static checks do not establish behavioral outcomes |

The language edition preserves protocol fields and authority boundaries.
It does not automatically upgrade installed projects or grant publication authority.
Only neutral source files, guides, fixtures, and release artifacts belong in the public repository.
Local backups and credentials are excluded. [Continue here](tasks/orchestration/HANDOFF.md).
