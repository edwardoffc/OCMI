# Install OCMI

[English](install-system.md) · [Русский](../locales/ru/docs/install-system.md)

## Canonical native route

1. Obtain or reuse a verified matching ReadyKit. A URL or downloaded source is
   orientation only: it must not be treated as an installed source checkout.
2. Verify the trusted archive hash, then unpack the kit into an empty chosen
   home. Its public payload is `system/`; its only native entry is
   `system/bin/ocmi[.exe]`. Preserve an occupied, unknown, or partial home.
3. Run `ocmi install --home ABS` (or the read-only
   `ocmi install --home ABS --inspect`). This creates only declared machine
   infrastructure in `local/` and the `OCMI_Sync` root.
4. The installation coordinator runs
   `ocmi coordinator prepare --home ABS --language en|ru --client-project NAME`.
   It generates `local/bindings/coordinator/STARTUP.md` and the flat portable
   `OCMI_Sync/coordinator/{coordinator.json,AGENTS.md,INDEX.md,HANDOFF.md}`.
5. The user creates or reuses the client project bound to that exact home,
   opens a separate coordinator chat, and sends the generated local prompt.
6. The coordinator reads the flat inputs and local binding, verifies current
   project association with tools or current user confirmation, then records
   explicit local evidence with `ocmi coordinator confirm --home ABS
   --client-project NAME --evidence-source user-current|client-tool --evidence TEXT`.
   `ocmi coordinator inspect --home ABS` is read-only.
7. Only after confirmation may the coordinator offer optional personal storage
   or project integration. Neither is created automatically.

The native command records reported evidence; it cannot independently prove a
chat identity or client UI. Do not copy old client trust, databases, receipts,
prompts, or confirmation into a new home. Existing legacy setups remain in
place as compatibility inputs; migrate them only through a separately selected
route. Never publish an installed-home archive.

## Update an installed home

Use a verified, external candidate binary to update one explicitly selected
home. The updater process runs from a separately verified ReadyKit outside that
home; activation then installs its verified public payload into the target. In
the commands below, `ocmi` means that external candidate executable, never the
old executable in the selected home. First inspect without writing:

```text
ocmi update inspect --home ABS [--candidate ABS]
```

Apply only when the inspection reports the supported identity transition and
the exact expected source and candidate commits are known:

```text
ocmi update apply --home ABS --candidate ABS --expected-source HEX40 --expected-candidate HEX40
```

The update accepts only the declared compatible formats in
`build-info.updateCompatibility` (schema, layout, installation receipt,
coordinator memory/binding, and state schema). It is not an arbitrary data
migration, source-checkout replacement, compiler/download workflow, or
whole-home restore. An old, future, or otherwise unsupported format must
refuse before activation.

The supported transition retains the selected home's stable paths and identity:
`OCMI_Sync`, `local`, bindings, cache, runs, IDs, client-project association,
language, lifecycle/evidence data, and STOP resources remain state rather than
update payload. Only the public system payload and the declared source receipts
or generated coordinator STARTUP input may change, with the expected prompt
validated. An update does not automatically hot-reload a running process;
restart or re-open an affected client only through its normal user-authorized
route. If a busy executable or native work prevents activation, stop that work
only through its own normal authorized route before retrying; never force-delete
or replace the busy file.

If activation was interrupted, inspect reports pending recovery and the next
action. Use `ocmi update recover --home ABS` only for that attributable pending
activation. Use `ocmi update rollback --home ABS` only when the latest retained
verified prior public generation is available. Both commands preserve the same
retained state; they do not invent a legacy migration or change trust/binding
evidence.

Recovery provenance refusals include a bounded `diagnostic` with the original
boundary, validator, failed predicate, error category, phase and recovery step;
IO failures can include a numeric OS error code. Fields contain fixed labels
and counters. Preserve the first response and retained update state for diagnosis:
the boundary distinguishes checks before admission from failures during recovery.
