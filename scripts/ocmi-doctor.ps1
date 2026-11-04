param(
  [Parameter(Mandatory=$true)][string]$Root,
  [Parameter(Mandatory=$true)][ValidateSet('system','work','personal')][string]$Profile,
  [string]$InputFile = '',
  [string]$ExpectedVersion = '',
  [switch]$Json
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'doctor-lib.ps1')
$report = Invoke-OCMIDoctor -Root $Root -Profile $Profile -InputFile $InputFile -ExpectedVersion $ExpectedVersion
if ($Json) { $report | ConvertTo-Json -Depth 12 }
else {
  Write-Output ("OCMI doctor: {0}; profile: {1}; readiness: not-evaluated" -f $report.status,$Profile)
  foreach ($item in $report.findings) { Write-Output ("{0} {1} [{2}]: {3}" -f $item.level,$item.code,$item.path,$item.action) }
  Write-Output ("Read {0} files / {1} bytes; no target files changed. Exit {2}." -f $report.coverage.files,$report.coverage.bytes,$report.exitCode)
}
exit $report.exitCode
