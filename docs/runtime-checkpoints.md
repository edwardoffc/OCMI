# Recovery checkpoints

[English](runtime-checkpoints.md) · [Русский](../locales/ru/docs/runtime-checkpoints.md)

This source candidate adds a private file-checkpoint component to the runtime preview. It is not
yet the complete fresh-device recovery workflow or an installed-project update. Git/WIP recovery,
ready package installation and client/environment restoration have separate qualification gates.

A file-only topic does not need a work-task database. Ask the coordinator to save a **specific topic**
or non-Git continuation to a selected private location. It prepares the file list and request,
explains required external data, and verifies the finished checkpoint. It must not collect other
topics, client settings, secrets or live SQLite/WAL files. Select protected storage separately for
restricted data; a reference to that storage does not grant access.

The local CLI provides three actions through `ocmi request`:

| Action | Result |
| --- | --- |
| `checkpoint.capture` | Save explicitly selected files and continuation/dependency metadata to a new package directory |
| `checkpoint.inspect` | Verify completion marker, full inventory, sizes, SHA256 and supported format |
| `checkpoint.restore` | Verify, then restore into an empty selected root without running saved instructions |

Requests use actor `main` and expectedRevision 0. These maintenance actions do not accept a task,
alter its revision or authenticate a person. The MCP adapter explicitly refuses `checkpoint.*`
until its scoped transport is qualified. Chat can prepare and invoke the local CLI; users do not
need to write JSON. This does not extend ordinary UI hooks or commands outside the runtime.

Package format 1 contains `manifest.json`, `COMPLETE` and the selected regular files under `data/`.
It records owner, purpose, retention condition, declared source/version, next action and external
dependencies. SHA256 detects changed bytes; it is not a signature, secret scanner or proof that
declared provenance is correct. Keep the entire closed directory, not only its manifest/checksums.
Use an independently recoverable backup; cloud synchronization alone is not backup.

The coordinator should prepare and inspect checkpoints in selected private local folders outside
synchronization. Private operation/stage groups are created beside the selected package or restore
destination, so choose those locations accordingly; the runtime does not detect every cloud/sync folder.
Transfer or synchronize only the closed, inspected portable package, never operation/stage groups or
private identity journals.

Scope is explicit: up to 512 files, 8 MiB per file and 32 MiB total. Quiesce the selected source
before capture. Concurrent-writer and power-loss durability guarantees are outside this component.
Git roots are refused rather than represented as complete project backups. Links, unsafe names,
unsupported/corrupt/partial material, changed selected files and occupied destinations must fail.
Existing packages and foreign data are preserved; owned temporary staging has a bounded lifecycle.

To continue on a different device, make the complete checkpoint available locally, ask the coordinator
to inspect it and restore the selected scope into an empty folder. Then confirm client folder/topic
binding and supply missing tools, data or permissions. Restore reports `requires_rebind` and
`readyToResume: false`: reading a saved next action never launches it. No old process identity,
START or acceptance becomes current authority. Runtime task state still uses its own consistent
handover workflow; this file component neither copies nor imports the work database.

## Git projects and uncommitted work

The Git/WIP extension is under source qualification; it is not yet an installed recovery feature.
Ask the project orchestrator to save the selected repository, its current branch and other explicitly
selected local branches, all tracked worktree changes and the necessary untracked/ignored files.
The `checkpoint.git.capture`, `.inspect` and `.restore` actions use a separate closed format 2;
the selected-file format above is unchanged. A GitHub clone alone cannot recover unpushed commits or WIP.

The package keeps the raw staged index separately from actual working bytes, so staged and unstaged
versions of one file stay distinct. It does not copy Git configuration, remotes, hooks, credentials,
live work databases or native process authority. The first slice supports ordinary attached SHA1
repositories; unfinished merges and unsupported repository/index features must refuse visibly.
Choose authorized private storage for the selected history; integrity checks are not a secret scanner.

The bounded first slice accepts up to 4096 paths/blobs, 32 MiB per raw blob, 256 MiB of unique raw
blobs, 512 MiB of history and 32 selected local branches. It refuses unsupported index/repository
states, links, unsafe portable names and credential/database payload names; neutral `.env.example`,
`.env.sample`, `.env.template` and ordinary `.codex` text remain inert source. Tags, stashes and
unselected branches are not selected by a request for the current branch. Identify any separately
needed history/data before preparing a reset. Each helper is bounded by 30 seconds and shares the
operation's 180-second budget, followed by the existing process teardown allowance.

For both formats, private operation receipts record original group/parent identities before payload
writes or helper launches. Exact issued journal/input names and declared producer layouts are checked
with held identities and bounded inventory. Capture publishes without replacement; restore reserves
the original existing-empty destination without replacement and permits rollback only from its verified
original reservation into an absent destination. Foreign or uncertain objects are preserved. Pending
results include `CHECKPOINT_UNRESOLVED` with `ownedStage` and a next inspection step, or
`CHECKPOINT_CLEANUP_REQUIRED` with `primaryCode`. An old PID is never permission to kill a process.
Capture or restore may already have succeeded: inspect the destination and retained
operation before retrying, and remove only verified owned resources within the granted scope.

Private bookkeeping bounds are 32,768 members and 2 GiB observed size per group, at most 32 separators
and 4096 UTF-8 bytes per relative directory path, and 8192 journal phases. These bounds do not replace
package limits or grant deletion authority. Receipts support checked cooperative operations, not a new
recovery API, host watcher, atomic Unix identity compare-and-swap, hostile same-user isolation or
power-loss guarantee.

Restoration must use an empty selected folder, preserve existing data and require fresh client/project
binding before work. Saved instructions never run automatically. Full Git/device recovery readiness
requires the independent consumer and native checks; source descriptions or successful primitive
probes do not establish it. Environment/data restoration and the fresh-device pilot remain separate.

[Preview and limits](runtime.md) · [Resource lifecycle](runtime-resources.md)
