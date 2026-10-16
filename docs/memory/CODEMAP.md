# OCMI source map

Verified: 2026-10-16 for new diagnostic/skill/context entries; unchanged workflow/setup rows retain earlier verification.
Hashes identify the inspected file snapshot; refresh only affected entries after changes.

| File / entry | Connections | Verification source |
| --- | --- | --- |
| scripts/check-all.ps1 / complete suite | Flow/doctor/skill/context guards plus source checks and whitespace; Windows/macOS CI | 2026-10-16; PS7/5.1 suites; new native CI pending delivery |
| scripts/path-utils.ps1 / scoped files and temporary guards | Native separators, Unix case, link rejection and owned cleanup | 2026-10-12; check-platform and native CI |
| scripts/check-platform.ps1 / platform fixtures | Spaces/Unicode, scope/case, links, cleanup and copied consumers | 2026-10-12; 20 Windows / 21 Unix checks |
| scripts/check-boundaries.ps1 / private-boundary mutations | Ignore/export, forced index, reachable history and root/nested ZIP paths | 2026-10-12; 8 synthetic checks on all runners |
| .github/workflows/validate.yml / native matrices and aggregates | Windows/macOS ARM64/x64; pinned Gitleaks; stable main protection | 2026-10-12; eight CI jobs in PR #7 |
| docs/platforms.md / common installation and validation | URL/download flow; native paths; PowerShell 7 maintenance | 2026-10-12; reviewed English/Russian guides |
| scripts/check-local-boundary.ps1 / private path boundary | .gitignore, .gitattributes, public index/history and ZIP paths; CI | 2026-10-12; isolated positive/negative fixtures and Git archive export |
| templates/work/INSTALL.md / Installation/migration | VERSION, role procedures, task protocol | 2026-10-16; SHA256 f14f98130fc613ed0bfb8d56b648ac8caf7b487106640575191eea29d3505c6e |
| templates/work/docs/memory/ORCHESTRATION.md / Role routing | Three role procedures; check-workflow | 2026-10-16; SHA256 d1693b554aee9929fc9bdcb8107b517b4249b8302e5c4f1a0c3c495dacc66464 |
| templates/personal/AGENTS.md / Lightweight startup | Local INDEX, brief, handoff; check-workflow | 2026-10-16; SHA256 3f06ef2ebd4e8c93c308ee3b55ee99efad7f773b503354855483be52d755118d |
| templates/work/docs/memory/roles/coordinator.md / Acceptance/resource completion | Contract, Result, Acceptance; protected evidence | 2026-10-16; SHA256 f80b1f8dfeeb922e7418c3c459012d9acb2f5026f8bca8358f8f794c919349b8 |
| templates/work/docs/memory/TASK-PROTOCOL.md / Task schema and cleanup | Stable headings and separate done/pending status | 2026-10-16; SHA256 4ad7c27f27641fe3c85a1b02efaa7f18f357333ca85223911bcfa035cdda6203 |
| scripts/check-docs.ps1 / Markdown/link/schema checks | English/Russian guides and templates; archive checksums | 2026-10-16; SHA256 e7cbcf40353c23463839c4c50725020976b82cd591043f7bd3f27b37574be359 |
| scripts/check-workflow.ps1 / Get-Routes / Read-Scoped / Check | localization/workflow-guards.json; both languages | 2026-10-16; SHA256 7f542f283db9639b36bc934c86ff7303173c0bef9a422aff4bc7d9cbdbad094b |
| scripts/check-localization.ps1 / Scoped-File / pair inventory | localization/manifest.json; hashes and paths | 2026-10-16; SHA256 9658f60643cc5116a9ea4edba8c447998ef2c16c35edd87009ddfb045fe79227 |
| scripts/check-release.ps1 / Receipt/source/ZIP verification | 1.3.4 receipt; historical SecurityBaseline/ArchivesOnly | 2026-10-16; SHA256 83b9c334200e1cfbfffd41849f6ed1af27d7a7db767dc6908a68cf509041f46e |
| scripts/check-security.ps1 / tracked inventory and extracted archives | Built-in Gitleaks rules, home/email indicators; CI pinned scanner | 2026-10-16; SHA256 ce44181a6ed0f1499c5f2208ff841115458088fb3d272dcc72502dc606ca85d0 |
| AGENTS.md / setup routing and selected-action boundaries | OCMI 1.3.4, unified numbering; coordinator receipt and onboarding checks | 2026-10-16; SHA256 f0482fa5e04e374f5547065ac51c89443ea3f928ab1c09bda471d83016b0f306 |
| BOOTSTRAP.md / setup routing and selected-action boundaries | OCMI 1.3.4, unified numbering; coordinator receipt and onboarding checks | 2026-10-16; SHA256 925c18c363aa9f1f3d355fde5674976fdc78a23dbbda21a08f70506c477e1f7a |
| templates/coordinator/AGENTS.md / setup routing and selected-action boundaries | OCMI 1.3.4, unified numbering; coordinator receipt and onboarding checks | 2026-10-16; SHA256 25022139d3d73f01dc9f3c62301b6404417d8f5ec4c89b44dec7c8b50fe08562 |
| scripts/check-onboarding.ps1 / setup routing and selected-action boundaries | OCMI 1.3.4, unified numbering; coordinator receipt and onboarding checks | 2026-10-16; SHA256 46d19158febd41616390c46cb2398dc7db3ab96e2d000e6367e6395084a14265 |
| COORDINATOR.md / installer-to-coordinator handover | OCMI 1.3.4, private prompt and required inputs; check-onboarding | 2026-10-16; SHA256 cdad757f1f8dcb371ab41f60024250524d01fa26f7d811e10861334669ea23fe |
| docs/install-system.md / installer-to-coordinator handover | OCMI 1.3.4, private prompt and required inputs; check-onboarding | 2026-10-16; SHA256 f08b1ab3a3bd2613c8f4f59a21d2437cb34dda8ec43eed835bd166fbd38ee4de |
| templates/coordinator/STARTUP.md / installer-to-coordinator handover | OCMI 1.3.4, private prompt and required inputs; check-onboarding | 2026-10-16; SHA256 1c741f5878108cf8bce02613f294600a7e096a35bb7fb8d1ac1a27312f65582d |
| README.md / both startup routes and user checklist | Bound project/core prerequisite; check-onboarding | 2026-10-16; SHA256 f62a7714c9ceeca1a7c1878168deb5b1b8fafa1f13239528f3672b013751d1e5 |
| docs/work-project-routing.md / component selection and entry gate | Global/client-neutral rule; existing installation exception; check-onboarding | 2026-10-16; SHA256 26166d3f6dc0f962440f5a11da0bc0108418335e1cb657db30b49842ed0f222c |
| templates/personal/INSTALL.md / lightweight installation entry | Same coordinator requirement; OCMI version 1.3.4; component receipt | 2026-10-16; SHA256 f10b4e9d900c496a8ea46564783846d0305c67442bb22544a5efec8234ca5db0 |
| templates/work/docs/memory/tasks/SMALL-TEMPLATE.md / compact task | Same schema and safeguarded criterion coverage | 2026-10-13; check-flow and check-docs |
| scripts/check-flow.ps1 / bilingual regression mutations | localization/flow-guards.json; compact fields/schema, baseline and whole-result failure handling | 2026-10-14; 148 instruction guards/mutations and development parity |
| docs/releases/1.3.4.json / current unified inventory | Six editions at one OCMI version; historical receipts preserved | 2026-10-16; check-release exact source/ZIP inventories |
| scripts/check-versioning.ps1 / shared release label | version-utils.ps1; unified six-profile receipt and version/header drift mutations | 2026-10-14; 85 static guards |
| scripts/ocmi-doctor.ps1 / CLI | doctor-lib.ps1; explicit root/profile, text/JSON and exits 0/1/2 | 2026-10-16; real CLI positive/negative fixtures in check-doctor |
| scripts/doctor-lib.ps1 / Invoke-OCMIDoctor | bounded reads/JSON, optional in-memory Selection, canonical blocks, graph/claims/hashes | 2026-10-16; DOCTOR-029 in-memory and existing boundary fixtures; docs/doctor.md |
| scripts/check-doctor.ps1 / synthetic consumer | check-all/native CI, target fingerprints; localization/doctor-guards.json and bilingual coordinator route | 2026-10-16; Windows PS7/5.1 validation in DOCTOR-029 Acceptance; native CI pending |
| COORDINATOR.md / Diagnostics on request | saved STARTUP already reads guide; scoped doctor + separate legacy/setup review | 2026-10-16; bilingual safeguards/omissions and restored-prompt route in check-doctor |

| scripts/skill-lib.ps1 / Get-OCMISkillSelection | metadata-only intent/need/assigned-ID candidates; no dispatch/readiness | 2026-10-16; check-skills synthetic controls |
| templates/work/skills/catalog.json / discovery | five original SKILL.md procedures, installed guide, selected START inputs | 2026-10-16; bilingual metadata/frontmatter and exact packages |
| scripts/context-lib.ps1 / Invoke-OCMIContext | explicit paths, current SHA256, AST/lexical outlines, session invalidation, required snippets | 2026-10-16; check-context synthetic freshness/budget/boundary controls |
| scripts/ocmi-context.ps1 / CLI | identical installed tools, bounded JSON, exits 0/2, no discovery/persistence | 2026-10-16; 62 context cases and real base/installed CLI consumers per Windows runtime |
| docs/context.md / context route | work guide, CODEMAP, agent-facing selection and manual fallback | 2026-10-16; bilingual guide and CONTEXT-021 Acceptance |

Renames, interfaces, or runner changes invalidate affected rows. Verify only the relevant area.
The [current component receipt](../releases/1.3.4.json) records complete template file snapshots.
This map locates sources; it does not establish model behavior.
