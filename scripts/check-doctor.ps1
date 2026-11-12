param([string]$Root=(Join-Path $PSScriptRoot '..'))
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'doctor-lib.ps1')
$repo=[IO.Path]::GetFullPath($Root)
$parent=[IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$temp=Join-Path $parent ('ocmi-doctor-'+[guid]::NewGuid().ToString('N'))
$base=Join-Path $temp ('work '+[char]0x03A9)
$written=[Collections.Generic.HashSet[string]]::new()
$passed=0
$utf8=[Text.UTF8Encoding]::new($false)
function Require([bool]$Condition,[string]$Message) { if (!$Condition) {throw $Message}; $script:passed++ }
function Put([string]$Path,[string]$Text) {
  $full=Join-Path $base $Path
  [IO.Directory]::CreateDirectory((Split-Path $full -Parent)) | Out-Null
  [IO.File]::WriteAllText($full,$Text,$utf8)
  $written.Add($full) | Out-Null
}
function Put-Block([string]$Path,$Value) { Put $Path ("~~~ocmi-diagnostic`n"+($Value | ConvertTo-Json -Depth 8)+"`n~~~`n") }
function Reset {
  foreach ($path in @('AGENTS.md','VERSION.md','docs/memory/INDEX.md','docs/memory/RULES.md','docs/memory/ORCHESTRATION.md','docs/memory/PLAN.md')) {Put $path '# OCMI 1.3.3'}
  $script:assignment=[ordered]@{schemaVersion=1;kind='assignment';id='A';revision='r1';role='worker';owner='slot1';workingCopy='.';state='current'}
  $script:start=[ordered]@{schemaVersion=1;kind='start';id='A';revision='r1';role='worker';owner='slot1';workingCopy='.';state='ready'}
  $script:a=[ordered]@{schemaVersion=1;kind='task';id='A';revision='r1';owner='slot1';state='ready';dependsOn=@('B');resources=@(@{key='test:port-1';owner='slot1'})}
  $script:b=[ordered]@{schemaVersion=1;kind='task';id='B';revision='r1';owner='slot2';state='integrated';dependsOn=@();resources=@()}
  Put 'runner.ps1' '# Synthetic protected input; never executed'
  $script:request=[ordered]@{schemaVersion=1;assignment='assignment.md';start='START.md';tasks=@('tasks/A.md','tasks/B.md');protected=@(@{path='runner.ps1';sha256=(Get-FileHash -LiteralPath (Join-Path $base 'runner.ps1')).Hash.ToLowerInvariant()})}
  Save
}
function Save {
  Put-Block 'assignment.md' $assignment; Put-Block 'START.md' $start
  Put-Block 'tasks/A.md' $a; Put-Block 'tasks/B.md' $b
  Put 'request.json' ($request | ConvertTo-Json -Depth 8)
}
function Run([switch]$InMemory) {
  $before=@{}
  foreach ($file in $written) { if (Test-Path -LiteralPath $file -PathType Leaf) {$before[$file]=(Get-FileHash -LiteralPath $file).Hash} }
  if ($InMemory) { $report=Invoke-OCMIDoctor -Root $base -Profile work -Selection $request -ExpectedVersion '1.3.3' }
  else { $report=Invoke-OCMIDoctor -Root $base -Profile work -InputFile request.json -ExpectedVersion '1.3.3' }
  foreach ($file in $before.Keys) { if ((Get-FileHash -LiteralPath $file).Hash -cne $before[$file]) {throw 'Doctor changed a fixture'} }
  if ($report.readiness -cne 'not-evaluated') {throw 'Doctor granted readiness'}
  return $report
}
function Has($Report,[string]$Code) { return @($Report.findings | Where-Object code -ceq $Code).Count -gt 0 }
try {
  [IO.Directory]::CreateDirectory($base) | Out-Null
  Reset
  $report=Run
  Require ($report.exitCode -eq 0 -and $report.coverage.files -eq 12) 'Valid assignment path failed'
  Require (@($report.observations | Where-Object path -ceq 'runner.ps1').Count -eq 1) 'Protected input provenance absent'
  Remove-Item -LiteralPath (Join-Path $base 'request.json')
  $memoryReport=Run -InMemory
  Require ($memoryReport.exitCode -eq 0 -and $memoryReport.coverage.files -eq 11 -and (Has $memoryReport SELECTION_IN_MEMORY)) 'In-memory coordinator route failed'
  Require (!(Test-Path -LiteralPath (Join-Path $base 'request.json'))) 'Coordinator route wrote a request file'; Reset
  $report=Invoke-OCMIDoctor -Root $base -Profile work -InputFile 'missing-request.json' -Selection $request
  Require ($report.exitCode -eq 2 -and (Has $report REQUEST_AMBIGUOUS) -and $report.coverage.files -eq 6) 'Conflicting interfaces were followed'
  $report=Invoke-OCMIDoctor -Root $base -Profile work -Selection 'not a plain selection object'
  Require ($report.exitCode -eq 2 -and (Has $report REQUEST_INVALID)) 'Unsupported memory selection accepted'
  $large=@{schemaVersion=1;extra=('x'*262145)}
  Require (Has (Invoke-OCMIDoctor -Root $base -Profile work -Selection $large) REQUEST_INVALID) 'Oversize memory selection accepted'
  Put 'START.md' 'Legacy assignment; review selected canonical sources manually'
  $report=Run -InMemory
  Require ($report.exitCode -eq 2 -and (Has $report BLOCK_REQUIRED)) 'Legacy memory route invented metadata'; Reset
  $start.revision='r0'; Save; Require (Has (Run) IDENTITY_MISMATCH) 'Stale START accepted'; Reset
  $start.role='consultant'; Save; Require (Has (Run) IDENTITY_MISMATCH) 'Wrong role accepted'; Reset
  $start.workingCopy='../other-copy'; Save; Require (Has (Run) COPY_MISMATCH) 'Wrong copy accepted'; Reset
  $start.workingCopy=$base; Save; Require ((Run).exitCode -eq 0) 'Equivalent absolute/relative copy rejected'; Reset
  $a.revision='r2'; Save; Require (Has (Run) TASK_REVISION) 'Task revision mismatch accepted'; Reset
  $a.state='draft'; Save; Require (Has (Run) TASK_STATE) 'Draft task accepted'; Reset
  $assignment.state='historical'; Save; Require (Has (Run) HISTORICAL) 'Historical assignment accepted'; Reset
  $b.state='accepted'; Save; Require (Has (Run) DEPENDENCY_UNMET) 'Acceptance mistaken for integration'; Reset
  $request.tasks=@('tasks/A.md'); Save; Require (Has (Run) DEPENDENCY_MISSING) 'Missing prerequisite guessed complete'; Reset
  $b.dependsOn=@('A'); Save; Require (Has (Run) DEPENDENCY_CYCLE) 'Dependency cycle missed'; Reset
  $b.state='running'; $b.resources=@(@{key='test:port-1';owner='slot2'}); Save
  Require (Has (Run) RESOURCE_CONFLICT) 'Conflicting resource claims accepted'; Reset
  $a.resources=@(@{key='test:port-1';owner='other'}); Save; Require (Has (Run) RESOURCE_INVALID) 'Resource owner mismatch missed'; Reset
  $a.dependsOn=@(); $b.state='running'; $b.resources=@(@{key='test:port-2';owner='slot2'}); Save
  Require ((Run).exitCode -eq 0) 'Independent distinct resources blocked'; Reset
  $request.tasks=@('tasks/A.md','tasks/A.md','tasks/B.md'); Save; Require (Has (Run) TASK_AMBIGUOUS) 'Duplicate task identity accepted'; Reset
  Put 'runner.ps1' '# Changed WIP, preserve it'; Require (Has (Run) HASH_MISMATCH) 'Changed protected input accepted'; Reset
  $request.protected=@(); Save; Require (Has (Run) HASHES_NOT_SELECTED) 'Unchecked hashes called complete'; Reset
  Put 'START.md' 'Legacy free-text START; no structured block'
  $report=Run; Require ($report.exitCode -eq 2 -and (Has $report BLOCK_REQUIRED)) 'Legacy text guessed ready'; Reset
  $block=[IO.File]::ReadAllText((Join-Path $base 'START.md')); Put 'START.md' ($block+$block)
  Require (Has (Run) BLOCK_REQUIRED) 'Multiple metadata blocks accepted'; Reset
  Put 'START.md' "~~~ocmi-diagnostic`n{broken`n~~~"; Require (Has (Run) BLOCK_INVALID) 'Malformed metadata accepted'; Reset
  Put 'START.md' "~~~ocmi-diagnostic`n{`"schemaVersion`":1,`"kind`":`"start`",`"id`":`"A`",`"id`":`"B`"}`n~~~"
  Require (Has (Run) BLOCK_INVALID) 'Duplicate JSON keys accepted'; Reset
  Put 'request.json' '{"schemaVersion":1,"SchemaVersion":1}'
  Require (Has (Run) REQUEST_INVALID) 'Case-ambiguous JSON keys accepted'; Reset
  $block=[IO.File]::ReadAllText((Join-Path $base 'START.md'))
  [IO.File]::WriteAllText((Join-Path $base 'START.md'),$block,[Text.UTF8Encoding]::new($true))
  Require ((Run).exitCode -eq 0) 'UTF-8 BOM rejected'; Reset
  $request.tasks=@(1..33 | ForEach-Object {'tasks/A.md'}); Save
  Require (Has (Run) REQUEST_INVALID) 'Unbounded task selection accepted'; Reset
  Put 'request.json' '{"schemaVersion":1,"tasks":"tasks/A.md","protected":[]}'
  Require (Has (Run) REQUEST_INVALID) 'Unsupported request shape accepted'; Reset
  Put 'AGENTS.md' '# OCMI 1.2.2'; Require (Has (Run) RULE_VERSION_MISMATCH) 'Stale rule version missed'; Reset
  Put 'START.md' ('x'*262145); Require (Has (Run) LIMIT) 'Oversize input called complete'; Reset
  [IO.File]::WriteAllBytes((Join-Path $base 'START.md'),[byte[]]@(255,254))
  Require (Has (Run) INPUT_UNAVAILABLE) 'Invalid UTF-8 accepted'; Reset
  Put 'AGENTS.md' '[Missing](missing.md)'; Require (Has (Run) LINK_UNRESOLVED) 'Broken link missed'; Reset
  Put 'AGENTS.md' "# OCMI 1.3.3`n[External](https://example.org/)"; Require ((Run).exitCode -eq 0) 'External URL fetched or rejected'; Reset
  Put 'AGENTS.md' '[Sibling](../work-other/sentinel.md)'; Require (Has (Run) LINK_UNRESOLVED) 'Sibling link followed'; Reset
  Put '.local/coordinator/private.md' 'Fictional private sentinel'
  foreach ($unsafe in @('../work-other/sentinel.md','.local/coordinator/private.md','C:/outside.md','runner.ps1:stream','%2e%2e/escape.md')) {
    $request.protected=@(@{path=$unsafe;sha256=('0'*64)}); Save
    $report=Run; Require (Has $report INPUT_UNAVAILABLE) 'Unsafe input accepted'
    Require (@($report.observations | Where-Object {$_.path -match 'sentinel|private|outside|escape'}).Count -eq 0) 'Excluded input content read'
    Reset
  }
  $outside=Join-Path $temp 'work-other'; [IO.Directory]::CreateDirectory($outside) | Out-Null
  [IO.File]::WriteAllText((Join-Path $outside 'sentinel.md'),'Fictional linked sentinel',$utf8)
  $link=Join-Path $base 'linked'; $kind=if ([Environment]::OSVersion.Platform -eq [PlatformID]::Win32NT) {'Junction'} else {'SymbolicLink'}
  try {
    New-Item -ItemType $kind -Path $link -Target $outside | Out-Null
    $request.protected=@(@{path='linked/sentinel.md';sha256=('0'*64)}); Save
    $report=Run; Require (Has $report INPUT_UNAVAILABLE) 'Linked external input accepted'
    Require (@($report.observations | Where-Object path -match '^linked/').Count -eq 0) 'Linked content loaded'
  } finally { if (Test-Path -LiteralPath $link) {[IO.Directory]::Delete($link)} }
  Reset
  if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    $request.protected=@($request.protected[0],@{path='RUNNER.ps1';sha256=$request.protected[0].sha256}); Save
    $caseReport=Run
    if ([IO.File]::Exists((Join-Path $base 'RUNNER.ps1'))) {
      # Default macOS volumes can be case-insensitive. Prove that the ordinal cache rereads the alias.
      $alias=@($caseReport.observations | Where-Object {$_.path -ceq 'RUNNER.ps1'})
      Require ($alias.Count -eq 1 -and $alias[0].sha256 -ceq $request.protected[0].sha256 -and !(Has $caseReport INPUT_UNAVAILABLE)) 'Case-alias input was skipped or read from wrong cache key'
    } else {Require (Has $caseReport INPUT_UNAVAILABLE) 'Case-sensitive volume accepted missing path'}
    Reset
  }
  $exe=(Get-Process -Id $PID).Path
  $json=& $exe -NoProfile -File (Join-Path $repo 'scripts/ocmi-doctor.ps1') -Root $base -Profile work -InputFile request.json -Json
  $cliExit=$LASTEXITCODE; $cli=$json -join "`n" | ConvertFrom-Json
  Require ($cliExit -eq 0 -and $cli.readiness -ceq 'not-evaluated') 'Real CLI success contract failed'
  Put 'runner.ps1' '# Changed'; & $exe -NoProfile -File (Join-Path $repo 'scripts/ocmi-doctor.ps1') -Root $base -Profile work -InputFile request.json -Json | Out-Null
  Require ($LASTEXITCODE -eq 1) 'Real CLI issue exit failed'; Reset
  Put 'START.md' 'Legacy'; & $exe -NoProfile -File (Join-Path $repo 'scripts/ocmi-doctor.ps1') -Root $base -Profile work -InputFile request.json -Json | Out-Null
  Require ($LASTEXITCODE -eq 2) 'Real CLI incomplete exit failed'; Reset
  $personal=Join-Path $temp 'personal'; [IO.Directory]::CreateDirectory((Join-Path $personal 'memory')) | Out-Null
  foreach ($path in @('AGENTS.md','memory/INDEX.md','memory/BRIEF.md')) {[IO.File]::WriteAllText((Join-Path $personal $path),'# OCMI 1.3.3',$utf8)}
  Require ((Invoke-OCMIDoctor -Root $personal -Profile personal).exitCode -eq 0) 'Personal layout failed'
  Require ((Invoke-OCMIDoctor -Root $personal -Profile system).exitCode -eq 1) 'Missing system inputs accepted'
  Require ((Invoke-OCMIDoctor -Root $base -Profile work -ExpectedVersion 9.9.9).exitCode -eq 1) 'Version mismatch missed'
  Require ((Invoke-OCMIDoctor -Root '' -Profile work).exitCode -eq 2) 'Empty root implicitly selected current directory'
  $request.protected=@()
  foreach ($i in 1..9) {
    $path="budget/$i.txt"; Put $path ('x'*262144)
    $request.protected+=@{path=$path;sha256=(Get-FileHash -LiteralPath (Join-Path $base $path)).Hash.ToLowerInvariant()}
  }
  Save; $report=Run
  Require ((Has $report LIMIT) -and $report.coverage.bytes -le 2097152) 'Total byte budget exceeded or called complete'
  $routeGuards=[IO.File]::ReadAllText((Get-OCMIScopedFile $repo 'localization/doctor-guards.json')) | ConvertFrom-Json
  if ($routeGuards.schemaVersion -ne 1) {throw 'Unsupported coordinator guard schema'}
  foreach ($language in @('en','ru')) {
    $prefix=if ($language -eq 'ru') {'locales/ru/'} else {''}
    $guide=[IO.File]::ReadAllText((Get-OCMIScopedFile $repo ($prefix+'COORDINATOR.md')))
    $clauses=@($routeGuards.$language)
    function Route-Valid([string]$Text) { $normalized=$Text -replace '\s+',' '; foreach ($clause in $clauses) {if (!$normalized.Contains($clause)) {return $false}}; return $true }
    $guide=$guide -replace '\s+',' '
    Require (Route-Valid $guide) 'Coordinator diagnostic safeguards absent'
    Require (!(Route-Valid ($guide.Replace($clauses[0],'')))) 'Missing user-friendly route accepted'
    Require (!(Route-Valid ($guide.Replace($clauses[2],'')))) 'Missing unverified/authority route accepted'
    $startup=[IO.File]::ReadAllText((Get-OCMIScopedFile $repo ($prefix+'templates/coordinator/STARTUP.md')))
    Require ($startup.Contains('COORDINATOR.md')) 'Restored coordinator bypasses its guide'
  }
  Write-Output "PASS: $passed doctor integration/negative checks; target fingerprints preserved; no readiness authorization."
} finally {
  Assert-OCMITemporaryDirectory $temp $parent 'ocmi-doctor-'
  if (Test-Path -LiteralPath $temp) { Remove-Item -LiteralPath $temp -Recurse -Force }
}
