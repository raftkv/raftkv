$procId = [int](Get-Content 'D:\235备份文件\V2.4_Performance_Sandbox\tests\auto\soak-pid.txt')
$proc = Get-Process -Id $procId -ErrorAction SilentlyContinue
if ($proc) {
    $elapsed = [int]((Get-Date) - $proc.StartTime).TotalMinutes
    Write-Output "RUNNING PID=$procId Elapsed=${elapsed}min"
} else {
    Write-Output "DONE"
}