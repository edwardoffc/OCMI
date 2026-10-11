# Установочные комплекты

[English](../../../releases/README.md) · **Русский**

Выберите язык и распакуйте профиль целиком. Новые языковые комплекты содержат [MIT](../../../LICENSE).
Рабочий протокол — 1.2.1, облегчённый — 1.2. Установленные проекты автоматически не обновляются.

| Профиль | Английский | Русский |
| --- | --- | --- |
| Рабочий 1.2.1 | [ZIP](../../../releases/ocmi-work-1.2.1-en-2026-10-11.zip) · [SHA256](../../../releases/ocmi-work-1.2.1-en-2026-10-11.zip.sha256) | [ZIP](../../../releases/ocmi-work-1.2.1-ru-2026-10-11.zip) · [SHA256](../../../releases/ocmi-work-1.2.1-ru-2026-10-11.zip.sha256) |
| Облегчённый 1.2 | [ZIP](../../../releases/ocmi-personal-1.2-en-2026-10-11.zip) · [SHA256](../../../releases/ocmi-personal-1.2-en-2026-10-11.zip.sha256) | [ZIP](../../../releases/ocmi-personal-1.2-ru-2026-10-11.zip) · [SHA256](../../../releases/ocmi-personal-1.2-ru-2026-10-11.zip.sha256) |

[Receipt языковых редакций](../../../docs/releases/languages-validation.json) содержит точный состав исходников/ZIP.
Для сверки четырёх комплектов запустите scripts/check-release.ps1 из корня репозитория.

## Исторические архивы

Исходные русские выпуски и receipts неизменяемы и доступны для восстановления.
В старых ZIP нет LICENSE; при распространении приложите MIT из репозитория.
Измерения байтов старых отчётов относятся к первоначальному языку.

| Версия | Архив | SHA256 |
| --- | --- | --- |
| Рабочий 1.2.1, 2026-10-11 | [ZIP](../../../releases/ocmi-1.2.1-2026-10-11.zip) | [Checksum](../../../releases/ocmi-1.2.1-2026-10-11.zip.sha256) |
| OCMI 1, 2026-10-03 | [ZIP](../../../releases/ocmi-1-2026-10-03.zip) | [Checksum](../../../releases/ocmi-1-2026-10-03.zip.sha256) |
| OCMI 1.1, 2026-10-05 | [ZIP](../../../releases/ocmi-1.1-2026-10-05.zip) | [Checksum](../../../releases/ocmi-1.1-2026-10-05.zip.sha256) |
| Рабочий 1.2, 2026-10-11 | [ZIP](../../../releases/ocmi-1.2-2026-10-11.zip) | [Checksum](../../../releases/ocmi-1.2-2026-10-11.zip.sha256) |
| Облегчённый 1.2, 2026-10-11 | [ZIP](../../../releases/ocmi-personal-1.2-2026-10-11.zip) | [Checksum](../../../releases/ocmi-personal-1.2-2026-10-11.zip.sha256) |

[Отчёт 1.2.1](../docs/releases/1.2.1.md) · [Receipt](../../../docs/releases/1.2.1-validation.json) ·
[Отчёт 1.2](../docs/releases/1.2.md) · [Receipt](../../../docs/releases/1.2-validation.json).
Для исторических проверок используйте check-release -Version 1.2.1 -ArchivesOnly (либо -Version 1.2).
Первоначальные commit/tag могут отсутствовать в отдельной публичной поставке.
ZIP и receipts позволяют восстановить содержимое без этой Git-истории.
Не перезаписывайте опубликованный архив; изменённое содержимое получает отдельное имя редакции.
