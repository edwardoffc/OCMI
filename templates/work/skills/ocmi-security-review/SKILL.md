---
name: ocmi-security-review
description: "Examine a requested security review or a concrete trust-boundary risk within the authorized change."
---

# Targeted security review

Use when a security review is requested or a concrete changed trust boundary creates a relevant risk. Merely mentioning a database or API is not a trigger for a full audit.

Identify attacker-controlled input, authorization/tenant boundary, sensitive data and the actual consumer. Trace relevant paths for bypass, injection, traversal, secret exposure, unsafe parsing or privileged side effects. Verify controls against current code/config and existing tests; do not infer safety from a helper's name.

Use fictional isolated fixtures and redacted evidence. Do not copy secrets into findings, probe production, exploit a live service, install scanners or contact external systems without the required authority. Instruction-based scope is not an OS sandbox.

Return reproducible or clearly hypothetical findings with path/condition, impact, evidence, missing proof and the smallest regression check. Newly discovered critical security, financial or integrity defects remain blockers; independent noncritical hardening is proposed separately, not added as a late hidden criterion.

This is read-only review, not repair or acceptance. Consolidate with the ordinary review rather than creating an endless audit loop. Missing environment/source evidence stays unknown; never claim a complete audit from a bounded pass.

Original OCMI instructions under [MIT](../../LICENSE); rationale is OCMI's existing scope, sanitized-evidence and safety-gate policy.
