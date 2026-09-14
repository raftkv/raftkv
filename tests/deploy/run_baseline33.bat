@echo off
cd /d "D:\235备份文件\V2.4_Performance_Sandbox\tests\deploy"
echo [BASELINE-START] %date% %time% >> "D:\235备份文件\V2.4_Performance_Sandbox\tests\evidence\d3-batch33\pre_raft_baseline_30min.txt"
e04_loadtest.exe -duration=30m -concurrency=128 -write-ratio=20 >> "D:\235备份文件\V2.4_Performance_Sandbox\tests\evidence\d3-batch33\pre_raft_baseline_30min.txt" 2>&1
echo [BASELINE-END] %date% %time% >> "D:\235备份文件\V2.4_Performance_Sandbox\tests\evidence\d3-batch33\pre_raft_baseline_30min.txt"