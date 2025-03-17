@echo off
setlocal enabledelayedexpansion

REM --- Read the current version from version.txt ---
if not exist version.txt (
    echo 1.0.0 > version.txt
)
set /p version=<version.txt
echo Current version: %version%

REM --- Split the version into major, minor, and patch components ---
for /f "tokens=1-3 delims=." %%a in ("%version%") do (
    set major=%%a
    set minor=%%b
    set patch=%%c
)
echo Major: %major%, Minor: %minor%, Patch: %patch%

REM --- Increment the patch number ---
set /a patch=%patch%+1
set newVersion=%major%.%minor%.%patch%
echo New version: %newVersion%

REM --- Write the new version back to version.txt ---
echo %newVersion% > version.txt

REM --- Build the executable using ldflags to set main.currentVersion ---
go build -ldflags "-X main.currentVersion=%newVersion%" -o usb-nas-cli-v%newVersion%.exe cmd/main.go

echo Build completed: usb-nas-cli-v%newVersion%.exe
pause
