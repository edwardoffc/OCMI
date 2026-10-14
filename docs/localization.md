# Localization

[English](localization.md) · [Русский](../locales/ru/docs/localization.md)

English is the canonical language. Read README.md, docs/, and templates/ for the current
English edition. README.ru.md is the Russian entry point; locales/ru mirrors human guides
and both complete installation profiles.

## Choosing a language

Start with root BOOTSTRAP.md or locales/ru/BOOTSTRAP.md for the temporary installation chat.
COORDINATOR.md has matching editions for the permanent role and filled-prompt handover.
The coordinator profile also has complete editions under templates/coordinator and
locales/ru/templates/coordinator. System VERSION is maintained in both languages;
all current profiles share that release number, including unchanged profiles.

Use templates/work/INSTALL.md or templates/personal/INSTALL.md for English.
For Russian, use the same path beneath locales/ru/. You can also select a language-qualified
[archive](../releases/README.md). Extract the entire profile, including LICENSE.
Choose one language per installation. Installed relative paths and task fields are identical;
no runtime translation, global locale setting, or source-repository dependency is needed.
Agents may communicate in your preferred language regardless of the instruction language.

Do not overlay one language on an active project. Treat a language change as an explicit migration:
preserve local rules, assignments, WIP, and protected evidence; review affected instructions.

## Translation contract

- Keep file names, links within installed profiles, IDs, commands, JSON keys, task headings
  Contract/Result/Acceptance, and machine-readable statuses stable.
- Translate meaning, examples, cautions, and authority boundaries together.
- Do not import permissions or active tasks from another installation.
- Keep one development queue: docs/memory is intentionally English-only.
  Executable scripts, machine-readable receipts, and the MIT license also have one canonical version.
- Historical ZIPs remain in their original language. Do not rewrite an old release to translate it.
  New language editions include a locale in their filename; a translation alone does not change the OCMI release number.

## Maintaining translations

1. Change the English source and its Russian counterpart in the same change.
2. Review semantic equivalence, especially negation, role boundaries, cleanup, and protected files.
3. Update the reviewed source/translation SHA256 pair in localization/manifest.json.
   The manifest is a freshness record, not evidence that two texts mean the same thing.
4. Run the [checks](../scripts/README.md). They reject missing, extra, or stale translations,
   mismatched profile inventories, and broken local links. Workflow guard checks cover both languages.
5. Rebuild changed distributions under new filenames and update their receipt; never overwrite a release.

A new language uses locales/<language>/ with the same guide/template layout and a README link.
Extend the manifest, workflow guard data, and distribution receipt before advertising it as maintained.
Templates are self-contained: put language navigation in repository guides, not installed agent rules.
