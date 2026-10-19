param([string]$Root=(Join-Path $PSScriptRoot '..'))
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'path-utils.ps1')
$repo=(Resolve-Path -LiteralPath $Root).Path
$data=Get-Content -Raw -Encoding UTF8 -LiteralPath (Get-OCMIScopedFile $repo 'localization/continuity-guards.json')|ConvertFrom-Json
if($data.schemaVersion -ne 1){throw 'Unsupported continuity guard schema'}
$passed=0
function Require([bool]$Condition,[string]$Message){if(!$Condition){throw $Message};$script:passed++}
function Reject([scriptblock]$Body){$rejected=$false;try{& $Body|Out-Null}catch{$rejected=$true};Require $rejected 'Unsafe/incomplete continuity fixture accepted'}
function Clauses([string]$Text,$Patterns){foreach($pattern in $Patterns){if($Text -notmatch $pattern){throw 'Continuity safeguard missing'}}}
function Fingerprint([string]$Base){(@(Get-ChildItem -LiteralPath $Base -File -Recurse|Sort-Object FullName|ForEach-Object {($_.FullName.Substring($Base.Length+1).Replace('\','/')+'|'+(Get-FileHash -LiteralPath $_.FullName).Hash)}) -join "`n")}
function Topic([string]$Base){
  foreach($path in @('AGENTS.md','memory/INDEX.md','memory/BRIEF.md','memory/tasks/main/HANDOFF.md','memory/GUIDE.md')){Get-OCMIScopedFile $Base $path|Out-Null}
  foreach($file in @(Get-ChildItem -LiteralPath $Base -Recurse -File -Filter '*.md')){
    $relative=Get-OCMIRelativeFile $Base $file.FullName
    $safe=Get-OCMIScopedFile $Base $relative
    $text=[IO.File]::ReadAllText($safe)
    foreach($match in [regex]::Matches($text,'\]\(([^)]+)\)')){
      $link=$match.Groups[1].Value.Split('#')[0]
      if(!$link -or $link -match '^https?://'){continue}
      if($link -match '^[A-Za-z][A-Za-z0-9+.-]*:|^[/\\]'){throw 'Nonportable/external local link'}
      $target=[IO.Path]::GetFullPath((Join-Path $file.DirectoryName $link))
      $targetRelative=Get-OCMIRelativeFile $Base $target
      Get-OCMIScopedFile $Base $targetRelative|Out-Null
    }
  }
}
foreach($language in @('en','ru')){
  $prefix=if($language -eq 'ru'){'locales/ru/'}else{''}
  foreach($guard in $data.guards){
    $text=[IO.File]::ReadAllText((Get-OCMIScopedFile $repo ($prefix+$guard.path)))
    $patterns=@($guard.$language);Clauses $text $patterns;$passed++
    foreach($pattern in $patterns){
      $mutated=[regex]::Replace($text,$pattern,'removed',[Text.RegularExpressions.RegexOptions]::IgnoreCase)
      Reject {Clauses $mutated $patterns}
    }
  }
}
$parent=[IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$temp=Join-Path $parent ('ocmi-continuity-'+[guid]::NewGuid().ToString('N'))
$utf8=[Text.UTF8Encoding]::new($false)
try{
  [IO.Directory]::CreateDirectory($temp)|Out-Null
  foreach($language in @('en','ru')){
    $prefix=if($language -eq 'ru'){'locales/ru/'}else{''}
    $source=Join-Path $repo ($prefix+'templates/personal')
    $sourceBefore=Fingerprint $source
    $area=Join-Path $temp $language;[IO.Directory]::CreateDirectory($area)|Out-Null
    $one=Join-Path $area ('inbox topic '+[char]0x03A9)
    $two=Join-Path $area 'other topic'
    Copy-Item -LiteralPath $source -Destination $one -Recurse
    Copy-Item -LiteralPath $source -Destination $two -Recurse
    $material=Join-Path $one 'memory/materials';[IO.Directory]::CreateDirectory($material)|Out-Null
    $unicodeFile='note-'+[char]0x0442+'.md'
    [IO.File]::WriteAllText((Join-Path $material $unicodeFile),('Synthetic '+[char]0x03A9+' '+[char]0x0442),$utf8)
    $brief=Join-Path $one 'memory/BRIEF.md'
    [IO.File]::AppendAllText($brief,("`nStable topic: topic-one; goal remains undecided.`n[Material](materials/"+$unicodeFile+")`n"),$utf8)
    Topic $one;$passed++
    $oneBefore=Fingerprint $one;$twoBefore=Fingerprint $two
    $moved=Join-Path $area 'settled topic'
    Copy-Item -LiteralPath $one -Destination $moved -Recurse
    Topic $moved;Require ((Fingerprint $moved) -ceq $oneBefore) 'Move/copy changed content or identity'
    Require ([IO.File]::ReadAllText((Join-Path $moved 'memory/BRIEF.md')).Contains('topic-one')) 'Topic identity lost'
    $portable=Join-Path $area 'cloud handover copy'
    Copy-Item -LiteralPath $moved -Destination $portable -Recurse
    Topic $portable;Require ((Fingerprint $portable) -ceq $oneBefore) 'Portable copy lost UTF-8/linked files'
    Require (!(Test-Path -LiteralPath (Join-Path $portable 'other topic'))) 'Sibling topic exported'
    [IO.File]::WriteAllText((Join-Path $moved 'memory/conflict.md'),'first account',$utf8)
    [IO.File]::WriteAllText((Join-Path $two 'memory/conflict.md'),'second account',$utf8)
    $twoPrepared=Fingerprint $two
    $conflict=Join-Path $moved 'memory/conflict.md';$conflictHash=(Get-FileHash -LiteralPath $conflict).Hash
    Reject {[IO.File]::Copy((Join-Path $two 'memory/conflict.md'),$conflict,$false)}
    Require ((Get-FileHash -LiteralPath $conflict).Hash -ceq $conflictHash) 'Conflict overwrote current content'
    $history=Join-Path $moved 'memory/history';[IO.Directory]::CreateDirectory($history)|Out-Null
    Copy-Item -LiteralPath $two -Destination (Join-Path $history 'topic-two') -Recurse
    [IO.File]::AppendAllText((Join-Path $moved 'memory/INDEX.md'),"`n[Preserved second topic](history/topic-two/memory/BRIEF.md)`n",$utf8)
    Topic $moved;$passed++
    Require ([IO.File]::ReadAllText((Join-Path $history 'topic-two/memory/conflict.md')) -ceq 'second account') 'Merge provenance lost'
    $retained=(Get-FileHash -LiteralPath (Join-Path $moved 'memory/tasks/main/HANDOFF.md')).Hash
    [IO.File]::AppendAllText((Join-Path $moved 'memory/INDEX.md'),"`nArchived in place; retained outcome and open questions remain at the current links.`n",$utf8)
    Topic $moved;Require ((Get-FileHash -LiteralPath (Join-Path $moved 'memory/tasks/main/HANDOFF.md')).Hash -ceq $retained) 'Archive rewrote task evidence'
    $bad=Join-Path $portable 'memory/bad.md'
    [IO.File]::WriteAllText($bad,'[Missing](absent.md)',$utf8);Reject {Topic $portable}
    [IO.File]::WriteAllText($bad,'[Sibling](../../other topic/AGENTS.md)',$utf8);Reject {Topic $portable}
    [IO.File]::WriteAllText($bad,'[Absolute](C:/outside.md)',$utf8);Reject {Topic $portable}
    [IO.File]::Delete($bad)
    $link=Join-Path $portable 'linked'
    $kind=if([Environment]::OSVersion.Platform -eq [PlatformID]::Win32NT){'Junction'}else{'SymbolicLink'}
    New-Item -ItemType $kind -Path $link -Target $two|Out-Null
    Reject {Get-OCMIScopedFile $portable 'linked/AGENTS.md'}
    [IO.Directory]::Delete($link)
    Require ((Fingerprint $one) -ceq $oneBefore) 'Original topic changed'
    Require ((Fingerprint $two) -ceq $twoPrepared) 'Second selected source changed by merge/archive'
    Require ((Fingerprint $source) -ceq $sourceBefore) 'Source template changed'
    Require (Test-Path -LiteralPath (Join-Path $two 'AGENTS.md')) 'Boundary cleanup removed sibling fixture'
  }
  Write-Output "PASS: $passed continuity instruction/omission and synthetic topic move/merge/UTF-8/relative-link/conflict/archive/export/boundary checks. No live AI/cloud or monitoring certification."
}finally{
  Assert-OCMITemporaryDirectory $temp $parent 'ocmi-continuity-'
  if(Test-Path -LiteralPath $temp){Remove-Item -LiteralPath $temp -Recurse -Force}
}
