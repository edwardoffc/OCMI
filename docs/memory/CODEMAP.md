# OCMI source map

Verified: 2026-10-13 for affected workflow/checker entries; unchanged setup/lightweight rows retain their earlier verification.
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
| templates/work/INSTALL.md / Installation/migration | VERSION, role procedures, task protocol | 2026-10-13; SHA256 87ccd26aaaee86d5d54adb9f6dba84010d450a42596b5a6d474d316ab55c77cf |
| templates/work/docs/memory/ORCHESTRATION.md / Role routing | Three role procedures; check-workflow | 2026-10-13; SHA256 b6c60e07d36b66d37c0d2d6daf97d296abb97ca16a25ef0fa299b7c62e1a9327 |
| templates/personal/AGENTS.md / Lightweight startup | Local INDEX, brief, handoff; check-workflow | 2026-10-11; SHA256 73fd35223e827af7fabe87ba1aae9fb7b6098256943dd395c1e6dcbb16f35da6 |
| templates/work/docs/memory/roles/coordinator.md / Acceptance/resource completion | Contract, Result, Acceptance; protected evidence | 2026-10-13; SHA256 c154a4a04d3349755c11d7dcc9cb15b7608da632cfc96542ce6d60bac8aa1197 |
| templates/work/docs/memory/TASK-PROTOCOL.md / Task schema and cleanup | Stable headings and separate done/pending status | 2026-10-13; SHA256 8ecdd7edadefabf45cb1b737b0e32acbba537514d03b677bc87b25e5d8118ac0 |
| scripts/check-docs.ps1 / Markdown/link/schema checks | English/Russian guides and templates; archive checksums | 2026-10-13; SHA256 e7cbcf40353c23463839c4c50725020976b82cd591043f7bd3f27b37574be359 |
| scripts/check-workflow.ps1 / Get-Routes / Read-Scoped / Check | localization/workflow-guards.json; both languages | 2026-10-13; SHA256 4fbbba013ec6a5a23454936fdf2f23ad3abb872555493d827e488d316baf8fde |
| scripts/check-localization.ps1 / Scoped-File / pair inventory | localization/manifest.json; hashes and paths | 2026-10-12; SHA256 9658f60643cc5116a9ea4edba8c447998ef2c16c35edd87009ddfb045fe79227 |
| scripts/check-release.ps1 / Receipt/source/ZIP verification | 1.3.3-components receipt; historical SecurityBaseline/ArchivesOnly | 2026-10-13; SHA256 1ffeb97d79470757220739a42a81a662d08243fabbf4e4fee64398b20d6121ba |
| scripts/check-security.ps1 / tracked inventory and extracted archives | Built-in Gitleaks rules, home/email indicators; CI pinned scanner | 2026-10-12; SHA256 ce44181a6ed0f1499c5f2208ff841115458088fb3d272dcc72502dc606ca85d0 |
| AGENTS.md / setup routing and selected-action boundaries | System 1.3.3, setup policy 1.3.2; coordinator receipt and onboarding checks | 2026-10-13; SHA256 945a12a9ebea9c5d9e7779e50ed9e16d6957c2767476d9054bc9b4a1e6ca5b3f |
| BOOTSTRAP.md / setup routing and selected-action boundaries | System 1.3.3, setup policy 1.3.2; coordinator receipt and onboarding checks | 2026-10-13; SHA256 e27cc50201d6a3cb702aa58db27f647fc33c88d1d64f102ed1fc515c681c3031 |
| templates/coordinator/AGENTS.md / setup routing and selected-action boundaries | System 1.3.3, setup policy 1.3.2; coordinator receipt and onboarding checks | 2026-10-12; SHA256 7f4773855a1a198bf7f372786aded9dfcab497d2d97e678ed587d8fd52dd904a |
| scripts/check-onboarding.ps1 / setup routing and selected-action boundaries | System 1.3.3, setup policy 1.3.2; coordinator receipt and onboarding checks | 2026-10-12; SHA256 1aeaf78d639d6abc02b37c270d6fb03ce9f42a845611e540d151c6d996ed0f3a |
| COORDINATOR.md / installer-to-coordinator handover | Source 1.3.2, private prompt and required inputs; check-onboarding | 2026-10-12; SHA256 bbef218b8e81fb99a2dadf3003b94bb0287517d29f12c4c639bd8350c7204de2 |
| docs/install-system.md / installer-to-coordinator handover | Source 1.3.2, private prompt and required inputs; check-onboarding | 2026-10-12; SHA256 f08b1ab3a3bd2613c8f4f59a21d2437cb34dda8ec43eed835bd166fbd38ee4de |
| templates/coordinator/STARTUP.md / installer-to-coordinator handover | Source 1.3.2, private prompt and required inputs; check-onboarding | 2026-10-12; SHA256 1c741f5878108cf8bce02613f294600a7e096a35bb7fb8d1ac1a27312f65582d |
| README.md / both startup routes and user checklist | Bound project/core prerequisite; check-onboarding | 2026-10-13; SHA256 a5a423c71727767b01a6eab238f497c9fb578e19200eb46ce521ae95ac74673f |
| docs/work-project-routing.md / component selection and entry gate | Global/client-neutral rule; existing installation exception; check-onboarding | 2026-10-12; SHA256 26166d3f6dc0f962440f5a11da0bc0108418335e1cb657db30b49842ed0f222c |
| templates/personal/INSTALL.md / lightweight installation entry | Same coordinator requirement; runtime profile remains 1.2.1; component receipt | 2026-10-12; SHA256 f0f461495722dd1a6384ff2d76513d4d0adc066cafb645cad1888ef389cd616e |
| templates/work/docs/memory/tasks/SMALL-TEMPLATE.md / compact task | Same schema and safeguarded criterion coverage | 2026-10-13; check-flow and check-docs |
| scripts/check-flow.ps1 / bilingual regression mutations | localization/flow-guards.json; compact fields/schema and six policy categories | 2026-10-13; 110 positive/negative instruction checks |
| docs/releases/1.3.3-components.json / current component inventory | Two new work editions and two reused lightweight editions; coordinator receipt unchanged | 2026-10-13; check-release exact source/ZIP inventories |

Renames, interfaces, or runner changes invalidate affected rows. Verify only the relevant area.
The [current component receipt](../releases/1.3.3-components.json) records complete template file snapshots.
This map locates sources; it does not establish model behavior.
