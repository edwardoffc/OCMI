# OCMI source map

Verified: 2026-10-12 for affected setup/checker entries; retained work 1.2.2/lightweight 1.2.1 snapshots.
Hashes identify the inspected file snapshot; refresh only affected entries after changes.

| File / entry | Connections | Verification source |
| --- | --- | --- |
| templates/work/INSTALL.md / Installation/migration | VERSION, role procedures, task protocol | 2026-10-12; SHA256 064896519e0d8978ae8d9bc05c48a42c3730801416d26c2dbc03bae6abc23e1e |
| templates/work/docs/memory/ORCHESTRATION.md / Role routing | Three role procedures; check-workflow | 2026-10-11; SHA256 c30fe3814890ecd5e995eeec8778ad9393e00afc32ef512417a3c2889b11e4ba |
| templates/personal/AGENTS.md / Lightweight startup | Local INDEX, brief, handoff; check-workflow | 2026-10-11; SHA256 73fd35223e827af7fabe87ba1aae9fb7b6098256943dd395c1e6dcbb16f35da6 |
| templates/work/docs/memory/roles/coordinator.md / Acceptance/resource completion | Contract, Result, Acceptance; protected evidence | 2026-10-11; SHA256 9f02b621cd35aaa98fe57dbcec2117abb8ebdfad426c7a4f79b824d72fd0f289 |
| templates/work/docs/memory/TASK-PROTOCOL.md / Task schema and cleanup | Stable headings and separate done/pending status | 2026-10-11; SHA256 e7543bb63b668c4e46ff8149660d22b62e8e17719f5a3d2c4b4f87ef8c8564ad |
| scripts/check-docs.ps1 / Markdown/link/schema checks | English/Russian guides and templates; archive checksums | 2026-10-12; SHA256 0f7ef063d38bae59e8297b2be7c6921e105cca9763ce5b257480b2c4515d2e0f |
| scripts/check-workflow.ps1 / Get-Routes / Read-Scoped / Check | localization/workflow-guards.json; both languages | 2026-10-11; SHA256 84946128cf7e632af03951ff99b0fe74115418ad81392cadf7dfeb0da2dfc29e |
| scripts/check-localization.ps1 / Scoped-File / pair inventory | localization/manifest.json; hashes and paths | 2026-10-12; SHA256 e8d8c13c0dbad1ed2fd33f3b038c430a859ee0dac769671521bb139ede8ffb70 |
| scripts/check-release.ps1 / Receipt/source/ZIP verification | 1.3.2-components receipt; historical SecurityBaseline/ArchivesOnly | 2026-10-12; SHA256 3eadda0bea49b824ec06e57b9a8247126485a11e396638f879c137e78e58ba72 |
| scripts/check-security.ps1 / tracked inventory and extracted archives | Built-in Gitleaks rules, home/email indicators; CI pinned scanner | 2026-10-11; SHA256 71b63fc071928f9c1db4663ea9f2379565f8141f736bee09593516510d2b3301 |
| AGENTS.md / setup routing and selected-action boundaries | System 1.3.2; coordinator receipt and onboarding checks | 2026-10-12; SHA256 aaaa1c7c1c28d9a72f6d728bed369b4ccc72ad446bf9825c902ed440043613c2 |
| BOOTSTRAP.md / setup routing and selected-action boundaries | System 1.3.2; coordinator receipt and onboarding checks | 2026-10-12; SHA256 bb5dafcf207570e59a929f6640f45b1b05721ed45ed84ae4708b2f7e108e59f3 |
| templates/coordinator/AGENTS.md / setup routing and selected-action boundaries | System 1.3.2; coordinator receipt and onboarding checks | 2026-10-12; SHA256 cc7a83dd2e2c1aec1423fe59a302f3f46e041f26a17a3802296f526e6257a73b |
| scripts/check-onboarding.ps1 / setup routing and selected-action boundaries | System 1.3.2; coordinator receipt and onboarding checks | 2026-10-12; SHA256 c4613fb7e298a8ef970b1efdddf0309940e5ef3c6709029e02105e4d07815cc4 |
| COORDINATOR.md / installer-to-coordinator handover | Source 1.3.2, private prompt and required inputs; check-onboarding | 2026-10-12; SHA256 9f103ed9dfad2fea6f2bcb320c1cf2964c558035651059e0a388b654d86bcf56 |
| docs/install-system.md / installer-to-coordinator handover | Source 1.3.2, private prompt and required inputs; check-onboarding | 2026-10-12; SHA256 ceeabf42f7b6dad045013afaf82e76ce7e6dff4ff5c76441c5aec8c84c2ece02 |
| templates/coordinator/STARTUP.md / installer-to-coordinator handover | Source 1.3.2, private prompt and required inputs; check-onboarding | 2026-10-12; SHA256 e809d452a749af043d54759bd7e37c6db73d8a334dd3dac78d05c3212ed82103 |
| README.md / both startup routes and user checklist | Bound project/core prerequisite; check-onboarding | 2026-10-12; SHA256 c105ec8ee23569b5eb9df006ce1697a8184cd191f1be1bb58aa7f1e4d2bb48ce |
| docs/work-project-routing.md / component selection and entry gate | Global/client-neutral rule; existing installation exception; check-onboarding | 2026-10-12; SHA256 26166d3f6dc0f962440f5a11da0bc0108418335e1cb657db30b49842ed0f222c |
| templates/personal/INSTALL.md / lightweight installation entry | Same coordinator requirement; runtime profile remains 1.2.1; component receipt | 2026-10-12; SHA256 0cd898b472e11e6681bc1f004c6e22922132f0f7f8d850d9bb580cde79fa33a2 |

Renames, interfaces, or runner changes invalidate affected rows. Verify only the relevant area.
The [language receipt](../releases/security-validation.json) records complete template file snapshots.
This map locates sources; it does not establish model behavior.
