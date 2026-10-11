# OCMI current state

Snapshot: 2026-10-12. System release prepared: 1.3.1 (two-stage guided setup).
Work protocol: 1.2.2. Optional lightweight protocol: 1.2.1.
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
Coordinator service source lives in templates/coordinator; installed setup memory belongs
outside the source/public Git scope. The temporary installer offers local installation; a separate
permanent coordinator confirms handover and then offers optional personal/work actions.
Service profile has eight files per language; work/lightweight sources are unchanged.
Owner-authorized GitHub delivery of 1.3.1 uses protected-main checks and a verified tag/release.
Confirm publication from origin/main and delivery evidence; template versions do not describe local installations.
Local backups and credentials are excluded. [Continue here](tasks/orchestration/HANDOFF.md).
