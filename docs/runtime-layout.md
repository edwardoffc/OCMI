# Runtime layout foundation

OCMI supports two explicitly selected runtime roots: a project root and an
installed-home root. This foundation selects paths; it does not establish
trust, package provenance, readiness, or task completion.

Direct `ocmi request` JSON supplies `projectRoot` and `rootKind`. MCP and hook
startup use `--project-root` and may additionally use
`--root-kind installation`. `layout.inspect` is read-only and reports the
selected root kind and relative paths. It does not initialize a database,
create directories, provision data, or grant a role.

An installed home must already contain a plain `.ocmi-home.json` copied from
the [neutral runtime declaration](../runtime/internal/installation-layout.json).
The declaration contains only these fixed relative paths:

| Name | Relative path |
| --- | --- |
| system | `system` |
| native binaries | `system/bin` |
| portable sync boundary | `OCMI_Sync` |
| coordinator data | `OCMI_Sync/coordinator` |
| optional personal data | `OCMI_Sync/personal` |
| recovery data | `OCMI_Sync/recovery` |
| machine-local data | `local` |
| state | `local/state` |
| store | `local/state/state.sqlite` |
| bindings | `local/bindings` |
| cache | `local/cache` |
| runs | `local/runs` |

The optional personal directory is never created by inspection or
initialization. Inspection and initialization do not scan for, migrate, or
copy user data. Installation/update tooling and recovery migration are later
separately qualified work; this layout foundation is not a finished installer.

Project mode remains compatible with its existing project-local state. A
project request at a declared installed home is refused rather than creating a
second ledger. An installed-home request refuses missing, malformed, linked,
escaped, or conflicting declarations and does not fall back to project mode.
In this slice, whole-home `handover.*`, `state.upgrade`, `checkpoint.*`, and
`environment.*` are unsupported.
