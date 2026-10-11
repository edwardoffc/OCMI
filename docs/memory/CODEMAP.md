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
| scripts/check-docs.ps1 / Markdown/link/schema checks | English/Russian guides and templates; archive checksums | 2026-10-12; SHA256 0f7ef063d38bae59e8297b2be7c6921e105cca9763ce5b257480b2c4515d2e0f |
| scripts/check-workflow.ps1 / Get-Routes / Read-Scoped / Check | localization/workflow-guards.json; both languages | 2026-10-11; SHA256 84946128cf7e632af03951ff99b0fe74115418ad81392cadf7dfeb0da2dfc29e |
| scripts/check-localization.ps1 / Scoped-File / pair inventory | localization/manifest.json; hashes and paths | 2026-10-12; SHA256 e8d8c13c0dbad1ed2fd33f3b038c430a859ee0dac769671521bb139ede8ffb70 |
| scripts/check-release.ps1 / Receipt/source/ZIP verification | docs/releases/security-validation.json; historical ArchivesOnly | 2026-10-12; SHA256 691ad7a0e6c4d5356b2b0360f2649864b1420972f5e024371b60f738f3039fce |
| scripts/check-security.ps1 / tracked inventory and extracted archives | Built-in Gitleaks rules, home/email indicators; CI pinned scanner | 2026-10-11; SHA256 71b63fc071928f9c1db4663ea9f2379565f8141f736bee09593516510d2b3301 |
| AGENTS.md / setup routing and selected-action boundaries | System 1.3.1; coordinator receipt and onboarding checks | 2026-10-12; SHA256 d2af8a5cfb333d778f5ca7660db50bad51ba66ab88271ae1a4d8ca46622c28aa |
| BOOTSTRAP.md / setup routing and selected-action boundaries | System 1.3.1; coordinator receipt and onboarding checks | 2026-10-12; SHA256 01719d542fd157272cc6c5b1a9db24b06862c5330d492848413a7d4ea00451ed |
| templates/coordinator/AGENTS.md / setup routing and selected-action boundaries | System 1.3.1; coordinator receipt and onboarding checks | 2026-10-12; SHA256 6051a35c05a8d63bbbf4faaa91481ac276844b2bbb44d47b7647663d99da682e |
| scripts/check-onboarding.ps1 / setup routing and selected-action boundaries | System 1.3.1; coordinator receipt and onboarding checks | 2026-10-12; SHA256 a6dc4effd37005c118cc956f5aa0b1b7be0142d6f9436b0b0a8dd628abf49c2e |
| COORDINATOR.md / installer-to-coordinator handover | Source 1.3.1, private prompt and required inputs; check-onboarding | 2026-10-12; SHA256 b4a652dd8ed9fead44d635685aa3826833b72aebd1ec081b922b7a1a772764a9 |
| docs/install-system.md / installer-to-coordinator handover | Source 1.3.1, private prompt and required inputs; check-onboarding | 2026-10-12; SHA256 cace1c272f4e89a9b56fcaac3b83addcc8b6ca30d8d748fbf1ff9201e8aa43af |
| templates/coordinator/STARTUP.md / installer-to-coordinator handover | Source 1.3.1, private prompt and required inputs; check-onboarding | 2026-10-12; SHA256 c06cf1168a29abd7e6660d0b44d81e7aa49a8956c303df53784ea43ec77d9de2 |

Renames, interfaces, or runner changes invalidate affected rows. Verify only the relevant area.
The [language receipt](../releases/security-validation.json) records complete template file snapshots.
This map locates sources; it does not establish model behavior.
