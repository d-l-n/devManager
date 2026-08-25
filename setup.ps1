# devManager Setup Script
# This script should only be called from run.vbs
param(
    [string]$Mode = "setup"
)

# Security check - only allow execution from VBS
$caller = (Get-WmiObject Win32_Process -Filter "ProcessId=$PID").ParentProcessId
$callerName = (Get-Process -Id $caller -ErrorAction SilentlyContinue).ProcessName

if ($callerName -ne "cscript" -and $callerName -ne "wscript") {
    Write-Host "Error: This script can only be executed from VBS"
    exit 1
}

# Set encoding for Spanish characters
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$Host.UI.RawUI.WindowTitle = "devManager v1.1 - Configuración"

# Configuration
$APP_NAME = "devManager"
$APP_VERSION = "1.1"
$MIN_PYTHON_VERSION = "3.8"
$DEFAULT_PORT = "5000"

# Colors
$GREEN = "Green"
$RED = "Red"
$YELLOW = "Yellow"
$BLUE = "Blue"
$CYAN = "Cyan"

function Write-ColorOutput($ForegroundColor) {
    $fc = $host.UI.RawUI.ForegroundColor
    $host.UI.RawUI.ForegroundColor = $ForegroundColor
    if ($args) {
        Write-Output $args
    }
    $host.UI.RawUI.ForegroundColor = $fc
}

function Write-OK($message) {
    Write-ColorOutput $GREEN "[OK] $message"
}

function Write-Error($message) {
    Write-ColorOutput $RED "[ERROR] $message"
}

function Write-Warn($message) {
    Write-ColorOutput $YELLOW "[WARN] $message"
}

function Write-Info($message) {
    Write-ColorOutput $BLUE "[INFO] $message"
}

# Main setup logic
try {
    Write-Info "Iniciando configuración de $APP_NAME v$APP_VERSION..."
    
    # Check Python
    $pythonCmd = Get-Command python -ErrorAction SilentlyContinue
    if (-not $pythonCmd) {
        Write-Error "Python no encontrado. Por favor instale Python $MIN_PYTHON_VERSION o superior"
        exit 1
    }
    
    $pythonVersion = python --version 2>&1
    Write-OK "Python encontrado: $pythonVersion"
    
    # Create virtual environment if not exists
    if (-not (Test-Path ".venv")) {
        Write-Info "Creando entorno virtual..."
        python -m venv .venv
        if ($LASTEXITCODE -ne 0) {
            Write-Error "No se pudo crear el entorno virtual"
            exit 1
        }
        Write-OK "Entorno virtual creado"
    } else {
        Write-OK "Entorno virtual ya existe"
    }
    
    # Activate virtual environment and install dependencies
    Write-Info "Instalando dependencias..."
    & .\.venv\Scripts\Activate.ps1
    python -m pip install --upgrade pip
    python -m pip install -r requirements.txt
    
    if ($LASTEXITCODE -eq 0) {
        Write-OK "Dependencias instaladas correctamente"
        Write-OK "¡Configuración completada exitosamente!"
        Write-Info "devManager está listo para usar"
        exit 0
    } else {
        Write-Error "Error al instalar dependencias"
        exit 1
    }
    
} catch {
    Write-Error "Error durante la configuración: $($_.Exception.Message)"
    exit 1
}