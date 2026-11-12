function Select-DurableShard {
 param([string[]]$Inventory,[int]$Index,[int]$Count,[string]$Pattern)
 if($Count -lt 1 -or $Index -lt 0 -or $Index -ge $Count -or !$Inventory.Count -or $Count -gt $Inventory.Count){throw 'Invalid shard or inventory'}
 $seen=[Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
 foreach($name in $Inventory){if($name -cnotmatch ($Pattern+'[A-Za-z0-9_]+$') -or !$seen.Add($name)){throw 'Invalid or duplicate durable name'}}
 $sorted=[string[]]$Inventory.Clone();[Array]::Sort($sorted,[StringComparer]::Ordinal)
 for($i=0;$i -lt $sorted.Count;$i++){if(($i % $Count) -eq $Index){$sorted[$i]}}
}
