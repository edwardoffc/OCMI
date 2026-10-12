param([string]$Root = (Join-Path $PSScriptRoot '..'))
$ErrorActionPreference = 'Stop'
foreach ($name in @('check-docs','check-workflow','check-flow','check-localization','check-onboarding','check-release','check-local-boundary','check-platform','check-boundaries')) {
  & (Join-Path $PSScriptRoot ($name+'.ps1')) -Root $Root
}
git -C $Root diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace check failed' }
Write-Output 'PASS: complete documentation/platform suite. Security scan requires a separately verified Gitleaks binary.'
