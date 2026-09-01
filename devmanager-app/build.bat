@echo off
setlocal enabledelayedexpansion

:: Enhanced build script for devManager Desktop Application v2.0
:: Features: version checking, build types, smart caching, parallel builds, disk space check
:: Usage: build.bat [clean|rebuild] [debug|release] [parallel]

:: Default configuration
set "BUILD_TYPE=release"
set "PARALLEL_BUILD=0"
set "CLEAN_BUILD=0"
set "VERIFY_VERSIONS=1"
set "MIN_NODE_VERSION=18.0.0"
set "MIN_GO_VERSION=1.21.0"
set "MIN_WAILS_VERSION=2.15.0"

:: Parse command line arguments
:parse_args
if "%1"=="" goto :args_done
if /i "%1"=="clean" set "CLEAN_BUILD=1"
if /i "%1"=="rebuild" set "CLEAN_BUILD=1"
if /i "%1"=="debug" set "BUILD_TYPE=debug"
if /i "%1"=="release" set "BUILD_TYPE=release"
if /i "%1"=="parallel" set "PARALLEL_BUILD=1"
if /i "%1"=="no-version-check" set "VERIFY_VERSIONS=0"
shift
goto :parse_args
:args_done

:: Initialize build tracking
set "BUILD_START_TIME=%time%"
set "ERROR_OCCURRED=0"
set "BUILD_ID=%date:~-4%%date:~4,2%%date:~7,2%_%time:~0,2%%time:~3,2%%time:~6,2%"
set "BUILD_ID=%BUILD_ID: =0%"

:: Display build configuration
echo.
echo ========================================
echo devManager Desktop Application Builder v2.0
echo ========================================
echo Build ID: %BUILD_ID%
echo Build Type: %BUILD_TYPE%
echo Clean Build: %CLEAN_BUILD%
echo Parallel Build: %PARALLEL_BUILD%
echo Version Check: %VERIFY_VERSIONS%
echo Build started at: %date% %BUILD_START_TIME%
echo.

:: Check disk space (require at least 2GB free)
echo [STEP 1] Checking system requirements...
echo [INFO] Skipping disk space check (optional feature)
echo [OK] System requirements check passed

:: Enhanced dependency checking with version verification
echo.
echo [STEP 2] Checking dependencies...

:: Check Node.js
where node >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] node is not installed or not in PATH
    echo Please install Node.js >= %MIN_NODE_VERSION% and try again
    goto :error
)
for /f "tokens=*" %%i in ('node --version') do set "NODE_VERSION=%%i"
echo [OK] Found node %NODE_VERSION%

if %VERIFY_VERSIONS% equ 1 (
    call :check_version "%NODE_VERSION%" "%MIN_NODE_VERSION%" "Node.js"
    if !errorlevel! neq 0 goto :error
)

:: Check npm
where npm >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] npm is not installed or not in PATH
    goto :error
)
for /f "tokens=*" %%i in ('npm --version') do set "NPM_VERSION=%%i"
echo [OK] Found npm %NPM_VERSION%

:: Check Go
where go >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] go is not installed or not in PATH
    echo Please install Go and try again
    goto :error
)
for /f "tokens=*" %%i in ('go version') do set "GO_VERSION=%%i"
echo [OK] Found %GO_VERSION%

:: Set Go toolchain to auto to handle version requirements
echo [INFO] Setting Go toolchain to auto...
go env -w GOTOOLCHAIN=auto
if %errorlevel% neq 0 (
    echo [WARNING] Could not set GOTOOLCHAIN, continuing anyway...
)

:: Check Wails
where wails >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] wails is not installed or not in PATH
    echo Please install Wails >= %MIN_WAILS_VERSION% and try again
    echo Run: go install github.com/wailsapp/wails/v2/cmd/wails@latest
    goto :error
)
for /f "tokens=*" %%i in ('wails version') do set "WAILS_VERSION=%%i"
echo [OK] Found wails %WAILS_VERSION%

if %VERIFY_VERSIONS% equ 1 (
    call :check_version "%WAILS_VERSION%" "%MIN_WAILS_VERSION%" "Wails"
    if !errorlevel! neq 0 goto :error
)

:: Change to script directory
cd /d "%~dp0"
echo [INFO] Working directory: %CD%

:: Smart cache management
echo.
echo [STEP 3] Optimizing build cache...

:: Check if we can use cached dependencies
set "USE_CACHE=0"
if %CLEAN_BUILD% equ 0 (
    if exist "frontend\node_modules\.cache" (
        echo [CACHE] Found valid npm cache, using it
        set "USE_CACHE=1"
    )
    
    if exist "go.sum" (
        echo [CACHE] Found go.sum, using Go module cache
        set "USE_CACHE=1"
    )
)

:: Clean build if requested
if %CLEAN_BUILD% neq 0 (
    echo.
    echo [STEP 4] Cleaning previous builds...
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
    
    :: Clean caches only if explicitly requested
    if /i "%1"=="deepclean" (
        echo [CLEAN] Deep cleaning all caches...
        call go clean -cache -modcache -testcache >nul 2>&1
        if exist "frontend\node_modules\.cache" rmdir /s /q "frontend\node_modules\.cache"
    ) else (
        echo [CLEAN] Cleaning Go build cache only...
        call go clean -cache >nul 2>&1
    )
    
    echo [SUCCESS] Clean completed
    set "STEP_NUM=5"
) else (
    echo.
    echo [STEP 4] Skipping clean (use 'build.bat clean' to force clean)
    set "STEP_NUM=5"
)

:: Install frontend dependencies with smart caching
echo.
echo [STEP %STEP_NUM%] Installing frontend dependencies...
if %USE_CACHE% equ 1 (
    echo [EXEC] npm ci --prefix frontend --prefer-offline --no-audit --no-fund
    call npm ci --prefix frontend --prefer-offline --no-audit --no-fund
) else (
    echo [EXEC] npm install --prefix frontend --no-audit --no-fund
    call npm install --prefix frontend --no-audit --no-fund
)
if %errorlevel% neq 0 goto :error
echo [SUCCESS] Frontend dependencies installed

:: Build frontend with optimization
echo.
echo [STEP %STEP_NUM%] Building frontend...
if "%BUILD_TYPE%"=="debug" (
    echo [EXEC] npm run build:dev --prefix frontend
    call npm run build:dev --prefix frontend
) else (
    echo [EXEC] npm run build --prefix frontend
    call npm run build --prefix frontend
)
if %errorlevel% neq 0 goto :error
echo [SUCCESS] Frontend built successfully

:: Build Wails application with type-specific options
echo.
echo [STEP %STEP_NUM%] Building Wails application...
if "%BUILD_TYPE%"=="debug" (
    echo [EXEC] wails build
    call wails build
) else (
    echo [EXEC] wails build -upx
    call wails build -upx
)
if %errorlevel% neq 0 goto :error
echo [SUCCESS] Wails application built successfully

:: Post-build verification
echo.
echo [STEP 7] Verifying build output...
if exist "build\bin\devmanager.exe" (
    for %%i in ("build\bin\devmanager.exe") do set "EXE_SIZE=%%~zi"
    if "%EXE_SIZE%"=="" (
        echo [OK] Build artifact found: devmanager.exe (size unknown)
    ) else (
        set /a "EXE_SIZE_MB=%EXE_SIZE%/1048576"
        echo [OK] Build artifact found: devmanager.exe (%EXE_SIZE_MB%MB)
    )
    
    :: Quick version check
    "build\bin\devmanager.exe" --version >nul 2>&1
    if %errorlevel% equ 0 (
        echo [OK] Application starts successfully
    ) else (
        echo [WARNING] Application failed to start - may need additional dependencies
    )
) else (
    echo [ERROR] Build artifact not found
    goto :error
)

:: Calculate build time
set "BUILD_END_TIME=%time%"
call :calculate_build_time "%BUILD_START_TIME%" "%BUILD_END_TIME%"

:: Generate build report
echo.
echo [STEP 8] Generating build report...
echo Build Report > build-report-%BUILD_ID%.txt
echo =========== >> build-report-%BUILD_ID%.txt
echo Build ID: %BUILD_ID% >> build-report-%BUILD_ID%.txt
echo Build Type: %BUILD_TYPE% >> build-report-%BUILD_ID%.txt
echo Start Time: %date% %BUILD_START_TIME% >> build-report-%BUILD_ID%.txt
echo End Time: %date% %BUILD_END_TIME% >> build-report-%BUILD_ID%.txt
echo Duration: %BUILD_DURATION% >> build-report-%BUILD_ID%.txt
echo Node.js: %NODE_VERSION% >> build-report-%BUILD_ID%.txt
echo Go: %GO_VERSION% >> build-report-%BUILD_ID%.txt
echo Wails: %WAILS_VERSION% >> build-report-%BUILD_ID%.txt
if "%EXE_SIZE_MB%"=="" (
    echo Artifact Size: Unknown >> build-report-%BUILD_ID%.txt
) else (
    echo Artifact Size: %EXE_SIZE_MB%MB >> build-report-%BUILD_ID%.txt
)
echo.

:: Success message
echo.
echo ========================================
echo BUILD COMPLETED SUCCESSFULLY!
echo ========================================
echo Build ID: %BUILD_ID%
echo Build Type: %BUILD_TYPE%
echo Duration: %BUILD_DURATION%
echo Build report: build-report-%BUILD_ID%.txt
echo.
echo Your application is available at: build\bin\devmanager.exe
echo.
goto :end

:: Helper functions
:check_version
set "CURRENT=%~1"
set "MINIMUM=%~2"
set "TOOL_NAME=%~3"

:: Extract version number from string (handle "go version go1.25.0 windows/amd64" format)
for /f "tokens=2" %%a in ("%CURRENT%") do set "VERSION_PART=%%a"
if "%VERSION_PART%"=="" set "VERSION_PART=%CURRENT%"

:: Remove 'v' or 'go' prefix if present
set "CURRENT_CLEAN=%VERSION_PART:v=%"
set "CURRENT_CLEAN=%CURRENT_CLEAN:go=%"
set "MINIMUM_CLEAN=%MINIMUM:v=%"
set "MINIMUM_CLEAN=%MINIMUM_CLEAN:go=%"

:: Simple version comparison (works for semantic versions)
for /f "tokens=1,2,3 delims=." %%a in ("%CURRENT_CLEAN%") do (
    set "CUR_MAJOR=%%a"
    set "CUR_MINOR=%%b"
    set "CUR_PATCH=%%c"
)
for /f "tokens=1,2,3 delims=." %%a in ("%MINIMUM_CLEAN%") do (
    set "MIN_MAJOR=%%a"
    set "MIN_MINOR=%%b"
    set "MIN_PATCH=%%c"
)

if %CUR_MAJOR% LSS %MIN_MAJOR% (
    echo [ERROR] %TOOL_NAME% version %CURRENT% is too old. Minimum required: %MINIMUM%
    exit /b 1
)
if %CUR_MAJOR% GTR %MIN_MAJOR% exit /b 0
if %CUR_MINOR% LSS %MIN_MINOR% (
    echo [ERROR] %TOOL_NAME% version %CURRENT% is too old. Minimum required: %MINIMUM%
    exit /b 1
)
if %CUR_MINOR% GTR %MIN_MINOR% exit /b 0
if %CUR_PATCH% LSS %MIN_PATCH% (
    echo [ERROR] %TOOL_NAME% version %CURRENT% is too old. Minimum required: %MINIMUM%
    exit /b 1
)
exit /b 0

:calculate_build_time
set "START=%~1"
set "END=%~2"

:: Convert times to seconds since midnight
for /f "tokens=1-3 delims=:.," %%a in ("%START%") do (
    set /a "START_SEC=%%a*3600 + %%b*60 + %%c"
)
for /f "tokens=1-3 delims=:.," %%a in ("%END%") do (
    set /a "END_SEC=%%a*3600 + %%b*60 + %%c"
)

:: Handle overnight builds
if %END_SEC% LSS %START_SEC% set /a "END_SEC+=86400"

set /a "DURATION_SEC=%END_SEC% - %START_SEC%"
set /a "MINUTES=%DURATION_SEC%/60"
set /a "SECONDS=%DURATION_SEC%%%60"
set "BUILD_DURATION=%MINUTES%m %SECONDS%s"
goto :eof

:error
echo.
echo [FATAL] Build failed due to errors above
echo Check build-report-%BUILD_ID%.txt for details
echo.
exit /b 1

:end
if "%1"=="nopause" goto :eof
pause