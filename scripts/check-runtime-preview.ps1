param([string]$PythonPath, [string]$OldBinaryPath, [string]$BaselineRoot,
      [string]$PreviousBinaryPath, [string]$PreviousBaselineRoot,
      [string]$ResourceBaselineBinaryPath, [string]$ResourceBaselineRoot,
      [string]$FixtureRoot, [string]$FixtureManifest,
      [ValidateSet('All','CoreOnly','DurableOnly')][string]$Mode='All',
      [int]$ShardIndex=0, [int]$ShardCount=1)
$ErrorActionPreference = 'Stop'
if($ShardCount -lt 1 -or $ShardIndex -lt 0 -or $ShardIndex -ge $ShardCount -or ($Mode -ne 'DurableOnly' -and ($ShardIndex -ne 0 -or $ShardCount -ne 1))){throw 'Invalid mode/shard combination'}
. (Join-Path $PSScriptRoot 'runtime-durable-selector.ps1')
Write-Output "RUNTIME mode=$Mode shard=$ShardIndex/$ShardCount"
$repo = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$scratch = Join-Path $repo '.local/build/runtime'
function Get-FixtureBinaryMap {
  param([string]$Root,[string]$Manifest,[string]$ScratchRoot)
  if(!(Test-Path -LiteralPath $Root) -or !(Test-Path -LiteralPath $Manifest)){throw 'Required legacy fixture root or manifest is missing.'}
  foreach($path in @($Root,$Manifest)){ $p=[IO.Path]::GetFullPath($path); while($true){if(Test-Path -LiteralPath $p){$i=Get-Item -LiteralPath $p -Force;if(($i.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0){throw "Fixture link refused: $path"}};$parent=Split-Path $p -Parent;if(!$parent -or $parent -eq $p){break};$p=$parent} }
  $rootFull=(Resolve-Path -LiteralPath $Root).Path; $rows=@(Get-Content -LiteralPath $Manifest -Raw|ConvertFrom-Json)
  if($rows.Count -ne 3 -or (@($rows.schema|Sort-Object)-join ',') -ne 'schema-v1,schema-v2,schema-v3'){throw 'Fixture manifest must declare exactly schema-v1, schema-v2 and schema-v3.'}
  $plans=@()
  foreach($row in $rows){$schema=[string]$row.schema;$dir=Join-Path $rootFull $schema;if(!(Test-Path $dir)){throw "Missing declared fixture: $schema"};$di=Get-Item $dir -Force;if(($di.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0){throw "Fixture link refused: $schema"};$expected=@{};$ordered=@();foreach($f in @($row.files)){if(!$f.path -or $f.path -match '(^|[\\/])\.\.([\\/]|$)|^[\\/]|:' -or $f.sha256 -notmatch '^[0-9a-fA-F]{64}$'){throw "Unsafe fixture manifest entry: $schema/$($f.path)"};$name=$f.path.Replace('\','/');$key=$name.ToLowerInvariant();if($expected.ContainsKey($key)){throw "Duplicate/case-colliding fixture path: $schema/$name"};$expected[$key]=[pscustomobject]@{path=$name;sha=$f.sha256.ToLowerInvariant()};$ordered+=@($expected[$key])};$seen=@{};foreach($item in @(Get-ChildItem -LiteralPath $dir -Force -Recurse)){if(($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0){throw "Fixture link refused: $schema"};if(!$item.PSIsContainer){$rel=[IO.Path]::GetRelativePath($dir,$item.FullName).Replace('\','/');$key=$rel.ToLowerInvariant();if(!$expected.ContainsKey($key) -or $expected[$key].path -cne $rel -or $seen.ContainsKey($key)){throw "Unexpected/case-colliding fixture file: $schema/$rel"};$sha=(Get-FileHash $item.FullName -Algorithm SHA256).Hash.ToLowerInvariant();if($sha -ne $expected[$key].sha){throw "Fixture digest mismatch: $schema/$rel"};$seen[$key]=$sha}};if($seen.Count -ne $expected.Count){throw "Fixture inventory incomplete: $schema"};$mat=(($ordered|ForEach-Object{"$($_.path):$($_.sha)"})-join "`n");$agg=[Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($mat))).ToLowerInvariant();if($agg -ne $row.aggregateSha256.ToLowerInvariant()){throw "Fixture aggregate mismatch: $schema"};$plans+=@([pscustomobject]@{schema=$schema;dir=$dir})}
  $out=@{};foreach($plan in $plans){$suffix=if($env:OS -eq 'Windows_NT'){'.exe'}else{''};$bin=Join-Path $ScratchRoot "bin/$($plan.schema)$suffix";& go -C (Join-Path $plan.dir 'runtime') build -mod=readonly -trimpath -o $bin ./cmd/ocmi;if($LASTEXITCODE -ne 0){throw "Fixture build failed: $($plan.schema)"};$out[$plan.schema]=$bin}
  return $out
}
$names = @('GOPATH','GOMODCACHE','GOCACHE','GOTMPDIR','GOTOOLCHAIN','GOWORK','CGO_ENABLED','TEMP','TMP','TMPDIR','GOOS','GOARCH','PYTHONDONTWRITEBYTECODE','PYTHONUTF8','OCMI_ROOT_BINARY')
$saved = @{}
foreach ($name in $names) { $saved[$name] = [Environment]::GetEnvironmentVariable($name,'Process') }
try {
  $env:OCMI_ROOT_BINARY = $null
  foreach ($part in @('gopath','gocache','tmp','bin','evidence')) {
    New-Item -ItemType Directory -Path (Join-Path $scratch $part) -Force | Out-Null
  }
  $env:GOPATH = Join-Path $scratch 'gopath'
  $env:GOMODCACHE = Join-Path $env:GOPATH 'pkg/mod'
  $env:GOCACHE = Join-Path $scratch 'gocache'
  $env:GOTMPDIR = Join-Path $scratch 'tmp'
  $env:TEMP = $env:GOTMPDIR; $env:TMP = $env:GOTMPDIR; $env:TMPDIR = $env:GOTMPDIR
  $env:GOTOOLCHAIN = 'local'; $env:GOWORK = 'off'; $env:CGO_ENABLED = '0'
  $env:GOOS = $null; $env:GOARCH = $null; $env:PYTHONDONTWRITEBYTECODE = '1'; $env:PYTHONUTF8 = '1'
  if ($Mode -ne 'DurableOnly' -and !$OldBinaryPath -and !$PreviousBinaryPath -and !$ResourceBaselineBinaryPath) {
    $explicitFixture=@($BaselineRoot,$PreviousBaselineRoot,$ResourceBaselineRoot|Where-Object {$_})
    if($explicitFixture.Count -gt 1){throw 'Select one pinned fixture root.'}
    if($explicitFixture.Count -eq 1){$FixtureRoot=$explicitFixture[0]}
    if (!$FixtureRoot) { $FixtureRoot = Join-Path $repo 'packaging/fixtures' }
    if (!$FixtureManifest) { $FixtureManifest = Join-Path $FixtureRoot 'manifest.json' }
    $fixtureBinaries = Get-FixtureBinaryMap -Root $FixtureRoot -Manifest $FixtureManifest -ScratchRoot $scratch
    $OldBinaryPath = $fixtureBinaries['schema-v1']; $PreviousBinaryPath = $fixtureBinaries['schema-v2']; $ResourceBaselineBinaryPath = $fixtureBinaries['schema-v3']
    $BaselineRoot=$null; $PreviousBaselineRoot=$null; $ResourceBaselineRoot=$null
  }
  $module = Join-Path $repo 'runtime'
  $suffix = if ($env:OS -eq 'Windows_NT') { '.exe' } else { '' }
  if ($Mode -ne 'DurableOnly') {
  $unformatted = & gofmt -l (Join-Path $module 'cmd') (Join-Path $module 'internal') (Join-Path $module 'adapters')
  if ($LASTEXITCODE -ne 0 -or $unformatted) { throw 'Unformatted runtime sources' }
  & go -C $module mod verify
  if ($LASTEXITCODE -ne 0) { throw 'Module verification failed' }
  & go -C $module vet -mod=readonly ./...
  if ($LASTEXITCODE -ne 0) { throw 'Runtime vet failed' }
  $stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssfffZ')
  # Keep the deadline per test rather than sharing it across independent durable
  # update/setup/Git-checkpoint scenarios. Discover from the native test binary so added tests cannot
  # disappear from a manually maintained group list. Compile once, run serially.
  $isolatedTestsPattern = '^Test((Main|Worker)Update|MainToolAuthoring|WorkerToolStorageAuthoring|CheckpointGit)'
  & go -C $module test -mod=readonly -count=1 -timeout=180s -skip $isolatedTestsPattern -v ./... 2>&1 | Tee-Object -FilePath (Join-Path $scratch "evidence/preview-$stamp.txt")
  if ($LASTEXITCODE -ne 0) { throw 'Runtime tests failed' }
  }
  $stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssfffZ')
  $isolatedTestsPattern = '^Test((Main|Worker)Update|MainToolAuthoring|WorkerToolStorageAuthoring|CheckpointGit)'
  if ($Mode -ne 'CoreOnly') {
  $updateTestsBinary = Join-Path $scratch "bin/update-tests-$stamp$suffix"
  & go -C $module test -mod=readonly -c -o $updateTestsBinary ./internal
  if ($LASTEXITCODE -ne 0) { throw 'Update test binary build failed' }
  & (Join-Path $PSScriptRoot 'test-runtime-durable-selector.ps1') -TestBinary $updateTestsBinary
  $updateTests = @(& $updateTestsBinary ("-test.list=" + $isolatedTestsPattern))
  if ($LASTEXITCODE -ne 0 -or !$updateTests.Count -or @($updateTests | Where-Object { $_ -cnotmatch ($isolatedTestsPattern + '[A-Za-z0-9_]+$') }).Count -or @($updateTests | Sort-Object -Unique).Count -ne $updateTests.Count) { throw 'Invalid native durable-scenario test inventory' }
  $selected = @(Select-DurableShard -Inventory $updateTests -Index $ShardIndex -Count $ShardCount -Pattern $isolatedTestsPattern)
  $ordered = [string[]]$updateTests.Clone(); [Array]::Sort($ordered,[StringComparer]::Ordinal)
  $digest = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes(($ordered -join "`n")))).ToLowerInvariant()
  Write-Output "DURABLE mode=$Mode shard=$ShardIndex/$ShardCount inventory=$($ordered.Count) selected=$($selected.Count) sha256=$digest"
  foreach ($selection in $selected) {
    & $updateTestsBinary '-test.count=1' '-test.timeout=180s' '-test.run' "^$([regex]::Escape($selection))$" '-test.v' 2>&1 | Tee-Object -FilePath (Join-Path $scratch "evidence/preview-$stamp-$selection.txt")
    if ($LASTEXITCODE -ne 0) { throw "Runtime $selection tests failed" }
  }
  }
  if ($Mode -eq 'DurableOnly') { Write-Output 'PASS: native durable shard'; return }
  $binary = Join-Path $scratch "bin/ocmi-preview-$stamp$suffix"
  & go -C $module build -mod=readonly -o $binary ./cmd/ocmi
  if ($LASTEXITCODE -ne 0) { throw 'Preview build failed' }
  $env:OCMI_ROOT_BINARY = $binary
  $python = $PythonPath
  if ($python) { $python = (Resolve-Path -LiteralPath $python).Path }
  else {
    $python = (Get-Command python3 -ErrorAction SilentlyContinue).Source
    if (!$python) { $python = (Get-Command python -ErrorAction Stop).Source }
  }
  & $python (Join-Path $repo 'runtime/acceptance/check_cli.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'CLI acceptance failed' }
  & $python (Join-Path $repo 'runtime/acceptance/check_context.py') --binary $binary --scratch-root $env:GOTMPDIR
  if ($LASTEXITCODE -ne 0) { throw 'Native continuation/context acceptance failed' }
  & $python (Join-Path $repo 'runtime/acceptance/check_portable.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Portable acceptance failed' }
  & $python (Join-Path $repo 'runtime/acceptance/check_root_alias.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Selected-root alias acceptance failed' }
  & $python (Join-Path $repo 'runtime/acceptance/check_checkpoint.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Selected-file checkpoint acceptance failed' }
  & $python (Join-Path $repo 'runtime/acceptance/check_git_checkpoint.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Git/WIP checkpoint acceptance failed' }
  & $python (Join-Path $repo 'runtime/acceptance/check_git_checkpoint_limits.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Git/WIP native blob/history bounds acceptance failed' }
  & $python (Join-Path $repo 'runtime/acceptance/check_environment.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Environment diagnosis/recovery acceptance failed' }
  & $python (Join-Path $repo 'runtime/acceptance/check_environment_lifecycle.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Environment owned-probe/recovery lifecycle acceptance failed' }
  & $python (Join-Path $repo 'runtime/acceptance/check_runner.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Runner acceptance failed' }
  & $python (Join-Path $repo 'runtime/acceptance/check_transport.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Transport acceptance failed' }
  & $python (Join-Path $repo 'runtime/acceptance/check_resources.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Resource lifecycle acceptance failed' }
  & $python (Join-Path $repo 'runtime/acceptance/check_lost_reply.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Lost reply acceptance failed' }
  & $python (Join-Path $repo 'runtime/acceptance/check_maintenance.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Resource maintenance acceptance failed' }
  & $python (Join-Path $repo 'runtime/acceptance/check_resource_process.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Resource process/TMP acceptance failed' }
  & $python (Join-Path $repo 'runtime/acceptance/check_workspace_recovery.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Native crash-workspace recovery acceptance failed' }
  & $python (Join-Path $repo 'runtime/acceptance/check_root_layout.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Root layout acceptance failed' }
  & go -C $module test -mod=readonly -count=1 -v -run '^Test(RootLayoutAcceptanceNativeMCP|ContextAcceptanceNativeMCP|WorkflowAcceptanceNativeMCP|CompactAcceptanceNativeMCP|ExecutionChannelsAcceptanceNativeTask|MaintenanceAcceptanceNativeMCP)$' ./adapters
  if ($LASTEXITCODE -ne 0) { throw 'Root layout native MCP acceptance failed' }
  & go -C $module test -mod=readonly -count=1 -timeout=90s -v -run '^TestMain(Tool(StorageNativeCLI|AuthoringNativeCLI(Probes)?)|ErgonomicsNativeCLI|BorrowedHistoryNativeCLI|Compact(Metadata|Text|TextNoEffects)?NativeCLI|Workflow(Profile|BoundConfig)?NativeCLI)$' ./internal
  if ($LASTEXITCODE -ne 0) { throw 'Tool-storage native authoring/ordinary-run acceptance failed' }
  & $python (Join-Path $repo 'scripts/acceptance/test_install_fixture.py')
  if ($LASTEXITCODE -ne 0) { throw 'Synthetic installation fixture retention controls failed' }
  & $python (Join-Path $repo 'scripts/acceptance/check_install_home.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Native installation/coordinator acceptance failed' }
  & $python (Join-Path $repo 'scripts/acceptance/check_update_home.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Native update acceptance failed' }
  & $python (Join-Path $repo 'scripts/acceptance/check_update_diagnostics.py') --binary $binary
  if ($LASTEXITCODE -ne 0) { throw 'Native update diagnostics acceptance failed' }
  if ($ResourceBaselineBinaryPath -and $ResourceBaselineRoot) { throw 'Select one resource-v3 fixture source' }
  if (!$ResourceBaselineBinaryPath) { throw 'Resource qualification requires actual v3; pass ResourceBaselineBinaryPath or ResourceBaselineRoot' }
  $v3 = (Resolve-Path -LiteralPath $ResourceBaselineBinaryPath).Path
  & $python (Join-Path $repo 'runtime/acceptance/check_resource_migration.py') --binary $binary --v3-binary $v3
  if ($LASTEXITCODE -ne 0) { throw 'Resource actual v3 migration/transfer acceptance failed' }
  if (Test-Path -LiteralPath (Join-Path $repo 'runtime/acceptance/check_lifecycle.py')) {
    if ($OldBinaryPath -and $BaselineRoot) { throw 'Select one old-binary fixture source' }
    if (!$OldBinaryPath) { throw 'Lifecycle/migration qualification requires an actual v1 fixture; pass OldBinaryPath or BaselineRoot' }
    $old = (Resolve-Path -LiteralPath $OldBinaryPath).Path
    & $python (Join-Path $repo 'runtime/acceptance/check_lifecycle.py') --binary $binary
    if ($LASTEXITCODE -ne 0) { throw 'Lifecycle/policy acceptance failed' }
    & $python (Join-Path $repo 'runtime/acceptance/check_outcome_decision.py') --binary $binary
    if ($LASTEXITCODE -ne 0) { throw 'Evidence-bound owner decision acceptance failed' }
    & $python (Join-Path $repo 'runtime/acceptance/check_episode.py') --binary $binary
    if ($LASTEXITCODE -ne 0) { throw 'Consecutive episode/history acceptance failed' }
    if ($PreviousBinaryPath -and $PreviousBaselineRoot) { throw 'Select one previous-v2 fixture source' }
    if (!$PreviousBinaryPath) { throw 'Scope/migration qualification requires actual v2; pass PreviousBinaryPath or PreviousBaselineRoot' }
    $v2 = (Resolve-Path -LiteralPath $PreviousBinaryPath).Path
    & $python (Join-Path $repo 'runtime/acceptance/check_preparation.py') --binary $binary --v2-binary $v2
    if ($LASTEXITCODE -ne 0) { throw 'Preparation/target scope and actual v2 migration acceptance failed' }
    & $python (Join-Path $repo 'runtime/acceptance/check_scope_legacy.py') --binary $binary --v2-binary $v2
    if ($LASTEXITCODE -ne 0) { throw 'Actual v2 backup/transfer and v3 scope validation failed' }
    & $python (Join-Path $repo 'runtime/acceptance/check_scope_edges.py') --binary $binary --v2-binary $v2
    if ($LASTEXITCODE -ne 0) { throw 'Scope continuation and legacy routing edge acceptance failed' }
    & $python (Join-Path $repo 'runtime/acceptance/check_scoping.py') --binary $binary
    if ($LASTEXITCODE -ne 0) { throw 'Exact-task evidence scoping failed' }
    & $python (Join-Path $repo 'runtime/acceptance/check_hook_lifecycle.py') --binary $binary
    if ($LASTEXITCODE -ne 0) { throw 'Lifecycle hook consumer acceptance failed' }
    & $python (Join-Path $repo 'runtime/acceptance/check_recovery.py') --binary $binary
    if ($LASTEXITCODE -ne 0) { throw 'Actual interrupted execution recovery acceptance failed' }
    & $python (Join-Path $repo 'runtime/acceptance/check_recovery_boundary.py') --binary $binary
    if ($LASTEXITCODE -ne 0) { throw 'Native recovery identity boundary acceptance failed' }
    foreach ($check in @('check_migration.py','check_history_transfer.py')) {
      & $python (Join-Path $repo "runtime/acceptance/$check") --binary $binary --old-binary $old
      if ($LASTEXITCODE -ne 0) { throw "Legacy/ledger acceptance failed: $check" }
    }
  }
  Write-Output "PASS: native runtime preview; candidate: $binary"
} finally {
  foreach ($name in $names) { [Environment]::SetEnvironmentVariable($name,$saved[$name],'Process') }
}
