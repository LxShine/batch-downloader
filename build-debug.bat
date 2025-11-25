@echo off
chcp 65001 >nul 2>nul
echo ========================================
echo    Batch File Downloader - Debug Build
echo ========================================
echo.

echo [1/3] Cleaning old files...
if exist batch-downloader-debug.exe del /q batch-downloader-debug.exe

echo [2/3] Setting build environment...
set CGO_ENABLED=1
set GOOS=windows
set GOARCH=amd64

echo [3/3] Building (with console window)...
go build -ldflags="-s -w" -o batch-downloader-debug.exe

if exist batch-downloader-debug.exe (
    echo.
    echo ========================================
    echo    Build Success!
    echo ========================================
    echo.
    echo Output: batch-downloader-debug.exe
    for %%I in (batch-downloader-debug.exe) do echo Size: %%~zI bytes
    echo.
    echo This version shows console window for debugging
    echo.
) else (
    echo.
    echo ========================================
    echo    Build Failed!
    echo ========================================
    echo.
)

pause
