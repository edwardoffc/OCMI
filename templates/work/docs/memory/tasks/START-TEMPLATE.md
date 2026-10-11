# START — message to the worker

**OCMI 1.2.2, rules revision 2026-10-11**

Template: do not begin until fields are filled and readiness is verified.
Status: ready [only after orchestrator verification]. Role: worker for project [name / root].
Only task [ID], revision [number] is assigned; current assignment record: [orchestrator handoff / section].
Working copy: [absolute path]. Immutable package: [path].
Baseline: [commit/snapshot]. Current HANDOFF: [path].
Required inputs: [rule versions, paths/sections/symbols]. Commands/cwd/checker: [contract].
Environment profile: [existing STATE/runbook / lines and evidence, or explicitly unknown].
Objective and revision delta: [briefly]. Remaining work/WIP/process/cursor: [for continuation].

Verify project/role/task/revision/copy, current assignment, scope, and protected hashes.
For a new assignment check the baseline; for continuation preserve Result/WIP and wait for the same check.
Do not start from an archived START or file recency alone; clarify contradictory assignments.
Change only AllowedPaths and the Result body; do not change Contract/Acceptance/JSON or protected files.
Meet criteria and mandatory checks; reuse only within the contract and checker.
Save the full sanitized Result/logs under RULES, send the record ID/path and a brief summary.
For a blocker: reproduction, tested hypotheses, and required external action.
Do not choose the next task. Do not copy history or unchanged rules into the message.
