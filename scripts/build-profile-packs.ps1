param(
 [Parameter(Mandatory=$true)][string]$Root,
 [Parameter(Mandatory=$true)][string]$OutputRoot,
 [Parameter(Mandatory=$true)][ValidatePattern('^\d{4}-\d{2}-\d{2}$')][string]$Date,
 [Parameter(Mandatory=$true)][string]$ReceiptName,
 [ValidateRange(1,9999)][int]$ArchiveRevision=1
)
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem
. (Join-Path $PSScriptRoot 'path-utils.ps1')

function Test-PrivateComponent([string]$Name) {
 $n=$Name.ToLowerInvariant()
 return $n -in @('local','ocmi_sync','.local','.git','.codex') -or $n -match '(secret|database|credential)'
}
function Assert-NoReparseAncestors([string]$Path) {
 $item=Get-Item -LiteralPath ([IO.Path]::GetFullPath($Path)) -Force
 while($null -ne $item){
  if($item.Attributes -band [IO.FileAttributes]::ReparsePoint){throw 'Linked source or output path'}
  $item=if($item -is [IO.FileInfo]){$item.Directory}else{$item.Parent}
 }
}
function Assert-ExistingParentsSafe([string]$Path) {
 $probe=[IO.Path]::GetFullPath($Path)
 while(!(Test-Path -LiteralPath $probe)){ $next=[IO.Path]::GetDirectoryName($probe); if(!$next -or $next -ceq $probe){throw 'Missing output ancestor'}; $probe=$next }
 Assert-NoReparseAncestors $probe
}
function Get-Sha256([string]$Path) {(Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()}
function Get-BoundedStreamHash([IO.Stream]$Stream,[int64]$Expected) {
 $sha=[Security.Cryptography.SHA256]::Create();$buffer=New-Object byte[] 65536;$left=$Expected
 try{
  while(($read=$Stream.Read($buffer,0,$buffer.Length)) -gt 0){if($read -gt $left){throw 'Input exceeds bounded inventory'};[void]$sha.TransformBlock($buffer,0,$read,$buffer,0);$left-=$read}
  [void]$sha.TransformFinalBlock((New-Object byte[] 0),0,0)
  if($left -ne 0){throw 'Input changed after inventory'}
  return ([BitConverter]::ToString($sha.Hash).Replace('-','').ToLowerInvariant())
 }finally{$sha.Dispose()}
}
function Get-BoundedFileHash([string]$Path,[int64]$Expected) {
 $input=[IO.File]::Open($Path,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read)
 try{return Get-BoundedStreamHash $input $Expected}finally{$input.Dispose()}
}
function Get-BoundedFileText([string]$Path,[int64]$Expected) {
 $input=[IO.File]::Open($Path,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read)
 $copy=[IO.MemoryStream]::new();$buffer=New-Object byte[] 65536;$left=$Expected
 try{
  while(($read=$input.Read($buffer,0,$buffer.Length)) -gt 0){if($read -gt $left){throw 'Input exceeds bounded inventory'};$copy.Write($buffer,0,$read);$left-=$read}
  if($left -ne 0){throw 'Input changed after inventory'}
  return [Text.Encoding]::UTF8.GetString($copy.ToArray())
 }finally{$copy.Dispose();$input.Dispose()}
}
function Copy-BoundedRecord($Record,[IO.Stream]$Destination) {
 $input=[IO.File]::Open($Record.source,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read)
 $sha=[Security.Cryptography.SHA256]::Create();$buffer=New-Object byte[] 65536;$left=[int64]$Record.bytes
 try{
  while(($read=$input.Read($buffer,0,$buffer.Length)) -gt 0){if($read -gt $left){throw 'Input exceeds bounded inventory'};$Destination.Write($buffer,0,$read);[void]$sha.TransformBlock($buffer,0,$read,$buffer,0);$left-=$read}
  [void]$sha.TransformFinalBlock((New-Object byte[] 0),0,0)
  $actual=([BitConverter]::ToString($sha.Hash).Replace('-','').ToLowerInvariant())
  if($left -ne 0 -or $actual -cne $Record.sha256){throw 'Input changed after inventory'}
 }finally{$sha.Dispose();$input.Dispose()}
}
function Assert-SafeRelative([string]$Relative) {
 if(!$Relative -or $Relative -match '^[A-Za-z][A-Za-z0-9+.-]*:|^[/\\]|(^|[/\\])\.\.([/\\]|$)'){throw 'Unsafe template member'}
 foreach($part in $Relative.Split('/')){if(Test-PrivateComponent $part){throw 'Private template path component'}}
}
function Get-EditionRecords([string]$Base,[string]$Version) {
 Assert-NoReparseAncestors $Base
 $queue=[Collections.Generic.Queue[IO.DirectoryInfo]]::new();$queue.Enqueue([IO.DirectoryInfo]::new($Base))
 $seen=[Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
 $records=[Collections.Generic.List[object]]::new();$visited=0;[int64]$total=0
 while($queue.Count){
  $directory=$queue.Dequeue()
  foreach($item in $directory.EnumerateFileSystemInfos()){
   $visited++;if($visited -gt 1024){throw 'Template inventory bounds'}
   if($item.Attributes -band [IO.FileAttributes]::ReparsePoint){throw 'Template member cannot be a link'}
   $relative=Get-OCMIRelativeFile $Base $item.FullName
   Assert-SafeRelative $relative
   if(!$seen.Add($relative)){throw 'Case-alias template member'}
   if($item -is [IO.DirectoryInfo]){$queue.Enqueue($item);continue}
   if(!($item -is [IO.FileInfo])){throw 'Unsupported template member'}
   if($item.Length -gt 16MB -or $total -gt (16MB-$item.Length)){throw 'Template inventory bounds'}
   $total+=$item.Length
   $records.Add([ordered]@{path=$relative;bytes=[int64]$item.Length;source=$item.FullName})
  }
 }
 if($records.Count -lt 1){throw 'Template inventory bounds'}
 foreach($record in $records){
  if([IO.FileInfo]::new($record.source).Length -ne $record.bytes){throw 'Input changed after inventory'}
  if([IO.Path]::GetExtension($record.source) -ceq '.md'){
   $text=Get-BoundedFileText $record.source $record.bytes
   foreach($match in [regex]::Matches($text,'(?m)(?:^# |\*\*)OCMI (\d+\.\d+\.\d+)(?:[, \r\n—]|$)')){if($match.Groups[1].Value -cne $Version){throw 'Current label differs from system release version'}}
  }
  $record.sha256=Get-BoundedFileHash $record.source $record.bytes
 }
 return @($records|Sort-Object path)
}
function Assert-OwnedStage([string]$Stage,[string]$Marker,[string]$ExpectedMarker,[string]$Output) {
 if(![IO.Path]::GetDirectoryName([IO.Path]::GetFullPath($Stage)).Equals([IO.Path]::GetFullPath($Output).TrimEnd([char[]]'\\/'),[StringComparison]::OrdinalIgnoreCase)){throw 'Unsafe temporary cleanup target'}
 if((Get-Item -LiteralPath $Stage -Force).Attributes -band [IO.FileAttributes]::ReparsePoint){throw 'Linked temporary cleanup target'}
 if(!(Test-Path -LiteralPath $Marker -PathType Leaf) -or [IO.File]::ReadAllText($Marker) -cne $ExpectedMarker){throw 'Unowned temporary cleanup target'}
 foreach($item in Get-ChildItem -LiteralPath $Stage -Recurse -Force){if($item.Attributes -band [IO.FileAttributes]::ReparsePoint){throw 'Linked temporary cleanup target'}}
}

$src=[IO.Path]::GetFullPath($Root);$out=[IO.Path]::GetFullPath($OutputRoot)
if(!(Test-Path -LiteralPath $src -PathType Container) -or !(Test-Path -LiteralPath $out -PathType Container)){throw 'Root and OutputRoot must exist'}
Assert-NoReparseAncestors $src;Assert-NoReparseAncestors $out
$versionPath=Get-OCMIScopedFile $src 'VERSION.md'
if([IO.FileInfo]::new($versionPath).Length -gt 64KB){throw 'VERSION.md exceeds bounded input'}
$versionText=Get-BoundedFileText $versionPath ([IO.FileInfo]::new($versionPath).Length)
$m=[regex]::Match($versionText,'\A# OCMI (\d+\.\d+\.\d+)(?:\r?\n|$)');if(!$m.Success){throw 'Invalid VERSION.md'};$version=$m.Groups[1].Value
if($ReceiptName -cnotmatch ('^'+[regex]::Escape($version)+'(?:-[A-Za-z0-9][A-Za-z0-9-]*)?\.json$')){throw 'ReceiptName must be a version-prefixed plain JSON basename'}

$editions=@();$allOutputs=@()
foreach($lang in 'en','ru'){foreach($profile in 'work','personal','coordinator'){
 $prefix=if($lang -eq 'ru'){'locales/ru/'}else{''};$base=Join-Path $src ($prefix+'templates/'+$profile)
 if(!(Test-Path -LiteralPath $base -PathType Container)){throw 'Missing canonical template'}
 $records=Get-EditionRecords $base $version
 $suffix=if($ArchiveRevision -gt 1){'-r'+$ArchiveRevision}else{''};$name="ocmi-$profile-$version-$lang-$Date$suffix.zip";$dest=Join-Path $out ('releases/'+$name)
 Assert-ExistingParentsSafe $dest;if(Test-Path -LiteralPath $dest){throw 'Immutable archive collision'}
 $editions+=,[ordered]@{profile=$profile;language=$lang;version=$version;archive='releases/'+$name;destination=$dest;files=$records};$allOutputs+=,$dest
}}
$receipt=Join-Path $out ('docs/releases/'+$ReceiptName);Assert-ExistingParentsSafe $receipt;if(Test-Path -LiteralPath $receipt){throw 'Immutable receipt collision'};$allOutputs+=,$receipt
foreach($path in $allOutputs){Assert-ExistingParentsSafe $path;if(Test-Path -LiteralPath $path){throw 'Output collision'}}
$historical=[ordered]@{};$old=Join-Path $out 'releases';if(Test-Path -LiteralPath $old){Get-ChildItem -LiteralPath $old -File -Force -Filter '*.zip'|ForEach-Object{Assert-NoReparseAncestors $_.FullName;$probe=[IO.Compression.ZipFile]::OpenRead($_.FullName);try{$historical[$_.Name]=Get-Sha256 $_.FullName}finally{$probe.Dispose()}}}

$stage=Join-Path $out ('.profile-pack-stage-'+[guid]::NewGuid().ToString('N'));Assert-ExistingParentsSafe $stage
$marker=Join-Path $stage 'OWNERSHIP.json';$markerText='{ "owner": "build-profile-packs", "purpose": "staging", "lifetime": "this invocation only" }'
[IO.Directory]::CreateDirectory($stage)|Out-Null;[IO.File]::WriteAllText($marker,$markerText,[Text.UTF8Encoding]::new($false))
try{
 $profiles=@()
 foreach($edition in $editions){
  $tmp=Join-Path $stage ([IO.Path]::GetFileName($edition.destination));$zip=[IO.Compression.ZipFile]::Open($tmp,[IO.Compression.ZipArchiveMode]::Create)
  try{foreach($record in $edition.files){$entry=$zip.CreateEntry($record.path);$stream=$entry.Open();try{Copy-BoundedRecord $record $stream}finally{$stream.Dispose()}}}finally{$zip.Dispose()}
  $verify=[IO.Compression.ZipFile]::OpenRead($tmp)
  try{foreach($record in $edition.files){$entry=$verify.GetEntry($record.path);if($null -eq $entry -or $entry.Length -ne $record.bytes){throw 'Archive verification failed'};$stream=$entry.Open();try{$hash=Get-BoundedStreamHash $stream $record.bytes}finally{$stream.Dispose()};if($hash -cne $record.sha256){throw 'Archive verification failed'}}}finally{$verify.Dispose()}
  $profiles += [ordered]@{profile=$edition.profile;language=$edition.language;version=$edition.version;archive=$edition.archive;sha256=(Get-Sha256 $tmp);files=@($edition.files|ForEach-Object{[ordered]@{path=$_.path;sha256=$_.sha256;bytes=$_.bytes}})}
 }
 $manifest=[ordered]@{schemaVersion=2;numbering='unified';version='OCMI '+$version;profiles=@($profiles);historicalArchives=$historical};$receiptStage=Join-Path $stage $ReceiptName;[IO.File]::WriteAllText($receiptStage,($manifest|ConvertTo-Json -Depth 12),[Text.UTF8Encoding]::new($false))
 foreach($edition in $editions){$from=Join-Path $stage ([IO.Path]::GetFileName($edition.archive));$to=Join-Path $out $edition.archive;Assert-ExistingParentsSafe $to;[IO.Directory]::CreateDirectory((Split-Path $to))|Out-Null;[IO.File]::Move($from,$to)}
 Assert-ExistingParentsSafe $receipt;[IO.Directory]::CreateDirectory((Split-Path $receipt))|Out-Null;[IO.File]::Move($receiptStage,$receipt)
}finally{
 if(Test-Path -LiteralPath $stage){Assert-OwnedStage $stage $marker $markerText $out;[IO.File]::WriteAllText($marker,($markerText+' closed'),[Text.UTF8Encoding]::new($false));Remove-Item -LiteralPath $stage -Recurse -Force}
}
