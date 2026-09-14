$csv = '.\tests\auto\auto-run-20260910-040350\phase2_mem.csv'
$lines = Get-Content $csv | Select-Object -Skip 1
$nodePeaks = @{}
$globalPeak = 0
$globalPeakNode = ""
foreach ($line in $lines) {
    $parts = $line -split ','
    if ($parts.Count -ge 3) {
        $node = $parts[1]; $mem = [double]$parts[2]
        if (-not $nodePeaks.ContainsKey($node) -or $mem -gt $nodePeaks[$node]) { $nodePeaks[$node] = $mem }
        if ($mem -gt $globalPeak) { $globalPeak=$mem; $globalPeakNode=$node }
    }
}
Write-Output "=== Phase2 (20min) 内存峰值 ==="
Write-Output "全局峰值: ${globalPeak}MiB (${globalPeakNode})"
Write-Output "各节点峰值:"
foreach ($k in ($nodePeaks.Keys | Sort-Object)) { Write-Output "  ${k}: $($nodePeaks[$k])MiB" }