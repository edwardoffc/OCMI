# Selected skills and optional specialists

OCMI 1.3.5. A skill is a reusable procedure inside the current role, not a new agent or permission.
The orchestrator reads only [catalog metadata](../../skills/catalog.json), identifies an actual gap,
and loads the relevant SKILL.md. No gap means no skill; ordinary tasks need no extra ceremony.

| Need | Skill | Useful input → output |
| --- | --- | --- |
| reuse | ocmi-search-first | Capability/consumer → existing path or reuse/extend/build recommendation |
| diagnose | ocmi-ci-triage | Failure/log/environment → causal hypothesis, targeted diagnostic and sufficient next checks |
| tests | ocmi-test-design | Criterion/coverage gap → proposed check, oracle, fixture and limits |
| review | ocmi-review | Fixed Contract/diff/evidence → one consolidated behavior/quality review |
| security | ocmi-security-review | Requested security review or concrete trust-boundary risk → scoped findings/regression gap |

The intent labels bug/change/research/review/release/question help narrow candidates; an intent or
risk keyword alone does not require every matching procedure. Relevant project safety gates still hold.
Read selected skill bodies only; do not preload every skill, unrelated source history or another role.
Record selected IDs and exact paths/source revision in existing START required inputs. Do not add
mandatory fields, a second checklist/PLAN or new Result file. Workers use only addressed selected inputs;
consultants remain read-only. Missing required inputs follow the existing readiness/revision rules.
The same selected skill does not need to be reloaded on every verified continuation.

## Specialist functions

Use the procedure in the current role by default. A specialist is optional when a concrete coverage,
expertise or context gap warrants a separate bounded contribution. No permanent chat per function.
The only orchestrator owns PLAN, criteria/protected checks, consolidated feedback, acceptance and integration.
One task retains one owner. No automatic dispatch, nested reviewers or model changes.

- **Test-design advice:** supply criterion, consumer, current checks and the gap. Return proposed
  checks/fixtures and their evidence. The orchestrator validates before freezing. A read-only adviser
  does not write or run tests. Actual delegated test writing needs a normal addressed worker START,
  permitted paths/resources and orchestrator review before its checks become protected.
- **Bounded review:** supply the fixed revision/diff, criteria and existing evidence, with a precise
  review question. Return read-only findings in the current exchange; the orchestrator records them
  in existing Acceptance, not a separate competing verdict. Review outside the diff for named risks.
- **CI or security advice:** use the corresponding skill for a named problem. Existing release and
  safety authority still applies. Architecture/planning and domain packs are later options, not defaults.

Actual delegation/chats/messages/tools require current authorization and available capability.
If unavailable, use the procedure within the current role or return a scoped recommendation; do not
ask for another model merely because the catalog names a function. Do not replace a worker's task.
Give each contribution goal, inputs/revision, read/write/command limits, result location, stop condition
and resource owner where relevant. Freeze a changed assignment/check only by explicit revision.

One consolidated first review covers requirement compliance and quality. After fixes, review the delta,
earlier findings and affected connections. Reuse valid evidence; extra tests/reviews cover actual gaps.
No mandatory duplicate full run, serial reviewer chain or convergence loop. New critical risks remain blockers.

## Use in an installed project

The user describes the task normally; the orchestrator selects procedures and explains any separate
contribution when relevant. No user-written catalog, JSON request, new chat or native skill installation
is required. An existing installation receives this pack only through an explicitly requested merge
preserving local rules, issued work and WIP. The pack is self-contained; routine work does not read a base registry.

Catalog metadata is a derived discovery aid, not authoritative task state. IDs, paths, needs and intent
labels are stable across languages; descriptions/bodies use the installed language. Optional source rationale
is linked in each skill; original OCMI wording uses MIT, with no third-party code imported.
Selection helpers, when available, read metadata only and return candidates/assigned IDs; they do not
read bodies, authenticate START or launch agents. A missing helper does not prevent direct catalog use.
Evaluate accepted outcomes, returns and available costs in existing Acceptance; no savings claim follows
from having more skills. Scripts/fixtures do not prove live-model routing or specialist quality.
