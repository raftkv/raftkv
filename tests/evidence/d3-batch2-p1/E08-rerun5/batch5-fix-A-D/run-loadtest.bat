@echo off
cd /d "<ARCHIVE>\V2.4_Performance_Sandbox"
e04_loadtest.exe -duration=20m -concurrency=100 -write-ratio=20