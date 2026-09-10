$ErrorActionPreference = "SilentlyContinue"
$base = "D:\235备份文件\V2.4_Performance_Sandbox\tests\evidence\d3-batch6\scene-preservation"

# === 1. OOM 节点完整日志（用 docker logs 不限 tail）===
Write-Host "[1] 提取 OOM 节点完整日志..."
docker logs daijin235-node-4 2>&1 | Out-File "$base\node4-full.log" -Encoding UTF8
docker logs daijin235-node-5 2>&1 | Out-File "$base\node5-full.log" -Encoding UTF8
$n4 = (Get-Content "$base\node4-full.log").Count
$n5 = (Get-Content "$base\node5-full.log").Count
Write-Host "  node4: $n4 lines, node5: $n5 lines"

# 提取 OOM 前最后 200 行（找 snapshot/compact 关键词）
foreach ($n in @("node4","node5")) {
    $full = "$base\$n-full.log"
    $tail200 = "$base\$n-tail200.log"
    Get-Content $full -Tail 200 | Set-Content $tail200 -Encoding UTF8
    $kw = Select-String -Path $tail200 -Pattern "snapshot|compact|Snapshot|Compact|OOM|oom|kill|Kill|memory|Memory|alloc|Alloc" -SimpleMatch
    if ($kw) {
        $kwFile = "$base\$n-keywords.log"
        $kw | ForEach-Object { "$($_.LineNumber): $($_.Line)" } | Set-Content $kwFile -Encoding UTF8
        Write-Host "  $n keywords: $($kw.Count) matches -> $n-keywords.log"
    } else {
        Write-Host "  $n keywords: 0 matches"
    }
}

# === 2. 全部5节点 snapshot/WAL 文件清单 ===
Write-Host "[2] 提取 snapshot/WAL 文件清单..."
$inv = "$base\snapshot-inventory.txt"
"=== Snapshot/WAL 文件清单 ===" | Set-Content $inv -Encoding UTF8
"" | Add-Content $inv
for ($i = 1; $i -le 5; $i++) {
    $node = "daijin235-node-$i"
    "--- $node ---" | Add-Content $inv
    $status = docker ps -a --filter "name=$node" --format "{{.Status}}" 2>$null
    "Status: $status" | Add-Content $inv
    
    if ($status -match "Up") {
        # 存活节点：docker exec
        $files = docker exec $node ls -la /app/wal-data/ 2>$null
        "Files in /app/wal-data/:" | Add-Content $inv
        $files | Add-Content $inv
        # du -sh
        $du = docker exec $node du -sh /app/wal-data/ 2>$null
        "Total size: $du" | Add-Content $inv
    } else {
        # 已停止节点：用 docker cp 临时复制文件列表
        # 先尝试创建临时容器查看文件系统
        $files = docker cp "${node}:/app/wal-data/" "$base\$node-waldata" 2>&1
        if ($LASTEXITCODE -eq 0) {
            "Copied /app/wal-data/ to $node-waldata/" | Add-Content $inv
            $copiedFiles = Get-ChildItem "$base\$node-waldata" -Recurse | Select-Object FullName, Length, LastWriteTime
            foreach ($f in $copiedFiles) {
                "  $($f.Name) - $($f.Length) bytes - $($f.LastWriteTime)" | Add-Content $inv
            }
        } else {
            "docker cp failed: $files" | Add-Content $inv
        }
    }
    "" | Add-Content $inv
}
Write-Host "  -> $inv"

# === 3. WAL 目录总大小 ===
Write-Host "[3] 提取 WAL 目录总大小..."
$wal = "$base\wal-dir-sizes.txt"
"=== WAL 目录总大小 ===" | Set-Content $wal -Encoding UTF8
"" | Add-Content $wal
for ($i = 1; $i -le 5; $i++) {
    $node = "daijin235-node-$i"
    "--- $node ---" | Add-Content $wal
    $status = docker ps -a --filter "name=$node" --format "{{.Status}}" 2>$null
    "Status: $status" | Add-Content $wal
    if ($status -match "Up") {
        $du = docker exec $node du -sh /app/wal-data/ 2>$null
        "du -sh /app/wal-data/: $du" | Add-Content $wal
        $duDetail = docker exec $node du -h /app/wal-data/* 2>$null
        "Per file:" | Add-Content $wal
        $duDetail | Add-Content $wal
    } else {
        $dir = "$base\$node-waldata"
        if (Test-Path $dir) {
            $totalSize = (Get-ChildItem $dir -Recurse | Measure-Object -Property Length -Sum).Sum
            "Total size (from cp): $totalSize bytes ($([math]::Round($totalSize/1MB, 2)) MB)" | Add-Content $wal
            Get-ChildItem $dir -Recurse | ForEach-Object { "  $($_.Name) - $($_.Length) bytes" | Add-Content $wal }
        } else {
            "No data available (container stopped, cp failed)" | Add-Content $wal
        }
    }
    "" | Add-Content $wal
}
Write-Host "  -> $wal"

# === 4. 存活3节点 docker stats 快照 ===
Write-Host "[4] 提取 docker stats 快照..."
$stats = "$base\docker-stats-snapshot.txt"
"=== Docker Stats 快照 ===" | Set-Content $stats -Encoding UTF8
"" | Add-Content $stats
docker stats --no-stream --format "NAME={{.Name}} CPU={{.CPUPerc}} MEM={{.MemUsage}} MEM_PCT={{.MemPerc}} NET={{.NetIO}} BLOCK={{.BlockIO}}" 2>$null | Add-Content $stats
Write-Host "  -> $stats"

# === 汇总 ===
Write-Host ""
Write-Host "=== 现场保全完成 ==="
Get-ChildItem $base | ForEach-Object { Write-Host "  $($_.Name) ($($_.Length) bytes)" }
