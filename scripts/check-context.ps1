param([string]$Root=(Join-Path $PSScriptRoot '..'))
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'context-lib.ps1')
. (Join-Path $PSScriptRoot 'path-utils.ps1')
$repo=[IO.Path]::GetFullPath($Root);$passed=0
function Require([bool]$Condition,[string]$Message) {if (!$Condition) {throw $Message};$script:passed++}
function Reject([scriptblock]$Body) {$rejected=$false;try {& $Body | Out-Null} catch {$rejected=$true};Require $rejected 'Unsafe/incomplete input accepted'}
$parent=[IO.Path]::GetFullPath([IO.Path]::GetTempPath())
# macOS /var is commonly a system symlink; fixtures use its physical /private/var spelling.
if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT -and $parent.StartsWith('/var/') -and [IO.Directory]::Exists('/private'+$parent)) {$parent='/private'+$parent}
$temp=Join-Path $parent ('ocmi-context-'+[guid]::NewGuid().ToString('N'))
$utf8=[Text.UTF8Encoding]::new($false)
function Put([string]$Relative,[string]$Body) {
  $destination=Join-Path $temp $Relative
  [IO.Directory]::CreateDirectory((Split-Path $destination -Parent)) | Out-Null
  [IO.File]::WriteAllText($destination,$Body,$utf8)
}
try {
  Put 'contract.md' 'Required rule: Charge rejects negative amounts.'
  Put 'src/pay.go' ("package pay`nfunc Charge(amount int) int { return amount }`n"+('// unrelated filler' + "`n")*200)
  Put 'src/api.ts' "export function submit() { return Charge(10); }`n"
  Put 'tests/pay_test.go' "package pay`nfunc TestCharge() { Charge(-1) }`n"
  Put 'src/unused.py' (('# unrelated module'+"`n")*150)
  Put 'src/outline.ps1' "function Invoke-Charge {`n Charge 10`n}`n"
  $files=@('src/pay.go','src/api.ts','tests/pay_test.go','src/unused.py','src/outline.ps1')
  $params=@{Root=$temp;Files=$files;RequiredFiles=@('contract.md');Terms=@('Charge');SourceRevision='fixture-1'}
  $before=@{};foreach ($file in $files+@('contract.md')) {$before[$file]=(Get-FileHash -LiteralPath (Join-Path $temp $file)).Hash}
  $report=Invoke-OCMIContext @params
  Require ($report.status -ceq 'selected' -and $report.completeness -ceq 'not-established' -and $report.readiness -ceq 'not-evaluated') 'Scoped retrieval gained completeness/readiness'
  foreach ($path in @('src/pay.go','src/api.ts','tests/pay_test.go','contract.md')) {Require ($path -cin @($report.selected.path)) 'Lost definition/caller/test/contract'}
  Require ($report.selected[0].kind -ceq 'required' -and $report.selected[0].text -ceq 'Required rule: Charge rejects negative amounts.') 'Required input truncated or ranked below optional code'
  Require ('src/unused.py' -cnotin @($report.selected.path)) 'Unrelated body loaded into context'
  Require (@($report.outline | Where-Object {$_.name -ceq 'Charge' -and $_.line -eq 2}).Count -eq 1) 'Go definition candidate missing'
  Require (($report.provenance.files | Where-Object path -EQ 'src/outline.ps1').provider -ceq 'powershell-ast') 'PowerShell did not use actual AST'
  Require ($report.metrics.snippetChars -lt $report.metrics.sourceChars/3) 'Fixture context not reduced versus full-read baseline'
  $reportChars=($report | ConvertTo-Json -Depth 10 -Compress).Length
  Require ($reportChars -lt $report.metrics.sourceChars) 'Report metadata erased fixture context reduction'
  $second=Invoke-OCMIContext @params
  Require ($second.cache.reused -eq 6 -and $second.cache.parsed -eq 0 -and $second.metrics.sourceBytesRead -eq $report.metrics.sourceBytesRead) 'Warm cache skipped content verification or reparsed everything'
  $mtime=(Get-Item -LiteralPath (Join-Path $temp 'src/pay.go')).LastWriteTimeUtc
  Put 'src/pay.go' "package pay`nfunc ChargeNew(amount int) int { return amount }`n"
  (Get-Item -LiteralPath (Join-Path $temp 'src/pay.go')).LastWriteTimeUtc=$mtime
  $changed=Invoke-OCMIContext @params
  Require ($changed.cache.parsed -eq 1 -and $changed.cache.reused -eq 5 -and @($changed.outline | Where-Object name -CEQ 'Charge').Count -eq 0) 'Dirty content with preserved mtime reused stale symbol'
  Require (($changed.provenance.files | Where-Object path -EQ 'src/pay.go').sha256 -cne ($report.provenance.files | Where-Object path -EQ 'src/pay.go').sha256) 'Working-copy fingerprint unchanged'
  Move-Item -LiteralPath (Join-Path $temp 'src/pay.go') -Destination (Join-Path $temp 'src/renamed.go')
  Reject {Invoke-OCMIContext @params}
  $params.Files=@('src/renamed.go','src/api.ts','tests/pay_test.go','src/outline.ps1')
  $renamed=Invoke-OCMIContext @params
  Require ($renamed.cache.dropped -eq 2 -and 'src/pay.go' -cnotin @($renamed.provenance.files.path) -and 'src/unused.py' -cnotin @($renamed.provenance.files.path)) 'Renamed/deleted scope entries retained'
  $params.SourceRevision='fixture-2';$revision=Invoke-OCMIContext @params
  Require ($revision.cache.reused -eq 0 -and $revision.cache.parsed -eq 5) 'Revision change reused prior index'
  $script:OCMIContextEngine+='-fixture';$engine=Invoke-OCMIContext @params
  Require ($engine.cache.reused -eq 0) 'Engine version change reused prior index'
  Clear-OCMIContextCache
  $cleared=Invoke-OCMIContext @params
  Require ($cleared.cache.reused -eq 0 -and $cleared.cache.parsed -eq 5) 'Explicit cache reset retained index'
  Put 'deleted.ts' 'Charge(1)'
  $null=Invoke-OCMIContext -Root $temp -Files deleted.ts -Terms Charge
  Remove-Item -LiteralPath (Join-Path $temp 'deleted.ts')
  Reject {Invoke-OCMIContext -Root $temp -Files deleted.ts -Terms Charge}
  Put 'big-contract.md' ('x'*300)
  $small=Invoke-OCMIContext -Root $temp -Files src/api.ts -RequiredFiles big-contract.md -Terms Charge -BudgetChars 128
  Require ($small.status -ceq 'incomplete' -and @($small.omitted | Where-Object kind -CEQ 'required').Count -eq 1 -and $small.metrics.snippetChars -le 128) 'Required budget overrun hidden'
  $none=Invoke-OCMIContext -Root $temp -Files src/api.ts -Terms NoSuchIdentifier
  Require ($none.status -ceq 'incomplete' -and $none.completeness -ceq 'not-established') 'No lexical hit claimed absence'
  Put 'broken.ps1' 'function Broken { Charge('
  $broken=Invoke-OCMIContext -Root $temp -Files broken.ps1 -Terms Charge
  Require ($broken.status -ceq 'incomplete' -and $broken.notices[0].reason -ceq 'parse-error-lexical-fallback') 'Invalid AST silently treated as semantic proof'
  foreach ($unsafe in @('../sibling.md','/escape.md','file.md:stream','.local/coordinator/HANDOFF.md','.codex/secret.md','node_modules/x.ts','src/../contract.md','src/./api.ts','src//api.ts','src/missing.ts','secret.pem')) {
    Reject {Invoke-OCMIContext -Root $temp -Files $unsafe -Terms Charge}
  }
  Reject {Invoke-OCMIContext -Root '.' -Files src/api.ts -Terms Charge}
  Reject {Invoke-OCMIContext -Root $temp -Files src/api.ts -Terms '.*'}
  Reject {Invoke-OCMIContext -Root $temp -Files (@('src/api.ts')*129) -Terms Charge}
  Put 'oversize.ts' ('x'*131073)
  Reject {Invoke-OCMIContext -Root $temp -Files oversize.ts -Terms Charge}
  [IO.File]::WriteAllBytes((Join-Path $temp 'invalid.ts'),[byte[]]@(255,255))
  Reject {Invoke-OCMIContext -Root $temp -Files invalid.ts -Terms Charge}
  Put 'binary.ts' ([string][char]0)
  Reject {Invoke-OCMIContext -Root $temp -Files binary.ts -Terms Charge}
  Put 'too-many-lines.ts' ("`n"*16385)
  Reject {Invoke-OCMIContext -Root $temp -Files too-many-lines.ts -Terms Charge}
  $largeFiles=@()
  foreach ($i in 1..17) {$name='large-'+$i+'.ts';Put $name ('x'*131072);$largeFiles+=$name}
  Reject {Invoke-OCMIContext -Root $temp -Files $largeFiles -Terms Charge}
  Put 'many-windows.ts' (("Charge(1)`n"+("// filler`n"*10))*513)
  $limited=Invoke-OCMIContext -Root $temp -Files many-windows.ts -Terms Charge
  Require ($limited.status -ceq 'incomplete' -and 'reference-window-limit' -cin @($limited.notices.reason)) 'Reference cap silently omitted evidence'
  $outside=Join-Path $parent ('ocmi-context-'+[guid]::NewGuid().ToString('N'))
  [IO.Directory]::CreateDirectory($outside) | Out-Null
  [IO.File]::WriteAllText((Join-Path $outside 'x.ts'),'Charge(1)',$utf8)
  $link=Join-Path $temp 'linked'
  try {
    Put 'x.ts' 'Charge(1)'
    $null=Invoke-OCMIContext -Root $temp -Files x.ts -Terms Charge -SourceRevision same
    $otherRoot=Invoke-OCMIContext -Root $outside -Files x.ts -Terms Charge -SourceRevision same
    Require ($otherRoot.cache.reused -eq 0 -and $otherRoot.cache.parsed -eq 1) 'Same-name/content cache crossed project roots'
    $kind=if ([Environment]::OSVersion.Platform -eq [PlatformID]::Win32NT) {'Junction'} else {'SymbolicLink'}
    New-Item -ItemType $kind -Path $link -Target $outside | Out-Null
    Reject {Invoke-OCMIContext -Root $temp -Files linked/x.ts -Terms Charge}
    Reject {Invoke-OCMIContext -Root $link -Files x.ts -Terms Charge}
    Reject {Invoke-OCMIContext -Root (Join-Path $link 'nested') -Files x.ts -Terms Charge}
    [IO.Directory]::Delete($link)
    Require (Test-Path -LiteralPath (Join-Path $outside 'x.ts')) 'Junction cleanup touched target'
  } finally {
    if (Test-Path -LiteralPath $link) {[IO.Directory]::Delete($link)}
    Assert-OCMITemporaryDirectory $outside $parent 'ocmi-context-'
    Remove-Item -LiteralPath $outside -Recurse -Force
  }
  Put ('src/name '+[char]0x03A9+'.ts') 'Charge(2)'
  $unicode=Invoke-OCMIContext -Root $temp -Files ('src/name '+[char]0x03A9+'.ts') -Terms Charge
  Require ($unicode.selected.Count -eq 1) 'Unicode/spaces scope failed'
  if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    Put 'src/Case.ts' 'Charge(3)'
    $caseAlias=[IO.File]::Exists((Join-Path $temp 'src/case.ts'))
    if (!$caseAlias) {Put 'src/case.ts' 'Charge(4)'}
    $case=Invoke-OCMIContext -Root $temp -Files src/Case.ts,src/case.ts -Terms Charge
    $sameHash=$case.provenance.files[0].sha256 -ceq $case.provenance.files[1].sha256
    Require ($case.metrics.filesRead -eq 2 -and $case.selected.Count -eq 2 -and $sameHash -eq $caseAlias) 'Native volume case handling/cache scope differs'
  }
  foreach ($path in @('context-lib.ps1','ocmi-context.ps1')) {
    Require ((Get-FileHash -LiteralPath (Join-Path $repo ('scripts/'+$path))).Hash -ceq (Get-FileHash -LiteralPath (Join-Path $repo ('templates/work/tools/'+$path))).Hash) 'Installed English helper drift'
    Require ((Get-FileHash -LiteralPath (Join-Path $repo ('scripts/'+$path))).Hash -ceq (Get-FileHash -LiteralPath (Join-Path $repo ('locales/ru/templates/work/tools/'+$path))).Hash) 'Installed Russian helper drift'
  }
  foreach ($file in @('src/api.ts','tests/pay_test.go','src/unused.py','src/outline.ps1','contract.md')) {Require ((Get-FileHash -LiteralPath (Join-Path $temp $file)).Hash -ceq $before[$file]) 'Retrieval changed input'}
  $hostExe=(Get-Process -Id $PID).Path
  $cli=Join-Path $repo 'scripts/ocmi-context.ps1'
  $raw=& $hostExe -NoProfile -File $cli -Root $temp -Files src/api.ts -Terms Charge
  Require ($LASTEXITCODE -eq 0 -and ($raw -join "`n" | ConvertFrom-Json).selected.Count -eq 1) 'Real CLI consumer failed'
  $raw=& $hostExe -NoProfile -File (Join-Path $repo 'templates/work/tools/ocmi-context.ps1') -Root $temp -Files src/api.ts -Terms Charge
  Require ($LASTEXITCODE -eq 0 -and ($raw -join "`n" | ConvertFrom-Json).selected.Count -eq 1) 'Installed CLI consumer failed'
  $raw=& $hostExe -NoProfile -File $cli -Root $temp -Files ../outside.ts -Terms Charge
  Require ($LASTEXITCODE -eq 2 -and ($raw -join "`n") -notmatch [regex]::Escape($temp)) 'CLI failure leaks absolute scope or passes'
  $real=Invoke-OCMIContext -Root $repo -Files scripts/context-lib.ps1,scripts/ocmi-context.ps1 -RequiredFiles docs/memory/RULES.md -Terms Invoke-OCMIContext
  Require ('scripts/ocmi-context.ps1' -cin @($real.selected.path) -and 'docs/memory/RULES.md' -cin @($real.selected.path)) 'Actual source CLI/rules not recovered'
  Write-Output "PASS: $passed context index/retrieval checks; fixture snippets $($report.metrics.snippetChars), full report $reportChars / $($report.metrics.sourceChars) source chars; fresh content verified, completeness unclaimed."
} finally {
  Clear-OCMIContextCache
  Assert-OCMITemporaryDirectory $temp $parent 'ocmi-context-'
  if (Test-Path -LiteralPath $temp) {Remove-Item -LiteralPath $temp -Recurse -Force}
}
