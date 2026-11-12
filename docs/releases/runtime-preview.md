# OCMI runtime preview

[English](runtime-preview.md) · [Русский](../../locales/ru/docs/releases/runtime-preview.md)

This technical preview extends the OCMI 1.3.5 file workflow with a native local executable.
Use an isolated synthetic project first. Existing projects are not updated automatically.

## Included

- Transactional task and operation state, explicit acceptance, retained outcome history and retry gates.
- Accountable local files and supervised processes, with bounded interruption and recovery paths.
- Selected file-memory checkpoints and Git checkpoints preserving staged, unstaged and selected non-Git work.
- Project-environment inspection using selected manifests and setup guides.
- Complete Windows x64, macOS Apple Silicon/Intel and Linux x64 kits, with bilingual guides,
  a ready executable, dependency notices, exact source identity and checksums.

[Download the preview](https://github.com/edwardoffc/OCMI/releases/tag/preview-2026-11-04).
Use the archive for your device and verify its checksum against the publisher's record.
The native core runs without installing Go, Git, Python or PowerShell just to invoke it.
Individual project tools and contributor checks have their own prerequisites.

Follow [new-device recovery](../new-device.md) for selected memory and fresh project/coordinator
bindings. Private data and setup records are not part of the downloaded system.

## Limits

The runtime is a preview, not a universal sandbox or control over arbitrary agent shell commands.
Only explicitly supported resource providers and client routes are qualified; additional providers
need their own tracking and completion checks. File restoration does not restore authentication,
client trust, old process ownership or permission to resume a saved operation.

Native checks cover four platforms and selected synthetic recovery paths. A full fresh-device
work rehearsal, ordinary client behavior and measured productivity remain separate evaluations.
Do not infer readiness for erasing an existing device from successful archive verification alone.

[Runtime guide](../runtime.md) · [Installation](../install-system.md) · [Security](../../SECURITY.md)
