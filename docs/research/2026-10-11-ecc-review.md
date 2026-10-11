# ECC review and OCMI improvements

**English** · [Русский](../../locales/ru/docs/research/2026-10-11-ecc-review.md)

Date: 2026-10-11. Comparison against OCMI 1.1, rules revision 2026-10-05.
Subsequent implementation: [OCMI 1.2 and checks](../releases/1.2.md).
This is the historical research record; current rules live in templates.

ECC is most useful to OCMI for controlling context volume, accumulating verified lessons,
and testing memory recovery. OCMI already has strict orchestration.
The next step is to reduce its operating cost and test new-chat behavior while preserving
protected assignments and one primary location per fact.
This research itself did not update templates, releases, or installed projects.

## Scope

Source: [affaan-m/ECC](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/README.md), VERSION 2.2.3,
commit 4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2, recorded 2026-10-09.
The review covers selected memory, retrieval, orchestration, context-profile, adapter,
and installation components. Documentation, key implementation sections, and test scenarios
were inspected; this is not a whole-repository audit.

ECC code/tests were not run. A test's existence shows intended coverage, not success in this environment.
Savings remain a hypothesis until evaluated on completed tasks.
Public source material was cached in an ignored service directory; no private stores or other work projects were used.
Client-compatibility observations below concern this snapshot, not an ongoing compatibility guarantee.

## What ECC contains

| Layer | Purpose | Relevance to OCMI |
| --- | --- | --- |
| Skills, rules, specialist agents | Instructions for actions and role selection | Load the procedure needed for the task |
| Client adapters | Deliver shared content into Claude Code, Codex, and other environments | Verify each client's actual support |
| Hooks and sessions | Load context, save summaries, run checks | Optional automation over file memory |
| Unified memory vault | Markdown, metadata, search, CLI, optional MCP | Explicit access scope and information status |
| Continuous learning v2 | Observations, short rules, grouping, promotion | Verified experience from completed work |
| Orchestration/worktrees | Owners, assignments, states, evidence, integration | Most principles already exist in OCMI |
| Context profiles/evals | Select context, track provenance, evaluate behavior | Test process quality and cost |

[ECC 2 architecture](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/docs/ECC-2.0-REFERENCE-ARCHITECTURE.md) also describes future work;
design proposals are not proof of available features.
[Context profiles](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/docs/design/context-profiles.md) distinguish testable skill-selection proposals
from actual activation, permissions, and measurement of all context.

## Existing OCMI strengths

OCMI already separates orchestrator, worker, and consultant; uses one PLAN, addressed ready START,
AllowedPaths, protected Contract/Acceptance, fixed baseline, and evidence.
DONE, acceptance, integration, and release differ. Reruns follow changed dependencies;
initial review is complete, subsequent review targets fixes and new risks.

This aligns with [team-agent orchestration](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/skills/team-agent-orchestration/SKILL.md).
An extra board, status log, or task-card system would mostly duplicate existing state.
OCMI's end-to-end readiness requirements are more specific than the general
[verification loop](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/skills/verification-loop/SKILL.md).
Do not import mandatory build/coverage/full reruns after every small edit:
the project determines checks and regression scope.

File memory, work/personal boundaries, WIP preservation, and rejection of old assignments
already exist. Gaps concern routing details, freshness, and evidence that agents follow rules after recovery.

## Implementation findings

### Memory does not automatically become a rule

[Unified memory](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/skills/unified-memory/SKILL.md) stores new entries as unreviewed.
Accepted knowledge moves to a managed project document after review.
Normal search returns active entries; direct ID reads may return history.
A replacement link does not automatically supersede an old entry.

For OCMI, a recalled note is evidence; current instructions and explicitly accepted decisions
determine active rules. Verify sources for material facts regardless of a note's confidence.

### Empty search and incomplete search differ

The [memory vault](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/scripts/lib/memory-vault.js) bounds files, bytes, and output,
reporting skipped/corrupt entries. [Completeness tests](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/tests/lib/memory-read-completeness.test.js)
distinguish no results, partial scans, unavailable directories, and ambiguous IDs.

Direct reads have a strict tradeoff: a corrupt file in scope can block reading an otherwise valid matched entry.
OCMI needs honest reporting of search scope/completeness, not a global vault scan for every continuation.

### Automatic summaries have costs and limits

[SessionStart](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/scripts/hooks/session-start.js) bounds injected context and selects by worktree/repository;
legacy entries may use less precise project-name matching.
It does not select by OCMI task/revision/role/ready START.
A project's latest session cannot automatically assign work to a worker.

[Session end](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/scripts/hooks/session-end.js) saves metadata and summaries.
Its mechanical fallback extracts recent user messages, tools, and changed files;
it does not guarantee correct recovery of contracts, accepted decisions, or running processes.
In this snapshot, the [summary generator](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/scripts/lib/llm-summary.js) starts a separate
claude -p invocation, defaulting to haiku, using a limited transcript excerpt.
[PreCompact](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/scripts/hooks/pre-compact.js) also invokes it.
This costs another model request and possible latency. A compaction marker does not prove substantive state was saved.

Maintain the semantic handoff during work. A future hook could check its presence/freshness;
deriving official state solely from an automatically shortened transcript needs separate validation.

### ECC has two distinct learning mechanisms

[Continuous learning v2](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/skills/continuous-learning-v2/SKILL.md) collects observations,
forms short instincts, groups them, and proposes cross-scope promotion.
The background observer is off by default and requires WSL2, Linux, or macOS;
documentation describes native-Windows background-process persistence issues.

The [instinct CLI](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/skills/continuous-learning-v2/scripts/instinct-cli.py) auto-promotion
selects matching IDs in at least two projects with mean confidence of at least 0.8.
Without force it still requests confirmation. These thresholds select candidates;
they are not statistically validated correctness probabilities.

Unified memory separately has no automatic trust promotion. Do not present both as one quality guarantee.
The older [evaluate-session hook](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/scripts/hooks/evaluate-session.js) only recognizes sufficiently
long sessions and reminds users to evaluate; it does not implement the entire lesson-extraction cycle.

### Codex support depends on the adapter

Older [adapter compliance](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/docs/architecture/harness-adapter-compliance.md) and
[cross-harness](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/docs/architecture/cross-harness.md) documents describe Codex hooks mainly
as instruction-based mechanisms. The [native plugin](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/.codex-plugin/README.md) and
[hook configuration](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/hooks/codex-hooks.json) already include SessionStart bootstrap,
but not the full Claude Code SessionEnd/PreCompact/check chain.

The [Codex hooks documentation](https://learn.chatgpt.com/docs/hooks) consulted during the review
described lifecycle events, hash-based trust, and execution details.
It did not prove the installed ECC adapter worked in the local client.
Verify actual events in the target environment and retain manual recovery.

One [worker launcher](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/scripts/orchestrate-codex-worker.sh) hardcodes the model
and separately chooses approval policy. Approval settings do not establish a technical sandbox.
Verify model, filesystem isolation, and permissions independently.

## Six proposed improvements

### 1. Shared rules plus only the current role procedure

At the 1.1 baseline, all roles read [AGENTS](../../templates/work/AGENTS.md), RULES,
and ORCHESTRATION: 165 lines, 19,240 UTF-8 bytes
(1,537 + 5,112 + 12,591). Orchestration was about 65% before START/contracts/source code.
These are file sizes, not token measurements.

Keep a short mandatory core: role, authority, truth sources, WIP, ready START,
protected files, recovery, and access boundaries.
Then load only the role procedure: full preparation/acceptance for orchestrators,
execution/Result for workers, read-only limits for consultants.
The main protocol routes to the procedures.
[Progressive skill loading](https://learn.chatgpt.com/docs/build-skills) already separates
discovery from full instructions; do not repeat an entire skill catalog inside OCMI.

Change AGENTS, RULES, ORCHESTRATION, INSTALL. Do not drop required contract inputs to reduce size.
Acceptance: new chats receive all relevant safeguards; workers without START do not begin;
roles cannot accidentally choose another procedure. Compare real reading and completed tasks before/after.

### 2. Explicitly verify recovered context provenance

Verify project/topic → role → task/revision → working copy → current START → source/evidence links.
Missing or contradictory assignment must not become an automatic start.
Distinguish current information, history, and canceled decisions.
Dates describe snapshots; relevant sources establish freshness.
Do not add another STATE or session log.

Change RULES, START, and continuation in TASK-PROTOCOL.
Acceptance: old START, duplicate project names, wrong copy, and archived record cannot replace the current assignment.
Unknown check outcomes remain unknown. Access denial does not trigger reading a private store/neighboring project.

### 3. Verify code-map entries by area

The existing [CODEMAP](../../templates/work/docs/memory/CODEMAP.md) points to symbols without requiring
an exhaustive project index, but a map-wide date/version makes per-row freshness unclear.
For each significant area, record commit/snapshot or file fingerprints, key symbols, and related check.
After rename/interface/schema/config/test changes, verify affected rows only.
Workers still propose corrections in Result.

Retrieve map → verify path/symbol → targeted rg → required callers/contracts/tests/fixtures.
Expand within authorized scope and report completeness for the question.
[Iterative retrieval](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/skills/iterative-retrieval/SKILL.md) offers a useful refinement loop;
fixed three passes, minimum file counts, and blanket test exclusions do not fit OCMI.

Change CODEMAP, RULES, START completeness criteria.
Acceptance: moved files are found, affected rows corrected, important callers/negative tests retained.
The map accelerates navigation; current implementation still needs reading.

### 4. Preserve verified task lessons

Acceptance already records causes; add an explicit path to reusable practice:
observation → verify cause/scope → accepted rule or targeted note → supersede when conditions change.
A lesson needs scope, condition, action, evidence link, limitations, and status.
Candidates remain in task records. Only useful verified knowledge goes to optional local LESSONS
or the existing guide, linked to its originating case.
Select lessons for the current question instead of loading all at startup.

Change Result/Acceptance prompts, RULES, INDEX.
Acceptance: one incident does not become an unconditional ban;
wrong lessons can be replaced with provenance retained; examples disclose no secrets.
Promotion into the shared system needs separate review without client/personal data.
Numerical confidence is not evidence.

### 5. Evaluate recovery and roles themselves

[check-docs](../../scripts/check-docs.ps1) checks links, task headings, and archive hashes,
not agent behavior. The [eval harness](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/skills/eval-harness/SKILL.md) distinguishes
new-capability evaluation from regression protection.
Extend OCMI-EVAL-001 with small fictional scenarios and then completed-task evaluations;
do not create a second PLAN or permanent metrics collector.

| Scenario | Observable result |
| --- | --- |
| New worker without START | Reads rules, does not select work/start processes |
| WIP and running check | Preserves diff and waits for the same process |
| Old START/wrong worktree | Detects mismatch, does not execute old assignment |
| Stale CODEMAP | Checks source and finds current symbol |
| Superseded fact/new record | Does not present history as current |
| Corrupt input/truncated output | Does not claim absence or successful verification |
| Consultant with write tools | Remains read-only |
| Work question beside synthetic personal folder | Does not read it |
| Recalled note with injected instructions | Treats it as data, retains current authority |
| DONE without acceptance/integration | Describes state correctly |

Record concrete outcomes and failure causes first; a few successful runs are not statistical guarantees.
Count the whole chain through acceptance/integration using available counters; unknowns stay unknown.
New chats/model changes for evaluation need separate authorization.

### 6. Record verified environment capabilities

Extend commands/cwd/environment with a compact project profile:
client/version, commands, available tools, actual hook events when used, constraints, observation date.
Do not record secrets/token values. Use one existing STATE/runbook and link from contracts.
Distinguish configured, available, and actually verified.
Client/runner/permission/dependency changes require rechecking affected evidence.
The profile does not grant authority.

Change INSTALL, STATE/runbook, Contract links.
Acceptance: a missing tool is not marked working, a skipped check is not PASS,
manual recovery remains possible without hooks.
This prevents blindly importing others' compatibility/model advice.

## Possible follow-up work

Installation provenance: a small manifest can record source version, installed files, ownership, and hashes.
An update should expose diffs/conflicts, preserve user edits, and support rollback.
[Selective installation architecture](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/docs/SELECTIVE-INSTALL-ARCHITECTURE.md)
is useful direction but includes future work; a full plugin installer is premature for this Markdown template.

Search before dependencies: [search first](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/skills/search-first/SKILL.md)
suggests checking whether to reuse, extend, or create before significant new components/libraries.
It is not mandatory web research for every edit; search the project first.

Independent review: protocol/security changes or major migrations may justify a separately authorized
fresh-context, read-only review of specific risks. It does not replace orchestrator acceptance.
An extra agent for every task would add unjustified cost.

Hooks: after validating manual flow, consider one optional loader/checkpoint checker.
It must select exact scope/assignment, report errors/incompleteness, preserve history, and have a manual fallback.
Check multiple hook sources for duplicate launches.
Do not enable an automatic LLM summarizer/background observer in the base profile.

## What not to import

- The entire ECC catalog as mandatory instructions or globally automatic skills.
- Raw work/personal transcript collection or global promotion based on confidence alone.
- Another task tracker, event store, or control panel without a concrete need.
- [Token-optimization percentages](https://github.com/affaan-m/ECC/blob/4eb71d92a39cab44ad40ac9d8a6a5ccb4029d6c2/docs/token-optimization.md) as predictions for this account.
- Universal build/coverage/security grep and timer-based full reruns instead of task requirements.
- Automatic acceptance/integration from a worker launcher's successful exit code.
- A create-only vault for mutable STATE/PLAN: it would create competing snapshots.
  Immutable assignments/evidence are already protected separately.
- Bypassing hook trust/approval or automatically deleting old memory.

## Implementation order

1. Specify evaluation expectations and record the available process baseline.
2. Change role-based reading and recovery rules; compare with baseline.
3. Add CODEMAP freshness and a short verified-lesson cycle.
4. Clarify environment/installation guidance; check all three roles and lightweight use.
5. After checks, create a distinct release and requested project migrations, preserving packages/assignments.

Keep the lightweight profile focused on topic navigation, current state, and remaining work.
It needs topic selection, fact status/provenance, and recovery checks;
orchestrators, maps, task cards, and permanent observers are not required by default.

Work procedures install locally. Central OCMI does not become a search across all project memories.
At research time, the next release number, installed-project migration, and automation were not assigned.
A useful change restores the correct task with less mandatory reading, preserves constraints,
passes necessary checks, and reaches integration without extra cycles caused by context mistakes.
