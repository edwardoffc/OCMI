# Derived continuation and scoped context

## Native compact task

1. Bind the selected task and mark it ready through the existing API, with its required originals and bound checker.
2. Request `context.inspect` with `{"mode":"start","taskId":"TASK","taskRevision":1,"worker":"WORKER"}` and CLI `--format text`; use the selected root/actor and expected revision when needed.
3. The text START body is the exact native `data.start`: main explicitly delivers it once, without rewriting or saving another START. Tool/channel delivery needs its own authority; the view is an observation, not an assignment.
4. Before writes, the worker uses compact with the delivered `data.bindingRef` as `expectedBinding` and reads the current semantic originals through the route below. Run only the bound checker after reading its semantic criteria.
5. Compact example: Result: “Added both examples and expected JSON; evidence: core operation ID.” Main Acceptance: “Match CLI; prior text preserved.” Add any deviation, unverified point or real exception. Chat: result/link; native facts remain in core.

A low-risk README correction can use this semantic Contract:

```text
Outcome: README readers get correct count/no-match examples; other text stays.
Check: CLI semantics and bound docs check.
Scope: README.md/Result; code/tests/acceptance protected.
WIP: preserve current edits.
Decision: docs only; behavior change needs owner revision.
Resources: main keeps files/evidence through review.
```

Keep IDs, machine facts and general rules out of this example; reference verified core facts. Unknown/sensitive risk and file-only work retain full safeguards.

Review with one fresh compact of needed whole originals/current Result/state/revision, without preliminary duplicate status/text reads. Inspect output/resources for a real question.

### Whole context and retained receipt

In code-mode, the optional dependency-free [caller formatter](../examples/codex/native-output.js)
can be evaluated as reviewed source; it needs no Node APIs. Each `emit(text)` must
create its own text block. Use default native JSON (omit --format text) as input. Supply the actual exec result and the same output limit
used for that call; do not serialize the exec envelope as context:

```js
// forwardNativeOutput is the reviewed function from native-output.js.
const contextStatus = forwardNativeOutput(contextResult, text, {
  mode: "context", maxOutputTokens: 12000
});
// A separately requested operation.output remains a separate native JSON block.
const outputStatus = forwardNativeOutput(operationResult, text, {
  mode: "native-json", maxOutputTokens: 4000
});
```

Select needed originals explicitly in compact `requiredFiles`. Consume its whole
Contract once, then read only originals still missing; do not blindly read
Contract and products again after a facts-only selection. This caller uses default JSON for context and other actions; direct human --format text remains supported. The formatter forwards decoded data.context once and emits all remaining native metadata separately without context. It returns the native object in status.native for retained receipt/state; do not print it again. Pending sessions are returned to the caller;
it never polls or retries. Missing reliable token counts, truncation, unknown envelopes
and mixed responses produce metadata only. It is optional caller transport, not an
installation dependency, authority/security boundary or economic qualification.

For ordinary chat/human reading use, for example,
`ocmi call --root ABS --actor ACTOR --action context.inspect --format text --payload '{"mode":"compact","files":["rules.md"],"requiredFiles":["input.txt"],"budgetBytes":4096}'`.
`ocmi request --format text` also accepts the request envelope on stdin. `files` is reusable role context;
`requiredFiles` and the active Contract are whole within allowed text scope. No `terms` are used.
START separately lists exact bound project-root `.gitignore`/`.gitattributes`
in `data.metadataOnlyFiles`; they remain binding-protected metadata, not required text.
Explicit compact text selection refuses with `CONTEXT_METADATA_ONLY`. Omit them
from text retrieval only if the semantic Contract does not require their contents;
otherwise obtain an agreed readable original. A metadata-only active Contract
also refuses safely and needs an approved readable Contract/task revision.
Other scope/privacy restrictions still apply; refusal delivers no context or freshness claim.

Supply the opaque base64url `receipt` only for context actually retained in this
chat; new/uncertain chats omit it. Core freshly verifies originals: unchanged
context appears in `retainedFiles` without repetition; `receiptUnchanged:true`
keeps the old receipt. Changed bytes or same-byte native replacement return the
whole original and a new receipt. It stores no source and grants no authority.
Cold setup and recurring use remain separately visible.

Compact `expectedBinding` uses the delivered START's `data.bindingRef`: it
requires still-ready binding, STOP false and no unresolved activity. Read whole
Contract/required originals before acting. Native run/submit/review/integrate
verify bindings; do not mirror them with manual per-file SHA scripts. Protected
tests, STOP and resource ownership remain. Integrated/cleanup-done tasks may
omit unrequested historical Contract text; bound originals are still verified.

Compact text retains the typed header, state/STOP/freshness, retainedFiles,
new receipt and whole originals. Default JSON remains for programmatic parsing
or receipt handling when needed; text is only for CLI compact/start, never
mutations/summary/selected/MCP. Refusals stay full JSON with nonzero exit;
unsafe raw text falls back to full JSON. The context budget includes labels
(default 4096, range 128..65536); needed text is never truncated. A typed budget
refusal names required bytes. No measured savings are claimed.

[English](continuation.md) · [Русский](../locales/ru/docs/continuation.md)

This preview guide describes the native `context.inspect` route in a matching runtime. It helps a person or coordinator begin with current operational facts, then deliberately read only the task files that answer an actual gap. It does not replace the human PLAN, rationale, unresolved questions, handoff, or a project's own permissions.

## Everyday flow

1. Keep the selected root explicit. Use summary for a state-only question; for review use the consolidated compact route above.
2. Use the summary to see the current assignment, STOP/operation state, and next action. It is an observation, not approval to proceed.
3. Select a small set of named files and literal identifiers only when the assignment leaves a concrete question unanswered. Expand the selection if the response reports omissions or incomplete coverage.
4. Have people or agents record only changed semantic decisions, PLAN, and remaining work. Do not manually copy task/STOP/operation/cleanup state or counters across documents: the core derives those operational facts. A later request must obtain a fresh view after relevant edits or state changes.

For an installation, the coordinator first confirms the client association and uses the selected home. For a work project, the orchestrator selects the root and records the assignment; a worker receives that bounded assignment rather than discovering a project or client relationship. A summary is disposable session evidence; the derived START above requires explicit delivery and is not itself an acceptance or permission to change files.

## Optional summary observation

The native request CLI receives one JSON request on standard input. Replace `ABSOLUTE_SELECTED_ROOT` with the root already selected for the task; do not substitute a Git label, URL, or a different checkout.

```json
{
  "schemaVersion": 1,
  "action": "context.inspect",
  "projectRoot": "ABSOLUTE_SELECTED_ROOT",
  "rootKind": "project",
  "actor": "main",
  "requestId": "continuation-summary-001",
  "expectedRevision": 0,
  "payload": {"mode": "summary"}
}
```

This JSON is a technical example for an agent/tool; an everyday user does not need to fill it in. Its actor is the already assigned/bound actor (`main` here), not a role inferred by the request. Pipe the object to `ocmi request`. A successful summary reports current native state, bound assignment/provenance, a bounded plain-language summary, and `inputFreshness: "not-read"`. It reads no task source and makes no claim that the task is ready, complete, or fully understood. `expectedRevision: 0` observes the current revision; a positive value requires an exact revision match.

## Select only the gap

When the summary and human handoff identify a precise question, request named files and literal identifiers. For example, this asks for the bound inputs plus two explicit files and the terms that locate a request handler and its test:

```json
{
  "schemaVersion": 1,
  "action": "context.inspect",
  "projectRoot": "ABSOLUTE_SELECTED_ROOT",
  "rootKind": "project",
  "actor": "worker-1",
  "requestId": "continuation-selected-002",
  "expectedRevision": 0,
  "payload": {
    "mode": "selected",
    "files": ["runtime/internal/service.go", "runtime/internal/service_test.go"],
    "requiredFiles": [],
    "terms": ["Handle", "status"],
    "budgetBytes": 16000
  }
}
```

This actor is likewise already assigned/bound (`worker-1` here). The bound contract and `requiredFiles` are always whole text; the contract can never be reduced to excerpts. A bound input not explicitly named in `files` also stays whole by default. A bound source explicitly named in `files` may return only relevant excerpts unless it is also in `requiredFiles`, but its complete bytes are still read and hash-verified against the binding. Put every original that the START requires the model to read whole in `requiredFiles`. Other selected files require terms. `budgetBytes` limits UTF-8 excerpt text; bounded metadata is accounted separately. Paths are plain, relative, and root-scoped: there is no automatic repository scan, directory discovery, shell command, network access, persisted index, or automatic memory write. A request cannot use this route to obtain hidden/generated/private locations, links, databases, archives, keys, or excluded directories. On an installation home, selected code scope is public root text, `system/**`, and `OCMI_Sync/coordinator/**`; personal topics, recovery data, and `local/**` are outside it.

The response explicitly lists selected excerpts, whole required inputs, omissions, notices, outline candidates, provenance, and read metrics. `inputFreshness` records whether guarded bound bytes were verified; it does not say that the model received every bound byte. It caches only bounded parsing facts in the current process; raw source is not persisted, and a new CLI process starts cold. Returned source can still be confidential.

## Treat incomplete and stale results as evidence

`CONTEXT_SELECTED` is a useful scoped read, not semantic dependency analysis. It still reports `readiness: "not-evaluated"`, `coverage: "explicit-files-only"`, and `completeness: "not-established"`.

`CONTEXT_INCOMPLETE` gives safe partial data with visible omissions or notices: read the stated next action, reduce/expand the explicit scope, or review the missing input manually. `CONTEXT_INPUTS_CHANGED` deliberately returns no source excerpts when bound task inputs changed; inspect and revise that same task. `CONTEXT_CHANGED` means root, state, or source drift was observed while reading; make a new request rather than combining it with the old result. Invalid or unsafe scope is refused without excerpts. STOP remains meaningful for new work, but read-only observation may still explain the stopped state.

## Adoption and limits

Existing installed templates, ZIPs, receipts, and project documentation remain as issued. Later adoption is coordinator-led: link an existing human PLAN and handoff to this guide, preserve their history and local rules, and avoid copying operational status into several documents. Do not rewrite an established project or treat a derived view as a migration instruction.

The route is preview functionality until release qualification. It does not promise a client association, broad tool authorization, client trust, semantic cross-language references, a complete code map, automatic discovery of unregistered tools or resources, or measured time/token savings. The existing core retains its own operational accounting. See [runtime](runtime.md) for the request envelope and operational recovery boundaries, and [context](context.md) for the optional portable PowerShell helper.
