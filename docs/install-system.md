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
