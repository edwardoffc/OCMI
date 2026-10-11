param([string]$Root = (Join-Path $PSScriptRoot '..'))
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'path-utils.ps1')
$rootPath = (Resolve-Path -LiteralPath $Root).Path
$files = @()
foreach ($name in @('README.md', 'README.ru.md', 'AGENTS.md', 'BOOTSTRAP.md', 'COORDINATOR.md', 'VERSION.md', 'CHANGELOG.md', 'CONTRIBUTING.md', 'SECURITY.md', 'scripts/README.md', 'releases/README.md')) {
  $p = Join-Path $rootPath $name
  if (Test-Path -LiteralPath $p) { $files += Get-Item -LiteralPath $p }
}
foreach ($name in @('docs', 'templates', 'locales')) {
  $p = Join-Path $rootPath $name
  if (Test-Path -LiteralPath $p) { $files += Get-ChildItem -LiteralPath $p -Recurse -File -Filter '*.md' }
}
$issues = @()
$links = 0
foreach ($file in $files) {
  Assert-OCMISafePath $rootPath $file.FullName
  $fence = ''
  $number = 0
  foreach ($line in [IO.File]::ReadAllLines($file.FullName)) {
    $number++
    if ($line -match '^\s*(\x60{3,}|~{3,})') {
      $marker = $Matches[1].Substring(0, 1)
      if (!$fence) { $fence = $marker } elseif ($fence -eq $marker) { $fence = '' }
      continue
    }
    if ($fence) { continue }
    foreach ($match in [regex]::Matches($line, '\[[^\]\r\n]*\]\((?<target>[^)\r\n]+)\)')) {
      $href = ($match.Groups['target'].Value -replace '\s+"[^"]*"$', '').Trim().Trim('<', '>')
      if ($href -match '^[A-Za-z][A-Za-z0-9+.-]*:' -or $href.StartsWith('#') -or $href.StartsWith('//')) { continue }
      $href = [Uri]::UnescapeDataString(($href -split '[#?]', 2)[0])
      if (!$href) { continue }
      $links++
      $target = [IO.Path]::GetFullPath((Join-Path $file.DirectoryName $href))
      if (!(Test-OCMIContainedPath $rootPath $target) -or !(Test-Path -LiteralPath $target)) {
        $issues += "$($file.FullName):$($number): unresolved/outside link: $href"
      }
    }
  }
}
foreach ($p in @('templates/work/docs/memory/tasks/TEMPLATE.md', 'docs/memory/tasks/TEMPLATE.md', 'locales/ru/templates/work/docs/memory/tasks/TEMPLATE.md')) {
  $headings = @([IO.File]::ReadAllLines((Join-Path $rootPath $p)) | Where-Object { $_ -match '^## ' })
  if (($headings -join '|') -cne '## Contract|## Result|## Acceptance') { $issues += "Card schema mismatch: $p" }
}
$archives = 0
foreach ($file in Get-ChildItem -LiteralPath (Join-Path $rootPath 'releases') -Filter '*.zip.sha256' -File) {
  $archive = $file.FullName.Substring(0, $file.FullName.Length - 7)
  $expected = ([IO.File]::ReadAllText($file.FullName).Trim() -split '\s+')[0]
  if ($expected -notmatch '^[a-fA-F0-9]{64}$' -or !(Test-Path -LiteralPath $archive) -or (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash -ne $expected) {
    $issues += "Archive checksum mismatch: $($file.Name)"
  }
  $archives++
}
if ($issues.Count) { throw ($issues -join [Environment]::NewLine) }
Write-Output "PASS: $($files.Count) Markdown files; $links local links; 3 card schemas; $archives archive checksums."
