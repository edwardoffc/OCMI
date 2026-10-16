---
name: ocmi-search-first
description: "Find an existing in-scope implementation or suitable dependency before building a new capability."
---

# Search before building

Use when a task needs a capability whose existing implementation is uncertain. Skip a new search when current evidence already locates the right implementation.

Read the selected code-map entry and required inputs, then search symbols, callers, tests and package manifests within the assigned root. Check the real consumer and compatibility, not a similarly named helper. Expand an incomplete search within scope; do not exclude tests or declare absence from truncated output.

Only if the local search leaves a gap, evaluate an external package/service when that access is permitted. Compare API fit, maintenance, license, transitive dependencies and operational cost against a small local implementation. Use primary documentation for current APIs; never send private code, paths, customer data or secrets in search queries. Finding a package does not authorize installing it or adding network access.

Return the located path/symbol and evidence, or a short reuse/extend/build recommendation with material tradeoffs and unresolved gaps. Save it in the existing Result/Acceptance; do not create a second inventory.

Tools unavailable: name the unverified area and use supplied sources, without fabricating search results. No researcher agent or global search is launched automatically.

Original OCMI instructions under [MIT](../../LICENSE). Optional rationale: [ECC search-first](https://github.com/affaan-m/ECC/blob/76aaa77df6adc5b889e689007bf99b9a4f74af34/skills/search-first/SKILL.md).
