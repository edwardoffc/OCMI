# Maintaining and distributing OCMI

[English](maintaining.md) · [Русский](../locales/ru/docs/maintaining.md)

## Sources and boundaries

This repository is the primary source. English work rules live in templates/work,
the lightweight profile in templates/personal. Russian equivalents live in locales/ru.
docs/memory is the single active development memory; installed project data stays outside this repository.
Use any separate directory. [Layout](layout.md) · [Localization policy](localization.md).

Install directly from one selected template language.
An exact INSTALL.md path is sufficient; an optional global client rule may point to it,
but is not required. Installed projects do not depend on this repository for routine work.

## Changing rules

Start with an observable failure or unnecessary reading.
Change a shared rule only after verifying that it does not import source-project permissions/models.
Preserve Contract/Result/Acceptance, protected tests, WIP, evidence, and integration.
Use one development PLAN. Put experiment details in the report, not every handoff.
Review translations when English changes. Refresh translation hashes only after reviewing both versions;
a hash does not establish semantic equivalence.

## Releases

1. Change canonical rules/version and update CHANGELOG/usage when behavior changes.
2. Update affected translations; preserve paths, IDs, schema headings, and machine-readable statuses.
3. Run check-docs, check-workflow, check-localization, check-release, and git diff --check.
   Structural checks do not replace [model/cost evaluation](evals/context-recovery.md).
4. Save each changed distribution under a new locale-qualified filename with SHA256 and a receipt.
   Verify exact archive inventory, file sizes, and hashes against the selected language source.
   Never overwrite an old archive. Semantic incompatibility needs an explicit migration.
5. Accept the result, commit, and publish only within current authorization.
   Verify INSTALL availability. Applying changes to installed projects is a separate requested migration,
   preserving local rules and old packages.
6. Record cleanup: done/pending in Acceptance; pending has reason, owner, and next action.
   Release artifacts are permanent; other resources follow their stated purpose.

Protocol versions and localization editions are distinct.
The English 1.2.1 work distribution and English 1.2 lightweight distribution translate existing rules;
they do not migrate projects or claim new behavior.
Original Russian archives remain immutable. Historical reports and byte measurements refer to their original text.
Use check-release -Version 1.2 -ArchivesOnly or -Version 1.2.1 -ArchivesOnly for historical receipts.

Documentation checks do not prove savings. Across accepted/integrated tasks, compare returns,
repeated checks, and available costs in existing Acceptance; do not add a separate metric system.
