param([string]$Root = (Join-Path $PSScriptRoot '..'))
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'path-utils.ps1')
$repo = (Resolve-Path -LiteralPath $Root).Path
$parent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$temp = Join-Path $parent ('ocmi-platform-' + [guid]::NewGuid().ToString('N'))
[IO.Directory]::CreateDirectory($temp) | Out-Null
$utf8 = [Text.UTF8Encoding]::new($false)
$passed = 0
function Require([bool]$Condition,[string]$Message) { if (!$Condition) { throw $Message }; $script:passed++ }
function Reject([scriptblock]$Body) {
  $rejected = $false
  try { & $Body | Out-Null } catch { $rejected = $true }
  Require $rejected 'Unsafe fixture accepted'
}
try {
  $base = Join-Path $temp 'workspace with spaces'
  $outside = Join-Path $temp 'workspace with spaces-other'
  [IO.Directory]::CreateDirectory($base) | Out-Null
  [IO.Directory]::CreateDirectory($outside) | Out-Null
  $unicode = 'notes-' + [char]0x03A9 + '-' + [char]0x0416 + '.md'
  $file = Join-Path $base $unicode
  [IO.File]::WriteAllText($file,'Synthetic Unicode fixture',$utf8)
  Require ((Get-OCMIScopedFile $base $unicode) -ceq $file) 'Spaced/Unicode file resolution failed'
  Require ((Get-OCMIRelativeFile $base $file) -ceq $unicode) 'Portable relative path differs'
  Require (Test-OCMIContainedPath $base $file) 'Safe path rejected'
  Require (!(Test-OCMIContainedPath $base (Join-Path $outside $unicode))) 'Sibling prefix accepted'
  Require (!(Test-OCMIContainedPath $base $base)) 'Root accepted as child'
  Require (Test-OCMIContainedPath ($base+[IO.Path]::DirectorySeparatorChar) $file) 'Trailing separator rejected'
  foreach ($unsafe in @('../escape.md','/escape.md','\escape.md','C:/escape.md','a/../../escape.md','file.md:stream','missing.md')) {
    Reject { Get-OCMIScopedFile $base $unsafe }
  }
  if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    Require (!(Test-OCMIContainedPath $base (Join-Path $base.ToUpperInvariant() $unicode))) 'Unix root case boundary ignored'
  }
  $linkedFile = Join-Path $outside 'outside.md'
  [IO.File]::WriteAllText($linkedFile,'Synthetic external fixture',$utf8)
  $link = Join-Path $base 'linked'
  $kind = if ([Environment]::OSVersion.Platform -eq [PlatformID]::Win32NT) {'Junction'} else {'SymbolicLink'}
  New-Item -ItemType $kind -Path $link -Target $outside | Out-Null
  Reject { Get-OCMIScopedFile $base 'linked/outside.md' }
  [IO.Directory]::Delete($link)
  Require (Test-Path -LiteralPath $linkedFile) 'Link cleanup removed its target'
  Assert-OCMITemporaryDirectory $temp $parent 'ocmi-platform-'
  $passed++
  Reject { Assert-OCMITemporaryDirectory $base $parent 'ocmi-platform-' }
  Reject { Assert-OCMITemporaryDirectory $temp $outside 'ocmi-platform-' }
  $checkout = Join-Path $temp ('public checkout ' + [char]0x03A9)
  [IO.Directory]::CreateDirectory($checkout) | Out-Null
  $files = @(git -C $repo -c core.quotepath=false ls-files)
  if ($LASTEXITCODE -ne 0 -or !$files.Count) { throw 'Public fixture inventory unavailable' }
  foreach ($relative in $files) {
    if ($relative -match '(^|/)\.local(/|$)|^\.codex/') { throw 'Private path in public fixture inventory' }
    $source = Get-OCMIScopedFile $repo $relative
    $destination = Join-Path $checkout $relative
    [IO.Directory]::CreateDirectory((Split-Path $destination -Parent)) | Out-Null
    [IO.File]::Copy($source,$destination)
  }
  & (Join-Path $checkout 'scripts/check-localization.ps1') -Root $checkout
  $passed++
  & (Join-Path $checkout 'scripts/check-docs.ps1') -Root $checkout
  $passed++
  Write-Output "PASS: $passed platform checks; spaces/Unicode, scope/case, link rejection, cleanup and actual copied consumers."
} finally {
  Assert-OCMITemporaryDirectory $temp $parent 'ocmi-platform-'
  Remove-Item -LiteralPath $temp -Recurse -Force
}
