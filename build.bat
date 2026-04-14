@echo off
chcp 65001 >nul 2>nul
echo ========================================
echo    Batch File Downloader - Wails Build
echo ========================================
echo.

echo [1/3] Checking wails CLI...
where wails >nul 2>&1
if errorlevel 1 (
    echo [!] wails not found. Installing...
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
)

echo [2/3] Building with wails (release mode)...
wails build -clean -platform windows/amd64

if exist "build\bin\batch-downloader.exe" (
    echo.
    echo ========================================
    echo    Build Success!
    echo ========================================
    echo.
    echo Output: build\bin\batch-downloader.exe
    for %%I in ("build\bin\batch-downloader.exe") do echo Size: %%~zI bytes
    echo.
) else (
    echo.
    echo ========================================
    echo    Build Failed!
    echo ========================================
    echo.
    echo Please check:
    echo 1. wails CLI installed?   go install github.com/wailsapp/wails/v2/cmd/wails@latest
    echo 2. Node.js installed?     https://nodejs.org
    echo 3. Check error messages above
    echo.
)

pause
