# OCMI source map

Verified: 2026-10-12 for affected setup/checker entries; retained work 1.2.2/lightweight 1.2.1 snapshots.
Hashes identify the inspected file snapshot; refresh only affected entries after changes.

| File / entry | Connections | Verification source |
| --- | --- | --- |
| scripts/check-all.ps1 / complete suite | All source checks and whitespace; Windows/macOS CI | 2026-10-12; native Windows/Apple Silicon/Intel CI |
| scripts/path-utils.ps1 / scoped files and temporary guards | Native separators, Unix case, link rejection and owned cleanup | 2026-10-12; check-platform and native CI |
| scripts/check-platform.ps1 / platform fixtures | Spaces/Unicode, scope/case, links, cleanup and copied consumers | 2026-10-12; 20 Windows / 21 Unix checks |
| scripts/check-boundaries.ps1 / private-boundary mutations | Ignore/export, forced index, reachable history and root/nested ZIP paths | 2026-10-12; 8 synthetic checks on all runners |
| .github/workflows/validate.yml / native matrices and aggregates | Windows/macOS ARM64/x64; pinned Gitleaks; stable main protection | 2026-10-12; eight CI jobs in PR #7 |
| docs/platforms.md / common installation and validation | URL/download flow; native paths; PowerShell 7 maintenance | 2026-10-12; reviewed English/Russian guides |
| scripts/check-local-boundary.ps1 / private path boundary | .gitignore, .gitattributes, public index/history and ZIP paths; CI | 2026-10-12; isolated positive/negative fixtures and Git archive export |
| templates/work/INSTALL.md / Installation/migration | VERSION, role procedures, task protocol | 2026-10-12; SHA256 59580fa9d3c9db593416a701d71c239fcc7200ad8668d9dcc1a0f70d4ad26f39 |
| templates/work/docs/memory/ORCHESTRATION.md / Role routing | Three role procedures; check-workflow | 2026-10-11; SHA256 c30fe3814890ecd5e995eeec8778ad9393e00afc32ef512417a3c2889b11e4ba |
| templates/personal/AGENTS.md / Lightweight startup | Local INDEX, brief, handoff; check-workflow | 2026-10-11; SHA256 73fd35223e827af7fabe87ba1aae9fb7b6098256943dd395c1e6dcbb16f35da6 |
| templates/work/docs/memory/roles/coordinator.md / Acceptance/resource completion | Contract, Result, Acceptance; protected evidence | 2026-10-11; SHA256 9f02b621cd35aaa98fe57dbcec2117abb8ebdfad426c7a4f79b824d72fd0f289 |
| templates/work/docs/memory/TASK-PROTOCOL.md / Task schema and cleanup | Stable headings and separate done/pending status | 2026-10-11; SHA256 e7543bb63b668c4e46ff8149660d22b62e8e17719f5a3d2c4b4f87ef8c8564ad |
| scripts/check-docs.ps1 / Markdown/link/schema checks | English/Russian guides and templates; archive checksums | 2026-10-12; SHA256 a29a71563415e797cd3c22494459814eea218e2da46d6cdc26ca42b18e6c6c94 |
| scripts/check-workflow.ps1 / Get-Routes / Read-Scoped / Check | localization/workflow-guards.json; both languages | 2026-10-12; SHA256 6df09afec2535331c71869b2ffad313f6e8e141f919fbd4a9454306c4483f897 |
| scripts/check-localization.ps1 / Scoped-File / pair inventory | localization/manifest.json; hashes and paths | 2026-10-12; SHA256 9658f60643cc5116a9ea4edba8c447998ef2c16c35edd87009ddfb045fe79227 |
| scripts/check-release.ps1 / Receipt/source/ZIP verification | 1.3.2-components receipt; historical SecurityBaseline/ArchivesOnly | 2026-10-12; SHA256 7112fc9d7838b847bf57676c71c3463092b9b3f178121fd5e042a239326b23ab |
| scripts/check-security.ps1 / tracked inventory and extracted archives | Built-in Gitleaks rules, home/email indicators; CI pinned scanner | 2026-10-12; SHA256 ce44181a6ed0f1499c5f2208ff841115458088fb3d272dcc72502dc606ca85d0 |
| AGENTS.md / setup routing and selected-action boundaries | System 1.3.2; coordinator receipt and onboarding checks | 2026-10-12; SHA256 22a7b13c43e8d4bffa0231cdb50b62de59fd7d8dbcb6ba40b8e86cfa435a8c16 |
| BOOTSTRAP.md / setup routing and selected-action boundaries | System 1.3.2; coordinator receipt and onboarding checks | 2026-10-12; SHA256 2fc4a9d5886fecb345c91125f9d92299e82febdae526c909d7f4bb3d9896f8ec |
| templates/coordinator/AGENTS.md / setup routing and selected-action boundaries | System 1.3.2; coordinator receipt and onboarding checks | 2026-10-12; SHA256 7f4773855a1a198bf7f372786aded9dfcab497d2d97e678ed587d8fd52dd904a |
| scripts/check-onboarding.ps1 / setup routing and selected-action boundaries | System 1.3.2; coordinator receipt and onboarding checks | 2026-10-12; SHA256 1aeaf78d639d6abc02b37c270d6fb03ce9f42a845611e540d151c6d996ed0f3a |
| COORDINATOR.md / installer-to-coordinator handover | Source 1.3.2, private prompt and required inputs; check-onboarding | 2026-10-12; SHA256 bbef218b8e81fb99a2dadf3003b94bb0287517d29f12c4c639bd8350c7204de2 |
| docs/install-system.md / installer-to-coordinator handover | Source 1.3.2, private prompt and required inputs; check-onboarding | 2026-10-12; SHA256 f08b1ab3a3bd2613c8f4f59a21d2437cb34dda8ec43eed835bd166fbd38ee4de |
| templates/coordinator/STARTUP.md / installer-to-coordinator handover | Source 1.3.2, private prompt and required inputs; check-onboarding | 2026-10-12; SHA256 1c741f5878108cf8bce02613f294600a7e096a35bb7fb8d1ac1a27312f65582d |
| README.md / both startup routes and user checklist | Bound project/core prerequisite; check-onboarding | 2026-10-12; SHA256 e0b04461ccf7c49a87fe69ccd33d5ed4fa2cb8a1cf0ca76ea4594f5673f55bce |
| docs/work-project-routing.md / component selection and entry gate | Global/client-neutral rule; existing installation exception; check-onboarding | 2026-10-12; SHA256 26166d3f6dc0f962440f5a11da0bc0108418335e1cb657db30b49842ed0f222c |
| templates/personal/INSTALL.md / lightweight installation entry | Same coordinator requirement; runtime profile remains 1.2.1; component receipt | 2026-10-12; SHA256 f0f461495722dd1a6384ff2d76513d4d0adc066cafb645cad1888ef389cd616e |

Renames, interfaces, or runner changes invalidate affected rows. Verify only the relevant area.
The [language receipt](../releases/security-validation.json) records complete template file snapshots.
This map locates sources; it does not establish model behavior.
