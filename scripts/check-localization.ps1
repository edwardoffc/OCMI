param([string]$Root = (Join-Path $PSScriptRoot '..'))
$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path -LiteralPath $Root).Path
function Scoped-File([string]$Relative) {
  if ($Relative -match '^[A-Za-z][A-Za-z0-9+.-]*:|^[/\\]|(^|[/\\])\.\.([/\\]|$)') { throw 'Unsafe translation path' }
  $full = [IO.Path]::GetFullPath((Join-Path $repo $Relative))
  if (!$full.StartsWith($repo + '\', [StringComparison]::OrdinalIgnoreCase) -or !(Test-Path -LiteralPath $full -PathType Leaf)) { throw "Missing/outside translation: $Relative" }
  $item = Get-Item -LiteralPath $full
  while ($item.FullName -ne $repo) {
    if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Reparse translation path' }
    $item = Get-Item -LiteralPath (Split-Path $item.FullName -Parent)
  }
  return $full
}
$manifest = Get-Content -Raw -Encoding UTF8 -LiteralPath (Join-Path $repo 'localization/manifest.json') | ConvertFrom-Json
if ($manifest.canonicalLanguage -cne 'en' -or $manifest.language -cne 'ru' -or $manifest.schemaVersion -ne 1) { throw 'Unsupported localization manifest' }
$expected = @('README.md','BOOTSTRAP.md','VERSION.md','CONTRIBUTING.md','SECURITY.md','CHANGELOG.md','scripts/README.md','releases/README.md')
foreach ($dir in @('templates','docs')) {
  $expected += @(Get-ChildItem -LiteralPath (Join-Path $repo $dir) -Recurse -File -Filter '*.md' |
    Where-Object { $_.FullName -notlike '*\docs\memory\*' -or $_.FullName -like '*\templates\*' } |
    ForEach-Object { $_.FullName.Substring($repo.Length + 1).Replace('\','/') })
}
$seen = @{}
$translated = @()
foreach ($pair in $manifest.files) {
  if ($seen.ContainsKey($pair.source) -or $pair.source -notin $expected) { throw 'Unexpected/duplicate translation source' }
  $target = if ($pair.source -ceq 'README.md') { 'README.ru.md' } else { 'locales/ru/' + $pair.source }
  if ($pair.translation -cne $target) { throw 'Translation mapping differs' }
  foreach ($side in @(@($pair.source,$pair.sourceSha256),@($pair.translation,$pair.translationSha256))) {
    $file = Scoped-File $side[0]
    if ($side[1] -cnotmatch '^[a-f0-9]{64}$' -or (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant() -cne $side[1]) { throw "Stale/unreviewed translation: $($pair.source)" }
  }
  $seen[$pair.source] = $true
  $translated += $pair.translation
}
if ($seen.Count -ne $expected.Count) { throw 'Missing translation record' }
$actual = @('README.ru.md') + @(Get-ChildItem -LiteralPath (Join-Path $repo 'locales/ru') -Recurse -File -Filter '*.md' | ForEach-Object {$_.FullName.Substring($repo.Length+1).Replace('\','/')})
if (@(Compare-Object ($translated | Sort-Object) ($actual | Sort-Object)).Count) { throw 'Translation inventory differs' }
foreach ($profile in @('work','personal','coordinator')) {
  $en = Join-Path $repo "templates/$profile"
  $ru = Join-Path $repo "locales/ru/templates/$profile"
  $enFiles = @(Get-ChildItem -LiteralPath $en -Recurse -File | ForEach-Object {$_.FullName.Substring($en.Length+1).Replace('\','/')} | Sort-Object)
  $ruFiles = @(Get-ChildItem -LiteralPath $ru -Recurse -File | ForEach-Object {$_.FullName.Substring($ru.Length+1).Replace('\','/')} | Sort-Object)
  if (@(Compare-Object $enFiles $ruFiles).Count) { throw 'Template paths differ across languages' }
  foreach ($path in @("$en/LICENSE","$ru/LICENSE")) {
    if ((Get-FileHash -LiteralPath $path).Hash -ne (Get-FileHash -LiteralPath (Join-Path $repo 'LICENSE')).Hash) { throw 'Profile license differs' }
  }
}
Write-Output "PASS: $($seen.Count) reviewed en/ru pairs; exact inventories, hashes, paths, and licenses. Meaning requires human review."
