param(
  [Parameter(Mandatory=$true)][string]$Root,
  [Parameter(Mandatory=$true)][string[]]$Files,
  [Parameter(Mandatory=$true)][string[]]$Terms,
  [string[]]$RequiredFiles=@(),
  [int]$BudgetChars=16000,
  [string]$SourceRevision='unknown'
)
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'context-lib.ps1')
try {
  $report=Invoke-OCMIContext -Root $Root -Files $Files -Terms $Terms -RequiredFiles $RequiredFiles -BudgetChars $BudgetChars -SourceRevision $SourceRevision
  $report | ConvertTo-Json -Depth 10
  if ($report.status -eq 'incomplete') {exit 2}
  exit 0
} catch {
  # Do not echo arbitrary filesystem/decoder errors or absolute private paths.
  Write-Output '{"status":"incomplete","reason":"Invalid, unavailable, unsafe or excessive input; narrow authorized retrieval or use manual fallback.","readiness":"not-evaluated"}'
  exit 2
}
