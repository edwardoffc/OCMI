# OCMI 1.3.3

Release date: 2026-10-13. Compatible workflow patch: compact low-risk tasks, sufficient checks and causal failure diagnosis.

| Component | Version |
| --- | --- |
| System | 1.3.3 |
| Coordinator service profile and setup policy | 1.3.2 |
| Work protocol | 1.2.3 |
| Lightweight protocol | 1.2.1 |

Choose full/compact by risk; scope, protected inputs, required project gates and acceptance remain mandatory.
Validate existing coverage before adding tests; run relevant cheap checks before expensive CI.
A successful retry alone does not resolve a failure. Repeated rework needs preserved evidence and a changed diagnostic approach.
Read changed records during a verified continuation; new chats still restore required inputs.

Installation keeps the same URL/downloaded flow, local source, bound client project and initialized coordinator.
Private coordinator memory stays in ignored/export-excluded .local/coordinator.
Personal storage, folder organization and work integration remain optional after initialization.
Two new work ZIPs accompany unchanged coordinator 1.3.2-r2 and lightweight 1.2.1-r2 editions.
Previously issued archives/contracts remain unchanged. Installed projects are not migrated automatically.
Windows/macOS use the same process; new changes require native CI verification at delivery.

[Start](BOOTSTRAP.md) · [Installation](docs/install-system.md) ·
[Release report](docs/releases/1.3.3.md) · [Changelog](CHANGELOG.md) · [Platforms](docs/platforms.md).
