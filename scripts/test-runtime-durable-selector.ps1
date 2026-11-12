param([Parameter(Mandatory=$true)][string]$TestBinary)
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'runtime-durable-selector.ps1')
$pattern='^Test((Main|Worker)Update|MainToolAuthoring|WorkerToolStorageAuthoring|CheckpointGit)'
$inventory=@(& $TestBinary ("-test.list="+$pattern))
if($LASTEXITCODE -ne 0){throw 'Inventory command failed'}
$all=[Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
foreach($shard in 0..2){foreach($name in @(Select-DurableShard $inventory $shard 3 $pattern)){if(!$all.Add($name)){throw 'Shard intersection'}}}
if($all.Count -ne $inventory.Count){throw 'Incomplete union'}
foreach($name in $inventory){if(!$all.Contains($name)){throw 'Missing test'}}
foreach($case in @('duplicate','index','negativeIndex','countZero','excessCount','foreign')){
 $rejected=$false
 try{switch($case){
  'duplicate' { Select-DurableShard @($inventory[0],$inventory[0]) 0 3 $pattern | Out-Null }
  'index' { Select-DurableShard $inventory 3 3 $pattern | Out-Null }
  'negativeIndex' { Select-DurableShard $inventory -1 3 $pattern | Out-Null }
  'countZero' { Select-DurableShard $inventory 0 0 $pattern | Out-Null }
  'excessCount' { Select-DurableShard $inventory 0 ($inventory.Count+1) $pattern | Out-Null }
  'foreign' { Select-DurableShard @('TestForeign') 0 3 $pattern | Out-Null }
 }}catch{$rejected=$true}
 if(!$rejected){throw "Accepted $case"}
}
Write-Output "PASS: $($inventory.Count) native tests exactly once across three shards; duplicate, index, count and foreign controls rejected"
