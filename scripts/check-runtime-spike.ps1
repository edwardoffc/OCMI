param([switch]$CompileTargets)
$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$module = Join-Path $repo 'runtime'
$scratch = Join-Path $repo '.local/build/runtime'
$go = (Get-Command go -ErrorAction Stop).Source
$saved = @{}
$envNames = @('GOPATH','GOMODCACHE','GOCACHE','GOTMPDIR','GOTOOLCHAIN','GOWORK','CGO_ENABLED','TEMP','TMP','TMPDIR','GOOS','GOARCH')
foreach ($name in $envNames) { $saved[$name] = [Environment]::GetEnvironmentVariable($name, 'Process') }
try {
  foreach ($part in @('gopath','gocache','tmp','evidence','bin')) {
    New-Item -ItemType Directory -Path (Join-Path $scratch $part) -Force | Out-Null
  }
  $env:GOPATH = Join-Path $scratch 'gopath'
  $env:GOMODCACHE = Join-Path $env:GOPATH 'pkg/mod'
  $env:GOCACHE = Join-Path $scratch 'gocache'
  $env:GOTMPDIR = Join-Path $scratch 'tmp'
  $env:TEMP = $env:GOTMPDIR
  $env:TMP = $env:GOTMPDIR
  $env:TMPDIR = $env:GOTMPDIR
  $env:GOTOOLCHAIN = 'local'
  $env:GOWORK = 'off'
  $env:CGO_ENABLED = '0'
  $env:GOOS = $null
  $env:GOARCH = $null
  $stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssfffZ')
  & $go version
  if ($LASTEXITCODE -ne 0) { throw 'Go toolchain unavailable' }
  $unformatted = & (Join-Path (Split-Path $go) 'gofmt') -l (Join-Path $module 'spike')
  if ($LASTEXITCODE -ne 0 -or $unformatted) { throw "Unformatted probe source: $unformatted" }
  & $go -C $module mod verify
  if ($LASTEXITCODE -ne 0) { throw 'Dependency verification failed' }
  & $go -C $module vet -mod=readonly ./spike
  if ($LASTEXITCODE -ne 0) { throw 'Probe vet failed' }
  $log = Join-Path $scratch "evidence/native-$stamp.txt"
  & $go -C $module test -mod=readonly -count=1 -timeout=60s -v ./spike 2>&1 | Tee-Object -FilePath $log
  if ($LASTEXITCODE -ne 0) { throw "Native probe failed; evidence: $log" }
  if ($CompileTargets) {
    foreach ($target in @('windows/amd64','darwin/arm64','darwin/amd64','linux/amd64')) {
      $env:GOOS, $env:GOARCH = $target.Split('/')
      $name = $target.Replace('/','-')
      $suffix = if ($env:GOOS -eq 'windows') { '.exe' } else { '' }
      $output = Join-Path $scratch "bin/spike-$name-$stamp.test$suffix"
      & $go -C $module test -mod=readonly -c -o $output ./spike
      if ($LASTEXITCODE -ne 0) { throw "Test binary compilation failed: $target" }
      Write-Output "COMPILE ONLY: $target (not native runtime qualification)"
    }
  }
  Write-Output "PASS: native premise probes; evidence: $log"
} finally {
  foreach ($name in $envNames) { [Environment]::SetEnvironmentVariable($name, $saved[$name], 'Process') }
}
