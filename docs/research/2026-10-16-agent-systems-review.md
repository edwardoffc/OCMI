# Agent systems review and development priorities

**English** · [Русский](../../locales/ru/docs/research/2026-10-16-agent-systems-review.md)

Date: 2026-10-16. Baseline: OCMI 1.3.3, workflow revision 2026-10-14.
This is research and roadmap rationale, not installed instructions. The only queue is
[PLAN](../memory/PLAN.md); [task evidence](../memory/tasks/OCMI-RESEARCH-026/HANDOFF.md).

## Scope and confidence

Reviewed nine active systems through their official repositories: README, selected workflow/skill
documents, and relevant implementation sections. Aider's repository-map tests were inspected,
not executed. No external system, installer, hook, skill or model evaluation was run.
This is a targeted comparison, not a complete code/security audit or a measured performance benchmark.
Source dates/commits identify snapshots; a README claim is not proof of effectiveness or client support.

| Source snapshot | Inspected area | License note |
| --- | --- | --- |
| [ECC, 76aaa77](https://github.com/affaan-m/ECC/blob/76aaa77df6adc5b889e689007bf99b9a4f74af34/README.md) | Search-first, iterative retrieval, evals, learning, memory-vault design, install manifests/doctor | MIT |
| [OpenSpec, f80d860](https://github.com/Fission-AI/OpenSpec/blob/f80d860fc510ba4f51180c8ddde35b91b32de657/README.md) | Brownfield changes, specification deltas, review/archive | MIT |
| [Spec Kit, f6e72bc](https://github.com/github/spec-kit/blob/f6e72bc8523de8ec8368129cbdea78d45f27cef8/README.md) | Artifact provenance, manifest-aware upgrades, workflow overlay merge code | MIT |
| [Superpowers, bb92a77](https://github.com/obra/superpowers/blob/bb92a77741419a4ab5f06e711a283343f1ada0c3/README.md) | Systematic debugging and bounded task review | MIT |
| [GSD Core, 43850b9](https://github.com/open-gsd/gsd-core/blob/43850b922007c39a96e13fb63ec6a6fae7c41264/README.md) | Context engineering, skill routing, verdict ownership, optional review lanes | MIT |
| [BMAD, 8f0d50e](https://github.com/bmad-code-org/BMAD-METHOD/blob/8f0d50e1817fdd8fcf05a20c524218222d13cd8d/README.md) | Right-sized planning and verified existing-project context | README states MIT; main describes v7 preview, not a verified stable v7 |
| [Serena, d9e4c79](https://github.com/oraios/serena/blob/d9e4c796fae0207ba6848d1df4b331122a5567b5/README.md) | Project activation, symbol navigation, memory-manager code | SolidLSP component MIT; Serena application GPL-3.0-or-later; check exact component before reuse |
| [Aider, 5dc9490](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/README.md) | Repository-map docs, ranking/cache implementation and map tests | Apache-2.0 |
| [Beads, b911ed2](https://github.com/gastownhall/beads/blob/b911ed28cd47da47e167484f32db586e7ad8a279/README.md) | Dependency readiness, ownership claims, doctor and asynchronous gates | MIT |

The [old GSD repository](https://github.com/gsd-build/get-shit-done/blob/bdcaab2c752d9a33a1a1ca9acf3a3c81fb991815/README.md)
is archived and redirects to GSD Core. Beads' previous owner URL redirects to gastownhall/beads.
No third-party code or instruction text is incorporated by this research. Any implementation reuse
requires checking the exact files, notices, dependency licenses and chosen distribution model.

## Findings and OCMI adaptations

| Pattern and evidence | OCMI already has | Useful next step / limit |
| --- | --- | --- |
| ECC [search-first](https://github.com/affaan-m/ECC/blob/76aaa77df6adc5b889e689007bf99b9a4f74af34/skills/search-first/SKILL.md), [install manifests](https://github.com/affaan-m/ECC/blob/76aaa77df6adc5b889e689007bf99b9a4f74af34/scripts/lib/install-manifests.js) | Targeted retrieval, selected-role loading, setup checks | Small reviewed skill catalog and scoped doctor; do not copy automatic researcher dispatch or install the entire catalog |
| ECC [memory-vault design](https://github.com/affaan-m/ECC/blob/76aaa77df6adc5b889e689007bf99b9a4f74af34/docs/design/ecc-memory-vault.md) | Markdown authority, current/historical distinction, verified lessons, privacy boundaries | Retrieval results should carry source, scope, freshness and incompleteness; do not make a secondary graph authoritative or auto-promote learned rules |
| Superpowers [task reviewer](https://github.com/obra/superpowers/blob/bb92a77741419a4ab5f06e711a283343f1ada0c3/skills/subagent-driven-development/task-reviewer-prompt.md), [debugging](https://github.com/obra/superpowers/blob/bb92a77741419a4ab5f06e711a283343f1ada0c3/skills/systematic-debugging/SKILL.md) | Consolidated review, sufficient checks, changed approach from the second failed outcome cycle | Reusable bounded review/debug procedures; current reviewer checks specification and quality in one pass. Follow outside-diff dependencies for named risks; do not add mandatory duplicate reviewers or infer architectural failure from a retry count |
| GSD Core [two-stage skill routing](https://github.com/open-gsd/gsd-core/blob/43850b922007c39a96e13fb63ec6a6fae7c41264/docs/features/namespace-meta-skills-two-stage-routing.md), [one verdict owner](https://github.com/open-gsd/gsd-core/blob/43850b922007c39a96e13fb63ec6a6fae7c41264/docs/adr/5057-one-owner-per-workflow-verdict.md) | Thin orchestrator, one PLAN/acceptance owner, role-specific recovery | Load catalog metadata first, then selected procedures; derive readiness/evidence status from one owner. Do not adopt fixed context-window assumptions, automatic agent waves or a second planning tree |
| Aider [repository map](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/website/docs/repomap.md), [implementation](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/repomap.py); Serena [workflow](https://github.com/oraios/serena/blob/d9e4c796fae0207ba6848d1df4b331122a5567b5/docs/02-usage/040_workflow.md) | Verified CODEMAP and rg, but no automatic symbol service | Pilot budgeted symbol maps and definition/reference lookup. Aider ranks a dependency graph and caches map/tag data; cache validity needs its own tests. Consider tree-sitter/LSP or an external Serena adapter, with lexical fallback and exact project boundaries |
| Beads [README](https://github.com/gastownhall/beads/blob/b911ed28cd47da47e167484f32db586e7ad8a279/README.md), [gates](https://github.com/gastownhall/beads/blob/b911ed28cd47da47e167484f32db586e7ad8a279/docs/workflows/gates.md) | Explicit dependencies, single task/resource owner, integration before dependent work | Diagnose unmet prerequisites, cycles and conflicting claims from existing records. Distinguish pending external conditions from failure and use bounded waits where supported; no default Dolt/task-tracker migration or deletion-as-success gate |
| OpenSpec [editing changes](https://github.com/Fission-AI/OpenSpec/blob/f80d860fc510ba4f51180c8ddde35b91b32de657/docs/editing-changes.md) | Contract, acceptance matrix, fixed revisions, human guides/ADRs | Optional before/after requirement delta and affected-consumer map inside existing artifacts. OpenSpec permits live artifact edits; OCMI keeps explicit contract revisions. Do not add four mandatory files, a second queue or advisory-only mandatory checks |
| BMAD [planning paths](https://github.com/bmad-code-org/BMAD-METHOD/blob/8f0d50e1817fdd8fcf05a20c524218222d13cd8d/docs/plan/choose-a-planning-path.md), [project context](https://github.com/bmad-code-org/BMAD-METHOD/blob/8f0d50e1817fdd8fcf05a20c524218222d13cd8d/docs/existing-codebases/set-and-maintain-project-context.md) | Risk-based compact tasks, local overlays and validated entry instructions | Refresh only verified, missing agent guidance; preserve existing instructions. Select task procedures by intent/risk; no compulsory PRD/role ceremony for small changes |
| Spec Kit [artifact inventory](https://github.com/github/spec-kit/blob/f6e72bc8523de8ec8368129cbdea78d45f27cef8/docs/reference/artifacts.md), [upgrade guide](https://github.com/github/spec-kit/blob/f6e72bc8523de8ec8368129cbdea78d45f27cef8/docs/upgrade.md) | Immutable distributions, local policy/WIP preservation, explicit migrations | Add installed-file provenance and a preview of base/local/new differences. Spec Kit detects modified managed files; a conflict-preserving three-way OCMI updater is our proposed adaptation, not a claim about its updater |

## Revised priorities and acceptance principles

Measure first and alongside each pilot: accepted/integrated outcomes, missed requirements,
returns and causes, relevant files/bytes read, costly checks and reasons, observed elapsed time
and tokens/cost only where available. Separate waiting from active work. Reuse normal task evidence;
do not add continuous telemetry or extra model/CI runs merely to populate a dashboard.

Prioritize scoped diagnostics, then selected skills/specialists, then incremental symbol retrieval.
Add requirement-delta support only for changes that need it, and a safe adoption assistant for
installed copies. Client lifecycle automation remains last. Detailed dependencies and pilot exit
criteria belong to the existing PLAN, not this report or a new parallel backlog.

Diagnostic fixtures must distinguish missing, stale, ambiguous, incomplete, historical and valid
inputs; no claim of absence after a partial scan. A generated ready list is disposable and cannot
authorize START, acceptance, release or new access. A context budget cannot drop necessary callers,
contracts, tests or fixtures. Rename/delete, dirty working copies, symlink escapes, unsupported
languages and stale caches need explicit fallback behavior on Windows and macOS.

Each skill needs a trigger, scope, inputs/output, authority limits, evidence, fallback and source/license.
Start with search-first, causal CI/debugging, test-design and bounded review/security; domain-specific
skills follow demonstrated tasks. Keep one orchestrator with final responsibility. A specialist is a
bounded optional function, not a requirement to create another permanent chat or change models.

Not selected as defaults: whole-stack installations, always-on multi-agent swarms, mandatory
vector/Dolt databases, automatic transcript collection/global learning, cross-project retrieval,
unbounded review convergence, or hooks assumed available in every client. Instructions alone are
not a filesystem sandbox. No savings or native client compatibility are established by this review.

This change saves the revised plan only. Templates, runtime rules, installed projects, package
archives and OCMI's version remain unchanged. Implementation/publication require a subsequent request.
