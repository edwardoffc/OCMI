# Validation

[English](README.md) · [Русский](../locales/ru/scripts/README.md)

Run from the repository root:

~~~powershell
powershell -NoProfile -File scripts/check-docs.ps1
powershell -NoProfile -File scripts/check-workflow.ps1
powershell -NoProfile -File scripts/check-localization.ps1
powershell -NoProfile -File scripts/check-release.ps1
git diff --check
~~~

check-docs checks local Markdown file links, three task schemas, and archive checksums.
It includes English and Russian guides/templates. External URLs and heading anchors are not checked.

check-workflow checks role routing, scoped inputs, authority and cleanup clauses, task schema,
and negative mutations in both languages. English development procedures must match canonical templates.
These are structural checks; no model process or cleanup action is launched.
[Behavioral evaluation](../docs/evals/context-recovery.md) remains separate.

check-localization checks the maintained translation inventory, reviewed hashes, stable template
paths, and license copies. Hash agreement detects drift; human review establishes meaning.
The manifest is updated deliberately after review, never by a validation run.

check-release compares each language distribution with its source, receipt, exact ZIP inventory,
sizes, and SHA256; it also checks immutable historical archives.
For historical receipts use -Version 1.2 -ArchivesOnly or -Version 1.2.1 -ArchivesOnly.
ArchivesOnly compares ZIPs to the selected receipt without requiring historical sources in templates/.

Scripts require PowerShell 5.1 or later and .NET ZIP support. They do not need a production build,
GitHub access, or installed AI clients. See [maintenance](../docs/maintaining.md) for release procedure.
