# OCMI current state

Snapshot: 2026-10-13. System patch: 1.3.3 (compact tasks, sufficient checks and causal rework). The owner authorized GitHub delivery; verify final status through current refs/release and the ignored delivery receipt.
Retained setup correction: coordinator/lightweight distribution r2; coordinator memory at .local/coordinator inside the system,
ignored/export-excluded and rejected from public index/history/ZIPs by CI. No installed memory is moved.
Work protocol: 1.2.3; rules revision 2026-10-13. Setup/coordinator: 1.3.2. Optional lightweight protocol: 1.2.1.
English is canonical; Russian guides/templates are maintained under locales/ru.
docs/memory is the single development memory, not installed project data.

Language editions add LICENSE to each installable profile: work has 20 files, lightweight 6.
Original Russian archives (18 and 5 files for 1.2/1.2.1) remain unchanged.
[Historical verification](../releases/1.2.1.md) · [Distribution index](../../releases/README.md).

## Environment

| Capability / scope | Requirement | State | Evidence |
| --- | --- | --- | --- |
| PowerShell checks / repository root | PowerShell 7 and .NET ZIP support; Windows 5.1 compatibility | platform baseline verified; 1.3.3 local checks recorded separately, native macOS CI pending delivery | [Platform baseline](tasks/OCMI-MAC-017/HANDOFF.md); [current acceptance](tasks/OCMI-FLOW-018/HANDOFF.md) |
| Git / repository root | Native Git on Windows/macOS | available locally and on CI runners | Diff/export/history checks recorded at acceptance |
| Model behavior and task costs | Separate OCMI-EVAL-001 | unknown | Static checks do not establish behavioral outcomes |

The language edition preserves protocol fields and authority boundaries.
It does not automatically upgrade installed projects or grant publication authority.
Only neutral source files, guides, fixtures, and release artifacts belong in the public repository.
Coordinator service source lives in templates/coordinator; installed setup memory belongs
in .local/coordinator inside the system, excluded from Git/public packages. The temporary installer offers local installation; a separate
permanent coordinator confirms handover and then offers optional personal/work actions.
Service profile has eight files per language. Lightweight remains unchanged; work 1.2.3 adds compact cards, coverage/preflight and causal failure rules;
INSTALL entry checks and work VERSION installer notes now require coordinator-led new installation.
Two new work ZIPs dated 2026-10-13 use 1.3.3-components.json; current lightweight/coordinator editions and 29 previous ZIPs are retained.
System/coordinator 1.3.2 adds the sixth startup field for expected client project/binding.
First integration waits for core confirmation; suitable existing copies/projects are reused.
1.3.1 delivery remains historical. The owner authorized 1.3.2 publication;
its remote integration is recorded in historical acceptance. Owner-authorized 1.3.3 delivery requires native CI and source/asset verification.
Local backups and credentials are excluded. [Continue here](tasks/orchestration/HANDOFF.md).
