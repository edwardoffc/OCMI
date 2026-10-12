# OCMI PLAN

Updated: 2026-10-13. The only development queue.

| ID | Outcome | Status |
| --- | --- | --- |
| OCMI-REPO-001 | Repository of verified templates and releases | Completed historical setup |
| OCMI-LOC-002 / OCMI-LOC-003 | Local source organization | Completed; local paths/reports excluded from distribution |
| OCMI-ECC-001 | Evaluate applicable ECC patterns | Completed [static analysis](../research/2026-10-11-ecc-review.md); ECC not executed |
| OCMI-CORE-004 | Role routing, recovery, verified CODEMAP; 1.2 | [Accepted](tasks/OCMI-CORE-004/HANDOFF.md); structural checks and ZIP verification |
| OCMI-EVAL-001 | Behavioral recovery and full-cycle cost evaluation | [Scenarios ready](../evals/context-recovery.md); model runs and costs unmeasured |
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
| OCMI-SPECIAL-019 | Optional specialist delegation and selectively loaded skill pilot | Deferred; depends on FLOW-018; no new roles/chats/models are assigned |
| OCMI-DOCTOR-020 | Scoped installation/assignment diagnostics and protected-input integrity checks | Deferred; specify requirements and fixtures before choosing a runtime |
| OCMI-CONTEXT-021 | Role/task context assembly over Markdown and verified code-map entries | Deferred; depends on diagnostics/provenance; generated indexes remain rebuildable |
| OCMI-AUTO-022 | Optional client lifecycle adapters and verified lessons promoted into skills | Exploratory/deferred; requires measured manual workflow and client capability evidence |

## Saved development scope — stage one authorized only

The owner saved this proposal on 2026-10-13, then explicitly authorized FLOW-018 in the base system.
Only stage one is assigned: system patch 1.3.3/work 1.2.3. The remaining stages below stay deferred.
The order expands the queue items, not a second PLAN; no next-stage START or model run is assigned. The owner separately authorized 1.3.3 publication.

1. **FLOW-018:** choose a compact Contract/Result/Acceptance for reversible low-risk tasks;
   size by risk, not line count. Preserve financial/security/data-integrity gates, fixed inputs and WIP.
   Before START, select and validate sufficient existing acceptance checks; write new checks for gaps.
   Run project-appropriate cheap checks before costly CI. An unexplained successful retry does not
   establish that the original failure is fixed. Before repeated substantive rework, record cause,
   hypothesis, changed approach and expected new evidence in existing Acceptance.
   Clarify delta reading during an established continuation without weakening new-chat recovery.
2. **SPECIAL-019:** keep one orchestrator responsible for the only PLAN, approved criteria,
   protected check baseline, consolidated feedback, acceptance and integration. Pilot test-design
   and read-only review specialists; CI/release and architecture/planning specializations are optional.
   Delegated test preparation needs orchestrator validation before freezing/implementation.
   Specialists return bounded evidence; do not require duplicate full review/runs or a permanent chat
   for every function. Existing permissions remain the boundary, including separate release authority.
   First skills: search-first, test-design, security-review and OCMI-specific ci-triage.
   Follow with api-design, database-migrations, e2e-testing and error-handling as tasks justify them.
   Select skills in START; review/adapt source instructions and retain attribution; load only selected skills.
3. **DOCTOR-020 → CONTEXT-021:** diagnose current assignment/revision/copy, missing or stale links,
   incomplete scans and protected input changes first; distinguish current records from historical packets.
   Then assemble scoped role/task context and code-map references with verified provenance/freshness.
   Markdown remains authoritative; caches/indexes are reconstructable. No cross-project/private search
   by default. Choose SQLite/JSON-schema/parsers only after requirements, not as mandatory dependencies.
4. **Existing EVAL-001 throughout:** capture an available baseline and compare completed tasks through
   acceptance/integration: outcome, returns/reasons, repeated checks/reasons and observed time/cost.
   Test missing/old START, wrong copy, WIP/running processes, stale maps, access boundaries and role limits.
   Existing fixtures and evidence are reused when valid. Live model runs need their own authorization;
   neither smaller files nor more agents establish savings. Expand adoption only after useful pilot results.
5. **AUTO-022 later:** optional verified SessionStart/checkpoint adapters with exact scope,
   manual recovery and duplicate-launch protection. Saved drafts do not replace accepted memory.
   Promote a verified recurring lesson into a targeted skill through review; no default transcript
   collection, background learning or global rule promotion.

Project CI optimization remains a separate project assignment: diagnose queue/setup/check/build/deploy
time from existing logs, isolate independent jobs/resources, preserve every mandatory release gate,
and compare subsequent verified deliveries. Saving this system plan does not assign that project work.

Do not start the next task automatically. Historical archives remain immutable.
Updating these sources does not change installed project memory.
