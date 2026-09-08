$exe = "D:\235备份文件\V2.4_Performance_Sandbox\e04_loadtest.exe"
$out = "D:\235备份文件\V2.4_Performance_Sandbox\tests\evidence\d3-batch2-p1\E08-rerun\raw-output.txt"
& $exe -concurrency 100 -duration 7200s -nodes "9001,9002,9003,9004,9005" -write-ratio 20 2>&1 | Out-File -FilePath $out -Encoding utf8