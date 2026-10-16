# Metadata-only selection aid; does not authenticate assignments or execute skills.
. (Join-Path $PSScriptRoot 'path-utils.ps1')
function Get-OCMISkillSelection {
  param(
    [Parameter(Mandatory=$true)][string]$Root,
    [ValidateSet('orchestrator','worker','consultant')][string]$Role='orchestrator',
    [ValidateSet('bug','change','research','review','release','question')][string]$Intent='change',
    [ValidateSet('reuse','diagnose','tests','review','security')][string[]]$Needs=@(),
    [string[]]$AssignedIds=@()
  )
  if (!$Root) {throw 'Explicit profile root required'}
  if ($Needs.Count -gt 5 -or $AssignedIds.Count -gt 5 -or ($Needs.Count -and $AssignedIds.Count)) {throw 'Ambiguous or excessive selection'}
  if ($Role -eq 'worker' -and (!$AssignedIds.Count -or $Needs.Count)) {throw 'Worker requires addressed skill IDs; needs do not authorize selection'}
  $file=Get-OCMIScopedFile $Root 'skills/catalog.json'
  $stream=[IO.File]::OpenRead($file)
  try {
    $bytes=New-Object byte[] 32769; $length=0
    while ($length -lt $bytes.Length) { $count=$stream.Read($bytes,$length,$bytes.Length-$length); if (!$count) {break}; $length+=$count }
    if ($length -gt 32768) {throw 'Catalog exceeds metadata limit'}
    $text=[Text.UTF8Encoding]::new($false,$true).GetString($bytes,0,$length).TrimStart([char]0xfeff)
  } finally {$stream.Dispose()}
  $catalog=$text | ConvertFrom-Json -ErrorAction Stop
  if ($catalog.schemaVersion -ne 1 -or $catalog.language -cnotin @('en','ru') -or @($catalog.skills).Count -ne 5) {throw 'Unsupported catalog'}
  $byId=@{}; $needsSeen=@{}
  $allowedIntents=@('bug','change','research','review','release','question')
  foreach ($skill in $catalog.skills) {
    if ($skill.id -isnot [string] -or $skill.id -cnotmatch '^ocmi-[a-z0-9]+(?:-[a-z0-9]+)*$' -or $skill.id.Length -ge 64 -or $byId.ContainsKey($skill.id) -or $skill.path -cne ('skills/'+$skill.id+'/SKILL.md')) {throw 'Invalid/duplicate skill path or ID'}
    if ($skill.description -isnot [string] -or !$skill.description.Trim() -or $skill.description.Length -gt 320 -or $skill.need -cnotin @('reuse','diagnose','tests','review','security') -or $needsSeen.ContainsKey($skill.need)) {throw 'Invalid discovery metadata'}
    if ($skill.intents -isnot [Array] -or !$skill.intents.Count) {throw 'Missing intent mapping'}
    foreach ($value in $skill.intents) {if ($value -cnotin $allowedIntents) {throw 'Unsupported intent'}}
    $byId[$skill.id]=$skill; $needsSeen[$skill.need]=$true
  }
  $selected=@(); $unmatched=@()
  if ($AssignedIds.Count) {
    $seen=@{}
    foreach ($id in $AssignedIds) {
      if (!$byId.ContainsKey($id) -or $byId[$id].id -cne $id -or $seen.ContainsKey($id)) {throw 'Unknown or duplicate assigned skill'}
      $selected+=$byId[$id]; $seen[$id]=$true
    }
  } else {
    foreach ($skill in $catalog.skills) {if ($skill.need -cin $Needs -and $Intent -cin $skill.intents) {$selected+=$skill}}
    foreach ($need in $Needs) {if ($need -cnotin @($selected | ForEach-Object {$_.need})) {$unmatched+=$need}}
  }
  [pscustomobject]@{
    schemaVersion=1;language=$catalog.language;role=$Role;intent=$Intent
    selected=@($selected);unmatchedNeeds=@($unmatched)
    readFiles=@('skills/catalog.json');readBytes=$length;procedureBodiesLoaded=0
    readiness='not-evaluated';dispatch='none'
  }
}
