# Local Dev Manager

Una aplicación de escritorio nativa para Windows (Go + Wails v2) para gestionar proyectos de desarrollo locales, sus servidores, tests con Playwright, dependencias, evidencias y backlog desde una única interfaz gráfica.

---

## Características

- **Gestión de proyectos:** Añade, edita, elimina y fija (pin) proyectos locales desde la interfaz o directamente en `projects.json`. Detección automática de configuración (comando de servidor, dependencias, Playwright) al añadir un proyecto.
- **Control de servidores:** Inicia, detiene y reinicia servidores (`npm run dev`, `vite`, `yarn`, `pnpm`, etc.) con monitoreo automático de estado y puerto.
- **Detección de disponibilidad:** Sondeo de puerto con timeout configurable para asegurar que el servidor esté activo antes de lanzar pruebas.
- **Integración con Playwright:**
  - Ejecución de tests estándar en segundo plano.
  - Modo UI interactivo (`--ui`) gestionado como proceso persistente.
  - Modo Debug (`--debug`).
  - Visualización del reporte HTML (`show-report`).
  - Auto-inicio de servidor: si no está corriendo al lanzar tests, se levanta y espera disponibilidad antes de ejecutar Playwright.
- **Tabs personalizables por proyecto:** Oculta tabs y reordénalos (icono de ajustes en la barra de tabs). `logs` no se puede ocultar (fallback del tab activo). Los tabs se sincronizan con `project.tabs.{hidden,order}`.
- **Tab Deps:** Dashboard de dependencias del proyecto (npm/pnpm/yarn/bun + Go): lista de dependencias, versiones desactualizadas (`outdated`) y auditoría de seguridad. Refresca solo cuando el tab está activo (no ejecuta `npm outdated`/`go list` en background).
- **Tab Evidencia:** Galería de screenshots, videos y traces de `test-results/` con preview integrado, apertura externa y visor de traces; estado automático activo/inactivo según existan evidencias.
- **Tab Obscura:** Navegador headless auxiliar (herramienta experimental).
- **Tab Backlog:** Items por proyecto con estados (`todo`, `in-progress`, `done`) y prioridades.
- **Tab Git:** Rama, estado dirty, ahead/behind, último commit y acciones Pull/Fetch/Stash con salida en vivo en Logs; herramientas de diff, branches y tags.
- **Tab Monitor:** Estado de puertos configurados (libre/ocupado/proceso ajeno con Kill) y CPU/RAM por árbol de proceso de cada servidor (auto-refresh 3s, configurable en Settings). Ventana global independiente de proyectos (`Ctrl+Alt+M` o botón del sidebar).
- **Creación de usuarios por proyecto:** Ejecuta un comando de creación de usuario propio del proyecto (Firebase, REST, seed de DB, …) desde la interfaz. El comando recibe los datos en variables de entorno `DM_USER_EMAIL`, `DM_USER_NAME`, `DM_USER_PASSWORD`, `DM_USER_ROLE` (sin interpolación de shell) y se auto-detecta desde `package.json`/scripts al configurar el proyecto.
- **Acciones globales:** Iniciar y detener todos los servidores habilitados en lote.
- **Updater integrado:** Comprueba actualizaciones y descarga nuevas versiones desde GitHub Releases.
- **Toasts in-app:** Notificaciones visuales apilables cuando la ventana está visible; bandeja del sistema cuando está minimizada.
- **Uptime:** Tiempo encendido de cada servidor en el panel Server.
- **Filtros:** Sidebar con chips All/Running/Stopped combinables con búsqueda, y logs con modo "Errors only".
- **App Log global:** Ventana independiente (`Ctrl+Alt+L` o botón del sidebar) con el log de la aplicación, separada del contexto de proyecto.
- **Sidebar compacto:** Acciones Add/Edit/Remove como iconos en el header (`Ctrl+N`, `Ctrl+E`, `Supr`) y doble clic para editar.
- **Settings (`Ctrl+,`):** Preferencias persistentes (polling de recursos, toasts) y personalización de acentos por estilo y global.
- **Temas y estilos:** Modo claro / oscuro / OLED (`Ctrl+Shift+T`) y estilos Brutalist, Glassmorphism, Retro y Dracula, con acentos calibrados para mantener contraste WCAG AA (≥4.5:1).
- **Iconografía Reicon:** Iconos vectoriales SVG de la librería open-source [Reicon](https://github.com/dqev/reicon) (trazo 1.5px, outline/filled).
- **Consola de logs en tiempo real:** Captura `stdout`/`stderr` con marcas `[HH:MM:SS]`, scroll automático y diferenciación de errores.
- **Seguridad en Windows:** Terminación segura de árboles de procesos mediante PID (`taskkill /T /F /PID`), sin afectar procesos de terceros; procesos hijo sin consola visible (evita flashes de ventana en la app GUI).

---

## Requisitos

- **Sistema Operativo:** Windows 10 o Windows 11
- **Node.js y npm / pnpm / yarn / bun:** Disponibles en el `PATH` para proyectos web (opcional, solo si los proyectos lo necesitan)
- **Go ≥ 1.25 y Wails CLI ≥ v2.15.0:** Solo para compilar desde código (ver `devmanager-app/README.md`)

---

## Instalación y Ejecución

### Windows
- **Uso diario (binario ya compilado):** Doble clic en `devmanager-app/run-desktop.bat` (inicia `build\bin\devmanager.exe`; si no existe, pide compilar antes).
- **Construir manualmente:**
  ```powershell
  cd devmanager-app
  wails build
  ```
  (o `.\build.bat` con popups de éxito/error)
- **Ejecutable directo:** `devmanager-app/build/bin/devmanager.exe`

### Linux y macOS
```bash
cd devmanager-app
wails build
```
> Requisitos de sistema (GTK/WebKit2) en `devmanager-app/README.md`.

La aplicación compilada es nativa y no requiere Python ni Node.js en tiempo de ejecución.

---

## Desarrollo y Tests

```bash
cd devmanager-app
# Backend (race detector)
go test ./... -v -race
# Frontend (vitest, jsdom)
cd frontend && npm run test:frontend   # o: npx vitest run
# Suite completa vía npm
cd devmanager-app && npm run test
```

Live-reload:

```bash
cd devmanager-app
wails dev
```

---

## Estructura del Proyecto

```text
devManager/
├── README.md                  # Este documento
├── .gitignore
├── CHANGELOG.md
├── scripts/                   # Automatización de releases (EN)
│   └── bump-version.sh, create-release.sh, check-release.sh
│
└── devmanager-app/            # Aplicación principal (Go + Wails v2 + Vite)
    ├── main.go                # Punto de entrada + //go:embed all:frontend/dist
    ├── app.go                 # Bindings Wails, ciclos de vida, config path
    ├── app_*.go               # Bindings temáticos (user, deps, git, evidence,
    │                          #   playwright, monitor, log, notify, settings,
    │                          #   backlog, discovery)
    ├── tray.go, build.js, run-desktop.bat, build.bat, wails.json
    │
    ├── internal/              # Módulos internos de Go
    │   ├── config/            # Carga/guardado de projects.json + settings
    │   ├── models/            # Project, ServerConfig, PlaywrightConfig,
    │   │                      #   UserConfig, TabsConfig, BacklogItem
    │   ├── server/            # Ciclo de vida, uptime y estados de servidores
    │   ├── process/           # Ejecución de procesos (runner, kill tree, consolas ocultas)
    │   ├── playwright/        # Orquestación de tests Playwright
    │   ├── scripts/           # Scripts personalizados por proyecto
    │   ├── sysmon/            # Monitor: puertos, CPU/RAM por árbol, kill tree
    │   ├── obscura/           # Navegador headless auxiliar (experimental)
    │   ├── logger/            # Logger con RingBuffer de líneas
    │   ├── testutil/          # Helpers de test multiplataforma
    │   └── utils/
    │       ├── deps/          # Manifiestos npm/go, outdated, audit (hideCmd)
    │       ├── detection/     # Detección automática (config + comando user)
    │       ├── evidence/      # Gestión de evidencias
    │       ├── git/           # Operaciones Git
    │       ├── ports/         # IsPortOpen, WaitForPort
    │       └── theme/         # Detección de temas
    │
    ├── frontend/              # Interfaz de usuario (HTML/JS/CSS, Vite)
    │   ├── index.html
    │   ├── vitest.config.js, package.json
    │   └── src/
    │       ├── main.js        # Lógica principal de la UI (tabs, panels, estados)
    │       ├── api.js         # Comunicación con Go
    │       ├── icons.js / icons/  # Iconografía Reicon
    │       ├── theme.js       # Temas y acentos
    │       ├── theme.css, brutalist-enhanced.css, glassmorphism.css,
    │       │   retro.css, dracula.css
    │       ├── dialogs/       # project, settings, tabs, create-user, message,
    │       │                  #   applog, backlog-item
    │       ├── panels/        # git, monitor, playwright, scripts, evidence,
    │       │                  #   deps, obscura, backlog
    │       ├── views/         # settings (full-screen)
    │       ├── widgets/       # toast, contextmenu
    │       └── __tests__/     # Tests vitest (theme, toast, tabs, create-user, …)
    │
    └── build/
        ├── bin/               # Binario compilado (devmanager.exe)
        └── version.json       # Metadatos de build (CI)
```

---

## Formato de Configuración (`projects.json`)

`projects.json` vive **junto al ejecutable** (paridad con la app Python original). Si contiene sintaxis inválida, la aplicación crea un respaldo `projects.json.bak` y genera una configuración limpia sin cerrarse.

```json
{
  "projects": [
    {
      "name": "MPoints Tracker",
      "path": "D:/Mi Home/Desktop/proyectos/mpoints-tracker",
      "pinned": true,
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
      },
      "user": {
        "enabled": true,
        "command": "npm run create-user"
      },
      "tabs": {
        "hidden": ["obscura"],
        "order": ["git", "scripts", "deps", "playwright", "evidence", "backlog", "logs"]
      }
    }
  ]
}
```

Campos opcionales (claves ausentes → defaults):

- `server` default: `enabled=true, command="npm run dev", port=5173, url="http://localhost:5173", startup_timeout=15000`
- `playwright` default: `enabled=true, command="npx playwright test"` (+ variantes `--ui`, `--debug`, `show-report`)
- `user` default: `enabled=true, command=""` (la app auto-detecta `create-user`, `create:user`, `add-user`, … en `package.json` o scripts auxiliares)
- `tabs` default: vacío → todos visibles, orden del DOM. `logs` nunca se oculta; ids desconocidos o duplicados se rechazan al validar.
- `pinned`, `backlog` — opcionales.

---

## Uso de la Interfaz

1. **Añadir Proyecto:** `+ Add` en la barra lateral → nombre, ruta, comando del servidor y comandos de Playwright (o autodetección).
2. **Gestionar Servidor:** Selecciona el proyecto y usa `▶ Start`, `■ Stop` o `↻ Restart`.
3. **Abrir en el Navegador:** Pulsa `🌐 Open URL in Browser` para ir a la URL del proyecto.
4. **Ejecutar Pruebas de Playwright:**
   - `▶ Run Tests`: tests headless.
   - `🖥 UI Mode`: interfaz gráfica interactiva.
   - `🐛 Debug`: con Playwright Inspector.
   - `📊 Show Report`: reporte HTML de la última corrida.
5. **Crear Usuario (si el proyecto lo tiene configurado):** abre el diálogo de creación, rellena email/contraseña/rol y ejecuta. El comando auto-detectado se precarga en la edición del proyecto.
6. **Customizar Tabs:** icono de ajustes al final de la barra de tabs → oculta/muestra y reordena; los cambios son por proyecto y se persisten.
7. **Ver Logs:** pestaña *Logs* con salida del servidor y Playwright en tiempo real.

---

## Ejecución de Tests Automatizados

Suite unitaria de la aplicación (backend + frontend):

```bash
cd devmanager-app
npm run test                     # build debug + go test (race) + vitest
```

Tests de Playwright de tus proyectos: usa el panel de Playwright de la app o ejecuta directamente `npx playwright test` desde la raíz del proyecto.

---

## Solución de Problemas

- **`npm` o `npx` no se reconoce:** Asegúrate de que Node.js esté en el `PATH` de Windows y reinicia la aplicación.
- **Timeout al iniciar servidor:** Aumenta `Startup Timeout` en el diálogo de edición del proyecto.
- **Configuración corrupta:** La app crea `projects.json.bak` y genera una configuración limpia sin cerrarse.
- **La aplicación no inicia:** Windows 10/11 requerido; la app es nativa y no requiere Python ni dependencias adicionales.
- **Error al compilar:** Go ≥ 1.25 y Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`.