# Roles and procedure selection

**OCMI 1.2.1, rules revision 2026-10-11**

After [RULES](RULES.md), select exactly your own procedure; read another only for a relevant action.
Inputs required by the current START/contract remain mandatory regardless of this routing.
These rules apply to new assignments/explicit revisions; do not rewrite issued packages.

| Role | Procedure | Context after the shared rules |
| --- | --- | --- |
| Orchestrator | [Preparation and acceptance](roles/coordinator.md) | INDEX, BRIEF, current STATE/handoff, relevant PLAN section |
| Worker | [Execution and Result](roles/worker.md) | Addressed START, fixed Contract, own Result, listed inputs |
| Consultant | [Answers grounded in sources](roles/consultant.md) | INDEX and sources relevant to the question |

[PLAN](PLAN.md) is the single queue; [handoff](tasks/orchestration/HANDOFF.md) records assignments.
The [task protocol](TASK-PROTOCOL.md) defines formats, not a new assignment.
The project determines slot count/models; each task has one owner and nonconflicting paths/resources.
A separate copy does not isolate processes/secrets. Verify environment and permissions separately.
Creating chats, sending messages, commits/publication, production operations, background tasks,
and model changes are not authorized by the template; handoff is manual by default.
