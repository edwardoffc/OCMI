param([string]$Root = (Join-Path $PSScriptRoot '..'))
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'path-utils.ps1')
$repo = (Resolve-Path -LiteralPath $Root).Path
Add-Type -AssemblyName System.IO.Compression.FileSystem
$ignore = [IO.File]::ReadAllText((Join-Path $repo '.gitignore'))
$attributes = [IO.File]::ReadAllText((Join-Path $repo '.gitattributes'))
if ($ignore -notmatch '(?m)^/\.local/\s*$') { throw 'Missing root .local ignore rule' }
if ($attributes -notmatch '(?m)^/\.local\s+export-ignore\s*$') { throw 'Missing .local export exclusion' }
$tracked = @(git -C $repo -c core.quotepath=false ls-files)
if ($LASTEXITCODE -ne 0) { throw 'Cannot enumerate public Git index' }
if (@($tracked | Where-Object { $_ -match '(^|/)\.local(/|$)' }).Count) { throw 'Local memory is tracked; ignoring does not untrack it' }
$ignored = git -C $repo check-ignore --no-index -- .local/coordinator/STARTUP.md
if ($LASTEXITCODE -ne 0 -or !$ignored) { throw 'Local coordinator path is not effectively ignored' }
$export = git -C $repo check-attr export-ignore -- .local
if ($LASTEXITCODE -ne 0 -or $export -notmatch ': export-ignore: set$') { throw 'Local export exclusion is ineffective' }
$history = @(git -C $repo -c core.quotepath=false log --format= --name-only HEAD)
if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect reachable public history' }
if (@($history | Where-Object { $_ -match '(^|/)\.local(/|$)' }).Count) { throw 'Local memory path exists in reachable public history' }
$archives = 0
foreach ($file in Get-ChildItem -LiteralPath (Join-Path $repo 'releases') -File -Filter '*.zip') {
  $archive = [IO.Compression.ZipFile]::OpenRead($file.FullName)
  try {
    if ($archive.Entries.Count -gt 5000) { throw 'Archive boundary inventory limit' }
    foreach ($entry in $archive.Entries) {
      $path = $entry.FullName.Replace('\','/')
      if ($path -match '(^|/)\.local(/|$)') { throw 'Private local directory included in public archive' }
    }
    $archives++
  } finally { $archive.Dispose() }
}
Write-Output "PASS: .local ignored/export-excluded; no private paths in index/history or $archives public ZIPs. Private memory was not read."
