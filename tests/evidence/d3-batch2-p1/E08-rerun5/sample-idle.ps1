$ErrorActionPreference = "Continue"
$outFile = "<ARCHIVE>\V2.4_Performance_Sandbox\tests\evidence\d3-batch2-p1\E08-rerun5\idle-baseline.csv"
$nodes = 1..5

$header = "timestamp"
foreach ($n in $nodes) { $header += ",node${n}_anon,node${n}_file,node${n}_rss,node${n}_commit" }
$header | Out-File -FilePath $outFile -Encoding UTF8

$rounds = 360
for ($i = 0; $i -lt $rounds; $i++) {
    $ts = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $line = $ts
    foreach ($n in $nodes) {
        $name = "daijin235-node-$n"
        $memstat = docker exec $name sh -c "grep -E '^(anon|file) ' /sys/fs/cgroup/memory.stat 2>/dev/null" 2>$null
        $anon = ($memstat | Where-Object { $_ -match "^anon " } | ForEach-Object { ($_ -split ' ')[1] })
        $file = ($memstat | Where-Object { $_ -match "^file " } | ForEach-Object { ($_ -split ' ')[1] })
        $rss = docker exec $name sh -c "grep VmRSS /proc/1/status 2>/dev/null" 2>$null
        $rssVal = ($rss | ForEach-Object { ($_ -split ':')[1].Trim() })
        $st = curl.exe -s --max-time 2 "http://localhost:900${n}/raft/status" 2>$null
        $commit = ""
        if ($st -match '"commit_index":(\d+)') { $commit = $Matches[1] }
        if (-not $anon) { $anon = 0 }
        if (-not $file) { $file = 0 }
        if (-not $rssVal) { $rssVal = 0 }
        $line += ",$anon,$file,$rssVal,$commit"
    }
    $line | Out-File -Append -FilePath $outFile -Encoding UTF8
    Start-Sleep -Seconds 5
}