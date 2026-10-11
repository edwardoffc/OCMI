# OCMI current state

Snapshot: 2026-10-12. System release prepared: 1.3.2 (uniform entry and required core completion).
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
Service profile has eight files per language. Work/lightweight runtime files remain unchanged;
INSTALL entry checks and work VERSION installer notes now require coordinator-led new installation.
Four component ZIPs dated 2026-10-12 accompany the two coordinator editions; prior archives are retained.
System/coordinator 1.3.2 adds the sixth startup field for expected client project/binding.
First integration waits for core confirmation; suitable existing copies/projects are reused.
1.3.1 delivery remains historical. The owner authorized 1.3.2 publication;
remote integration and all six release editions require separate delivery verification.
Local backups and credentials are excluded. [Continue here](tasks/orchestration/HANDOFF.md).
