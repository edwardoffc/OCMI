param(
  [string]$Root = (Join-Path $PSScriptRoot '..'),
  [switch]$ArchivesOnly,
  [switch]$ReleaseOnly
)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.IO.Compression.FileSystem
$repo = (Resolve-Path -LiteralPath $Root).Path
$passed = 0
function Assert-Valid([bool]$Condition, [string]$Message) {
  if (!$Condition) { throw $Message }
}
function Scoped-File([string]$Base, [string]$Relative) {
  Assert-Valid ($Relative -notmatch '^[A-Za-z][A-Za-z0-9+.-]*:|^[/\\]|(^|[/\\])\.\.([/\\]|$)') 'Unsafe input path'
  $full = [IO.Path]::GetFullPath((Join-Path $Base $Relative))
  Assert-Valid ($full.StartsWith($Base.TrimEnd('\','/') + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) 'Outside input root'
  Assert-Valid (Test-Path -LiteralPath $full -PathType Leaf) "Missing input: $Relative"
  $item = Get-Item -LiteralPath $full
  while ($item.FullName -ne $Base) {
    Assert-Valid (!(($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0)) 'Reparse input'
    $item = Get-Item -LiteralPath (Split-Path $item.FullName -Parent)
  }
  return $full
}
function Read-Scoped([string]$Base, [string]$Relative) {
  return [IO.File]::ReadAllText((Scoped-File $Base $Relative))
}
function Check([string]$Name, [scriptblock]$Body) {
  & $Body
  $script:passed++
}
function Must-Reject([scriptblock]$Body) {
  $rejected = $false
  try { & $Body | Out-Null } catch { $rejected = $true }
  Assert-Valid $rejected 'Invalid input was accepted'
}
if (!$ReleaseOnly -and !$ArchivesOnly) {
  $guards = (Read-Scoped $repo 'localization/onboarding-guards.json') | ConvertFrom-Json
  Assert-Valid ($guards.schemaVersion -eq 1) 'Invalid guard schema'
  $entry = Read-Scoped $repo 'AGENTS.md'
  Check 'entry route precedes development' {
    $setup = $entry.IndexOf('[BOOTSTRAP](BOOTSTRAP.md)')
    $development = $entry.IndexOf('# Developing OCMI')
    Assert-Valid ($setup -ge 0 -and $development -gt $setup) 'Missing/late onboarding route'
  }
  foreach ($language in @('en','ru')) {
    $prefix = if ($language -eq 'ru') { 'locales/ru/' } else { '' }
    $base = Join-Path $repo ($prefix + 'templates/coordinator')
    $texts = @{
      bootstrap = (Read-Scoped $repo ($prefix + 'BOOTSTRAP.md'))
      install = (Read-Scoped $base 'INSTALL.md')
      agent = (Read-Scoped $base 'AGENTS.md')
      brief = (Read-Scoped $base 'memory/BRIEF.md')
    }
    Check "$language version agreement" {
      foreach ($relative in @('AGENTS.md','INSTALL.md','VERSION.md')) {
        Assert-Valid ((Read-Scoped $base $relative).Contains('OCMI coordinator 1.3.0')) 'Wrong coordinator version'
      }
      Assert-Valid ((Read-Scoped $repo ($prefix + 'VERSION.md')).Contains('1.3.0')) 'Wrong system version'
    }
    foreach ($guard in $guards.clauses) {
      $text = $texts[$guard.input]
      $pattern = $guard.$language
      Check "$language $($guard.id)" {
        Assert-Valid ($text -match $pattern) "Lost onboarding safeguard: $($guard.id)"
      }
      Check "$language $($guard.id) mutation" {
        $mutated = [regex]::Replace($text, $pattern, '', [Text.RegularExpressions.RegexOptions]::IgnoreCase)
        Must-Reject { Assert-Valid ($mutated -match $pattern) 'Missing safeguard' }
      }
    }
    Check "$language self-contained profile" {
      foreach ($relative in @('memory/INDEX.md','memory/BRIEF.md','memory/tasks/main/HANDOFF.md','VERSION.md')) {
        Scoped-File $base $relative | Out-Null
      }
      Assert-Valid ($texts.agent -notmatch '\]\([^)]*docs/memory|D:\\Projects|C:\\Users') 'Nonportable startup'
      Assert-Valid ((Get-FileHash -LiteralPath (Scoped-File $base 'LICENSE')).Hash -eq (Get-FileHash -LiteralPath (Join-Path $repo 'LICENSE')).Hash) 'License differs'
    }
  }
  foreach ($unsafe in @('../README.md','/README.md','D:/outside.md','missing.md')) {
    Check 'unsafe/missing inputs rejected' { Must-Reject { Read-Scoped (Join-Path $repo 'templates/coordinator') $unsafe } }
  }
}
$receipt = (Read-Scoped $repo 'docs/releases/1.3.0-validation.json') | ConvertFrom-Json
Assert-Valid ($receipt.schemaVersion -eq 1 -and $receipt.version -ceq 'OCMI 1.3.0' -and @($receipt.profiles).Count -eq 2) 'Invalid onboarding receipt'
$languages = @{}
foreach ($profile in $receipt.profiles) {
  Assert-Valid ($profile.profile -ceq 'coordinator' -and $profile.version -ceq '1.3.0' -and $profile.language -cin @('en','ru')) 'Invalid coordinator edition'
  Assert-Valid (!$languages.ContainsKey($profile.language)) 'Duplicate edition'
  $languages[$profile.language] = $true
  Assert-Valid ($profile.archive -cmatch ('^releases/ocmi-coordinator-1\.3\.0-' + $profile.language + '-\d{4}-\d{2}-\d{2}\.zip$')) 'Invalid archive path'
  $prefix = if ($profile.language -eq 'ru') { 'locales/ru/' } else { '' }
  $base = Join-Path $repo ($prefix + 'templates/coordinator')
  $expected = @{}
  foreach ($record in $profile.files) {
    Assert-Valid ($record.path -notmatch '^[A-Za-z][A-Za-z0-9+.-]*:|^[/\\]|(^|[/\\])\.\.([/\\]|$)' -and !$expected.ContainsKey($record.path)) 'Unsafe/duplicate receipt entry'
    Assert-Valid ($record.sha256 -cmatch '^[a-f0-9]{64}$' -and $record.bytes -ge 0) 'Invalid file digest/size'
    if (!$ArchivesOnly) {
      $file = Scoped-File $base $record.path
      Assert-Valid ((Get-FileHash -LiteralPath $file).Hash.ToLowerInvariant() -ceq $record.sha256 -and (Get-Item -LiteralPath $file).Length -eq $record.bytes) 'Coordinator source differs'
    }
    $expected[$record.path] = $record
  }
  if (!$ArchivesOnly) {
    Assert-Valid (@(Get-ChildItem -LiteralPath $base -Recurse -File).Count -eq $expected.Count) 'Coordinator source inventory differs'
  }
  Assert-Valid ($expected.Count -eq 7) 'Coordinator inventory differs'
  $archiveFile = Scoped-File $repo $profile.archive
  Assert-Valid ((Get-FileHash -LiteralPath $archiveFile).Hash.ToLowerInvariant() -ceq $profile.sha256) 'Coordinator archive digest differs'
  $archive = [IO.Compression.ZipFile]::OpenRead($archiveFile)
  try {
    $entries = @($archive.Entries | Where-Object { !$_.FullName.EndsWith('/') })
    Assert-Valid ($entries.Count -eq $expected.Count) 'Coordinator ZIP inventory differs'
    $seen = @{}
    foreach ($entry in $entries) {
      $relative = $entry.FullName.Replace('\','/')
      Assert-Valid ($expected.ContainsKey($relative) -and !$seen.ContainsKey($relative)) 'Unexpected/duplicate ZIP entry'
      $stream = $entry.Open()
      $algorithm = [Security.Cryptography.SHA256]::Create()
      try { $hash = [BitConverter]::ToString($algorithm.ComputeHash($stream)).Replace('-','').ToLowerInvariant() }
      finally { $stream.Dispose(); $algorithm.Dispose() }
      Assert-Valid ($hash -ceq $expected[$relative].sha256 -and $entry.Length -eq $expected[$relative].bytes) 'Coordinator ZIP bytes differ'
      $seen[$relative] = $true
    }
  } finally { $archive.Dispose() }
  Check 'coordinator archive verified' {}
}
foreach ($property in $receipt.historicalArchives.PSObject.Properties) {
  Assert-Valid ($property.Name -cmatch '^ocmi-((work|personal)-)?\d+(\.\d+){0,2}(-(en|ru))?-\d{4}-\d{2}-\d{2}\.zip$') 'Unsafe historical archive name'
  Assert-Valid ((Get-FileHash -LiteralPath (Scoped-File $repo ('releases/' + $property.Name))).Hash.ToLowerInvariant() -ceq $property.Value) 'Historical archive changed'
}
Write-Output "PASS: $passed onboarding checks; two coordinator editions verified; historical archives unchanged. Structural checks do not prove model behavior."
