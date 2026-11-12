# OCMI

**English** · [Русский](README.ru.md)

OCMI 2.0 keeps selected project context in files and provides a native core for
task/resource lifecycle, scoped code context and Git/WIP checkpoint recovery.
A new chat resumes with explicit scope and current handoff. See [installation](docs/install-system.md),
[layout](docs/layout.md), and [usage](docs/usage.md).

## Start with a native kit

1. Obtain or reuse a verified matching ReadyKit; a URL or downloaded source is
   orientation, not an installed source checkout.
2. Verify the standalone launcher and archive hashes against the trusted delivery
   receipt. Use that launcher's `unpack` command to create an absent chosen home,
   then run `system/bin/ocmi[.exe] install --home ABS`; see [installation](docs/install-system.md).
3. The installation coordinator runs `ocmi coordinator prepare --home ABS
   --language en|ru --client-project NAME`, producing the local startup prompt.
4. Create/reuse the client project bound to that home, open a separate
   coordinator chat, and send `local/bindings/coordinator/STARTUP.md`.
5. The coordinator verifies current association and records explicit local
   evidence with `ocmi coordinator confirm`; only then are optional personal
   storage and project integration available.

Opening a link does not install or authorize changes. Preserve occupied homes
and existing operational setups; legacy profiles are compatibility inputs, not
the new-install route. Native public payload is `system/`; `OCMI_Sync` holds
selected portable coordinator data and `local/` is machine-only.

For source development use [CONTRIBUTING](CONTRIBUTING.md) and
[maintenance](docs/maintaining.md). Never publish installed homes or private
state.

### From a URL

Give [the OCMI repository URL](https://github.com/edwardoffc/OCMI) to a chat and ask it to study OCMI. It may help obtain a verified matching ReadyKit; the URL itself is not local execution or installation.

### From a downloaded copy

Open the downloaded source folder in a chat or project and ask it to study OCMI. The source copy can guide acquisition of a matching ReadyKit, but a source ZIP does not become an installation. Both entries continue through the [canonical native guide](docs/install-system.md).

## What OCMI provides

**Orchestration, Context Memory & Indexing** is a file-based workflow for AI
agents. An orchestrator owns the plan and acceptance, a worker completes a
bounded assignment, and a consultant reads without changing files. The usual
cycle is plan → assign → implement → verify → accept → integrate. Memory and a
code map point a new chat to the relevant facts and code instead of claiming to
restore unsaved conversation.

OCMI is not a server, client trust manager, or automatic project scanner.
Written rules do not grant technical access. See [Start](BOOTSTRAP.md),
[version 2.0.0](VERSION.md), [user guide](docs/usage.md), [context](docs/context.md),
[derived continuation](docs/continuation.md),
[installation](docs/install-system.md), [MIT](LICENSE), and [security](SECURITY.md).
