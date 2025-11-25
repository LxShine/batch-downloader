@echo off
chcp 65001 >nul 2>nul
echo ========================================
echo    Icon Preparation Tool
echo ========================================
echo.
echo This tool helps you prepare icons for your application.
echo.
echo You need TWO icon files:
echo   1. icon.ico  - For EXE file icon (Windows Explorer)
echo   2. icon.png  - For window icon (app title bar)
echo.
echo ========================================
echo.

REM Check for icon.ico
if exist icon.ico (
    echo [OK] icon.ico found
    for %%I in (icon.ico) do echo      Size: %%~zI bytes
) else (
    echo [!] icon.ico NOT found
    echo     This is needed for EXE file icon
)

echo.

REM Check for icon.png
if exist icon.png (
    echo [OK] icon.png found
    for %%I in (icon.png) do echo      Size: %%~zI bytes
) else (
    echo [!] icon.png NOT found
    echo     This is needed for window icon
)

echo.
echo ========================================
echo.

if not exist icon.ico (
    if not exist icon.png (
        echo You need to prepare icon files:
        echo.
        echo Option 1: Download from icon websites
        echo   - https://icons8.com/icons/set/download
        echo   - Save as icon.png ^(PNG format^)
        echo.
        echo Option 2: Convert existing image
        echo   - Use online converter: https://convertio.co/
        echo   - Convert your image to PNG and ICO formats
        echo.
        echo After preparing icons:
        echo   1. Place icon.png in: %CD%
        echo   2. Place icon.ico in: %CD%
        echo   3. Run build.bat
        echo.
        pause
        exit /b
    )
)

if not exist icon.ico (
    echo Warning: icon.ico is missing
    echo EXE file will have default Windows icon
    echo.
)

if not exist icon.png (
    echo Warning: icon.png is missing  
    echo Window will have default Fyne icon
    echo.
)

echo Ready to build!
echo Run build.bat to compile with icons
echo.
pause
