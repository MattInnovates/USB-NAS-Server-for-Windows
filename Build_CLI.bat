@echo off
REM ================================
REM usb-nas-cli Build Script
REM ================================

echo.
echo Building usb-nas-cli.exe...
echo.

REM Check if Go is installed
where go >nul 2>&1
if errorlevel 1 (
    echo ERROR: Go is not installed or not in your PATH.
    pause
    exit /b 1
)

REM Build the executable from main.go
go build -o usb-nas-cli.exe cmd/main.go
if errorlevel 1 (
    echo ERROR: Build failed. See output above for details.
    pause
    exit /b 1
) else (
    echo SUCCESS: Build succeeded! usb-nas-cli.exe created.
)

echo.
pause
