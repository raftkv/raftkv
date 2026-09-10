param(
    [switch]$Soak,
    [switch]$Resume,
    [switch]$Quick,
    [switch]$Evidence
)

$ErrorActionPreference = "Stop"

# === 路径常量 ===
$ProjectRoot  = "<ARCHIVE>\V2.4_Performance_Sandbox"
$AutoDir      = "$ProjectRoot\tests\auto"
$ComposeF1    = "$ProjectRoot\tests\deploy\docker-compose-5node.yml"
$ComposeF2    = "$ProjectRoot\tests\deploy\docker-compose-5node-ports.yml"
$Sm4KeyFile   = "$ProjectRoot\tests\deploy\.sm4_key"
$LoadtestExe  = "$ProjectRoot\e04_loadtest.exe"
$ThreshFile   = "$AutoDir\thresholds.json"

# === 阈值与时长 ===
$thr = Get-Content $ThreshFile -Raw | ConvertFrom-Json
if ($Quick) {
    $smokeDur = "10s";  $profileDur = "30s";  $e08Dur = "30s";  $soakDur = "1m"
} elseif ($Evidence) {
    $smokeDur = "30s";  $profileDur = "20m";  $e08Dur = "15m";  $soakDur = "8h"
} else {
    $smokeDur = $thr.smoke_duration;  $profileDur = $thr.profile_duration
    $e08Dur   = $thr.e08_duration;   $soakDur   = $thr.soak_duration
}
$sampleInt = $thr.sample_interval_sec
$nodeCount = $thr.node_count

# === Run 目录 ===
$runId   = Get-Date -Format "yyyyMMdd-HHmmss"
$runDir  = "$AutoDir\auto-run-$runId"
$stateF  = "$runDir\state.json"

# ── state ──
function Load-State { if (Test-Path $stateF) { return Get-Content $stateF -Raw | ConvertFrom-Json }; return $null }
function Save-State($s) { $s | ConvertTo-Json -Depth 5 | Set-Content $stateF -Encoding UTF8 }
function Init-State {
    @{ run_id=$runId; start_time=(Get-Date -Format "o"); end_time=$null; current_phase=0; phases=@{} } | ConvertTo-Json -Depth 5 | Set-Content $stateF -Encoding UTF8
}
function Update-Phase($n, $st, $dur) {
    $s = Load-State; $s.phases | Add-Member -MemberType NoteProperty -Name "P$n" -Value @{status=$st; duration_sec=$dur} -Force; $s.current_phase = $n + 1; Save-State $s
}
function Write-Fail($n, $reason) {
    $ts = Get-Date -Format "o"; $msg = "[$ts] Phase $n FAILED: $reason"
    Add-Content -Path "$runDir\FAILED.log" -Value $msg -Encoding UTF8
    Write-Host $msg -ForegroundColor Red
    $s = Load-State; $s.phases | Add-Member -MemberType NoteProperty -Name "P$n" -Value @{status="FAIL"; duration_sec=0} -Force; $s.end_time = $ts; Save-State $s
}

# ── docker ──
function Set-DockerEnv {
    $env:IMAGE_NAME="daijin235-v26:ci-knife"; $env:FP_ANCHOR="tcx4-v25-test"
    $env:GRPC_PORT="9500"; $env:HTTP_PORT="9000"
    $env:LICENSE_DIR="<HOME>/.daijin235/tcx4_test/licenses_v25"
    $env:SM4_KEY=(Get-Content $Sm4KeyFile -Raw) -replace '\s',''
}
function Wait-Healthy($t=90) {
    $dl=(Get-Date).AddSeconds($t)
    while((Get-Date) -lt $dl){
        $h=0; for($i=1;$i -le $nodeCount;$i++){ if((docker ps --filter "name=daijin235-node-$i" --format "{{.Status}}" 2>$null) -match "healthy"){$h++} }
        if($h -eq $nodeCount){return $true}; Start-Sleep 2
    }; return $false
}

# ── 内存采样 ──
function Start-MemSample($csv) {
    return Start-Job -ScriptBlock {
        param($c,$iv,$n)
        while($true){
            $ts=Get-Date -Format "yyyy-MM-ddTHH:mm:ss"
            for($i=1;$i -le $n;$i++){
                $st=docker stats --no-stream --format "{{.MemUsage}}" "daijin235-node-$i" 2>$null
                if($st -match '([\d.]+)(MiB|GiB)'){ $m=[double]$Matches[1]; if($Matches[2] -eq 'GiB'){$m*=1024}; Add-Content $c "$ts,node-$i,$m" }
            }
            Start-Sleep $iv
        }
    } -ArgumentList $csv,$sampleInt,$nodeCount
}
function Stop-MemSample($j){ if($j){ Stop-Job $j -EA SilentlyContinue; Remove-Job $j -EA SilentlyContinue } }

# ── 压测 ──
function Run-Loadtest($dur,$out) {
    $psi=New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName=$LoadtestExe; $psi.Arguments="-duration=$dur -concurrency=100 -write-ratio=20"
    $psi.RedirectStandardOutput=$true; $psi.RedirectStandardError=$true; $psi.UseShellExecute=$false; $psi.WorkingDirectory=$ProjectRoot
    $pr=[System.Diagnostics.Process]::Start($psi); $o=$pr.StandardOutput.ReadToEnd(); $pr.WaitForExit()
    $o | Set-Content $out -Encoding UTF8; return $pr.ExitCode
}

# ── 验收 ──
function Run-Accept($lt,$mem,$res) {
    & "$AutoDir\check_accept.ps1" -LoadtestOutput $lt -MemoryCsv $mem -ThresholdsFile $ThreshFile -ResultOutput $res | Out-Null
    return (Get-Content $res -Raw | ConvertFrom-Json).overall
}

# ── 节点存活 ──
function Check-Alive {
    $a=0; for($i=1;$i -le $nodeCount;$i++){ if((docker ps --filter "name=daijin235-node-$i" --format "{{.Status}}" 2>$null) -match "Up"){$a++} }; return $a
}

# ── pprof 采集 ──
function Get-HeapProfile($nodeIdx, $outDir, $tag) {
    $ts = Get-Date -Format "HHmmss"
    $out = "$outDir\heap_node${nodeIdx}_${tag}_${ts}.pb"
    $prevEAP = $ErrorActionPreference; $ErrorActionPreference = "SilentlyContinue"
    docker exec "daijin235-node-$nodeIdx" curl -s "http://127.0.0.1:9600/debug/pprof/heap" -o "/tmp/heap.pb" 2>$null
    docker cp "daijin235-node-${nodeIdx}:/tmp/heap.pb" $out 2>$null
    $ErrorActionPreference = $prevEAP
    if (Test-Path $out) { return $out }; return $null
}
function Get-GoroutineProfile($nodeIdx, $outDir, $tag) {
    $ts = Get-Date -Format "HHmmss"
    $out = "$outDir\goroutine_node${nodeIdx}_${tag}_${ts}.pb"
    $prevEAP = $ErrorActionPreference; $ErrorActionPreference = "SilentlyContinue"
    docker exec "daijin235-node-$nodeIdx" curl -s "http://127.0.0.1:9600/debug/pprof/goroutine" -o "/tmp/goroutine.pb" 2>$null
    docker cp "daijin235-node-${nodeIdx}:/tmp/goroutine.pb" $out 2>$null
    $ErrorActionPreference = $prevEAP
    if (Test-Path $out) { return $out }; return $null
}
function Get-AllProfiles($outDir, $tag) {
    for ($i = 1; $i -le $nodeCount; $i++) {
        $st = docker ps --filter "name=daijin235-node-$i" --format "{{.Status}}" 2>$null
        if ($st -match "Up") {
            Get-HeapProfile $i $outDir $tag
            Get-GoroutineProfile $i $outDir $tag
        }
    }
}

# ── 后台压测 ──
function Start-LoadtestBg($dur, $out) {
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $LoadtestExe; $psi.Arguments = "-duration=$dur -concurrency=100 -write-ratio=20"
    $psi.RedirectStandardOutput = $true; $psi.RedirectStandardError = $true
    $psi.UseShellExecute = $false; $psi.WorkingDirectory = $ProjectRoot
    $pr = [System.Diagnostics.Process]::Start($psi)
    return $pr
}
function Get-LatestTps($outFile) {
    if (-not (Test-Path $outFile)) { return -1 }
    $lines = Get-Content $outFile -Tail 5 -EA SilentlyContinue
    foreach ($line in $lines) {
        $parts = $line -split '\s+'
        if ($parts.Count -ge 3 -and $parts[0] -match '^\d+s$') {
            return [int]$parts[2]
        }
    }
    return -1
}

# === 阶段实现 ===

function P0-Env {
    Write-Host "[Phase 0] 环境自检..." -ForegroundColor Cyan; $s=Get-Date
    Set-DockerEnv
    $prevEAP = $ErrorActionPreference; $ErrorActionPreference = "SilentlyContinue"
    docker compose -f $ComposeF1 -f $ComposeF2 down --volumes 2>&1 | Out-Null
    docker compose -f $ComposeF1 -f $ComposeF2 up -d 2>&1 | Out-Null
    $ErrorActionPreference = $prevEAP
    if(-not(Wait-Healthy 90)){ Write-Fail 0 "5节点未在90s内healthy"; return $false }
    $d=[int]((Get-Date)-$s).TotalSeconds; Update-Phase 0 "PASS" $d; Write-Host "[Phase 0] PASS (${d}s)" -ForegroundColor Green; return $true
}

function P1-Smoke {
    Write-Host "[Phase 1] 冒烟..." -ForegroundColor Cyan; $s=Get-Date
    Start-Sleep 5
    $prevEAP = $ErrorActionPreference; $ErrorActionPreference = "SilentlyContinue"
    $lp=0; for($i=1;$i -le $nodeCount;$i++){ $p=9000+$i; if((curl.exe -s "http://localhost:${p}/raft/status" 2>$null) -match '"state"\s*:\s*"Leader"'){$lp=$p;break} }
    $ErrorActionPreference = $prevEAP
    if($lp -eq 0){ Write-Fail 1 "未找到Leader"; return $false }
    $ok=0; for($i=1;$i -le 10;$i++){ $b='{"key":"smoke-'+$i+'","value":"v'+$i+'"}'; if((curl.exe -s -X POST "http://localhost:${lp}/raft/propose" -H "Content-Type: application/json" -d $b 2>$null).Length -gt 0){$ok++} }
    if($ok -ne 10){ Write-Fail 1 "写入ok=$ok/10"; return $false }
    if((Run-Loadtest $smokeDur "$runDir\phase1_smoke.txt") -ne 0){ Write-Fail 1 "冒烟压测异常"; return $false }
    $d=[int]((Get-Date)-$s).TotalSeconds; Update-Phase 1 "PASS" $d; Write-Host "[Phase 1] PASS (${d}s)" -ForegroundColor Green; return $true
}

function P2-Profile {
    Write-Host "[Phase 2] ${profileDur}对照压测..." -ForegroundColor Cyan; $s=Get-Date
    $mem="$runDir\phase2_mem.csv"; "timestamp,node,mem_mib" | Set-Content $mem -Encoding UTF8
    $j=Start-MemSample $mem; $ec=Run-Loadtest $profileDur "$runDir\phase2_loadtest.txt"; Stop-MemSample $j
    if($ec -ne 0){ Write-Fail 2 "压测退出码=$ec"; return $false }
    $d=[int]((Get-Date)-$s).TotalSeconds; Update-Phase 2 "PASS" $d; Write-Host "[Phase 2] PASS (${d}s)" -ForegroundColor Green; return $true
}

function P3-Accept2 {
    Write-Host "[Phase 3] 机械验收(对照)..." -ForegroundColor Cyan; $s=Get-Date
    $r=Run-Accept "$runDir\phase2_loadtest.txt" "$runDir\phase2_mem.csv" "$runDir\phase2_accept.json"
    if($r -ne "PASS"){ Write-Fail 3 "对照机械验收FAIL"; return $false }
    $d=[int]((Get-Date)-$s).TotalSeconds; Update-Phase 3 "PASS" $d; Write-Host "[Phase 3] PASS (${d}s)" -ForegroundColor Green; return $true
}

function P4-E08 {
    Write-Host "[Phase 4] E08 ${e08Dur}终审..." -ForegroundColor Cyan; $s=Get-Date
    $mem="$runDir\phase4_mem.csv"; "timestamp,node,mem_mib" | Set-Content $mem -Encoding UTF8
    $ltOut = "$runDir\phase4_loadtest.txt"
    
    if ($Evidence) {
        $pprofDir = "C:\temp\e08_evidence\pprof"; New-Item -ItemType Directory -Force -Path $pprofDir | Out-Null
        $tsLog = "$runDir\pprof_timeline.csv"; "timestamp,action,tag,detail" | Set-Content $tsLog -Encoding UTF8
        Write-Host "  [Evidence] pprof 采集目录: $pprofDir" -ForegroundColor Yellow
        
        $j = Start-MemSample $mem
        $tmpDir = "C:\temp\e08_evidence"
        New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
        Copy-Item $LoadtestExe "$tmpDir\e04_loadtest.exe" -Force
        $tmpOut = "$tmpDir\lt_out.txt"
        if (Test-Path $tmpOut) { Remove-Item $tmpOut -Force }
        $prevEAP = $ErrorActionPreference; $ErrorActionPreference = "SilentlyContinue"
        $pr = Start-Process -FilePath "$tmpDir\e04_loadtest.exe" -ArgumentList "-duration=$e08Dur", "-concurrency=100", "-write-ratio=20" -RedirectStandardOutput $tmpOut -RedirectStandardError "$tmpDir\lt_err.txt" -NoNewWindow -PassThru
        $ErrorActionPreference = $prevEAP
        Write-Host "  [Evidence] 压测已后台启动 (PID=$($pr.Id)) 输出->$tmpOut" -ForegroundColor Yellow
        
        $crashDetected = $false
        $prId = $pr.Id
        while ($true) {
            $procCheck = Get-Process -Id $prId -EA SilentlyContinue
            if (-not $procCheck) { break }
            Start-Sleep 60
            $now = Get-Date -Format "yyyy-MM-ddTHH:mm:ss"
            for ($i = 1; $i -le $nodeCount; $i++) {
                $st = docker ps --filter "name=daijin235-node-$i" --format "{{.Status}}" 2>$null
                if ($st -match "Up") { Get-HeapProfile $i $pprofDir "periodic" }
            }
            $tps = Get-LatestTps $tmpOut
            $alive = Check-Alive
            Write-Host "  [Evidence] $now TPS=$tps Alive=$alive/$nodeCount" -ForegroundColor Gray
            Add-Content $tsLog "$now,periodic,60s,TPS=$tps Alive=$alive/$nodeCount" -Encoding UTF8
            
            if (($tps -ge 0 -and $tps -lt 1000) -or $alive -lt $nodeCount) {
                if (-not $crashDetected) {
                    $crashDetected = $true
                    Write-Host "  [Evidence] !!! 检测到异常 TPS=$tps Alive=$alive/$nodeCount — 立即抓全量 profile !!!" -ForegroundColor Red
                    Add-Content $tsLog "$now,crash,detected,TPS=$tps Alive=$alive/$nodeCount" -Encoding UTF8
                    Get-AllProfiles $pprofDir "crash"
                    for ($i = 1; $i -le 5; $i++) {
                        $st = docker ps -a --filter "name=daijin235-node-$i" --format "{{.Status}}" 2>$null
                        Write-Host "    node-${i}: $st" -ForegroundColor Gray
                        Add-Content $tsLog "$now,crash,node-$i,$st" -Encoding UTF8
                    }
                }
            }
        }
        $pr.WaitForExit()
        Stop-MemSample $j
        if (Test-Path $tmpOut) { Copy-Item $tmpOut $ltOut -Force }
        $ec = $pr.ExitCode
        if ($ec -eq $null) { $ec = 1 }
        Write-Host "  [Evidence] 压测结束 ExitCode=$ec" -ForegroundColor Yellow
        Get-AllProfiles $pprofDir "final"
        Add-Content $tsLog "$(Get-Date -Format 'o'),final,profiles,ExitCode=$ec" -Encoding UTF8
    } else {
        $j=Start-MemSample $mem; $ec=Run-Loadtest $e08Dur $ltOut; Stop-MemSample $j
    }
    
    if($ec -ne 0){
        $hasResult = $false
        if (Test-Path $ltOut) { $hasResult = (Select-String -Path $ltOut -Pattern "=== E04 最终结果 ===" -Quiet -EA SilentlyContinue) }
        if (-not $hasResult) { Write-Fail 4 "E08退出码=$ec 且无有效输出"; return $false }
        Write-Host "  [Warning] ExitCode=$ec 但有有效输出，继续验收" -ForegroundColor Yellow
    }
    $a=Check-Alive; if($a -ne $nodeCount){ Write-Fail 4 "存活=$a/$nodeCount"; return $false }
    $r=Run-Accept $ltOut $mem "$runDir\phase4_accept.json"
    if($r -ne "PASS"){ Write-Fail 4 "E08机械验收FAIL"; return $false }
    $d=[int]((Get-Date)-$s).TotalSeconds; Update-Phase 4 "PASS" $d; Write-Host "[Phase 4] PASS (${d}s)" -ForegroundColor Green; return $true
}

function P5-Soak {
    Write-Host "[Phase 5] ${soakDur}浸泡..." -ForegroundColor Cyan; $s=Get-Date
    $mem="$runDir\phase5_mem.csv"; "timestamp,node,mem_mib" | Set-Content $mem -Encoding UTF8
    $j=Start-MemSample $mem; $ec=Run-Loadtest $soakDur "$runDir\phase5_loadtest.txt"; Stop-MemSample $j
    if($ec -ne 0){ Write-Fail 5 "浸泡退出码=$ec"; return $false }
    $a=Check-Alive; if($a -ne $nodeCount){ Write-Fail 5 "存活=$a/$nodeCount"; return $false }
    $r=Run-Accept "$runDir\phase5_loadtest.txt" $mem "$runDir\phase5_accept.json"
    if($r -ne "PASS"){ Write-Fail 5 "浸泡验收FAIL"; return $false }
    $d=[int]((Get-Date)-$s).TotalSeconds; Update-Phase 5 "PASS" $d; Write-Host "[Phase 5] PASS (${d}s)" -ForegroundColor Green; return $true
}

function P6-Report {
    Write-Host "[Phase 6] 生成报告..." -ForegroundColor Cyan; $s=Get-Date
    $st=Load-State; $st.end_time=Get-Date -Format "o"; Save-State $st
    & "$AutoDir\make_report.ps1" -StateFile $stateF -RunDir $runDir -OutputFile "$runDir\REPORT.md"
    $d=[int]((Get-Date)-$s).TotalSeconds; Update-Phase 6 "PASS" $d; Write-Host "[Phase 6] PASS (${d}s)" -ForegroundColor Green; return $true
}

# === 主流程 ===

$startPhase = 0
if ($Resume) {
    $latest = Get-ChildItem "$AutoDir\auto-run-*\state.json" -EA SilentlyContinue | Sort LastWriteTime -Desc | Select -First 1
    if ($latest) { $runDir=$latest.DirectoryName; $stateF=$latest.FullName; $startPhase=(Load-State).current_phase; Write-Host "续跑: $runDir 从阶段$startPhase" -ForegroundColor Yellow }
    else { New-Item -ItemType Directory -Force -Path $runDir | Out-Null; Init-State }
} else {
    New-Item -ItemType Directory -Force -Path $runDir | Out-Null; Init-State
}

$ok = $true
if ($startPhase -le 0 -and $ok) { $ok = P0-Env }
if ($startPhase -le 1 -and $ok) { $ok = P1-Smoke }
if (-not $Evidence) {
    if ($startPhase -le 2 -and $ok) { $ok = P2-Profile }
    if ($startPhase -le 3 -and $ok) { $ok = P3-Accept2 }
}
if ($startPhase -le 4 -and $ok) { $ok = P4-E08 }
if ($ok -and $Soak -and $startPhase -le 5) { $ok = P5-Soak }
if ($ok) { P6-Report }

if ($ok) { Write-Host "`n=== 全部通过 ===`n报告: $runDir\REPORT.md" -ForegroundColor Green }
else    { Write-Host "`n=== 失败 ===`n查看: $runDir\FAILED.log" -ForegroundColor Red }