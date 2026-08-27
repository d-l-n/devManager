@echo off
echo Building devManager Desktop Application...
cd /d "%~dp0"
echo Installing frontend dependencies...
call npm install --prefix frontend
echo Building frontend...
call npm run build --prefix frontend
echo Building desktop application...
call wails build
echo.
echo Build completed! 
echo Run 'run-desktop.bat' to start the application.
echo.
pause