# Contributing to OCMI

[English](CONTRIBUTING.md) · [Русский](locales/ru/CONTRIBUTING.md)

Propose changes through an Issue or Pull Request with an observable problem,
reproducible example, and expected result. Use fictional data;
do not attach chat logs, secrets, or private-project documents.

English is the canonical source. Work rules live in templates/work; the lightweight profile
in templates/personal; human documentation in docs. Russian translations live in locales/ru.
Do not import local paths, permissions, model choices, or current user assignments into templates.

Keep translations consistent with the changed English source and follow the
[localization policy](docs/localization.md). Preserve file paths, IDs, Contract/Result/Acceptance,
ready/running/review/accepted/integrated, and cleanup: done/pending in every language.
Refresh manifest hashes only after reviewing translation meaning.

Run from the repository root:

~~~powershell
powershell -NoProfile -File scripts/check-docs.ps1
powershell -NoProfile -File scripts/check-workflow.ps1
powershell -NoProfile -File scripts/check-localization.ps1
powershell -NoProfile -File scripts/check-onboarding.ps1
powershell -NoProfile -File scripts/check-release.ps1
git diff --check
~~~

Checks target Windows PowerShell 5.1. Templates are ordinary Markdown.
Structural checks do not establish model behavior or token savings.
For workflow changes, describe verifiable scenarios and limitations.

Do not overwrite released ZIP files. Template changes require a distinct distribution
and receipt following the [release procedure](docs/maintaining.md).
Installed project memory does not belong in an OCMI pull request.
Repository materials use the [MIT license](LICENSE).

Run the security check with a verified scanner before publication. Report vulnerabilities privately using [SECURITY](SECURITY.md). Changes enter main through a PR with Documentation and Secrets and privacy checks; no second maintainer approval is required.
