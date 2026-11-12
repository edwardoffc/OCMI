# OCMI runtime preview

Use `ocmi --help`, `ocmi help task.bind` and `ocmi help operation.output` for JSON guidance without opening state. `ocmi call --root ABS --actor ACTOR --action status` observes the selected root. For mutations supply `--revision N` explicitly and `--payload JSON`. Without `--request-id`, a mutating call derives a reproducible opaque ID from its root, root kind, actor, action, original revision and normalized JSON payload. Retain the original invocation: repeating it after a lost response recovers the same request ID; JSON whitespace and object-key order do not affect it. Changing the invocation is a different request. Explicit `--request-id` values remain supported; observation calls keep fresh automatic IDs. No revision refresh or retry is automatic. The existing `request` stdin envelope remains supported.

The lifecycle is init → task.bind → task.ready → run → submit → review → integrate → resource.close; each step retains existing actor, revision, input and cleanup gates. Bind command `outputPolicy: "shareable"` only after owner review. Retrieve registered terminal evidence with action `operation.output` and payload `{"operationId":"ID","limitBytes":4096}` (default 4096, maximum 65536). Private/legacy commands are withheld. The frozen task revision controls policy; request payloads cannot promote it. Shareable is an owner declaration, not a DLP boundary. Output reports task/outcome/operation identities, exit/state, capturedTruncated, returnedTruncated and invalidUTF8. A complete transcript does not mean acceptance, success, or current inputs. Status and history continue to withhold output.

For current native homes use the verified ReadyKit and `system/bin/ocmi[.exe]`; see [installation](install-system.md) and [layout](runtime-layout.md). `OCMI_Sync/coordinator`, optional `OCMI_Sync/personal`, and `OCMI_Sync/recovery` hold selected portable data; `local/` is machine-only. The archive instructions below describe the historical preview route.

For the preview `context.inspect` daily flow, explicit source selection, and incomplete/stale responses, see [derived continuation](continuation.md). This reference remains the authority for the request envelope and operational recovery boundaries.

This is a local technical preview of the OCMI runtime. It is not a released
system, an installed-project migration, or a promise of client integration.
Use it only with a new synthetic root until the runtime, its notices, and its
platform checks have been reviewed together.

## Use a reviewed preview archive

Ask the coordinator for the reviewed archive matching your operating system.
The coordinator verifies its SHA256, manifests, and notices before it is used.
Do not build the runtime merely to start a synthetic task. After that result,
unpack it into a fresh platform directory in the local OCMI system, for example
`.local/bin/<preview-id>/<platform>/`. Do not replace an existing preview
directory, a prior executable, or a project folder. A package for another
operating system is not usable evidence for this one. Windows uses a ZIP;
macOS and Linux use a `.tar.gz` whose `ocmi` file is marked executable by the
coordinator's verified install route.

For a first synthetic task, create a new empty test folder and send the
coordinator this scoped request, replacing only the path and fictional task:

> Use the OCMI runtime preview only for `<absolute synthetic root>`. Bind that
> root explicitly, show its current status and identities, then prepare one
> fictional task. Before any consequential operation, report the bound inputs,
> checks, command, and owner. Do not read or initialize any other project.

The coordinator should return the selected root, identity/binding, current
state, and the next permitted action. Treat a missing state, unknown operation,
or unavailable input as a reason to stop and preserve the evidence, not as
permission to guess or retry.

To continue after an interruption, use the same explicit root:

> Continue the selected OCMI preview root `<absolute synthetic root>`. Inspect
> status and preserve the last response. If an operation is unknown or
> incomplete, name the evidence and reconciliation required; do not re-run it.

For a different device, do not copy the SQLite database or its WAL. Keep the
source root available and ask the coordinator to prepare the selected transfer,
verify the destination binding, and report conflicts or a source-unavailable
interval. No timeout, copied binary, old chat, or transcript transfers
authority automatically.

## Operator and developer reference

For explicitly selected project tools and source-linked recovery steps, see
[environment recovery](runtime-environment.md). It complements memory diagnostics and
[checkpoints](runtime-checkpoints.md); inspection never starts an old task or installs software.

The RESOURCE-040 source candidate includes bounded resource accounting; component/native checks
do not establish complete installed-project recovery. For the
coordinator-led user path, ownership/use distinction, managed temporary groups, STOP and provider
limits, see [runtime resources](runtime-resources.md).

Give the agent one selected synthetic root, task, worker and reviewer. The agent owns request IDs,
reads `status` before each step, and reports the response and next action; the user need not compose
JSON. The bounded sequence is: `init` (owner) → `task.bind` (owner, with an absolute executable,
arguments, contract and inputs) → `task.ready` (owner) → `run` (worker) → `submit` (worker) →
`review` (different reviewer) → `integrate` (owner) → `resource.close` (owner). Every mutation
uses the response revision as `expectedRevision`; a stale revision or reused request ID with changed
content is rejected. `status` is read-only.

This preview keeps one active task and bounded prior task/revision/review/decision history. A revision
keeps its task/outcome identity and needs fresh readiness and evidence; renaming never erases a failed
outcome. Retries are not automatic: the project owner records one bounded changed approach with
selected evidence, and incomplete is never success. The agent records that judgment without asking
the user to manage JSON.

The current source candidate uses private state/portable format v4; the immutable core-r1 archive
remains v2 and is not replaced by a source update. For the candidate, `task.bind`/`task.revise`
require `completionScope: "target"|"preparation"`. Accepting preparation preserves the target
failure gate and consumes no new retry credit; only an accepted explicit target closes its episode.
Scope is retained in status/history and transfers, rather than inferred from a review reason.

Owner `state.upgrade {}` explicitly upgrades selected v1/v2 state with a consistent old-readable
backup. Legacy task scopes become `legacy_unknown`; acceptance cannot infer target completion.
Missing, corrupt, ambiguous or newer state stays untouched. Portable v3 retains the bounded ledger;
reviewed v1/v2 packages import with unknown scope into quarantine. Transfers require local rebind
and fresh evidence; assign scope explicitly in a new revision when needed. An
interrupted operation needs registered ownership and verified absence for inspection/reconcile.
Missing registration, context or absence proof remains unknown: no reason-only bypass or auto-run.

For the operator interface, each request has `schemaVersion: 1`, explicit `projectRoot`, `actor`,
`requestId`, `expectedRevision`, `action` and `payload`. A fictional bind payload includes
`taskId`, `outcomeId`, `taskRevision`, `completionScope`, `worker`, `reviewer`, relative `contract`/`inputs`, and a
`command` with absolute `executable`, `args`, relative `cwd`, timeout and output limit. Never make
worker and reviewer the same identity.

| Action | Payload |
| --- | --- |
| `init` | `{ "projectId", "deviceId", "copyId", "owner" }` |
| `task.bind` | `{ "taskId", "outcomeId", "taskRevision", "completionScope", "worker", "reviewer", "contract", "inputs", "command": { "id", "executable", "args", "cwd", "timeoutMs", "outputLimitBytes" } }` |
| `task.ready`, `run`, `submit`, `status` | `{}` |
| `stop` | `{ "enabled": true|false }` |
| `review` | `{ "accept": true|false, "reason" }` |
| `integrate` | `{ "candidate", "reason" }` |
| `resource.close` | `{ "disposition": "keep"|"remove"|"transfer", "reason" }` |
| `handover.rebind` | `{ "command": { "id", "executable", "args", "cwd", "timeoutMs", "outputLimitBytes" } }` |
| `task.revise` | Full current bind payload plus `{ "reason" }` |
| `task.history`, `state.upgrade` | `{}` (history may select a task and limit) |
| `operation.inspect` | `{ "operationId" }` |
| `operation.reconcile` | `{ "operationId", "disposition": "abandon", "reason" }` |
| `outcome.reassess`, `outcome.review`, `outcome.decide` | Bounded owner judgment for the named `outcomeId`; core validates evidence and grants |

### Selected-root transfer

Export only the contract and every required input, for example `{ "transferId": "fictional-x", "files": ["contract.md", "input.txt"] }`.
It returns a checksummed JSON package; copy that JSON only, never SQLite or WAL. Import it with
`{ "package": <exported package>, "deviceId": "fictional-device", "copyId": "fictional-copy" }`
into an explicitly selected **empty** root. A package is limited to 32 files, 1 MiB each, 4 MiB raw
content and 8 MiB as an encoded request. History is preserved but raw operation output is omitted and
marked. Destination bindings are STOP/draft: clear STOP, use `handover.rebind` with a local command,
then perform a fresh execution before submission. Ack uses `{ "transferId": "fictional-x",
"generation": <export generation> }`. Export freezes the source; cancel is refused in this preview
because an offline package cannot be revoked. Missing source means ownership remains unresolved.

The initial CLI accepts one JSON request on standard input. This is an
operator interface, not the first-task path for a user:

```powershell
Get-Content C:\path\to\runtime\examples\init-request.json -Raw |
  & C:\path\to\ocmi.exe request
```

The example is intentionally fictional. It uses an explicit root and owner;
it is not a command to initialize an existing project.

For a reviewed preview that includes the MCP adapter, start the bounded stdio
server with its explicit root and actor:

```powershell
& C:\path\to\ocmi.exe mcp --project-root C:\ocmi-preview-synthetic --actor preview-owner
```

It does not discover or initialize another project. Give an agent this start
prompt after the server is selected:

> Use the OCMI runtime preview only for `C:\ocmi-preview-synthetic`. Inspect
> the selected status and binding first. Prepare one fictional task, and show
> its inputs, checks, command, owner, and next permitted action before any
> consequential operation.

The adapter's presence does not establish hook trust, client setup, or broad
tool enforcement.

## Execution channels and source self-use

| Channel | Supported observation/control | Limit |
| --- | --- | --- |
| CLI | Explicit selected-root requests use core actor, revision, binding and lifecycle gates. | A command launched separately bypasses this channel. |
| Bound stdio MCP | Requests reach the same core with the server's selected root and actor. | Protocol tests do not establish a current client connection or authenticate a host identity. |
| Exact hook events | `SessionStart` returns context; `PreToolUse` for `mcp__ocmi__ocmi_request` applies the documented admission guard. | Other tool events return `no_coverage`; admission is not execution or acceptance. |
| Registered process children | Native `run` supervises its registered process group, bounded output, timeout and owned scratch. | Separate launches and undeclared resources need independent evidence. |
| Separate model chats | Explicit assignments and returned evidence can be reviewed. | Chat creation, model activity and external tools are not contained by the runtime. |
| Unknown tools/providers | Coverage must be established for the actual consumer. | Names and labels confer no authenticated host security. |

Source self-use may qualify explicitly authorized checks with the public checkout
as a relative command cwd and private Contracts/state outside that checkout.
Select a root containing both named inputs and cwd, preserve hidden-path guards,
and close each task's registered resources before the next task. Shared tool
storage must satisfy its binding and disjoint-root rules; inherited caches are
not proof of coverage. This route needs an actual controlled run and owner review;
it does not establish blanket interception of source edits or migrate a project.
No current client connection is implied by these supported protocol routes.

Build a preview only from a reviewed checkout. The builder creates a fresh,
uniquely named directory under `.local/development/runtime-043/distributions`;
it never updates a numbered OCMI release or an existing distribution. It needs
Go 1.26.2 already present, the pinned module cache, and a reviewed notice
manifest. It does not install Go, fetch dependencies, configure a client, or
reuse an old binary.

```powershell
pwsh -NoProfile -File scripts/build-runtime-preview.ps1 `
  -NoticesManifest C:\reviewed-notices\notices.json
```

The manifest is JSON next to the reviewed notice files. `notice` is a relative
path from the manifest directory and every digest is lowercase SHA256. Its
module list must exactly equal the non-standard modules linked into the chosen
binary; this illustrative shape is not a complete manifest:

```json
{
  "schemaVersion": 1,
  "toolchain": {
    "version": "go1.26.2",
    "notice": "go/LICENSE",
    "sha256": "<64 lowercase hexadecimal characters>"
  },
  "modules": [
    {
      "path": "modernc.org/sqlite",
      "version": "v1.61.1",
      "notice": "modules/modernc.org/sqlite/LICENSE",
      "sha256": "<64 lowercase hexadecimal characters>"
    }
  ]
}
```

The builder uses `go build -trimpath` with CGO disabled for Windows amd64,
macOS arm64/amd64, and Linux amd64. It records the module graph and never
reuses an old binary. A missing, stale, duplicate, or unreadable notice stops
packaging. Windows is packaged as ZIP; macOS and Linux are packaged as
`.tar.gz` with `ocmi` mode `0755` and all other included files mode `0644`.
Cross-compilation shows that a target was built; it does not prove native
execution on that operating system. `build-info.json` records a sorted,
relative-only SHA256 snapshot of `runtime` Go sources plus `go.mod` and
`go.sum`; the builder refuses packaging if that snapshot changes during build.

Before a real build, exercise the builder's controlled staging checks:

```powershell
pwsh -NoProfile -File scripts/build-runtime-preview.ps1 -SelfTest
```

This checks the packaging boundary with temporary fictional files. It does not
build the runtime or establish that a package is usable.

## Continuation and failure

Use `status` against the same explicit root to inspect the bounded current
state. An interrupted or unavailable operation is not success and must not be
blindly re-run. Preserve the root, the generated evidence, and the last
response; reconcile only through an explicitly supported action once that
action is available. A nonzero command, truncated output, stale input, missing
state, or unknown operation cannot satisfy a required check.

`STATE_UNAVAILABLE` or `TASK_UNBOUND` never authorizes guessing about an existing root. Only a
new, explicitly selected empty synthetic root may be initialized with `init`.

STOP prevents a new consequential operation. It does not by itself prove that
an already-running child stopped. Cleanup, review, and integration remain
separate decisions with their own evidence.

For a registered operation on the same context with native proof of absence, inspect first and the
owner may reconcile it to incomplete. Launching, unregistered, legacy or ambiguous ownership stays
unknown. There is no auto-rerun or reason-only reconciliation. Hooks are optional bounded guardrails; no claim is
made that a client installed, trusts, runs them, or enforces arbitrary shell commands. Native
qualification for new runtime code remains pending; old probe CI is not that qualification.

## Device and personal data boundaries

A preview root is bound to its declared project, device, copy, and actor
identities. Do not copy a live SQLite database or its WAL between devices.
Portable transfer, when available, must use its explicit export/import path;
partial, conflicting, replayed, or source-unavailable transfers stay
unresolved until reconciliation. A build for another OS does not make prior
command evidence valid there.

The historical preview illustrated an optional personal-file layout:

```
.local/sync/personal/
  00-inbox/  10-projects/  20-areas/  30-resources/  40-archive/  90-system/
```

Topic-file recovery requires complete local files and a scoped client binding,
not an old chat transcript or a runtime database. The paths above are historical;
use the current native locations described at the start. Choose any cloud provider and
sync policy separately. Conflicts, unavailable source files, hook trust,
sidebar/chat transfer, model behavior, and measured efficiency remain outside
this preview's guarantees.

## Update and preservation

Selected file-only recovery uses the separate [checkpoint component](runtime-checkpoints.md);
its scope and `requires_rebind` result do not establish complete project/device recovery.

Keep the previous executable and the complete selected root before trying a
new preview. Do not overwrite an existing preview directory. Provision secrets
outside the archive and separately from the runtime; ordinary status and
packages must not be used as a secret transport. No automatic migration,
client setup, cloud setup, or personal-store synchronization occurs here.
