$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = "<ARCHIVE>\V2.4_Performance_Sandbox\e04_loadtest.exe"
$psi.Arguments = "-duration=1200s -concurrency=100 -write-ratio=20"
$psi.WorkingDirectory = "<ARCHIVE>\V2.4_Performance_Sandbox"
$psi.UseShellExecute = $false
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true
$proc = New-Object System.Diagnostics.Process
$proc.StartInfo = $psi
$proc.Start() | Out-Null
$pid_val = $proc.Id
Write-Output "LOADTEST_PID=$pid_val"
$out = $proc.StandardOutput.ReadToEnd()
$err = $proc.StandardError.ReadToEnd()
$proc.WaitForExit()
$outFile = "<ARCHIVE>\V2.4_Performance_Sandbox\tests\evidence\d3-batch8\probe-loadtest-output.txt"
$errFile = "<ARCHIVE>\V2.4_Performance_Sandbox\tests\evidence\d3-batch8\probe-loadtest-output.err"
[System.IO.File]::WriteAllText($outFile, $out)
[System.IO.File]::WriteAllText($errFile, $err)
Write-Output "LOADTEST_DONE exit=$($proc.ExitCode) outLen=$($out.Length)"
($out -split "`r?`n") | Select-Object -Last 20