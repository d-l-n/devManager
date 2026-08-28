# Local Dev Manager

Una aplicación de escritorio nativa para Windows desarrollada con Go y Wails v2 para gestionar proyectos de desarrollo locales, sus servidores de desarrollo y la ejecución de tests con Playwright desde una única interfaz gráfica.

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
- **Temas Light / Dark / OLED:** Ciclo con `Ctrl+Shift+T`; el modo OLED usa negro puro (`#000000`) con textos y acentos calibrados para mantener contraste WCAG AA (≥4.5:1).
- **Iconografía Reicon:** Integración completa de iconos vectoriales SVG limpios basados en la librería open-source [Reicon](https://github.com/dqev/reicon) (trazo 1.5px, estilo outline y filled nativo para PySide6).
- **Consola de logs en tiempo real:** Captura `stdout` y `stderr` con marcas de tiempo `[HH:MM:SS]`, scroll automático y diferenciación de errores.
- **Seguridad en Windows:** Terminación segura de árboles de procesos externos mediante PID (`taskkill /T /F /PID <pid>`), sin afectar procesos de terceros.

---

## Requisitos

- **Sistema Operativo:** Windows 10 o Windows 11
- **Node.js y npm / pnpm / yarn:** Disponibles en el `PATH` del sistema para proyectos web (opcional, solo para proyectos que lo necesiten)

---

## Instalación y Ejecución

La aplicación es multiplataforma. Elige tu sistema operativo:

### Windows
- **Para iniciar en el día a día:** Haz doble clic en `devmanager-app/run-desktop.bat`
- **Para construir manualmente:** 
  ```powershell
  cd devmanager-app
  ./build-desktop.bat
  ```
- **Ejecutable directo:** `devmanager-app/build/bin/devmanager.exe`

### Linux y macOS
- **Para construir manualmente:**
  ```bash
  cd devmanager-app
  wails build
  ```
- **Para ejecutar:** `./build/bin/devmanager` (Linux) o `./build/bin/devmanager.app` (macOS)

La aplicación compilada es nativa y no requiere Python ni Node.js en tiempo de ejecución.

---

## Desarrollo y Tests

Para ejecutar la suite de pruebas unitarias:

```bash
cd devmanager-app
go test ./internal/... -v
```

Para desarrollo en modo live-reload:

```bash
cd devmanager-app
wails dev
```

---

## Estructura del Proyecto

```text
devManager/
├── README.md
├── projects.json              # Configuración de proyectos (compartido)
├── .gitignore
│
└── devmanager-app/                # Aplicación principal (Go + Wails v2)
    ├── main.go                # Punto de entrada y bindings de la app
    ├── app.go                 # Lógica principal de la aplicación
    ├── tray.go                # Integración con bandeja del sistema
    ├── wails.json             # Configuración de Wails
    ├── go.mod                 # Dependencias de Go
    ├── go.sum                 # Lock de dependencias
    │
    ├── internal/              # Módulos internos de Go
    │   ├── config/            # Gestión de configuración
    │   │   ├── manager.go     # Carga y guardado de projects.json
    │   │   └── settings.go    # Configuración persistente de la app
    │   ├── models/            # Modelos de datos
    │   │   └── project.go     # Estructuras Project, ServerState, etc.
    │   ├── server/            # Gestión de servidores
    │   │   └── manager.go     # Ciclo de vida, uptime y estados
    │   ├── process/           # Ejecución de procesos
    │   │   └── runner.go      # Wrapper para procesos Windows
    │   ├── playwright/        # Integración con Playwright
    │   │   └── manager.go     # Comandos y flujo de tests
    │   ├── scripts/           # Scripts personalizados
    │   │   └── manager.go     # Detección y ejecución
    │   ├── sysmon/            # Monitor de sistema
    │   │   └── sysmon.go      # CPU, RAM, procesos
    │   └── utils/             # Utilidades varias
    │       ├── git/           # Operaciones Git
    │       ├── evidence/       # Gestión de evidencias
    │       ├── ports/         # Detección de puertos
    │       └── detection/     # Detección automática
    │
    ├── frontend/              # Interfaz de usuario (HTML/JS/CSS)
    │   ├── index.html         # Ventana principal
    │   ├── package.json       # Dependencias del frontend
    │   └── src/
    │       ├── main.js        # Lógica principal de la UI
    │       ├── api.js         # Comunicación con Go
    │       ├── theme.js       # Gestión de temas
    │       ├── theme.css      # Estilos (Light/Dark/OLED)
    │       ├── dialogs/
    │       │   └── settings.js # Modal de configuración
    │       ├── panels/
    │       │   ├── git.js     # Panel de Git
    │       │   ├── monitor.js # Panel de monitor
    │       │   ├── playwright.js # Panel de Playwright
    │       │   ├── scripts.js # Panel de scripts
    │       │   └── evidence.js # Panel de evidencias
    │       └── widgets/
    │           └── toast.js   # Notificaciones
    │
    └── build/                 # Binarios compilados
        └── bin/
            └── devManager.exe # Ejecutable final
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

Para ejecutar la suite de pruebas unitarias de la aplicación:

```bash
cd devmanager-app
go test ./internal/... -v
```

Para ejecutar tests de Playwright en tus proyectos:

Usa el panel de Playwright en la aplicación o ejecuta directamente:
```bash
npx playwright test    # Desde la raíz de tu proyecto
```

---

## Solución de Problemas

- **El comando `npm` o `npx` no se reconoce:** Asegúrate de que Node.js esté añadido a las Variables de Entorno (`PATH`) de Windows y reinicia la aplicación.
- **Timeout al iniciar servidor:** Si el servidor demora más de lo esperado en compilar, aumenta el valor de `Startup Timeout` en el diálogo de edición del proyecto.
- **Configuración corrupta:** Si `projects.json` contiene sintaxis inválida, la aplicación crea un archivo de respaldo `projects.json.bak` y genera una configuración limpia sin cerrarse abruptamente.
- **La aplicación no inicia:** Verifica que tienes Windows 10 o 11. La aplicación es completamente nativa y no requiere instalación de Python ni dependencias adicionales.
- **Error al compilar:** Asegúrate de tener Go 1.22+ instalado y Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
