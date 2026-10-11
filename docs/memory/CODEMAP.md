# OCMI source map

Verified: 2026-10-11, language edition of work 1.2.1/lightweight 1.2.
Hashes identify the inspected file snapshot; refresh only affected entries after changes.

| File / entry | Connections | Verification source |
| --- | --- | --- |
| templates/work/INSTALL.md / Installation/migration | VERSION, role procedures, task protocol | 2026-10-11; SHA256 cbe2cf4c3374604b20d5bfec894d8c6e38cd97ddee6e63e6869d71ce76f51509 |
| templates/work/docs/memory/ORCHESTRATION.md / Role routing | Three role procedures; check-workflow | 2026-10-11; SHA256 fad17a6d629ceb75d650119854f848a75c7065356b5bca8347775c17bc012a9e |
| templates/personal/AGENTS.md / Lightweight startup | Local INDEX, brief, handoff; check-workflow | 2026-10-11; SHA256 e65761ebdb6ce233f976bc2f1b9182afea947aafe2751c06a8645220ae82d9b3 |
| templates/work/docs/memory/roles/coordinator.md / Acceptance/resource completion | Contract, Result, Acceptance; protected evidence | 2026-10-11; SHA256 83d89eb10ca94b90d155d193cb6bb2d7b5ba9a1d27b516aa4f0db76590e2ff1c |
| templates/work/docs/memory/TASK-PROTOCOL.md / Task schema and cleanup | Stable headings and separate done/pending status | 2026-10-11; SHA256 9de4598ec4f3f09c337991a5123b19d4587d78faf38ee90da63fb1c7a37e4007 |
| scripts/check-docs.ps1 / Markdown/link/schema checks | English/Russian guides and templates; archive checksums | 2026-10-11; SHA256 a964f0222e353936d3d468b85597756f2fc806882c30972791c013a301be040e |
| scripts/check-workflow.ps1 / Get-Routes / Read-Scoped / Check | localization/workflow-guards.json; both languages | 2026-10-11; SHA256 2e211acc98a95752afd350c33767be0362a4ca6d735b3e2336e540c3830f7607 |
| scripts/check-localization.ps1 / Scoped-File / pair inventory | localization/manifest.json; hashes and paths | 2026-10-11; SHA256 970662e87b3f69259ba1577bd164d759116fe0f3e0f1dd9dd175bfb2801aa16b |
| scripts/check-release.ps1 / Receipt/source/ZIP verification | docs/releases/languages-validation.json; historical ArchivesOnly | 2026-10-11; SHA256 11d64056022f7e6afc17dd8d7a61e12ff18c5dd06251122d86b1cf29ca812156 |

Renames, interfaces, or runner changes invalidate affected rows. Verify only the relevant area.
The [language receipt](../releases/languages-validation.json) records complete template file snapshots.
This map locates sources; it does not establish model behavior.
