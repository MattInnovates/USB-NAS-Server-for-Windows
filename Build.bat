@echo off
echo Building usb-nas-cli from ./cmd...
go build -o usb-nas-cli.exe ./cmd
if %ERRORLEVEL% neq 0 (
    echo Build failed. Check the error messages above.
    pause
    exit /b 1
) else (
    echo Build succeeded.
    pause
)
exit
