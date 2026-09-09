$dir = "<ARCHIVE>\V2.4_Performance_Sandbox\tests\evidence\d3-batch2-p1\E08-rerun5\batch5-fix-A-D"
$outFile = "$dir\loadtest-20min.txt"
$errFile = "$dir\loadtest-err.txt"

Set-Location "<ARCHIVE>\V2.4_Performance_Sandbox"

$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = "<ARCHIVE>\V2.4_Performance_Sandbox\e04_loadtest.exe"
$psi.Arguments = "-duration=20m -concurrency=100 -write-ratio=20"
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true
$psi.UseShellExecute = $false
$psi.WorkingDirectory = "<ARCHIVE>\V2.4_Performance_Sandbox"

$proc = [System.Diagnostics.Process]::Start($psi)
Write-Output "Loadtest PID: $($proc.Id)"

Start-Sleep -Seconds 300
Write-Output "Capturing T+5min profiles..."
for ($i = 1; $i -le 5; $i++) {
    $name = "daijin235-node-$i"
    docker exec $name curl -s -o /tmp/heap.prof http://127.0.0.1:9600/debug/pprof/heap 2>$null
    docker cp "${name}:/tmp/heap.prof" "$dir\heap-t5min-node$i.prof" 2>$null
    Write-Output "  node-$i done"
}

Start-Sleep -Seconds 600
Write-Output "Capturing T+15min profiles..."
for ($i = 1; $i -le 5; $i++) {
    $name = "daijin235-node-$i"
    docker exec $name curl -s -o /tmp/heap.prof http://127.0.0.1:9600/debug/pprof/heap 2>$null
    docker cp "${name}:/tmp/heap.prof" "$dir\heap-t15min-node$i.prof" 2>$null
    Write-Output "  node-$i done"
}

while (-not $proc.HasExited) {
    Start-Sleep -Seconds 5
}

$output = $proc.StandardOutput.ReadToEnd()
$output | Out-File -FilePath $outFile -Encoding UTF8
Write-Output "Loadtest complete. Exit code: $($proc.ExitCode)"
Write-Output "Output saved to $outFile"