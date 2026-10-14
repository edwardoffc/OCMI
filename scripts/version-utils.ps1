function Get-OCMIReleaseVersion([string]$Repo) {
  $text=[IO.File]::ReadAllText((Get-OCMIScopedFile $Repo 'VERSION.md'))
  $match=[regex]::Match($text,'\A# OCMI (\d+\.\d+\.\d+)(?:\r?\n|$)')
  if(!$match.Success){throw 'Missing system release version'}
  $match.Groups[1].Value
}
function Assert-OCMIUnifiedReceipt($Receipt,[string]$Version) {
  if($Receipt.schemaVersion -ne 2 -or $Receipt.numbering -cne 'unified' -or $Receipt.version -cne "OCMI $Version" -or @($Receipt.profiles).Count -ne 6){throw 'Invalid unified release receipt'}
  foreach($name in @('installerVersion','workVersion','lightweightVersion')){if($Receipt.PSObject.Properties.Name -ccontains $name){throw 'Independent component version field'}}
  $keys=@{}
  foreach($profile in $Receipt.profiles){
    if($profile.profile -cnotin @('work','personal','coordinator') -or $profile.language -cnotin @('en','ru') -or $profile.version -cne $Version){throw 'Component differs from system release version'}
    foreach($name in @('installerVersion','workVersion','lightweightVersion')){if($profile.PSObject.Properties.Name -ccontains $name){throw 'Independent profile version field'}}
    $key=$profile.profile+'/'+$profile.language
    if($keys.ContainsKey($key)){throw 'Duplicate unified profile'}
    $keys[$key]=$true
  }
}
function Assert-OCMIVersionLabel([string]$Text,[string]$Version) {
  $match=[regex]::Match($Text,'(?m)(?:^# |\*\*)OCMI (\d+\.\d+\.\d+)(?:[, \r\n]|$)')
  if(!$match.Success -or $match.Groups[1].Value -cne $Version){throw 'Current label differs from system release version'}
}
