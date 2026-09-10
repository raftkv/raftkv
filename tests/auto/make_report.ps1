param(
    [Parameter(Mandatory=$true)][string]$StateFile,
    [Parameter(Mandatory=$true)][string]$RunDir,
    [Parameter(Mandatory=$true)][string]$OutputFile
)

$ErrorActionPreference = "Stop"
$state = Get-Content $StateFile -Raw | ConvertFrom-Json
$thresholds = Get-Content "$RunDir\..\thresholds.json" -Raw | ConvertFrom-Json

$sb = [System.Text.StringBuilder]::new()
[void]$sb.AppendLine("# E08 自动化长跑报告")
[void]$sb.AppendLine("")
[void]$sb.AppendLine("## 运行信息")
[void]$sb.AppendLine("| 项目 | 值 |")
[void]$sb.AppendLine("|------|-----|")
[void]$sb.AppendLine("| Run ID | $($state.run_id) |")
[void]$sb.AppendLine("| 开始时间 | $($state.start_time) |")
[void]$sb.AppendLine("| 结束时间 | $($state.end_time) |")
[void]$sb.AppendLine("| 当前阶段 | $($state.current_phase) |")
[void]$sb.AppendLine("| 阈值版本 | $($thresholds.version) |")
[void]$sb.AppendLine("")

[void]$sb.AppendLine("## 阶段结果")
[void]$sb.AppendLine("| 阶段 | 状态 | 耗时(s) |")
[void]$sb.AppendLine("|------|------|---------|")
foreach ($p in $state.phases.PSObject.Properties) {
    $v = $p.Value
    [void]$sb.AppendLine("| $($p.Name) | $($v.status) | $($v.duration_sec) |")
}
[void]$sb.AppendLine("")

function Add-TestSection($sb, $title, $resultFile, $outputFile) {
    [void]$sb.AppendLine("## $title")
    if (Test-Path $resultFile) {
        $r = Get-Content $resultFile -Raw | ConvertFrom-Json
        [void]$sb.AppendLine("| 指标 | 实测 | 阈值 | 判据 | 结果 |")
        [void]$sb.AppendLine("|------|------|------|------|------|")
        foreach ($c in $r.checks) {
            $tag = if ($c.pass) { "PASS" } else { "FAIL" }
            [void]$sb.AppendLine("| $($c.name) | $($c.value) | $($c.threshold) | $($c.op) | $tag |")
        }
        [void]$sb.AppendLine("| **Overall** | | | | **$($r.overall)** |")
    } else {
        [void]$sb.AppendLine("验收结果文件不存在: $resultFile")
    }
    if (Test-Path $outputFile) {
        [void]$sb.AppendLine("")
        [void]$sb.AppendLine("<details><summary>压测输出尾部</summary>")
        [void]$sb.AppendLine("")
        [void]$sb.AppendLine('```')
        Get-Content $outputFile -Tail 15 | ForEach-Object { [void]$sb.AppendLine($_) }
        [void]$sb.AppendLine('```')
        [void]$sb.AppendLine("")
        [void]$sb.AppendLine("</details>")
    }
    [void]$sb.AppendLine("")
}

Add-TestSection $sb "20min 对照压测" "$RunDir\phase2_accept.json" "$RunDir\phase2_loadtest.txt"
Add-TestSection $sb "E08 30min 终审" "$RunDir\phase4_accept.json" "$RunDir\phase4_loadtest.txt"

if ($state.phases.PSObject.Properties.Name -contains "P5") {
    Add-TestSection $sb "8h 浸泡" "$RunDir\phase5_accept.json" "$RunDir\phase5_loadtest.txt"
}

[void]$sb.AppendLine("## 节点存活检查")
$containers = docker ps -a --format "{{.Names}}`t{{.Status}}" 2>$null | Select-String "daijin235"
foreach ($line in $containers) {
    [void]$sb.AppendLine("- $line")
}
[void]$sb.AppendLine("")

[void]$sb.AppendLine("## 内存采样峰值")
foreach ($csv in @("phase2_mem.csv", "phase4_mem.csv", "phase5_mem.csv")) {
    $p = "$RunDir\$csv"
    if (Test-Path $p) {
        $peak = 0; $peakNode = ""
        Get-Content $p | Select-Object -Skip 1 | ForEach-Object {
            $parts = $_ -split ','
            if ($parts.Count -ge 3 -and [double]$parts[2] -gt $peak) { $peak = [double]$parts[2]; $peakNode = $parts[1] }
        }
        [void]$sb.AppendLine("- $csv : 峰值 $($peak)MiB ($peakNode)")
    }
}
[void]$sb.AppendLine("")

$overall = "PASS"
foreach ($p in $state.phases.PSObject.Properties) {
    if ($p.Value.status -ne "PASS") { $overall = "FAIL"; break }
}
[void]$sb.AppendLine("---")
[void]$sb.AppendLine("## 总结: **$overall**")

$sb.ToString() | Set-Content $OutputFile -Encoding UTF8
Write-Output "报告已生成: $OutputFile"