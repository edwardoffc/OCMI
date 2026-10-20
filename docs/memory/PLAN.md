# OCMI PLAN

Updated: 2026-10-21. The only development queue.

| ID | Outcome | Status |
| --- | --- | --- |
| OCMI-REPO-001 | Repository of verified templates and releases | Completed historical setup |
| OCMI-LOC-002 / OCMI-LOC-003 | Local source organization | Completed; local paths/reports excluded from distribution |
| OCMI-ECC-001 | Evaluate applicable ECC patterns | Completed [static analysis](../research/2026-10-11-ecc-review.md); ECC not executed |
| OCMI-CORE-004 | Role routing, recovery, verified CODEMAP; 1.2 | [Accepted](tasks/OCMI-CORE-004/HANDOFF.md); structural checks and ZIP verification |
| OCMI-EVAL-001 | Behavioral recovery and full-cycle cost evaluation | [Stage-one method/baseline accepted locally](tasks/OCMI-EVAL-001/HANDOFF.md); live model runs/savings unmeasured; observation continues |
| OCMI-CLOSE-005 | Resource completion and separate cleanup; 1.2.1 | [Accepted](tasks/OCMI-CLOSE-005/HANDOFF.md); cleanup: done |
| OCMI-MIG-006 | Validate adoption in existing installations | Completed locally with WIP preserved; evidence stays in those projects |
| OCMI-PUBLIC-007 / OCMI-README-008 | Public documentation, MIT, bilingual entry point | Completed preparation; superseded by the complete language edition below |
| OCMI-I18N-009 | English canonical project, maintained Russian edition, language checks and distributions | Source accepted; see [handoff](tasks/orchestration/HANDOFF.md) |
| OCMI-SEC-010 | Public snapshot privacy/security review | Completed; no current disclosure found; preventive gaps recorded in the handoff, remediation in OCMI-SEC-011 |
| OCMI-SEC-011 | Sanitized evidence, secret checks, private reporting and protected main | Source accepted after local validation; integration requires both CI checks |
| OCMI-MIG-012 | Adopt 1.2.2 in two owner-requested local work installations | Completed locally; Russian rules, local policy, queues, task packages and WIP preserved; evidence stays in each installation |
| OCMI-ONBOARD-013 | Optional orientation-first setup and coordinator profile; system 1.3.0 | [Source accepted](tasks/OCMI-ONBOARD-013/HANDOFF.md); owner-authorized delivery via [PR #2](https://github.com/edwardoffc/OCMI/pull/2); installed projects unchanged |
| OCMI-INSTALL-014 | Temporary installer, local system and separate coordinator; system 1.3.1 | [Source accepted](tasks/OCMI-INSTALL-014/HANDOFF.md); owner-authorized GitHub delivery; verification recorded separately |
| OCMI-ENTRY-015 | Uniform core entry plus component/global-routing gates; system 1.3.2 | [Source accepted](tasks/OCMI-ENTRY-015/HANDOFF.md); owner-authorized GitHub delivery, verification recorded separately; installed projects unchanged |
| OCMI-LOCAL-016 | Ignored .local/coordinator inside the system; in-place 1.3.2 correction | [Acceptance](tasks/OCMI-LOCAL-016/HANDOFF.md); source checks and owner-authorized current-release delivery; installed projects unchanged |
| OCMI-MAC-017 | Common Windows/macOS setup and native Intel/Apple Silicon validation; same 1.3.2 | [Source accepted](tasks/OCMI-MAC-017/HANDOFF.md); native CI passed; installed projects and private memory unchanged; final delivery verified separately |
| OCMI-FLOW-018 | Risk-based compact tasks, sufficient existing acceptance checks and causal rerun policy | [Source accepted](tasks/OCMI-FLOW-018/HANDOFF.md); stage one only; system 1.3.3/work 1.2.3; owner-authorized GitHub delivery; final integration/cleanup verified separately |
| OCMI-SPECIAL-019 | Task-selected skills and optional bounded specialists | [Source accepted locally](tasks/OCMI-SPECIAL-019/HANDOFF.md); five skills, PS7/5.1 checks and distinct work packs; live specialist efficacy unmeasured |
| OCMI-DOCTOR-020 | Scoped assignment/provenance, dependency and resource diagnostics | [Source accepted locally](tasks/OCMI-DOCTOR-020/HANDOFF.md); PS7/5.1 fixtures/checks pass; optional, read-only, no installed-project migration |
| OCMI-CONTEXT-021 | Incremental symbol retrieval and scoped role/task context | [Source accepted locally](tasks/OCMI-CONTEXT-021/HANDOFF.md); PS7/5.1 fresh retrieval/required-input fixtures and distinct work packs; semantic completeness/live savings unverified |
| OCMI-AUTO-022 | Optional client lifecycle adapters and verified lessons promoted into skills | Exploratory/deferred; requires measured manual workflow and client capability evidence |
| OCMI-VERSION-023 | One OCMI 1.3.3 label for all current profiles; retain historical receipts/assignments | [Source accepted locally](tasks/OCMI-VERSION-023/HANDOFF.md); selected adoption verified; cleanup: done for preparation; no other migration/publication |
| OCMI-FLOW-024 | Shared workflow baseline, whole-result failure history and actual-CI preflight | [Source accepted](tasks/OCMI-FLOW-024/HANDOFF.md); owner-authorized GitHub delivery includes unified numbering; final delivery verified separately; no project migration |
| OCMI-RESEARCH-026 | Compare ECC and other agent systems; revise this roadmap | [Accepted locally](tasks/OCMI-RESEARCH-026/HANDOFF.md); research/plan only; no implementation assigned |
| OCMI-CHANGE-027 | Optional requirement-delta and impact procedure in existing artifacts | Deferred; after initial SPECIAL-019 pilot; no second plan or mandatory PRD |
| OCMI-ADOPT-028 | Provenance-aware installed-copy upgrade preview and conflict preservation | Deferred; after DOCTOR-020; read-only preview before any write mode |
| OCMI-DOCTOR-029 | Coordinator-led diagnostics without user commands/JSON | [Source accepted locally](tasks/OCMI-DOCTOR-029/HANDOFF.md); PS7/5.1 checks pass; exact scope, in-memory selection and legacy review |
| OCMI-RELEASE-030 | Package and publish accepted base stages as unified OCMI 1.3.4 | [Release record](tasks/OCMI-RELEASE-030/HANDOFF.md); owner-authorized validation/delivery; installed-project adoption and roadmap review follow later |
| OCMI-FLOW-031 | Actionable risky-premise readiness, early diagnosis and bounded preparation | [Accepted locally](tasks/OCMI-FLOW-031/HANDOFF.md); included in RELEASE-032; historical packs/records retained |
| OCMI-RELEASE-032 | Bounded waiting, everyday topic memory and unified 1.3.5; publication and selected adoption | [Task record](tasks/OCMI-RELEASE-032/HANDOFF.md); owner-authorized implementation/validation/delivery and preserving work upgrades |
| OCMI-FLOW-033 | Bounded reassessment of orchestrator decisions and scoped development tests | [Accepted locally](tasks/OCMI-FLOW-033/HANDOFF.md); PS7/5.1, 352 flow guards, distinct work packs; owner-authorized same-version publication, no adoption or live efficacy claim |

## Revised development scope — later stages remain deferred

Owner requested comparative research and a saved revised plan on 2026-10-16.
[Research and pinned sources](../research/2026-10-16-agent-systems-review.md) cover ECC, OpenSpec,
Spec Kit, Superpowers, GSD Core, BMAD, Serena, Aider and Beads. This section replaces the saved
2026-10-13 proposal; existing task IDs/history remain. FLOW-018 and FLOW-024 are implemented;
compact cards, sufficient checks, causal reruns, delta recovery and shared/local boundaries need
application/evaluation, not another rewrite. Owner subsequently authorized stage-one measurement
method/baseline, then scoped DOCTOR-020 and coordinator-facing DOCTOR-029 implementation/checks on 2026-10-16. EVAL-001 observation
continues on ordinary tasks. Owner then authorized SPECIAL-019 and subsequently CONTEXT-021 implementation/checks in the base.
Owner now authorizes packaging/publication as OCMI 1.3.4 under RELEASE-030. Feature expansion stops;
installed-project adoption, future roadmap review and personal-profile discussion follow later.
FLOW-031 was accepted locally. On 2026-10-19 the owner authorizes RELEASE-032: bounded waiting/personal scenarios,
unified 1.3.5 publication and preserving upgrades of the selected work installations. No CHANGE/ADOPT/AUTO START or live model experiment is assigned. On 2026-10-21 the owner authorizes FLOW-033 base-only orchestration corrections/checks; no delivery or installed-project update. Order below remains priority, not compulsory ceremony.

1. **EVAL-001 — baseline and pilot gates, throughout.** Reuse normal completed-task Acceptance:
   outcome/integration, missed requirements, returns/causes, costly checks/reasons, observed read volume,
   time separated into active/waiting, and cost/tokens where available. Unknown stays unknown.
   Reuse/extend recovery scenarios for missing/stale START, wrong copy, WIP/processes, stale maps,
   incomplete scans, dependency/ownership conflicts and work/personal access boundaries. Fixtures first;
   live model runs need separate authority. Pilot adoption needs correct recovery, no lost requirements,
   preserved safety gates and useful reduction in observed overhead; file size/agent count proves none.
2. **DOCTOR-020 — first implementation candidate.** A read-only diagnostic for an explicitly selected
   installation/task: version/source, role, task/revision/copy, current START, protected inputs and links.
   Check unmet/cyclic dependencies and resource-owner conflicts using existing PLAN/HANDOFF facts.
   Distinguish missing, stale, ambiguous, incomplete and historical data; one owner per verdict.
   No disk-wide scan, private/sibling reads, auto-repair or inferred permissions. A derived ready list
   cannot issue START, accept or release. Exit: scoped positive/negative Windows/macOS fixtures,
   actionable bounded report and unchanged source/WIP. Choose runtime/schema from these requirements.
3. **SPECIAL-019 — small skill catalog, then optional functions.** Route by task intent/risk
   (bug/change/research/review/release); read metadata first, then only selected procedures.
   First: search-first; ci-triage/systematic debugging; test-design; bounded review/security.
   Each declares trigger, scope, inputs/output, authority, checks/fallback and reviewed source/license.
   Pilot test-design and read-only review specialists when a real gap justifies delegation;
   CI/release and architecture/planning functions are optional. Keep one orchestrator for PLAN,
   criteria/protected checks, consolidated feedback, acceptance and integration. Test preparation
   needs validation before freezing. No mandatory extra chat, nested reviewers or model choice.
   Exit: correct routing, bounded output and an accepted representative task without duplicate full runs.
   Add api-design, migrations, e2e and other language/domain packs only for demonstrated needs.
4. **CONTEXT-021 — useful indexing, not another memory authority.** Pilot rg/verified CODEMAP →
   symbol outline/definition/reference lookup → relevant callers/contracts/tests, within a context budget.
   Compare tree-sitter, LSP/SolidLSP or an optional external Serena adapter for chosen languages;
   inspect exact licenses before reuse. Keep lexical/manual fallback and explicit unsupported cases.
   Cache provenance includes scope, source revision/content, dirty-copy changes and tool version;
   invalidate affected entries for edits/rename/delete. Indexes/graphs remain disposable.
   Exit: relevant-location/dependency fixtures, stale-cache/rename/WIP/symlink tests on Windows/macOS,
   no omitted required evidence and observed read reduction against the baseline.
   Personal-topic retrieval may reuse scoped provenance/status concepts within its own selected root;
   do not search other projects/private stores or require a vector/SQLite service.
   [Local pilot](tasks/OCMI-CONTEXT-021/HANDOFF.md) uses PowerShell AST function outlines and explicitly
   lexical Go/JS/TS/Python/Rust candidates, current hashes and session-only cache. Semantic adapters and
   live-agent savings remain unverified; synthetic scope/context reduction does not establish either.
5. **CHANGE-027 — change-specific planning where needed.** Add an optional skill/procedure for
   before/after requirements, affected consumers and acceptance impact inside Contract or existing ADR.
   Preserve the single PLAN and explicit revisions of frozen inputs/checks. Do not introduce an
   extra task tree or mandatory PRD/spec bundle for small work. Depends on initial SPECIAL-019 pilot.
   Exit: existing-product change and small-fix examples; changed requirements are traceable while
   compact work stays compact and new scope is not smuggled into acceptance.
6. **ADOPT-028 — safer, cheaper installed-copy updates.** After DOCTOR-020 provenance, design an
   installed-file origin/base inventory and read-only old-base/local/new comparison. Preview changed
   rules, local overlays and conflicts before writing. A later explicitly assigned write mode needs
   conflict-preserving three-way merge, backup/rollback, idempotence and scope/language checks.
   Preserve tasks, local policy, WIP, evidence, private exclusions and immutable old packages;
   never silently force new policy over local rules. Exit: bilingual synthetic upgrade fixtures
   with changed/deleted/renamed/local-only files, conflicts and recovery; no real-project auto-migration.
7. **AUTO-022 — last, only after measured manual pilots.** Optional lifecycle/checkpoint and external
   wait adapters require verified client/OS capability, explicit authority, bounded waits/concurrency,
   resumable state and duplicate-launch protection. Pending external conditions are not test success
   or permission; missing support falls back to manual recovery. Saved drafts are not accepted memory.
   Promote verified recurring lessons into targeted skills through review; no default transcript
   collection, background learning, cross-project promotion or unattended extra review cycles.

Project CI optimization remains a separate project assignment: diagnose queue/setup/check/build/deploy
time from existing logs, isolate independent jobs/resources, preserve every mandatory release gate,
and compare subsequent verified deliveries. Saving this system plan does not assign that project work.

Do not install whole competing stacks or add permanent roles/databases to claim feature parity.
Keep baseline instructions short; detailed procedures load on demand. Each implementation chooses
only the affected checks and additions required by its agreed risks; safety gates remain binding.

Do not start the next task automatically. Historical archives remain immutable.
Updating these sources does not change installed project memory.
