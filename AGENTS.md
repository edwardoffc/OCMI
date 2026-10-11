# Developing OCMI

Read docs/memory/RULES.md, ORCHESTRATION.md, your role procedure, and its inputs.
This repository contains the system and neutral templates. Templates are examples,
not assignments or the active development state.

Current work format: OCMI 1.2.1, rules revision 2026-10-11.
The orchestrator owns PLAN, criteria/acceptance checks, and acceptance.
Workers require an addressed ready START; consultants are read-only.
The current request determines scope and authority, not an old recorded assignment.
Chats, messages, models, background work, and publication need relevant authorization.

English is the canonical source language. Shared work rules live in templates/work;
the lightweight profile lives in templates/personal. Russian translations live in locales/ru.
Follow docs/localization.md and keep translation coverage/freshness checks passing.
Do not mix language variants during installation. Stable paths, schema headings, IDs, and statuses stay unchanged.
docs/memory is the single active development memory; do not create translated competing plans or handoffs.
Do not import user data, other project memory, client materials, or secrets.
Use neutral public examples; keep local backups/reports out of Git.
Access to another project requires a relevant request.
Do not overwrite old packages/releases; save new distributions under distinct names.
Template changes do not automatically update installed projects or protected packages.

Before commit: powershell -NoProfile -File scripts/check-docs.ps1 and git diff --check.
For template/localization changes also run workflow, localization, and release checks.
Do not run production/build for documentation. Preserve others' changes, WIP, and evidence.
After a substantial result update the current handoff and only changed memory facts.
