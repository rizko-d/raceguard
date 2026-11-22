@echo off
setlocal
cd /d "%~dp0"

echo [1/2] Starting RaceGuard Local Testlab (http://127.0.0.1:8090)...
start "RaceGuard Testlab" cmd /k go run ./testlab/api

timeout /t 2 /nobreak >nul

echo [2/2] Starting RaceGuard Studio (Wails Dev)...
set WAILS_DEV_ALLOWED_HOSTS=wails.localhost,localhost,127.0.0.1
wails dev

if errorlevel 1 (
    echo.
    echo Wails Dev exited with an error.
    pause
)
endlocal
