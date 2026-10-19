# Long operations and continuation

[English](waiting.md) · [Русский](../locales/ru/docs/waiting.md)

Distinguish active execution, a named external event and no observed progress. Save a checkpoint
in the existing handoff before returning control; resume the same operation instead of relaunching it.
Use the self-contained [work waiting procedure](../templates/work/docs/guides/waiting.md), or the
[topic guide](../templates/personal/memory/GUIDE.md) for lightweight memory. The coordinator uses
the same observation/authority limits in its private setup handoff, never the source-development queue.

Prefer completion events; bound manual observation by interval/deadline and unchanged-poll limit.
Without a local rule stop after three unchanged polls in a turn. This ends observation, not the job,
acceptance gate or user's budget. Unverified execution remains unknown, not success or a confirmed hang.
Scheduling, notifications and background continuation need actual capability and authorization.
These file rules cannot revive a frozen AI client/model turn; investigate that application separately.
