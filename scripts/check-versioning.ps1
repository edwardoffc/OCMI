param([string]$Root=(Join-Path $PSScriptRoot '..'))
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'path-utils.ps1')
. (Join-Path $PSScriptRoot 'version-utils.ps1')
$repo=(Resolve-Path -LiteralPath $Root).Path
$version=Get-OCMIReleaseVersion $repo
$receipt=Get-Content -Raw -Encoding UTF8 -LiteralPath (Get-OCMIScopedFile $repo ("docs/releases/$version-flow.json"))|ConvertFrom-Json
$passed=0
function Reject([scriptblock]$Body){$rejected=$false;try{& $Body|Out-Null}catch{$rejected=$true};if(!$rejected){throw 'Version drift mutation accepted'};$script:passed++}
Assert-OCMIUnifiedReceipt $receipt $version;$passed++
foreach($index in 0..5){
  $mutant=($receipt|ConvertTo-Json -Depth 12)|ConvertFrom-Json
  $mutant.profiles[$index].version='0.0.0'
  Reject {Assert-OCMIUnifiedReceipt $mutant $version}
}
$mutant=($receipt|ConvertTo-Json -Depth 12)|ConvertFrom-Json
$mutant|Add-Member -NotePropertyName workVersion -NotePropertyValue $version
Reject {Assert-OCMIUnifiedReceipt $mutant $version}
foreach($profile in $receipt.profiles){
  $prefix=if($profile.language -ceq 'ru'){'locales/ru/'}else{''}
  foreach($file in @($profile.files|Where-Object {$_.path.EndsWith('.md')})){
    $text=[IO.File]::ReadAllText((Get-OCMIScopedFile $repo ($prefix+'templates/'+$profile.profile+'/'+$file.path)))
    if($text -match '(?m)^(?:# |\*\*)OCMI \d+\.\d+\.\d+'){
      Assert-OCMIVersionLabel $text $version;$passed++
      Reject {Assert-OCMIVersionLabel ($text.Replace('OCMI '+$version,'OCMI 0.0.0')) $version}
    }
  }
}
Assert-OCMIVersionLabel ([IO.File]::ReadAllText((Get-OCMIScopedFile $repo 'locales/ru/VERSION.md'))) $version;$passed++
Write-Output "PASS: $passed unified label/receipt guards and drift mutations; six profiles share OCMI $version."
