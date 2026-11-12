# OCMI layout

[English](layout.md) · [Русский](../locales/ru/docs/layout.md)

Source development and an installed native home are separate trees.

~~~text
source-checkout/                 development only
  docs/ templates/ runtime/ scripts/

00_OCMI/                          verified unpacked native home
  .ocmi-home.json  SHA256SUMS
  system/                         immutable public system and bin/ocmi[.exe]
  OCMI_Sync/
    coordinator/                  selected flat semantic files
    personal/                     absent until explicitly selected
    recovery/                     absent until selected recovery
  local/
    state/ bindings/ cache/ runs/ machine-only state
~~~

`system/**` is verified public payload. `OCMI_Sync/coordinator` holds only
portable semantic inputs; it has no device binding or confirmation. `local/**`
is excluded from public package verification and sharing. Do not convert a
source checkout into a home, recursively restore a whole home, or migrate an
occupied setup without an explicit route.
