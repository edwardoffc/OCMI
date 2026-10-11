# Distributions

[English](README.md) · [Русский](../locales/ru/releases/README.md)

Select a language and extract one complete profile. Current language editions include [MIT](../LICENSE).
Work uses protocol 1.2.2; lightweight uses 1.2.1. Installation does not update existing projects automatically.

System [1.3.1](../VERSION.md) starts with [brief orientation](../BOOTSTRAP.md).
After installation is selected, a full local system copy and coordinator service memory are required.
The ZIP below contains only the service profile; personal topics and work integration are optional.
[System installation](../docs/install-system.md) explains handover to a separate chat.

| Service profile | English | Russian |
| --- | --- | --- |
| Coordinator 1.3.1 | [ZIP](ocmi-coordinator-1.3.1-en-2026-10-12.zip) · [SHA256](ocmi-coordinator-1.3.1-en-2026-10-12.zip.sha256) | [ZIP](ocmi-coordinator-1.3.1-ru-2026-10-12.zip) · [SHA256](ocmi-coordinator-1.3.1-ru-2026-10-12.zip.sha256) |
| Historical 1.3.0 | [ZIP](ocmi-coordinator-1.3.0-en-2026-10-12.zip) · [SHA256](ocmi-coordinator-1.3.0-en-2026-10-12.zip.sha256) | [ZIP](ocmi-coordinator-1.3.0-ru-2026-10-12.zip) · [SHA256](ocmi-coordinator-1.3.0-ru-2026-10-12.zip.sha256) |

[1.3.1 receipt](../docs/releases/1.3.1-validation.json) verifies eight files per language
and all previous archives. [1.3.0 receipt](../docs/releases/1.3.0-validation.json) remains
available for historical ZIPs. Work/lightweight sources and distributions below are unchanged.

| Profile | English | Russian |
| --- | --- | --- |
| Work 1.2.2 | [ZIP](ocmi-work-1.2.2-en-2026-10-11.zip) · [SHA256](ocmi-work-1.2.2-en-2026-10-11.zip.sha256) | [ZIP](ocmi-work-1.2.2-ru-2026-10-11.zip) · [SHA256](ocmi-work-1.2.2-ru-2026-10-11.zip.sha256) |
| Lightweight 1.2.1 | [ZIP](ocmi-personal-1.2.1-en-2026-10-11.zip) · [SHA256](ocmi-personal-1.2.1-en-2026-10-11.zip.sha256) | [ZIP](ocmi-personal-1.2.1-ru-2026-10-11.zip) · [SHA256](ocmi-personal-1.2.1-ru-2026-10-11.zip.sha256) |

[Current security release receipt](../docs/releases/security-validation.json) records exact source/ZIP inventories.
Use scripts/check-release.ps1 from the repository root to verify all four editions.

## Historical archives

The following original Russian releases are immutable, including their receipts.
They are retained for recovery. Historical ZIPs do not contain LICENSE; include the repository's
MIT license when redistributing them. Byte measurements in old reports refer to the original language.

| Version | Archive | SHA256 |
| --- | --- | --- |
| Work 1.2.1, 2026-10-11 | [ZIP](ocmi-1.2.1-2026-10-11.zip) | [Checksum](ocmi-1.2.1-2026-10-11.zip.sha256) |
| OCMI 1, 2026-10-03 | [ZIP](ocmi-1-2026-10-03.zip) | [Checksum](ocmi-1-2026-10-03.zip.sha256) |
| OCMI 1.1, 2026-10-05 | [ZIP](ocmi-1.1-2026-10-05.zip) | [Checksum](ocmi-1.1-2026-10-05.zip.sha256) |
| Work 1.2, 2026-10-11 | [ZIP](ocmi-1.2-2026-10-11.zip) | [Checksum](ocmi-1.2-2026-10-11.zip.sha256) |
| Lightweight 1.2, 2026-10-11 | [ZIP](ocmi-personal-1.2-2026-10-11.zip) | [Checksum](ocmi-personal-1.2-2026-10-11.zip.sha256) |

[1.2.1 report](../docs/releases/1.2.1.md) · [Receipt](../docs/releases/1.2.1-validation.json) ·
[1.2 report](../docs/releases/1.2.md) · [Receipt](../docs/releases/1.2-validation.json).
Use check-release -Version 1.2.1 -ArchivesOnly (or -Version 1.2) for historical verification.
Original commit/tag references may not exist in a standalone public distribution.
ZIPs and receipts provide content recovery without that Git history.
Never overwrite a published archive; use a new edition filename for changed contents.

The [previous language-edition receipt](../docs/releases/languages-validation.json) retains the four original English/Russian 1.2.1/1.2 distributions unchanged.
