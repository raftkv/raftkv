$ErrorActionPreference = "Continue"
$logDir = ".\tests\auto"
$stdout = "$logDir\soak-stdout.log"
$stderr = "$logDir\soak-stderr.log"

$p = Start-Process -FilePath "powershell.exe" `
    -ArgumentList @("-ExecutionPolicy", "Bypass", "-File", ".\tests\auto\run_e08_auto.ps1", "-Soak") `
    -RedirectStandardOutput $stdout `
    -RedirectStandardError $stderr `
    -PassThru `
    -NoNewWindow

$p.Id | Out-File "$logDir\soak-pid.txt" -Encoding UTF8
Write-Output "STARTED PID=$($p.Id)"