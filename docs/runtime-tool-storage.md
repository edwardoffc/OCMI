# Tool-storage profiles for ordinary runs

[English](runtime-tool-storage.md) · [Русский](../locales/ru/docs/runtime-tool-storage.md)

This preview route lets an ordinary, already-authorized task command consume a
machine-local tool-storage profile. It applies the declared safe path settings
to that run and records the shared stores as external use before the command
starts. It is not a tool installer, a host migration command, an OS sandbox, or
a general monitor of every side effect.

## Through the coordinator

Ask the coordinator to review the existing approved profile for the selected
installation and current task, explain the requested tool storage and scratch
effect, and show the task's ordinary-run result. You do not write profile JSON
or create folders yourself. The coordinator either reuses an existing approved
profile or prepares one fresh selected layout through the native authoring
route; it never silently invents a profile or storage locations. Until one is
approved and active, it reports setup pending. A task without `toolStorage`
preserves the existing command behavior.

The coordinator may attach the exact `toolStorage` reference during `task.bind`
or an explicit `task.revise`; `task.ready` and STOP checks then govern the
ordinary run. It names an absolute selected installation home, a lowercase tool
ID, and the profile SHA-256. The runtime reads only the corresponding bounded
profile under that home's local bindings; it neither discovers tools across the
machine nor scans caches or source trees.

For `task.bind` or `task.revise`, the optional command field is exactly
`"toolStorage":{"home":"ABSOLUTE_APPROVED_INSTALLATION_HOME","toolId":"go","sha256":"APPROVED_PROFILE_SHA256"}`.
The active receipt's `profileSha256` maps to the reference's `sha256`; do not
put `profileSha256` or extra keys in this reference. Reuse an explicitly supplied
approved reference. A project actor cannot inspect installation coordinator
state to obtain it; a missing reference requires coordinator clarification,
not bypassing authority checks or repairing the profile. The default command
example remains usable without a profile. `ocmi help task.bind` and
`ocmi help task.revise` expose this object as `optionalCommandFields.toolStorage`.

## Native authoring preview

Where a matching native runtime provides this feature, a confirmed installation
coordinator can select a supported tool and ToolData root, show one concrete
plan and its effects, record the owner's existing consent reference, apply that
unchanged plan, and inspect the typed result. The selected root may already
exist and be shared, or only its final leaf may be new beneath an existing plain
parent; it cannot be an arbitrary new tree. The user supplies no JSON. Prepare
alone creates no external tool directory or usable profile; apply is the
consent-gated step that creates one fresh per-tool subtree and publishes it
only after its required verification succeeds.

An existing ToolData root may serve several tools, but an existing unknown
per-tool subtree is never adopted or overwritten. An inspect result distinguishes
`effective-paths-verified` for the fixed Go/npm procedures from
`path-variables-only` for a custom tool; custom namespace variables do not
prove that its provider accepts or uses them. A typed refusal, pending
transaction, or recovery result retains its evidence rather than inventing a
profile, rerunning a probe, or deleting created data.

For the fixed Go check, `HOME`, `APPDATA`, and `XDG_CONFIG_HOME` first point to
the owned probe scratch. Its first supervised Go command turns telemetry off
without changing user settings; a fresh fixed `go env` query then verifies that
mode, the telemetry configuration directory, and the declared paths. A typed
refusal or uncertain cleanup remains pending evidence, never a purge. This
does not prevent every detached child, provide a universal sandbox, or relocate
host data. See [Go telemetry](https://go.dev/doc/telemetry).

### Machine reference for the coordinator

Use the existing `ocmi request` envelope with `rootKind: "installation"`,
`actor: "coordinator"`, and `expectedRevision: 0`. The closed actions are
`tool-storage.prepare`, `tool-storage.apply`, `tool-storage.inspect`,
`tool-storage.cancel`, and `tool-storage.recover`; unknown or extra keys are
refused. A user still asks in plain language—the coordinator supplies the
request from the selected installation and approved inputs.

`tool-storage.prepare` has exactly `toolId`, `toolDataRoot`, `owner`,
`purpose`, `retention`, `maxBytes`, `maxAgeDays`, `executable`, `script`,
`stores`, and `scratchEnv`. Choose an absolute selected ToolData root, an owner,
purpose and retention condition; size is in bytes and age in days.

| Tool | `executable` | `script` | `stores` / `scratchEnv` |
| --- | --- | --- | --- |
| `go` | Absolute Go executable | `""` | `[]` / `[]`; fixed Go defaults |
| `npm` | Absolute native Node executable | Absolute `npm-cli.js` | `[]` / `[]`; fixed npm cache |
| Custom slug | `""` | `""` | Explicit namespace-only paths and scratch aliases |

For example, `widget` may declare `stores: [{"key":"WIDGET_CACHE","path":"cache","kind":"cache"}]`
and `scratchEnv: ["WIDGET_TMP"]`. Each store has exactly `key`, `path`, and `kind`
(`cache` or `data`); its path is relative to the new tool subtree.
`apply` and `recover` each require
exactly `toolId`, `planId`, `planSha256`, and `consentReference`; `inspect`
requires `toolId`; `cancel` requires `toolId`, `planId`, and `planSha256`.
These names are API details for the coordinator, not a request for manual JSON
authoring.

## What an opted-in run does

Before the command has effects, the runtime rechecks the selected profile,
home, ToolData root, and each declared existing store by native identity and
raw profile hash. It then records every shared store as an existing local
resource with mode `use`, external ownership, its declared owner/purpose/
retention, and release disposition. The ordinary operation keeps that use until
the supervised command has stopped; verified closure releases the use record.

Declared path-only variables are applied to the child process, not persisted
as machine configuration. Permitted profile keys are Go
`GOCACHE`, `GOMODCACHE`, `GOPATH`, and `GOBIN`, and npm
`npm_config_cache`. A different tool may use only its declared uppercase slug
namespace for path-only settings. Native authoring's fixed Go/npm procedures
verify their effective paths in a fresh supervised process; custom tools remain
path-variables-only. Runtime TMP remains operation-owned scratch:
declared Go `GOTMPDIR` or tool-namespace scratch aliases point to that exact
scratch, never to a shared store.

The profile itself declares the existing ToolData root, maximum bytes and age,
stores (cache or data), safe store environment keys, and scratch keys. These
are closed metadata for validation; a run does not create ToolData or stores,
edit the profile, enforce an age/space purge, or grant deletion authority.

## Shared use is not ownership

One machine-local profile can be referenced by tasks from two projects. Each
task gets its own use/release evidence; neither task obtains ownership of the
shared cache or data. A project cannot delete a shared store merely because it
used it. Unknown, pending, failed, or unverified closure remains visible for
review instead of being called complete.

Existing stores are use-only except for the bounded preserving relocation below.
This preview has no installation, arbitrary host mover, automatic cache pruning,
or global tool-configuration migration. Keep old data intact when
a profile is missing, malformed, replaced, unsupported, or cannot be revalidated.

## Preserving relocation preview

For a current lifecycle schema2 native-approved Go or npm profile, the confirmed
coordinator may prepare one selected fresh ToolData destination. Prepare only
shows that selection; apply requires the owner's explicit approval of the
unchanged plan. It copies only the complete bounded known stores; both source and
new copy are retained. Active, pending, corrupt, or unknown recorded ordinary
use refuses the transition. Apply proves fresh provider paths before publishing
the new generation; inspection reports selected old/new locations, known use and
the next action.

The coordinator route is `tool-storage.relocate.prepare` →
`tool-storage.relocate.apply`; `tool-storage.relocate.inspect`,
`tool-storage.relocate.recover`, and `tool-storage.relocate.rollback` remain
scoped to that transition. The first supported inventory is at most 1024 entries
and 16 MiB.

Rollback uses that same approved transition and refuses if new data diverged.
Interrupted state stays inspectable and pending; recovery closes only a proven
metadata phase and never replays an unproved child. Historical/manual schema1
and no-profile behavior is unchanged and cannot be silently upgraded. An updater
must not select an apply/rollback candidate or retained generation with active
schema2 profiles unless that generation's build-info top-level
`toolStorageProfileFormat` is `2`; an older strict profile reader rejects schema2
metadata.

## Freshness and refusal

The runtime refuses a supplied malformed profile rather than ignoring it. It
also refuses changed bytes, same-byte profile replacement, changed/missing home,
ToolData, or store identity, links/reparse points, unsafe paths, reserved or
case-colliding environment keys, and a profile that overlaps the selected
project or home. It rechecks immediately before launch, so an observed race can
leave inspectable operation evidence but cannot launch using unverified paths.

Profile variables cannot alter `PATH`, home/profile locations, common OS data
roots, dynamic-loader variables, general environment loaders, Node/Python
options, Go's persistent/default configuration, or `TMP`/`TEMP`/`TMPDIR`.
This is deliberate: a path setting is not authority to replace the host
environment or configure an arbitrary provider.

## Current boundary

The route applies an already-approved profile to an ordinary task command. The
native authoring preview above adds fresh layout/profile creation, fixed Go/npm
path verification, and only the preserving lifecycle-schema2 relocation described
above. It does not add host installation or global settings, source deletion,
automatic capacity/age cleanup, universal background/remote-effect observation,
Docker/Codex automation, host-tool installation, or global-configuration
migration command. A
compatible installation update preserves a closed active profile and receipt;
malformed or pending authoring state is refused before update effects. Those
remaining capabilities must not be inferred from a successful ordinary run.

`tool-storage.maintain.inspect` is available to a coordinator for one selected
current lifecycle-schema2 Go/npm cache store. It is bounded read-only
measurement: it reports policy thresholds, observed size/age and known use, but
does not delete or change shared external data (`mutationSupported` is false).
Measurement is limited to 1024 entries and 16 MiB; thresholds never authorize
deletion. Cleanup remains the existing [owned reproducible-cache resource
workflow](runtime-resources.md), not authority over shared stores. An unobserved
external provider remains pending.

For selected-project diagnosis before a task, see
[environment recovery](runtime-environment.md). Its inspection route remains
separate from this opt-in command-consumption preview.
