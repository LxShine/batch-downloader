@echo off
chcp 65001 >nul 2>nul
echo ========================================
echo    Batch File Downloader - Dev Mode
echo ========================================
echo.

echo [1/2] Checking wails CLI...
where wails >nul 2>&1
if errorlevel 1 (
    echo [!] wails not found. Installing...
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
)

echo [2/2] Starting dev mode (hot reload enabled)...
echo        Browser DevTools available | Live reload on save
echo.
wails dev

pause
