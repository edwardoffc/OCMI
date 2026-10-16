# Disposable session index. No discovery, subprocess, network, persistent cache or assignment authority.
$script:OCMIContextEngine = 'ocmi-context-1/ps-' + $PSVersionTable.PSVersion.ToString()
$script:OCMIContextCache = $null
function Clear-OCMIContextCache { $script:OCMIContextCache = $null }
function Read-OCMIContextFile([string]$Root,[string]$Relative) {
  if (!$Relative -or $Relative -match ':|^[/\\]|(^|[/\\])\.{1,2}([/\\]|$)' -or $Relative.Length -gt 240) {throw 'Invalid relative input'}
  $relativePath=$Relative.Replace('\','/')
  if ($relativePath -match '(?i)(^|/)(\.[^/]+|node_modules|vendor|dist|build|target|bin|obj|backups|outputs)(/|$)|\.(key|pem|pfx|sqlite\w*|db|zip|log)$') {throw 'Private/generated input rejected'}
  if ([IO.Path]::GetExtension($relativePath).ToLowerInvariant() -notin @('.ps1','.psm1','.go','.js','.mjs','.cjs','.ts','.tsx','.jsx','.py','.rs','.md','.json','.yaml','.yml','.toml')) {throw 'Unsupported text input; use scoped manual retrieval'}
  $comparison=if ([Environment]::OSVersion.Platform -eq [PlatformID]::Win32NT) {[StringComparison]::OrdinalIgnoreCase} else {[StringComparison]::Ordinal}
  $base=[IO.Path]::GetFullPath($Root)
  # Check root ancestors from the filesystem root before touching descendants.
  $probe=[IO.Path]::GetPathRoot($base)
  $item=Get-Item -LiteralPath $probe -Force -ErrorAction Stop
  if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) {throw 'Reparse filesystem root rejected'}
  foreach ($segment in $base.Substring($probe.Length).Split([char[]]@('/','\'),[StringSplitOptions]::RemoveEmptyEntries)) {
    $probe=Join-Path $probe $segment
    $item=Get-Item -LiteralPath $probe -Force -ErrorAction Stop
    if ($item -isnot [IO.DirectoryInfo] -or $item.Attributes -band [IO.FileAttributes]::ReparsePoint) {throw 'Unsafe input root ancestor'}
  }
  $full=$base
  foreach ($segment in $relativePath.Split('/')) {
    if (!$segment -or $segment.TrimEnd(' ','.') -cne $segment) {throw 'Ambiguous path segment'}
    $full=Join-Path $full $segment
    $item=Get-Item -LiteralPath $full -Force -ErrorAction Stop
    if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) {throw 'Reparse/symlink input rejected'}
  }
  $prefix=$base.TrimEnd([char[]]@('/','\'))+[IO.Path]::DirectorySeparatorChar
  if (!$item.FullName.StartsWith($prefix,$comparison) -or $item -isnot [IO.FileInfo]) {throw 'Input outside file scope'}
  $stream=[IO.File]::OpenRead($full)
  try {
    $bytes=New-Object byte[] 131073; $length=0
    while ($length -lt $bytes.Length) {$count=$stream.Read($bytes,$length,$bytes.Length-$length);if (!$count) {break};$length+=$count}
    if ($length -gt 131072) {throw 'File exceeds 128 KiB; narrow scope manually'}
    $text=[Text.UTF8Encoding]::new($false,$true).GetString($bytes,0,$length).TrimStart([char]0xfeff)
    if ($text.Contains([string][char]0)) {throw 'Binary text input rejected'}
    $hasher=[Security.Cryptography.SHA256]::Create()
    try {$sha=([BitConverter]::ToString($hasher.ComputeHash($bytes,0,$length))).Replace('-','').ToLowerInvariant()} finally {$hasher.Dispose()}
  } finally {$stream.Dispose()}
  [pscustomobject]@{path=$relativePath;sha256=$sha;bytes=$length;text=$text}
}
function Get-OCMIContextOutline([string]$Path,[string]$Text) {
  $symbols=@();$provider='lexical';$warning=$null
  if ([IO.Path]::GetExtension($Path).ToLowerInvariant() -in @('.ps1','.psm1')) {
    $tokens=$null;$parseErrors=$null
    $ast=[Management.Automation.Language.Parser]::ParseInput($Text,[ref]$tokens,[ref]$parseErrors)
    if (!$parseErrors.Count) {
      $provider='powershell-ast'
      $symbols=@($ast.FindAll({param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst]},$true) | ForEach-Object {
        [pscustomobject]@{name=$_.Name;line=$_.Extent.StartLineNumber;endLine=$_.Extent.EndLineNumber}
      })
    } else {$warning='parse-error-lexical-fallback'}
  }
  if ($provider -eq 'lexical') {
    $pattern='^\s*(?:(?:export|default|async|pub|public|private|static)\s+)*(?:function|class|interface|type|def|fn|struct|enum|const|let|var)\s+([\w$-]+)|^\s*func\s+(?:\([^)]*\)\s*)?([\w]+)|^\s*(?:function|filter)\s+([\w-]+)'
    $lines=[regex]::Split($Text,'\r?\n')
    for ($i=0;$i -lt $lines.Count;$i++) {
      $match=[regex]::Match($lines[$i],$pattern)
      if ($match.Success) {
        $name=(@($match.Groups | Select-Object -Skip 1 | Where-Object Success)[0]).Value
        $symbols += [pscustomobject]@{name=$name;line=$i+1;endLine=$i+1}
      }
    }
  }
  if ($symbols.Count -gt 512) {$symbols=@($symbols | Select-Object -First 512);$warning='outline-limit'}
  [pscustomobject]@{provider=$provider;symbols=@($symbols);warning=$warning}
}
function Invoke-OCMIContext {
  param(
    [Parameter(Mandatory=$true)][string]$Root,
    [Parameter(Mandatory=$true)][string[]]$Files,
    [Parameter(Mandatory=$true)][string[]]$Terms,
    [string[]]$RequiredFiles=@(),
    [ValidateRange(128,131072)][int]$BudgetChars=16000,
    [ValidateLength(1,128)][string]$SourceRevision='unknown'
  )
  if (!$Root -or ![IO.Path]::IsPathRooted($Root) -or $Root -match '^[/\\]{2}' -or !$Files.Count -or $Files.Count+$RequiredFiles.Count -gt 128 -or !$Terms.Count -or $Terms.Count -gt 8) {throw 'Explicit bounded local root, files and terms required'}
  foreach ($term in $Terms) {if ($term -cnotmatch '^[A-Za-z_][A-Za-z0-9_$-]{0,79}$') {throw 'Use literal identifier terms'} }
  $rootPath=[IO.Path]::GetFullPath($Root)
  $comparer=if ([Environment]::OSVersion.Platform -eq [PlatformID]::Win32NT) {[StringComparer]::OrdinalIgnoreCase} else {[StringComparer]::Ordinal}
  $paths=[Collections.Generic.HashSet[string]]::new($comparer)
  foreach ($relative in @($Files)+@($RequiredFiles)) {if ($relative) {[void]$paths.Add($relative.Replace('\','/'))} else {throw 'Empty scope input'}}
  $required=[Collections.Generic.HashSet[string]]::new($comparer)
  foreach ($relative in $RequiredFiles) {[void]$required.Add($relative.Replace('\','/'))}
  $previous=$script:OCMIContextCache
  $reuse=$null -ne $previous -and $previous.root -ceq $rootPath -and $previous.engine -ceq $script:OCMIContextEngine -and $previous.revision -ceq $SourceRevision
  $entries=[Collections.Generic.Dictionary[string,object]]::new($comparer)
  $snapshots=@();$readBytes=0;$sourceChars=0;$reused=0;$parsed=0;$notices=@()
  foreach ($relative in @($paths | Sort-Object)) {
    $source=Read-OCMIContextFile $rootPath $relative
    $readBytes+=$source.bytes;$sourceChars+=$source.text.Length
    if ($readBytes -gt 2097152) {throw 'Scope exceeds 2 MiB; select a narrower area'}
    if ($reuse -and $previous.entries.ContainsKey($relative) -and $previous.entries[$relative].sha256 -ceq $source.sha256) {
      $outline=$previous.entries[$relative].outline;$reused++
    } else {$outline=Get-OCMIContextOutline $relative $source.text;$parsed++}
    $entries[$relative]=[pscustomobject]@{sha256=$source.sha256;outline=$outline}
    if ($outline.warning) {$notices += [pscustomobject]@{path=$relative;reason=$outline.warning}}
    $snapshots += [pscustomobject]@{source=$source;outline=$outline}
  }
  # Only successful full reads replace the disposable snapshot. No persisted/imported index is trusted.
  $dropped=0
  if ($null -ne $previous) {foreach ($key in $previous.entries.Keys) {if (!$entries.ContainsKey($key) -or !$reuse) {$dropped++}}}
  $script:OCMIContextCache=[pscustomobject]@{root=$rootPath;engine=$script:OCMIContextEngine;revision=$SourceRevision;entries=$entries}
  $pattern='(?<![\w$-])(?:'+(($Terms | ForEach-Object {[regex]::Escape($_)}) -join '|')+')(?![\w$-])'
  $candidates=@();$outlineResult=@();$inventory=@();$windowCount=0
  foreach ($snapshot in $snapshots) {
    $source=$snapshot.source;$outline=$snapshot.outline
    $lines=[regex]::Split($source.text,'\r?\n')
    if ($lines.Count -gt 16384) {throw 'Too many source lines; use narrower manual retrieval'}
    $inventory += [pscustomobject]@{path=$source.path;sha256=$source.sha256;bytes=$source.bytes;provider=$outline.provider}
    foreach ($symbol in $outline.symbols) {
      if ($symbol.name -iin $Terms) {$outlineResult += [pscustomobject]@{path=$source.path;name=$symbol.name;line=$symbol.line;endLine=$symbol.endLine;provider=$outline.provider}}
    }
    if ($required.Contains($source.path)) {
      $candidates += [pscustomobject]@{path=$source.path;sha256=$source.sha256;startLine=1;endLine=$lines.Count;kind='required';rank=0;text=$source.text}
      continue
    }
    $windows=@()
    for ($i=0;$i -lt $lines.Count;$i++) {
      if ([regex]::IsMatch($lines[$i],$pattern,[Text.RegularExpressions.RegexOptions]::IgnoreCase)) {
        if ($windowCount -ge 512) {$notices += [pscustomobject]@{path=$source.path;reason='reference-window-limit'};break}
        $definition=@($outline.symbols | Where-Object {$_.line -eq $i+1 -and $_.name -iin $Terms}).Count -gt 0
        $start=[Math]::Max(1,$i-2);$end=[Math]::Min($lines.Count,$i+4)
        if ($windows.Count -and $start -le $windows[-1].end+1) {$windows[-1].end=$end;if ($definition) {$windows[-1].definition=$true}}
        else {$windows += [pscustomobject]@{start=$start;end=$end;definition=$definition};$windowCount++}
      }
    }
    foreach ($window in $windows) {
      $kind=if ($window.definition) {'definition-candidate'} else {'reference-candidate'}
      $rank=if ($window.definition) {1} else {2}
      $candidates += [pscustomobject]@{path=$source.path;sha256=$source.sha256;startLine=$window.start;endLine=$window.end;kind=$kind;rank=$rank;text=($lines[($window.start-1)..($window.end-1)] -join "`n")}
    }
  }
  $selected=@();$omitted=@();$used=0
  foreach ($candidate in @($candidates | Sort-Object rank,path,startLine)) {
    if ($used+$candidate.text.Length -le $BudgetChars) {$selected+=$candidate;$used+=$candidate.text.Length}
    else {$omitted += [pscustomobject]@{path=$candidate.path;startLine=$candidate.startLine;endLine=$candidate.endLine;kind=$candidate.kind;reason='context-budget'}}
  }
  if ($outlineResult.Count -gt 128) {$notices += [pscustomobject]@{path='';reason='outline-limit'};$outlineResult=@($outlineResult | Select-Object -First 128)}
  $status=if ($omitted.Count -or $notices.Count -or !$candidates.Count) {'incomplete'} else {'selected'}
  [pscustomobject]@{
    schemaVersion=1;status=$status;coverage='explicit-files-only';completeness='not-established';readiness='not-evaluated'
    provenance=[pscustomobject]@{engine=$script:OCMIContextEngine;sourceRevision=$SourceRevision;files=@($inventory)}
    cache=[pscustomobject]@{storage='session-memory-only';reused=$reused;parsed=$parsed;dropped=$dropped}
    requiredFiles=@($required);outline=@($outlineResult);selected=@($selected);omitted=@($omitted);notices=@($notices)
    metrics=[pscustomobject]@{filesRead=$snapshots.Count;sourceBytesRead=$readBytes;sourceChars=$sourceChars;snippetChars=$used;budgetChars=$BudgetChars;tokens='unknown'}
    nextAction='Verify required inputs and caller/contract/test coverage; expand authorized retrieval for gaps.'
  }
}
