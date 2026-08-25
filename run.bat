@echo off
cd /d "%~dp0"

where python >nul 2>nul
if %ERRORLEVEL% equ 0 (
    set PYTHON_CMD=python
    set PYTHONW_CMD=pythonw
) else (
    where py >nul 2>nul
    if %ERRORLEVEL% equ 0 (
        set PYTHON_CMD=py
        set PYTHONW_CMD=pyw
    ) else (
        echo Error: Python is not installed or not in PATH.
        pause
        exit /b 1
    )
)

if not exist .venv (
    %PYTHON_CMD% -m venv .venv
)

call .venv\Scripts\activate.bat

pip install -r requirements.txt --quiet 2>nul

:: Launch with pythonw (no console window).
:: Use start "" to detach so this batch script can close immediately.
start "" .venv\Scripts\pythonw.exe main.py
