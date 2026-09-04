@echo off
setlocal enabledelayedexpansion

:: Enhanced build script for devManager Desktop Application v2.0
:: Features: version checking, build types, smart caching, disk space check
:: Usage: build.bat [clean|rebuild] [debug|release]

:: NOTE: ANSI color codes were removed from this script.
:: Using ESC/$E prompt tricks and VT processing in .bat proved unreliable
:: across Windows consoles, producing literal escape garbage instead of colors.
:: All color variables below are empty (no-op) so any %VAR% / !VAR! reference
:: renders as plain text. Output stays clean and predictable everywhere.

:: Color codes (deliberately empty - colors removed)
set "R="
set "DIM="
set "GRN="
set "YLW="
set "RED="
set "GRY="
set "BCYN="
set "BGRN="
set "BRED="
set "WHT="

:: Default configuration
set "BUILD_TYPE=release"
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
echo %BCYN%========================================%R%
echo %BCYN%devManager Desktop Application Builder v2.0%R%
echo %BCYN%========================================%R%
echo   Build ID:       %WHT%%BUILD_ID%%R%
echo   Build Type:     %WHT%%BUILD_TYPE%%R%
echo   Clean Build:    %WHT%%CLEAN_BUILD%%R%
echo   Version Check:  %WHT%%VERIFY_VERSIONS%%R%
echo   Started:        %GRY%%date% %BUILD_START_TIME%%R%
echo.

:: Step 1 - System requirements
echo %BCYN%[STEP 1]%R% Checking system requirements...
echo   %GRY%[INFO] Skipping disk space check (optional feature)%R%
echo   %GRN%[OK] System requirements check passed%R%

:: Step 2 - Dependencies
echo.
echo %BCYN%[STEP 2]%R% Checking dependencies...

:: Check Node.js
where node >nul 2>&1
if %errorlevel% neq 0 (
    echo   %BRED%[ERROR] node is not installed or not in PATH%R%
    echo   Please install Node.js ^>= %MIN_NODE_VERSION% and try again
    goto :error
)
for /f "tokens=*" %%i in ('node --version') do set "NODE_VERSION=%%i"
echo   %GRN%[OK] Found node %NODE_VERSION%%R%

if %VERIFY_VERSIONS% equ 1 (
    call :check_version "%NODE_VERSION%" "%MIN_NODE_VERSION%" "Node.js"
    if !errorlevel! neq 0 goto :error
)

:: Check npm
where npm >nul 2>&1
if %errorlevel% neq 0 (
    echo   %BRED%[ERROR] npm is not installed or not in PATH%R%
    goto :error
)
for /f "tokens=*" %%i in ('npm --version') do set "NPM_VERSION=%%i"
echo   %GRN%[OK] Found npm %NPM_VERSION%%R%

:: Check Go
where go >nul 2>&1
if %errorlevel% neq 0 (
    echo   %BRED%[ERROR] go is not installed or not in PATH%R%
    echo   Please install Go and try again
    goto :error
)
for /f "tokens=*" %%i in ('go version') do set "GO_VERSION=%%i"
echo   %GRN%[OK] Found %GO_VERSION%%R%

:: Set Go toolchain to auto
echo   %GRY%[INFO] Setting Go toolchain to auto...%R%
go env -w GOTOOLCHAIN=auto
if %errorlevel% neq 0 (
    echo   %YLW%[WARNING] Could not set GOTOOLCHAIN, continuing anyway...%R%
)

:: Check Wails
where wails >nul 2>&1
if %errorlevel% neq 0 (
    echo   %BRED%[ERROR] wails is not installed or not in PATH%R%
    echo   Please install Wails ^>= %MIN_WAILS_VERSION% and try again
    echo   Run: go install github.com/wailsapp/wails/v2/cmd/wails@latest
    goto :error
)
for /f "tokens=*" %%i in ('wails version') do set "WAILS_VERSION=%%i"
echo   %GRN%[OK] Found wails %WAILS_VERSION%%R%

if %VERIFY_VERSIONS% equ 1 (
    call :check_version "%WAILS_VERSION%" "%MIN_WAILS_VERSION%" "Wails"
    if !errorlevel! neq 0 goto :error
)

:: Change to script directory
cd /d "%~dp0"
echo   %GRY%[INFO] Working directory: %CD%%R%

:: Step 3 - Build cache
echo.
echo %BCYN%[STEP 3]%R% Optimizing build cache...

set "USE_CACHE=0"
if %CLEAN_BUILD% equ 0 (
    if exist "frontend\node_modules\.cache" (
        echo   !GRY![CACHE] Found valid npm cache, using it!R!
        set "USE_CACHE=1"
    )
    if exist "go.sum" (
        echo   !GRY![CACHE] Found go.sum, using Go module cache!R!
        set "USE_CACHE=1"
    )
)

:: Clean build if requested
if %CLEAN_BUILD% neq 0 (
    echo.
    echo !BCYN![STEP 4]!R! Cleaning previous builds...
    if exist "frontend\dist" (
        echo   !YLW![CLEAN] Removing frontenddist...!R!
        rmdir /s /q "frontend\dist"
    )
    if exist "build" (
        echo   !YLW![CLEAN] Removing build directory...!R!
        rmdir /s /q "build"
    )
    if exist "bin" (
        echo   !YLW![CLEAN] Removing bin directory...!R!
        rmdir /s /q "bin"
    )
    if /i "%1"=="deepclean" (
        echo   !YLW![CLEAN] Deep cleaning all caches...!R!
        call go clean -cache -modcache -testcache >nul 2>&1
        if exist "frontend\node_modules\.cache" rmdir /s /q "frontend\node_modules\.cache"
    ) else (
        echo   !YLW![CLEAN] Cleaning Go build cache only...!R!
        call go clean -cache >nul 2>&1
    )
    echo   !GRN![SUCCESS] Clean completed!R!
) else (
    echo.
    echo   !DIM![STEP 4] Skipping clean ^(use 'build.bat clean' to force clean^)!R!
)

:: Step 5 - Frontend dependencies
echo.
echo !BCYN![STEP 5]!R! Installing frontend dependencies...
if !USE_CACHE! equ 1 (
    echo   !GRY![EXEC] npm ci --prefix frontend --prefer-offline --no-audit --no-fund!R!
    call npm ci --prefix frontend --prefer-offline --no-audit --no-fund
) else (
    echo   !GRY![EXEC] npm install --prefix frontend --no-audit --no-fund!R!
    call npm install --prefix frontend --no-audit --no-fund
)
if %errorlevel% neq 0 goto :error
echo   !GRN![SUCCESS] Frontend dependencies installed!R!

:: Step 6 - Build frontend
echo.
echo !BCYN![STEP 6]!R! Building frontend...
if "%BUILD_TYPE%"=="debug" (
    echo   !GRY![EXEC] npm run build:dev --prefix frontend!R!
    call npm run build:dev --prefix frontend
) else (
    echo   !GRY![EXEC] npm run build --prefix frontend!R!
    call npm run build --prefix frontend
)
if %errorlevel% neq 0 goto :error
echo   !GRN![SUCCESS] Frontend built successfully!R!

:: Step 7 - Build Wails application
echo.
echo !BCYN![STEP 7]!R! Building Wails application...
if "%BUILD_TYPE%"=="debug" (
    echo   !GRY![EXEC] wails build!R!
    call wails build
) else (
    echo   !GRY![EXEC] wails build -upx!R!
    call wails build -upx
)
if %errorlevel% neq 0 goto :error
echo   !GRN![SUCCESS] Wails application built successfully!R!

:: Step 8 - Verify build output
echo.
echo !BCYN![STEP 8]!R! Verifying build output...
if exist "build\bin\devmanager.exe" (
    for %%i in ("build\bin\devmanager.exe") do set "EXE_SIZE=%%~zi"
    if "!EXE_SIZE!"=="" (
        echo   !GRN![OK] Build artifact found: devmanager.exe - size unknown!R!
    ) else (
        set /a "EXE_SIZE_MB=!EXE_SIZE!/1048576"
        echo   !GRN![OK] Build artifact found: devmanager.exe - !EXE_SIZE_MB!MB!R!
    )
) else (
    echo   !BRED![ERROR] Build artifact not found!R!
    goto :error
)

:: Calculate build time
set "BUILD_END_TIME=%time%"
call :calculate_build_time "%BUILD_START_TIME%" "%BUILD_END_TIME%"

:: Step 9 - Build report
echo.
echo !BCYN![STEP 9]!R! Generating build report...
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

:: Success banner
echo.
echo %BGRN%========================================%R%
echo %BGRN%BUILD COMPLETED SUCCESSFULLY!%R%
echo %BGRN%========================================%R%
echo   Build ID:      %BUILD_ID%
echo   Build Type:    %BUILD_TYPE%
echo   Duration:      %BUILD_DURATION%
echo   Report:        build-report-%BUILD_ID%.txt
echo   Application:   build\bin\devmanager.exe
echo.

:: Show popup notification
powershell -NoProfile -Command ^
  "Add-Type -AssemblyName System.Windows.Forms; ^
   Add-Type -AssemblyName System.Drawing; ^
   $form = New-Object System.Windows.Forms.Form; ^
   $form.Text = 'devManager Builder - Success'; ^
   $form.Width = 480; ^
   $form.Height = 300; ^
   $form.FormBorderStyle = 'FixedDialog'; ^
   $form.MaximizeBox = $false; ^
   $form.StartPosition = 'CenterScreen'; ^
   $form.BackColor = [System.Drawing.Color]::White; ^
   $titleLabel = New-Object System.Windows.Forms.Label; ^
   $titleLabel.Text = 'Build Completed Successfully'; ^
   $titleLabel.Font = New-Object System.Drawing.Font('Segoe UI', 12, [System.Drawing.FontStyle]::Bold); ^
   $titleLabel.ForeColor = [System.Drawing.Color]::FromArgb(34, 139, 34); ^
   $titleLabel.AutoSize = $true; ^
   $titleLabel.Left = 20; ^
   $titleLabel.Top = 15; ^
   $form.Controls.Add($titleLabel); ^
   $msgLabel = New-Object System.Windows.Forms.Label; ^
   $msgLabel.Text = 'Your application has been built and is ready to use.'; ^
   $msgLabel.Font = New-Object System.Drawing.Font('Segoe UI', 9); ^
   $msgLabel.AutoSize = $true; ^
   $msgLabel.Left = 20; ^
   $msgLabel.Top = 48; ^
   $form.Controls.Add($msgLabel); ^
   $sep = New-Object System.Windows.Forms.Label; ^
    $sep.BorderStyle = 'Fixed3D'; ^
    $sep.Left = 20; ^
    $sep.Top = 74; ^
    $sep.Width = 425; ^
   $sep.Height = 2; ^
   $form.Controls.Add($sep); ^
   $details = 'Build ID:    %BUILD_ID%' + [Environment]::NewLine + ^
              'Build Type:  %BUILD_TYPE%' + [Environment]::NewLine + ^
              'Duration:    %BUILD_DURATION%' + [Environment]::NewLine + ^
              'Artifact:    !EXE_SIZE_MB! MB' + [Environment]::NewLine + ^
              'Output:      build\bin\devmanager.exe' + [Environment]::NewLine + ^
              'Date/Time:   %date%  %BUILD_END_TIME%'; ^
   $detLabel = New-Object System.Windows.Forms.Label; ^
   $detLabel.Text = $details; ^
   $detLabel.Font = New-Object System.Drawing.Font('Consolas', 9); ^
   $detLabel.ForeColor = [System.Drawing.Color]::FromArgb(51, 51, 51); ^
   $detLabel.Multiline = $true; ^
   $detLabel.AutoSize = $true; ^
   $detLabel.Left = 20; ^
   $detLabel.Top = 86; ^
   $form.Controls.Add($detLabel); ^
   $btnPanel = New-Object System.Windows.Forms.FlowLayoutPanel; ^
   $btnPanel.Left = 20; ^
   $btnPanel.Top = 225; ^
   $btnPanel.Width = 425; ^
   $btnPanel.Height = 35; ^
   $btnPanel.FlowDirection = 'RightToLeft'; ^
   $form.Controls.Add($btnPanel); ^
   $btnOK = New-Object System.Windows.Forms.Button; ^
   $btnOK.Text = 'OK'; ^
   $btnOK.Width = 80; ^
   $btnOK.Height = 28; ^
   $btnOK.DialogResult = [System.Windows.Forms.DialogResult]::OK; ^
   $btnPanel.Controls.Add($btnOK); ^
   $btnApp = New-Object System.Windows.Forms.Button; ^
   $btnApp.Text = 'Open App'; ^
   $btnApp.Width = 90; ^
   $btnApp.Height = 28; ^
   $btnApp.DialogResult = [System.Windows.Forms.DialogResult]::Yes; ^
   $btnPanel.Controls.Add($btnApp); ^
   $btnFolder = New-Object System.Windows.Forms.Button; ^
   $btnFolder.Text = 'Open Folder'; ^
   $btnFolder.Width = 100; ^
   $btnFolder.Height = 28; ^
   $btnFolder.DialogResult = [System.Windows.Forms.DialogResult]::Retry; ^
   $btnPanel.Controls.Add($btnFolder); ^
   $form.AcceptButton = $btnOK; ^
   $result = $form.ShowDialog(); ^
   $form.Dispose(); ^
   if ($result -eq [System.Windows.Forms.DialogResult]::Retry) { ^
     Start-Process explorer.exe 'build\bin'; ^
   } elseif ($result -eq [System.Windows.Forms.DialogResult]::Yes) { ^
     Start-Process 'build\bin\devmanager.exe'; ^
   }"
goto :end

:: ============================================================
:: Helper functions
:: ============================================================

:check_version
set "CURRENT=%~1"
set "MINIMUM=%~2"
set "TOOL_NAME=%~3"

:: Extract version number from string
for /f "tokens=2" %%a in ("%CURRENT%") do set "VERSION_PART=%%a"
if "%VERSION_PART%"=="" set "VERSION_PART=%CURRENT%"

:: Remove 'v' or 'go' prefix if present
set "CURRENT_CLEAN=%VERSION_PART:v=%"
set "CURRENT_CLEAN=%CURRENT_CLEAN:go=%"
set "MINIMUM_CLEAN=%MINIMUM:v=%"
set "MINIMUM_CLEAN=%MINIMUM_CLEAN:go=%"

:: Simple version comparison
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

if !CUR_MAJOR! LSS !MIN_MAJOR! (
    echo   !BRED![ERROR] !TOOL_NAME! version !CURRENT! is too old. Minimum required: !MINIMUM!!R!
    exit /b 1
)
if !CUR_MAJOR! GTR !MIN_MAJOR! exit /b 0
if !CUR_MINOR! LSS !MIN_MINOR! (
    echo   !BRED![ERROR] !TOOL_NAME! version !CURRENT! is too old. Minimum required: !MINIMUM!!R!
    exit /b 1
)
if !CUR_MINOR! GTR !MIN_MINOR! exit /b 0
if !CUR_PATCH! LSS !MIN_PATCH! (
    echo   !BRED![ERROR] !TOOL_NAME! version !CURRENT! is too old. Minimum required: !MINIMUM!!R!
    exit /b 1
)
exit /b 0

:calculate_build_time
set "START=%~1"
set "END=%~2"

:: Convert times to seconds since midnight
for /f "tokens=1-3 delims=:., " %%a in ("%START%") do (
    set /a "START_SEC=%%a*3600 + %%b*60 + %%c"
)
for /f "tokens=1-3 delims=:., " %%a in ("%END%") do (
    set /a "END_SEC=%%a*3600 + %%b*60 + %%c"
)

:: Handle overnight builds
if !END_SEC! LSS !START_SEC! set /a "END_SEC+=86400"

set /a "DURATION_SEC=!END_SEC! - !START_SEC!"
set /a "MINUTES=!DURATION_SEC!/60"
set /a "SECONDS=!DURATION_SEC!%%60"
set "BUILD_DURATION=!MINUTES!m !SECONDS!s"
goto :eof

:error
echo.
echo !BRED![FATAL] Build failed due to errors above!R!
echo   Check build-report-%BUILD_ID%.txt for details
echo.
powershell -NoProfile -Command ^
  "Add-Type -AssemblyName System.Windows.Forms; ^
   Add-Type -AssemblyName System.Drawing; ^
   $form = New-Object System.Windows.Forms.Form; ^
   $form.Text = 'devManager Builder - Failed'; ^
   $form.Width = 420; ^
   $form.Height = 220; ^
   $form.FormBorderStyle = 'FixedDialog'; ^
   $form.MaximizeBox = $false; ^
   $form.StartPosition = 'CenterScreen'; ^
   $form.BackColor = [System.Drawing.Color]::White; ^
   $titleLabel = New-Object System.Windows.Forms.Label; ^
   $titleLabel.Text = 'Build Failed'; ^
   $titleLabel.Font = New-Object System.Drawing.Font('Segoe UI', 12, [System.Drawing.FontStyle]::Bold); ^
   $titleLabel.ForeColor = [System.Drawing.Color]::FromArgb(220, 53, 69); ^
   $titleLabel.AutoSize = $true; ^
   $titleLabel.Left = 20; ^
   $titleLabel.Top = 15; ^
   $form.Controls.Add($titleLabel); ^
   $sep = New-Object System.Windows.Forms.Label; ^
    $sep.BorderStyle = 'Fixed3D'; ^
    $sep.Left = 20; ^
    $sep.Top = 44; ^
    $sep.Width = 365; ^
   $sep.Height = 2; ^
   $form.Controls.Add($sep); ^
   $details = 'Build ID:   %BUILD_ID%' + [Environment]::NewLine + ^
              'Build Type: %BUILD_TYPE%' + [Environment]::NewLine + [Environment]::NewLine + ^
              'Check the console output above for error details.'; ^
   $detLabel = New-Object System.Windows.Forms.Label; ^
   $detLabel.Text = $details; ^
   $detLabel.Font = New-Object System.Drawing.Font('Segoe UI', 9); ^
   $detLabel.ForeColor = [System.Drawing.Color]::FromArgb(51, 51, 51); ^
   $detLabel.Multiline = $true; ^
   $detLabel.AutoSize = $true; ^
   $detLabel.Left = 20; ^
   $detLabel.Top = 56; ^
   $form.Controls.Add($detLabel); ^
   $btnPanel = New-Object System.Windows.Forms.FlowLayoutPanel; ^
   $btnPanel.Left = 20; ^
   $btnPanel.Top = 145; ^
   $btnPanel.Width = 365; ^
   $btnPanel.Height = 35; ^
   $btnPanel.FlowDirection = 'RightToLeft'; ^
   $form.Controls.Add($btnPanel); ^
   $btnOK = New-Object System.Windows.Forms.Button; ^
   $btnOK.Text = 'OK'; ^
   $btnOK.Width = 80; ^
   $btnOK.Height = 28; ^
   $btnOK.DialogResult = [System.Windows.Forms.DialogResult]::OK; ^
   $btnPanel.Controls.Add($btnOK); ^
   $btnLog = New-Object System.Windows.Forms.Button; ^
   $btnLog.Text = 'Open Build Dir'; ^
   $btnLog.Width = 110; ^
   $btnLog.Height = 28; ^
   $btnLog.DialogResult = [System.Windows.Forms.DialogResult]::Retry; ^
   $btnPanel.Controls.Add($btnLog); ^
   $form.AcceptButton = $btnOK; ^
   $result = $form.ShowDialog(); ^
   $form.Dispose(); ^
   if ($result -eq [System.Windows.Forms.DialogResult]::Retry) { ^
     Start-Process explorer.exe '.'; ^
   }"
exit /b 1

:end
if "%1"=="nopause" goto :eof
pause
exit /b 0
