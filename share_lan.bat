@echo off
chcp 65001 >nul 2>&1
title RaftKV - 局域网分享

echo ============================================
echo   RaftKV 确定性管控中枢 - 局域网分享
echo ============================================
echo.

:: 检测本机局域网 IPv4 地址
set "LAN_IP="
for /f "tokens=2 delims=:" %%a in ('ipconfig ^| findstr /c:"IPv4"') do (
    for /f "tokens=1" %%b in ("%%a") do (
        set "LAN_IP=%%b"
    )
)

if "%LAN_IP%"=="" (
    echo [错误] 无法获取本机局域网 IP，请检查网络连接
    pause
    exit /b 1
)

echo.
echo   🚀 大屏已可访问，请分享给同WiFi的同事：
echo.
echo   http://%LAN_IP%:8096/daijing_235_dashboard.html
echo.
echo ============================================
echo.
echo 提示：如果同事打不开，请检查 Windows 防火墙是否放行了 8096 端口
echo.
pause