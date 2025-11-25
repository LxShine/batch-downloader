@echo off
chcp 65001 >nul 2>nul
echo ========================================
echo    Batch File Downloader - Build
echo ========================================
echo.

echo [1/5] Cleaning old files...
if exist batch-downloader.exe del /q batch-downloader.exe
if exist rsrc.syso del /q rsrc.syso

echo [2/5] Setting build environment...
set CGO_ENABLED=1
set GOOS=windows
set GOARCH=amd64

echo [3/5] Checking for icon files...
set HAS_ICO=0
set HAS_PNG=0

if exist icon.ico (
    echo [OK] icon.ico found - will embed in EXE
    set HAS_ICO=1
) else (
    echo [!] icon.ico not found - EXE will use default icon
)

if exist icon.png (
    echo [OK] icon.png found - will be used for window icon
    set HAS_PNG=1
) else (
    echo [!] icon.png not found - window will use default icon
)

if %HAS_ICO%==0 (
    if %HAS_PNG%==0 (
        echo.
        echo Warning: No icon files found!
        echo Run prepare-icon.bat for help
        echo.
        goto :build
    )
)

echo [4/5] Embedding icon resource...
echo Installing rsrc tool if needed...
go install github.com/akavel/rsrc@latest 2>nul

echo Generating resource file...
rsrc -manifest app.manifest -ico icon.ico -o rsrc.syso 2>nul
if errorlevel 1 (
    echo Warning: Failed to embed icon, continuing without it
)

:build
echo [5/5] Building (no console window)...
go build -ldflags="-s -w -H=windowsgui" -o batch-downloader.exe

if exist batch-downloader.exe (
    echo.
    echo ========================================
    echo    Build Success!
    echo ========================================
    echo.
    echo Output: batch-downloader.exe
    for %%I in (batch-downloader.exe) do echo Size: %%~zI bytes
    echo.
    echo Icons:
    if %HAS_ICO%==1 (
        echo   [OK] EXE icon embedded (icon.ico)
    ) else (
        echo   [!] EXE using default icon
    )
    if %HAS_PNG%==1 (
        echo   [OK] Window icon available (icon.png)
    ) else (
        echo   [!] Window using default icon
    )
    echo.
    echo Run batch-downloader.exe to start
    echo No console window will be shown
    echo.
    
    REM Clean up resource file
    if exist rsrc.syso del /q rsrc.syso
) else (
    echo.
    echo ========================================
    echo    Build Failed!
    echo ========================================
    echo.
    echo Please check:
    echo 1. GCC compiler installed?
    echo 2. In correct project directory?
    echo 3. Check error messages above
    echo.
)

pause
