param([string]$Root = (Join-Path $PSScriptRoot '..'))
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'path-utils.ps1')
$repo = (Resolve-Path -LiteralPath $Root).Path
$parent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$temp = Join-Path $parent ('ocmi-boundary-' + [guid]::NewGuid().ToString('N'))
[IO.Directory]::CreateDirectory($temp) | Out-Null
$utf8 = [Text.UTF8Encoding]::new($false)
$passed = 0
function Git-Fixture([string]$Directory,[string[]]$Arguments) {
  & git -C $Directory @Arguments | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Synthetic Git operation failed' }
}
function New-Fixture([string]$Name) {
  $directory = Join-Path $temp $Name
  [IO.Directory]::CreateDirectory((Join-Path $directory 'releases')) | Out-Null
  [IO.Directory]::CreateDirectory((Join-Path $directory '.local/coordinator')) | Out-Null
  [IO.File]::WriteAllText((Join-Path $directory '.gitignore'),"/.local/`n",$utf8)
  [IO.File]::WriteAllText((Join-Path $directory '.gitattributes'),"/.local export-ignore`n",$utf8)
  [IO.File]::WriteAllText((Join-Path $directory 'README.md'),"Synthetic public fixture`n",$utf8)
  [IO.File]::WriteAllText((Join-Path $directory '.local/coordinator/STARTUP.md'),'Synthetic private fixture',$utf8)
  Git-Fixture $directory @('init','--quiet')
  Git-Fixture $directory @('config','user.name','Fixture')
  Git-Fixture $directory @('config','user.email','fixture@example.com')
  Git-Fixture $directory @('config','core.autocrlf','false')
  Git-Fixture $directory @('add','.gitignore','.gitattributes','README.md')
  Git-Fixture $directory @('commit','--quiet','-m','Fixture')
  return $directory
}
function Require-Rejection([string]$Directory,[string]$Expected) {
  $rejected = $false
  try { & (Join-Path $repo 'scripts/check-local-boundary.ps1') -Root $Directory | Out-Null }
  catch { if ($_.Exception.Message -notmatch $Expected) { throw }; $rejected = $true }
  if (!$rejected) { throw 'Private-boundary mutation accepted' }
  $script:passed++
}
try {
  $positive = New-Fixture 'positive with spaces'
  & (Join-Path $repo 'scripts/check-local-boundary.ps1') -Root $positive
  $passed++
  # Export-ignore independently protects an archive even after a force-added private file.
  Git-Fixture $positive @('add','-f','.local/coordinator/STARTUP.md')
  Git-Fixture $positive @('commit','--quiet','-m','Synthetic export fixture')
  $export = Join-Path $temp 'public-export.zip'
  Git-Fixture $positive @('archive','--format=zip',('--output='+$export),'HEAD')
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  $zip = [IO.Compression.ZipFile]::OpenRead($export)
  try {
    if (@($zip.Entries | Where-Object { $_.FullName -match '(^|/)\.local(/|$)' }).Count) { throw 'Private fixture exported' }
    $passed++
  } finally { $zip.Dispose() }
  $directory = New-Fixture 'missing-ignore'
  [IO.File]::WriteAllText((Join-Path $directory '.gitignore'),'',$utf8)
  Require-Rejection $directory 'Missing root .local ignore rule'
  $directory = New-Fixture 'missing-export'
  [IO.File]::WriteAllText((Join-Path $directory '.gitattributes'),'',$utf8)
  Require-Rejection $directory 'Missing .local export exclusion'
  $directory = New-Fixture 'forced-index'
  Git-Fixture $directory @('add','-f','.local/coordinator/STARTUP.md')
  Require-Rejection $directory 'Local memory is tracked'
  $directory = New-Fixture 'private-history'
  Git-Fixture $directory @('add','-f','.local/coordinator/STARTUP.md')
  Git-Fixture $directory @('commit','--quiet','-m','Synthetic history fixture')
  Git-Fixture $directory @('rm','--cached','--quiet','.local/coordinator/STARTUP.md')
  Git-Fixture $directory @('commit','--quiet','-m','Untrack synthetic fixture')
  Require-Rejection $directory 'Local memory path exists in reachable public history'
  foreach ($entry in @('.local/coordinator/STARTUP.md','nested/.LOCAL/coordinator/STARTUP.md')) {
    $directory = New-Fixture ('archive-'+$passed)
    $archive = [IO.Compression.ZipFile]::Open((Join-Path $directory 'releases/fixture.zip'),[IO.Compression.ZipArchiveMode]::Create)
    try { $archive.CreateEntry($entry) | Out-Null } finally { $archive.Dispose() }
    Require-Rejection $directory 'Private local directory included in public archive'
  }
  Write-Output "PASS: $passed synthetic private-boundary checks; ignore/export, forced index, reachable history and root/nested ZIPs."
} finally {
  Assert-OCMITemporaryDirectory $temp $parent 'ocmi-boundary-'
  Remove-Item -LiteralPath $temp -Recurse -Force
}
