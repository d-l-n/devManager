# Diseño: Expansión de Widgets — devManager

**Fecha:** 2026-08-25
**Estado:** Aprobado en chat (ver decisiones abajo)
**Base:** Local Dev Manager (PySide6, Windows)

---

## 1. Contexto y objetivo

La app gestiona proyectos locales, servidores de dev y tests Playwright desde una GUI.
Actualmente tiene tabs: Server / Playwright / Scripts / Logs / App Log, sidebar con búsqueda,
badges de estado en header (Server/Playwright/Git) y system tray.

Esta expansión agrega 8 mejoras aprobadas:

| # | Widget | Ubicación |
|---|--------|-----------|
| 1 | Filtro por estado en sidebar | Sidebar (chips All/Running/Stopped) |
| 2 | Toggle "solo errores" en logs | LogPanel (Logs + App Log) |
| 3 | Toasts in-app | Overlay sobre MainWindow |
| 4 | Uptime del servidor | ServerPanel |
| 5 | Panel Git completo | Tab nuevo "Git" |
| 7 | Monitor de puertos | Tab nuevo "Monitor" (sección 1) |
| 8 | CPU/RAM por proceso (activable en ajustes) | Tab nuevo "Monitor" (sección 2) |
| 11 | Visor de screenshots/traces Playwright | Tab nuevo "Evidence" |

## 2. Decisiones tomadas

1. **Ubicación monitores:** un solo tab "Monitor" combinado (puertos + recursos). Git y Evidence en tabs propios.
2. **Ajustes:** diálogo dedicado `File → Settings...` (Ctrl+,) con secciones escalables.
3. **Dependencia:** se agrega `psutil` a `requirements.txt` para puertos/CPU/RAM.
4. **Acciones Git:** Pull + Fetch + Stash + Refresh solamente. Sin add/commit/push (riesgo de uso accidental).
5. **Persistencia ajustes:** `QSettings("LocalDevManager", "Settings")` vía singleton `AppSettings` (mismo patrón que `ThemeManager`). No se toca `projects.json`.
6. **Toasts vs tray:** ventana visible → toast in-app; minimizada/en tray → notificación de bandeja (comportamiento actual).

## 3. Especificación por feature

### 3.1 Subsistema de Settings (nuevo)

**Archivo:** `app/config/settings.py`

```python
class AppSettings(QObject):
    setting_changed = Signal(str, object)  # (key, value)
    # Singleton como ThemeManager.instance()
    # Backend: QSettings("LocalDevManager", "Settings")
    def get(self, key: str, default=None): ...
    def set(self, key: str, value): ...   # persiste + emite setting_changed
```

Claves v1:

| Clave | Tipo | Default | Efecto |
|-------|------|---------|--------|
| `monitor/polling_enabled` | bool | `True` | Auto-refresh sección Recursos del tab Monitor |
| `notifications/toasts_enabled` | bool | `True` | Toasts in-app ON/OFF |

**Archivo:** `app/ui/settings_dialog.py`

Modal `SettingsDialog(QDialog)` con grupos:
- **Monitores:** checkbox "Enable resource polling (CPU/RAM)".
- **Notificaciones:** checkbox "Enable in-app toast notifications".

Guardar aplica inmediatamente vía `AppSettings.set()` (no hay botón OK diferido para estos toggles; OK solo cierra).

### 3.2 Tab "Monitor" (#7 + #8)

**Archivo:** `app/process/monitor.py` (helpers psutil puros, sin Qt excepto nada — funciones libres)

```python
@dataclass
class PortOwner:
    pid: int
    name: str          # nombre de proceso ej. "node.exe"

def get_port_owner(port: int, host: str = "127.0.0.1") -> Optional[PortOwner]:
    """Dueño del puerto via psutil.net_connections(). None si libre/unknown."""

def kill_pid(pid: int) -> Tuple[bool, str]:
    """Termina PID (árbol no requerido aquí: kill recursivo de children primero)."""

def get_process_tree_usage(pid: int) -> Optional[ProcessUsage]:
    """CPU% (intervalo corto, suma árbol) y RSS MB del proceso + children."""
```

Todas envueltas en try/except contra `psutil.NoSuchProcess`, `AccessDenied`, `psutil.Error`.
Si `psutil` no está instalado → import guard, features degradan con mensaje.

**Archivo:** `app/ui/widgets/monitor_panel.py` — `MonitorPanel(QWidget)`

API hacia MainWindow:
- `set_projects(projects: list[Project])`
- `set_managers(server_managers: Dict[int, ServerManager])` (lectura de state/pid activos)
- `refresh_requested` → Signal() (MainWindow responde escaneando)
- `kill_requested(pid: int)` → Signal() (MainWindow ejecuta kill_pid + log)
- `apply_theme(mode)`

Layout:
- Barra superior: botón Refresh, checkbox "Auto-refresh (3s)", label resumen ("2 servers · 1 conflicto").
- **Grupo "Configured Ports":** fila por proyecto con `server.enabled`: puerto, estado Libre/Ocupado, si ocupado por proceso ajeno → nombre+PID + botón Kill (danger); si el dueño es otro proyecto del manager → badge con su nombre (sin kill).
- **Grupo "Resources":** fila por servidor RUNNING: proyecto, PID raíz (+n children), CPU%, RAM MB, barra horizontal coloreada (verde <50%, amarillo <80%, rojo ≥80%).
- Setting apagado → grupo Recursos muestra placeholder "Resource polling disabled in Settings"; botón Refresh sigue operativo (scan on-demand).
- QTimer de auto-refresh corre **solo cuando el tab es visible** (`showEvent`/`hideEvent` controlan start/stop).

### 3.3 Tab "Git" (#5)

**Archivo:** `app/utils/git.py` (extendido)

```python
def run_git(project_path: str, args: list[str], timeout: float = 10.0) -> Tuple[int, str, str]:
    """Corre git CLI oculto (STARTUPINFO ya existente). (code, stdout, stderr)."""

def get_git_status_full(project_path: str) -> Dict[str, Any]:
    """Extiende get_git_info con: ahead:int, behind:int, last_commit:{hash, subject, date_rel}.
       ahead/behind = 0 si no hay upstream."""
```

**Archivo:** `app/ui/widgets/git_panel.py` — `GitPanel(QWidget)`

API:
- `set_project(project: Optional[Project])`
- Signals: `git_command_started(name)`, `output(text, is_error)` (rutea a Logs), `command_finished(name, code)`
- `apply_theme(mode)`

UI:
- Grupo info: branch (icono `git-branch`), badge dirty/clean, `↑N ↓M` con iconos `arrow_up`/`arrow_down`, línea último commit (`hash7 · subject · hace X`).
- Botonera: **Refresh / Pull / Fetch / Stash**. Ejecutan `run_git` dentro de hilo QThread simple o `QProcess` — decisión: `QProcess` (consistente con ProcessRunner de la app, no bloquea UI, output va a Logs en vivo). Botones disabled mientras corre uno.
- Resultado → status bar + toast en error (pull conflict etc).
- No-repo → empty-state "Not a git repository".
- Stash sin cambios → mensaje informativo, no error.

### 3.4 Tab "Evidence" (#11)

**Archivo:** `app/utils/evidence.py` (funciones libres)

```python
IMAGE_EXTS = {".png", ".jpg", ".jpeg"}
MEDIA_EXTS = IMAGE_EXTS | {".webm"}   # videos
TRACE_EXTS = {".zip"}

@dataclass
class EvidenceFile:
    path: str
    rel_path: str      # relativo a project.path
    kind: str          # "image" | "video" | "trace"
    mtime: float
    test_dir: str      # carpeta test-results contenedora

def scan_evidence(project_path: str, max_items: int = 200) -> List[EvidenceFile]:
    """Escanea test-results/** recursivo. Ordena mtime desc. Ignora node_modules."""

def find_html_report(project_path: str) -> Optional[str]:
    """Retorna playwright-report/index.html si existe."""
```

**Archivo:** `app/ui/widgets/evidence_panel.py` — `EvidencePanel(QWidget)`

API:
- `set_project(project: Optional[Project])` → re-escanea
- Signal: `trace_open_requested(path)` (MainWindow lo corre via PlaywrightManager-style runner: `npx playwright show-trace <file>`)
- `apply_theme(mode)`

UI:
- Splitter: izquierda QListWidget IconMode (thumbnails QPixmap 180px, label = rel_path corto), derecha QLabel preview (pixmap escalado al contenedor, keep aspect).
- Toolbar: Refresh, "Open HTML Report" (disabled si no existe).
- Doble clic imagen → preview grande derecha (default) ; context menu: Open Image Externally / Open Containing Folder.
- `.webm` → icono archivo + doble clic abre con player del sistema. `.zip` → icono trace + acción "Show Trace" (context menu).
- Escaneo en carga con límite 200 items (evita colgar UI); thumbnails lazy por lote con QTimer (evita bloquear).
- Empty-state: "No evidence found. Run Playwright tests first."

### 3.5 Toasts (#3)

**Archivo:** `app/ui/widgets/toast.py`

```python
class ToastLevel(Enum): SUCCESS INFO WARNING ERROR

class ToastManager:
    def __init__(self, main_window): ...
    def show(self, title: str, message: str, level=ToastLevel.INFO, duration_ms=4000): ...
```

- Overlay frameless hijo de MainWindow (`Qt.Tool | FramelessWindowHint`, transparente), posicionado bottom-right con margen 16px, respeta resize (reposiciona en `resizeEvent` de main_window vía callback del manager).
- Máx 3 visibles: el más viejo se descarta. Stack vertical.
- Animación fade-in/out `QPropertyAnimation` sobre `QGraphicsOpacityEffect`. Clic → cierra inmediato.
- Estilo por nivel usando colores tema (`success/warning/danger/info`) + icono (`check/info/bell/bug`).
- Respeta `notifications/toasts_enabled`: OFF → no-op (tray sigue cubriendo eventos).

**Routing en main_window:** helper `_notify(title, msg, level)`:
```python
if ventana visible: ToastManager.show(...) else: tray.show_notification(...)
```
Reemplaza llamadas actuales a `_tray.show_notification` en: `_on_server_state_changed`, `_on_playwright_state_changed` (FAILED/ERROR y también PASSED→toast success), `_on_script_finished` (fallo), `_on_config_error`, `_on_port_mismatch`.

### 3.6 Uptime (#4)

- `ServerManager`: atributo `_started_at: Optional[float]`; set en transición a `RUNNING` (tanto `_on_port_ready` como detección dinámica en `_on_runner_output` y caso port<=0); reset `None` en `stop()` y `_on_runner_finished`. Property `started_at`.
- `ServerPanel`: label `Uptime: —` junto a Status. Método `set_uptime(seconds: Optional[float])` formatea `1h 23m 45s` / `4m 12s` / `45s`.
- `main_window`: QTimer 1s → si proyecto actual RUNNING, calcula uptime y llama `set_uptime`; STOPPED → `set_uptime(None)`.

### 3.7 Filtro de estado sidebar (#1 resto)

`project_list.py`:
- Debajo de `_search_input`: 3 botones checkables exclusivos (segmentados): **All / Running / Stopped**, estilo chips existentes.
- Estado interno `_status_filter: str = "all"`.
- Predicado combinado en `_filter_projects`: texto AND estado. Running = `state == RUNNING`; Stopped = todo lo demás (incluye ERROR — visible, no oculto).
- Re-filtrar automáticamente en `update_status()` si filtro activo.

### 3.8 Solo errores en logs (#2 resto)

`log_panel.py`:
- Checkbox **Errors only** entre Wrap Lines y Auto-scroll.
- Estado `_errors_only: bool`. `_apply_filter` combina texto + flag. `append_log/append_error` consultan ambos antes de insertar.
- Al togglear → re-render completo desde `_raw_lines` (ya existe esa ruta en `_apply_filter`).
- Beneficio automático para tabs Logs y App Log (misma clase).

## 4. Flujo de datos

```
MainWindow (orquestador)
 ├── ConfigManager ── projects.json (sin cambios)
 ├── AppSettings (singleton) ── QSettings registro Windows
 ├── ServerManager[*] ── +started_at ──┐
 │     └── ProcessRunner.pid() ────────┼── MonitorPanel (poll psutil)
 ├── GitPanel ── git.py.run_git ── QProcess out → LogPanel
 ├── EvidencePanel ── evidence.scan_evidence(fs) ── trace → ProcessRunner
 ├── ToastManager ← _notify() routing
 └── Tray (fallback cuando hidden)
```

## 5. Manejo de errores

- psutil ausente/fallando → panel muestra aviso inline, app no crashea.
- Procesos desaparecen mid-poll → `NoSuchProcess` capturada, fila se remueve en próximo refresh.
- Kill denegado (AccessDenied) → toast error + log.
- Git no instalado / no repo → empty-states claros; comandos deshabilitados.
- Upstream inexistente → ahead/behind muestran "—" (no error).
- test-results gigante → cap 200 evidencias + thumbnails lazy.
- Settings corruptos → QSettings retorna defaults, sin crash.

## 6. Testing

Tests nuevos (pytest, siguiendo patrón existente — solo lógica/utils):

| Archivo | Cubre |
|---------|-------|
| `tests/test_settings.py` | Defaults, set/get roundtrip, signal emitida |
| `tests/test_system_monitor.py` | Puerto libre → owner None; pid inválido → no crash; kill_pid pid muerto → (False, msg) |
| `tests/test_evidence.py` | tmp dir con png/webm/zip fake → scan ordena mtime desc, filtra extensiones, respeta max_items |
| `tests/test_git.py` (extender) | `get_git_status_full` en no-repo → is_repo False sin excepción; `run_git` args inválidos → code != 0 |

Verificación manual post-implementación: correr app real (`run.bat`), probar cada tab, toasts con server real, settings toggle persiste tras restart.

Suite completa debe quedar verde: `.venv\Scripts\python -m pytest tests/ -v`

## 7. Inventario de archivos

**Nuevos:**
```
app/config/settings.py
app/process/monitor.py
app/utils/evidence.py
app/ui/settings_dialog.py
app/ui/widgets/toast.py
app/ui/widgets/git_panel.py
app/ui/widgets/monitor_panel.py
app/ui/widgets/evidence_panel.py
tests/test_settings.py
tests/test_system_monitor.py
tests/test_evidence.py
docs/superpowers/specs/2026-08-25-widgets-expansion-design.md  (este archivo)
```

**Modificados:**
```
requirements.txt                  (+psutil)
app/server/manager.py             (started_at)
app/utils/git.py                  (run_git, get_git_status_full)
app/ui/main_window.py             (tabs nuevos, menú Settings, _notify routing,
                                   timer uptime, wiring señales, apply_theme nuevos paneles)
app/ui/widgets/project_list.py    (chips filtro estado)
app/ui/widgets/log_panel.py       (checkbox errors-only)
app/ui/widgets/server_panel.py    (label uptime)
README.md                         (documentar features nuevas)
```

## 8. Fuera de alcance (explícito)

- Command palette (Ctrl+K), editor .env, monitor dependencias npm, historial persistente de tests.
- Acciones git destructivas (add/commit/push/reset).
- Refactor de `main_window.py` más allá del wiring necesario.
- Tests unitarios de widgets Qt (patrón actual no los cubre).

## 9. Orden de implementación

1. **Fase A — lógica:** settings.py, monitor.py, evidence.py, git.py ext, started_at + todos los tests. Suite verde.
2. **Fase B — widgets:** toast.py, git_panel.py, monitor_panel.py, evidence_panel.py, settings_dialog.py, edits sidebar/log/server_panel.
3. **Fase C — integración:** main_window wiring, menú, theme propagation, README, verificación manual.
