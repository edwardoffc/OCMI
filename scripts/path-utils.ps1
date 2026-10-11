# Shared filesystem guards. Compatible with Windows PowerShell 5.1 and PowerShell 7.
function Get-OCMIPathComparison {
  if ([Environment]::OSVersion.Platform -eq [PlatformID]::Win32NT) { return [StringComparison]::OrdinalIgnoreCase }
  return [StringComparison]::Ordinal
}
function Get-OCMIPathPrefix([string]$Path) {
  $full = [IO.Path]::GetFullPath($Path)
  return $full.TrimEnd([char[]]@([IO.Path]::DirectorySeparatorChar,[IO.Path]::AltDirectorySeparatorChar)) + [IO.Path]::DirectorySeparatorChar
}
function Test-OCMIContainedPath([string]$Base,[string]$Path) {
  return [IO.Path]::GetFullPath($Path).StartsWith((Get-OCMIPathPrefix $Base),(Get-OCMIPathComparison))
}
function Assert-OCMIRelativePath([string]$Relative) {
  if (!$Relative -or $Relative -match ':|^[/\\]|(^|[/\\])\.\.([/\\]|$)') { throw 'Unsafe relative path' }
}
function Assert-OCMISafePath([string]$Base,[string]$Path) {
  $root = [IO.Path]::GetFullPath($Base).TrimEnd([char[]]@([IO.Path]::DirectorySeparatorChar,[IO.Path]::AltDirectorySeparatorChar))
  if (!$root) { $root = [IO.Path]::GetPathRoot([IO.Path]::GetFullPath($Base)) }
  if ($root -match '^[A-Za-z]:$') { $root += [IO.Path]::DirectorySeparatorChar }
  $full = [IO.Path]::GetFullPath($Path)
  if (!(Test-OCMIContainedPath $root $full)) { throw 'Path outside input root' }
  $item = Get-Item -LiteralPath $full -Force -ErrorAction Stop
  while ($null -ne $item) {
    if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Symlink/reparse input' }
    if ($item.FullName.Equals($root,(Get-OCMIPathComparison))) { return }
    $item = if ($item -is [IO.FileInfo]) { $item.Directory } else { $item.Parent }
  }
  throw 'Input root not reached'
}
function Get-OCMIScopedFile([string]$Base,[string]$Relative) {
  Assert-OCMIRelativePath $Relative
  $native = $Relative.Replace('\',[string][IO.Path]::DirectorySeparatorChar).Replace('/',[string][IO.Path]::DirectorySeparatorChar)
  $full = [IO.Path]::GetFullPath((Join-Path $Base $native))
  if (!(Test-Path -LiteralPath $full -PathType Leaf)) { throw 'Missing input file' }
  Assert-OCMISafePath $Base $full
  return $full
}
function Get-OCMIRelativeFile([string]$Base,[string]$Path) {
  if (!(Test-OCMIContainedPath $Base $Path)) { throw 'File outside relative root' }
  return [IO.Path]::GetFullPath($Path).Substring((Get-OCMIPathPrefix $Base).Length).Replace('\','/')
}
function Assert-OCMITemporaryDirectory([string]$Path,[string]$Parent,[string]$Prefix) {
  $full = [IO.Path]::GetFullPath($Path)
  if (!(Test-OCMIContainedPath $Parent $full) -or [IO.Path]::GetFileName($full) -cnotmatch ('^'+[regex]::Escape($Prefix)+'[a-f0-9]{32}$')) { throw 'Unsafe temporary cleanup target' }
  $actualParent = [IO.Path]::GetDirectoryName($full)
  $expectedParent = [IO.Path]::GetFullPath($Parent).TrimEnd([char[]]@([IO.Path]::DirectorySeparatorChar,[IO.Path]::AltDirectorySeparatorChar))
  if (!$actualParent.Equals($expectedParent,(Get-OCMIPathComparison))) { throw 'Temporary directory is not a direct child' }
  if ((Test-Path -LiteralPath $full) -and ((Get-Item -LiteralPath $full -Force).Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Reparse temporary cleanup target' }
}
