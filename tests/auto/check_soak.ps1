$procId = [int](Get-Content '.\tests\auto\soak-pid.txt')
$proc = Get-Process -Id $procId -ErrorAction SilentlyContinue
if ($proc) {
    $elapsed = [int]((Get-Date) - $proc.StartTime).TotalMinutes
    Write-Output "RUNNING PID=$procId Elapsed=${elapsed}min"
} else {
    Write-Output "DONE"
}