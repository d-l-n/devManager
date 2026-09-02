# devManager App

Código fuente de la aplicación de escritorio **Local Dev Manager**: la interfaz nativa
construida con **Go + Wails v2** y un frontend **Vite**.

El README principal del proyecto (features, uso, config de `projects.json`) vive en
`../README.md`. Este documento se centra en **desarrollo, build, tests y arquitectura**
de este directorio.

---

## Requisitos

- **Go ≥ 1.25**
- **Node.js ≥ 18** y npm
- **Wails CLI ≥ v2.15.0**

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
```

- **Linux:** compilar app GTK necesita dependencias del sistema:

```bash
sudo apt-get install libgtk-3-dev libwebkit2gtk-4.0-dev
```

---

## Scripts (package.json)

Todos se ejecutan desde `devmanager-app/`:

| Script | Descripción |
|--------|-------------|
| `npm run build`            | Build release (via `build.js --clean`) |
| `npm run build:debug`      | Build debug (`build.js --debug`) |
| `npm run build:clean`      | Build release limpio (`build.js --clean`) |
| `npm run build:parallel`   | Build con paralelismo (`build.js --parallel`) |
| `npm run build:dev`        | Debug + limpio |
| `npm run build:release`    | Release + limpio |
| `npm run dev`              | Live-reload (`build.js --debug --parallel`) |
| `npm run test`             | Build debug + tests frontend y backend |
| `npm run test:frontend`    | Tests del frontend |
| `npm run test:backend`     | `go test -v -race -coverprofile` sobre `./...` |
| `npm run lint`             | ESLint frontend + `gofmt` backend |
| `npm run security`         | `npm audit` + `gosec` |
| `npm run clean`            | Limpia build dirs (sin version check) |
| `npm run clean:deep`       | Clean + purga caches Go (`go clean -cache -modcache -testcache`) |
| `npm run ci`               | Build limpio + verbose (ideal CI) |
| `npm run release`          | Build release + paralelo |

> `build.js` es el wrapper multiplataforma (sustituye a `build-desktop.bat`). Opciones:
> `--clean -c`, `--debug -d`, `--parallel -p`, `--no-version-check`, `--verbose -v`, `--help -h`.

---

## Desarrollo con live-reload

```bash
cd devmanager-app
wails dev
```

Levanta el servidor Vite (frontend) y recompila el backend Go, con hot-reload del
frontend en `http://localhost:5173`.

---

## Tests

```bash
# Backend (unit tests, race detector)
cd devmanager-app
go test -v -race ./internal/...

# Un solo paquete
go test ./internal/server/ -v

# Suite completa via npm
npm run test
```

Verificación estática:

```bash
go vet ./internal/...
gofmt -l .
```

---

## Estructura del directorio

```text
devmanager-app/
├── main.go              # Punto de entrada + //go:embed all:frontend/dist
├── app.go               # Bindings Wails y lógica principal
├── tray.go              # Bandeja del sistema
├── wails.json           # Configuración de Wails (cgo: true, outputfilename)
├── build.js             # Builder multiplataforma
├── go.mod / go.sum      # Dependencias Go
│
├── internal/
│   ├── config/          # Carga/guardado de projects.json y settings
│   ├── models/          # Structs Project, ServerState, BacklogItem, etc.
│   ├── server/          # Ciclo de vida del servidor (estados, uptime, puerto)
│   ├── process/         # Ejecución de procesos (runner) + kill tree por SO
│   ├── playwright/      # Orquestación de tests Playwright con auto-start de server
│   ├── scripts/         # Scripts personalizados por proyecto
│   ├── sysmon/          # Monitor: dueño de puerto, CPU/RAM por árbol, kill_tree
│   ├── logger/          # Logger con RingBuffer de líneas
│   ├── obscura/         # (pre-existente / experimental)
│   ├── testutil/        # Helpers de test multiplataforma (cmd strings)
│   └── utils/
│       ├── git/         # Operaciones Git
│       ├── ports/       # IsPortOpen, WaitForPort, BuildServerCommand
│       ├── detection/   # Autodetección de proyectos/config
│       ├── evidence/    # Gestión de evidencias
│       └── theme/       # Detección de temas
│
├── frontend/            # UI (Vite)
│   ├── index.html
│   ├── package.json
│   └── src/
│       ├── main.js      # Lógica principal de la UI
│       ├── api.js       # Comunicación con Go
│       ├── theme.js / theme.css
│       ├── dialogs/     # settings.js, etc.
│       ├── panels/      # git.js, monitor.js, playwright.js, scripts.js, evidence.js
│       └── widgets/     # toast.js
│
└── build/
    ├── bin/             # Binario compilado (devmanager.exe / devmanager)
    ├── version.json     # Metadatos de build (CI)
    └── build-report-*   # Reporte generado por build.js
```

---

## Convenciones de código

- El backend es una **port 1:1 de la app Python original** (`app/...py`). Cada archivo
  Go comienza con un comentario `// porta <modulo>.py` para trazar el origen y mantener
  la paridad de comportamiento. **No se añaden features nuevas a un módulo portado sin
  revisar la fuente Python.**
- Los comentarios y strings de la UI están en **español**.
- **Concurrencia:** los callbacks (`OnStdout`, `OnStderr`, `OnStateChange`, ...) se
  disparan desde goroutines; el consumidor debe serializar acceso a su estado (mutex).
- **Multiplataforma Windows/Unix:** funcionalidad dependiente del SO se separa con
  build tags (`_windows.go` / `_unix.go`, p. ej. `shell_*`, `kill_*` en `process/` y
  `sysmon/`).

---

## CI / GitHub Actions

Los workflows viven en `.github/workflows/`:

- `build-dev-v2.yml` — builds dev (Windows/macOS/Linux amd64) + tests + security.
- `build-multiplatform.yml` — builds de release multi-plataforma (incluye arm64) + release al tag.
- `version-check.yml` — consistencia de versiones entre `wails.json`, `package.json`.
- `cleanup.yml` — limpieza semanal de artifacts.

Notas para CI:

- El job `test` debe construir el frontend (`npm run build`) **antes** de `go test`,
  porque `main.go` embebe `all:frontend/dist`.
- Linux corre en `ubuntu-22.04` (compat `webkit2gtk-4.0`); el build envolver el binario
  con `dbus-launch` + `xvfb-run` para que el systray no paniquee en headless.
- El build Linux requiere `CGO_ENABLED=1` **y** `cgo: true` en `wails.json`.