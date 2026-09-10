@echo off
chcp 65001 >nul 2>&1
title ᷾ 235 - һ      
setlocal enabledelayedexpansion

echo.
echo  ========================================================
echo    ᷾ 235    ȷ   Թܿ         һ      
echo  ========================================================
echo.

:: ========   һ              ʵ IPv4 ========
echo  [1/4]   Ȿ         IP ...

set "LAN_IP="
for /f "tokens=2 delims=:" %%a in ('ipconfig ^| findstr /c:"IPv4"') do (
    for /f "tokens=1" %%b in ("%%a") do (
        set "c=%%b"
        echo !c! | findstr /b "192.168." >nul && set "LAN_IP=!c!"
        echo !c! | findstr /b "10." >nul && set "LAN_IP=!c!"
    )
)

if "%LAN_IP%"=="" (
    for /f "tokens=2 delims=:" %%a in ('ipconfig ^| findstr /c:"IPv4"') do (
        for /f "tokens=1" %%b in ("%%a") do (
            if not "%%b"=="127.0.0.1" if not "%%b"=="" set "LAN_IP=%%b"
        )
    )
)

if "%LAN_IP%"=="" (
    echo  [    ]  ޷   ȡ     IP
    pause & exit /b 1
)

echo  ?          ӣ http://%LAN_IP%:8096/daijing_235_dashboard.html
echo.

:: ========  ڶ          Ngrok ========
echo  [2/4]     Ngrok ...
set "NGROK_DIR=%~dp0ngrok_tool"
set "NGROK_EXE=%NGROK_DIR%\ngrok.exe"

if not exist "%NGROK_EXE%" (
    echo           Ngrok ...
    if not exist "%NGROK_DIR%" mkdir "%NGROK_DIR%"
    set "NGROK_ZIP=%NGROK_DIR%\ngrok.zip"
    powershell -Command "[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri 'https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-windows-amd64.zip' -OutFile \"%NGROK_ZIP%\""
    if not exist "%NGROK_ZIP%" (
        echo  [错误] 下载失败
        pause & exit /b 1
    )
    echo  正在解压 ...
    powershell -Command "Expand-Archive -Path \"%NGROK_ZIP%\" -DestinationPath \"%NGROK_DIR%\" -Force"
    del "%NGROK_ZIP%" >nul 2>&1
    echo  ? Ngrok        
) else (
    echo  ? Ngrok  Ѿ   
)
echo.

:: ========              authtoken ========
echo  [3/4]      Ngrok authtoken ...

set "HAS_TOKEN=0"
if exist "%USERPROFILE%\.ngrok2\ngrok.yml" (
    findstr /i "authtoken" "%USERPROFILE%\.ngrok2\ngrok.yml" >nul 2>&1 && set "HAS_TOKEN=1"
)
if exist "%USERPROFILE%\AppData\Local\ngrok\ngrok.yml" (
    findstr /i "authtoken" "%USERPROFILE%\AppData\Local\ngrok\ngrok.yml" >nul 2>&1 && set "HAS_TOKEN=1"
)

if "!HAS_TOKEN!"=="0" (
    echo.
    echo                                                                                                              
    echo       ״ ʹ         authtoken                            
    echo            ngrok.com ע      ˺ţ         authtoken   
    echo                                                                                                              
    echo.
    set /p "AUTHTOKEN=  ճ      authtoken: "
    if "!AUTHTOKEN!"=="" (
        echo  [    ] authtoken     Ϊ  
        pause & exit /b 1
    )
    "%NGROK_EXE%" config add-authtoken !AUTHTOKEN!
    if !errorlevel! neq 0 (
        echo  [    ] authtoken     ʧ  
        pause & exit /b 1
    )
    echo  ? authtoken    óɹ 
) else (
    echo  ? authtoken       
)
echo.

:: ========    Ĳ        Ngrok +   ȡ      ַ ========
echo  [4/4]              ...
echo.

taskkill /f /im ngrok.exe >nul 2>&1
start "" /b "%NGROK_EXE%" http 8096 >nul 2>&1

set "PUBLIC_URL="
for /l %%i in (1,1,20) do (
    if "!PUBLIC_URL!"=="" (
        timeout /t 1 /nobreak >nul
        for /f "tokens=*" %%u in ('powershell -Command "try{$r=Invoke-WebRequest -Uri 'http://127.0.0.1:4040/api/tunnels' -UseBasicParsing -TimeoutSec 3;$j=ConvertFrom-Json $r.Content;foreach($t in $j.tunnels){if($t.public_url -match '^https'){$t.public_url}}}catch{}" 2^>nul') do (
            set "PUBLIC_URL=%%u"
        )
    )
)

if "%PUBLIC_URL%"=="" (
    set "PUBLIC_URL=  ȡ  ...   鿴 http://127.0.0.1:4040"
)

:: ========    ս   ========
echo.
echo  ========================================================
echo.
echo    ?          ӣ ͬ WiFi  ɷ  ʣ   
echo    http://%LAN_IP%:8096/daijing_235_dashboard.html
echo.
echo    ??        ӣ ȫ  ɷ  ʣ   
echo    %PUBLIC_URL%/daijing_235_dashboard.html
echo.
echo    ??  Ngrok        : http://127.0.0.1:4040
echo.
echo  ========================================================
echo.
echo  ??           ͬ ´򲻿         Windows     ǽ 8096  ˿ 
echo.
echo          ˳   Ngrok        ں ̨   У ...
pause >nul