# Local Dev Manager

Una aplicación de escritorio nativa para Windows desarrollada con Python y PySide6 para gestionar proyectos de desarrollo locales, sus servidores de desarrollo y la ejecución de tests con Playwright desde una única interfaz gráfica.

---

## Características

- **Gestión de proyectos:** Añade, edita y elimina proyectos locales mediante la interfaz gráfica o directamente en `projects.json`.
- **Control de servidores:** Inicia, detiene y reinicia servidores (`npm run dev`, `vite`, `yarn`, `pnpm`, etc.) con monitoreo automático de estado y puerto.
- **Detección de disponibilidad:** Sondeo de puerto con timeout configurable para asegurar que el servidor esté activo antes de lanzar pruebas.
- **Integración con Playwright:**
  - Ejecución de tests estándar en segundo plano.
  - Modo UI interactivo (`--ui`) gestionado como proceso persistente.
  - Modo Debug (`--debug`).
  - Visualización del reporte HTML de Playwright (`show-report`).
  - Auto-inicio de servidor: Si el servidor no está corriendo al lanzar los tests, se levanta y se espera su disponibilidad antes de ejecutar Playwright.
- **Acciones globales:** Iniciar y detener todos los servidores habilitados en lote.
- **Tab Git:** Rama, estado dirty, ahead/behind, último commit y acciones Pull/Fetch/Stash con salida en vivo en Logs.
- **Tab Monitor:** Estado de puertos configurados (libre/ocupado/proceso ajeno con Kill) y CPU/RAM por árbol de proceso de cada servidor (auto-refresh 3s, configurable en Settings). Ventana global independiente de proyectos (`Ctrl+Alt+M` o botón del sidebar).
- **Tab Evidence:** Galería de screenshots, videos y traces de `test-results/` con preview integrado, apertura externa y visor de traces.
- **Toasts in-app:** Notificaciones visuales apilables cuando la ventana está visible; bandeja del sistema cuando está minimizada.
- **Uptime:** Tiempo encendido de cada servidor en el panel Server.
- **Filtros:** Sidebar con chips All/Running/Stopped combinables con búsqueda, y logs con modo "Errors only".
- **App Log global:** Ventana independiente (`Ctrl+Alt+L` o botón del sidebar) con el log de la aplicación, separada del contexto de proyecto.
- **Sidebar compacto:** Acciones Add/Edit/Remove como iconos en el header (`Ctrl+N`, `Ctrl+E`, tecla `Supr`) y doble clic para editar.
- **Settings (Ctrl+,):** Preferencias persistentes de la aplicación (polling de recursos, toasts in-app).
- **Temas Light / Dark / OLED:** Ciclo con `Ctrl+Shift+T` o desde el menú View; el modo OLED usa negro puro (`#000000`) con textos y acentos calibrados para mantener contraste WCAG AA (≥4.5:1).
- **Iconografía Reicon:** Integración completa de iconos vectoriales SVG limpios basados en la librería open-source [Reicon](https://github.com/dqev/reicon) (trazo 1.5px, estilo outline y filled nativo para PySide6).
- **Consola de logs en tiempo real:** Captura `stdout` y `stderr` con marcas de tiempo `[HH:MM:SS]`, scroll automático y diferenciación de errores.
- **Seguridad en Windows:** Terminación segura de árboles de procesos externos mediante PID (`taskkill /T /F /PID <pid>`), sin afectar procesos de terceros.

---

## Requisitos

- **Sistema Operativo:** Windows 10 o Windows 11
- **Python:** 3.11 o superior
- **Node.js y npm / pnpm / yarn:** Disponibles en el `PATH` del sistema para proyectos web.

---

## Instalación y Ejecución

La forma recomendada de ejecutar la aplicación:

- **Para iniciar en el día a día (sin ventana negra de consola):** Haz doble clic en `run.vbs`.
- **Para la primera instalación o actualizar paquetes:** Ejecuta `run.bat`.

El script `run.bat`:
- Detecta la instalación de Python.
- Crea el entorno virtual `.venv` automáticamente si no existe.
- Instala o actualiza las dependencias de `requirements.txt`.
- Inicia `main.py`.

---

## Versión Go/Wails (experimental)

Existe una reescritura nativa del núcleo en **Go + Wails v2** (`desktop-go/`), sin
dependencia de Python ni Node. Fase 1 incluye: CRUD de `projects.json` (comparte el
mismo archivo con la app Python), ciclo de vida de servidores y logs en vivo.

- **Ejecutar:** doble clic en `run-go.vbs`, o directamente `desktop-go\build\bin\devManager.exe`.
- **Compilar:** requiere Go 1.22+ y Wails CLI:
  ```powershell
  cd desktop-go
  wails build
  ```
- **Tests:** `go test ./internal/...` desde `desktop-go/`.

> Estado: paridad parcial con la app Python. Playwright, Git, Monitor, Evidence y
> Settings llegan en fases siguientes (ver `docs/superpowers/plans/`).

---

## Estructura del Proyecto

```text
devManager/
├── main.py
├── requirements.txt
├── README.md
├── projects.json
├── run.bat
├── .gitignore
│
├── app/
│   ├── __init__.py
│   ├── config/
│   │   ├── __init__.py
│   │   └── manager.py          # Carga, guardado y validación de projects.json
│   │   └── settings.py         # Ajustes globales de la app (QSettings)
│   ├── models/
│   │   ├── __init__.py
│   │   └── project.py          # Modelos de datos y Enums de estado
│   ├── process/
│   │   ├── __init__.py
│   │   ├── runner.py           # Encapsulador QProcess para Windows
│   │   └── monitor.py          # psutil: dueños de puertos, kill, CPU/RAM por proceso
│   ├── server/
│   │   ├── __init__.py
│   │   └── manager.py          # Ciclo de vida, uptime y verificación de servidor
│   ├── playwright/
│   │   ├── __init__.py
│   │   └── manager.py          # Gestión de comandos y flujo de Playwright
│   ├── scripts/
│   │   ├── __init__.py
│   │   └── manager.py          # Ejecución de scripts personalizados por proyecto
│   ├── ui/
│   │   ├── __init__.py
│   │   ├── main_window.py      # Ventana principal y docking/layouts
│   │   ├── project_dialog.py   # Modal de creación y edición
│   │   ├── settings_dialog.py  # Modal de ajustes de la app (Ctrl+,)
│   │   ├── system_tray.py      # Bandeja del sistema con acciones rápidas
│   │   ├── theme.py            # Tema claro/oscuro/OLED y tipografías
│   │   ├── icons.py            # Sistema de iconos Reicon (SVG)
│   │   └── widgets/
│   │       ├── __init__.py
│   │       ├── project_list.py     # Sidebar con búsqueda, chips de estado y contexto
│   │       ├── server_panel.py     # Controles Start/Stop/Restart, URL y uptime
│   │       ├── playwright_panel.py # Botones de test, UI, Debug y Report
│   │       ├── scripts_panel.py    # Scripts personalizados por proyecto
│   │       ├── git_panel.py        # Estado git + Pull/Fetch/Stash/Refresh
│   │       ├── monitor_panel.py    # Puertos configurados + recursos por servidor
│   │       ├── evidence_panel.py   # Galería de screenshots/videos/traces
│   │       ├── toast.py            # Notificaciones in-app apilables
│   │       └── log_panel.py        # Visor de logs con timestamp y errors-only
│   └── utils/
│       ├── __init__.py
│       ├── ports.py            # Detección asíncrona de puertos y sockets
│       ├── git.py              # Info de rama/dirty/ahead-behind y run_git
│       ├── evidence.py         # Escáner de artefactos de test-results
│       ├── detection.py        # Detección de scripts y puertos en logs
│       ├── app_logger.py       # Logger global de la app
│       └── paths.py            # Utilidades de rutas en Windows
│
└── tests/
    ├── __init__.py
    ├── test_config.py
    ├── test_settings.py
    ├── test_project.py
    ├── test_ports.py
    ├── test_detection.py
    ├── test_logger.py
    ├── test_scripts.py
    ├── test_server_manager.py
    ├── test_system_monitor.py
    ├── test_evidence.py
    ├── test_git.py
    ├── test_theme.py
    └── test_main_window.py
```

---

## Formato de Configuración (`projects.json`)

Los proyectos se guardan en el archivo `projects.json` en la raíz de la aplicación:

```json
{
  "projects": [
    {
      "name": "MPoints Tracker",
      "path": "D:/Mi Home/Desktop/proyectos/mpoints-tracker",
      "server": {
        "enabled": true,
        "command": "npm run dev",
        "port": 5173,
        "url": "http://localhost:5173",
        "startup_timeout": 15000
      },
      "playwright": {
        "enabled": true,
        "command": "npx playwright test",
        "ui_command": "npx playwright test --ui",
        "debug_command": "npx playwright test --debug",
        "report_command": "npx playwright show-report"
      }
    }
  ]
}
```

---

## Uso de la Interfaz

1. **Añadir Proyecto:** Pulsa en `+ Add` en la barra lateral e ingresa el nombre, la ruta física de la carpeta, el comando del servidor (ej. `npm run dev`) y los comandos de Playwright.
2. **Gestionar Servidor:** Selecciona un proyecto y utiliza los botones `▶ Start`, `■ Stop` o `↻ Restart`.
3. **Abrir en el Navegador:** Pulsa `🌐 Open URL in Browser` para ir a la URL del proyecto.
4. **Ejecutar Pruebas de Playwright:**
   - `▶ Run Tests`: Ejecuta los tests headless.
   - `🖥 UI Mode`: Abre la interfaz gráfica interactiva de Playwright.
   - `🐛 Debug`: Ejecuta con Playwright Inspector.
   - `📊 Show Report`: Muestra el reporte HTML de la última corrida.
5. **Ver Logs:** Cambia a la pestaña *Logs* para observar la salida del servidor y de Playwright en tiempo real.

---

## Ejecución de Tests Automatizados

Para ejecutar la suite de pruebas unitarias:

```bash
.venv\Scripts\python -m pytest tests/ -v
```

---

## Solución de Problemas

- **El comando `npm` o `npx` no se reconoce:** Asegúrate de que Node.js esté añadido a las Variables de Entorno (`PATH`) de Windows y reinicia la aplicación.
- **Timeout al iniciar servidor:** Si el servidor demora más de lo esperado en compilar, aumenta el valor de `Startup Timeout` en el diálogo de edición del proyecto.
- **Configuración corrupta:** Si `projects.json` contiene sintaxis inválida, la aplicación crea un archivo de respaldo `projects.json.bak` y genera una configuración limpia sin cerrarse abruptamente.
