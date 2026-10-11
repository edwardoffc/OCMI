# OCMI source map

Verified: 2026-10-11, language edition of work 1.2.2/lightweight 1.2.1.
Hashes identify the inspected file snapshot; refresh only affected entries after changes.

| File / entry | Connections | Verification source |
| --- | --- | --- |
| templates/work/INSTALL.md / Installation/migration | VERSION, role procedures, task protocol | 2026-10-11; SHA256 fbbfb24d480279cb2be6711aff347eb589805307909350d2dd52666057942535 |
| templates/work/docs/memory/ORCHESTRATION.md / Role routing | Three role procedures; check-workflow | 2026-10-11; SHA256 c30fe3814890ecd5e995eeec8778ad9393e00afc32ef512417a3c2889b11e4ba |
| templates/personal/AGENTS.md / Lightweight startup | Local INDEX, brief, handoff; check-workflow | 2026-10-11; SHA256 73fd35223e827af7fabe87ba1aae9fb7b6098256943dd395c1e6dcbb16f35da6 |
| templates/work/docs/memory/roles/coordinator.md / Acceptance/resource completion | Contract, Result, Acceptance; protected evidence | 2026-10-11; SHA256 9f02b621cd35aaa98fe57dbcec2117abb8ebdfad426c7a4f79b824d72fd0f289 |
| templates/work/docs/memory/TASK-PROTOCOL.md / Task schema and cleanup | Stable headings and separate done/pending status | 2026-10-11; SHA256 e7543bb63b668c4e46ff8149660d22b62e8e17719f5a3d2c4b4f87ef8c8564ad |
| scripts/check-docs.ps1 / Markdown/link/schema checks | English/Russian guides and templates; archive checksums | 2026-10-11; SHA256 bbbc5864c7745f1963193c3733e85a4fc47e205fa6b74a19f2a6aa3efd03bd0d |
| scripts/check-workflow.ps1 / Get-Routes / Read-Scoped / Check | localization/workflow-guards.json; both languages | 2026-10-11; SHA256 84946128cf7e632af03951ff99b0fe74115418ad81392cadf7dfeb0da2dfc29e |
| scripts/check-localization.ps1 / Scoped-File / pair inventory | localization/manifest.json; hashes and paths | 2026-10-11; SHA256 695bbbaccee8a8914db2072ec3a50ae3c6bb95db94e7e796cc33b68bb82162e8 |
| scripts/check-release.ps1 / Receipt/source/ZIP verification | docs/releases/security-validation.json; historical ArchivesOnly | 2026-10-11; SHA256 68f7fbf8c0b2ca6246bebfd7b40a25ecaa7e58f65e239dda6730a7804bc08be4 |
| scripts/check-security.ps1 / tracked inventory and extracted archives | Built-in Gitleaks rules, home/email indicators; CI pinned scanner | 2026-10-11; SHA256 71b63fc071928f9c1db4663ea9f2379565f8141f736bee09593516510d2b3301 |

Renames, interfaces, or runner changes invalidate affected rows. Verify only the relevant area.
The [language receipt](../releases/security-validation.json) records complete template file snapshots.
This map locates sources; it does not establish model behavior.
