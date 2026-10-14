param(
  [string]$Root = (Join-Path $PSScriptRoot '..'),
  [ValidatePattern('^(\d+\.\d+(\.\d+)?)?$')][string]$Version = '',
  [switch]$ArchivesOnly,
  [switch]$SecurityBaseline,
  [switch]$LegacyComponents
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'path-utils.ps1')
. (Join-Path $PSScriptRoot 'version-utils.ps1')
Add-Type -AssemblyName System.IO.Compression.FileSystem
$repoRoot = (Resolve-Path -LiteralPath $Root).Path
if ($SecurityBaseline -and (!$ArchivesOnly -or $Version)) { throw 'SecurityBaseline requires ArchivesOnly and no Version' }
if ($LegacyComponents -and (!$ArchivesOnly -or $Version -cne '1.3.3' -or $SecurityBaseline)) {throw 'LegacyComponents requires Version 1.3.3 and ArchivesOnly'}
if ($Version -cin @('1.3.0','1.3.1')) {
  & (Join-Path $PSScriptRoot 'check-onboarding.ps1') -Root $repoRoot -Version $Version -ReleaseOnly -ArchivesOnly:$ArchivesOnly
  return
}
$componentVersion = if ($Version) {$Version} else {'1.3.3'}
$languageEdition = !$Version -or $Version -cin @('1.3.2','1.3.3')
$unified = !$SecurityBaseline -and !$LegacyComponents -and $componentVersion -ceq '1.3.3'
if (!$SecurityBaseline -and $componentVersion -ceq '1.3.2' -and !$ArchivesOnly) {throw 'Historical 1.3.2 components require ArchivesOnly; current source is 1.3.3'}
$receipt = if ($SecurityBaseline) { 'docs/releases/security-validation.json' }
  elseif ($unified) {'docs/releases/1.3.3-flow.json'}
  elseif ($languageEdition) { "docs/releases/$componentVersion-components.json" }
  else { "docs/releases/$Version-validation.json" }
$manifest = Get-Content -Raw -Encoding UTF8 -LiteralPath (Get-OCMIScopedFile $repoRoot $receipt) | ConvertFrom-Json
if ($Version -and $manifest.version -cne "OCMI $Version") { throw 'Receipt version differs' }
if ($languageEdition -and !$SecurityBaseline -and $manifest.version -cne "OCMI $componentVersion") { throw 'Component receipt version differs' }
if ($unified) {Assert-OCMIUnifiedReceipt $manifest (Get-OCMIReleaseVersion $repoRoot)}
elseif ($languageEdition -and ($manifest.schemaVersion -ne 1 -or @($manifest.profiles).Count -ne 4)) { throw 'Invalid language receipt' }
$profileKeys = @{}
foreach ($profile in $manifest.profiles) {
  if ($profile.profile -notin @('work','personal') -and !($unified -and $profile.profile -ceq 'coordinator')) { throw 'Invalid profile' }
  if (!$languageEdition) {
    $prefix = if ($profile.profile -eq 'personal') { 'ocmi-personal-' } else { 'ocmi-' }
    if ($profile.archive -notmatch ('^releases/' + $prefix + '\d+\.\d+(\.\d+)?-\d{4}-\d{2}-\d{2}\.zip$')) { throw 'Invalid archive path' }
    $source = Join-Path $repoRoot ("templates/" + $profile.profile)
    $key = $profile.profile
  } else {
    if ($profile.language -cnotin @('en','ru')) { throw 'Invalid language' }
    $expectedVersion = if ($unified) {$componentVersion} elseif ($profile.profile -eq 'work') {
      if (!$SecurityBaseline -and $componentVersion -ceq '1.3.3') {'1.2.3'} else {'1.2.2'}
    } else {'1.2.1'}
    if (!$unified -and !$SecurityBaseline -and ($manifest.installerVersion -cne '1.3.2' -or $profile.installerVersion -cne '1.3.2')) { throw 'Wrong installer policy revision' }
    if ($profile.version -cne $expectedVersion) { throw 'Invalid language edition version' }
    $archivePattern = '^releases/ocmi-' + $profile.profile + '-' + [regex]::Escape($profile.version) + '-' + $profile.language + '-\d{4}-\d{2}-\d{2}(-r[2-9][0-9]*)?\.zip$'
    if ($profile.archive -cnotmatch $archivePattern) { throw 'Invalid language archive path' }
    $prefix = if ($profile.language -eq 'ru') { 'locales/ru/' } else { '' }
    $source = Join-Path $repoRoot ($prefix + 'templates/' + $profile.profile)
    $key = $profile.profile + '/' + $profile.language
  }
  if ($profileKeys.ContainsKey($key)) { throw 'Duplicate profile/language' }
  $profileKeys[$key] = $true
  $expected = @{}
  foreach ($record in $profile.files) {
    if ($record.path -match '^[A-Za-z][A-Za-z0-9+.-]*:|^[/\\]|(^|[/\\])\.\.([/\\]|$)' -or $expected.ContainsKey($record.path)) { throw 'Unsafe/duplicate manifest entry' }
    $file = if ($ArchivesOnly) { Join-Path $source $record.path } else { Get-OCMIScopedFile $source $record.path }
    if (!$ArchivesOnly -and (!(Test-Path -LiteralPath $file -PathType Leaf) -or (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant() -ne $record.sha256 -or (Get-Item -LiteralPath $file).Length -ne $record.bytes)) { throw "Source mismatch: $($record.path)" }
    $expected[$record.path] = $record
  }
  if (!$ArchivesOnly -and @(Get-ChildItem -LiteralPath $source -Recurse -File).Count -ne $expected.Count) { throw 'Source file inventory differs' }
  $zipPath = Get-OCMIScopedFile $repoRoot $profile.archive
  if ((Get-FileHash -LiteralPath $zipPath -Algorithm SHA256).Hash.ToLowerInvariant() -ne $profile.sha256) { throw 'Archive checksum differs' }
  $archive = [IO.Compression.ZipFile]::OpenRead($zipPath)
  try {
    $entries = @($archive.Entries | Where-Object { !$_.FullName.EndsWith('/') })
    if ($entries.Count -ne $expected.Count) { throw 'Archive file inventory differs' }
    $seen = @{}
    foreach ($entry in $entries) {
      $relative = $entry.FullName.Replace('\','/')
      if (!$expected.ContainsKey($relative) -or $seen.ContainsKey($relative)) { throw "Unexpected/duplicate ZIP entry: $relative" }
      $stream = $entry.Open()
      $algorithm = [Security.Cryptography.SHA256]::Create()
      try { $hash = [BitConverter]::ToString($algorithm.ComputeHash($stream)).Replace('-','').ToLowerInvariant() }
      finally { $stream.Dispose(); $algorithm.Dispose() }
      if ($hash -ne $expected[$relative].sha256 -or $entry.Length -ne $expected[$relative].bytes) { throw "ZIP content mismatch: $relative" }
      $seen[$relative] = $true
    }
  } finally { $archive.Dispose() }
  $scope = if ($ArchivesOnly) { 'receipt and ZIP' } else { 'source, receipt and ZIP' }
  Write-Output "PASS: $($profile.profile) $($expected.Count) files; $scope agree."
}
foreach ($property in $manifest.historicalArchives.PSObject.Properties) {
  if ($property.Name -notmatch '^ocmi-((work|personal|coordinator)-)?\d+(\.\d+){0,2}(-(en|ru))?-\d{4}-\d{2}-\d{2}(-r[2-9][0-9]*)?\.zip$') { throw 'Unexpected historical archive' }
  if ((Get-FileHash -LiteralPath (Join-Path $repoRoot ("releases/" + $property.Name)) -Algorithm SHA256).Hash.ToLowerInvariant() -ne $property.Value) { throw "Historical archive changed: $($property.Name)" }
}
Write-Output 'PASS: historical archives unchanged.'
if ($languageEdition -and !$SecurityBaseline) {
  $coordinatorVersion = if ($unified) {'1.3.3'} else {'1.3.2'}
  & (Join-Path $PSScriptRoot 'check-onboarding.ps1') -Root $repoRoot -Version $coordinatorVersion -ReleaseOnly -ArchivesOnly:$ArchivesOnly
}
