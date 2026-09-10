﻿$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = "powershell.exe"
$psi.Arguments = '-NoProfile -ExecutionPolicy Bypass -File "<ARCHIVE>\V2.4_Performance_Sandbox\tests\auto\batch7c_heartbeat.ps1"'
$psi.UseShellExecute = $false
$psi.CreateNoWindow = $true
$hb = [System.Diagnostics.Process]::Start($psi)
Write-Output "heartbeat pid=$($hb.Id)"