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
            $tarEntry.Uid = 0; $tarEntry.Gid = 0
            $tarEntry.UserName = ''; $tarEntry.GroupName = ''
            $tarEntry.Mode = if ($name -eq $binaryName) { $executableMode } else { $fileMode }
            $input = [IO.File]::OpenRead([string]$entry.SourcePath)
            try { $tarEntry.DataStream = $input; $writer.WriteEntry($tarEntry) } finally { $input.Dispose() }
        }
    } finally {
        $writer.Dispose(); $gzip.Dispose(); $stream.Dispose()
    }
}

function Assert-ReadyArchiveInputs {
    param([object[]]$Entries)
    if ($Entries.Count -lt 1 -or $Entries.Count -gt 4096) { throw 'ReadyKit file-count bound exceeded.' }
    $names = [Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
    $parents = @{}; [long]$total = 0
    foreach ($entry in $Entries) {
        $name = Assert-SafeRelativePath ([string]$entry.ArchivePath)
        $parts = $name.Split('/')
        if ($name -match '^(?i:local|OCMI_Sync)(/|$)|^system/(?i:local|OCMI_Sync)(/|$)|(^|/)(?i:\.git|\.local|\.codex|\.ocmi|__pycache__|node_modules)(/|$)|^system/(?i:docs/(memory|research)|diagnostics)(/|$)') { throw 'Private ReadyKit path refused.' }
        if ($name -cmatch '[^\x20-\x7e]' -or $name.Length -gt 1024 -or $parts.Count -gt 32 -or !$names.Add($name)) { throw 'Unsupported ReadyKit name/count profile.' }
        foreach ($part in $parts) {
            if (!$part -or $part -in @('.','..') -or $part.TrimEnd('.',' ') -cne $part -or $part -match '[<>:"|?*]' -or $part -match '^(?i:con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)') { throw 'Unsupported ReadyKit path alias.' }
        }
        for ($i=1; $i -lt $parts.Count; $i++) {
            $parent = $parts[0..($i-1)] -join '/'
            if ($parents.ContainsKey($parent) -and $parents[$parent] -cne $parent) { throw 'ReadyKit parent case alias.' }
            $parents[$parent] = $parent
        }
        $file = Assert-RegularFile ([string]$entry.SourcePath)
        $length = $file.Length
        if ($length -gt 32MB -or $total -gt 256MB-$length) { throw 'ReadyKit payload byte bound exceeded.' }
        if ($name -in @('SHA256SUMS','.ocmi-home.json','system/build-info.json','system/packaging/home/text.json','system/locales/ru/packaging/home/text.json') -and $length -gt 1MB) { throw 'ReadyKit metadata bound exceeded.' }
        $total += $length
    }
    foreach ($parent in $parents.Keys) { if ($names.Contains($parent)) { throw 'ReadyKit file/directory prefix collision.' } }
}

function Assert-ReadyZipProfile {
    param([string]$Path)
    $stream=[IO.File]::OpenRead($Path); $reader=[IO.BinaryReader]::new($stream)
    try {
        if ($stream.Length -lt 22 -or $stream.Length -gt 272MB) { throw 'ReadyKit ZIP compressed bound.' }
        $stream.Position=$stream.Length-22
        if ($reader.ReadUInt32() -ne 0x06054b50 -or $reader.ReadUInt16() -ne 0 -or $reader.ReadUInt16() -ne 0) { throw 'Unsupported ReadyKit ZIP end record.' }
        $diskCount=$reader.ReadUInt16(); $count=$reader.ReadUInt16(); $cdSize=$reader.ReadUInt32(); $cd=$reader.ReadUInt32(); $comment=$reader.ReadUInt16()
        if ($count -lt 1 -or $count -gt 4096 -or $diskCount -ne $count -or $cdSize -gt 8MB -or [long]$cd+$cdSize -ne $stream.Length-22 -or $comment -ne 0) { throw 'Unsupported ReadyKit ZIP directory profile.' }
        $stream.Position=$cd; [long]$localEnd=0; [long]$total=0
        for ($i=0; $i -lt $count; $i++) {
            if ($reader.ReadUInt32() -ne 0x02014b50 -or $reader.ReadUInt16() -ne 20 -or $reader.ReadUInt16() -ne 20 -or $reader.ReadUInt16() -ne 0 -or $reader.ReadUInt16() -ne 8) { throw 'Unsupported ReadyKit ZIP flags/version/method.' }
            $reader.ReadUInt32() | Out-Null; $crc=$reader.ReadUInt32(); $compressed=$reader.ReadUInt32(); $size=$reader.ReadUInt32()
            $nameLength=$reader.ReadUInt16(); $extra=$reader.ReadUInt16(); $entryComment=$reader.ReadUInt16()
            $disk=$reader.ReadUInt16(); $reader.ReadUInt16() | Out-Null; $attributes=$reader.ReadUInt32(); $offset=$reader.ReadUInt32()
            if ($nameLength -lt 1 -or $nameLength -gt 1024 -or $extra -ne 0 -or $entryComment -ne 0 -or $disk -ne 0 -or $attributes -ne 0 -or $size -gt 32MB -or $compressed -gt 272MB -or $total -gt 256MB-$size -or $offset -ne $localEnd) { throw 'Unsupported ReadyKit ZIP member profile.' }
            $name=$reader.ReadBytes($nameLength); if ($name.Length -ne $nameLength) { throw 'Truncated ZIP name.' }
            $next=$stream.Position; $stream.Position=$offset
            if ($reader.ReadUInt32() -ne 0x04034b50 -or $reader.ReadUInt16() -ne 20 -or $reader.ReadUInt16() -ne 0 -or $reader.ReadUInt16() -ne 8) { throw 'Unsupported ReadyKit ZIP local header.' }
            $reader.ReadUInt32() | Out-Null
            if ($reader.ReadUInt32() -ne $crc -or $reader.ReadUInt32() -ne $compressed -or $reader.ReadUInt32() -ne $size -or $reader.ReadUInt16() -ne $nameLength -or $reader.ReadUInt16() -ne 0) { throw 'ZIP local/directory disagreement.' }
            $localName=$reader.ReadBytes($nameLength)
            if ([Convert]::ToHexString($name) -cne [Convert]::ToHexString($localName)) { throw 'ZIP local name disagreement.' }
            $localEnd=$stream.Position+$compressed; if ($localEnd -gt $cd) { throw 'ZIP member overlaps directory.' }
            $total+=$size; $stream.Position=$next
        }
        if ($stream.Position -ne [long]$cd+$cdSize -or $localEnd -ne $cd) { throw 'ZIP directory/stream boundary disagreement.' }
    } finally { $reader.Dispose(); $stream.Dispose() }
}

function Read-ReadyTarBytes {
    param([IO.Stream]$Stream,[int]$Count,[ref]$Total)
    if ($Count -lt 0 -or $Count -gt 16KB -or $Total.Value -gt 272MB-$Count) { throw 'ReadyKit TAR stream bound.' }
    $buffer=[byte[]]::new($Count); $read=0
    while ($read -lt $Count) { $n=$Stream.Read($buffer,$read,$Count-$read); if ($n -le 0) { throw 'Truncated ReadyKit TAR stream.' }; $read+=$n }
    $Total.Value+=$Count
    return ,$buffer
}

function Read-ReadyTarOctal {
    param([byte[]]$Header,[int]$Offset,[int]$Length)
    $value=[Text.Encoding]::ASCII.GetString($Header,$Offset,$Length).Trim([char[]]@([char]0,[char]32))
    if (!$value) { return [long]0 }
    if ($value -notmatch '^[0-7]+$') { throw 'Unsupported TAR numeric encoding.' }
    return [Convert]::ToInt64($value,8)
}

function Assert-ReadyTarFraming {
    param([IO.Stream]$Stream,[long]$ExpectedStreamBytes)
    [long]$total=0; [long]$payload=0; [long]$metadata=0; $headers=0; $files=0; $pendingPax=$false
    while ($true) {
        $header=Read-ReadyTarBytes $Stream 512 ([ref]$total)
        if (@($header | Where-Object {$_ -ne 0}).Count -eq 0) {
            $end=Read-ReadyTarBytes $Stream 512 ([ref]$total)
            if ($pendingPax -or @($end | Where-Object {$_ -ne 0}).Count -ne 0 -or $Stream.ReadByte() -ne -1 -or $total -ne $ExpectedStreamBytes) { throw 'Unsupported TAR/gzip trailer or extra stream.' }
            break
        }
        $headers++; if ($headers -gt 8192) { throw 'ReadyKit TAR header-count bound.' }
        [long]$checksum=0
        for ($i=0; $i -lt 512; $i++) { $checksum+= $(if($i -ge 148 -and $i -lt 156){32}else{[int]$header[$i]}) }
        if ((Read-ReadyTarOctal $header 148 8) -ne $checksum -or [Text.Encoding]::ASCII.GetString($header,257,6) -cne "ustar`0" -or [Text.Encoding]::ASCII.GetString($header,263,2) -cne '00') { throw 'Unsupported TAR header/checksum.' }
        if ((Read-ReadyTarOctal $header 108 8) -ne 0 -or (Read-ReadyTarOctal $header 116 8) -ne 0 -or @($header[265..328] | Where-Object {$_ -ne 0}).Count -ne 0 -or @($header[157..256] | Where-Object {$_ -ne 0}).Count -ne 0) { throw 'Unsupported TAR owner/link metadata.' }
        $mode=Read-ReadyTarOctal $header 100 8
        if ($mode -notin @(420,493)) { throw 'Unsupported TAR privilege/mode.' }
        $size=Read-ReadyTarOctal $header 124 12
        if ($header[156] -eq 120) {
            if ($pendingPax -or $size -lt 1 -or $size -gt 16KB -or $metadata -gt 2MB-$size) { throw 'Unsupported PAX header/metadata bound.' }
            $metadata+=$size; $pendingPax=$true
            $raw=Read-ReadyTarBytes $Stream ([int]$size) ([ref]$total)
            $text=[Text.UTF8Encoding]::new($false,$true).GetString($raw)
            if ($text -cmatch '[^\x00-\x7f]') { throw 'Unsupported PAX character profile.' }
            $cursor=0; $keys=@{}
            while ($cursor -lt $text.Length) {
                $space=$text.IndexOf(' ',$cursor)
                if ($space -le $cursor) { throw 'Malformed PAX record length.' }
                $digits=$text.Substring($cursor,$space-$cursor)
                if ($digits -notmatch '^[1-9][0-9]{0,4}$') { throw 'Unsupported PAX record length.' }
                $length=[int]$digits
                if ($length -gt $text.Length-$cursor -or $length -le $space-$cursor+2 -or $text[$cursor+$length-1] -ne "`n") { throw 'Truncated PAX record.' }
                $record=$text.Substring($space+1,$length-($space-$cursor)-2); $equal=$record.IndexOf('=')
                if ($equal -lt 1) { throw 'Malformed PAX field.' }
                $key=$record.Substring(0,$equal); $value=$record.Substring($equal+1)
                if ($key -notin @('path','mtime') -or $keys.ContainsKey($key)) { throw 'Unsupported/duplicate PAX key.' }; $keys[$key]=$value
                $cursor+=$length
            }
            if ($keys.Count -ne 2 -or !$keys.ContainsKey('path') -or !$keys.ContainsKey('mtime') -or $keys.path.Length -gt 1024 -or $keys.mtime -notmatch '^\d+(\.\d{1,9})?$') { throw 'Unsupported PAX field profile.' }
        } elseif ($header[156] -eq 48) {
            $files++
            if (!$pendingPax -or $files -gt 4096 -or $size -gt 32MB -or $payload -gt 256MB-$size) { throw 'Unsupported TAR regular-file bound/profile.' }
            $pendingPax=$false; $payload+=$size; $left=$size
            while ($left -gt 0) { $take=[int][Math]::Min($left,16KB); $null=Read-ReadyTarBytes $Stream $take ([ref]$total); $left-=$take }
        } else { throw 'Unsupported TAR type.' }
        $pad=[int]((512-$size%512)%512)
        if ($pad) { $padding=Read-ReadyTarBytes $Stream $pad ([ref]$total); if (@($padding | Where-Object {$_ -ne 0}).Count -ne 0) { throw 'Nonzero TAR member padding.' } }
    }
    if ($files -lt 1) { throw 'Empty ReadyKit TAR.' }
}

function Assert-ReadyTarProfile {
    param([string]$Path,[string]$ExecutableArchivePath)
    $file=[IO.File]::OpenRead($Path)
    try {
        if ($file.Length -gt 272MB -or $file.Length -lt 18) { throw 'ReadyKit gzip bound.' }
        $header=[byte[]]::new(10); if ($file.Read($header,0,10) -ne 10 -or [BitConverter]::ToUInt16($header,0) -ne 0x8b1f -or $header[2] -ne 8 -or $header[3] -ne 0) { throw 'Unsupported ReadyKit gzip header.' }
        $file.Position=$file.Length-4; $footer=[byte[]]::new(4)
        if ($file.Read($footer,0,4) -ne 4) { throw 'Truncated gzip footer.' }
        $streamBytes=[long][BitConverter]::ToUInt32($footer,0)
        if ($streamBytes -lt 1024 -or $streamBytes -gt 272MB) { throw 'ReadyKit decompressed stream bound.' }
        $file.Position=0; $framing=[IO.Compression.GZipStream]::new($file,[IO.Compression.CompressionMode]::Decompress,$true)
        try { Assert-ReadyTarFraming $framing $streamBytes } finally { $framing.Dispose() }
        $file.Position=0; $gzip=[IO.Compression.GZipStream]::new($file,[IO.Compression.CompressionMode]::Decompress,$true)
        try {
            $reader=[System.Formats.Tar.TarReader]::new($gzip,$true)
            $count=0; [long]$bytes=0; [long]$metadata=0
            try {
                while ($null -ne ($entry=$reader.GetNextEntry())) {
                    $count++; if ($count -gt 4096 -or $entry -isnot [System.Formats.Tar.PaxTarEntry] -or $entry.EntryType -ne [System.Formats.Tar.TarEntryType]::RegularFile -or $entry.Length -gt 32MB -or $bytes -gt 256MB-$entry.Length) { throw 'Unsupported ReadyKit TAR member/count/byte profile.' }
                    $bytes+=$entry.Length
                    if ($entry.Uid -ne 0 -or $entry.Gid -ne 0 -or $entry.UserName -ne '' -or $entry.GroupName -ne '') { throw 'Unsupported ReadyKit TAR ownership metadata.' }
                    $mode=if ($entry.Name -eq $ExecutableArchivePath) {493}else{420}
                    if ([int]$entry.Mode -ne $mode) { throw 'Unsupported ReadyKit TAR mode.' }
                    if ($entry.ExtendedAttributes.Count -ne 2 -or !$entry.ExtendedAttributes.ContainsKey('path') -or !$entry.ExtendedAttributes.ContainsKey('mtime') -or $entry.ExtendedAttributes['path'] -cne $entry.Name -or $entry.ExtendedAttributes['mtime'] -notmatch '^\d+(\.\d{1,9})?$') { throw 'Unsupported ReadyKit PAX metadata.' }
                    [long]$entryMetadata=0
                    foreach ($pair in $entry.ExtendedAttributes.GetEnumerator()) { $entryMetadata += [Text.Encoding]::UTF8.GetByteCount($pair.Key)+[Text.Encoding]::UTF8.GetByteCount($pair.Value)+32 }
                    if ($entryMetadata -gt 16KB -or $metadata -gt 2MB-$entryMetadata) { throw 'ReadyKit PAX metadata bound.' }; $metadata+=$entryMetadata
                }
            } finally { $reader.Dispose() }
        } finally { $gzip.Dispose() }
    } finally { $file.Dispose() }
}

function Assert-ReadyKitId {
    param([string]$Id)
    $legacy = '\Aocmi-ready-[A-Za-z0-9-]+\z'
    $release = '\Aocmi-ready-(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\z'
    if ($Id -notmatch $legacy -and $Id -notmatch $release) {
        throw 'ReadyKit ID must be ocmi-ready- followed by letters, digits and hyphens, or an exact numeric MAJOR.MINOR.PATCH version.'
    }
}

function New-ReadyDeliveryAsset {
    param([string]$Distribution,[string]$Preview,[string]$TargetName,[string]$Binary,[string]$Archive)
    Assert-ReadyKitId $Preview
    if ($TargetName -notin @('windows-amd64','darwin-arm64','darwin-amd64','linux-amd64')) { throw 'Invalid delivery asset identity.' }
    Assert-NoReparseAncestor $Distribution
    Assert-RegularFile $Binary | Out-Null; Assert-RegularFile $Archive | Out-Null
    $extension=if($TargetName -eq 'windows-amd64'){'.zip'}else{'.tar.gz'}
    if ((Resolve-FullPath $Archive) -cne (Resolve-FullPath (Join-Path $Distribution "$Preview-$TargetName$extension"))) { throw 'Delivery archive name/placement mismatch.' }
    $launcherName="$Preview-$TargetName-ocmi"+$(if($TargetName -eq 'windows-amd64'){'.exe'}else{''})
    $launcher=Join-Path $Distribution $launcherName; $before=Get-Sha256 $Binary
    $source=[IO.File]::OpenRead($Binary)
    try { $dest=[IO.File]::Open($launcher,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None); try { $source.CopyTo($dest) } finally { $dest.Dispose() } } finally { $source.Dispose() }
    if ((Get-Sha256 $launcher) -ne $before -or (Get-Sha256 $Binary) -ne $before) { throw 'Standalone launcher identity differs from built binary.' }
    if (![OperatingSystem]::IsWindows()) { [IO.File]::SetUnixFileMode($launcher,[IO.UnixFileMode]493) }
    return [ordered]@{target=$TargetName;archive=[ordered]@{name=[IO.Path]::GetFileName($Archive);sha256=Get-Sha256 $Archive};launcher=[ordered]@{name=$launcherName;sha256=$before}}
}

function Write-ReadyDeliveryReceipt {
    param([string]$Distribution,[string]$Commit,[object[]]$Assets)
    if ($Commit -cnotmatch '^[0-9a-f]{40}$' -or $Assets.Count -lt 1 -or $Assets.Count -gt 4) { throw 'Invalid delivery receipt source/assets.' }
    $seen=@{}
    foreach ($asset in $Assets) {
        if ($asset -isnot [Collections.IDictionary] -or $asset.Keys.Count -ne 3 -or @($asset.Keys | Where-Object {$_ -notin @('target','archive','launcher')}).Count) { throw 'Unsupported delivery asset schema.' }
        if ($asset.target -notin @('windows-amd64','darwin-arm64','darwin-amd64','linux-amd64') -or $seen.ContainsKey($asset.target)) { throw 'Duplicate/unsupported delivery target.' }; $seen[$asset.target]=$true
        foreach ($part in @($asset.archive,$asset.launcher)) {
            if ($part -isnot [Collections.IDictionary] -or $part.Keys.Count -ne 2 -or @($part.Keys | Where-Object {$_ -notin @('name','sha256')}).Count) { throw 'Unsupported delivery file schema.' }
            if (!$part.name -or $part.name -cne [IO.Path]::GetFileName($part.name) -or $part.name -match '[\\/:]' -or $part.sha256 -cnotmatch '^[0-9a-f]{64}$' -or (Get-Sha256 (Join-Path $Distribution $part.name)) -cne $part.sha256) { throw 'Delivery receipt file/hash mismatch.' }
        }
    }
    $raw=[ordered]@{schemaVersion=1;sourceCommit=$Commit;assets=@($Assets)} | ConvertTo-Json -Depth 6
    if ([Text.Encoding]::UTF8.GetByteCount($raw) -gt 16KB) { throw 'Delivery receipt bound exceeded.' }
    $stream=[IO.File]::Open((Join-Path $Distribution 'delivery.json'),[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
    try { $bytes=[Text.UTF8Encoding]::new($false).GetBytes($raw); $stream.Write($bytes,0,$bytes.Length) } finally { $stream.Dispose() }
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
    Assert-ReadyKitId $PreviewId
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
        $deliveryAssets = @()
        foreach ($item in $built) {
            $targetName = "$($item.Target.GOOS)-$($item.Target.GOARCH)"
            $binaryPath = if ($item.Target.GOOS -eq 'windows') { 'system/bin/ocmi.exe' } else { 'system/bin/ocmi' }
            $compatibility = [ordered]@{ schemaVersion = 1; layoutVersion = 1; installationReceiptVersion = 1; coordinatorMemoryVersion = 1; coordinatorBindingVersion = 1; stateSchemaVersion = 4 }
            $info = [ordered]@{ previewId = $PreviewId; distributionKind = 'ready-kit'; systemVersion = $systemVersion; sourceCommit = $SourceCommit; sourceDirty = $false; layoutVersion = 1; layoutDeclaration = '.ocmi-home.json'; binaryPath = $binaryPath; target = $targetName; updateCompatibility = $compatibility; toolStorageProfileFormat = 2; operationWorkspaceInventoryPolicy = 1; sourceSnapshot = $runtimeSnapshot; systemSnapshot = $systemSnapshot; goVersion = $goVersion; cgoEnabled = '0'; moduleGraph = $moduleGraph; linkedModulesByTarget = $linked; linkedModulesUnion = $union }
            $infoPath = Join-Path $staging ("$targetName-build-info.json"); Set-Content -LiteralPath $infoPath -Value ($info | ConvertTo-Json -Depth 10) -Encoding utf8NoBOM
            $systemSourceEntries = @($sourceEntries | ForEach-Object { [pscustomobject]@{ ArchivePath = 'system/' + $_.ArchivePath; SourcePath = $_.SourcePath } })
            $systemNotices = @($notices | ForEach-Object { [pscustomobject]@{ ArchivePath = 'system/' + $_.ArchivePath; SourcePath = $_.SourcePath } })
            $entries = @($homeEntries) + $systemSourceEntries + $systemNotices + @([pscustomobject]@{ ArchivePath = $binaryPath; SourcePath = $item.Binary }, [pscustomobject]@{ ArchivePath = 'system/build-info.json'; SourcePath = $infoPath })
            $names = New-Object 'System.Collections.Generic.HashSet[string]' ([StringComparer]::OrdinalIgnoreCase); foreach ($entry in $entries) { if (-not $names.Add((Assert-SafeRelativePath $entry.ArchivePath))) { throw "Archive entry collision: $($entry.ArchivePath)" } }
            $sumsPath = Join-Path $staging ("$targetName-SHA256SUMS"); Set-Content -LiteralPath $sumsPath -Value (($entries | ForEach-Object { "$(Get-Sha256 $_.SourcePath)  $($_.ArchivePath.Replace('\\', '/'))" }) -join "`n") -Encoding utf8NoBOM
            $entries += [pscustomobject]@{ ArchivePath = 'SHA256SUMS'; SourcePath = $sumsPath }
            Assert-ReadyArchiveInputs -Entries $entries
            $archive = Join-Path $distribution ("$PreviewId-$targetName" + $(if ($item.Target.GOOS -eq 'windows') { '.zip' } else { '.tar.gz' }))
            if ($item.Target.GOOS -eq 'windows') { New-ArchiveFromAllowlist -ArchivePath $archive -Entries $entries } else { New-TarGzipFromAllowlist -ArchivePath $archive -Entries $entries -ExecutableArchivePath $binaryPath }
            if ($item.Target.GOOS -eq 'windows') { Assert-ReadyZipProfile $archive } else { Assert-ReadyTarProfile -Path $archive -ExecutableArchivePath $binaryPath }
            $deliveryAssets += New-ReadyDeliveryAsset -Distribution $distribution -Preview $PreviewId -TargetName $targetName -Binary $item.Binary -Archive $archive
        }
        $archives = @(Get-ChildItem -LiteralPath $distribution -File | Where-Object { $_.Name -match '\.(zip|tar\.gz)$' } | Sort-Object Name)
        Set-Content -LiteralPath (Join-Path $distribution 'SHA256SUMS-archives.txt') -Value (($archives | ForEach-Object { "$(Get-Sha256 $_.FullName)  $($_.Name)" }) -join "`n") -Encoding utf8NoBOM
        Write-ReadyDeliveryReceipt -Distribution $distribution -Commit $SourceCommit -Assets $deliveryAssets
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
            foreach ($entry in $tarEntries) { if ($entry.Uid -ne 0 -or $entry.Gid -ne 0 -or $entry.UserName -ne '' -or $entry.GroupName -ne '') { throw 'Tar ownership metadata not explicit-safe.' } }
        } finally { $reader.Dispose(); $gzipStream.Dispose(); $tarStream.Dispose() }
        Assert-ReadyArchiveInputs -Entries $allowlist
        $dotfile = Join-Path $testRoot '.ocmi-home.json'
        [IO.File]::WriteAllText($dotfile,'{"schemaVersion":1}',[Text.UTF8Encoding]::new($false))
        if ([OperatingSystem]::IsWindows()) { [IO.File]::SetAttributes($dotfile,[IO.File]::GetAttributes($dotfile) -bor [IO.FileAttributes]::Hidden) }
        Assert-ReadyArchiveInputs -Entries @([pscustomobject]@{ArchivePath='.ocmi-home.json';SourcePath=$dotfile})
        Assert-ReadyTarProfile -Path $tarArchive -ExecutableArchivePath 'ocmi'
        if ([OperatingSystem]::IsWindows()) { Assert-ReadyZipProfile $archive }
        foreach ($badName in @('../escape','CON.txt','name.','name ','one//two','one/./two','one:ads','local/private','system/OCMI_Sync/private','system/docs/memory/private',('x/'*32+'leaf'),([string][char]233+'.txt'))) {
            $refused=$false
            try { Assert-ReadyArchiveInputs -Entries @([pscustomobject]@{ArchivePath=$badName;SourcePath=$safe}) } catch { $refused=$true }
            if (!$refused) { throw "Unsupported archive name admitted: $badName" }
        }
        foreach ($badEntries in @(
            @($allowlist[0],$allowlist[0]),
            @([pscustomobject]@{ArchivePath='a';SourcePath=$safe},[pscustomobject]@{ArchivePath='a/b';SourcePath=$safe}),
            @([pscustomobject]@{ArchivePath='A/b';SourcePath=$safe},[pscustomobject]@{ArchivePath='a/c';SourcePath=$safe})
        )) {
            $refused=$false; try { Assert-ReadyArchiveInputs -Entries $badEntries } catch { $refused=$true }
            if (!$refused) { throw 'Archive collision/prefix/parent alias admitted.' }
        }
        $large=Join-Path $testRoot 'large'; $stream=[IO.File]::Open($large,[IO.FileMode]::CreateNew); try { $stream.SetLength(32MB+1) } finally { $stream.Dispose() }
        $refused=$false; try { Assert-ReadyArchiveInputs -Entries @([pscustomobject]@{ArchivePath='large';SourcePath=$large}) } catch { $refused=$true }
        if (!$refused) { throw 'Per-file producer limit not enforced.' }
        $stream=[IO.File]::Open($large,[IO.FileMode]::Open); try { $stream.SetLength(32MB) } finally { $stream.Dispose() }
        $aggregate=@(1..9 | ForEach-Object {[pscustomobject]@{ArchivePath="part-$_";SourcePath=$large}})
        $refused=$false; try { Assert-ReadyArchiveInputs -Entries $aggregate } catch { $refused=$true }
        if (!$refused) { throw 'Aggregate producer limit not enforced.' }
        $refused=$false; try { Assert-ReadyArchiveInputs -Entries @((1..4097 | ForEach-Object {$allowlist[0]})) } catch { $refused=$true }
        if (!$refused) { throw 'Producer entry-count limit not enforced.' }
        $stream=[IO.File]::Open($large,[IO.FileMode]::Open); try { $stream.SetLength(1MB+1) } finally { $stream.Dispose() }
        $refused=$false; try { Assert-ReadyArchiveInputs -Entries @([pscustomobject]@{ArchivePath='system/build-info.json';SourcePath=$large}) } catch { $refused=$true }
        if (!$refused) { throw 'Small metadata limit not enforced.' }
        if ([OperatingSystem]::IsWindows()) {
            $badZip=Join-Path $testRoot 'unsupported.zip'; $raw=[IO.File]::ReadAllBytes($archive); $raw[4]=45; [IO.File]::WriteAllBytes($badZip,$raw)
            $refused=$false; try { Assert-ReadyZipProfile $badZip } catch { $refused=$true }
            if (!$refused) { throw 'Unsupported ZIP local-version profile admitted.' }
        }
        $badTar=Join-Path $testRoot 'unsupported.tar.gz'
        $file=[IO.File]::Open($badTar,[IO.FileMode]::CreateNew); $gzip=[IO.Compression.GZipStream]::new($file,[IO.Compression.CompressionLevel]::Optimal,$false); $writer=[System.Formats.Tar.TarWriter]::new($gzip,$false)
        try {
            $entry=[System.Formats.Tar.PaxTarEntry]::new([System.Formats.Tar.TarEntryType]::RegularFile,'safe.txt'); $entry.Uid=1; $entry.Gid=0; $entry.UserName=''; $entry.GroupName=''; $entry.Mode=[IO.UnixFileMode]420
            $input=[IO.File]::OpenRead($safe); try {$entry.DataStream=$input;$writer.WriteEntry($entry)} finally {$input.Dispose()}
        } finally {$writer.Dispose();$gzip.Dispose();$file.Dispose()}
        $refused=$false; try { Assert-ReadyTarProfile -Path $badTar -ExecutableArchivePath 'ocmi' } catch {$refused=$true}
        if (!$refused) {throw 'Unsupported TAR owner profile admitted.'}
        $delivery=Join-Path $testRoot 'delivery'; New-Item -ItemType Directory -Path $delivery | Out-Null
        $preview='ocmi-ready-selftest'; $deliveryArchive=Join-Path $delivery "$preview-windows-amd64.zip"
        [IO.File]::Copy($archive,$deliveryArchive,$false)
        $asset=New-ReadyDeliveryAsset -Distribution $delivery -Preview $preview -TargetName 'windows-amd64' -Binary $unixBinary -Archive $deliveryArchive
        Write-ReadyDeliveryReceipt -Distribution $delivery -Commit ('a'*40) -Assets @($asset)
        $receipt=Get-Content -LiteralPath (Join-Path $delivery 'delivery.json') -Raw | ConvertFrom-Json
        if ($receipt.schemaVersion -ne 1 -or $receipt.sourceCommit -cne ('a'*40) -or $receipt.assets.Count -ne 1 -or $receipt.assets[0].launcher.name -cne "$preview-windows-amd64-ocmi.exe" -or $receipt.assets[0].launcher.sha256 -cne (Get-Sha256 $unixBinary) -or $receipt.assets[0].archive.sha256 -cne (Get-Sha256 $deliveryArchive)) { throw 'Standalone/receipt identity mismatch.' }
        $refused=$false; try { New-ReadyDeliveryAsset -Distribution $delivery -Preview $preview -TargetName 'windows-amd64' -Binary $unixBinary -Archive $deliveryArchive | Out-Null } catch { $refused=$true }
        if (!$refused) { throw 'Existing standalone launcher was overwritten.' }
        $refused=$false; try { Write-ReadyDeliveryReceipt -Distribution $delivery -Commit ('a'*40) -Assets @($asset) } catch { $refused=$true }
        if (!$refused) { throw 'Existing delivery receipt was overwritten.' }
        $asset.launcher.sha256='0'*64
        $refused=$false; try { Write-ReadyDeliveryReceipt -Distribution $delivery -Commit ('a'*40) -Assets @($asset) } catch { $refused=$true }
        if (!$refused) { throw 'Wrong standalone hash admitted into receipt.' }
        Write-Output 'Runtime preview staging self-test passed: allowlist, private exclusion, traversal, and collisions.'
        Write-Output 'Ready delivery self-test passed: exact standalone bytes, schema1 receipt, bounds/aliases, profile and collision refusal.'
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
