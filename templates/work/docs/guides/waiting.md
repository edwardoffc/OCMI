# Bounded waiting and recovery

Use when an assigned operation takes time or needs an external event. This is a continuation
procedure, not a new role, task status, background service or authority to restart work.

## Identify the observed state

- **Executing:** an identified process/job is active; record its handle/cursor and actual progress.
  Wait on that same operation; do not submit another command or job merely because output is quiet.
- **Waiting externally:** name the event, its owner/source and exact run/request ID, such as a queued
  CI job or an awaited decision. Pending is neither execution success nor a failed product check.
- **No observed progress:** state what evidence stopped changing and for how long, or mark timing
  unknown. Silence alone does not prove a hang; a missing status source leaves execution unverified.

## Bound observation

Prefer a supported event/completion wait over polling. Before polling choose an interval, deadline
or stop event, and a maximum of unchanged observations from the task's conditions. Without a local
policy, stop after three unchanged polls in one turn, save the checkpoint and return control.
Back off between unchanged observations and respect client/tool wait limits. A new message or
shorter authorized stop condition takes precedence. Do not continuously poll to keep a chat alive.

For no observed progress, use one minimal authorized read-only status/log diagnostic to distinguish
running, queued, failed, missing and unknown. Inspect the same handle and relevant evidence only.
Do not extend a monitoring loop indefinitely under new wording. Escalate with the exact missing
evidence/decision or continue independent authorized work; never invent a completion or a new budget.
Explain actionable changes or a required decision; repeated unchanged checks need no chat narration.

## Save and resume

In existing Result/handoff keep: task/revision/copy; observed phase/time/source; process/run/cursor;
completed work and WIP/evidence links; remaining work; next event/check and observation limit;
owner/authority/stop conditions. A short paragraph is enough; no new JSON or monitoring log.
On continuation revalidate assignment, permissions, WIP and the saved operation before any launch.
An absent/stale handle is not proof of failure or permission to restart. A retry needs evidence of
the prior operation's state, a causal reason, permitted resources and duplicate prevention.
Elapsed waiting does not invalidate unchanged test evidence; an affected dependency still can.

If the turn cannot remain open, say observation has ended and how to resume it. Do not promise a
notification, scheduled check or automatic continuation without supported capability and authorization.
Do not create chats, message workers, change models or start background automation just to wait.
These instructions cannot wake a stalled client/model turn; diagnosing that application requires
separate evidence and scope. Unsaved conversation cannot be recovered from files that were never written.
