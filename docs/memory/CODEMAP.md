# OCMI source map

Verified: 2026-10-12 for affected setup/checker entries; retained work 1.2.2/lightweight 1.2.1 snapshots.
Hashes identify the inspected file snapshot; refresh only affected entries after changes.

| File / entry | Connections | Verification source |
| --- | --- | --- |
| templates/work/INSTALL.md / Installation/migration | VERSION, role procedures, task protocol | 2026-10-11; SHA256 fbbfb24d480279cb2be6711aff347eb589805307909350d2dd52666057942535 |
| templates/work/docs/memory/ORCHESTRATION.md / Role routing | Three role procedures; check-workflow | 2026-10-11; SHA256 c30fe3814890ecd5e995eeec8778ad9393e00afc32ef512417a3c2889b11e4ba |
| templates/personal/AGENTS.md / Lightweight startup | Local INDEX, brief, handoff; check-workflow | 2026-10-11; SHA256 73fd35223e827af7fabe87ba1aae9fb7b6098256943dd395c1e6dcbb16f35da6 |
| templates/work/docs/memory/roles/coordinator.md / Acceptance/resource completion | Contract, Result, Acceptance; protected evidence | 2026-10-11; SHA256 9f02b621cd35aaa98fe57dbcec2117abb8ebdfad426c7a4f79b824d72fd0f289 |
| templates/work/docs/memory/TASK-PROTOCOL.md / Task schema and cleanup | Stable headings and separate done/pending status | 2026-10-11; SHA256 e7543bb63b668c4e46ff8149660d22b62e8e17719f5a3d2c4b4f87ef8c8564ad |
| scripts/check-docs.ps1 / Markdown/link/schema checks | English/Russian guides and templates; archive checksums | 2026-10-12; SHA256 fea2201e6a5b5540ae9ac0ce79230210a1a2888440b42604e8d244c3761a4107 |
| scripts/check-workflow.ps1 / Get-Routes / Read-Scoped / Check | localization/workflow-guards.json; both languages | 2026-10-11; SHA256 84946128cf7e632af03951ff99b0fe74115418ad81392cadf7dfeb0da2dfc29e |
| scripts/check-localization.ps1 / Scoped-File / pair inventory | localization/manifest.json; hashes and paths | 2026-10-12; SHA256 858f6fefd9b2e4d2e2edee26f8947f073b60c88be4afb5e9c48715e8f764a27b |
| scripts/check-release.ps1 / Receipt/source/ZIP verification | docs/releases/security-validation.json; historical ArchivesOnly | 2026-10-12; SHA256 b3b90b12f82518c944e147fc194918b0c0d9509647ec14a3ce0c65f9b61e873c |
| scripts/check-security.ps1 / tracked inventory and extracted archives | Built-in Gitleaks rules, home/email indicators; CI pinned scanner | 2026-10-11; SHA256 71b63fc071928f9c1db4663ea9f2379565f8141f736bee09593516510d2b3301 |
| AGENTS.md / setup routing and selected-action boundaries | System 1.3.0; coordinator receipt and onboarding checks | 2026-10-12; SHA256 e224c0ae414bbdc918990c8c0f2a85d0af77ecf8c7cae23fe31402b227112061 |
| BOOTSTRAP.md / setup routing and selected-action boundaries | System 1.3.0; coordinator receipt and onboarding checks | 2026-10-12; SHA256 ce299416fb6df47e1c629e70bac69c9ce0e87fb4e6837f52715f5005a80fcce1 |
| templates/coordinator/AGENTS.md / setup routing and selected-action boundaries | System 1.3.0; coordinator receipt and onboarding checks | 2026-10-12; SHA256 c9b47f9ba3e8bcd16516d56f26d23159e99b2af7fd141238714255e00d842440 |
| scripts/check-onboarding.ps1 / setup routing and selected-action boundaries | System 1.3.0; coordinator receipt and onboarding checks | 2026-10-12; SHA256 69d9aa6d155e2777d04bb0a2f850697eecef30531a9124c6497bc351d238e3f8 |

Renames, interfaces, or runner changes invalidate affected rows. Verify only the relevant area.
The [language receipt](../releases/security-validation.json) records complete template file snapshots.
This map locates sources; it does not establish model behavior.
