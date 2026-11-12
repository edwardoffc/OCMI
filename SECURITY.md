# Security policy

[Русский](locales/ru/SECURITY.md)

Report vulnerabilities privately through [GitHub's reporting form](https://github.com/edwardoffc/OCMI/security/advisories/new).
Do not put credentials, personal/client data, private logs, or exploit details in public Issues.
Use fictional or redacted examples and describe affected versions, reproduction, and impact.

## Scope and maintenance

The current work profile and lightweight profile receive fixes in new releases.
Historical archives remain immutable; they are recovery artifacts, not automatically updated installs.
Adoption in an existing project requires an explicit migration preserving local work and protected packages.

OCMI supplies instructions and local validation scripts. File rules require real client permissions,
filesystem/network isolation, and review of untrusted inputs.
Integrity checks and secret scanners do not prove the absence of every vulnerability or personal detail.

## Evidence and publication

Sanitize logs, Result/HANDOFF, fixtures, and archives before saving or sharing them.
Never persist secret values or confidential personal/client content in shared memory.
Keep necessary originals only in an explicitly authorized restricted location, with an owner and retention condition.
If a credential is exposed, revoke/rotate it first; deleting a file or rewriting Git does not revoke it.
Then assess the affected history, artifacts, and copies without reposting the secret.

Before publishing, run the [checks](scripts/README.md), including tracked files, reachable Git history,
and extracted ZIP contents. .gitignore does not inspect text or remove previously tracked content.
