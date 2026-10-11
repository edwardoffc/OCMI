# OCMI development handoff

**OCMI 1.2.2, rules revision 2026-10-11**

Updated: 2026-10-12. [PLAN](../../PLAN.md) · [STATE](../../STATE.md).

## Current: OCMI-ONBOARD-013 / system 1.3.0

[Source acceptance](../OCMI-ONBOARD-013/HANDOFF.md): optional orientation-first setup,
root setup/development routing, private coordinator profile, complete en/ru editions.
Downloaded-checkout and GitHub-URL routes offer choices; no unselected configuration.
Work 1.2.2/lightweight 1.2.1, existing template bytes and all previous distributions stay unchanged.
Checks: docs/links/schema/checksums, 67 workflow checks, 47 translation pairs,
67 onboarding checks, six rejected fixture mutations, exact new ZIP inventories and diff.
Privacy/secret scan passed for 166 candidate files, 185 ZIP entries and reachable history.
Behavioral scenarios E14-E21 are not run; token savings remain unmeasured.
During preparation, no user store, project, folder binding, global setting or remote state was changed.
cleanup: done for preparation; releases permanent, ignored local evidence retained until
delivery/recovery verification. The owner separately authorized publication on 2026-10-12:
[PR #2](https://github.com/edwardoffc/OCMI/pull/2) requires Documentation and Secrets and privacy.
Final integration/delivery is verified against public PR/main and the ignored local receipt.
No subsequent source task is assigned; publication does not migrate installed projects.

Earlier entries below are historical records, not current setup instructions.

## OCMI-I18N-009: language edition

Source acceptance: accepted. English is canonical throughout guides, templates, scripts,
and development memory. Russian equivalents are in locales/ru; README.ru.md is their entry point.
Both complete profiles preserve stable paths, protocol fields, role boundaries, and cleanup rules.
Work remains 1.2.1; lightweight remains 1.2. No installed project was migrated.

Validation: 91 Markdown files, 377 local links, three card schemas, nine archive checksums;
65 en/ru workflow checks; 37 reviewed translation pairs; source/receipt/ZIP agreement for four
language editions; five original archives and historical receipts preserved.
Four negative localization cases rejected source drift, translation drift, unsafe mapping,
and a missing translation. Four extracted profiles resolved 110 internal links independently.
git diff --check passed. These checks do not establish model behavior or token savings.

The requested initial commit subject is Initial OCMI release.
Publication status is verified from origin/main and the ignored local delivery receipt;
an embedded commit hash would become stale. Never push backup refs with --mirror.
Further publication or installed-project changes require their own applicable authorization.

cleanup: done for source preparation. Release artifacts are permanent.
Ignored backups and validation evidence are retained by the maintainer for recovery;
review their retention after remote delivery is verified. No temporary worktree or model process
was started. Transport/ref cleanup is recorded in the local delivery receipt.
Next source task is not assigned; do not start evaluation automatically.

## Retained evidence

[1.2.1 acceptance](../OCMI-CLOSE-005/HANDOFF.md) and
[report](../../../releases/1.2.1.md) cover resource completion and separate cleanup.
[1.2 acceptance](../OCMI-CORE-004/HANDOFF.md) and
[report](../../../releases/1.2.md) cover role/recovery changes.
[ECC analysis](../../../research/2026-10-11-ecc-review.md) is historical rationale.

Existing installation migration was completed separately with local rules and WIP preserved.
Those projects' reports are outside this repository.
OCMI-EVAL-001 remains open: model behavior and costs have not been measured.

## Security review: OCMI-SEC-010

2026-10-11: audited public commit 8053da0 (125 files, nine ZIPs/121 entries) and GitHub settings.
No exposed secrets or private context found in privacy review and three Gitleaks scans.
Preventive gaps: disabled repository secret scanning/push protection, incomplete log sanitization
rules, unprotected main, and no private vulnerability-reporting channel. Remediation follows in OCMI-SEC-011.
Detailed evidence is local and ignored: .codex/security-audit-2026-10-11/REPORT.md.
No remote settings or published files changed. cleanup: done; audit materials retained by the
maintainer for reproducibility, with retention review when findings are resolved or superseded.

## Security remediation: OCMI-SEC-011

2026-10-11, owner authorized all four audit fixes and their publication.
Work 1.2.2/lightweight 1.2.1 add sanitized evidence in both languages; task schema remains stable.
Nine previous archives/receipts are preserved; four new distributions have a separate security receipt.
Local checks: 93 Markdown, 387 links, 38 translation pairs, 67 workflow checks, 13 ZIP checksums.
Security control passed; synthetic home identifiers, a secret inside ZIP and traversal entries were rejected.
GitHub secret scanning/push protection and private vulnerability reporting are enabled and read back.
CI uses pinned checkout/scanner, read-only tokens, no persisted credentials or secret-bearing artifacts.
Source acceptance: accepted; integration requires Documentation and Secrets and privacy checks in the PR.
Final main protection and remote integration are verified in the ignored local delivery receipt:
.codex/security-fix-2026-10-11/delivery.json. Current GitHub checks/settings are authoritative.
cleanup: done for source preparation; release artifacts are permanent. Local evidence/backups are kept
by the maintainer for recovery, retention reviewed after verified delivery; branch removed after integration.
No other project or installed memory was changed. Model evaluations remain unmeasured and unassigned.

## Local adoption: OCMI-MIG-012

2026-10-11: two owner-requested work installations upgraded to the Russian 1.2.2 rules.
Local policies, queues, assignments, task packages and WIP preserved; only explicit instruction/version
edits and local migration reports were made. Hash and reversible-text checks passed.
Active instruction links are valid; one installation retains its unchanged historical snapshot-link debt.
Project details and private evidence remain in those installations. No source-template changes,
product checks, commits, publication, deployment or new START were performed by this migration.
cleanup: done; local backup/evidence retained by each installation's orchestrator until acceptance
and verified recovery, with no automatic deletion or reassessment of old task cleanup.
