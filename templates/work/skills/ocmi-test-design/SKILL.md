---
name: ocmi-test-design
description: "Close a demonstrated acceptance-coverage gap with sufficient checks before freezing or revising an assignment."
---

# Design sufficient acceptance checks

Inputs: agreed behavior/consumer, risk, existing checks and their dependencies, protected baseline and permitted test resources. First map each criterion to an existing check and explain sufficiency; add a check only for a material gap.

Choose a real affected consumer path and material negative cases. Verify actual IDs, units, formats, compatibility, stop/restart or data-integrity behavior where relevant. Mocks establish only their substituted boundary. Do not demand product end-to-end tests for prose or mirror implementation details as the oracle.

Use a valid control: a known correct case plus a targeted wrong behavior that the check rejects, or another justified oracle. Distinguish a requirement failure from broken fixture/environment setup. A missing file alone does not prove a behavioral regression.

A specialist's default output is a proposed criterion-to-check mapping, fixture/oracle, commands, expected outcomes and limits. Writing/running tests requires the assigned scope and role; a read-only consultant returns advice only. The orchestrator reviews and validates proposals before freezing checks; an issued protected checker changes only through an explicit revision.

Reuse valid evidence; do not rerun a full suite just to format a report. If a prerequisite is missing, describe the uncovered criterion and next diagnostic rather than inventing coverage. No test result is acceptance.

Original OCMI instructions under [MIT](../../LICENSE); rationale is OCMI's existing sufficient-check and fixed-input workflow.
