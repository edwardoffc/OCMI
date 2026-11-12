# Project environment recovery

[English](runtime-environment.md) · [Русский](../locales/ru/docs/runtime-environment.md)

This runtime source candidate diagnoses a selected project's environment and prepares a recovery
plan. It complements [checkpoints](runtime-checkpoints.md), without installing software or starting
services. Ready packages and the complete fresh-device rehearsal have separate acceptance gates.

For the separate opt-in preview that consumes an already-approved machine-local storage profile
during an ordinary task command, see [tool-storage profiles](runtime-tool-storage.md).

## Through the coordinator

After restoring a project into its selected folder, ask:

> Check the environment of `<absolute selected project folder>` before continuing. Use its existing
> setup guide and dependency manifests. Show what is installed, missing or still unverified, and
> the next recovery step. Preserve WIP; do not install tools or start services yet.

The coordinator selects existing files and current trusted installed executables, then invokes
`environment.inspect`. You neither maintain another environment checklist nor write JSON. The
report links to existing project setup instructions. The coordinator explains the smallest action
and verifies its permissions before carrying it out through the reviewed setup/resource route.
`missing` means no current tool binding was supplied; this is not a scan proving absence from
the whole computer. The coordinator first checks the required tool by name on this device.

For example, a restored Go project can have intact source but no Go installation or database.
First select/install compatible Go; a new inspection verifies its version and points to remaining
dependencies/data. A database client or `.env` file cannot prove restored data, credentials, access
or service readiness. These remain unverified until checked through the project's own route.
Resolve conflicting or unsupported declarations before guessing. Missing versions do not mean
install-latest. After setup confirm the new client/folder binding and current task; old STARTs never run.

## Coverage

| Source `kind` | Selected source | Diagnosed requirements |
| --- | --- | --- |
| `go-mod` | `go.mod` | Go minimum from `go`; `toolchain` is a suggestion, not an exact mandatory pin |
| `node-package` | `package.json` | Node `engines.node`; declared package manager as a remaining setup step |
| `python-project` | `pyproject.toml` | Python `project.requires-python`, parsed as TOML without running build backends |
| `guide` | Existing readable setup guide, such as `README.md` | Hash and references for setup, data/secrets/services/access and continuation |

Numeric exact versions and conjunctions of numeric comparison bounds are supported. Other range
syntax stays unknown; unsupported manifests/providers are not silently covered. This is not
dependency resolution, lock verification, connectivity or a product test. Existing PowerShell
doctor still checks memory/instructions and does not acquire these new meanings.

## Operator reference and limits

`ocmi request`: schema1, actor `main`, expectedRevision0, explicit `projectRoot`, action
`environment.inspect`. Closed payload: `sources:[{kind,path}]`, `tools:[{kind,executable}]`,
existing external `scratchRoot`; optional `provisions:[{kind,source,line}]` and
`continuation:{source,line}` refer to supplied guides. No credentials or arbitrary commands.
The role field is a workflow identity, not authentication or an OS sandbox. MCP refuses `environment.*`.

Tool `kind` is `go`, `node` or `python`. For an existing Go project with a readable `README.md`,
the payload can be:

```json
{
  "sources": [{"kind": "go-mod", "path": "go.mod"}, {"kind": "guide", "path": "README.md"}],
  "tools": [{"kind": "go", "executable": "<ABS_TRUSTED_GO_EXECUTABLE>"}],
  "scratchRoot": "<ABS_EXISTING_EXTERNAL_SCRATCH>"
}
```

Replace the placeholders with selected absolute paths. Source paths are relative to `projectRoot`;
the executable path is absolute, and `scratchRoot` must already exist outside the project.

Select at most32 regular UTF-8 files, 256KiB each/2MiB total. Unsafe, linked, secret/private and
outside paths are refused. Sources and executables are rechecked for drift. The project and its
Git index stay unchanged; no work DB is initialized. Reports contain hashes and typed observations,
not guide contents or raw subprocess output. A restored executable path is not automatically trusted.
Selected installed executables are regular files, streamed up to 256MiB each for identity checks.

Only fixed version commands for freshly selected installed Go/Node/Python run, outside the project,
with isolated configuration/TMP, no shell/build/installer or automatic Go download. Each probe has
a five-second limit plus native teardown; at most3 probes share forty seconds, output8KiB each.
The owned scratch operation records intent, child registration and completion. Tools are use-only.
Unknown process/cleanup state retains evidence and a next inspection action; no blind retry or
old-PID deletion. No Docker/GitHub calls, global settings or neighboring project access.

`ENVIRONMENT_INSPECTED` means inspection completed. Missing tools and unsupported declarations
remain visible, provisions remain unverified, and `readyToResume` stays false. Complete restoration
requires remaining setup/data checks and fresh task/client binding. This bounded component proves
neither whole-device restoration nor control over every command an agent might execute.
