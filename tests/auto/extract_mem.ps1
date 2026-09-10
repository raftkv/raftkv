$csv = '<ARCHIVE>\V2.4_Performance_Sandbox\tests\auto\auto-run-20260910-040350\phase4_mem.csv'
$lines = Get-Content $csv | Select-Object -Skip 1
$nodePeaks = @{}
$globalPeak = 0
$globalPeakNode = ""
$globalPeakTs = ""
foreach ($line in $lines) {
    $parts = $line -split ','
    if ($parts.Count -ge 3) {
        $ts = $parts[0]; $node = $parts[1]; $mem = [double]$parts[2]
        if (-not $nodePeaks.ContainsKey($node) -or $mem -gt $nodePeaks[$node].mem) {
            $nodePeaks[$node] = @{ mem=$mem; ts=$ts }
        }
        if ($mem -gt $globalPeak) { $globalPeak=$mem; $globalPeakNode=$node; $globalPeakTs=$ts }
    }
}
Write-Output "=== Phase4 内存峰值 ==="
Write-Output "全局峰值: ${globalPeak}MiB (${globalPeakNode} @ ${globalPeakTs})"
Write-Output ""
Write-Output "各节点峰值:"
foreach ($k in ($nodePeaks.Keys | Sort-Object)) {
    $v = $nodePeaks[$k]
    Write-Output "  ${k}: $($v.mem)MiB @ $($v.ts)"
}