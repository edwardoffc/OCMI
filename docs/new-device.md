# OCMI on a new device

[English](new-device.md) · [Русский](../locales/ru/docs/new-device.md)

Use a ready system kit, restore only selected memory, then confirm fresh client bindings. Windows,
macOS and Linux use this same sequence. The coordinator prepares commands; you do not write JSON.
This source-preview workflow has separate package/file-consumer checks; the complete fresh-device
task rehearsal and a numbered public release have their own gates.

## 1. Put the system in place

Choose the archive for this device: `windows-amd64` for Windows x64, `darwin-arm64` for Apple Silicon,
`darwin-amd64` for Intel Mac, or `linux-amd64` for Linux x64. Verify the archive checksum against the
trusted publisher's record and its exact source identity. An internal checksum alone is not authenticity.
Unpack into an empty chosen system folder, such as `00_OCMI`. Preserve existing copies and WIP.

A ready kit includes the public system, both language editions, `build-info.json`, checksums,
reviewed dependency notices and `bin/ocmi.exe` (Windows) or `bin/ocmi` (Unix, executable mode).
The core runs directly: no compiler, Git, Python or PowerShell is needed to invoke it. Maintenance
checks and a particular project's tools may have their own requirements. A source ZIP or template
ZIP is different; verify a matching ready executable before claiming the runtime is installed.
Respect the OS's downloaded-program checks; do not disable system protection to make a test pass.

## 2. Restore coordinator memory

Give the temporary installation chat the exact selected complete private checkpoint:

> Restore OCMI on this device from `<coordinator checkpoint>`, using this system folder.
> Restore only coordinator memory. Preserve history and unresolved operations. Show missing inputs
> and the client setup steps. Personal topics and work projects are not selected yet.

The installer reads [BOOTSTRAP](../BOOTSTRAP.md) and this selected procedure. It verifies the kit,
checkpoint schema/kind, source/version compatibility and exact destination before writing. A declared
version or hash is not trust. Unknown compatibility/migration stays pending. Use the existing
[checkpoint inspect/restore](runtime-checkpoints.md) through the bundled native executable.
Restore into empty `.local/coordinator`, preserving all selected original files and continuation.
Do not replace existing memory with blank templates or overwrite a conflicting destination.

Keep the old filled STARTUP and current snapshot as private history. Render a new STARTUP from the
chosen edition's existing [template](../templates/coordinator/STARTUP.md), filling all six fields
with this device's actual system/memory paths, source identity, languages and expected client project.
Update only current private path/binding facts; preserve decisions, STOP, missing inputs and unknown
operations. Old paths, project pointers, PIDs, leases and past approvals do not become new authority.
Do not execute a saved START or reconnect accounts merely because a checkpoint mentions them.

## 3. Confirm the project and coordinator

Create/reuse a local OCMI project, attach the exact installed system folder as its primary folder,
open a separate coordinator chat inside it, grant the required access and send the new filled prompt.
The coordinator verifies its files, source, actual project/folder association and access, then records
handover in its private HANDOFF. Use current tools or your current confirmation; old client records
or restored files alone leave setup pending. No chat transcript is required.

Codex projects organize folder context; actual access is controlled separately. Project configuration,
MCP connections and hooks need their supported current trust/configuration steps. Do not restore
user-global client configuration, authentication or hook approvals from ordinary memory packages.
See official [projects](https://learn.chatgpt.com/docs/projects?surface=app),
[MCP](https://learn.chatgpt.com/docs/extend/mcp) and
[configuration](https://learn.chatgpt.com/docs/config-file/config-basic).
The [runtime guide](runtime.md) describes supported transports and their limits.
File restoration keeps `readyToResume=false` until the remaining task/environment/binding checks.

## 4. Continue a selected topic or project

Personal memory remains optional. For a selected file-only topic, make its complete files/checkpoint
available locally, resolve sync conflicts, and bind a separate client project to that topic folder.
Its new chat reads its own entry, facts and handoff. Reuse [topic recovery](personal-memory.md);
do not open all topics or create a work-task database for an ordinary conversation.
Existing external stores remain where they are until a separately selected move.

For a selected work project, restore its repository and required Git/non-Git continuation separately,
then follow its own setup guide and [environment diagnosis](runtime-environment.md). GitHub restores
only pushed work. Saved evidence, data, secrets and access may need their separately permitted source.
The coordinator registry is navigation, not permission to read or update that project.

## Private transport

Keep active coordinator memory in `.local/coordinator`; machine-specific records, when needed, in
`.local/device`. Optional `.local/sync/personal` holds selected topics; `.local/sync/coordinator`
holds closed coordinator checkpoints, not a second edited coordinator. An explicitly selected
external transport folder is equally valid. Public kits contain none of these private areas.
Synchronize only the selected private transport; exclude live databases/WAL, caches, executables,
credentials and active device state. Fully download placeholders before verification. Preserve both
sides of a conflict and an independent recoverable backup; synchronization alone is not backup.
Installing the kit does not configure a cloud provider or create a personal store.

Ask the coordinator to report: restored scope; preserved originals; missing/conflicting/unverified
items; current binding status; one next action. Stop at the specific missing input or permission,
preserve WIP and continue from files when it is resolved.
