@echo off
echo Building usb-nas-cli from ./cmd/main.go...
go build -o usb-nas-cli.exe ./cmd
if %ERRORLEVEL% neq 0 (
    echo Build failed.
) else (
    echo Build succeeded.
)
pause
