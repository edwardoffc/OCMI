# Maintaining and distributing OCMI

[English](maintaining.md) · [Русский](../locales/ru/docs/maintaining.md)

## Sources and boundaries

This repository is the primary source. English work rules live in templates/work,
the lightweight profile in templates/personal. Russian equivalents live in locales/ru.
docs/memory is the single active development memory; installed project data stays outside this repository.
Private coordinator setup lives in .local/coordinator, excluded from Git and public distributions.
Source development must not read .local; updates preserve it. Never share an installed-folder ZIP.
Use any separate directory. [Layout](layout.md) · [Localization policy](localization.md).

After core initialization, the coordinator installs from one selected template language.
An exact INSTALL.md path identifies the component; first use follows BOOTSTRAP and confirmed
client binding/coordinator startup. Installed projects need no repeat core setup for routine work.

## Changing rules

Start with an observable failure or unnecessary reading.
Change a shared rule only after verifying that it does not import source-project permissions/models.
Preserve Contract/Result/Acceptance, protected tests, WIP, evidence, and integration.
Use one development PLAN. Put experiment details in the report, not every handoff.
Review translations when English changes. Refresh translation hashes only after reviewing both versions;
a hash does not establish semantic equivalence.

## Releases

System version is recorded in root VERSION.md; profiles have their own versions.
System 1.3.2 unifies URL/downloaded startup and requires confirmed client binding before integration.
Runtime work 1.2.2 and lightweight 1.2.1 are unchanged; both INSTALL entry checks require a coordinator. The separate 1.3.2 receipt verifies coordinator
archives; 1.3.2-components.json verifies four updated component editions.
Keep all previous receipts/ZIPs unchanged; use -SecurityBaseline -ArchivesOnly for the old security editions. Use -Version 1.3.0 -ArchivesOnly for old setup ZIPs.
The owner-authorized in-place 1.3.2 correction uses distinct r2 filenames and retained initial receipts;
current receipts describe r2. Refreshing an existing public release/tag requires explicit authority,
verified original refs, preserved recovery history and renewed source-download verification.
Run check-onboarding alongside existing checks for setup/profile changes. Its negative
mutations verify instruction safeguards, not actual model behavior.

1. Change canonical rules/version and update CHANGELOG/usage when behavior changes.
2. Update affected translations; preserve paths, IDs, schema headings, and machine-readable statuses.
3. Run check-docs, check-workflow, check-localization, check-release, check-local-boundary, and git diff --check.
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
