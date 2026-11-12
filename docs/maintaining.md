# Maintaining and distributing OCMI

[English](maintaining.md) · [Русский](../locales/ru/docs/maintaining.md)

## Sources and boundaries

This repository is the primary source. English work rules live in templates/work,
the lightweight profile in templates/personal. Russian equivalents live in locales/ru.
Actual plans, task handoffs and installed-project data are private and excluded from the public source.
Existing private development/historical compatibility setup may live in `.local`; source development
must not read it. A new installed home instead has public `system/`, portable private `OCMI_Sync/`,
and machine-only `local/` state. Never share an installed-home ZIP.
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

Root VERSION.md supplies the single OCMI release number for every current profile, including unchanged components.
Record unchanged behavior in CHANGELOG, not an independent component version counter.
2.0.0-workflow-r4.json records the current candidate profiles; the original 2.0.0.json remains immutable.
1.3.5-update.json verifies the historical in-place update;
1.3.5-flow.json retains historical source preparation and 1.3.5.json the initial release; earlier receipts remain immutable.
check-versioning rejects inconsistent labels and separate version fields.
Keep all prior archives/receipts fixed. Original 1.3.3 components use -Version 1.3.3 -LegacyComponents -ArchivesOnly.

Historical setup changes:
System 1.3.2 unifies URL/downloaded startup and requires confirmed client binding before integration.
Runtime work 1.2.2 and lightweight 1.2.1 are unchanged; both INSTALL entry checks require a coordinator. The separate 1.3.2 receipt verifies coordinator
archives; 1.3.2-components.json verifies four updated component editions.
Keep all previous receipts/ZIPs unchanged; use -SecurityBaseline -ArchivesOnly for the old security editions. Use -Version 1.3.0 -ArchivesOnly for old setup ZIPs.
The owner-authorized in-place 1.3.2 correction uses distinct r2 filenames and retained initial receipts;
the 1.3.2 receipts describe r2. Refreshing an existing public release/tag requires explicit authority,
verified original refs, preserved recovery history and renewed source-download verification.
Run check-onboarding alongside existing checks for setup/profile changes. Its negative
mutations verify instruction safeguards, not actual model behavior.

1. Change canonical rules/version and update CHANGELOG/usage when behavior changes.
2. Update affected translations; preserve paths, IDs, schema headings, and machine-readable statuses.
3. Run pwsh -NoProfile -File scripts/check-all.ps1 on Windows/macOS; run the verified native secret scanner before publication.
   Structural checks do not replace [model/cost evaluation](evals/context-recovery.md).
4. Save each changed distribution under a new locale-qualified filename with SHA256 and a receipt.
   Verify exact archive inventory, file sizes, and hashes against the selected language source.
   Never overwrite an old archive. Semantic incompatibility needs an explicit migration.
5. Accept the result, commit, and publish only within current authorization.
   Verify INSTALL availability. Applying changes to installed projects is a separate requested migration,
   preserving local rules and old packages.
6. Record cleanup: done/pending in Acceptance; pending has reason, owner, and next action.
   Release artifacts are permanent; other resources follow their stated purpose.

The OCMI release number and translation language are distinct metadata.
The English 1.2.1 work distribution and English 1.2 lightweight distribution translate existing rules;
they do not migrate projects or claim new behavior.
Original Russian archives remain immutable. Historical reports and byte measurements refer to their original text.
Use check-release -Version 1.2 -ArchivesOnly or -Version 1.2.1 -ArchivesOnly for historical receipts.

Documentation checks do not prove savings. Across accepted/integrated tasks, compare returns,
repeated checks, and available costs in existing Acceptance; do not add a separate metric system.

## Native final-home packages

ReadyKit is the single native package authority. It ships immutable public `system/**`, root declaration/checksums, reviewed notices, and `system/bin/ocmi[.exe]`; never retrofit or overwrite released archives. Consumers verify exact source inventory, root text/ignore rules, notices, checksums, platform binary, and standalone execution. `OCMI_Sync` and `local` are user/runtime state and never enter the kit. A source checkout is development material, not an installation home.

Publish the same native binary as a standalone launcher beside each archive and
`delivery.json`, which binds source/target and both filenames/SHA256 values. Keep
archive-only sums unchanged. The native consumer must run that verified launcher
before extracting the current kit; historical factory fixtures remain pinned.

Release names use the single root `VERSION.md` version: for example,
`ocmi-ready-2.0.0-TARGET.zip` (Windows) or `.tar.gz` (other targets), with the
standalone launcher `ocmi-ready-2.0.0-TARGET-ocmi[.exe]`. The producer adds the
target once. Names identify the version and platform; `delivery.json` and its
SHA256 values bind the exact source commit and delivered bytes. Existing legacy
ReadyKit IDs remain supported. Keep historical archives and receipts immutable;
a release name does not replace exact-source qualification or authorize updates.

For Windows external-tool QA, use an `ownedTemp` prefix of at most 92 UTF-16 code units; the guard refuses longer prefixes.
Keep the original deadlines and actual Go/npm consumers. This is a fixture budget, not a guarantee for arbitrary product/provider paths.
Optional provider failure diagnostics contain only finite flags, counts and lengths; never raw paths or output.

## Compatible installed-home updates

The updater is a separately verified external candidate, not a mutable command
inside the target home. Maintain only the explicit supported identity transition
and the compatibility values in `build-info.updateCompatibility`; do not claim
an arbitrary schema/data migration, update a source checkout, or hot-reload a
running client. The candidate replaces the selected public system payload while
preserving `OCMI_Sync` and `local`; recovery and rollback require the recorded
pending activation or verified previous public generation. See
[installation updates](install-system.md#update-an-installed-home).

Full native workspace qualification also needs an unchanged separate checkout of
`0dc0528a773d9749c6e309111dc995f9bbca4c8f`, supplied to
`scripts/check-runtime-preview.ps1 -WorkspaceLegacyRoot ABS`. CI prepares this
fixture explicitly; it builds and invokes the actual previous reader instead of
inferring compatibility from unchanged schema or relabeled metadata.

Compatibility source refs are immutable test inputs. Select their exact commit
and complete source-tree identities, rather than a moving branch:

| Ref | Commit | Source tree |
| --- | --- | --- |
| `compat/ready-kit-v1` | `436b398960e25e1f0d7568cdbb09e740e8b74f10` | `8a3ec802dee13c7466132225955f2732a30e72fd` |
| `compat/workspace-reader-v1` | `0dc0528a773d9749c6e309111dc995f9bbca4c8f` | `254d440ad0c09d2c97f42e3ab39871b65715ba04` |

Preserve these identities and the actual factory/reader consumers when qualifying
new deliveries; a tag name alone does not replace exact-source verification.
