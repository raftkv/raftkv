param(
    [Parameter(Mandatory=$true)][string]$LoadtestOutput,
    [Parameter(Mandatory=$true)][string]$MemoryCsv,
    [Parameter(Mandatory=$true)][string]$ThresholdsFile,
    [Parameter(Mandatory=$true)][string]$ResultOutput
)

$ErrorActionPreference = "Stop"

$thresholds = Get-Content $ThresholdsFile -Raw | ConvertFrom-Json

$output = Get-Content $LoadtestOutput -Raw -ErrorAction SilentlyContinue
if (-not $output) {
    $result = @{ overall = "FAIL"; reason = "压测输出文件不存在或为空: $LoadtestOutput" }
    $result | ConvertTo-Json -Depth 5 | Set-Content $ResultOutput -Encoding UTF8
    Write-Output "FAIL: 压测输出文件不存在或为空"
    exit 1
}

$tps = 0
$p99 = 0
$successRate = 0
$failures = 0

if ($output -match '平均TPS:\s+(\d+)')            { $tps = [double]$Matches[1] }
if ($output -match 'P99:\s+([\d.]+)ms')           { $p99 = [double]$Matches[1] }
if ($output -match '成功率:\s+([\d.]+)%')         { $successRate = [double]$Matches[1] }
if ($output -match '失败:\s+(\d+)')               { $failures = [int64]$Matches[1] }

$memPeak = 0
if (Test-Path $MemoryCsv) {
    $lines = Get-Content $MemoryCsv | Select-Object -Skip 1
    foreach ($line in $lines) {
        $parts = $line -split ','
        if ($parts.Count -ge 3) {
            $mem = [double]$parts[2]
            if ($mem -gt $memPeak) { $memPeak = $mem }
        }
    }
}

$checks = @()
$checks += @{ name = "TPS";       value = $tps;          threshold = $thresholds.tps_min;                   op = ">="; pass = ($tps -ge $thresholds.tps_min) }
$checks += @{ name = "P99_ms";    value = $p99;          threshold = $thresholds.p99_max_ms;                op = "<="; pass = ($p99 -le $thresholds.p99_max_ms) }
$checks += @{ name = "成功率%";   value = $successRate;  threshold = $thresholds.success_rate_min_percent;  op = ">="; pass = ($successRate -ge $thresholds.success_rate_min_percent) }
$checks += @{ name = "内存峰值MiB"; value = $memPeak;    threshold = $thresholds.memory_peak_max_mib;       op = "<="; pass = ($memPeak -le $thresholds.memory_peak_max_mib) }

$overall = "PASS"
foreach ($c in $checks) {
    if (-not $c.pass) { $overall = "FAIL"; break }
}

$result = @{
    overall      = $overall
    tps          = $tps
    p99_ms       = $p99
    success_rate = $successRate
    failures     = $failures
    mem_peak_mib = $memPeak
    checks       = $checks
    thresholds   = $thresholds
}

$result | ConvertTo-Json -Depth 5 | Set-Content $ResultOutput -Encoding UTF8

foreach ($c in $checks) {
    $tag = if ($c.pass) { "PASS" } else { "FAIL" }
    Write-Output "$tag  $($c.name): $($c.value) $($c.op) $($c.threshold)"
}
Write-Output "Overall: $overall"