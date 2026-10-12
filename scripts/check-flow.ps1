param([string]$Root = (Join-Path $PSScriptRoot '..'))
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'path-utils.ps1')
$repo=(Resolve-Path -LiteralPath $Root).Path
$guards=Get-Content -Raw -Encoding UTF8 -LiteralPath (Get-OCMIScopedFile $repo 'localization/flow-guards.json') | ConvertFrom-Json
if ($guards.schemaVersion -ne 1) {throw 'Unsupported flow guard schema'}
$passed=0
function Require([bool]$Condition,[string]$Name) {if (!$Condition) {throw $Name}; $script:passed++}
function Clauses([string]$Text,[string[]]$Patterns) {
  foreach ($pattern in $Patterns) {if ($Text -notmatch $pattern) {throw 'Required workflow clause missing'}}
}
function Schema([string]$Text) {
  $headings=@($Text -split '\r?\n' | Where-Object {$_ -match '^## '})
  if (($headings -join '|') -cne '## Contract|## Result|## Acceptance') {throw 'Card schema changed'}
}
function Compact([string]$Text,[string[]]$Fields) {
  Schema $Text
  foreach ($field in $Fields) {if (!$Text.Contains($field)) {throw 'Compact field missing'}}
}
function Reject([scriptblock]$Body) {
  $rejected=$false
  try {& $Body | Out-Null} catch {$rejected=$true}
  Require $rejected 'Unsafe/missing workflow fixture accepted'
}
foreach ($language in @('en','ru')) {
  $prefix=if ($language -ceq 'ru') {'locales/ru/'} else {''}
  $memory=Join-Path $repo ($prefix+'templates/work/docs/memory')
  $texts=@{}
  foreach ($pair in @(@('protocol','TASK-PROTOCOL.md'),@('coordinator','roles/coordinator.md'),@('worker','roles/worker.md'),@('rules','RULES.md'))) {
    $texts[$pair[0]]=[IO.File]::ReadAllText((Get-OCMIScopedFile $memory $pair[1]))
  }
  $small=[IO.File]::ReadAllText((Get-OCMIScopedFile $memory 'tasks/SMALL-TEMPLATE.md'))
  Compact $small @($guards.fields.$language); $passed++
  foreach ($field in $guards.fields.$language) {
    Require ($small.Contains($field)) 'Compact field missing'
    Reject {Compact ($small.Replace($field,'removed')) @($guards.fields.$language)}
  }
  Reject {Schema ($small.Replace('## Acceptance','## accepted'))}
  foreach ($guard in $guards.guards) {
    $text=$texts[$guard.input]
    $patterns=@($guard.$language)
    Clauses $text $patterns; $passed++
    # Removing any required clause must fail, including risk/gate/recovery safeguards.
    foreach ($pattern in $patterns) {
      $lines=@($text -split '\r?\n' | Where-Object {$_ -notmatch $pattern})
      Reject {Clauses ($lines -join "`n") $patterns}
    }
  }
}
Write-Output "PASS: $passed bilingual compact/schema/coverage/preflight/retry/recovery guards and negative mutations. Instruction checks, not live-agent behavior."
