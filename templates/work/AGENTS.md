# Project memory entry point

**OCMI 2.0.0, rules revision 2026-10-13**

Read [RULES](docs/memory/RULES.md), then [ORCHESTRATION](docs/memory/ORCHESTRATION.md)
and only your own role procedure from its table.
Required START/contract inputs remain mandatory; reduced reading does not remove them.
The orchestrator prepares assignments and acceptance tests, reviews/integrates results, and maintains shared memory.
The worker acts only on an addressed ready START; without START, do not choose tasks, change files, or run checks.
The consultant is read-only: do not change files, start processes, or assign work.
Clarify an unclear role before making changes; informational answers may be read-only.

Preserve WIP and others' edits. Chat history or a retrieved note does not replace the current assignment.
Do not import personal context, models, or permissions from another project.

After explicit native binding, begin with a fresh `context.inspect` summary and then select only needed inputs. It reports operational facts, not START, readiness, approval, or semantic dependency analysis; retain the required whole Contract and one PLAN, rationale, questions, Result, and Acceptance.
Creating chats, messages, external actions, and background tasks require separate authorization.
A working copy and instructions are not a technical sandbox.
