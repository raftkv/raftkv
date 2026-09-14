@echo off
chcp 65001 >nul 2>&1
title RaftKV - 公网穿透

echo ============================================
echo   RaftKV 确定性管控中枢 - 公网穿透 (Ngrok)
echo ============================================
echo.

:: 设置工作目录
set "NGROK_DIR=%~dp0ngrok_tool"
set "NGROK_EXE=%NGROK_DIR%\ngrok.exe"

:: 检查是否已下载 ngrok
if not exist "%NGROK_EXE%" (
    echo [1/3] 正在下载 Ngrok Windows 版本...
    echo.

    :: 创建目录
    if not exist "%NGROK_DIR%" mkdir "%NGROK_DIR%"

    :: 下载 ngrok zip
    set "NGROK_ZIP=%NGROK_DIR%\ngrok.zip"
    echo 正在从 https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-windows-amd64.zip 下载...
    powershell -Command "& {[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri 'https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-windows-amd64.zip' -OutFile '%NGROK_ZIP%'}"

    if not exist "%NGROK_ZIP%" (
        echo [错误] 下载失败，请检查网络连接
        echo 你也可以手动下载 ngrok 并解压到 ngrok_tool 文件夹
        pause
        exit /b 1
    )

    :: 解压
    echo 正在解压...
    powershell -Command "& {Expand-Archive -Path '%NGROK_ZIP%' -DestinationPath '%NGROK_DIR%' -Force}"

    :: 清理 zip
    del "%NGROK_ZIP%" >nul 2>&1

    echo 下载并解压完成！
    echo.
) else (
    echo [1/3] Ngrok 已存在，跳过下载
    echo.
)

:: 检查 authtoken 配置
echo [2/3] 检查 Ngrok 认证配置...
echo.

:: 检查是否已配置 authtoken
"%NGROK_EXE%" config check >nul 2>&1
if %errorlevel% neq 0 (
    echo ====================================================
    echo   首次使用 Ngrok 需要配置 authtoken
    echo.
    echo   步骤：
    echo   1. 访问 https://dashboard.ngrok.com/signup 注册免费账号
    echo   2. 登录后进入 https://dashboard.ngrok.com/get-started/your-authtoken
    echo   3. 复制你的 authtoken
    echo ====================================================
    echo.
    set /p AUTHTOKEN="请粘贴你的 authtoken: "
    echo.
    echo 正在配置 authtoken...
    "%NGROK_EXE%" config add-authtoken %AUTHTOKEN%
    if %errorlevel% neq 0 (
        echo [错误] authtoken 配置失败
        pause
        exit /b 1
    )
    echo authtoken 配置成功！
    echo.
)

:: 启动 ngrok
echo [3/3] 启动公网穿透...
echo.
echo   🌐 公网访问链接已生成，把下面这个网址发给任何人，都能看到你的大屏！
echo.
echo   注意：Ngrok 启动后会显示 Forwarding 地址，那就是你的公网链接
echo   格式类似: https://xxxx-xx-xx.ngrok-free.app
echo   在后面加上 /daijing_235_dashboard.html 即可访问
echo.
echo   完整链接示例: https://xxxx-xx-xx.ngrok-free.app/daijing_235_dashboard.html
echo.
echo ============================================
echo.

"%NGROK_EXE%" http 8096