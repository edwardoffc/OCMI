# OCMI source map

Verified: 2026-10-12 for affected setup/checker entries; retained work 1.2.2/lightweight 1.2.1 snapshots.
Hashes identify the inspected file snapshot; refresh only affected entries after changes.

| File / entry | Connections | Verification source |
| --- | --- | --- |
| scripts/check-local-boundary.ps1 / private path boundary | .gitignore, .gitattributes, public index/history and ZIP paths; CI | 2026-10-12; isolated positive/negative fixtures and Git archive export |
| templates/work/INSTALL.md / Installation/migration | VERSION, role procedures, task protocol | 2026-10-12; SHA256 59580fa9d3c9db593416a701d71c239fcc7200ad8668d9dcc1a0f70d4ad26f39 |
| templates/work/docs/memory/ORCHESTRATION.md / Role routing | Three role procedures; check-workflow | 2026-10-11; SHA256 c30fe3814890ecd5e995eeec8778ad9393e00afc32ef512417a3c2889b11e4ba |
| templates/personal/AGENTS.md / Lightweight startup | Local INDEX, brief, handoff; check-workflow | 2026-10-11; SHA256 73fd35223e827af7fabe87ba1aae9fb7b6098256943dd395c1e6dcbb16f35da6 |
| templates/work/docs/memory/roles/coordinator.md / Acceptance/resource completion | Contract, Result, Acceptance; protected evidence | 2026-10-11; SHA256 9f02b621cd35aaa98fe57dbcec2117abb8ebdfad426c7a4f79b824d72fd0f289 |
| templates/work/docs/memory/TASK-PROTOCOL.md / Task schema and cleanup | Stable headings and separate done/pending status | 2026-10-11; SHA256 e7543bb63b668c4e46ff8149660d22b62e8e17719f5a3d2c4b4f87ef8c8564ad |
| scripts/check-docs.ps1 / Markdown/link/schema checks | English/Russian guides and templates; archive checksums | 2026-10-12; SHA256 0f7ef063d38bae59e8297b2be7c6921e105cca9763ce5b257480b2c4515d2e0f |
| scripts/check-workflow.ps1 / Get-Routes / Read-Scoped / Check | localization/workflow-guards.json; both languages | 2026-10-11; SHA256 84946128cf7e632af03951ff99b0fe74115418ad81392cadf7dfeb0da2dfc29e |
| scripts/check-localization.ps1 / Scoped-File / pair inventory | localization/manifest.json; hashes and paths | 2026-10-12; SHA256 e8d8c13c0dbad1ed2fd33f3b038c430a859ee0dac769671521bb139ede8ffb70 |
| scripts/check-release.ps1 / Receipt/source/ZIP verification | 1.3.2-components receipt; historical SecurityBaseline/ArchivesOnly | 2026-10-12; SHA256 bc1d4b201451b26dac14a527e4a763ffa4da8c89d48240f3c66473b0c756f35d |
| scripts/check-security.ps1 / tracked inventory and extracted archives | Built-in Gitleaks rules, home/email indicators; CI pinned scanner | 2026-10-11; SHA256 759ccf52325edd6894b95a2fa9ab3915b156e785c47b818a6552d6989ed9212c |
| AGENTS.md / setup routing and selected-action boundaries | System 1.3.2; coordinator receipt and onboarding checks | 2026-10-12; SHA256 e3a3a766efc689ec1b6ef51a9e745f39b65d8c410453060cc0ca0c49f9fd470a |
| BOOTSTRAP.md / setup routing and selected-action boundaries | System 1.3.2; coordinator receipt and onboarding checks | 2026-10-12; SHA256 ef66d4762017252276acac2ae980ed7a78cf6486e3203a1b5bc0c313455ca6cd |
| templates/coordinator/AGENTS.md / setup routing and selected-action boundaries | System 1.3.2; coordinator receipt and onboarding checks | 2026-10-12; SHA256 7f4773855a1a198bf7f372786aded9dfcab497d2d97e678ed587d8fd52dd904a |
| scripts/check-onboarding.ps1 / setup routing and selected-action boundaries | System 1.3.2; coordinator receipt and onboarding checks | 2026-10-12; SHA256 8c61fae4a8688207a67b12df0677b93f893d41244f0fe93168e7c1b8bcab2f63 |
| COORDINATOR.md / installer-to-coordinator handover | Source 1.3.2, private prompt and required inputs; check-onboarding | 2026-10-12; SHA256 bbef218b8e81fb99a2dadf3003b94bb0287517d29f12c4c639bd8350c7204de2 |
| docs/install-system.md / installer-to-coordinator handover | Source 1.3.2, private prompt and required inputs; check-onboarding | 2026-10-12; SHA256 4b3457957bf60d517edaf3a08c97001be465daf322b97297b4c34354daf5dc3a |
| templates/coordinator/STARTUP.md / installer-to-coordinator handover | Source 1.3.2, private prompt and required inputs; check-onboarding | 2026-10-12; SHA256 1c741f5878108cf8bce02613f294600a7e096a35bb7fb8d1ac1a27312f65582d |
| README.md / both startup routes and user checklist | Bound project/core prerequisite; check-onboarding | 2026-10-12; SHA256 3c54901a3063db6daca1b0676cfdfd859473a4845288b3e10b6a6be13d78d8b0 |
| docs/work-project-routing.md / component selection and entry gate | Global/client-neutral rule; existing installation exception; check-onboarding | 2026-10-12; SHA256 26166d3f6dc0f962440f5a11da0bc0108418335e1cb657db30b49842ed0f222c |
| templates/personal/INSTALL.md / lightweight installation entry | Same coordinator requirement; runtime profile remains 1.2.1; component receipt | 2026-10-12; SHA256 f0f461495722dd1a6384ff2d76513d4d0adc066cafb645cad1888ef389cd616e |

Renames, interfaces, or runner changes invalidate affected rows. Verify only the relevant area.
The [language receipt](../releases/security-validation.json) records complete template file snapshots.
This map locates sources; it does not establish model behavior.
