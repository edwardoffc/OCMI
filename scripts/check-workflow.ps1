param([string]$Root = (Join-Path $PSScriptRoot '..'))
$ErrorActionPreference = 'Stop'
$rootPath = (Resolve-Path -LiteralPath $Root).Path
$guards = Get-Content -Raw -Encoding UTF8 (Join-Path $rootPath 'localization/workflow-guards.json') | ConvertFrom-Json
$passed = 0
function Assert-Check([bool]$Condition,[string]$Message) { if (!$Condition) { throw $Message } }
function Read-Scoped([string]$Base,[string]$Relative) {
  Assert-Check ($Relative -notmatch '^[A-Za-z][A-Za-z0-9+.-]*:|^[/\\]|(^|[/\\])\.\.([/\\]|$)') "Unsafe path: $Relative"
  $full=[IO.Path]::GetFullPath((Join-Path $Base $Relative))
  Assert-Check ($full.StartsWith($Base.TrimEnd('\','/')+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)) 'Outside profile'
  Assert-Check (Test-Path -LiteralPath $full -PathType Leaf) "Missing input: $Relative"
  $current=Get-Item -LiteralPath $full
  while($current.FullName -ne $Base) {
    Assert-Check (!(($current.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0)) 'Reparse input'
    if($current -is [IO.FileInfo]){$current=$current.Directory}else{$current=$current.Parent}
    Assert-Check ($null -ne $current) 'Missing parent'
  }
  return [IO.File]::ReadAllText($full)
}
function Get-Routes([string]$Text,[string[]]$Names) {
  $routes=@{}
  $namesPattern=($Names|ForEach-Object{[regex]::Escape($_)}) -join '|'
  foreach($match in [regex]::Matches($Text,('(?m)^\|\s*('+ $namesPattern +')\s*\|\s*\[[^\]]+\]\(([^)]+)\)\s*\|'))) {
    $name=$match.Groups[1].Value;$path=$match.Groups[2].Value
    Assert-Check (!$routes.ContainsKey($name)) "Duplicate role: $name"
    Assert-Check ($path -match '^roles/[a-z-]+\.md$') "Unsafe role route: $path"
    $routes[$name]=$path
  }
  foreach($name in $Names){Assert-Check ($routes.ContainsKey($name)) "Missing role: $name"}
  Assert-Check ($routes.Count -eq 3) 'Role inventory'
  return $routes
}
function Assert-Clauses([string]$Text,[string[]]$Patterns,[string]$Context) {
  foreach($pattern in $Patterns){Assert-Check ($Text -match $pattern) "Lost safeguard in $Context : $pattern"}
}
function Check([string]$Name,[scriptblock]$Body){ & $Body; $script:passed++; Write-Output "PASS $Name" }
function Must-Reject([scriptblock]$Body) {
  $rejected=$false;try{& $Body|Out-Null}catch{$rejected=$true}
  Assert-Check $rejected 'Invalid input was accepted'
}
foreach($language in @('en','ru')) {
  $prefix=if($language -eq 'en'){''}else{'locales/ru/'}
  $work=Join-Path $rootPath ($prefix+'templates/work')
  $memory=Join-Path $work 'docs/memory'
  $texts=@{
    agent=(Read-Scoped $work 'AGENTS.md'); rules=(Read-Scoped $memory 'RULES.md')
    router=(Read-Scoped $memory 'ORCHESTRATION.md'); protocol=(Read-Scoped $memory 'TASK-PROTOCOL.md')
    card=(Read-Scoped $memory 'tasks/TEMPLATE.md'); start=(Read-Scoped $memory 'tasks/START-TEMPLATE.md')
    map=(Read-Scoped $memory 'CODEMAP.md'); state=(Read-Scoped $memory 'STATE.md')
    personal=(Read-Scoped (Join-Path $rootPath ($prefix+'templates/personal')) 'AGENTS.md')
  }
  $names=@($guards.roles.$language)
  $routes=Get-Routes $texts.router $names
  foreach($i in 0..2){$texts[@('coordinator','worker','consultant')[$i]]=Read-Scoped $memory $routes[$names[$i]]}
  Check "$language role routes" {
    Assert-Check ($routes[$names[0]] -eq 'roles/coordinator.md' -and $routes[$names[1]] -eq 'roles/worker.md' -and $routes[$names[2]] -eq 'roles/consultant.md') 'Wrong role procedure'
    Assert-Check ($texts.worker -notmatch '\]\([^)]*coordinator\.md\)') 'Worker loads coordinator'
    Assert-Check ($texts.consultant -notmatch '\]\([^)]*(coordinator|worker)\.md\)') 'Consultant loads another role'
  }
  Check "$language missing role rejected" { Must-Reject { Get-Routes ($texts.router -replace ('(?m)^\| '+[regex]::Escape($names[1])+'.*\r?\n'),'') $names } }
  Check "$language duplicate role rejected" { Must-Reject { Get-Routes ($texts.router+"`n| "+$names[1]+" | [Procedure](roles/worker.md) | inputs |") $names } }
  Check "$language unsafe route rejected" { Must-Reject { Get-Routes ($texts.router.Replace('roles/worker.md','roles/../../private.md')) $names } }
  Check "$language external/missing inputs rejected" {
    foreach($path in @('../private.md','roles/missing.md','D:/private.md')){Must-Reject {Read-Scoped $memory $path}}
  }
  foreach($guard in $guards.clauses) {
    Check "$language $($guard.id)" {
      $text=($guard.inputs|ForEach-Object{$texts[$_]}) -join "`n"
      Assert-Clauses $text @($guard.$language) $guard.id
    }
  }
  Check "$language stable task schema" {
    $headings=@($texts.card -split '\r?\n'|Where-Object{$_ -match '^## '})
    Assert-Check (($headings -join '|') -ceq '## Contract|## Result|## Acceptance') 'Task schema changed'
  }
  Check "$language versions" {
    $marker=$guards.markers.$language
    foreach($key in @('agent','rules','router','protocol','card','start','coordinator','worker','consultant')) {
      Assert-Check ($texts[$key].Contains("OCMI 1.2.1, $marker 2026-10-11")) "Wrong version: $key"
    }
    Assert-Check ((Read-Scoped $work 'VERSION.md').Contains("OCMI 1.2.1, $marker 2026-10-11")) 'Wrong VERSION'
    Assert-Check ($texts.personal.Contains("OCMI 1.2, $marker 2026-10-11")) 'Wrong lightweight version'
  }
  Check "$language lightweight startup" {
    Assert-Check ($texts.personal -notmatch '\]\([^)]*ORCHESTRATION\.md\)') 'Heavy lightweight startup'
  }
  Check "$language stale-evidence mutation rejected" {
    Must-Reject {Assert-Clauses ($texts.rules.Replace('superseded','obsolete')) @('superseded') 'mutated recovery'}
  }
  Check "$language unsafe cleanup mutation rejected" {
    $phrase=$guards.mutations.$language.cleanup
    Must-Reject {Assert-Clauses ($texts.coordinator.Replace($phrase,'removed')) @([regex]::Escape($phrase)) 'mutated cleanup'}
  }
  Check "$language pending-accountability mutation rejected" {
    $phrase=$guards.mutations.$language.pending
    Must-Reject {Assert-Clauses ($texts.protocol.Replace($phrase,'removed')) @([regex]::Escape($phrase)) 'mutated pending'}
  }
}
Check 'English development procedures match canonical templates' {
  foreach($path in @('ORCHESTRATION.md','TASK-PROTOCOL.md','roles/coordinator.md','roles/worker.md','roles/consultant.md','tasks/TEMPLATE.md','tasks/START-TEMPLATE.md')){
    Assert-Check ((Read-Scoped (Join-Path $rootPath 'templates/work/docs/memory') $path) -ceq (Read-Scoped (Join-Path $rootPath 'docs/memory') $path)) "Development drift: $path"
  }
}
Write-Output "PASS: $passed static checks across en/ru. Not a live-model evaluation."
