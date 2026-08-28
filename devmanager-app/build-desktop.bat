@echo off
setlocal enabledelayedexpansion

:: Enhanced build script for devManager Desktop Application
:: Features: error handling, dependency checking, clean builds, detailed logging

set "BUILD_START_TIME=%time%"
set "ERROR_OCCURRED=0"
set "CLEAN_BUILD=0"

:: Parse command line arguments
if /i "%1"=="clean" set "CLEAN_BUILD=1"
if /i "%1"=="rebuild" set "CLEAN_BUILD=1"

echo.
echo ========================================
echo devManager Desktop Application Builder
echo ========================================
echo Build started at: %date% %BUILD_START_TIME%
echo.

:: Check dependencies
echo [STEP 1] Checking dependencies...

where node >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] node is not installed or not in PATH
    echo Please install node and try again
    goto :error
)
echo [OK] Found node

where npm >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] npm is not installed or not in PATH
    echo Please install npm and try again
    goto :error
)
echo [OK] Found npm

where go >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] go is not installed or not in PATH
    echo Please install go and try again
    goto :error
)
echo [OK] Found go

where wails >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] wails is not installed or not in PATH
    echo Please install wails and try again
    goto :error
)
echo [OK] Found wails

:: Change to script directory
cd /d "%~dp0"
echo [INFO] Working directory: %CD%

:: Clean build if requested
if %CLEAN_BUILD% neq 0 (
    echo.
    echo [STEP 2] Cleaning previous builds...
    if exist "frontend\dist" (
        echo [CLEAN] Removing frontend\dist...
        rmdir /s /q "frontend\dist"
    )
    if exist "build" (
        echo [CLEAN] Removing build directory...
        rmdir /s /q "build"
    )
    if exist "bin" (
        echo [CLEAN] Removing bin directory...
        rmdir /s /q "bin"
    )
    
    :: Clean Go modules cache
    echo [CLEAN] Cleaning Go modules cache...
    call go clean -cache -modcache -testcache >nul 2>&1
    
    echo [SUCCESS] Clean completed
    set "STEP_NUM=3"
) else (
    echo.
    echo [STEP 2] Skipping clean (use 'build-desktop.bat clean' to force clean)
    set "STEP_NUM=3"
)

:: Install frontend dependencies
echo.
echo [STEP %STEP_NUM%] Installing frontend dependencies...
echo [EXEC] npm install --prefix frontend --no-audit --no-fund
call npm install --prefix frontend --no-audit --no-fund
echo [SUCCESS] Frontend dependencies installed

:: Build frontend
echo.
echo [STEP %STEP_NUM%] Building frontend...
echo [EXEC] npm run build --prefix frontend
call npm run build --prefix frontend
echo [SUCCESS] Frontend built successfully

:: Build Wails application
echo.
echo [STEP %STEP_NUM%] Building Wails application...
echo [EXEC] wails build
call wails build
echo [SUCCESS] Wails application built successfully

:: Calculate build time
set "BUILD_END_TIME=%time%"

:: Success message
echo.
echo ========================================
echo BUILD COMPLETED SUCCESSFULLY!
echo ========================================
echo Build started at: %date% %BUILD_START_TIME%
echo Build completed at: %date% %BUILD_END_TIME%
echo.
echo Your application is available at: build\bin\devmanager.exe
echo.
goto :end

:error
echo.
echo [FATAL] Build failed due to errors above
echo.
exit /b 1

:end
pause