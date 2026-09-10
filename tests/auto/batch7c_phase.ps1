﻿param(
    [string]$Phase,
    [int]$DurationSec
)

$ErrorActionPreference = "Continue"
$projectRoot = (Get-Location).Path
$evdir = Join-Path $projectRoot "tests\evidence\d3-batch7"
New-Item -ItemType Directory -Force -Path $evdir | Out-Null
$loadtest = Join-Path $projectRoot "e04_loadtest.exe"
$outFile = Join-Path $evdir "soak-$Phase-output.txt"
$errFile = Join-Path $evdir "soak-$Phase-output.err"

$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $loadtest
$psi.Arguments = "-duration=${DurationSec}s -concurrency=100 -write-ratio=20"
$psi.WorkingDirectory = $projectRoot
$psi.UseShellExecute = $false
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true

$proc = New-Object System.Diagnostics.Process
$proc.StartInfo = $psi
try {
    $proc.Start() | Out-Null
} catch {
    Write-Output "START_EXCEPTION: $($_.Exception.Message)"
    exit 1
}

$out = $proc.StandardOutput.ReadToEnd()
$err = $proc.StandardError.ReadToEnd()
$proc.WaitForExit()

[System.IO.File]::WriteAllText($outFile, $out)
[System.IO.File]::WriteAllText($errFile, $err)

Write-Output "PHASE=$Phase DONE exit=$($proc.ExitCode) outLen=$($out.Length)"
($out -split "`r?`n") | Select-Object -Last 16