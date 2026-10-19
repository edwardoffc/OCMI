---
name: ocmi-ci-triage
description: "Diagnose a failing or repeatedly returning check using causal evidence before another costly run."
---

# Causal CI and failure diagnosis

Use for a real failure or unexplained repeated return, not as a compulsory phase of every change.

Read the preserved failed log, command, revision, runner/tool versions and relevant fixtures. Separate queue/setup/check/build/deploy time when observed; missing timing stays unknown. Compare the actual CI mode with local execution and follow the affected consumer.

Classify implementation, assignment/check, environment or unexplained instability. State one falsifiable hypothesis and the smallest diagnostic that distinguishes it from alternatives. Preserve WIP and original failed evidence; use isolated data and authorized resources. A green retry alone does not establish a fix.

If an unexplained setup/barrier failure occurs before behavioral assertions, locate that layer before speculative product-algorithm changes or repeats. Record observation/hypothesis/next check in existing Result/Acceptance; run only authorized diagnostics, or report the exact missing authority/evidence. A demonstrated implementation defect may be fixed directly. A control that disproves a test premise does not validate the product; protected inputs still require an explicit orchestrator revision.

Count failures across the integrated result and prerequisite repairs, including new task IDs. From the second unexpected failed cycle, record the changed approach, next evidence and sufficient check set in existing Acceptance/handoff before another costly run. Do not infer an architectural failure from a fixed retry count.

Return cause/evidence, changed approach, targeted check result and what remains blocked. Broader tests need an affected-dependency/integration/risk reason. Mandatory safety failures remain blockers. No production repair, release, model change or unattended retry is authorized here.

Without a runnable environment, return a bounded diagnosis and the exact missing evidence, not a claimed PASS.

Original OCMI instructions under [MIT](../../LICENSE). Optional rationale: [Superpowers debugging](https://github.com/obra/superpowers/blob/bb92a77741419a4ab5f06e711a283343f1ada0c3/skills/systematic-debugging/SKILL.md).
