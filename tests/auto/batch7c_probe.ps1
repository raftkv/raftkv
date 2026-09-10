﻿$ErrorActionPreference = "Continue"
$projectRoot = (Get-Location).Path
$loadtest = Join-Path $projectRoot "e04_loadtest.exe"
$outFile = Join-Path $env:TEMP "batch7c_probe_out.txt"

Write-Output "loadtest path: $loadtest"
Write-Output "exists: $(Test-Path $loadtest)"

$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $loadtest
$psi.Arguments = "-duration=8s -concurrency=100 -write-ratio=20"
$psi.WorkingDirectory = $projectRoot
$psi.UseShellExecute = $false
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true

$proc = New-Object System.Diagnostics.Process
$proc.StartInfo = $psi

try {
    $started = $proc.Start()
    Write-Output "started=$started pid=$($proc.Id)"
} catch {
    Write-Output "start EXCEPTION: $($_.Exception.Message)"
    exit 1
}

try {
    $proc.BeginOutputReadLine()
    $proc.BeginErrorReadLine()
    Write-Output "BeginOutputReadLine OK"
} catch {
    Write-Output "beginread EXCEPTION: $($_.Exception.Message)"
}

while (-not $proc.HasExited) {
    Start-Sleep -Milliseconds 500
}
$proc.WaitForExit()
Write-Output "exit=$($proc.ExitCode)"