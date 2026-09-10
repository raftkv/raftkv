$ErrorActionPreference = "Continue"
$testDir = "D:\235备份文件\V2.4_Performance_Sandbox\tests\evidence\d3-batch2-p1\E08-rerun4"
$outputFile = "$testDir\raw-output.txt"
$exePath = "D:\235备份文件\V2.4_Performance_Sandbox\e04_loadtest.exe"
$workDir = "D:\235备份文件\V2.4_Performance_Sandbox"

$startMem = Get-CimInstance Win32_OperatingSystem
$freeGB = [math]::Round($startMem.FreePhysicalMemory/1MB, 1)
$startTime = Get-Date
Write-Output "E08-rerun4 START: $($startTime.ToString('yyyy-MM-dd HH:mm:ss'))"
Write-Output "开测前可用内存: ${freeGB}GB"

$job = Start-Job -ScriptBlock {
    param($exe, $out, $wd)
    Set-Location $wd
    & $exe -concurrency 100 -duration 7200s -nodes "9001,9002,9003,9004,9005" -write-ratio 20 2>&1 | Out-File -FilePath $out -Encoding UTF8
} -ArgumentList $exePath, $outputFile, $workDir

Write-Output "压测Job ID: $($job.Id)"

$abort = $false
$lastSnapMinute = 0

while ($job.State -eq "Running") {
    Start-Sleep -Seconds 30
    $minute = [int]((Get-Date) - $startTime).TotalMinutes
    if ($minute -ge ($lastSnapMinute + 30) -and $minute -le 125) {
        $lastSnapMinute = $minute
        $snapFile = "$testDir\docker-snapshot-${minute}min.txt"
        $snap = docker ps -a --filter "name=daijin235" --format "{{.Names}}: {{.Status}}" 2>&1
        $snap | Out-File -FilePath $snapFile -Encoding UTF8
        $exited = $snap | Where-Object { $_ -match "Exited \((?!0\))" }
        if ($exited) {
            Write-Output "!!! 容器异常退出 @${minute}min !!!"
            $exited | ForEach-Object { Write-Output $_ }
            Write-Output "该轮数据即时作废，停止压测"
            Stop-Job -Job $job
            $abort = $true
            break
        }
        $os = Get-CimInstance Win32_OperatingSystem
        $memPct = [math]::Round(($os.TotalVisibleMemorySize - $os.FreePhysicalMemory)/$os.TotalVisibleMemorySize*100, 1)
        $upCount = ($snap | Where-Object { $_ -match "Up" }).Count
        Write-Output "[${minute}min] 快照已保存 | 内存=${memPct}% | ${upCount}/5 Up"
    }
}

Receive-Job -Job $job 2>&1 | Out-Null
Remove-Job -Job $job -Force

$endTime = Get-Date
$elapsed = ($endTime - $startTime).TotalMinutes
$endMem = Get-CimInstance Win32_OperatingSystem
$endFreeGB = [math]::Round($endMem.FreePhysicalMemory/1MB, 1)
Write-Output "E08-rerun4 END: $($endTime.ToString('yyyy-MM-dd HH:mm:ss'))"
Write-Output "耗时: $([math]::Round($elapsed, 1))分钟"
Write-Output "结束时可用内存: ${endFreeGB}GB"
if ($abort) { Write-Output "!!! 该轮已作废 !!!" }
