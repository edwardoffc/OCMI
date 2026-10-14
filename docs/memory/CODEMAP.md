# OCMI source map

Verified: 2026-10-14 for affected workflow/checker entries; unchanged setup/lightweight rows retain their earlier verification.
Hashes identify the inspected file snapshot; refresh only affected entries after changes.

| File / entry | Connections | Verification source |
| --- | --- | --- |
| scripts/check-all.ps1 / complete suite | Flow guards plus all source checks and whitespace; Windows/macOS CI | 2026-10-13; local suite; new native CI pending delivery |
| scripts/path-utils.ps1 / scoped files and temporary guards | Native separators, Unix case, link rejection and owned cleanup | 2026-10-12; check-platform and native CI |
| scripts/check-platform.ps1 / platform fixtures | Spaces/Unicode, scope/case, links, cleanup and copied consumers | 2026-10-12; 20 Windows / 21 Unix checks |
| scripts/check-boundaries.ps1 / private-boundary mutations | Ignore/export, forced index, reachable history and root/nested ZIP paths | 2026-10-12; 8 synthetic checks on all runners |
| .github/workflows/validate.yml / native matrices and aggregates | Windows/macOS ARM64/x64; pinned Gitleaks; stable main protection | 2026-10-12; eight CI jobs in PR #7 |
| docs/platforms.md / common installation and validation | URL/download flow; native paths; PowerShell 7 maintenance | 2026-10-12; reviewed English/Russian guides |
| scripts/check-local-boundary.ps1 / private path boundary | .gitignore, .gitattributes, public index/history and ZIP paths; CI | 2026-10-12; isolated positive/negative fixtures and Git archive export |
| templates/work/INSTALL.md / Installation/migration | VERSION, role procedures, task protocol | 2026-10-13; SHA256 667d8f7301d9574edb3c77cc8a1921199bd459ba5aa1444004341082485f90c6 |
| templates/work/docs/memory/ORCHESTRATION.md / Role routing | Three role procedures; check-workflow | 2026-10-14; SHA256 8ceab5d6f1a0a53ea70406ac1908f4ab965807380f3252533ea411841bf5acba |
| templates/personal/AGENTS.md / Lightweight startup | Local INDEX, brief, handoff; check-workflow | 2026-10-11; SHA256 73fd35223e827af7fabe87ba1aae9fb7b6098256943dd395c1e6dcbb16f35da6 |
| templates/work/docs/memory/roles/coordinator.md / Acceptance/resource completion | Contract, Result, Acceptance; protected evidence | 2026-10-14; SHA256 41b31891488cb4d871a6bfe068298efff87b137ffda50a1ea338a5501c40864d |
| templates/work/docs/memory/TASK-PROTOCOL.md / Task schema and cleanup | Stable headings and separate done/pending status | 2026-10-14; SHA256 efeccdea97ce175bc29d1ad96d718a0f57c165c9abc76beb9f6a02c86fb1e40e |
| scripts/check-docs.ps1 / Markdown/link/schema checks | English/Russian guides and templates; archive checksums | 2026-10-13; SHA256 e7cbcf40353c23463839c4c50725020976b82cd591043f7bd3f27b37574be359 |
| scripts/check-workflow.ps1 / Get-Routes / Read-Scoped / Check | localization/workflow-guards.json; both languages | 2026-10-14; SHA256 863c8b90cdf3b309181cb3c8379607b6de475d0fcc54c0e9a8a9bc78d9aa8600 |
| scripts/check-localization.ps1 / Scoped-File / pair inventory | localization/manifest.json; hashes and paths | 2026-10-12; SHA256 9658f60643cc5116a9ea4edba8c447998ef2c16c35edd87009ddfb045fe79227 |
| scripts/check-release.ps1 / Receipt/source/ZIP verification | 1.3.3-flow receipt; historical SecurityBaseline/ArchivesOnly | 2026-10-14; SHA256 b2ae38db3083fed802da366689cf9a08adf9b2044416a210ff28b8adbbba181a |
| scripts/check-security.ps1 / tracked inventory and extracted archives | Built-in Gitleaks rules, home/email indicators; CI pinned scanner | 2026-10-12; SHA256 ce44181a6ed0f1499c5f2208ff841115458088fb3d272dcc72502dc606ca85d0 |
| AGENTS.md / setup routing and selected-action boundaries | OCMI 1.3.3, unified numbering; coordinator receipt and onboarding checks | 2026-10-14; SHA256 b826055efca5cd1b2043c49533d84a3e3dab86e6a7f38eab8b91cf112725daf4 |
| BOOTSTRAP.md / setup routing and selected-action boundaries | OCMI 1.3.3, unified numbering; coordinator receipt and onboarding checks | 2026-10-13; SHA256 81d34ec896f95ed363baca3c397d68e002d7f254fba9edf7398a593e234a895e |
| templates/coordinator/AGENTS.md / setup routing and selected-action boundaries | OCMI 1.3.3, unified numbering; coordinator receipt and onboarding checks | 2026-10-13; SHA256 9d6e38bfa1a25c984f13908d2ae0d04144cd26016fa8f675f68c69df8a73a5fc |
| scripts/check-onboarding.ps1 / setup routing and selected-action boundaries | OCMI 1.3.3, unified numbering; coordinator receipt and onboarding checks | 2026-10-14; SHA256 f529a6b0a0a067e82de8cdbe10973168d8378664955265c512bf26f3d0ef0c41 |
| COORDINATOR.md / installer-to-coordinator handover | OCMI 1.3.3, private prompt and required inputs; check-onboarding | 2026-10-13; SHA256 a4429ecb7a672ea5b45e18e0e7d1b009d3040cd738f157a6871036479d00f914 |
| docs/install-system.md / installer-to-coordinator handover | OCMI 1.3.3, private prompt and required inputs; check-onboarding | 2026-10-12; SHA256 f08b1ab3a3bd2613c8f4f59a21d2437cb34dda8ec43eed835bd166fbd38ee4de |
| templates/coordinator/STARTUP.md / installer-to-coordinator handover | OCMI 1.3.3, private prompt and required inputs; check-onboarding | 2026-10-13; SHA256 1c741f5878108cf8bce02613f294600a7e096a35bb7fb8d1ac1a27312f65582d |
| README.md / both startup routes and user checklist | Bound project/core prerequisite; check-onboarding | 2026-10-13; SHA256 a5a423c71727767b01a6eab238f497c9fb578e19200eb46ce521ae95ac74673f |
| docs/work-project-routing.md / component selection and entry gate | Global/client-neutral rule; existing installation exception; check-onboarding | 2026-10-12; SHA256 26166d3f6dc0f962440f5a11da0bc0108418335e1cb657db30b49842ed0f222c |
| templates/personal/INSTALL.md / lightweight installation entry | Same coordinator requirement; OCMI version 1.3.3; component receipt | 2026-10-13; SHA256 03120feb0d63e68c6a68621b2a4b5c648ab3a015e279631bf75b3c20e2cc74a8 |
| templates/work/docs/memory/tasks/SMALL-TEMPLATE.md / compact task | Same schema and safeguarded criterion coverage | 2026-10-13; check-flow and check-docs |
| scripts/check-flow.ps1 / bilingual regression mutations | localization/flow-guards.json; compact fields/schema, baseline and whole-result failure handling | 2026-10-14; 148 instruction guards/mutations and development parity |
| docs/releases/1.3.3-flow.json / current unified inventory | Six editions at one OCMI version; historical receipts preserved | 2026-10-14; check-release exact source/ZIP inventories |
| scripts/check-versioning.ps1 / shared release label | version-utils.ps1; unified six-profile receipt and version/header drift mutations | 2026-10-14; 85 static guards |

Renames, interfaces, or runner changes invalidate affected rows. Verify only the relevant area.
The [current component receipt](../releases/1.3.3-flow.json) records complete template file snapshots.
This map locates sources; it does not establish model behavior.
