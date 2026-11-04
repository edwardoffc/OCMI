# Reviewed public upstream copyright text may contain authors' public email addresses.
# This exact-manifest/text exception covers only the generic email indicator, never secrets or home paths.
function Get-OCMIReviewedNoticeHashes([string]$Root) {
  $hashes = @{}
  $path = Join-Path $Root 'packaging/notices/manifest.json'
  if (!(Test-Path -LiteralPath $path -PathType Leaf)) { return $hashes }
  $path = Get-OCMIScopedFile $Root 'packaging/notices/manifest.json'
  # Reviewed old notice set plus exact TOML v2.4.3 MIT license; a changed manifest needs explicit review.
  if ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() -cne '428a73becd4d772c547c1e3fb54f9b093160aa3ffe05512b3f1f7cbb54ec0508') { return $hashes }
  $manifest = [IO.File]::ReadAllText($path) | ConvertFrom-Json
  foreach ($entry in @($manifest.toolchain) + @($manifest.modules)) {
    if ($entry.notice -cnotmatch '^[A-Za-z0-9_.-]+\.txt$' -or $entry.sha256 -cnotmatch '^[a-f0-9]{64}$') { throw 'Invalid reviewed notice entry' }
    $hashes['packaging/notices/' + $entry.notice] = [string]$entry.sha256
  }
  return $hashes
}
function Test-OCMIReviewedNoticeText([hashtable]$Hashes,[string]$Label,[string]$Text) {
  if (!$Hashes.ContainsKey($Label)) { return $false }
  $algorithm = [Security.Cryptography.SHA256]::Create()
  try { $sha = [BitConverter]::ToString($algorithm.ComputeHash([Text.Encoding]::UTF8.GetBytes($Text))).Replace('-','').ToLowerInvariant() }
  finally { $algorithm.Dispose() }
  return $sha -ceq $Hashes[$Label]
}
