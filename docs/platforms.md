# Windows and macOS

[English](platforms.md) · [Русский](../locales/ru/docs/platforms.md)

Windows x64, Apple Silicon, Intel Mac and Linux x64 use the same setup: verified native launcher
and ReadyKit → native `unpack` into an absent home →
installation folder → bound client project → confirmed
coordinator → optional personal storage and project integration. URL and downloaded-copy entry
lead to that result. If the client cannot bind a folder or access files, setup remains pending;
the installer provides the missing manual steps. See [installation](install-system.md).

Use native paths: for example, `C:/Work/00_OCMI` on Windows or `~/Projects/00_OCMI` on macOS.
The installer resolves `~` to the selected absolute path in private startup records.
Current coordinator memory is `OCMI_Sync/coordinator`; machine-only state is `local/`, neither of
which is public payload. Personal storage is optional and separately selected.
Profile filenames, language selection, roles and recovery inputs are identical.

Ordinary file-based setup and use do not require PowerShell: the agent uses available file/Git
tools or gives manual steps. Do not run remote scripts merely to install the Markdown system.
Client support and permissions still determine which actions the agent can perform.

## Maintaining the source

Install Git and [PowerShell 7](https://learn.microsoft.com/en-us/powershell/scripting/install/install-powershell).
Run from the repository root on either platform:

~~~powershell
pwsh -NoProfile -File scripts/check-all.ps1
~~~

On macOS, `pwsh` runs natively on Intel and Apple Silicon. Windows PowerShell 5.1 is a separate
Windows compatibility check, not a macOS dependency. Secret scanning also requires a verified
Gitleaks binary matching the operating system and CPU. See [validation](../scripts/README.md).

CI runs all documentation and privacy checks on Windows, macOS Apple Silicon (`macos-15`)
and Intel (`macos-15-intel`). The required aggregate checks fail if any platform fails or skips.
Shared path guards use native separators and case-sensitive Unix scope, reject symlink inputs,
and limit cleanup to invocation-owned temporary directories. Synthetic fixtures exercise spaces,
Unicode, unsafe paths, private-memory exclusion and actual copied validation scripts.
No private memory or installed project content is needed.

These checks validate source behavior on the runners; they do not test every AI client's UI,
model recovery quality or token savings. Platform support also depends on the chosen client and runtime.

## Native kits

The separately delivered, hash-verified native launcher runs before unpacking. Windows x64 then uses `system/bin/ocmi.exe`; macOS and Linux use executable `system/bin/ocmi`. Neither the launcher nor a verified ReadyKit requires a compiler, Git, PowerShell, or a PATH-installed OCMI command. Those tools remain development-only requirements for source maintenance. Respect platform protection, executable permissions and absolute paths; see [installation](install-system.md).
