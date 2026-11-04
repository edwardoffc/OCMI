# Code lookup and task context

Use when locating an implementation/dependency or recovering a task would otherwise require loading
many whole files. The user can simply ask the project orchestrator to find the relevant code and
assemble context for the task; the agent chooses paths/tools. No user JSON, index service or extra chat.

1. Restore role/assignment/copy, current START and fixed inputs first. For the orchestrator, use the
   current request and its relevant memory. An index never selects work, grants access or replaces START.
2. Read the relevant CODEMAP rows; verify their paths, symbols and snapshot. Use scoped rg --files
   and literal symbol searches to locate definitions, callers/imports, contracts, tests and fixtures.
   Generated/private directories stay excluded. Do not enumerate neighboring projects or personal stores.
3. Select explicit files from that authorized area, literal identifier terms and mandatory input paths.
   Optional tools/ocmi-context.ps1 returns current hashes, definition/reference candidates and bounded
   source windows. tools/context-lib.ps1 provides Invoke-OCMIContext in a retained PowerShell session.
   Every query rereads/hashes selected files; unchanged outlines may be reused in session memory.
4. Read the actual affected functions and relevant callers/contracts/tests before deciding or editing.
   Search new names/imports as needed. Lexical matches include comments/unrelated names; dynamic,
   aliased and indirect calls may be missing. A context window is a locator, not the whole dependency.
5. RequiredFiles are included whole before optional snippets. If a required file/window does not fit,
   report status incomplete and omitted paths/ranges; raise the budget or retrieve required evidence
   separately within scope. Never use a context budget to waive protected inputs or acceptance checks.
   No hit, parse error, unsupported format or truncated search does not prove absence/completeness.
6. Put selected paths/snapshots and remaining gaps in existing START inputs/Result; no second task log.
   Propose only affected CODEMAP corrections; the orchestrator verifies and accepts them. Reverify
   sources after changes and before relying on previous context. Keep unknown coverage explicit.

The portable helper uses PowerShell 7 on Windows/macOS (Windows PowerShell 5.1 is also checked).
It parses PowerShell function outlines with the built-in AST; Go, JS/TS, Python and Rust declaration
outlines and all reference matches are lexical candidates, not compiler/LSP resolution. Other allowed
text contracts/configs can be required inputs. UTF-8 only. No imported language server or dependency.

Bounds: 128 selected/required path entries, eight identifier terms, 128 KiB/file, 2 MiB/read,
16,384 lines/file, 512 symbols/file, 512 reference windows, 128 matching outline entries.
Over-limit inputs fail closed; limited results remain incomplete. BudgetChars (default 16,000;
128–131,072) limits source snippet characters, not tokens or total JSON/report size. Metadata is bounded
separately. RequiredFiles must be supplied from the assignment; the helper cannot infer missing criteria.

The index is disposable session memory, scoped to one root/revision/tool version and content hashes.
Dirty edits are checked even with unchanged mtime; renamed/deleted paths must be reselected, not guessed.
Only the latest successful scope is cached; no disk cache, transcript, cross-project search, vector/SQLite
service or background indexing. Clear-OCMIContextCache discards it; a new process rebuilds it. Cache
metadata/source output can still be private: do not commit/publish raw context or include it in packs.
This is a bounded snapshot, not an atomic filesystem transaction or OS sandbox. Explicit file selection
and excluded names do not detect every secret; the agent must respect the task's access/data boundaries.

If PowerShell/helper is unavailable or a format is unsupported, use verified CODEMAP + scoped rg and
manual source ranges, stating the missing capability. Tree-sitter/LSP/Serena may be evaluated later
for an actual language gap; do not install/activate them or search other roots automatically.
