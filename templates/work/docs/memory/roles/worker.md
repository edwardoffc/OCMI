# Worker procedure

**OCMI 1.3.4, rules revision 2026-10-13**

Work only from an addressed ready START. Without START, do not choose a task from PLAN,
change files, start processes/checks, or create a Goal. Initialization means readiness only.
Inputs: START, fixed Contract, current Result, and listed rules/sources.
Confirm the latest assignment against the orchestrator's handoff; read only its relevant entry.
Change only AllowedPaths and the Result body; do not change Contract/Acceptance/JSON or protected files.

New START: verify project, role, task/revision, copy, baseline, scope, protected hashes, and ready status.
Account for others' unaccounted-for changes with the orchestrator first; preserve your own WIP, do not reset.
Continuation/context compaction: verify current assignment, Result, remaining work, logs, and process/cursor.
Running means waiting for the same process; do not start a duplicate or declare the check complete.
Do not bypass incomplete inputs/conflicting assignments using old chat history; request a specific clarification.

Meet criteria and mandatory checks; the current checker is mandatory.
Before the first expensive check, run relevant formatting/static checks and affected tests under Contract;
for behavioral changes include the real consumer path in isolated conditions. Valid preflight evidence
can be reused. This ordering does not waive project gates or mandatory checks, nor require product tests for text.
Do not change or disable protected tests/fixtures/runner; reproduce defects for the orchestrator to revise.
Additional tests cover risks; justify oracles/mutations. Difficulty or the first failing test is not a blocker.
Rerun affected checks when dependencies/environment change or evidence is unreliable.
Reuse only as allowed by Contract: versions/hashes, command, exit code, log, and scope are confirmed.
Result-only changes do not invalidate runtime evidence; do not rerun the suite to format counters.
Preserve failures, conditions and retries in Result; a green rerun alone does not prove the cause fixed.
For recurrence report the error, tested hypotheses and next diagnostic; do not launch duplicate full runs.

Result: running/review/blocked status and date; baseline/full diff with added/deleted files;
criteria, commands/cwd/exit codes/pass-fail-skip, environment/dependencies/logs, protected hashes;
reuse/reasons for reruns, risks/unverified items, remaining work/WIP/processes, map proposals/lesson candidate.
Time/cost/model must be observed values with source/scope, or unknown.
For temporary task resources, report path/branch/process, owner, WIP state, and preservation needs;
mark unknowns explicitly. The orchestrator owns completion:
submission does not authorize deleting a copy/branch or another owner's processes.
Save full sanitized Result/logs under RULES. Send the HANDOFF ID/path and a brief summary; do not create a separate Result.md.
From another copy, provide its full diff/new files and path. DONE only means ready for review.
For a blocker: error, reproduction, tested hypotheses, and required external decision.
Address review findings within agreed scope; new requirements/protected changes require revision.
Do not choose the next task. Goals/background continuation require an explicit request and applicable tool rules.
