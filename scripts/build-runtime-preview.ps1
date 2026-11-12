[CmdletBinding()]
param(
    [string]$RepoRoot = (Join-Path $PSScriptRoot '..'),
    [string]$OutputRoot,
    [string]$PreviewId,
    [string]$NoticesManifest,
    [switch]$ReadyKit,
    [ValidateSet('windows-amd64', 'darwin-arm64', 'darwin-amd64', 'linux-amd64')]
    [string]$Target,
    [string]$SourceCommit,
    [switch]$SelfTest
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Resolve-FullPath {
    param([Parameter(Mandatory = $true)][string]$Path)
    if ([IO.Path]::IsPathRooted($Path)) {
        return [IO.Path]::GetFullPath($Path)
    }
    return [IO.Path]::GetFullPath((Join-Path (Get-Location).Path $Path))
}

function Test-DescendantPath {
    param([Parameter(Mandatory = $true)][string]$Parent, [Parameter(Mandatory = $true)][string]$Child)
    $relative = [IO.Path]::GetRelativePath($Parent, $Child)
    return $relative -ne '..' -and -not $relative.StartsWith('..' + [IO.Path]::DirectorySeparatorChar) -and -not [IO.Path]::IsPathRooted($relative)
}

function Assert-SafeRelativePath {
    param([Parameter(Mandatory = $true)][string]$Path)
    $normal = $Path.Replace('\', '/')
    if ([IO.Path]::IsPathRooted($Path) -or $normal -eq '' -or $normal.Contains(':') -or $normal.Split('/') -contains '..') {
        throw "Unsafe relative path: $Path"
    }
    return $normal
}

function Assert-NoReparseAncestor {
    param([Parameter(Mandatory = $true)][string]$Path)
    $candidate = Resolve-FullPath $Path
    while ($true) {
        if (Test-Path -LiteralPath $candidate) {
            $item = Get-Item -LiteralPath $candidate -Force
            if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
                throw "Reparse-point path is not permitted: $candidate"
            }
        }
        $parent = Split-Path -Path $candidate -Parent
        if ([string]::IsNullOrEmpty($parent) -or $parent -eq $candidate) { return }
        $candidate = $parent
    }
}

function Assert-RegularFile {
    param([Parameter(Mandatory = $true)][string]$Path)
    Assert-NoReparseAncestor $Path
    $item = Get-Item -LiteralPath $Path -Force
    if ($item.PSIsContainer -or (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0)) {
        throw "Expected a non-link file: $Path"
    }
    return $item
}

function Remove-VerifiedDirectory {
    param([Parameter(Mandatory = $true)][string]$Path, [Parameter(Mandatory = $true)][string]$AllowedParent)
    $target = Resolve-FullPath $Path
    $parent = Resolve-FullPath $AllowedParent
    if (-not (Test-DescendantPath $parent $target) -or -not (Test-Path -LiteralPath $target)) { throw "Refusing to remove outside the owned directory: $target" }
    Assert-NoReparseAncestor $parent
    $item = Get-Item -LiteralPath $target -Force
    if (-not $item.PSIsContainer -or (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0)) { throw "Refusing to remove a linked or non-directory target: $target" }
    foreach ($descendant in @(Get-ChildItem -LiteralPath $target -Force -Recurse)) {
        if (($descendant.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw "Refusing to remove staging containing a reparse point: $($descendant.FullName)" }
    }
    Remove-Item -LiteralPath $target -Recurse -Force
}

function Get-Sha256 {
    param([Parameter(Mandatory = $true)][string]$Path)
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Get-RuntimeSourceSnapshot {
    param([Parameter(Mandatory = $true)][string]$RepoRoot)
    $root = Resolve-FullPath $RepoRoot
    $runtimeRoot = Join-Path $root 'runtime'
    Assert-NoReparseAncestor $runtimeRoot
    $files = @(
        Get-ChildItem -LiteralPath $runtimeRoot -File -Recurse -Filter '*.go'
        Get-Item -LiteralPath (Join-Path $runtimeRoot 'go.mod')
        Get-Item -LiteralPath (Join-Path $runtimeRoot 'go.sum')
    )
    $records = foreach ($file in $files) {
        Assert-RegularFile $file.FullName | Out-Null
        $relative = [IO.Path]::GetRelativePath($root, $file.FullName).Replace('\', '/')
        if ($relative -notmatch '^runtime/' -or $relative -match '(^|/)\.(local|git)(/|$)') { throw "Unexpected runtime snapshot path: $relative" }
        [pscustomobject]@{ path = $relative; sha256 = Get-Sha256 $file.FullName }
    }
    $sorted = @($records | Sort-Object path)
    $material = (($sorted | ForEach-Object { "$($_.sha256)  $($_.path)" }) -join "`n") + "`n"
    $digest = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($material))).ToLowerInvariant()
    return [ordered]@{ sha256 = $digest; files = $sorted }
}

function Read-NoticeManifest {
    param([Parameter(Mandatory = $true)][string]$Path, [Parameter(Mandatory = $true)][string]$GoVersion, [Parameter(Mandatory = $true)][string[]]$Modules)
    $manifestPath = Resolve-FullPath $Path
    Assert-RegularFile $manifestPath | Out-Null
    $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
    if ($manifest.schemaVersion -ne 1 -or $null -eq $manifest.toolchain -or $manifest.toolchain.version -ne $GoVersion -or $null -eq $manifest.modules) {
        throw 'Notices manifest must be schemaVersion 1 and match the selected Go toolchain.'
    }
    $base = Split-Path -Path $manifestPath -Parent
    $entries = @()
    $seenModules = @{}
    foreach ($module in @($manifest.modules)) {
        if ([string]::IsNullOrWhiteSpace($module.path) -or [string]::IsNullOrWhiteSpace($module.version) -or [string]::IsNullOrWhiteSpace($module.notice) -or $module.sha256 -notmatch '^[0-9a-fA-F]{64}$') {
            throw 'Every module notice needs path, version, relative notice, and SHA256.'
        }
        $key = "$($module.path) $($module.version)"
        if ($seenModules.ContainsKey($key)) { throw "Duplicate module notice: $key" }
        $seenModules[$key] = $true
        $relative = Assert-SafeRelativePath ([string]$module.notice)
        $noticePath = Join-Path $base $relative
        Assert-RegularFile $noticePath | Out-Null
        if ((Get-Sha256 $noticePath) -ne $module.sha256.ToLowerInvariant()) { throw "Notice digest mismatch: $relative" }
        $entries += [pscustomobject]@{ ArchivePath = ('NOTICES/modules/' + $relative); SourcePath = $noticePath }
    }
    foreach ($module in $Modules) {
        if (-not $seenModules.ContainsKey($module)) { throw "Missing reviewed notice for linked module: $module" }
    }
    foreach ($known in $seenModules.Keys) {
        if ($Modules -notcontains $known) { throw "Notice is not applicable to this linked module set: $known" }
    }
    $toolchainRelative = Assert-SafeRelativePath ([string]$manifest.toolchain.notice)
    $toolchainPath = Join-Path $base $toolchainRelative
    Assert-RegularFile $toolchainPath | Out-Null
    if ([string]$manifest.toolchain.sha256 -notmatch '^[0-9a-fA-F]{64}$' -or (Get-Sha256 $toolchainPath) -ne $manifest.toolchain.sha256.ToLowerInvariant()) {
        throw 'Go toolchain notice is missing or has a different digest.'
    }
    $entries += [pscustomobject]@{ ArchivePath = ('NOTICES/go/' + $toolchainRelative); SourcePath = $toolchainPath }
    return $entries
}

function New-ArchiveFromAllowlist {
    param([Parameter(Mandatory = $true)][string]$ArchivePath, [Parameter(Mandatory = $true)][object[]]$Entries)
    Assert-NoReparseAncestor (Split-Path -Path $ArchivePath -Parent)
    if (Test-Path -LiteralPath $ArchivePath) { throw "Archive collision: $ArchivePath" }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $seen = New-Object 'System.Collections.Generic.HashSet[string]' ([StringComparer]::OrdinalIgnoreCase)
    foreach ($entry in $Entries) {
        $name = Assert-SafeRelativePath ([string]$entry.ArchivePath)
        Assert-RegularFile ([string]$entry.SourcePath) | Out-Null
        if (-not $seen.Add($name)) { throw "Archive entry collision: $name" }
    }
    $zip = [IO.Compression.ZipFile]::Open($ArchivePath, [IO.Compression.ZipArchiveMode]::Create)
    try {
        foreach ($entry in $Entries) {
            [IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, [string]$entry.SourcePath, (Assert-SafeRelativePath ([string]$entry.ArchivePath)), [IO.Compression.CompressionLevel]::Optimal) | Out-Null
        }
    } finally {
        $zip.Dispose()
    }
}

function New-TarGzipFromAllowlist {
    param([Parameter(Mandatory = $true)][string]$ArchivePath, [Parameter(Mandatory = $true)][object[]]$Entries, [Parameter(Mandatory = $true)][string]$ExecutableArchivePath)
    if (Test-Path -LiteralPath $ArchivePath) { throw "Archive collision: $ArchivePath" }
    Assert-NoReparseAncestor (Split-Path -Path $ArchivePath -Parent)
    $seen = New-Object 'System.Collections.Generic.HashSet[string]' ([StringComparer]::OrdinalIgnoreCase)
    foreach ($entry in $Entries) {
        $name = Assert-SafeRelativePath ([string]$entry.ArchivePath)
        Assert-RegularFile ([string]$entry.SourcePath) | Out-Null
        if (-not $seen.Add($name)) { throw "Archive entry collision: $name" }
    }
    $binaryName = Assert-SafeRelativePath $ExecutableArchivePath
    if (-not $seen.Contains($binaryName)) { throw "Unix executable is not in the archive allowlist: $binaryName" }
    $stream = [IO.File]::Open($ArchivePath, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
    $gzip = [IO.Compression.GZipStream]::new($stream, [IO.Compression.CompressionLevel]::Optimal, $false)
    $writer = [System.Formats.Tar.TarWriter]::new($gzip, $false)
    $fileMode = [IO.UnixFileMode]::UserRead -bor [IO.UnixFileMode]::UserWrite -bor [IO.UnixFileMode]::GroupRead -bor [IO.UnixFileMode]::OtherRead
    $executableMode = $fileMode -bor [IO.UnixFileMode]::UserExecute -bor [IO.UnixFileMode]::GroupExecute -bor [IO.UnixFileMode]::OtherExecute
    try {
        foreach ($entry in $Entries) {
            $name = Assert-SafeRelativePath ([string]$entry.ArchivePath)
            $tarEntry = [System.Formats.Tar.PaxTarEntry]::new([System.Formats.Tar.TarEntryType]::RegularFile, $name)
            $tarEntry.Mode = if ($name -eq $binaryName) { $executableMode } else { $fileMode }
            $input = [IO.File]::OpenRead([string]$entry.SourcePath)
            try { $tarEntry.DataStream = $input; $writer.WriteEntry($tarEntry) } finally { $input.Dispose() }
        }
    } finally {
        $writer.Dispose(); $gzip.Dispose(); $stream.Dispose()
    }
}

function Assert-ReadySourcePath {
    param([Parameter(Mandatory = $true)][string]$Path)
    $safe = Assert-SafeRelativePath $Path
    $normalized = if ($safe.StartsWith('system/', [StringComparison]::OrdinalIgnoreCase)) { $safe.Substring(7) } else { $safe }
    $parts = $normalized.Split('/')
    if ($parts | Where-Object { $_ -in @('.git', '.local', '.codex', '.ocmi', '__pycache__', 'node_modules') }) { throw "Private or unsafe source path: $safe" }
    if ($parts[0] -in @('OCMI_Sync', 'local')) { throw "Private or unsafe source path: $safe" }
    if ($normalized -match '^(?i:docs/(memory|research)(/|$)|locales/ru/docs/research(/|$)|diagnostics(/|$))' -or $normalized -in @('.github/workflows/environment-binding-probe.yml', '.github/workflows/runtime-group-probe.yml')) {
        throw "ReadyKit public source excludes development-only path: $safe"
    }
    if ($parts[0] -in @('bin', 'NOTICES') -or $normalized -in @('build-info.json', 'SHA256SUMS')) { throw "Source path reserves a generated package path: $safe" }
    return $safe
}

function Get-SystemSourceSnapshot {
    param([Parameter(Mandatory = $true)][object[]]$Entries)
    $records = @($Entries | ForEach-Object { [pscustomobject]@{ path = $_.ArchivePath; sha256 = Get-Sha256 $_.SourcePath } } | Sort-Object path)
    $material = (($records | ForEach-Object { "$($_.sha256)  $($_.path)" }) -join "`n") + "`n"
    return [ordered]@{ sha256 = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($material))).ToLowerInvariant(); files = $records }
}

function Write-Utf8NoBomFile {
    param([Parameter(Mandatory = $true)][string]$Path, [Parameter(Mandatory = $true)][string]$Value)
    $parent = Split-Path -Path $Path -Parent
    New-Item -ItemType Directory -Force -Path $parent | Out-Null
    [IO.File]::WriteAllText($Path, $Value, [Text.UTF8Encoding]::new($false))
}

function Get-ReadyHomeEntries {
    param([Parameter(Mandatory = $true)][string]$SourceRoot, [Parameter(Mandatory = $true)][string]$Stage)
    $required = @('rootReadme', 'rootAgent', 'syncReadme', 'coordinatorAgent', 'coordinatorIndex', 'coordinatorHandoff', 'startup')
    $expectedKeys = @('schemaVersion') + $required
    $strictUtf8 = [Text.UTF8Encoding]::new($false, $true)
    $texts = @{}
    foreach ($language in @('en', 'ru')) {
        $relative = if ($language -eq 'en') { 'packaging/home/text.json' } else { 'locales/ru/packaging/home/text.json' }
        $path = Join-Path $SourceRoot $relative.Replace('/', [IO.Path]::DirectorySeparatorChar)
        Assert-RegularFile $path | Out-Null
        $raw = [IO.File]::ReadAllBytes($path)
        try { $text = ($strictUtf8.GetString($raw) | ConvertFrom-Json -AsHashtable) } catch { throw "Home text JSON is invalid: $relative" }
        if ($text.Count -ne $expectedKeys.Count -or @($text.Keys | Where-Object { $_ -notin $expectedKeys }).Count -ne 0 -or @($expectedKeys | Where-Object { -not $text.Contains($_) }).Count -ne 0 -or $text['schemaVersion'] -isnot [long] -or $text['schemaVersion'] -ne 1) { throw "Home text schema is invalid: $relative" }
        foreach ($key in $required) {
            $value = $text[$key]
            if ($value -isnot [string] -or [string]::IsNullOrWhiteSpace($value)) { throw "Home text is invalid: $relative $key" }
            try { if ($strictUtf8.GetByteCount($value) -gt 32768) { throw "Home text is too long: $relative $key" } } catch { throw "Home text is invalid UTF-8: $relative $key" }
        }
        foreach ($token in @('<HOME>', '<MEMORY>', '<GUIDE>', '<PROJECT>', '<SOURCE>', '<LANGUAGE>')) {
            if ([regex]::Matches($text['startup'], [regex]::Escape($token)).Count -ne 1) { throw "Home startup tokens are invalid: $relative" }
        }
        $texts[$language] = $text
    }
    $marker = Join-Path $SourceRoot 'runtime\internal\installation-layout.json'
    Assert-RegularFile $marker | Out-Null
    $root = Join-Path $Stage 'home-root'
    $ignore = "/OCMI_Sync/`n/local/`n/.local/`n/.codex/`n"
    $attributes = "/OCMI_Sync export-ignore`n/local export-ignore`n/.local export-ignore`n/.codex export-ignore`n"
    $generated = @(
        @{ Path = '.ocmi-home.json'; Value = [IO.File]::ReadAllText($marker, [Text.UTF8Encoding]::new($false)) },
        @{ Path = 'README.md'; Value = $texts.en['rootReadme'] },
        @{ Path = 'README.ru.md'; Value = $texts.ru['rootReadme'] },
        @{ Path = 'AGENTS.md'; Value = $texts.en['rootAgent'] },
        @{ Path = '.gitignore'; Value = $ignore },
        @{ Path = '.gitattributes'; Value = $attributes }
    )
    $entries = @()
    foreach ($item in $generated) {
        $path = Join-Path $root $item.Path
        Write-Utf8NoBomFile -Path $path -Value $item.Value
        $entries += [pscustomobject]@{ ArchivePath = $item.Path; SourcePath = $path }
    }
    return $entries
}

function Assert-ReadyGitState {
    param([Parameter(Mandatory = $true)][string]$Root, [Parameter(Mandatory = $true)][string]$Commit)
    if ($Commit -notmatch '^[0-9a-f]{40}$') { throw '-SourceCommit must be an exact lowercase 40-character Git commit.' }
    $head = (& git -C $Root rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $head -ne $Commit) { throw "ReadyKit requires HEAD at -SourceCommit ($Commit)." }
    $status = @(& git -C $Root status --porcelain --untracked-files=all)
    if ($LASTEXITCODE -ne 0) { throw 'Could not inspect Git source state.' }
    $publicChanges = @($status | Where-Object { $_.Length -lt 4 -or $_.Substring(3).Replace('\', '/') -notmatch '^\.local(/|$)' })
    if ($publicChanges.Count -ne 0) { throw 'ReadyKit requires no modified, deleted, staged, or untracked public source files.' }
}

function Get-ReadyGitSourcePaths {
    param([Parameter(Mandatory = $true)][string]$Root, [Parameter(Mandatory = $true)][string]$Commit)
    $tree = @(& git -C $Root -c core.quotePath=false ls-tree -r --full-tree $Commit)
    if ($LASTEXITCODE -ne 0 -or $tree.Count -eq 0) { throw 'Could not read the indexed source tree.' }
    $seen = @{}
    foreach ($line in $tree) {
        if ($line -notmatch '^(100644|100755) blob [0-9a-f]{40}\t(.+)$') { throw "ReadyKit only accepts indexed regular source files: $line" }
        $relative = Assert-ReadySourcePath $Matches[2]
        if ($seen.ContainsKey($relative.ToLowerInvariant())) { throw "Case-colliding source path: $relative" }
        $seen[$relative.ToLowerInvariant()] = $relative
    }
    return @($seen.Values | Sort-Object)
}

function Get-ReadySourceEntries {
    param([Parameter(Mandatory = $true)][string]$Root, [Parameter(Mandatory = $true)][string]$Commit, [Parameter(Mandatory = $true)][string]$Stage)
    $expected = @{}
    foreach ($relative in (Get-ReadyGitSourcePaths -Root $Root -Commit $Commit)) {
        $expected[$relative.ToLowerInvariant()] = $relative
    }
    $sourceZip = Join-Path $Stage 'source-revision.zip'
    & git -C $Root archive --format=zip "--output=$sourceZip" $Commit
    if ($LASTEXITCODE -ne 0) { throw 'Could not create the exact Git source archive.' }
    $sourceRoot = Join-Path $Stage 'source'
    New-Item -ItemType Directory -Path $sourceRoot | Out-Null
    $zip = [IO.Compression.ZipFile]::OpenRead($sourceZip)
    $seen = @{}
    try {
        foreach ($entry in $zip.Entries) {
            if ($entry.FullName.EndsWith('/')) { continue }
            $relative = Assert-ReadySourcePath $entry.FullName
            $key = $relative.ToLowerInvariant()
            if (-not $expected.ContainsKey($key) -or $seen.ContainsKey($key)) { throw "Git archive inventory mismatch: $relative" }
            $destination = Join-Path $sourceRoot $relative.Replace('/', [IO.Path]::DirectorySeparatorChar)
            $parent = Split-Path -Path $destination -Parent
            New-Item -ItemType Directory -Force -Path $parent | Out-Null
            Assert-NoReparseAncestor $parent
            $input = $entry.Open(); $output = [IO.File]::Create($destination)
            try { $input.CopyTo($output) } finally { $output.Dispose(); $input.Dispose() }
            Assert-RegularFile $destination | Out-Null
            $seen[$key] = $destination
        }
    } finally { $zip.Dispose() }
    if ($seen.Count -ne $expected.Count) { throw 'Git archive omitted indexed source files.' }
    Remove-Item -LiteralPath $sourceZip -Force
    return @($expected.Values | Sort-Object | ForEach-Object { [pscustomobject]@{ ArchivePath = $_; SourcePath = $seen[$_.ToLowerInvariant()] } })
}

function Assert-TargetBinary {
    param([Parameter(Mandatory = $true)][string]$Path, [Parameter(Mandatory = $true)][hashtable]$TargetInfo)
    $raw = [IO.File]::ReadAllBytes($Path)
    if ($raw.Length -lt 64) { throw "Built binary is too short: $Path" }
    $targetName = "$($TargetInfo.GOOS)-$($TargetInfo.GOARCH)"
    if ($targetName -eq 'windows-amd64') {
        $offset = [BitConverter]::ToInt32($raw, 60)
        if ($raw[0] -ne 0x4d -or $raw[1] -ne 0x5a -or $offset -lt 0 -or $offset + 6 -gt $raw.Length -or $raw[$offset + 4] -ne 0x64 -or $raw[$offset + 5] -ne 0x86) { throw 'Built binary has the wrong Windows header.' }
    } elseif ($targetName -like 'darwin-*') {
        $cpu = [BitConverter]::ToUInt32($raw, 4)
        $expected = if ($TargetInfo.GOARCH -eq 'arm64') { 0x100000c } else { 0x1000007 }
        if ($raw[0] -ne 0xcf -or $raw[1] -ne 0xfa -or $raw[2] -ne 0xed -or $raw[3] -ne 0xfe -or $cpu -ne $expected) { throw 'Built binary has the wrong Darwin header.' }
    } elseif ($raw[0] -ne 0x7f -or $raw[1] -ne 0x45 -or $raw[2] -ne 0x4c -or $raw[3] -ne 0x46 -or [BitConverter]::ToUInt16($raw, 18) -ne 62) { throw 'Built binary has the wrong Linux header.' }
}

function Assert-ReadySourceStability {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [Parameter(Mandatory = $true)][string]$Commit,
        [Parameter(Mandatory = $true)][object[]]$SourceEntries,
        [Parameter(Mandatory = $true)][System.Collections.IDictionary]$ExpectedSystem,
        [Parameter(Mandatory = $true)][string]$SourceRoot,
        [Parameter(Mandatory = $true)][System.Collections.IDictionary]$ExpectedRuntime
    )
    Assert-ReadyGitState -Root $Root -Commit $Commit
    if ((Get-SystemSourceSnapshot $SourceEntries).sha256 -ne $ExpectedSystem.sha256) { throw 'Exported full source changed during packaging.' }
    if ((Get-RuntimeSourceSnapshot $SourceRoot).sha256 -ne $ExpectedRuntime.sha256) { throw 'Exported runtime source changed during packaging.' }
}

function Invoke-ReadyKit {
    if ([string]::IsNullOrWhiteSpace($SourceCommit)) { throw '-SourceCommit is required with -ReadyKit.' }
    if ([string]::IsNullOrWhiteSpace($NoticesManifest)) { throw '-NoticesManifest is required for a ReadyKit build.' }
    $root = Resolve-FullPath $RepoRoot
    Assert-NoReparseAncestor $root
    Assert-ReadyGitState -Root $root -Commit $SourceCommit
    Get-ReadyGitSourcePaths -Root $root -Commit $SourceCommit | Out-Null
    if ([string]::IsNullOrWhiteSpace($PreviewId)) { $PreviewId = 'ocmi-ready-' + (Get-Date -Format 'yyyyMMddTHHmmssfff') + '-' + [guid]::NewGuid().ToString('N').Substring(0, 8) }
    if ($PreviewId -notmatch '^ocmi-ready-[A-Za-z0-9-]+$') { throw 'ReadyKit PreviewId must begin with ocmi-ready- and contain only letters, digits, and hyphens.' }
    if ([string]::IsNullOrWhiteSpace($OutputRoot)) { $OutputRoot = Join-Path $root '.local\build\distributions' }
    $output = Resolve-FullPath $OutputRoot
    if ((Test-DescendantPath $root $output) -and -not (Test-DescendantPath (Resolve-FullPath (Join-Path $root '.local')) $output)) { throw 'OutputRoot inside the repository must stay under its ignored .local directory.' }
    Assert-NoReparseAncestor $output
    $distribution = Join-Path $output $PreviewId
    if (Test-Path -LiteralPath $distribution) { throw "Distribution collision: $distribution" }
    $manifest = Resolve-FullPath $NoticesManifest
    $expectedManifest = Resolve-FullPath (Join-Path $root 'packaging\notices\manifest.json')
    if ($manifest -ne $expectedManifest) { throw 'ReadyKit notices must use the reviewed source manifest.' }
    $goVersion = (& go version).Trim()
    if ($LASTEXITCODE -ne 0 -or $goVersion -notmatch '^go version (go1\.26\.2) ') { throw "Go 1.26.2 is required; observed: $goVersion" }
    $declaredTargets = @(@{ GOOS = 'windows'; GOARCH = 'amd64'; Executable = 'ocmi.exe' }, @{ GOOS = 'darwin'; GOARCH = 'arm64'; Executable = 'ocmi' }, @{ GOOS = 'darwin'; GOARCH = 'amd64'; Executable = 'ocmi' }, @{ GOOS = 'linux'; GOARCH = 'amd64'; Executable = 'ocmi' })
    $targets = if ($Target) { @($declaredTargets | Where-Object { "$($_.GOOS)-$($_.GOARCH)" -eq $Target }) } else { $declaredTargets }
    New-Item -ItemType Directory -Path $distribution | Out-Null
    $staging = Join-Path $distribution 'staging'; New-Item -ItemType Directory -Path $staging | Out-Null
    $previous = @{ CGO_ENABLED = $env:CGO_ENABLED; GOPROXY = $env:GOPROXY; GOSUMDB = $env:GOSUMDB; GOOS = $env:GOOS; GOARCH = $env:GOARCH; GOFLAGS = $env:GOFLAGS; GOTOOLCHAIN = $env:GOTOOLCHAIN; GOENV = $env:GOENV; GOWORK = $env:GOWORK }
    try {
        $env:CGO_ENABLED = '0'; $env:GOPROXY = 'off'; $env:GOSUMDB = 'off'; $env:GOTOOLCHAIN = 'local'; $env:GOENV = 'off'; $env:GOWORK = 'off'
        $env:GOFLAGS = '-buildvcs=false -mod=readonly'
        $sourceEntries = Get-ReadySourceEntries -Root $root -Commit $SourceCommit -Stage $staging
        $systemSnapshot = Get-SystemSourceSnapshot $sourceEntries
        $sourceRoot = Join-Path $staging 'source'
        $sourceManifest = Join-Path $sourceRoot 'packaging\notices\manifest.json'
        $runtimeSnapshot = Get-RuntimeSourceSnapshot $sourceRoot
        $moduleRoot = Join-Path $sourceRoot 'runtime'; Push-Location $moduleRoot
        try {
            $linked = [ordered]@{}
            foreach ($item in $declaredTargets) { $env:GOOS = $item.GOOS; $env:GOARCH = $item.GOARCH; $lines = @(& go list -mod=readonly -deps -f '{{if and (not .Standard) .Module (not .Module.Main)}}{{.Module.Path}} {{.Module.Version}}{{end}}' ./cmd/ocmi | Where-Object { $_ -and $_.Trim() } | Sort-Object -Unique); if ($LASTEXITCODE -ne 0 -or $lines.Count -eq 0) { throw "Could not determine linked modules for $($item.GOOS)/$($item.GOARCH)." }; $linked["$($item.GOOS)/$($item.GOARCH)"] = $lines }
            $union = @($linked.Values | ForEach-Object { $_ } | Sort-Object -Unique); $notices = Read-NoticeManifest -Path $sourceManifest -GoVersion $Matches[1] -Modules $union
            $moduleGraph = @(& go list -mod=readonly -m -f '{{.Path}} {{.Version}} {{.Sum}}' all | Where-Object { $_ -and $_.Trim() }); if ($LASTEXITCODE -ne 0) { throw 'Could not record the pinned module graph.' }
            $built = @(); foreach ($item in $targets) { $binary = Join-Path $staging ("$($item.GOOS)-$($item.GOARCH)-$($item.Executable)"); $env:GOOS = $item.GOOS; $env:GOARCH = $item.GOARCH; & go build -mod=readonly -trimpath -o $binary ./cmd/ocmi; if ($LASTEXITCODE -ne 0) { throw "Build failed for $($item.GOOS)/$($item.GOARCH)." }; Assert-TargetBinary -Path $binary -TargetInfo $item; $built += [pscustomobject]@{ Target = $item; Binary = $binary } }
        } finally { Pop-Location }
        Assert-ReadySourceStability -Root $root -Commit $SourceCommit -SourceEntries $sourceEntries -ExpectedSystem $systemSnapshot -SourceRoot $sourceRoot -ExpectedRuntime $runtimeSnapshot
        $systemVersion = ([regex]::Match((Get-Content -LiteralPath (Join-Path $sourceRoot 'VERSION.md') -Raw), 'OCMI\s+(\d+\.\d+\.\d+)')).Groups[1].Value
        if ([string]::IsNullOrWhiteSpace($systemVersion)) { throw 'Could not determine the system version from the exact source revision.' }
        $homeEntries = Get-ReadyHomeEntries -SourceRoot $sourceRoot -Stage $staging
        foreach ($item in $built) {
            $targetName = "$($item.Target.GOOS)-$($item.Target.GOARCH)"
            $binaryPath = if ($item.Target.GOOS -eq 'windows') { 'system/bin/ocmi.exe' } else { 'system/bin/ocmi' }
            $compatibility = [ordered]@{ schemaVersion = 1; layoutVersion = 1; installationReceiptVersion = 1; coordinatorMemoryVersion = 1; coordinatorBindingVersion = 1; stateSchemaVersion = 4 }
            $info = [ordered]@{ previewId = $PreviewId; distributionKind = 'ready-kit'; systemVersion = $systemVersion; sourceCommit = $SourceCommit; sourceDirty = $false; layoutVersion = 1; layoutDeclaration = '.ocmi-home.json'; binaryPath = $binaryPath; target = $targetName; updateCompatibility = $compatibility; toolStorageProfileFormat = 2; sourceSnapshot = $runtimeSnapshot; systemSnapshot = $systemSnapshot; goVersion = $goVersion; cgoEnabled = '0'; moduleGraph = $moduleGraph; linkedModulesByTarget = $linked; linkedModulesUnion = $union }
            $infoPath = Join-Path $staging ("$targetName-build-info.json"); Set-Content -LiteralPath $infoPath -Value ($info | ConvertTo-Json -Depth 10) -Encoding utf8NoBOM
            $systemSourceEntries = @($sourceEntries | ForEach-Object { [pscustomobject]@{ ArchivePath = 'system/' + $_.ArchivePath; SourcePath = $_.SourcePath } })
            $systemNotices = @($notices | ForEach-Object { [pscustomobject]@{ ArchivePath = 'system/' + $_.ArchivePath; SourcePath = $_.SourcePath } })
            $entries = @($homeEntries) + $systemSourceEntries + $systemNotices + @([pscustomobject]@{ ArchivePath = $binaryPath; SourcePath = $item.Binary }, [pscustomobject]@{ ArchivePath = 'system/build-info.json'; SourcePath = $infoPath })
            $names = New-Object 'System.Collections.Generic.HashSet[string]' ([StringComparer]::OrdinalIgnoreCase); foreach ($entry in $entries) { if (-not $names.Add((Assert-SafeRelativePath $entry.ArchivePath))) { throw "Archive entry collision: $($entry.ArchivePath)" } }
            $sumsPath = Join-Path $staging ("$targetName-SHA256SUMS"); Set-Content -LiteralPath $sumsPath -Value (($entries | ForEach-Object { "$(Get-Sha256 $_.SourcePath)  $($_.ArchivePath.Replace('\\', '/'))" }) -join "`n") -Encoding utf8NoBOM
            $entries += [pscustomobject]@{ ArchivePath = 'SHA256SUMS'; SourcePath = $sumsPath }
            $archive = Join-Path $distribution ("$PreviewId-$targetName" + $(if ($item.Target.GOOS -eq 'windows') { '.zip' } else { '.tar.gz' }))
            if ($item.Target.GOOS -eq 'windows') { New-ArchiveFromAllowlist -ArchivePath $archive -Entries $entries } else { New-TarGzipFromAllowlist -ArchivePath $archive -Entries $entries -ExecutableArchivePath $binaryPath }
        }
        $archives = @(Get-ChildItem -LiteralPath $distribution -File | Where-Object { $_.Name -match '\.(zip|tar\.gz)$' } | Sort-Object Name)
        Set-Content -LiteralPath (Join-Path $distribution 'SHA256SUMS-archives.txt') -Value (($archives | ForEach-Object { "$(Get-Sha256 $_.FullName)  $($_.Name)" }) -join "`n") -Encoding utf8NoBOM
        Assert-ReadySourceStability -Root $root -Commit $SourceCommit -SourceEntries $sourceEntries -ExpectedSystem $systemSnapshot -SourceRoot $sourceRoot -ExpectedRuntime $runtimeSnapshot
        Remove-VerifiedDirectory -Path $staging -AllowedParent $distribution
        Write-Output "Created ready distribution: $distribution"
    } finally { foreach ($key in $previous.Keys) { if ($null -eq $previous[$key]) { Remove-Item -Path ("env:" + $key) -ErrorAction SilentlyContinue } else { Set-Item -Path ("env:" + $key) -Value $previous[$key] } } }
}

function Invoke-PackagingSelfTest {
    $testRoot = Join-Path ([IO.Path]::GetTempPath()) ('ocmi-runtime-preview-selftest-' + [guid]::NewGuid().ToString('N'))
    try {
        New-Item -ItemType Directory -Path $testRoot | Out-Null
        $safe = Join-Path $testRoot 'safe.txt'
        $privateDirectory = Join-Path $testRoot '.local\coordinator'
        New-Item -ItemType Directory -Force -Path $privateDirectory | Out-Null
        Set-Content -LiteralPath $safe -Value 'synthetic safe content' -NoNewline
        Set-Content -LiteralPath (Join-Path $privateDirectory 'secret.txt') -Value 'synthetic private content' -NoNewline
        $archive = Join-Path $testRoot 'safe.zip'
        $allowlist = @([pscustomobject]@{ ArchivePath = 'safe.txt'; SourcePath = $safe })
        New-ArchiveFromAllowlist -ArchivePath $archive -Entries $allowlist
        $zip = [IO.Compression.ZipFile]::OpenRead($archive)
        try {
            $names = @($zip.Entries | ForEach-Object { $_.FullName })
            if ($names.Count -ne 1 -or $names[0] -ne 'safe.txt' -or $names -match 'secret|\.local') { throw 'Allowlist failed to exclude private staging content.' }
        } finally { $zip.Dispose() }
        $collisionRejected = $false
        try { New-ArchiveFromAllowlist -ArchivePath (Join-Path $testRoot 'collision.zip') -Entries @($allowlist[0], $allowlist[0]) } catch { $collisionRejected = $true }
        if (-not $collisionRejected) { throw 'Archive entry collision was accepted.' }
        $traversalRejected = $false
        try { Assert-SafeRelativePath '../escape.txt' | Out-Null } catch { $traversalRejected = $true }
        if (-not $traversalRejected) { throw 'Traversal path was accepted.' }
        $outputCollisionRejected = $false
        try { New-ArchiveFromAllowlist -ArchivePath $archive -Entries $allowlist } catch { $outputCollisionRejected = $true }
        if (-not $outputCollisionRejected) { throw 'Existing archive was overwritten.' }
        $unixBinary = Join-Path $testRoot 'ocmi'
        Set-Content -LiteralPath $unixBinary -Value 'synthetic executable' -NoNewline
        $tarArchive = Join-Path $testRoot 'safe.tar.gz'
        New-TarGzipFromAllowlist -ArchivePath $tarArchive -Entries @([pscustomobject]@{ ArchivePath = 'ocmi'; SourcePath = $unixBinary }, $allowlist[0]) -ExecutableArchivePath 'ocmi'
        $tarStream = [IO.File]::OpenRead($tarArchive)
        $gzipStream = [IO.Compression.GZipStream]::new($tarStream, [IO.Compression.CompressionMode]::Decompress, $false)
        $reader = [System.Formats.Tar.TarReader]::new($gzipStream, $false)
        try {
            $tarEntries = @()
            while ($null -ne ($tarEntry = $reader.GetNextEntry())) { $tarEntries += $tarEntry }
            $unixEntry = @($tarEntries | Where-Object { $_.Name -eq 'ocmi' })
            if ($tarEntries.Count -ne 2 -or $unixEntry.Count -ne 1 -or (($unixEntry[0].Mode -band [IO.UnixFileMode]::UserExecute) -eq 0) -or ($tarEntries.Name -match 'secret|\.local')) { throw 'Tar allowlist or executable mode check failed.' }
        } finally { $reader.Dispose(); $gzipStream.Dispose(); $tarStream.Dispose() }
        Write-Output 'Runtime preview staging self-test passed: allowlist, private exclusion, traversal, and collisions.'
    } finally {
        if (Test-Path -LiteralPath $testRoot) { Remove-VerifiedDirectory -Path $testRoot -AllowedParent ([IO.Path]::GetTempPath()) }
    }
}

if ($SelfTest) {
    Invoke-PackagingSelfTest
    return
}

if ($ReadyKit) {
    Invoke-ReadyKit
    return
}

if ([string]::IsNullOrWhiteSpace($NoticesManifest)) { throw '-NoticesManifest is required for a real preview build.' }
$root = Resolve-FullPath $RepoRoot
if (-not (Test-Path -LiteralPath (Join-Path $root 'runtime\go.mod'))) { throw "Not an OCMI runtime root: $root" }
$defaultOutput = Join-Path $root '.local\build\distributions'
if ([string]::IsNullOrWhiteSpace($OutputRoot)) { $OutputRoot = $defaultOutput }
$output = Resolve-FullPath $OutputRoot
if ((Test-DescendantPath $root $output) -and -not (Test-DescendantPath (Resolve-FullPath (Join-Path $root '.local')) $output)) {
    throw 'OutputRoot inside the repository must stay under its ignored .local directory.'
}
Assert-NoReparseAncestor $root
Assert-NoReparseAncestor $output
if ([string]::IsNullOrWhiteSpace($PreviewId)) { $PreviewId = 'runtime-preview-' + (Get-Date -Format 'yyyyMMddTHHmmssfff') + '-' + [guid]::NewGuid().ToString('N').Substring(0, 8) }
if ($PreviewId -notmatch '^runtime-preview-[A-Za-z0-9-]+$') { throw 'PreviewId must begin with runtime-preview- and contain only letters, digits, and hyphens.' }
$distribution = Join-Path $output $PreviewId
if (Test-Path -LiteralPath $distribution) { throw "Distribution collision: $distribution" }

$goVersion = (& go version).Trim()
if ($LASTEXITCODE -ne 0 -or $goVersion -notmatch '^go version (go1\.26\.2) ') { throw "Go 1.26.2 is required; observed: $goVersion" }
$toolchainVersion = $Matches[1]
$moduleRoot = Join-Path $root 'runtime'
$previousCgo = $env:CGO_ENABLED
$previousProxy = $env:GOPROXY
$previousSumDb = $env:GOSUMDB
$previousGoos = $env:GOOS
$previousGoarch = $env:GOARCH
try {
    $env:CGO_ENABLED = '0'
    $env:GOPROXY = 'off'
    $env:GOSUMDB = 'off'
    Push-Location $moduleRoot
    try {
        $targets = @(@{ GOOS = 'windows'; GOARCH = 'amd64'; Executable = 'ocmi.exe' }, @{ GOOS = 'darwin'; GOARCH = 'arm64'; Executable = 'ocmi' }, @{ GOOS = 'darwin'; GOARCH = 'amd64'; Executable = 'ocmi' }, @{ GOOS = 'linux'; GOARCH = 'amd64'; Executable = 'ocmi' })
        $linkedModulesByTarget = [ordered]@{}
        foreach ($target in $targets) {
            $env:GOOS = $target.GOOS; $env:GOARCH = $target.GOARCH
            $lines = @(& go list -mod=readonly -deps -f '{{if and (not .Standard) .Module (not .Module.Main)}}{{.Module.Path}} {{.Module.Version}}{{end}}' ./cmd/ocmi | Where-Object { $_ -and $_.Trim() } | Sort-Object -Unique)
            if ($LASTEXITCODE -ne 0 -or $lines.Count -eq 0) { throw "Could not determine the pinned linked module set for $($target.GOOS)/$($target.GOARCH) without downloading dependencies." }
            $linkedModulesByTarget[($target.GOOS + '/' + $target.GOARCH)] = $lines
        }
        $moduleLines = @($linkedModulesByTarget.Values | ForEach-Object { $_ } | Sort-Object -Unique)
        if ($LASTEXITCODE -ne 0 -or $moduleLines.Count -eq 0) { throw 'Could not determine the pinned linked module set without downloading dependencies.' }
        $notices = Read-NoticeManifest -Path $NoticesManifest -GoVersion $toolchainVersion -Modules $moduleLines
        $moduleGraph = @(& go list -mod=readonly -m -f '{{.Path}} {{.Version}} {{.Sum}}' all | Where-Object { $_ -and $_.Trim() })
        if ($LASTEXITCODE -ne 0) { throw 'Could not record the pinned module graph.' }
        New-Item -ItemType Directory -Path $distribution | Out-Null
        $staging = Join-Path $distribution 'staging'
        New-Item -ItemType Directory -Path $staging | Out-Null
        $sourceEntries = @(
            [pscustomobject]@{ ArchivePath = 'README.md'; SourcePath = (Join-Path $root 'docs\runtime.md') },
            [pscustomobject]@{ ArchivePath = 'README.ru.md'; SourcePath = (Join-Path $root 'locales\ru\docs\runtime.md') },
            [pscustomobject]@{ ArchivePath = 'LICENSE'; SourcePath = (Join-Path $root 'LICENSE') },
            [pscustomobject]@{ ArchivePath = 'examples\README.md'; SourcePath = (Join-Path $root 'runtime\examples\README.md') },
            [pscustomobject]@{ ArchivePath = 'examples\init-request.json'; SourcePath = (Join-Path $root 'runtime\examples\init-request.json') }
        )
        foreach ($entry in $sourceEntries) { Assert-RegularFile $entry.SourcePath | Out-Null }
        $sourceSnapshotBefore = Get-RuntimeSourceSnapshot $root
        $builtTargets = @()
        foreach ($target in $targets) {
            $binary = Join-Path $staging ($target.GOOS + '-' + $target.GOARCH + '-' + $target.Executable)
            $env:GOOS = $target.GOOS; $env:GOARCH = $target.GOARCH
            & go build -mod=readonly -trimpath -o $binary ./cmd/ocmi
            if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $binary)) { throw "Build failed for $($target.GOOS)/$($target.GOARCH); no archive was reused." }
            $builtTargets += [pscustomobject]@{ Target = $target; Binary = $binary }
        }
        $sourceSnapshotAfter = Get-RuntimeSourceSnapshot $root
        if ($sourceSnapshotBefore.sha256 -ne $sourceSnapshotAfter.sha256) { throw 'Runtime source changed during build; refusing to package an ambiguous snapshot.' }
        $buildInfo = [ordered]@{ previewId = $PreviewId; sourceCommit = (& git -C $root rev-parse HEAD).Trim(); sourceDirty = [bool](& git -C $root status --porcelain); sourceSnapshot = $sourceSnapshotAfter; goVersion = $goVersion; cgoEnabled = $env:CGO_ENABLED; moduleGraph = $moduleGraph; linkedModulesByTarget = $linkedModulesByTarget; linkedModulesUnion = $moduleLines }
        Set-Content -LiteralPath (Join-Path $staging 'build-info.json') -Value ($buildInfo | ConvertTo-Json -Depth 8) -Encoding utf8NoBOM
        $archives = @()
        foreach ($built in $builtTargets) {
            $target = $built.Target
            $binary = $built.Binary
            $entries = @([pscustomobject]@{ ArchivePath = $target.Executable; SourcePath = $binary }) + $sourceEntries + $notices + @([pscustomobject]@{ ArchivePath = 'build-info.json'; SourcePath = (Join-Path $staging 'build-info.json') })
            $checksums = foreach ($entry in $entries) { "$(Get-Sha256 $entry.SourcePath)  $($entry.ArchivePath.Replace('\', '/'))" }
            $checksumFile = Join-Path $staging ($target.GOOS + '-' + $target.GOARCH + '-SHA256SUMS')
            Set-Content -LiteralPath $checksumFile -Value $checksums -Encoding ascii
            $entries += [pscustomobject]@{ ArchivePath = 'SHA256SUMS'; SourcePath = $checksumFile }
            if ($target.GOOS -eq 'windows') {
                $archive = Join-Path $distribution ("$PreviewId-$($target.GOOS)-$($target.GOARCH).zip")
                New-ArchiveFromAllowlist -ArchivePath $archive -Entries $entries
            } else {
                $archive = Join-Path $distribution ("$PreviewId-$($target.GOOS)-$($target.GOARCH).tar.gz")
                New-TarGzipFromAllowlist -ArchivePath $archive -Entries $entries -ExecutableArchivePath $target.Executable
            }
            $archives += $archive
        }
        $archiveManifest = foreach ($archive in $archives) { "$(Get-Sha256 $archive)  $([IO.Path]::GetFileName($archive))" }
        Set-Content -LiteralPath (Join-Path $distribution 'SHA256SUMS-archives.txt') -Value $archiveManifest -Encoding ascii
        Remove-VerifiedDirectory -Path $staging -AllowedParent $distribution
        Write-Output "Created preview distribution: $distribution"
    } finally { Pop-Location }
} finally {
    $env:CGO_ENABLED = $previousCgo; $env:GOPROXY = $previousProxy; $env:GOSUMDB = $previousSumDb; $env:GOOS = $previousGoos; $env:GOARCH = $previousGoarch
}
