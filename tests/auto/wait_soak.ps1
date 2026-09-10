$procId = [int](Get-Content 'D:\235备份文件\V2.4_Performance_Sandbox\tests\auto\soak-pid.txt')
$proc = Get-Process -Id $procId -ErrorAction SilentlyContinue
if (-not $proc) {
    Write-Output "DONE"
    exit 0
}
try {
    $proc.WaitForExit()
    Write-Output "DONE"
} catch {
    Write-Output "STILL_RUNNING"
}