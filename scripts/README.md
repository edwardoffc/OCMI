# Validation

[English](README.md) · [Русский](../locales/ru/scripts/README.md)

Run from the repository root:

~~~powershell
pwsh -NoProfile -File scripts/check-all.ps1
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

check-onboarding checks the setup/development entry route, installer/coordinator handover and access
clauses in both languages, negative text mutations, unsafe/missing inputs, coordinator
versions/licenses and exact source/ZIP bytes. This checks instructions, not agent execution.
check-release also validates the coordinator receipt; -Version 1.3.2 selects the current receipt;
-Version 1.3.0 or 1.3.1 with -ArchivesOnly checks historical coordinator distributions.
Startup validation requires six fields, including the expected client project/binding.

check-release compares each language distribution with its source, receipt, exact ZIP inventory,
sizes, and SHA256; it also checks immutable historical archives.
For historical receipts use -Version 1.2 -ArchivesOnly or -Version 1.2.1 -ArchivesOnly.
ArchivesOnly compares ZIPs to the selected receipt without requiring historical sources in templates/.

Source checks require Git, PowerShell 7 and .NET ZIP support on Windows/macOS. Windows PowerShell 5.1 remains tested for compatibility. The local-boundary check additionally
requires Git; it checks ignore/export behavior, the index, reachable history and public ZIP paths
without reading private .local contents. No production build, GitHub access or installed AI client is needed.
See [maintenance](../docs/maintaining.md) for release procedure.

check-all runs the complete source suite and whitespace check. check-platform exercises native separators,
Unix case boundaries, spaces/Unicode, symlink rejection, bounded temporary cleanup and actual copied
documentation/localization checks. check-boundaries verifies ignore/export protections and rejects
six synthetic index/history/ZIP mutations. Fixtures contain no user data and are removed after guarded cleanup.
Individual checks accept -Root; no script installs PowerShell or changes client settings.
[Windows/macOS instructions](../docs/platforms.md).

## Security checks

Run scripts/check-security.ps1 -GitleaksPath <verified-native-gitleaks> before publication.
The script scans tracked files and reachable HEAD history, explicitly extracts every ZIP, rejects unsafe
archive paths/symlinks and tracked backup/key paths, and flags home identifiers or non-example email addresses.
Gitleaks uses built-in rules without repository allowlists and redacts output. Review personal details manually too.
CI downloads native Windows x64/macOS ARM64/macOS x64 Gitleaks 8.30.1 binaries from the official release, verifies pinned SHA256 and runner architecture, and runs documentation/privacy matrices. Stable Documentation and Secrets and privacy aggregate checks require every platform to pass.
The checkout action is pinned by commit, credentials are not persisted, tokens are read-only, and PRs receive no secrets.
Scan limits fail closed. Temporary extracted copies are removed only from the invocation-created temp directory.

In 1.3.2, default and -Version 1.3.2 verify four current component editions plus coordinator packages.
The old security-validation.json is historical: use -SecurityBaseline -ArchivesOnly.
Both INSTALL entry gates and neutral routing are covered by bilingual check-onboarding guards.
