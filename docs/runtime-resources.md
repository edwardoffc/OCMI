# OCMI runtime resources (source candidate)

This guide describes the RESOURCE-040 source candidate, qualified for the bounded local-provider
scope on Windows, macOS Intel/ARM and Linux. It is not a released runtime guarantee, an installed-project
migration, or evidence that product CI has completed. Use it only in a new synthetic root through
the project coordinator.

## Ask the coordinator, not the runtime API

Give the coordinator one selected synthetic root and a plain-language purpose. For example:

> In the selected synthetic root only, prepare a task that needs a temporary workspace and one
> ordinary supervised command. Before it runs, show what OCMI will create, what it will only use,
> who owns each item, its planned lifetime, and what will be kept or removed. Stop if any provider
> or proof is unknown.

The coordinator chooses the task, owner, worker and reviewer, checks the bound inputs, and reports
the next permitted action. You do not need to write request JSON, invent resource IDs, or treat a
chat message as ownership. Do not point this preview at an existing project or personal data.

## What the candidate accounts for

The ledger distinguishes **ownership** from **use**. A created-here object may be eligible for a
later qualified cleanup; an existing tool, cache, input, or external path is use-only and is never
made deletable just because OCMI observed it.

For a managed ordinary command, the candidate is designed to record its supervised process/group,
the bound executable and runtime dependencies, selected workspace, and permitted environment path
groups as use. It also gives that operation a project-local temporary group below
`.ocmi/local/runs/<operation-id>` and directs `TMP`, `TEMP`, and `TMPDIR` to its `tmp` child for that command.
The temporary group is owned only after its intent and native proof are recorded; it is not a
global temporary-directory setting.

The first executable provider scope is deliberately small: project-confined local files and
directories, plus ordinary supervised processes. Docker, GitHub, cloud resources, and other
providers require separately qualified adapters before they can claim verified execution. An
unknown or unsupported provider can be recorded as pending, but cannot be treated as successful,
owned, or safely removable.

## Keep, stop, and uncertainty

Before an effect, the coordinator obtains an intent and reports the proposed owner, dependencies,
holds, lifetime, and disposition. After an effect, it needs an actual provider proof. A request to
**keep** requires verified presence and a retention condition. A request to **remove** requires
created-here provenance, the exact recorded identity and known inventory, and no active use, hold,
or dependent. Changed, incomplete, unsupported, or uncertain evidence remains pending rather than
being cleaned up by guesswork.

STOP blocks new creation, write, use, and grants. It does not prove that an existing child process
has stopped. Preservation, verified owned teardown, releasing a verified stopped use, and a
qualified `resource.close` remain separate, evidence-based actions; none may resume work.

There is no host-wide watcher or always-on cleanup daemon. The supervisor observes only its own
registered process/group or Job within the qualified route. A detached descendant, an unknown
native call, or Unix supervisor death stays unknown until real absence is verified; OCMI does not
claim guaranteed Unix supervisor-death cleanup.

## Short developer reference

In resource.list, each item's pending follows inspect observation; dispositionPending and summary.pending also count unfinished managed cleanup. A present active project-owned object can therefore have pending:false and dispositionPending:true.

Use `ocmi help ACTION` or `call --root ABS --actor ACTOR --action ACTION --payload JSON`.
Mutations require explicit `--revision N`; keep the returned requestId for exact continuation.

| Action | Payload | Scope |
| --- | --- | --- |
| resource.inspect | `{"resourceId":"ID"}` | Read-only selected root |
| resource.maintain.inspect | `{"resourceId":"ID","sizeLimitBytes":1048576,"ageLimitDays":30,"maxEntries":1024,"maxBytes":16777216}` | Read-only bounded metadata for an original owned local group or protected `ocmi-state` |
| resource.list | `{"taskId":"TASK","limit":100,"cursor":0}` | Read-only; taskId optional, limit defaults/caps at 100, cursor nonnegative |
| resource.rebind | `{"resourceId":"ID","locator":"ABSOLUTE_EXISTING_PATH","ownerRef":"owner","reason":"explicit binding"}` | Owner, current revision; external use only, no deletion authority |
| tool-storage.inspect | `{"toolId":"go"}` | Read-only verified installation home; see [tool storage](runtime-tool-storage.md) |

`resource.maintain.inspect` reports bytes, entry count and oldest file mtime in whole
days (future times count as zero), plus retention indicators, holds and active uses.
All limits are positive: at most 1024 entries and a configured ceiling of at most
2^50 observed bytes per measurement; review thresholds are at most 2^50 bytes/36500
days. These metadata limits do not expand the existing 256-entry/16 MiB content
inventory and disposal proof limits. Unknown, changed, linked, unsupported or over-bound
observations return `complete:false`; they are not zero-size success. Shared/external
groups are not traversed. `ocmi-state` measures only live SQLite/WAL/SHM metadata and
stays protected. Age is not last access or a deletion TTL; thresholds never authorize
pruning, releasing uses, adoption or deletion (`mutationSupported:false`).

For an original automatic local operation workspace after a qualified crash,
the owner first releases only verified stopped uses. `resource.inspect` can then
expose a `recovery` candidate with the complete inventory and its `inventorySHA256`.
The owner reviews it, then calls `resource.reconcile` with `resourceId`, `reason`, and that exact
digest at the current revision. Core rechecks native provenance, registered
process/group absence and the unchanged snapshot; active uses, holds, replacements,
links or uncertain ownership block recovery. STOP may remain set.
Recovery retains diagnostics; it neither deletes them nor makes the task successful.
Only a separate qualified `resource.dispose` removes the reviewed workspace.
This also covers a previous no-effect disposal failure with intact original proof.
Other providers, imported/use-only objects and ambiguous creation remain unsupported
or pending; this is not generic adoption of changed directories.
`resource.close` completes a task; it does not delete resources.

A fully qualified closed external loan may report `historicalReleased:true` and
`pending:false` after its old locator changes or disappears. This requires local
use-only native proof, the exact integrated/closed task revision, an attributable
verified stopped operation, a released use, no active use anywhere, no hold or
in-flight/unresolved state, and qualified local provenance. The raw
`observedState` remains `unknown` or `absent`; original native identity, ledger,
uses and events remain unchanged. This view grants no reuse or deletion right.
A fresh use still checks the actual identity and refuses a replaced/absent binding.
Active, open, held, project-owned, unknown-provider or unqualified imported records
remain pending when their required proof is missing.

Portable data omits raw file content, absolute local locators, native identities
and process handles. Imported paths require explicit external-use rebind and
never gain deletion authority. See [runtime](runtime.md) for the request envelope.
