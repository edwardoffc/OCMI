param(
  [string]$Root = '',
  [Parameter(Mandatory=$true)][string]$GitleaksPath
)
$ErrorActionPreference = 'Stop'
if (!$Root) { $Root = Join-Path $PSScriptRoot '..' }
Add-Type -AssemblyName System.IO.Compression.FileSystem
$repo = (Resolve-Path -LiteralPath $Root).Path
$scanner = (Resolve-Path -LiteralPath $GitleaksPath).Path
$files = @(git -C $repo -c core.quotepath=false ls-files)
if ($LASTEXITCODE -ne 0 -or !$files.Count) { throw 'Cannot enumerate tracked public files' }
$temp = Join-Path ([IO.Path]::GetTempPath()) ('ocmi-scan-' + [guid]::NewGuid().ToString('N'))
[IO.Directory]::CreateDirectory($temp) | Out-Null
$tree = Join-Path $temp 'tree'
[IO.Directory]::CreateDirectory($tree) | Out-Null
$count = 0
$totalBytes = 0L
$issues = [Collections.Generic.List[string]]::new()
function Check-Text([string]$Text,[string]$Label) {
  # Generic privacy indicators; never print matching values.
  if ($Text -match '(?i)(?:[A-Z]:[/\\]Users[/\\]|/(?:home|Users)/)[a-z0-9_.-]+') {
    $issues.Add("Home-directory identifier in $Label")
  }
  foreach ($m in [regex]::Matches($Text,'[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}')) {
    if ($m.Value -notmatch '(?i)@(?:users\.noreply\.github\.com|example\.(?:com|org|net))$') {
      $issues.Add("Review email address in $Label")
    }
  }
}
try {
  foreach ($relative in $files) {
    if ($relative -match '^[A-Za-z][A-Za-z0-9+.-]*:|^[/\\]|(^|[/\\])\.\.([/\\]|$)|^(\.codex|backups|outputs|tmp)/|(^|/)\.local(/|$)|(^|/)\.env($|\.)|\.(pem|key|pfx|p12|bundle|sqlite|db)$') {
      throw 'Unsafe/sensitive tracked path'
    }
    $path = [IO.Path]::GetFullPath((Join-Path $repo $relative))
    if (!$path.StartsWith($repo+'\',[StringComparison]::OrdinalIgnoreCase)) { throw 'Tracked path outside root' }
    $item = Get-Item -LiteralPath $path
    while ($item.FullName -ne $repo) {
      if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Tracked reparse path' }
      $item = Get-Item -LiteralPath (Split-Path $item.FullName -Parent)
    }
    if ($relative.EndsWith('.zip')) {
      $archive = [IO.Compression.ZipFile]::OpenRead($path)
      try {
        $seen = @{}
        foreach ($entry in $archive.Entries) {
          $name = $entry.FullName.Replace('\','/')
          if ($name.EndsWith('/')) { continue }
          if ($name -match '^[A-Za-z][A-Za-z0-9+.-]*:|^/|(^|/)\.\.(/|$)|(^|/)\.local(/|$)|:' -or $seen.ContainsKey($name)) { throw 'Unsafe/duplicate/private archive entry' }
          if ((($entry.ExternalAttributes -shr 16) -band 0xF000) -eq 0xA000) { throw 'Archive symlink' }
          $seen[$name] = $true
          $count++
          $totalBytes += $entry.Length
          if ($count -gt 5000 -or $entry.Length -gt 5MB -or $totalBytes -gt 50MB) { throw 'Archive scan size limit exceeded' }
          $destination = Join-Path (Join-Path $tree $relative.Replace('.zip','-contents')) $name
          [IO.Directory]::CreateDirectory((Split-Path $destination -Parent)) | Out-Null
          $source = $entry.Open()
          $output = [IO.File]::Create($destination)
          try { $source.CopyTo($output) } finally { $source.Dispose(); $output.Dispose() }
          Check-Text ([IO.File]::ReadAllText($destination)) ($relative+'!'+$name)
        }
      } finally { $archive.Dispose() }
    } else {
      $destination = Join-Path (Join-Path $tree 'tracked') $relative
      [IO.Directory]::CreateDirectory((Split-Path $destination -Parent)) | Out-Null
      [IO.File]::Copy($path,$destination)
      Check-Text ([IO.File]::ReadAllText($path)) $relative
    }
  }
  if ($issues.Count) { throw ($issues -join [Environment]::NewLine) }
  $config = Join-Path $temp 'scan.toml'
  [IO.File]::WriteAllText($config,"[extend]"+[char]10+"useDefault = true"+[char]10)
  & $scanner git $repo --log-opts=HEAD --config $config --gitleaks-ignore-path $temp --ignore-gitleaks-allow --max-archive-depth 4 --max-decode-depth 5 --redact=100 --no-banner --no-color
  if ($LASTEXITCODE -ne 0) { throw 'Reachable history secret scan failed' }
  & $scanner dir $tree --config $config --gitleaks-ignore-path $temp --ignore-gitleaks-allow --max-decode-depth 5 --redact=100 --no-banner --no-color
  if ($LASTEXITCODE -ne 0) { throw 'Tracked/extracted content secret scan failed' }
  Write-Output "PASS: $($files.Count) tracked files, $count ZIP entries, reachable history; privacy indicators and Gitleaks."
} finally {
  # Only remove the exact temporary directory this invocation created.
  $resolved = [IO.Path]::GetFullPath($temp)
  $base = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\')+'\'
  if (!$resolved.StartsWith($base,[StringComparison]::OrdinalIgnoreCase) -or [IO.Path]::GetFileName($resolved) -notmatch '^ocmi-scan-[a-f0-9]{32}$') { throw 'Unsafe scanner cleanup target' }
  Remove-Item -LiteralPath $resolved -Recurse -Force
}
