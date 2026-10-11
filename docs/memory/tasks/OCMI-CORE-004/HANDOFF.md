# OCMI-CORE-004: core improvements and release 1.2

Historical record translated into English. Original release measurements refer to Russian sources.

## Contract

Owner request, 2026-10-11: implement six improvements from the ECC review and release 1.2 if checks pass.
The current chat performs local changes by direct request; no separate workers/new chats assigned.
Git baseline: 246d2f975d58016dfea3e9c50706c71a7753907c; prior research/memory preserved as WIP.
Scope: both templates, own docs/memory, guides/evals, structural checks, and new release.
Protected: releases 1/1.1, Contract/Result/Acceptance schema, others' WIP; installed projects outside scope.
Do not install ECC/hooks/observer, change models/permissions, or publish remotely.

| Requirement | Verification and sufficiency |
| --- | --- |
| Role-based reading | Local routes, three procedures, no mandatory orchestrator procedure for worker/consultant |
| Process preservation | Protected tests, baseline/WIP, full/delta review, reuse, integration remain accessible |
| Recovery | Explicit START identity, current assignment, incomplete/historical data, process/cursor |
| CODEMAP, lessons, environment | Area verification source; candidate/acceptance/evidence/status; configured/available/verified |
| Lightweight profile | Topic/freshness selection without work protocol |
| Release | Documents, schemas/links, diff check, immutable old ZIPs, complete new ZIP/source agreement |

Release gates are structural and packaging checks. New-chat behavior/costs remain separate OCMI-EVAL-001;
do not mark them passed based on structural PASS.

## Result

Completed. New procedures and profile refinements implemented; work and lightweight 1.2 releases prepared.
Baseline 1.1 rejected by the new route checker because separate routes were missing; after changes, 15 checks passed.
Final checker: 17 checks passed, including canonical/development procedure agreement and version labels.
check-docs, git diff --check, and check-release passed; 18 work/5 lightweight files matched ZIPs;
old 1/1.1 ZIPs unchanged. [Evidence and inventory](../../../releases/1.2.md).
Required instruction bytes: orchestrator 21,203, worker 14,014, consultant 11,366; baseline 19,240.
No model runs in new chats; tokens/cost unknown, retained under OCMI-EVAL-001.

## Acceptance

Accepted: assigned structural/packaging checks passed; compatible schema and key safeguards retained.
Local base integrated into sources; OCMI 1.2 recorded in separate archives/receipt.
Installed projects, models/permissions, and old packages unchanged; push not assigned.
Structural PASS did not replace independent behavioral evaluation. No direct savings measurement.
