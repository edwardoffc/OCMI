# Scoped code indexing and context assembly

[English](context.md) · [Русский](../locales/ru/docs/context.md)

Ask the project orchestrator: “Find the implementation for this task and assemble the relevant
context.” It restores the assignment, checks CODEMAP, searches the selected area, then retrieves
definitions, reference candidates and related required contracts/tests. The user need not run commands
or prepare JSON. The installation coordinator can explain this method; project access/integration
still needs a relevant request and the installed project's own assignment.

The self-contained [work guide](../templates/work/docs/guides/context.md) ships with two optional
PowerShell helpers under tools/. Base development uses the identical helpers under scripts/.
They never scan the repository or infer required files; the agent explicitly selects paths using
CODEMAP/rg and the task. No service, dependency installation or automatic project upgrade is needed.

For an agent/tool session (PowerShell 7 on Windows/macOS; native absolute path for the selected root):

~~~powershell
. ./scripts/context-lib.ps1
$context = Invoke-OCMIContext -Root (Get-Location).Path `
  -Files scripts/context-lib.ps1,scripts/ocmi-context.ps1 `
  -RequiredFiles docs/memory/RULES.md -Terms Invoke-OCMIContext -BudgetChars 16000
$context | ConvertTo-Json -Depth 10
~~~

The CLI scripts/ocmi-context.ps1 takes the same parameters, emits JSON and exits 0 for selected
candidates or 2 for incomplete/invalid retrieval. Invoke it with array parameters from a PowerShell
session; a shell's -File argument handling may not preserve comma-separated arrays. Library calls
keep an in-memory outline cache; separate CLI processes rebuild it. No imported/persisted cache.

Each selected file is reread and SHA256-checked on every request, including dirty WIP. Provenance
records exact file scope/content, caller-supplied SourceRevision (unknown by default, not verified Git
identity) and engine/runtime version. Root/revision/engine changes invalidate reuse; changed files are
reparsed, and entries outside the latest successful scope disappear. This saves parser/context work,
not filesystem reads. Reverify after later edits; the snapshot is not atomic. No raw source is retained
in the index, but returned context and names may be private. Neither is automatically saved/exported.

Required source text is prioritized whole; budget omissions/parse errors/limits are explicit. Status
selected means that retrieved candidates fit, not complete dependency analysis or readiness. Fixed
inputs, full functions and caller/contract/test coverage still need the agent's review/expansion.
See the work guide for limits, excluded directories, unsupported formats and scoped manual fallback.

## Backend decision

The pilot uses built-in PowerShell AST function outlines and explicit lexical Go/JS/TS/Python/Rust
candidates. This works without a new runtime or automatic installation; it does not provide semantic
cross-language references. These original OCMI helpers contain no copied third-party code.

| Option | Useful capability / cost | Decision |
| --- | --- | --- |
| CODEMAP + rg + session helper | Scoped locations, fresh snapshots, no server; lexical gaps remain | Default portable pilot with manual fallback |
| [Tree-sitter](https://github.com/tree-sitter/tree-sitter/blob/master/LICENSE) | Syntax-aware outlines; each binding/grammar needs its own version/license review | Core MIT inspected 2026-10-16; no grammar or binding selected/imported |
| [Serena/SolidLSP](https://github.com/oraios/serena/blob/d9e4c796fae0207ba6848d1df4b331122a5567b5/LICENSE) | Language-server definitions/references; language runtimes, activation and server processes | Inspected pinned component/application notices: SolidLSP MIT, application GPL-3.0-or-later; no vendoring/adapter installed |
| [Aider map](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/repomap.py) | Syntax tags and ranked graph under a budget; extra parser/graph dependencies | Apache-2.0 rationale inspected; no imported implementation or graph authority |

Choose a semantic adapter later only for a demonstrated language gap with exact-root validation,
capability/license checks and current installation authority. Unsupported/indirect references remain
unknown now. Personal retrieval is not enabled; a selected topic keeps its own roots/permissions.

check-context exercises definitions/callers/contracts/tests, content-vs-mtime edits, scope/revision/engine
changes, rename/delete, required budget overflow, encoding/size/path/symlink controls and real CLI/base
consumers. It joins check-all and the existing Windows/macOS matrix. Fixture character reduction is
measured against full text of that same scope; live token/cost/time savings remain unmeasured.
