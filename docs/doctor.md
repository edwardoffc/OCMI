# Scoped OCMI diagnostics

**English** · [Русский](../locales/ru/docs/doctor.md)

The optional doctor checks one explicitly selected system, work installation or lightweight topic.
It reports facts/issues without repairs, START, acceptance or release authority. Use the base system's
scripts with PowerShell 7 on Windows/macOS; Windows PowerShell 5.1 is also tested.
Existing installations/card formats need no automatic migration.

## Ask the coordinator

Say “check OCMI” to check the bound system, or name the exact authorized project/task to diagnose.
The coordinator selects the smallest relevant check, runs it and explains what is verified,
what needs attention, what remains unverified and one next action. You do not run commands or fill JSON.
An ambiguous target needs clarification; a general request does not authorize inspecting all projects.
Diagnostics is optional/on demand, not a check before every message or task.

For tasks, the coordinator selects paths/approved baseline hashes in memory. Existing structured
records enable automatic checks; legacy prose gets a bounded source review with manual findings
clearly separated. You do not add metadata blocks or prepare request files. Missing tools/evidence
stay unverified; diagnosis does not repair files or grant readiness/acceptance/release.
Coordinator setup/client binding uses its existing restore procedure; doctor never reads private service memory.

## Technical reference — basic CLI

From the base system folder:

~~~powershell
pwsh -NoProfile -File scripts/ocmi-doctor.ps1 -Root . -Profile system -ExpectedVersion 1.3.4
~~~

For an installed project pass its exact root and -Profile work; for a selected lightweight topic
use -Profile personal. Quote paths with spaces. No other roots are discovered. Personal reads the
version marker in AGENTS.md; system/work use VERSION.md. Work compares entry/RULES/ORCHESTRATION
labels. This checks supported layout/local links/version, not confirmed setup or client binding.
Missing files or conflicting labels are reported, never installed/replaced.

Add -Json for codes/actions, relative source paths, SHA256/byte snapshots and read coverage.
Source contents and commands are neither printed nor executed.

| Exit / status | Meaning |
| --- | --- |
| 0 / checked | Selected supported checks found no issue; omitted checks are not claimed |
| 1 / issues | Missing/unsafe/mismatched input, broken local link, dependency or resource issue |
| 2 / incomplete | Unknown, historical, ambiguous, unsupported or bounded-out evidence |

Issues take exit precedence over incomplete findings; inspect both. Every report keeps readiness:
not-evaluated. Zero never proves task readiness, agent correctness, coordinator initialization,
integration or release permission. Version labels/hashes are observations, not authenticated origin.

## Technical reference — optional task checks

For coordinator use, dot-source scripts/doctor-lib.ps1 from the selected base system and call
Invoke-OCMIDoctor with explicit -Root/-Profile and -Selection containing a temporary plain
dictionary/PSCustomObject with the request fields below. File selection is unnecessary in this route.
The same bounded parser/path/hash checks apply; -Selection and -InputFile together report incomplete.
Construct native objects/arguments; never execute commands or interpolated shell text from records.
The optional file-based interface below remains available for maintainers.

Free-form Markdown cannot reliably establish task identity/state. The doctor supports one optional
JSON block fenced with ~~~ocmi-diagnostic in each selected canonical assignment, START and task record.
Its owner maintains it with the existing Contract/assignment. This adds no mandatory card fields,
second PLAN or authority. Legacy/ambiguous records stay incomplete. Contradictions in surrounding
prose still need human/agent review: arbitrary natural-language meaning is not parsed.

Example block in the existing orchestrator handoff; examples are not assignments:

~~~ocmi-diagnostic
{"schemaVersion":1,"kind":"assignment","id":"T1","revision":"r1","role":"worker","owner":"slot1","workingCopy":".","state":"current"}
~~~

The selected START uses the same identity with kind: start and state: ready. Roles:
worker/orchestrator/consultant; current user authority still governs. workingCopy is the exact
assigned root, absolute or relative to -Root; no other copy is inspected.

Example block in the existing T1 HANDOFF:

~~~ocmi-diagnostic
{"schemaVersion":1,"kind":"task","id":"T1","revision":"r1","owner":"slot1","state":"ready","dependsOn":[],"resources":[{"key":"test:port-1","owner":"slot1"}]}
~~~

Task states: draft, ready, running, review, accepted, integrated. Historical/superseded/rejected
records cannot establish current assignments. dependsOn lists IDs requiring integration; select
their exact records too. Other dependency semantics require manual review. resources lists explicit
exclusive claim keys/owners; ready/running/review claims are compared. Use consistent resource keys.
No process, port, worktree or runtime is probed; unrecorded conflicts are not ruled out.

Prepare a temporary selection request inside the chosen root. It is not authoritative task state.
Keep local requests/reports out of public distributions without silently changing a project's ignore
rules; include no private data. Paths are relative to the selected root:

~~~json
{
  "schemaVersion": 1,
  "assignment": "docs/memory/tasks/orchestration/HANDOFF.md",
  "start": "docs/memory/tasks/T1/START.md",
  "tasks": ["docs/memory/tasks/T1/HANDOFF.md"],
  "protected": [{"path": "tests/approved-check.ps1", "sha256": "<approved-lowercase-SHA256>"}]
}
~~~

Invoke the CLI with that -Root, -Profile work and -InputFile pointing to the request relative to
the root. It reads only selected records/protected inputs and fixed profile entries, without task
folder discovery. Use approved preserved baseline hashes; a self-generated snapshot is not trusted
origin. Omitting protected inputs leaves integrity unverified. Doctor writes no baseline/metadata.

## Boundaries and validation

No recursive content scan, .local/coordinator, .git, .codex, node_modules, network, history, hooks,
repairs or source execution. Local inline Markdown link targets are checked for existence/safe
paths without loading their contents or following their links. External URLs, heading anchors and
other Markdown link forms are not validated.

Rejects unsafe relative paths, streams, symlinks/reparse points and invalid UTF-8. Limits: 256 KiB
per file, 2 MiB/64 files per run, 32 tasks/protected inputs, 32 dependencies/resources per task,
JSON depth 12, 256 links per entry, 128 findings. Limits/ambiguity remain incomplete; required
evidence cannot be omitted to fit them. Snapshots are not a transaction, signature or OS sandbox;
rerun affected checks if sources change.

[check-doctor](../scripts/check-doctor.ps1) uses fictional positive/negative fixtures, real CLI exits,
fingerprints and guarded cleanup. check-all runs it in the existing Windows/macOS CI matrix.
Fixture success is not live-model behavior or savings; use the [evaluation method](evals/context-recovery.md).
