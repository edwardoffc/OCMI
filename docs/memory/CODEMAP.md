# OCMI source map

Verified: 2026-10-19 for FLOW-031 and 1.3.5 entries; unchanged diagnostic/skill/context/setup rows retain earlier verification.
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
| templates/work/INSTALL.md / Installation/migration | VERSION, role procedures, task protocol | 2026-10-19; SHA256 71b0b67751d5ff4d94930c5c197d613ceb29fad83cf400724bceb4112bb8da31 |
| templates/work/docs/memory/ORCHESTRATION.md / Role routing | Three role procedures; check-workflow | 2026-10-19; SHA256 c97ff0c3cc148ab624100f6c191a2785861fe5f3f34136a7f337052087201cb9 |
| templates/personal/AGENTS.md / Lightweight startup | Local INDEX, brief, handoff; check-workflow | 2026-10-19; SHA256 d7cbfce406d239bb9947f497df618950ff6a594e0d302e2a2fc9de1fff038f93 |
| templates/work/docs/memory/roles/coordinator.md / Acceptance/resource completion | Contract, Result, Acceptance; protected evidence | 2026-10-19; SHA256 f2158f9e08670d7a837d184b91acca705af97c0c0adfcecaa0e5cc83ae40103e |
| templates/work/docs/memory/TASK-PROTOCOL.md / Task schema and cleanup | Stable headings and separate done/pending status | 2026-10-19; SHA256 a718fb68998ebe86d088d4a237ca707b9b9c41498279ca1e1b957ab6a0e823f8 |
| scripts/check-docs.ps1 / Markdown/link/schema checks | English/Russian guides and templates; archive checksums | 2026-10-16; SHA256 e7cbcf40353c23463839c4c50725020976b82cd591043f7bd3f27b37574be359 |
| scripts/check-workflow.ps1 / Get-Routes / Read-Scoped / Check | localization/workflow-guards.json; both languages | 2026-10-19; SHA256 2ebf0786e8ca42d3b2ed82013b79b389d9e05ae95034f2c64187f220e500affc |
| scripts/check-localization.ps1 / Scoped-File / pair inventory | localization/manifest.json; hashes and paths | 2026-10-16; SHA256 9658f60643cc5116a9ea4edba8c447998ef2c16c35edd87009ddfb045fe79227 |
| scripts/check-release.ps1 / Receipt/source/ZIP verification | 1.3.4 receipt; historical SecurityBaseline/ArchivesOnly | 2026-10-16; SHA256 83b9c334200e1cfbfffd41849f6ed1af27d7a7db767dc6908a68cf509041f46e |
| scripts/check-security.ps1 / tracked inventory and extracted archives | Built-in Gitleaks rules, home/email indicators; CI pinned scanner | 2026-10-16; SHA256 ce44181a6ed0f1499c5f2208ff841115458088fb3d272dcc72502dc606ca85d0 |
| AGENTS.md / setup routing and selected-action boundaries | OCMI 1.3.4, unified numbering; coordinator receipt and onboarding checks | 2026-10-19; SHA256 460237c0083b69935b4f3eb29b8b1ac5d01c50df6c2585f14605222454bf8e70 |
| BOOTSTRAP.md / setup routing and selected-action boundaries | OCMI 1.3.4, unified numbering; coordinator receipt and onboarding checks | 2026-10-19; SHA256 75fd81d712f1657f42a08e9a10038ebc745269c62f5c2b0d52eda1ee735ae2ce |
| templates/coordinator/AGENTS.md / setup routing and selected-action boundaries | OCMI 1.3.4, unified numbering; coordinator receipt and onboarding checks | 2026-10-19; SHA256 048eb7965e9de77e85b995ef547f82db5831345b3937e2b282dab2505c263004 |
| scripts/check-onboarding.ps1 / setup routing and selected-action boundaries | OCMI 1.3.4, unified numbering; coordinator receipt and onboarding checks | 2026-10-16; SHA256 46d19158febd41616390c46cb2398dc7db3ab96e2d000e6367e6395084a14265 |
| COORDINATOR.md / installer-to-coordinator handover | OCMI 1.3.4, private prompt and required inputs; check-onboarding | 2026-10-19; SHA256 c68b4d177c519d79585965ab00590aa152a7158ecc08f6697476596a4bad538d |
| docs/install-system.md / installer-to-coordinator handover | OCMI 1.3.4, private prompt and required inputs; check-onboarding | 2026-10-16; SHA256 f08b1ab3a3bd2613c8f4f59a21d2437cb34dda8ec43eed835bd166fbd38ee4de |
| templates/coordinator/STARTUP.md / installer-to-coordinator handover | OCMI 1.3.4, private prompt and required inputs; check-onboarding | 2026-10-16; SHA256 1c741f5878108cf8bce02613f294600a7e096a35bb7fb8d1ac1a27312f65582d |
| README.md / both startup routes and user checklist | Bound project/core prerequisite; check-onboarding | 2026-10-19; SHA256 18a249cfe7df7120e79693db27cde8ea897cc63418685de4b3493d2e2920867e |
| docs/work-project-routing.md / component selection and entry gate | Global/client-neutral rule; existing installation exception; check-onboarding | 2026-10-16; SHA256 26166d3f6dc0f962440f5a11da0bc0108418335e1cb657db30b49842ed0f222c |
| templates/personal/INSTALL.md / lightweight installation entry | Same coordinator requirement; OCMI version 1.3.4; component receipt | 2026-10-19; SHA256 87807501bba5f7d6624628ce885dad6227f058c834b45bf8239aee2a385e3732 |
| templates/work/docs/memory/tasks/SMALL-TEMPLATE.md / compact task | Same schema and safeguarded criterion coverage | 2026-10-13; check-flow and check-docs |
| scripts/check-flow.ps1 / bilingual regression mutations | localization/flow-guards.json; risky premises, early diagnosis, bounded research/current state and existing safeguards | 2026-10-19; 257 instruction guards/omissions/weakening controls and development parity |
| docs/releases/1.3.5.json / current inventory | Six distinct editions; work 31/personal 7/coordinator 8 files per language; previous receipts/ZIPs fixed | 2026-10-19; check-release exact source/ZIP inventories |
| scripts/check-continuity.ps1 / waiting and topic regressions | localization/continuity-guards.json; bilingual clauses and synthetic copy/merge/archive/UTF-8/conflict/boundary checks | 2026-10-19; 132 checks on local PS7/5.1; no live AI/cloud claim |
| templates/work/docs/guides/waiting.md / operation continuation | RULES and role entry; saved operation/evidence/trigger and bounded observation | 2026-10-19; continuity positive/omission guards |
| templates/personal/memory/GUIDE.md / everyday topics | Topic AGENTS/INSTALL; selected scope, move/merge/archive and portable copy | 2026-10-19; continuity file/negative fixtures |
| templates/work/docs/memory/roles/worker.md / early failure diagnosis | Existing Result, authorized diagnostics, protected-input revisions | 2026-10-19; check-flow positive/omission/weakening controls |
| templates/work/skills/ocmi-test-design/SKILL.md / risky oracle control | Readiness matrix, actual engine/harness, behavioral invariant and control limits | 2026-10-19; selected-skill flow guards; no live-agent efficacy claim |
| templates/work/skills/ocmi-ci-triage/SKILL.md / pre-assertion failure | Cause discrimination before speculative algorithm changes/retries; scope and independent acceptance | 2026-10-19; selected-skill flow guards; no product correctness claim |
| scripts/check-versioning.ps1 / shared release label | version-utils.ps1; prepared FLOW-031 receipt, unified labels and drift mutations | 2026-10-19; 88 static guards |
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
The [current component receipt](../releases/1.3.5.json) records complete template file snapshots.
This map locates sources; it does not establish model behavior.
