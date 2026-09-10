﻿$ErrorActionPreference = "SilentlyContinue"
$evdir = "D:\235备份文件\V2.4_Performance_Sandbox\tests\evidence\d3-batch7"
$heartbeat = Join-Path $evdir "heartbeat.log"
$phaseFile = Join-Path $evdir "phase.txt"
$nodeCount = 5

while ($true) {
    $phase = "IDLE"
    if (Test-Path $phaseFile) {
        $phase = (Get-Content $phaseFile -Raw).Trim()
    }
    if ($phase -eq "STOP") { break }

    $ts = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $alive = 0
    for ($i = 1; $i -le $nodeCount; $i++) {
        $st = docker ps --filter "name=daijin235-node-$i" --format "{{.Status}}" 2>$null
        if ($st -match "Up") { $alive++ }
    }
    $mems = @()
    for ($i = 1; $i -le $nodeCount; $i++) {
        $m = docker stats --no-stream --format "{{.MemUsage}}" "daijin235-node-$i" 2>$null
        if ($m -match '([\d.]+)(MiB|GiB)') {
            $v = [double]$Matches[1]
            if ($Matches[2] -eq 'GiB') { $v = $v * 1024 }
            $mems += [math]::Round($v, 0)
        } else { $mems += -1 }
    }
    Add-Content -Path $heartbeat -Value "[$ts] phase=$phase alive=$alive/$nodeCount mem=$($mems -join ',')" -Encoding UTF8
    Start-Sleep 300
}