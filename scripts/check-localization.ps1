param([string]$Root = (Join-Path $PSScriptRoot '..'))
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'path-utils.ps1')
$repo = (Resolve-Path -LiteralPath $Root).Path
function Scoped-File([string]$Relative) {
  return Get-OCMIScopedFile $repo $Relative
}
$manifest = Get-Content -Raw -Encoding UTF8 -LiteralPath (Join-Path $repo 'localization/manifest.json') | ConvertFrom-Json
if ($manifest.canonicalLanguage -cne 'en' -or $manifest.language -cne 'ru' -or $manifest.schemaVersion -ne 1) { throw 'Unsupported localization manifest' }
$expected = @('README.md','AGENTS.md','BOOTSTRAP.md','COORDINATOR.md','VERSION.md','CONTRIBUTING.md','SECURITY.md','CHANGELOG.md','scripts/README.md','releases/README.md')
$expected += 'packaging/home/text.json'
foreach ($dir in @('templates','docs')) {
  $expected += @(Get-ChildItem -LiteralPath (Join-Path $repo $dir) -Recurse -File -Filter '*.md' |
    ForEach-Object { Get-OCMIRelativeFile $repo $_.FullName } |
    Where-Object { $_ -notlike 'docs/memory/*' })
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
$actual = @('README.ru.md','locales/ru/packaging/home/text.json') + @(Get-ChildItem -LiteralPath (Join-Path $repo 'locales/ru') -Recurse -File -Filter '*.md' | ForEach-Object {$_.FullName.Substring($repo.Length+1).Replace('\','/')})
if (@(Compare-Object ($translated | Sort-Object) ($actual | Sort-Object)).Count) { throw 'Translation inventory differs' }
$textKeys = @('rootReadme','rootAgent','syncReadme','coordinatorAgent','coordinatorIndex','coordinatorHandoff','startup')
foreach ($prefix in @('','locales/ru/')) {
  $texts = Get-Content -LiteralPath (Scoped-File ($prefix+'packaging/home/text.json')) -Raw -Encoding UTF8 | ConvertFrom-Json
  $names = @($texts.PSObject.Properties.Name | Sort-Object)
  $wanted = @(@('schemaVersion') + $textKeys | Sort-Object)
  if ($texts.schemaVersion -ne 1 -or @(Compare-Object $wanted $names -CaseSensitive).Count) { throw 'Invalid native home text schema' }
  foreach ($key in $textKeys) {
    $value = $texts.$key
    if ($value -isnot [string] -or [string]::IsNullOrWhiteSpace($value) -or [Text.Encoding]::UTF8.GetByteCount($value) -gt 32768) { throw 'Invalid native home text value' }
  }
  $startupTokens = @([regex]::Matches($texts.startup,'<[A-Z_]+>') | ForEach-Object {$_.Value} | Sort-Object -Unique)
  $wantedTokens = @('<HOME>','<MEMORY>','<GUIDE>','<PROJECT>','<SOURCE>','<LANGUAGE>' | Sort-Object)
  if (@(Compare-Object $wantedTokens $startupTokens -CaseSensitive).Count) { throw 'Native startup fields differ' }
  $guideTokens = @([regex]::Matches($texts.coordinatorAgent,'<[A-Z_]+>') | ForEach-Object {$_.Value} | Sort-Object -Unique)
  if ($guideTokens.Count -ne 1 -or $guideTokens[0] -cne '<GUIDE_RELATIVE>') { throw 'Native coordinator guide field differs' }
}
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
