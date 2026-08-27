@echo off
echo Starting devManager Desktop Application...
cd /d "%~dp0"
if exist "build\bin\devmanager.exe" (
    start "" "build\bin\devmanager.exe"
    echo devManager started successfully!
) else (
    echo devManager not found. Please build it first with: wails build
    pause
)