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

:: Function to check if command exists
:checkCommand
where %1 >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] %1 is not installed or not in PATH
    echo Please install %1 and try again
    set "ERROR_OCCURRED=1"
    goto :eof
)
echo [OK] Found %1
goto :eof

:: Function to execute command with error checking
:executeCommand
echo.
echo [EXEC] %*
%*
if %errorlevel% neq 0 (
    echo [ERROR] Command failed with exit code %errorlevel%
    echo Command: %*
    set "ERROR_OCCURRED=1"
    goto :eof
)
echo [SUCCESS] Command completed successfully
goto :eof

:: Check dependencies
echo [STEP 1] Checking dependencies...
call :checkCommand node
call :checkCommand npm
call :checkCommand go
call :checkCommand wails

if %ERROR_OCCURRED% neq 0 (
    echo.
    echo [FATAL] Dependency check failed. Please install missing tools.
    goto :error
)

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
) else (
    echo.
    echo [STEP 2] Skipping clean (use 'build-desktop.bat clean' to force clean)"
)

:: Install frontend dependencies
echo.
echo [STEP 3] Installing frontend dependencies...
call :executeCommand npm install --prefix frontend --no-audit --no-fund

if %ERROR_OCCURRED% neq 0 goto :error

:: Verify frontend dependencies
echo [INFO] Verifying frontend dependencies...
if not exist "frontend\node_modules" (
    echo [ERROR] Frontend node_modules not found after install
    set "ERROR_OCCURRED=1"
    goto :error
)

:: Build frontend
echo.
echo [STEP 4] Building frontend...
call :executeCommand npm run build --prefix frontend

if %ERROR_OCCURRED% neq 0 goto :error

:: Verify frontend build
echo [INFO] Verifying frontend build...
if not exist "frontend\dist" (
    echo [ERROR] Frontend dist directory not found after build
    set "ERROR_OCCURRED=1"
    goto :error
)

:: Check if frontend build has content
dir /b "frontend\dist\*" >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Frontend dist directory is empty
    set "ERROR_OCCURRED=1"
    goto :error
)

:: Build desktop application
echo.
echo [STEP 5] Building desktop application...
call :executeCommand wails build -clean

if %ERROR_OCCURRED% neq 0 goto :error

:: Verify build output
echo.
echo [STEP 6] Verifying build output...
set "BUILD_OUTPUT_FOUND=0"

:: Check common output directories
if exist "build\bin" (
    set "BUILD_OUTPUT_FOUND=1"
    echo [INFO] Found build output in build\bin\
    dir /b "build\bin\*"
) else if exist "bin" (
    set "BUILD_OUTPUT_FOUND=1"
    echo [INFO] Found build output in bin\
    dir /b "bin\*"
)

if %BUILD_OUTPUT_FOUND% equ 0 (
    echo [WARNING] Could not locate build output directory
    echo This might be normal depending on your Wails configuration
)

:: Calculate build time
set "BUILD_END_TIME=%time%"
call :calculateDuration "%BUILD_START_TIME%" "%BUILD_END_TIME%"

:: Success message
echo.
echo ========================================
echo BUILD COMPLETED SUCCESSFULLY!
echo ========================================
echo Build duration: !DURATION!
echo Build date: %date% %BUILD_END_TIME%
echo.

if %BUILD_OUTPUT_FOUND% equq 1 (
    echo To run the application:
    echo   run-desktop.bat
    echo.
    echo Or execute directly from the build output directory
) else (
    echo Check your Wails configuration for output location
    echo Run 'run-desktop.bat' to start the application
)

echo.
pause
goto :eof

:calculateDuration
set "START=%~1"
set "END=%~2"

:: Remove time separators and convert to hundredths
set "START_H=!START:~0,2!"
set "START_M=!START:~3,2!"
set "START_S=!START:~6,2!"
set "START_CS=!START:~9,2!"

set "END_H=!END:~0,2!"
set "END_M=!END:~3,2!"
set "END_S=!END:~6,2!"
set "END_CS=!END:~9,2!"

:: Convert to total hundredths of second
set /a "START_TOTAL=!START_H!*360000 + !START_M!*6000 + !START_S!*100 + !START_CS!"
set /a "END_TOTAL=!END_H!*360000 + !END_M!*6000 + !END_S!*100 + !END_CS!"

:: Calculate duration
set /a "DURATION_TOTAL=!END_TOTAL! - !START_TOTAL!"

:: Convert back to HH:MM:SS.CS format
set /a "DURATION_H=!DURATION_TOTAL!/360000"
set /a "DURATION_TOTAL=!DURATION_TOTAL!%%360000"
set /a "DURATION_M=!DURATION_TOTAL!/6000"
set /a "DURATION_TOTAL=!DURATION_TOTAL!%%6000"
set /a "DURATION_S=!DURATION_TOTAL!/100"
set /a "DURATION_CS=!DURATION_TOTAL!%%100"

:: Format with leading zeros
if !DURATION_H! lss 10 set "DURATION_H=0!DURATION_H!"
if !DURATION_M! lss 10 set "DURATION_M=0!DURATION_M!"
if !DURATION_S! lss 10 set "DURATION_S=0!DURATION_S!"
if !DURATION_CS! lss 10 set "DURATION_CS=0!DURATION_CS!"

set "DURATION=!DURATION_H!:!DURATION_M!:!DURATION_S!.!DURATION_CS!"
goto :eof

:error
echo.
echo ========================================
echo BUILD FAILED!
echo ========================================
echo Check the error messages above for details
echo.
echo Common solutions:
echo   - Ensure all dependencies are installed
echo   - Try running 'build-desktop.bat clean' to force a clean build
echo   - Check your Node.js and Go installations
echo   - Verify network connectivity for npm packages
echo.
pause
exit /b 1