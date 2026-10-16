# Read-only diagnostics. Input records are data, never commands or authority.
. (Join-Path $PSScriptRoot 'path-utils.ps1')
function Invoke-OCMIDoctor {
  param([string]$Root,[ValidateSet('system','work','personal')][string]$Profile,[string]$InputFile='',[string]$ExpectedVersion='',[object]$Selection=$null)
  $findings = [Collections.Generic.List[object]]::new()
  $observations = [Collections.Generic.List[object]]::new()
  $cache = [Collections.Generic.Dictionary[string,string]]::new([StringComparer]::Ordinal)
  $budget = @{ bytes=0; files=0 }
  $version=$null
  function Note([string]$Level,[string]$Code,[string]$Path,[string]$Action) {
    if ($findings.Count -ge 128) { return }
    if ($findings.Count -eq 127) { $Level='unknown'; $Code='FINDING_LIMIT'; $Path=''; $Action='Report truncated; narrow the selected scope and review remaining inputs.' }
    $findings.Add([pscustomobject]@{level=$Level;code=$Code;path=$Path;action=$Action})
  }
  function Parse-Json([string]$Text) {
    # ConvertFrom-Json alone may silently accept duplicate keys. Check tokenized keys first.
    $tokens=[regex]::Matches($Text,'"(?:\\.|[^"\\])*"|[{}\[\]:,]')
    $stack=[Collections.Generic.List[object]]::new()
    for ($i=0; $i -lt $tokens.Count; $i++) {
      $token=$tokens[$i].Value
      if ($token -cin @('{','[')) {
        if ($stack.Count -ge 12) { throw 'JSON depth limit' }
        $stack.Add([pscustomobject]@{kind=$token;keys=[Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)})
      } elseif ($token -cin @('}',']')) {
        if (!$stack.Count) { throw 'Malformed JSON' }; $stack.RemoveAt($stack.Count-1)
      } elseif ($token.StartsWith('"') -and $i+1 -lt $tokens.Count -and $tokens[$i+1].Value -ceq ':') {
        if (!$stack.Count -or $stack[$stack.Count-1].kind -cne '{') { throw 'Malformed JSON key' }
        $key=(('{"key":'+$token+'}') | ConvertFrom-Json -ErrorAction Stop).key
        if (!$stack[$stack.Count-1].keys.Add($key)) { throw 'Duplicate JSON key' }
      }
    }
    return ($Text | ConvertFrom-Json -ErrorAction Stop)
  }
  function Safe-Relative([string]$Path) {
    Assert-OCMIRelativePath $Path
    if ($Path -match '[\x00-\x1f%?#]|(^|[/\\])(\.git|\.local|\.codex|node_modules)([/\\]|$)') { throw 'Excluded input' }
    return $Path.Replace('\','/')
  }
  function Safe-File([string]$Relative) {
    # Inspect each component before stepping into it; do not stat a symlink's outside leaf first.
    $cursor=$base
    foreach ($part in ($Relative -split '/')) {
      $cursor=Join-Path $cursor $part
      $item=Get-Item -LiteralPath $cursor -Force -ErrorAction Stop
      if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Linked input component' }
    }
    return $cursor
  }
  function Read-Input([string]$Path) {
    $relative=''
    try {
      $relative = Safe-Relative $Path
      if ($cache.ContainsKey($relative)) { return $cache[$relative] }
      $file = Safe-File $relative
      if (!((Get-Item -LiteralPath $file) -is [IO.FileInfo])) { throw 'Not a file' }
      $size = (Get-Item -LiteralPath $file).Length
      if ($size -gt 262144 -or $budget.files -ge 64 -or $budget.bytes+$size -gt 2097152) {
        Note unknown LIMIT $relative 'Reduce the explicitly selected inputs; coverage is incomplete.'
        return $null
      }
      $stream=[IO.File]::Open($file,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read)
      try {
        if ($stream.Length -ne $size) { throw 'Changed before read' }
        $bytes=New-Object byte[] ([int]$size)
        $offset=0
        while ($offset -lt $size) { $read=$stream.Read($bytes,$offset,[int]$size-$offset); if (!$read) {throw 'Incomplete read'}; $offset+=$read }
        if ($stream.ReadByte() -ne -1) { throw 'Changed during read' }
      } finally { $stream.Dispose() }
      $sha = [Security.Cryptography.SHA256]::Create()
      try { $hash = ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-','').ToLowerInvariant() } finally { $sha.Dispose() }
      $text = [Text.UTF8Encoding]::new($false,$true).GetString($bytes).TrimStart([char]0xfeff)
      $budget.files++; $budget.bytes += $size
      $observations.Add([pscustomobject]@{path=$relative;sha256=$hash;bytes=$size})
      $cache[$relative] = $text
      return $text
    } catch {
      # Do not disclose OS errors, outside paths or file contents in the report.
      Note error INPUT_UNAVAILABLE $relative 'Use an existing UTF-8 file inside the selected root; private, linked and unsafe paths are rejected.'
      return $null
    }
  }
  function Block([string]$Path,[string]$Kind) {
    $text = Read-Input $Path
    if ($null -eq $text) { return $null }
    $matches = [regex]::Matches($text,'(?ms)^~~~ocmi-diagnostic\r?\n(.*?)\r?\n~~~\s*(?:\r?\n|$)')
    if ($matches.Count -ne 1) {
      Note unknown BLOCK_REQUIRED $Path 'Provide one diagnostic block in this canonical record, or review legacy/ambiguous text manually.'
      return $null
    }
    try {
      $value = Parse-Json $matches[0].Groups[1].Value
      if ($value -is [Array] -or $value.schemaVersion -ne 1 -or $value.kind -cne $Kind) { throw 'Unsupported block' }
      if ($value.state -cin @('historical','superseded','rejected')) {
        Note unknown HISTORICAL $Path 'Select a current canonical record; historical evidence cannot establish a current assignment.'
        return $null
      }
      return $value
    } catch { Note unknown BLOCK_INVALID $Path 'Correct the unsupported or malformed diagnostic block; do not infer its meaning.'; return $null }
  }
  function Check-Links([string]$Path,[string]$Text) {
    $fence = ''; $count=0
    foreach ($line in ($Text -split '\r?\n')) {
      if ($line -match '^\s*(`{3,}|~{3,})') { $marker=$Matches[1].Substring(0,1); if (!$fence) {$fence=$marker} elseif ($fence -eq $marker) {$fence=''}; continue }
      if ($fence) { continue }
      foreach ($match in [regex]::Matches($line,'\[[^\]\r\n]*\]\(([^)\r\n]+)\)')) {
        $count++; if ($count -gt 256) { Note unknown LINK_LIMIT $Path 'Review the remaining links manually; coverage is incomplete.'; return }
        $target=$match.Groups[1].Value.Trim().Trim('<','>')
        if ($target -match '^[a-zA-Z][a-zA-Z0-9+.-]*:|^#|^//') { continue }
        try {
          $target=([Uri]::UnescapeDataString($target) -split '[#?]',2)[0]
          if (!$target) { continue }
          $joined=[IO.Path]::GetFullPath((Join-Path (Split-Path (Join-Path $base $Path) -Parent) $target))
          if (!(Test-OCMIContainedPath $base $joined)) { throw 'Outside root' }
          $relative=Safe-Relative (Get-OCMIRelativeFile $base $joined)
          # Check path/link safety only; do not load the target or traverse its links.
          Safe-File $relative | Out-Null
        } catch { Note error LINK_UNRESOLVED $Path 'Repair or explicitly review a missing, linked, private or out-of-scope local link; its target was not read.' }
      }
    }
  }
  try {
    if (!$Root -or $Profile -notin @('system','work','personal')) { throw 'Explicit root/profile required' }
    $base=[IO.Path]::GetFullPath($Root)
    $item=Get-Item -LiteralPath $base -Force -ErrorAction Stop
    if (!($item -is [IO.DirectoryInfo]) -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Invalid root' }
    $required = switch ($Profile) {
      system { @('AGENTS.md','VERSION.md','BOOTSTRAP.md','COORDINATOR.md') }
      work { @('AGENTS.md','VERSION.md','docs/memory/INDEX.md','docs/memory/RULES.md','docs/memory/ORCHESTRATION.md','docs/memory/PLAN.md') }
      personal { @('AGENTS.md','memory/INDEX.md','memory/BRIEF.md') }
    }
    foreach ($path in $required) {
      $text=Read-Input $path
      if ($null -ne $text) { Check-Links $path $text }
    }
    $versionPath=if ($Profile -eq 'personal') {'AGENTS.md'} else {'VERSION.md'}
    if ($cache.ContainsKey($versionPath)) {
      $versions=@([regex]::Matches($cache[$versionPath],'\bOCMI\s+(\d+\.\d+\.\d+)\b') | ForEach-Object {$_.Groups[1].Value} | Sort-Object -Unique)
      if ($versions.Count -ne 1) { Note unknown VERSION_UNKNOWN $versionPath 'Select one current OCMI version; conflicting or absent labels need manual review.' }
      elseif ($ExpectedVersion -and $versions[0] -cne $ExpectedVersion) { Note error VERSION_MISMATCH $versionPath 'Reconcile the selected expected version and the current installation.' }
      else { Note info VERSION_CHECKED $versionPath 'Version label checked; this does not authenticate origin or prove installation readiness.' }
      if ($versions.Count -eq 1) {
        $version=$versions[0]
        $versioned=if ($Profile -eq 'work') {@('AGENTS.md','docs/memory/RULES.md','docs/memory/ORCHESTRATION.md')} else {@('AGENTS.md')}
        foreach ($path in $versioned) {
          if (!$cache.ContainsKey($path)) { continue }
          $marker=[regex]::Match($cache[$path],'\bOCMI\s+(\d+\.\d+\.\d+)\b')
          if (!$marker.Success) { Note unknown RULE_VERSION_UNKNOWN $path 'The selected rule version is not machine-readable; review it manually.' }
          elseif ($marker.Groups[1].Value -cne $version) { Note error RULE_VERSION_MISMATCH $path 'Reconcile current entry/rule version labels with VERSION; historical records are not rewritten.' }
        }
      }
    }
    if (!$InputFile -and $null -eq $Selection) { Note info TASK_NOT_REQUESTED '' 'Only the selected layout/links/version were checked; task, hashes, resources and initialization were not evaluated.' }
    else {
      $requestText=$null
      if ($InputFile -and $null -ne $Selection) {
        Note unknown REQUEST_AMBIGUOUS '' 'Select either an in-memory request or one request file; neither task selection was followed.'
      } elseif ($null -ne $Selection) {
        try {
          if ($Selection -isnot [Collections.IDictionary] -and $Selection -isnot [pscustomobject]) { throw 'Unsupported selection type' }
          $requestText=ConvertTo-Json -InputObject $Selection -Depth 16 -Compress -WarningAction Stop -ErrorAction Stop
          if ([Text.Encoding]::UTF8.GetByteCount($requestText) -gt 262144) { throw 'Selection too large' }
          Note info SELECTION_IN_MEMORY '' 'Temporary path/baseline selection; no request file or canonical metadata was written.'
        } catch {
          $requestText=$null
          Note unknown REQUEST_INVALID '' 'Use a bounded plain in-memory selection object; no task data was inferred.'
        }
      } else { $requestText=Read-Input $InputFile }
      $request=$null
      if ($null -ne $requestText) {
        try {
          $request=Parse-Json $requestText
          if ($request -is [Array] -or $request.schemaVersion -ne 1 -or $request.tasks -isnot [Array] -or $request.protected -isnot [Array] -or @($request.tasks).Count -gt 32 -or @($request.protected).Count -gt 32) { throw 'Unsupported request' }
        } catch { Note unknown REQUEST_INVALID $InputFile 'Use a supported bounded diagnostic request; no task readiness can be inferred.'; $request=$null }
      }
      if ($null -ne $request) {
        $current=Block $request.assignment 'assignment'
        $start=Block $request.start 'start'
        $tasks=[Collections.Generic.Dictionary[string,object]]::new([StringComparer]::Ordinal)
        foreach ($path in @($request.tasks)) {
          $task=Block $path 'task'
          if ($null -eq $task) { continue }
          if ($task.id -isnot [string] -or $task.id -cnotmatch '^[A-Za-z0-9][A-Za-z0-9_.-]{0,79}$' -or $task.revision -isnot [string] -or !$task.revision.Trim() -or $task.owner -isnot [string] -or !$task.owner.Trim() -or $task.state -cnotin @('draft','ready','running','review','accepted','integrated') -or $task.dependsOn -isnot [Array] -or $task.resources -isnot [Array] -or @($task.dependsOn).Count -gt 32 -or @($task.resources).Count -gt 32) {
            Note unknown TASK_INVALID $path 'Supply a supported task identity, revision, owner and state in the canonical record.'; continue
          }
          if ($tasks.ContainsKey($task.id)) { Note error TASK_AMBIGUOUS $path 'Resolve duplicate task identities in the selected records.'; continue }
          $tasks[$task.id]=$task
        }
        if ($null -ne $current -and $null -ne $start) {
          foreach ($key in @('id','revision','role','owner','workingCopy')) {
            if ($current.$key -isnot [string] -or $start.$key -isnot [string] -or !$current.$key.Trim() -or !$start.$key.Trim()) { Note unknown IDENTITY_MISSING '' 'Assignment and START need matching id/revision/role/owner/workingCopy.' }
            elseif ($key -cne 'workingCopy' -and $current.$key -cne $start.$key) { Note error IDENTITY_MISMATCH '' 'Reconcile assignment and START identity; do not execute stale or contradictory inputs.' }
          }
          if ($current.state -cne 'current' -or $start.state -cne 'ready' -or $current.role -cnotin @('worker','orchestrator','consultant')) {
            Note unknown ASSIGNMENT_STATE '' 'Use an explicitly current assignment and addressed ready START; no role or status is inferred.'
          }
          foreach ($record in @($current,$start)) {
            try {
              $copy=if ([IO.Path]::IsPathRooted($record.workingCopy)) {[IO.Path]::GetFullPath($record.workingCopy)} else {[IO.Path]::GetFullPath((Join-Path $base $record.workingCopy))}
              if (!$copy.TrimEnd('/','\').Equals($base.TrimEnd('/','\'),(Get-OCMIPathComparison))) { throw 'Wrong copy' }
            } catch { Note error COPY_MISMATCH '' 'Select the assigned working copy; no other copy was inspected.' }
          }
          if (!$tasks.ContainsKey($current.id)) { Note unknown CURRENT_TASK_MISSING '' 'Explicitly include the current canonical task record.' }
          elseif ($tasks[$current.id].revision -cne $current.revision -or $tasks[$current.id].owner -cne $current.owner) { Note error TASK_REVISION '' 'Reconcile the current task revision/owner with assignment and START.' }
          elseif ($tasks[$current.id].state -cnotin @('ready','running','review')) { Note unknown TASK_STATE '' 'The selected task is not in a supported current execution/review state; inspect its canonical record.' }
        }
        $visiting=@{}; $visited=@{}
        function Visit([string]$Id) {
          if ($visiting.ContainsKey($Id)) { Note error DEPENDENCY_CYCLE '' 'Resolve a cycle in the explicitly selected task dependencies.'; return }
          if ($visited.ContainsKey($Id)) { return }
          $visiting[$Id]=$true
          foreach ($dependency in @($tasks[$Id].dependsOn)) {
            if (!$tasks.ContainsKey([string]$dependency)) { Note unknown DEPENDENCY_MISSING '' 'Include the exact prerequisite record; do not assume it completed.'; continue }
            if ($tasks[$dependency].state -cne 'integrated') { Note error DEPENDENCY_UNMET '' 'A selected prerequisite lacks the required integrated state; DONE or acceptance alone is insufficient.' }
            Visit $dependency
          }
          $visiting.Remove($Id); $visited[$Id]=$true
        }
        $claims=@{}
        foreach ($id in @($tasks.Keys)) {
          Visit $id
          $task=$tasks[$id]
          if ($task.state -cin @('ready','running','review')) {
            foreach ($resource in @($task.resources)) {
              if (!$resource.key -or !$resource.owner -or $resource.owner -cne $task.owner) { Note unknown RESOURCE_INVALID '' 'Record one explicit owner per claimed resource; no process/port probe was performed.'; continue }
              $key=[string]$resource.key
              if ($claims.ContainsKey($key) -and $claims[$key] -cne $id) { Note error RESOURCE_CONFLICT '' 'Resolve conflicting live task claims or explicitly serialize/isolate the resource.' }
              $claims[$key]=$id
            }
          }
        }
        if (!$request.protected) { Note unknown HASHES_NOT_SELECTED '' 'Protected-input/origin integrity was not checked; select exact paths and trusted baseline hashes when required.' }
        foreach ($entry in @($request.protected | Where-Object {$null -ne $_})) {
          if ($entry.sha256 -cnotmatch '^[a-f0-9]{64}$') { Note unknown HASH_INVALID '' 'Provide a SHA256 from the approved baseline; a diagnostic snapshot cannot authenticate its own origin.'; continue }
          $text=Read-Input $entry.path
          if ($null -ne $text) {
            $hash=@($observations | Where-Object path -eq $entry.path.Replace('\','/'))[0].sha256
            if ($hash -cne $entry.sha256) { Note error HASH_MISMATCH $entry.path 'Preserve WIP and reconcile this changed input with its approved baseline.' }
          }
        }
      }
    }
  } catch { Note unknown INCOMPLETE '' 'Unsupported or inaccessible input prevented complete diagnostics; review the selected scope manually.' }
  $exitCode=if (@($findings | Where-Object level -eq 'error').Count) {1} elseif (@($findings | Where-Object level -eq 'unknown').Count) {2} else {0}
  [pscustomobject]@{
    schemaVersion=1;profile=$Profile;version=$version;status=@('checked','issues','incomplete')[$exitCode];exitCode=$exitCode
    readiness='not-evaluated';coverage=[pscustomobject]@{files=$budget.files;bytes=$budget.bytes;recursive=$false}
    findings=@($findings.ToArray());observations=@($observations.ToArray())
  }
}
