# Permanent OCMI coordinator

**English** · [Русский](locales/ru/COORDINATOR.md)

Use this guide only in the separate bound coordinator chat after native
`coordinator prepare`. Read `OCMI_Sync/coordinator/coordinator.json`,
`AGENTS.md`, `INDEX.md`, and `HANDOFF.md`, then
`local/bindings/coordinator/STARTUP.md` and `local/bindings/coordinator/coordinator.json`. These are
flat inputs; do not create the legacy eight-file profile for a new native home.

Verify the exact home, selected language, required files, and current project
association using tools or current user confirmation. If anything is missing,
malformed, conflicting, or unavailable, preserve it and report setup pending.
After verification record only current local evidence with `ocmi coordinator
confirm --home ABS --client-project NAME --evidence-source user-current|client-tool
--evidence TEXT`. It records evidence but does not prove client UI/chat identity
or copy confirmation into portable semantic memory.

Only after confirmed handover offer optional personal storage or project
integration. Do not scan unrelated roots, create chats automatically, import old
trust/DB state, or move an existing legacy setup without an explicit migration.

## Diagnostics on request

The coordinator performs the smallest read-only check for the exact selected target and scope; the
user does not run commands or fill JSON. For a native home, use `ocmi install --home ABS --inspect`
or `ocmi coordinator inspect --home ABS`; the optional legacy doctor is limited to supported selected
public payload `HOME/system` or an explicitly selected work project, never private `OCMI_Sync` or
`local` memory. A registry is not permission. Unavailable execution stays unverified; Diagnosis
grants no readiness/acceptance/release, repair or wider access. Diagnostics are optional on request,
not a routine message step.

This role requires explicit permanent-coordinator assignment or the generated prompt. A missing required input prevents confirmation; a pointer is navigation, not authority. Selected projects retain their own plans and assignments. On request, use [diagnostics](docs/doctor.md) or [code context](docs/context.md) only in the selected authorized scope; use [work INSTALL](templates/work/INSTALL.md) or [personal INSTALL](templates/personal/INSTALL.md) only after explicit selection. Do not require user JSON. Diagnosis grants neither access, repair, publication, nor integration authority.
