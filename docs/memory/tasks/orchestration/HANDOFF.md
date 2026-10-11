# OCMI development handoff

**OCMI 1.2.1, rules revision 2026-10-11**

Updated: 2026-10-11. [PLAN](../../PLAN.md) · [STATE](../../STATE.md).

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
