param([string]$Root=(Join-Path $PSScriptRoot '..'),[switch]$ControlOnly)
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'path-utils.ps1')
Add-Type -AssemblyName System.IO.Compression.FileSystem
Add-Type -AssemblyName System.IO.Compression
$source=(Resolve-Path -LiteralPath $Root).Path
$tempBase=[IO.Path]::GetFullPath([IO.Path]::GetTempPath())
# macOS exposes its temporary root through /var -> /private/var. Select the
# physical test root; the producer still rejects all linked caller inputs.
if($tempBase.StartsWith('/var/',[StringComparison]::Ordinal)){
 $alias=[IO.DirectoryInfo]::new('/var')
 if($alias.Attributes -band [IO.FileAttributes]::ReparsePoint){
  $physical=$alias.ResolveLinkTarget($true)
  Assert-OCMIRelativePath $tempBase.Substring(5)
  $tempBase=Join-Path $physical.FullName $tempBase.Substring(5)
 }
}
$owned=Join-Path $tempBase ('ocmi-profiles-'+[guid]::NewGuid().ToString('N'))
[IO.Directory]::CreateDirectory($owned)|Out-Null
$encoding=[Text.UTF8Encoding]::new($false)
function Assert-Pack([bool]$Condition,[string]$Message){if(!$Condition){throw $Message}}
function Write-PackText([string]$Path,[string]$Text){[IO.Directory]::CreateDirectory((Split-Path -Parent $Path))|Out-Null;[IO.File]::WriteAllText($Path,$Text,$encoding)}
function Make-PackFixture([string]$Name){
 $base=Join-Path $owned $Name
 Write-PackText (Join-Path $base 'VERSION.md') "# OCMI 9.9.9`n"
 Write-PackText (Join-Path $base 'locales/ru/VERSION.md') "# OCMI 9.9.9`n"
 foreach($language in @('en','ru')){foreach($profile in @('work','personal','coordinator')){
  $prefix=if($language -ceq 'ru'){'locales/ru/'}else{''}
  $members=if($profile -ceq 'coordinator'){@('AGENTS.md','INSTALL.md','VERSION.md','STARTUP.md','memory/INDEX.md','memory/BRIEF.md','memory/tasks/main/HANDOFF.md','LICENSE')}else{@('AGENTS.md')}
  foreach($member in $members){Write-PackText (Join-Path $base ($prefix+'templates/'+$profile+'/'+$member)) "# OCMI 9.9.9`n`nFictional $language $profile edition.`n"}
 }}
 return $base
}
function Existing-PackChecker([string]$Base){
 & (Join-Path $source 'scripts/check-release.ps1') -Root $Base
 & (Join-Path $source 'scripts/check-versioning.ps1') -Root $Base
}
function Public-PackSnapshot([string]$Base){
 $result=[ordered]@{}
 foreach($folder in @('releases','docs/releases')){
  $path=Join-Path $Base $folder
  if(Test-Path -LiteralPath $path){foreach($file in Get-ChildItem -LiteralPath $path -Recurse -File -Force | Sort-Object FullName){$result[(Get-OCMIRelativeFile $Base $file.FullName)]=(Get-FileHash -LiteralPath $file.FullName).Hash}}
 }
 return ($result|ConvertTo-Json -Compress)
}
try{
 Write-PackText (Join-Path $owned 'OWNERSHIP.json') '{"owner":"profile-pack acceptance","purpose":"fictional bounded producer controls","retention":"this invocation only","disposition":"remove exact owned root"}'
 $oracle=Make-PackFixture 'oracle'
 $profiles=[Collections.Generic.List[object]]::new()
 foreach($language in @('en','ru')){foreach($profile in @('work','personal','coordinator')){
  $prefix=if($language -ceq 'ru'){'locales/ru/'}else{''}
  $base=Join-Path $oracle ($prefix+'templates/'+$profile)
  $records=@(Get-ChildItem -LiteralPath $base -Recurse -File | Sort-Object FullName | ForEach-Object {[ordered]@{path=(Get-OCMIRelativeFile $base $_.FullName);sha256=(Get-FileHash -LiteralPath $_.FullName).Hash.ToLowerInvariant();bytes=$_.Length}})
  $name="ocmi-$profile-9.9.9-$language-2099-01-01.zip"
  $path=Join-Path $oracle ('releases/'+$name)
  [IO.Directory]::CreateDirectory((Split-Path -Parent $path))|Out-Null
  $archive=[IO.Compression.ZipFile]::Open($path,[IO.Compression.ZipArchiveMode]::Create)
  try{foreach($record in $records){$entry=$archive.CreateEntry($record.path);$stream=$entry.Open();try{$bytes=[IO.File]::ReadAllBytes((Join-Path $base $record.path));$stream.Write($bytes,0,$bytes.Length)}finally{$stream.Dispose()}}}finally{$archive.Dispose()}
  $profiles.Add([ordered]@{profile=$profile;language=$language;version='9.9.9';archive='releases/'+$name;sha256=(Get-FileHash -LiteralPath $path).Hash.ToLowerInvariant();files=$records})
 }}
 $manifest=[ordered]@{schemaVersion=2;numbering='unified';version='OCMI 9.9.9';profiles=@($profiles.ToArray());historicalArchives=[ordered]@{}}
 $receiptPath=Join-Path $oracle 'docs/releases/9.9.9.json'
 Write-PackText $receiptPath ($manifest|ConvertTo-Json -Depth 12)
 Existing-PackChecker $oracle
 $manifest.profiles[0].sha256='0'*64
 Write-PackText $receiptPath ($manifest|ConvertTo-Json -Depth 12)
 $rejected=$false;try{Existing-PackChecker $oracle}catch{$rejected=$true}
 Assert-Pack $rejected 'Existing checker accepted wrong archive hash'
 Write-Output 'PROFILE_PACK_RECEIPT_CONTROL_OK six source/ZIP editions, unified labels, wrong-hash refusal'
 if($ControlOnly){return}
 $producer=Join-Path $source 'scripts/build-profile-packs.ps1'
 Assert-Pack (Test-Path -LiteralPath $producer -PathType Leaf) 'PROFILE_PACK_PRODUCER_MISSING after qualified actual receipt control'
 $actual=Make-PackFixture 'actual'
 $oldName='ocmi-work-9.9.9-en-2099-01-01.zip'
 [IO.Directory]::CreateDirectory((Join-Path $actual 'releases'))|Out-Null
 Copy-Item -LiteralPath (Join-Path $oracle ('releases/'+$oldName)) -Destination (Join-Path $actual ('releases/'+$oldName))
 $oldHash=(Get-FileHash -LiteralPath (Join-Path $actual ('releases/'+$oldName))).Hash.ToLowerInvariant()
 & $producer -Root $actual -OutputRoot $actual -Date '2099-01-02' -ReceiptName '9.9.9.json' -ArchiveRevision 1
 Assert-Pack $? 'Producer command failed'
 Existing-PackChecker $actual
 $produced=Get-Content -LiteralPath (Join-Path $actual 'docs/releases/9.9.9.json') -Raw|ConvertFrom-Json
 Assert-Pack ($produced.historicalArchives.PSObject.Properties[$oldName].Value -ceq $oldHash) 'Historical archive proof missing'
 $before=Public-PackSnapshot $actual
 $refused=$false;try{& $producer -Root $actual -OutputRoot $actual -Date '2099-01-02' -ReceiptName '9.9.9.json' -ArchiveRevision 1}catch{$refused=$true}
 Assert-Pack $refused 'Producer overwrote existing distribution'
 Assert-Pack ((Public-PackSnapshot $actual) -ceq $before) 'Refusal changed accepted distribution'
 $collision=Make-PackFixture 'collision'
 Write-PackText (Join-Path $collision 'releases/ocmi-coordinator-9.9.9-ru-2099-01-02.zip') 'Preserve unknown preexisting output.'
 $before=Public-PackSnapshot $collision
 $refused=$false;try{& $producer -Root $collision -OutputRoot $collision -Date '2099-01-02' -ReceiptName '9.9.9.json' -ArchiveRevision 1}catch{$refused=$true}
 Assert-Pack $refused 'Producer adopted unknown output'
 Assert-Pack ((Public-PackSnapshot $collision) -ceq $before) 'Collision produced partial immutable output'
 $findings=[Collections.Generic.List[string]]::new()
 try{
  $before=Public-PackSnapshot $actual|ConvertFrom-Json
  & $producer -Root $actual -OutputRoot $actual -Date '2099-01-02' -ReceiptName '9.9.9-profiles-r2.json' -ArchiveRevision 2
  foreach($property in $before.PSObject.Properties){Assert-Pack ((Get-FileHash -LiteralPath (Join-Path $actual $property.Name)).Hash -ceq $property.Value) 'Revision changed earlier immutable bytes'}
  $reader=Make-PackFixture 'revision-reader'
  [IO.Directory]::CreateDirectory((Join-Path $reader 'releases'))|Out-Null
  Copy-Item -LiteralPath (Join-Path $actual 'releases') -Destination $reader -Recurse -Force
  Write-PackText (Join-Path $reader 'docs/releases/9.9.9.json') ([IO.File]::ReadAllText((Join-Path $actual 'docs/releases/9.9.9-profiles-r2.json')))
  Existing-PackChecker $reader
 }catch{$findings.Add('distinct receipt revision: '+$_.Exception.Message)}
 foreach($scenario in @('private-source','entry-bound','version-drift','version-bound')){
  try{
   $limited=Make-PackFixture $scenario
   if($scenario -ceq 'private-source'){Write-PackText (Join-Path $limited 'templates/work/local/coordinator/HANDOFF.md') 'Fictional private canary: never a distributable source.'}
   elseif($scenario -ceq 'entry-bound'){foreach($i in 1..1025){[IO.Directory]::CreateDirectory((Join-Path $limited ('templates/work/empty-'+$i)))|Out-Null}}
   elseif($scenario -ceq 'version-drift'){Write-PackText (Join-Path $limited 'templates/work/AGENTS.md') "# OCMI 8.8.8`n"}
   else{Write-PackText (Join-Path $limited 'VERSION.md') ("# OCMI 9.9.9`n"+('x'*65537))}
   $before=Public-PackSnapshot $limited
   $refused=$false;try{& $producer -Root $limited -OutputRoot $limited -Date '2099-01-02' -ReceiptName '9.9.9.json' -ArchiveRevision 1}catch{$refused=$true}
   Assert-Pack $refused 'Unsupported private/unbounded source was accepted'
   Assert-Pack ((Public-PackSnapshot $limited) -ceq $before) 'Refusal published immutable outputs'
  }catch{$findings.Add($scenario+': '+$_.Exception.Message)}
 }
 if([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT){
  foreach($scenario in @('linked-version','linked-history')){
   $limited=Make-PackFixture $scenario
   $foreignFile=Join-Path $owned ($scenario+'-foreign.txt')
   $leaf=if($scenario -ceq 'linked-version'){Join-Path $limited 'VERSION.md'}else{Join-Path $limited 'releases/old.zip'}
   if($scenario -ceq 'linked-version'){[IO.File]::Move($leaf,$foreignFile)}else{Write-PackText $foreignFile 'Fictional foreign archive; must not be opened.';[IO.Directory]::CreateDirectory((Split-Path -Parent $leaf))|Out-Null}
   $beforeHash=(Get-FileHash -LiteralPath $foreignFile).Hash
   try{
    New-Item -ItemType SymbolicLink -Path $leaf -Target $foreignFile|Out-Null
    Assert-Pack (((Get-Item -LiteralPath $leaf -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) 'File-link premise missing'
    $refused=$false;try{& $producer -Root $limited -OutputRoot $limited -Date '2099-01-02' -ReceiptName '9.9.9.json'}catch{$refused=$true;Assert-Pack ($_.Exception.Message -match 'link|reparse') 'Refusal did not identify the linked input'}
    Assert-Pack $refused 'Linked input was read'
    Assert-Pack ((Get-FileHash -LiteralPath $foreignFile).Hash -ceq $beforeHash) 'Foreign input changed'
    Assert-Pack (!(Test-Path -LiteralPath (Join-Path $limited 'docs/releases/9.9.9.json'))) 'Linked input produced a receipt'
   }catch{$findings.Add($scenario+': '+$_.Exception.Message)}finally{
    if(Test-Path -LiteralPath $leaf){Assert-Pack (((Get-Item -LiteralPath $leaf -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) 'File-link fixture changed';[IO.File]::Delete($leaf)}
   }
  }
 }
 $linked=Make-PackFixture 'linked-output'
 $foreign=Join-Path $owned 'foreign-output'
 Write-PackText (Join-Path $foreign 'retained.txt') 'Preserve foreign-output scope.'
 $link=Join-Path $linked 'releases'
 try{
  $linkType=if([Environment]::OSVersion.Platform -eq [PlatformID]::Win32NT){'Junction'}else{'SymbolicLink'}
  New-Item -ItemType $linkType -Path $link -Target $foreign|Out-Null
  Assert-Pack (((Get-Item -LiteralPath $link -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) 'Native link premise missing'
  $before=@(Get-ChildItem -LiteralPath $foreign -File).Count
  $refused=$false;try{& $producer -Root $linked -OutputRoot $linked -Date '2099-01-02' -ReceiptName '9.9.9.json' -ArchiveRevision 1}catch{$refused=$true}
  Assert-Pack $refused 'Linked output parent was followed'
  Assert-Pack (@(Get-ChildItem -LiteralPath $foreign -File).Count -eq $before) 'Foreign-output scope changed'
 }catch{$findings.Add('linked output: '+$_.Exception.Message)}finally{
  if(Test-Path -LiteralPath $link){Assert-Pack (((Get-Item -LiteralPath $link -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) 'Fixture link changed; preserve for review';[IO.Directory]::Delete($link,$false)}
 }
 if($findings.Count){throw ('Profile safety controls failed: '+($findings -join '; '))}
 Write-Output 'PROFILE_PACK_PRODUCER_OK actual six-edition consumer and pre-effect output preservation'
}finally{
 $absolute=[IO.Path]::GetFullPath($owned)
 Assert-OCMITemporaryDirectory $absolute $tempBase 'ocmi-profiles-'
 foreach($item in Get-ChildItem -LiteralPath $absolute -Recurse -Force){Assert-Pack (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -eq 0) 'Owned acceptance fixture contains a link; preserve for review'}
 Remove-Item -LiteralPath $absolute -Recurse -Force
}
