param([string]$Root=(Join-Path $PSScriptRoot '..'))
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'skill-lib.ps1')
$repo=[IO.Path]::GetFullPath($Root)
$passed=0
function Require([bool]$Condition,[string]$Message) {if (!$Condition) {throw $Message}; $script:passed++}
function Reject([scriptblock]$Body) {$rejected=$false;try{& $Body | Out-Null}catch{$rejected=$true};Require $rejected 'Invalid skill selection accepted'}
$inventories=@{}
foreach ($language in @('en','ru')) {
  $prefix=if ($language -eq 'ru') {'locales/ru/'} else {''}
  $profile=Join-Path $repo ($prefix+'templates/work')
  $catalog=[IO.File]::ReadAllText((Get-OCMIScopedFile $profile 'skills/catalog.json')) | ConvertFrom-Json
  $inventories[$language]=@($catalog.skills | ForEach-Object {($_.id,$_.need,$_.path,($_.intents -join ',')) -join '|'})
  foreach ($skill in $catalog.skills) {
    $body=[IO.File]::ReadAllText((Get-OCMIScopedFile $profile $skill.path))
    $front=[regex]::Match($body,'\A---\r?\nname: ([a-z0-9-]+)\r?\ndescription: ("[^\r\n]*")\r?\n---\r?\n')
    Require $front.Success 'Unsupported skill frontmatter'
    $description=(('{"value":'+$front.Groups[2].Value+'}') | ConvertFrom-Json).value
    Require ($front.Groups[1].Value -ceq $skill.id -and $description -ceq $skill.description) 'Stale catalog description/ID'
  }
  $plan=Get-OCMISkillSelection -Root $profile -Intent change
  Require ($plan.selected.Count -eq 0 -and $plan.procedureBodiesLoaded -eq 0 -and $plan.readFiles.Count -eq 1) 'Ordinary change eagerly loads skills'
  foreach ($case in @(@('research','reuse','ocmi-search-first'),@('bug','diagnose','ocmi-ci-triage'),@('change','tests','ocmi-test-design'),@('review','review','ocmi-review'),@('question','security','ocmi-security-review'))) {
    $plan=Get-OCMISkillSelection -Root $profile -Intent $case[0] -Needs $case[1]
    Require ($plan.selected.Count -eq 1 -and $plan.selected[0].id -ceq $case[2] -and $plan.dispatch -ceq 'none' -and $plan.readiness -ceq 'not-evaluated') 'Need/intent routed incorrectly or gained authority'
  }
  $plan=Get-OCMISkillSelection -Root $profile -Intent review -Needs tests,review
  Require ($plan.selected.Count -eq 2 -and $plan.procedureBodiesLoaded -eq 0) 'Bounded combined contribution failed'
  $plan=Get-OCMISkillSelection -Root $profile -Intent question -Needs diagnose
  Require ($plan.selected.Count -eq 0 -and $plan.unmatchedNeeds.Count -eq 1) 'Unsupported intent guessed covered'
  $plan=Get-OCMISkillSelection -Root $profile -Role worker -AssignedIds ocmi-test-design
  Require ($plan.selected.Count -eq 1 -and $plan.selected[0].id -ceq 'ocmi-test-design') 'Worker addressed selection failed'
  Reject {Get-OCMISkillSelection -Root $profile -Role worker -Needs security}
  Reject {Get-OCMISkillSelection -Root $profile -AssignedIds unknown-skill}
  Reject {Get-OCMISkillSelection -Root $profile -AssignedIds OCMI-REVIEW}
  Reject {Get-OCMISkillSelection -Root $profile -AssignedIds ocmi-review,ocmi-review}
  Reject {Get-OCMISkillSelection -Root $profile -Needs review -AssignedIds ocmi-review}
  $plan=Get-OCMISkillSelection -Root $profile -Role consultant -Intent question -Needs security
  Require ($plan.role -ceq 'consultant' -and $plan.dispatch -ceq 'none' -and $plan.readiness -ceq 'not-evaluated') 'Consultation escalated authority'
}
Require (($inventories.en -join "`n") -ceq ($inventories.ru -join "`n")) 'Language routing inventories differ'
$parent=[IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$temp=Join-Path $parent ('ocmi-skills-'+[guid]::NewGuid().ToString('N'))
try {
  [IO.Directory]::CreateDirectory((Join-Path $temp 'skills')) | Out-Null
  $path=Join-Path $temp 'skills/catalog.json'
  $original=[IO.File]::ReadAllText((Join-Path $repo 'templates/work/skills/catalog.json'))
  [IO.File]::WriteAllText($path,$original,[Text.UTF8Encoding]::new($false))
  $before=(Get-FileHash -LiteralPath $path).Hash
  $plan=Get-OCMISkillSelection -Root $temp -Needs review -Intent review
  Require ($plan.selected.Count -eq 1 -and !(Test-Path -LiteralPath (Join-Path $temp $plan.selected[0].path))) 'Selector loaded/created procedure bodies'
  Require ((Get-FileHash -LiteralPath $path).Hash -ceq $before) 'Selector changed catalog'
  $mutant=$original | ConvertFrom-Json; $mutant.skills[0].path='../private/SKILL.md'
  [IO.File]::WriteAllText($path,($mutant | ConvertTo-Json -Depth 8))
  Reject {Get-OCMISkillSelection -Root $temp -Needs reuse -Intent research}
  $mutant=$original | ConvertFrom-Json; $mutant.skills[1].id=$mutant.skills[0].id
  [IO.File]::WriteAllText($path,($mutant | ConvertTo-Json -Depth 8))
  Reject {Get-OCMISkillSelection -Root $temp}
  [IO.File]::WriteAllText($path,('x'*32769))
  Reject {Get-OCMISkillSelection -Root $temp}
  Write-Output "PASS: $passed skill catalog/routing/assignment/boundary checks; metadata only, no dispatch or acceptance."
} finally {
  Assert-OCMITemporaryDirectory $temp $parent 'ocmi-skills-'
  if (Test-Path -LiteralPath $temp) {Remove-Item -LiteralPath $temp -Recurse -Force}
}
