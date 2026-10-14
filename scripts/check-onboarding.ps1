param(
  [string]$Root = (Join-Path $PSScriptRoot '..'),
  [ValidateSet('1.3.0','1.3.1','1.3.2','1.3.3')][string]$Version = '1.3.3',
  [switch]$ArchivesOnly,
  [switch]$ReleaseOnly
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'path-utils.ps1')
Add-Type -AssemblyName System.IO.Compression.FileSystem
$repo = (Resolve-Path -LiteralPath $Root).Path
$passed = 0
function Assert-Valid([bool]$Condition, [string]$Message) {
  if (!$Condition) { throw $Message }
}
function Scoped-File([string]$Base, [string]$Relative) {
  return Get-OCMIScopedFile $Base $Relative
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
function Check-Startup([string]$Text) {
  $match = [regex]::Match($Text, '(?s)\x60{3}text\r?\n(?<body>.*?)\x60{3}')
  Assert-Valid $match.Success 'Missing startup prompt fence'
  $body = $match.Groups['body'].Value
  $tokens = @('<SYSTEM_PATH>','<COORDINATOR_MEMORY_PATH>','<SOURCE_REVISION>','<TEMPLATE_LANGUAGE>','<COMMUNICATION_LANGUAGE>','<CLIENT_PROJECT>')
  $found = @([regex]::Matches($body, '<[A-Z_]+>') | ForEach-Object { $_.Value })
  Assert-Valid ($found.Count -eq 6 -and @($found | Select-Object -Unique).Count -eq 6) 'Startup fields missing or duplicated'
  foreach ($token in $tokens) { Assert-Valid ($token -cin $found) 'Unexpected startup field' }
  foreach ($relative in @('COORDINATOR.md','VERSION.md','AGENTS.md','memory/INDEX.md','memory/BRIEF.md','memory/tasks/main/HANDOFF.md','STARTUP.md')) {
    Assert-Valid $body.Contains($relative) 'Missing restoration input'
  }
}
if (!$ReleaseOnly -and !$ArchivesOnly) {
  Assert-Valid ($Version -ceq '1.3.3') 'Old instructions require ArchivesOnly verification'
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
      routing = (Read-Scoped $repo ($prefix + 'docs/work-project-routing.md'))
      workInstall = (Read-Scoped $repo ($prefix + 'templates/work/INSTALL.md'))
      personalInstall = (Read-Scoped $repo ($prefix + 'templates/personal/INSTALL.md'))
      entry = $entry
      readme = (Read-Scoped $repo $(if ($language -eq 'ru') { 'README.ru.md' } else { 'README.md' }))
      coordinator = (Read-Scoped $repo ($prefix + 'COORDINATOR.md'))
      startup = (Read-Scoped $base 'STARTUP.md')
      bootstrap = (Read-Scoped $repo ($prefix + 'BOOTSTRAP.md'))
      install = (Read-Scoped $base 'INSTALL.md')
      agent = (Read-Scoped $base 'AGENTS.md')
      brief = (Read-Scoped $base 'memory/BRIEF.md')
    }
    Check "$language version agreement" {
      foreach ($relative in @('AGENTS.md','INSTALL.md','VERSION.md')) {
        Assert-Valid ((Read-Scoped $base $relative).Contains("OCMI $Version")) 'Wrong coordinator version'
      }
      Assert-Valid ((Read-Scoped $repo ($prefix + 'VERSION.md')).Contains($Version)) 'Wrong system version'
    }
    Check "$language startup fields and restoration inputs" { Check-Startup $texts.startup }
    foreach ($token in @('<SYSTEM_PATH>','<COORDINATOR_MEMORY_PATH>','<CLIENT_PROJECT>')) {
      Check "$language startup field mutation" {
        Must-Reject { Check-Startup ($texts.startup.Replace($token,'')) }
      }
    }
    Check "$language coordinator route" {
      Assert-Valid ($entry.Contains('[COORDINATOR](COORDINATOR.md)')) 'Missing permanent role route'
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
$receiptPath = if ($Version -ceq '1.3.3') {'docs/releases/1.3.3-flow.json'} else {"docs/releases/$Version-validation.json"}
$receipt = (Read-Scoped $repo $receiptPath) | ConvertFrom-Json
if ($Version -ceq '1.3.3') {
  Assert-Valid ($receipt.schemaVersion -eq 2 -and $receipt.numbering -ceq 'unified' -and @($receipt.profiles).Count -eq 6) 'Invalid unified receipt'
  $profiles = @($receipt.profiles | Where-Object {$_.profile -ceq 'coordinator'})
} else {
  Assert-Valid ($receipt.schemaVersion -eq 1) 'Invalid historical onboarding schema'
  $profiles = @($receipt.profiles)
}
Assert-Valid ($receipt.version -ceq "OCMI $Version" -and $profiles.Count -eq 2) 'Invalid onboarding receipt'
$languages = @{}
foreach ($profile in $profiles) {
  Assert-Valid ($profile.profile -ceq 'coordinator' -and $profile.version -ceq $Version -and $profile.language -cin @('en','ru')) 'Invalid coordinator edition'
  Assert-Valid (!$languages.ContainsKey($profile.language)) 'Duplicate edition'
  $languages[$profile.language] = $true
  Assert-Valid ($profile.archive -cmatch (('^releases/ocmi-coordinator-' + [regex]::Escape($Version) + '-') + $profile.language + '-\d{4}-\d{2}-\d{2}(-r[2-9][0-9]*)?\.zip$')) 'Invalid archive path'
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
  Assert-Valid ($expected.Count -eq $(if ($Version -eq '1.3.0') { 7 } else { 8 })) 'Coordinator inventory differs'
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
  Assert-Valid ($property.Name -cmatch '^ocmi-((work|personal|coordinator)-)?\d+(\.\d+){0,2}(-(en|ru))?-\d{4}-\d{2}-\d{2}(-r[2-9][0-9]*)?\.zip$') 'Unsafe historical archive name'
  Assert-Valid ((Get-FileHash -LiteralPath (Scoped-File $repo ('releases/' + $property.Name))).Hash.ToLowerInvariant() -ceq $property.Value) 'Historical archive changed'
}
Write-Output "PASS: $passed onboarding checks; two coordinator editions verified; historical archives unchanged. Structural checks do not prove model behavior."
