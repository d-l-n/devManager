# Widgets Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Agregar 8 widgets/features a Local Dev Manager: filtro de estado en sidebar, "solo errores" en logs, toasts in-app, uptime de servidor, tab Git, tab Monitor (puertos + CPU/RAM con ajuste ON/OFF), y tab Evidence (screenshots/traces de Playwright).

**Architecture:** Fase A agrega capa lógica pura (settings singleton QSettings, helpers psutil, escáner de evidencias, extensiones git, timestamp de arranque en ServerManager) con tests unitarios. Fase B crea widgets PySide6 siguiendo el patrón existente (`apply_theme(mode)`, señales Qt, iconos Reicon vía `get_icon`). Fase C integra todo en `main_window.py` (tabs nuevos, menú Settings, routing de notificaciones, timers) y actualiza docs.

**Tech Stack:** Python 3.11+, PySide6 >= 6.5, psutil (nueva), pytest.

**Spec:** `docs/superpowers/specs/2026-08-25-widgets-expansion-design.md`

## Global Constraints

- NO es repo git: OMITIR todos los pasos de commit. Verificación = suite verde.
- Comando de tests SIEMPRE: `.venv\Scripts\python -m pytest <ruta> -v` desde `D:\Mi Home\Desktop\proyectos\devManager`.
- Suite completa debe quedar verde al final de cada fase: `.venv\Scripts\python -m pytest tests/ -v`
- Patrón obligatorio widgets: `__init__` termina con `self.apply_theme(ThemeManager.instance().mode)`; cada widget expone `apply_theme(self, mode: ThemeMode)` leyendo `ThemeManager.instance().get_colors(mode)`.
- Iconos SOLO del sistema Reicon: `get_icon(nombre, color)` de `app/ui/icons.py`. SVG nuevos van al dict `REICON_SVGS` (viewBox 24, stroke 1.5, round caps).
- Texto de UI en inglés (consistente con la app existente).
- Sin deps nuevas salvo `psutil`.
- No refactorizar código existente fuera de los anclajes indicados por tarea.
- Fixtures de test: `qapp` session-scoped como en `tests/test_config.py:8-12`.

---

### Task 1: Subsistema AppSettings

**Files:**
- Create: `app/config/settings.py`
- Test: `tests/test_settings.py`

**Interfaces:**
- Consumes: nada (solo Qt Core).
- Produces: `AppSettings` singleton con `get(key, default)`, `set(key, value)`, signal `setting_changed(str, object)`; constantes `KEY_POLLING_ENABLED = "monitor/polling_enabled"`, `KEY_TOASTS_ENABLED = "notifications/toasts_enabled"`.

- [ ] **Step 1: Write the failing test**

```python
# tests/test_settings.py
import pytest
from PySide6.QtCore import QSettings
from app.config.settings import AppSettings, KEY_POLLING_ENABLED, KEY_TOASTS_ENABLED


@pytest.fixture()
def qapp():
    from PySide6.QtWidgets import QApplication
    app = QApplication.instance() or QApplication([])
    return app


@pytest.fixture()
def settings(qapp, tmp_path):
    s = AppSettings(QSettings(str(tmp_path / "test_settings.ini"), QSettings.Format.IniFormat))
    yield s
    s._settings.clear()


def test_defaults(settings):
    assert settings.get(KEY_POLLING_ENABLED, True) is True
    assert settings.get(KEY_TOASTS_ENABLED, True) is True
    assert settings.get("nonexistent/key", "fallback") == "fallback"


def test_set_get_roundtrip(settings):
    settings.set(KEY_POLLING_ENABLED, False)
    assert settings.get(KEY_POLLING_ENABLED, True) is False
    settings.set(KEY_TOASTS_ENABLED, False)
    assert settings.get(KEY_TOASTS_ENABLED, True) is False


def test_signal_emitted_on_change(settings):
    received = []
    settings.setting_changed.connect(lambda k, v: received.append((k, v)))
    settings.set(KEY_POLLING_ENABLED, False)
    assert received == [(KEY_POLLING_ENABLED, False)]


def test_signal_not_emitted_when_same_value(settings):
    received = []
    settings.setting_changed.connect(lambda k, v: received.append((k, v)))
    settings.set(KEY_POLLING_ENABLED, True)   # default ya es True
    assert received == []


def test_bool_coercion_from_string(settings):
    # QSettings ini puede devolver "true"/"false" como string
    assert settings._to_bool("true") is True
    assert settings._to_bool("false") is False
    assert settings._to_bool(True) is True
    assert settings._to_bool(False) is False
```

- [ ] **Step 2: Run test to verify it fails**

Run: `.venv\Scripts\python -m pytest tests/test_settings.py -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'app.config.settings'`

- [ ] **Step 3: Write minimal implementation**

```python
# app/config/settings.py
from typing import Any, Optional
from PySide6.QtCore import QObject, Signal, QSettings

KEY_POLLING_ENABLED = "monitor/polling_enabled"
KEY_TOASTS_ENABLED = "notifications/toasts_enabled"


class AppSettings(QObject):
    """Application-wide settings backed by QSettings (Windows registry).

    Singleton mirroring ThemeManager.instance(). Inject a QSettings for tests.
    """

    setting_changed = Signal(str, object)  # (key, value)

    _instance = None

    def __init__(self, settings: Optional[QSettings] = None, parent=None):
        super().__init__(parent)
        self._settings = settings or QSettings("LocalDevManager", "Settings")
        self._cache: dict[str, Any] = {}

    @classmethod
    def instance(cls, parent=None) -> 'AppSettings':
        if cls._instance is None:
            cls._instance = cls(parent=parent)
        return cls._instance

    def _to_bool(self, value: Any) -> bool:
        if isinstance(value, bool):
            return value
        if isinstance(value, str):
            return value.strip().lower() in ("true", "1", "yes", "on")
        return bool(value)

    def get(self, key: str, default: Any = None) -> Any:
        if key in self._cache:
            return self._cache[key]
        raw = self._settings.value(key, default)
        if isinstance(default, bool) or isinstance(raw, str) and isinstance(default, bool):
            value = self._to_bool(raw)
        else:
            value = raw
        self._cache[key] = value
        return value

    def set(self, key: str, value: Any):
        current = self.get(key, None)
        self._settings.setValue(key, value)
        self._settings.sync()
        self._cache[key] = value
        if current != value:
            self.setting_changed.emit(key, value)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `.venv\Scripts\python -m pytest tests/test_settings.py -v`
Expected: PASS (5 tests)

---

### Task 2: Helpers psutil (monitor de procesos/puertos)

**Files:**
- Modify: `requirements.txt` (agregar línea)
- Create: `app/process/monitor.py`
- Test: `tests/test_system_monitor.py`

**Interfaces:**
- Consumes: nada.
- Produces:
  - `PSUTIL_AVAILABLE: bool`
  - `PortOwner(pid: int, name: str)` dataclass
  - `ProcessUsage(pid: int, name: str, cpu_percent: float, rss_mb: float, children: int)` dataclass
  - `get_port_owner(port: int) -> Optional[PortOwner]`
  - `kill_tree(pid: int) -> tuple[bool, str]`
  - `get_process_tree_usage(pid: int) -> Optional[ProcessUsage]`

- [ ] **Step 1: Install dependency**

Edit `requirements.txt`, contenido final completo:

```
PySide6>=6.5.0
pytest>=7.0.0
psutil>=5.9.0
```

Run: `.venv\Scripts\python -m pip install psutil>=5.9.0`
Expected: instalado sin errores.

- [ ] **Step 2: Write the failing test**

```python
# tests/test_system_monitor.py
import socket
import pytest
from app.process import monitor

pytest.importorskip("psutil")


def _get_free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def test_port_owner_free_port_returns_none():
    port = _get_free_port()
    assert monitor.get_port_owner(port) is None


def test_kill_tree_invalid_pid_no_crash():
    ok, msg = monitor.kill_tree(9_999_999)
    assert ok is False
    assert msg  # mensaje descriptivo no vacío


def test_process_usage_invalid_pid_returns_none():
    assert monitor.get_process_tree_usage(9_999_999) is None


def test_psutil_available_flag():
    assert monitor.PSUTIL_AVAILABLE is True


def test_self_process_usage():
    import os
    usage = monitor.get_process_tree_usage(os.getpid())
    # Primer llamado: cpu 0.0 (baseline), rss > 0
    assert usage is not None
    assert usage.rss_mb > 0
    assert usage.cpu_percent >= 0.0
```

- [ ] **Step 3: Run test to verify it fails**

Run: `.venv\Scripts\python -m pytest tests/test_system_monitor.py -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'app.process.monitor'`

- [ ] **Step 4: Write minimal implementation**

```python
# app/process/monitor.py
"""Process/port inspection helpers built on psutil.

CPU% strategy: Process objects are cached per-pid so psutil's internal
baseline persists between polls. First poll reports cpu_percent == 0.0;
real values appear from the second poll onwards (3s cadence in MonitorPanel).
"""
import threading
from dataclasses import dataclass
from typing import Dict, Optional, Tuple

try:
    import psutil
    PSUTIL_AVAILABLE = True
except ImportError:
    psutil = None
    PSUTIL_AVAILABLE = False

_proc_lock = threading.Lock()
_proc_cache: Dict[int, "psutil.Process"] = {}


def _get_cached_process(pid: int):
    with _proc_lock:
        proc = _proc_cache.get(pid)
        if proc is None:
            try:
                proc = psutil.Process(pid)
                proc.cpu_percent(None)  # prime baseline
                _proc_cache[pid] = proc
            except (psutil.NoSuchProcess, psutil.AccessDenied, psutil.Error):
                return None
        return proc


def get_port_owner(port: int) -> Optional[PortOwner]:
    """Return the process listening on localhost:port, or None."""
    if not PSUTIL_AVAILABLE or port <= 0:
        return None
    try:
        for conn in psutil.net_connections(kind="inet"):
            if (
                conn.status == psutil.CONN_LISTEN
                and conn.laddr
                and conn.laddr.port == port
                and conn.pid
            ):
                try:
                    proc = psutil.Process(conn.pid)
                    return PortOwner(pid=conn.pid, name=proc.name())
                except (psutil.NoSuchProcess, psutil.AccessDenied):
                    return PortOwner(pid=conn.pid, name=f"PID {conn.pid}")
    except (psutil.AccessDenied, psutil.Error):
        return None
    return None


def kill_tree(pid: int) -> Tuple[bool, str]:
    """Kill a process and its children. Returns (success, message)."""
    if not PSUTIL_AVAILABLE:
        return False, "psutil not available"
    try:
        root = psutil.Process(pid)
    except psutil.NoSuchProcess:
        return False, f"No such process (pid {pid})"
    except psutil.AccessDenied:
        return False, f"Access denied killing pid {pid}"

    killed = []
    try:
        children = root.children(recursive=True)
    except (psutil.NoSuchProcess, psutil.Error):
        children = []
    for child in children:
        try:
            child.kill()
            killed.append(child.pid)
        except (psutil.NoSuchProcess, psutil.AccessDenied):
            pass
    try:
        root.kill()
        killed.append(root.pid)
    except (psutil.NoSuchProcess, psutil.AccessDenied) as e:
        return False, f"Failed killing root pid {pid}: {e}"

    _, alive = psutil.wait_procs([root] + children, timeout=3)
    with _proc_lock:
        for p in list(_proc_cache):
            try:
                if not psutil.pid_exists(p):
                    _proc_cache.pop(p, None)
            except psutil.Error:
                _proc_cache.pop(p, None)
    if alive:
        return False, f"{len(alive)} process(es) survived termination"
    return True, f"Terminated {len(killed)} process(es)"


def get_process_tree_usage(pid: int) -> Optional[ProcessUsage]:
    """Aggregate CPU%/RSS for a process tree. CPU valid from 2nd poll."""
    if not PSUTIL_AVAILABLE or pid <= 0:
        return None
    root = _get_cached_process(pid)
    if root is None:
        return None
    try:
        with _proc_lock:
            procs = [root] + root.children(recursive=True)
        total_cpu = 0.0
        total_rss = 0
        for p in procs:
            cached = _get_cached_process(p.pid)
            if cached is None:
                continue
            try:
                total_cpu += cached.cpu_percent(None)
                total_rss += cached.memory_info().rss
            except (psutil.NoSuchProcess, psutil.AccessDenied, psutil.Error):
                continue
        return ProcessUsage(
            pid=root.pid,
            name=root.name(),
            cpu_percent=round(total_cpu, 1),
            rss_mb=round(total_rss / (1024 * 1024), 1),
            children=max(0, len(procs) - 1),
        )
    except (psutil.NoSuchProcess, psutil.AccessDenied, psutil.Error):
        return None


@dataclass
class PortOwner:
    pid: int
    name: str


@dataclass
class ProcessUsage:
    pid: int
    name: str
    cpu_percent: float
    rss_mb: float
    children: int
```

Nota: mover las dos dataclasses ARRIBA de las funciones (orden: imports → dataclasses → constantes → funciones). El test importa solo funciones, pero Python evalúa anotaciones de retorno en tiempo de definición de firma solo si son strings o objetos existentes — para evitar `NameError`, las dataclasses DEBEN declararse antes que las funciones que las referencian.

- [ ] **Step 5: Run test to verify it passes**

Run: `.venv\Scripts\python -m pytest tests/test_system_monitor.py -v`
Expected: PASS (5 tests)

---

### Task 3: Escáner de evidencias Playwright

**Files:**
- Create: `app/utils/evidence.py`
- Test: `tests/test_evidence.py`

**Interfaces:**
- Consumes: nada.
- Produces:
  - `IMAGE_EXTS = {".png", ".jpg", ".jpeg"}`
  - `EvidenceFile(path, rel_path, kind, mtime, test_dir)` — kind ∈ `"image" | "video" | "trace"`
  - `scan_evidence(project_path: str, max_items: int = 200) -> list[EvidenceFile]` (mtime descendente)
  - `find_html_report(project_path: str) -> Optional[str]`

- [ ] **Step 1: Write the failing test**

```python
# tests/test_evidence.py
import os
import time
from app.utils.evidence import scan_evidence, find_html_report, EvidenceFile


def _make_file(root, rel, mtime_offset=0):
    p = root / rel
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_bytes(b"x")
    ts = time.time() - mtime_offset
    os.utime(p, (ts, ts))
    return p


def test_scan_orders_by_mtime_desc(tmp_path):
    _make_file(tmp_path, "test-results/old/a.png", mtime_offset=100)
    _make_file(tmp_path, "test-results/new/b.png", mtime_offset=10)
    _make_file(tmp_path, "test-results/newest/c.webm", mtime_offset=0)

    files = scan_evidence(str(tmp_path))
    names = [f.rel_path.replace("\\", "/") for f in files]
    assert names.index("test-results/newest/c.webm") < names.index("test-results/new/b.png")
    assert names.index("test-results/new/b.png") < names.index("test-results/old/a.png")


def test_scan_kinds(tmp_path):
    _make_file(tmp_path, "test-results/a.png")
    _make_file(tmp_path, "test-results/b.jpeg")
    _make_file(tmp_path, "test-results/sub/c.webm")
    _make_file(tmp_path, "test-results/d.zip")

    files = scan_evidence(str(tmp_path))
    kinds = {f.rel_path.replace("\\", "/"): f.kind for f in files}
    assert kinds["test-results/a.png"] == "image"
    assert kinds["test-results/b.jpeg"] == "image"
    assert kinds["test-results/sub/c.webm"] == "video"
    assert kinds["test-results/d.zip"] == "trace"


def test_scan_ignores_node_modules_and_git(tmp_path):
    _make_file(tmp_path, "node_modules/pkg/shot.png")
    _make_file(tmp_path, ".git/hooks/hook.png")
    _make_file(tmp_path, "test-results/real.png")

    files = scan_evidence(str(tmp_path))
    rels = [f.rel_path.replace("\\", "/") for f in files]
    assert rels == ["test-results/real.png"]


def test_scan_respects_max_items(tmp_path):
    for i in range(10):
        _make_file(tmp_path, f"test-results/s{i}.png", mtime_offset=i)
    files = scan_evidence(str(tmp_path), max_items=5)
    assert len(files) == 5
    # Los 5 más recientes
    assert all(f.rel_path.endswith(f"s{i}.png") for i, f in enumerate(files))


def test_scan_empty_project(tmp_path):
    assert scan_evidence(str(tmp_path)) == []
    assert find_html_report(str(tmp_path)) is None


def test_find_html_report(tmp_path):
    report = _make_file(tmp_path, "playwright-report/index.html")
    assert find_html_report(str(tmp_path)) == str(report)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `.venv\Scripts\python -m pytest tests/test_evidence.py -v`
Expected: FAIL — `ModuleNotFoundError`

- [ ] **Step 3: Write minimal implementation**

```python
# app/utils/evidence.py
"""Filesystem scanner for Playwright test artifacts (screenshots, videos, traces)."""
import os
from dataclasses import dataclass
from typing import List, Optional

IMAGE_EXTS = {".png", ".jpg", ".jpeg"}
VIDEO_EXTS = {".webm"}
TRACE_EXTS = {".zip"}
SKIP_DIRS = {"node_modules", ".git", "venv", ".venv", "__pycache__"}


@dataclass
class EvidenceFile:
    path: str
    rel_path: str
    kind: str          # "image" | "video" | "trace"
    mtime: float
    test_dir: str      # carpeta contenedora inmediata


def _classify(ext: str) -> Optional[str]:
    if ext in IMAGE_EXTS:
        return "image"
    if ext in VIDEO_EXTS:
        return "video"
    if ext in TRACE_EXTS:
        return "trace"
    return None


def scan_evidence(project_path: str, max_items: int = 200) -> List[EvidenceFile]:
    results_root = os.path.join(project_path, "test-results")
    found: List[EvidenceFile] = []
    if not os.path.isdir(results_root):
        return found

    for dirpath, dirnames, filenames in os.walk(results_root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        for fname in filenames:
            ext = os.path.splitext(fname)[1].lower()
            kind = _classify(ext)
            if not kind:
                continue
            full = os.path.join(dirpath, fname)
            try:
                mtime = os.path.getmtime(full)
            except OSError:
                continue
            found.append(EvidenceFile(
                path=full,
                rel_path=os.path.relpath(full, project_path),
                kind=kind,
                mtime=mtime,
                test_dir=dirpath,
            ))

    found.sort(key=lambda f: f.mtime, reverse=True)
    return found[:max_items]


def find_html_report(project_path: str) -> Optional[str]:
    candidate = os.path.join(project_path, "playwright-report", "index.html")
    return candidate if os.path.isfile(candidate) else None
```

- [ ] **Step 4: Run test to verify it passes**

Run: `.venv\Scripts\python -m pytest tests/test_evidence.py -v`
Expected: PASS (6 tests)

---

### Task 4: Extensiones Git (run_git + estado completo)

**Files:**
- Modify: `app/utils/git.py` (append al final)
- Test: `tests/test_git.py` (crear — NO existe hoy)

**Interfaces:**
- Consumes: patrón STARTUPINFO oculto existente en `git.py:53-56`.
- Produces:
  - `run_git(project_path: str, args: list, timeout: float = 10.0) -> tuple[int, str, str]` → `(code, stdout, stderr)`
  - `get_git_status_full(project_path: str) -> dict` → extiende `get_git_info` con `ahead: int, behind: int, has_upstream: bool, last_commit: Optional[dict{hash, subject, date_rel}]`

- [ ] **Step 1: Write the failing test**

```python
# tests/test_git.py
import shutil
import pytest
from app.utils.git import get_git_status_full, run_git, get_git_info

GIT_AVAILABLE = shutil.which("git") is not None


def test_status_full_non_repo(tmp_path):
    info = get_git_status_full(str(tmp_path))
    assert info["is_repo"] is False
    assert info["branch"] == ""
    assert info["is_dirty"] is False
    assert info["ahead"] == 0
    assert info["behind"] == 0
    assert info["has_upstream"] is False
    assert info["last_commit"] is None


def test_run_git_invalid_args_non_repo(tmp_path):
    code, out, err = run_git(str(tmp_path), ["status"])
    assert code != 0
    assert err  # mensaje de error presente


@pytest.mark.skipif(not GIT_AVAILABLE, reason="git CLI not available")
def test_run_git_in_real_repo(tmp_path):
    assert run_git(str(tmp_path), ["init"]).returncode if False else True
    code, out, err = run_git(str(tmp_path), ["init"])
    assert code == 0
    code, out, err = run_git(str(tmp_path), [
        "-c", "user.email=test@test.dev",
        "-c", "user.name=Test",
        "commit", "--allow-empty", "-m", "initial",
    ])
    assert code == 0
    code, out, err = run_git(str(tmp_path), ["rev-parse", "--abbrev-ref", "HEAD"])
    assert code == 0

    info = get_git_status_full(str(tmp_path))
    assert info["is_repo"] is True
    assert info["branch"]
    assert info["has_upstream"] is False   # repo sin remote
    assert info["ahead"] == 0
    assert info["behind"] == 0
    assert info["last_commit"]["subject"] == "initial"
    assert len(info["last_commit"]["hash"]) == 7
    assert info["last_commit"]["date_rel"]  # ej. "seconds ago"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `.venv\Scripts\python -m pytest tests/test_git.py -v`
Expected: FAIL — `ImportError: cannot import name 'run_git'`

- [ ] **Step 3: Write minimal implementation**

Append al final de `app/utils/git.py`:

```python
def run_git(project_path: str, args: list, timeout: float = 10.0):
    """Runs a git command hidden. Returns (returncode, stdout, stderr)."""
    startupinfo = None
    if os.name == 'nt':
        startupinfo = subprocess.STARTUPINFO()
        startupinfo.dwFlags |= subprocess.STARTF_USESHOWWINDOW
        startupinfo.wShowWindow = subprocess.SW_HIDE
    try:
        proc = subprocess.run(
            ["git", *args],
            cwd=project_path,
            capture_output=True,
            text=True,
            timeout=timeout,
            startupinfo=startupinfo,
        )
        return proc.returncode, proc.stdout, proc.stderr
    except FileNotFoundError:
        return -1, "", "git executable not found in PATH"
    except subprocess.TimeoutExpired:
        return -1, "", f"git command timed out after {timeout}s"
    except Exception as e:
        return -1, "", str(e)


def get_git_status_full(project_path: str) -> Dict[str, Any]:
    """Extends get_git_info with ahead/behind counts and last commit metadata."""
    result = get_git_info(project_path)
    result.update({
        "ahead": 0,
        "behind": 0,
        "has_upstream": False,
        "last_commit": None,
    })
    if not result["is_repo"]:
        return result

    code, out, _err = run_git(
        project_path,
        ["rev-list", "--left-right", "--count", "@{upstream}...HEAD"],
        timeout=5.0,
    )
    if code == 0 and "\t" in out:
        left, right = out.strip().split("\t")
        result["has_upstream"] = True
        result["behind"] = int(left) if left.isdigit() else 0
        result["ahead"] = int(right) if right.isdigit() else 0

    code, out, _err = run_git(
        project_path,
        ["log", "-1", "--pretty=format:%h|%s|%cr"],
        timeout=5.0,
    )
    if code == 0 and out.strip():
        parts = out.split("|", 2)
        if len(parts) == 3:
            result["last_commit"] = {
                "hash": parts[0].strip(),
                "subject": parts[1].strip(),
                "date_rel": parts[2].strip(),
            }

    return result
```

- [ ] **Step 4: Run test to verify it passes**

Run: `.venv\Scripts\python -m pytest tests/test_git.py -v`
Expected: PASS (3 tests; el tercero puede SKIPear si no hay git)

---

### Task 5: Uptime en ServerManager (`started_at`)

**Files:**
- Modify: `app/server/manager.py`
- Test: `tests/test_server_manager.py` (extender)

**Interfaces:**
- Consumes: transiciones de estado existentes (`_set_state`).
- Produces: `ServerManager.started_at -> Optional[float]` (epoch seconds; `None` si parado). Se setea en TODAS las rutas hacia `RUNNING` (`_on_runner_started` caso port<=0, `_on_port_ready`, detección dinámica en `_on_runner_output`) y se resetea en `stop()` y `_on_runner_finished`.

- [ ] **Step 1: Write the failing test**

Append a `tests/test_server_manager.py`:

```python
import time

def test_started_at_lifecycle():
    proj = Project(name="T", path=".", server=ServerConfig(enabled=True, port=5173))
    sm = ServerManager(proj)
    assert sm.started_at is None

    # Transición directa a RUNNING (como hace _on_port_ready)
    sm._set_state(ServerState.RUNNING)
    assert sm.started_at is not None
    first = sm.started_at
    assert abs(first - time.time()) < 5

    # Re-set no cambia el timestamp mientras siga RUNNING
    time.sleep(0.01)
    sm._set_state(ServerState.RUNNING)  # no-op interno, no debe resetear
    assert sm.started_at == first

    # Stop resetea
    sm.stop()
    assert sm.started_at is None
    assert sm.state == ServerState.STOPPED


def test_started_at_resets_on_crash_finish():
    proj = Project(name="T", path=".", server=ServerConfig(enabled=True, port=5173))
    sm = ServerManager(proj)
    sm._set_state(ServerState.RUNNING)
    assert sm.started_at is not None
    sm._on_runner_finished(1, "CrashExit")
    assert sm.started_at is None
    assert sm.state == ServerState.ERROR
```

- [ ] **Step 2: Run test to verify it fails**

Run: `.venv\Scripts\python -m pytest tests/test_server_manager.py -v`
Expected: FAIL — `AttributeError: 'ServerManager' object has no attribute 'started_at'`

- [ ] **Step 3: Write minimal implementation**

En `app/server/manager.py`:

(a) Import arriba: `import time`

(b) En `__init__`, después de `self._state = ServerState.STOPPED`:
```python
        self._started_at: Optional[float] = None
```

(c) Después del property `is_port_mismatch` (línea ~56), agregar:
```python
    @property
    def started_at(self) -> Optional[float]:
        """Epoch timestamp captured when the server entered RUNNING. None if stopped."""
        return self._started_at
```

(d) Helper nuevo después de `_set_state`:
```python
    def _enter_running(self):
        if self._started_at is None:
            self._started_at = time.time()
        self._set_state(ServerState.RUNNING)
```

(e) Reemplazar TODAS las transiciones directas a RUNNING por `_enter_running()`:
- En `_on_runner_started`, rama `elif port <= 0:` → `self._set_state(ServerState.RUNNING)` se convierte en:
```python
        elif port <= 0:
            self._enter_running()
            self.ready.emit()
```
- En `_on_port_ready`:
```python
    def _on_port_ready(self):
        self._enter_running()
        self.ready.emit()
        self.log_output.emit(f'Server is ready on port {self._active_port}')
```
- En `_on_runner_output`, bloque final dentro de `if self._state == ServerState.STARTING:` → `self._set_state(ServerState.RUNNING)` pasa a:
```python
                self._enter_running()
                self.ready.emit()
```

(f) Reset en `stop()` — después de `self._runner.stop()`:
```python
        self._runner.stop()
        self._started_at = None
        self._set_state(ServerState.STOPPED)
```

(g) Reset en `_on_runner_finished` — antes del if de exit_code:
```python
        self._started_at = None
```

- [ ] **Step 4: Run test to verify it passes**

Run: `.venv\Scripts\python -m pytest tests/test_server_manager.py tests/test_main_window.py -v`
Expected: PASS (todos, incluidos los preexistentes)

---

### Task 6: Iconos Reicon nuevos

**Files:**
- Modify: `app/ui/icons.py` (entradas nuevas en `REICON_SVGS`, antes de la llave de cierre `"bell"`)

**Interfaces:**
- Produces: iconos `"settings"`, `"activity"`, `"image"`, `"film"`, `"archive"`, `"layers"` usables con `get_icon()`.

- [ ] **Step 1: Add SVG entries**

Insertar ANTES de la entrada `"bell"` (línea ~194):

```python
    "settings": """<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="{color}" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  <circle cx="12" cy="12" r="3"></circle>
  <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
</svg>""",
    "activity": """<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="{color}" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline>
</svg>""",
    "image": """<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="{color}" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  <rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect>
  <circle cx="8.5" cy="8.5" r="1.5"></circle>
  <polyline points="21 15 16 10 5 21"></polyline>
</svg>""",
    "film": """<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="{color}" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  <rect x="2" y="2" width="20" height="20" rx="2.18" ry="2.18"></rect>
  <line x1="7" y1="2" x2="7" y2="22"></line>
  <line x1="17" y1="2" x2="17" y2="22"></line>
  <line x1="2" y1="12" x2="22" y2="12"></line>
  <line x1="2" y1="7" x2="7" y2="7"></line>
  <line x1="2" y1="17" x2="7" y2="17"></line>
  <line x1="17" y1="17" x2="22" y2="17"></line>
  <line x1="17" y1="7" x2="22" y2="7"></line>
</svg>""",
    "archive": """<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="{color}" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  <polyline points="21 8 21 21 3 21 3 8"></polyline>
  <rect x="1" y="3" width="22" height="5"></rect>
  <line x1="10" y1="12" x2="14" y2="12"></line>
</svg>""",
    "layers": """<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="{color}" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  <polygon points="12 2 2 7 12 12 22 7 12 2"></polygon>
  <polyline points="2 17 12 22 22 17"></polyline>
  <polyline points="2 12 12 17 22 12"></polyline>
</svg>""",
```

Asignación: `settings`→menú/diálogo Settings, `activity`→tab Monitor, `image`→tab Evidence + thumbs fallback, `film`→videos, `archive`→traces zip, `layers`→botón Stash.

- [ ] **Step 2: Smoke verification**

Run: `.venv\Scripts\python -c "from app.ui.icons import REICON_SVGS; assert all(k in REICON_SVGS for k in ['settings','activity','image','film','archive','layers']); from app.ui.icons import ensure_resources_exist; ensure_resources_exist(); print('OK')"`
Expected: imprime `OK`

---

### Task 7: ToastManager (notificaciones in-app)

**Files:**
- Create: `app/ui/widgets/toast.py`

**Interfaces:**
- Consumes: `AppSettings.instance().get(KEY_TOASTS_ENABLED, True)`; `ThemeManager`.
- Produces: `ToastLevel(Enum)`: SUCCESS/INFO/WARNING/ERROR; `ToastManager(main_window)` con `show(title, message, level=ToastLevel.INFO, duration_ms=4000)` y `reposition()` (llamar desde resizeEvent de MainWindow).

- [ ] **Step 1: Write implementation**

```python
# app/ui/widgets/toast.py
"""Stacked toast notifications overlaid on the main window (bottom-right)."""
from enum import Enum
from PySide6.QtWidgets import QWidget, QFrame, QHBoxLayout, QVBoxLayout, QLabel, QGraphicsOpacityEffect
from PySide6.QtCore import Qt, QTimer, QPropertyAnimation, QEasingCurve
from PySide6.QtGui import QFont
from app.config.settings import AppSettings, KEY_TOASTS_ENABLED
from app.ui.icons import get_icon
from app.ui.theme import ThemeManager, ui_font

TOAST_WIDTH = 340
TOAST_SPACING = 8
MARGIN = 16
MAX_VISIBLE = 3


class ToastLevel(Enum):
    SUCCESS = "success"
    INFO = "info"
    WARNING = "warning"
    ERROR = "error"


_LEVEL_STYLE = {
    ToastLevel.SUCCESS: ("success", "check"),
    ToastLevel.INFO: ("info", "info"),
    ToastLevel.WARNING: ("warning", "bell"),
    ToastLevel.ERROR: ("danger", "bug"),
}


class ToastFrame(QFrame):
    """Single toast card. Click dismisses immediately."""

    def __init__(self, title: str, message: str, level: ToastLevel, duration_ms: int, on_close, parent=None):
        super().__init__(parent)
        self._on_close = on_close
        self.setFixedWidth(TOAST_WIDTH)
        self.setCursor(Qt.CursorShape.PointingHandCursor)
        self.mousePressEvent = lambda _e: self.close_now()

        color_key, icon_name = _LEVEL_STYLE[level]
        c = ThemeManager.instance().get_colors()
        accent = c[color_key]

        self.setStyleSheet(
            f"QFrame {{ background-color: {c['bg_card']}; border: 1px solid {accent}; "
            f"border-left: 4px solid {accent}; border-radius: 8px; }}"
        )

        row = QHBoxLayout(self)
        row.setContentsMargins(12, 10, 12, 10)
        row.setSpacing(10)

        icon_label = QLabel()
        icon_label.setPixmap(get_icon(icon_name, accent).pixmap(18, 18))
        icon_label.setFixedWidth(20)
        icon_label.setStyleSheet("border: none;")
        row.addWidget(icon_label, 0, Qt.AlignmentFlag.AlignTop)

        col = QVBoxLayout()
        col.setSpacing(2)
        title_lbl = QLabel(title)
        title_lbl.setFont(ui_font(13, QFont.Weight.DemiBold))
        msg_lbl = QLabel(message)
        msg_lbl.setFont(ui_font(12))
        msg_lbl.setWordWrap(True)
        for lbl in (title_lbl, msg_lbl):
            lbl.setStyleSheet(f"border: none; color: {c['text_primary']};")
        col.addWidget(title_lbl)
        col.addWidget(msg_lbl)
        row.addLayout(col, stretch=1)

        self._effect = QGraphicsOpacityEffect(self)
        self.setGraphicsEffect(self._effect)
        self._anim = QPropertyAnimation(self._effect, b"opacity", self)
        self._anim.setDuration(220)
        self._anim.setEasingCurve(QEasingCurve.Type.OutCubic)

        QTimer.singleShot(duration_ms, self.close_now)

    def appear(self):
        self._anim.stop()
        self._anim.setStartValue(0.0)
        self._anim.setEndValue(1.0)
        self._anim.start()

    def close_now(self):
        self._anim.stop()
        self._anim.setStartValue(self._effect.opacity())
        self._anim.setEndValue(0.0)
        try:
            self._anim.finished.disconnect()
        except RuntimeError:
            pass
        self._anim.finished.connect(self._finish_close)
        self._anim.start()

    def _finish_close(self):
        self._on_close(self)
        self.deleteLater()


class ToastManager(QWidget):
    """Container stacking up to MAX_VISIBLE toasts at the main window's bottom-right."""

    def __init__(self, main_window: QWidget):
        super().__init__(main_window)
        self._host = main_window
        self._toasts: list[ToastFrame] = []
        self.setAttribute(Qt.WidgetAttribute.WA_TranslucentBackground)
        self.setAttribute(Qt.WidgetAttribute.WA_ShowWithoutActivating)

        self._layout = QVBoxLayout(self)
        self._layout.setContentsMargins(0, 0, 0, 0)
        self._layout.setSpacing(TOAST_SPACING)
        self._layout.setAlignment(Qt.AlignmentFlag.AlignBottom | Qt.AlignmentFlag.AlignRight)
        self.reposition()

    def show(self, title: str, message: str, level: ToastLevel = ToastLevel.INFO, duration_ms: int = 4000):
        if not AppSettings.instance().get(KEY_TOASTS_ENABLED, True):
            return
        toast = ToastFrame(title, message, level, duration_ms, self._remove, self)
        self._toasts.append(toast)
        self._layout.addWidget(toast)
        while len(self._toasts) > MAX_VISIBLE:
            oldest = self._toasts.pop(0)
            oldest.close_now()
        toast.appear()
        self.reposition()
        self.raise_()
        self.show_normal()

    def show_normal(self):
        QWidget.setVisible(self, True)

    def _remove(self, toast: ToastFrame):
        if toast in self._toasts:
            self._toasts.remove(toast)
        self.reposition()
        if not self._toasts:
            QWidget.setVisible(self, False)

    def reposition(self):
        if not self._host:
            return
        host_w = self._host.width()
        host_h = self._host.height()
        height = sum(t.sizeHint().height() for t in self._toasts) + TOAST_SPACING * max(0, len(self._toasts) - 1)
        height = max(height, 60)
        self.setGeometry(
            host_w - TOAST_WIDTH - MARGIN * 2,
            host_h - height - MARGIN * 2,
            TOAST_WIDTH + MARGIN,
            height + MARGIN,
        )
```

- [ ] **Step 2: Syntax verification**

Run: `.venv\Scripts\python -c "import app.ui.widgets.toast; print('OK')"`
Expected: `OK` (sin QApplication puede fallar por instanciación — este import solo define clases, no instancia; si fallara por Qt platform plugin, verificar con qapp fixture: `.venv\Scripts\python -c "from PySide6.QtWidgets import QApplication; a=QApplication([]); import app.ui.widgets.toast; print('OK')"`)

---

### Task 8: SettingsDialog

**Files:**
- Create: `app/ui/settings_dialog.py`

**Interfaces:**
- Consumes: `AppSettings`, `KEY_POLLING_ENABLED`, `KEY_TOASTS_ENABLED`.
- Produces: `SettingsDialog(parent)` — modal; cambios aplican inmediato vía `AppSettings.set()`.

- [ ] **Step 1: Write implementation**

```python
# app/ui/settings_dialog.py
from PySide6.QtWidgets import (
    QDialog, QVBoxLayout, QGroupBox, QCheckBox, QDialogButtonBox, QLabel
)
from PySide6.QtCore import Qt
from PySide6.QtGui import QFont
from app.config.settings import AppSettings, KEY_POLLING_ENABLED, KEY_TOASTS_ENABLED
from app.ui.icons import get_icon
from app.ui.theme import ThemeManager, ui_font


class SettingsDialog(QDialog):
    """Application settings. Toggles persist immediately via AppSettings."""

    def __init__(self, parent=None):
        super().__init__(parent)
        self.setWindowTitle("Settings")
        self.setMinimumWidth(440)
        self._settings = AppSettings.instance()

        layout = QVBoxLayout(self)
        layout.setContentsMargins(18, 18, 18, 14)
        layout.setSpacing(14)

        # --- Monitors ---
        monitors_group = QGroupBox("Monitors")
        mon_layout = QVBoxLayout(monitors_group)
        mon_layout.setContentsMargins(14, 16, 14, 14)
        mon_layout.setSpacing(8)

        self._polling_cb = QCheckBox("Enable resource polling (CPU / RAM)")
        self._polling_cb.setChecked(self._settings.get(KEY_POLLING_ENABLED, True))
        self._polling_cb.setToolTip("When off, the Monitor tab stops auto-refreshing resources. Manual refresh still works.")
        self._polling_cb.toggled.connect(
            lambda v: self._settings.set(KEY_POLLING_ENABLED, bool(v))
        )
        mon_layout.addWidget(self._polling_cb)

        polling_hint = QLabel("Polling runs every 3 seconds while the Monitor tab is visible.")
        polling_hint.setObjectName("hintLabel")
        mon_layout.addWidget(polling_hint)
        layout.addWidget(monitors_group)

        # --- Notifications ---
        notif_group = QGroupBox("Notifications")
        notif_layout = QVBoxLayout(notif_group)
        notif_layout.setContentsMargins(14, 16, 14, 14)
        notif_layout.setSpacing(8)

        self._toasts_cb = QCheckBox("Enable in-app toast notifications")
        self._toasts_cb.setChecked(self._settings.get(KEY_TOASTS_ENABLED, True))
        self._toasts_cb.setToolTip("When off, notifications go only to the system tray.")
        self._toasts_cb.toggled.connect(
            lambda v: self._settings.set(KEY_TOASTS_ENABLED, bool(v))
        )
        notif_layout.addWidget(self._toasts_cb)
        layout.addWidget(notif_group)

        # --- Buttons ---
        buttons = QDialogButtonBox(QDialogButtonBox.StandardButton.Ok)
        buttons.accepted.connect(self.accept)
        layout.addWidget(buttons)

        self.apply_theme()

    def apply_theme(self):
        c = ThemeManager.instance().get_colors()
        hint_style = f"QLabel#hintLabel {{ color: {c['text_muted']}; font-size: 11px; }}"
        cb_style = (
            f"QCheckBox {{ color: {c['text_primary']}; font-size: 13px; spacing: 8px; }}"
            f"QCheckBox::indicator {{ width: 16px; height: 16px; }}"
        )
        for group in self.findChildren(QGroupBox):
            group.setStyleSheet(
                f"QGroupBox {{ color: {c['text_secondary']}; font-weight: bold; "
                f"border: 1px solid {c['border']}; border-radius: 8px; margin-top: 10px; }} "
                f"QGroupBox::title {{ subcontrol-origin: margin; left: 12px; padding: 0 4px; }} "
                f"{cb_style} {hint_style}"
            )
```

- [ ] **Step 2: Syntax verification**

Run: `.venv\Scripts\python -c "from PySide6.QtWidgets import QApplication; a=QApplication([]); from app.ui.settings_dialog import SettingsDialog; print('OK')"`
Expected: `OK`

---

### Task 9: GitPanel

**Files:**
- Create: `app/ui/widgets/git_panel.py`

**Interfaces:**
- Consumes: `get_git_status_full(path)`, `Project`.
- Produces: `GitPanel(QWidget)`:
  - `set_project(project: Optional[Project])`
  - Signals: `output(text: str, is_error: bool)`, `command_finished(name: str, exit_code: int)`
  - `apply_theme(mode)`; comandos: Refresh(sync)/Pull(`pull --ff-only`)/Fetch(`fetch --all --prune`)/Stash(`stash`)

- [ ] **Step 1: Write implementation**

```python
# app/ui/widgets/git_panel.py
from typing import Optional
from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QGroupBox, QPushButton, QFrame
)
from PySide6.QtCore import Signal, QProcess, QSize
from PySide6.QtGui import QFont
from app.models.project import Project
from app.utils.git import get_git_status_full, is_git_repo
from app.ui.icons import get_icon
from app.ui.theme import ThemeManager, ThemeMode, ui_font, mono_font


class GitPanel(QWidget):
    output = Signal(str, bool)             # (text, is_error) → ruteado a Logs por MainWindow
    command_finished = Signal(str, int)    # (name, exit_code)

    GIT_COMMANDS = {
        "Pull": ["pull", "--ff-only"],
        "Fetch": ["fetch", "--all", "--prune"],
        "Stash": ["stash"],
    }

    def __init__(self, parent=None):
        super().__init__(parent)
        self._project: Optional[Project] = None
        self._info: dict = {}
        self._process: Optional[QProcess] = None
        self._running_cmd: Optional[str] = None
        self._setup_ui()
        self.apply_theme(ThemeManager.instance().mode)

    def _setup_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(14, 14, 14, 14)
        layout.setSpacing(14)

        # --- Info card ---
        self._info_group = QGroupBox("Repository Status")
        info_layout = QVBoxLayout(self._info_group)
        info_layout.setContentsMargins(14, 16, 14, 14)
        info_layout.setSpacing(8)

        row1 = QHBoxLayout()
        self._branch_label = QLabel("Branch: —")
        self._branch_label.setFont(ui_font(14, QFont.Weight.DemiBold))
        row1.addWidget(self._branch_label)
        row1.addStretch()
        self._sync_label = QLabel("")  # ↑ahead ↓behind
        row1.addWidget(self._sync_label)
        self._dirty_badge = QLabel("")
        row1.addWidget(self._dirty_badge)
        info_layout.addLayout(row1)

        self._commit_label = QLabel("Last commit: —")
        self._commit_label.setFont(mono_font(11))
        self._commit_label.setWordWrap(True)
        info_layout.addWidget(self._commit_label)

        layout.addWidget(self._info_group)

        # --- Actions ---
        self._actions_group = QGroupBox("Actions")
        actions_layout = QHBoxLayout(self._actions_group)
        actions_layout.setContentsMargins(14, 16, 14, 14)
        actions_layout.setSpacing(10)

        self._refresh_btn = QPushButton(" Refresh")
        self._pull_btn = QPushButton(" Pull")
        self._fetch_btn = QPushButton(" Fetch")
        self._stash_btn = QPushButton(" Stash")

        self._refresh_btn.setIcon(get_icon("refresh", "#ffffff"))
        self._pull_btn.setIcon(get_icon("git-pull", "#ffffff"))
        self._fetch_btn.setIcon(get_icon("external-link", "#ffffff"))
        self._stash_btn.setIcon(get_icon("layers", "#ffffff"))
        for b in (self._refresh_btn, self._pull_btn, self._fetch_btn, self._stash_btn):
            b.setIconSize(QSize(14, 14))
            b.setMinimumHeight(36)
            actions_layout.addWidget(b)

        self._refresh_btn.clicked.connect(self.refresh_info)
        self._pull_btn.clicked.connect(lambda: self._run_command("Pull"))
        self._fetch_btn.clicked.connect(lambda: self._run_command("Fetch"))
        self._stash_btn.clicked.connect(lambda: self._run_command("Stash"))

        layout.addWidget(self._actions_group)

        # --- Result strip ---
        self._result_frame = QFrame()
        result_layout = QHBoxLayout(self._result_frame)
        result_layout.setContentsMargins(12, 8, 12, 8)
        self._result_label = QLabel("")
        self._result_label.setFont(ui_font(12))
        self._result_label.setWordWrap(True)
        result_layout.addWidget(self._result_label)
        self._result_frame.hide()
        layout.addWidget(self._result_frame)

        layout.addStretch()

        # Empty state
        self._empty_label = QLabel("Not a git repository.\nOpen a project folder containing a .git directory.")
        self._empty_label.setAlignment(Qt.AlignmentFlag.AlignCenter)
        layout.addWidget(self._empty_label)

    # ---------- Public API ----------

    def set_project(self, project: Optional[Project]):
        self._project = project
        self.refresh_info()

    def refresh_info(self):
        if not self._project or not is_git_repo(self._project.path):
            self._show_empty_state(True)
            return
        self._show_empty_state(False)
        self._info = get_git_status_full(self._project.path)
        c = ThemeManager.instance().get_colors()

        branch = self._info.get("branch") or "unknown"
        dirty = self._info.get("is_dirty", False)
        self._branch_label.setText(f"Branch: {branch}")

        if dirty:
            self._dirty_badge.setText("● Uncommitted changes")
            self._dirty_badge.setStyleSheet(
                f"color: {c['warning']}; background-color: {c['warning_bg']}; "
                f"border-radius: 10px; padding: 3px 10px; font-size: 11px; font-weight: 600;"
            )
        else:
            self._dirty_badge.setText("● Clean")
            self._dirty_badge.setStyleSheet(
                f"color: {c['success']}; background-color: {c['success_bg']}; "
                f"border-radius: 10px; padding: 3px 10px; font-size: 11px; font-weight: 600;"
            )

        if self._info.get("has_upstream"):
            self._sync_label.setText(f"↑ {self._info['ahead']}   ↓ {self._info['behind']}")
            self._sync_label.setStyleSheet(f"color: {c['text_secondary']}; font-weight: 600;")
        else:
            self._sync_label.setText("(no upstream)")
            self._sync_label.setStyleSheet(f"color: {c['text_muted']}; font-size: 11px;")

        lc = self._info.get("last_commit")
        if lc:
            self._commit_label.setText(f"Last commit: {lc['hash']} · {lc['subject']} · {lc['date_rel']}")
        else:
            self._commit_label.setText("Last commit: —")

    # ---------- Command execution ----------

    def _run_command(self, name: str):
        if not self._project or self._process is not None:
            return
        args = self.GIT_COMMANDS[name]
        self._running_cmd = name
        self._set_buttons_enabled(False)
        self._show_result(f"Running: git {args[0]}…", ThemeManager.instance().get_colors()["text_secondary"])

        self._process = QProcess(self)
        self._process.setWorkingDirectory(self._project.path)
        self._process.readyReadStandardOutput.connect(self._on_stdout)
        self._process.readyReadStandardError.connect(self._on_stderr)
        self._process.finished.connect(self._on_finished)
        self._process.start("git", args)

    def _on_stdout(self):
        if self._process:
            text = bytes(self._process.readAllStandardOutput()).decode("utf-8", errors="replace").strip()
            if text:
                self.output.emit(text, False)

    def _on_stderr(self):
        if self._process:
            text = bytes(self._process.readAllStandardError()).decode("utf-8", errors="replace").strip()
            if text:
                self.output.emit(text, True)

    def _on_finished(self, exit_code, _status):
        name = self._running_cmd or "?"
        was_stash = name == "Stash"
        stdout_text = ""
        if self._process:
            stdout_text = bytes(self._process.readAllStandardOutput()).decode("utf-8", errors="replace")
        c = ThemeManager.instance().get_colors()

        if exit_code == 0:
            if was_stash and "No local changes" in stdout_text:
                self._show_result("Nothing to stash — working tree clean.", c["text_secondary"])
            else:
                self._show_result(f"{name} completed successfully.", c["success"])
        else:
            self._show_result(f"{name} failed (exit code {exit_code}). See Logs tab.", c["danger"])

        self.output.emit(f"[git {name.lower()}] exited with code {exit_code}", exit_code != 0)
        self.command_finished.emit(name, exit_code)

        self._process = None
        self._running_cmd = None
        self._set_buttons_enabled(True)
        self.refresh_info()

    def _show_result(self, text: str, color: str):
        self._result_label.setText(text)
        self._result_label.setStyleSheet(f"color: {color}; border: none;")
        self._result_frame.show()

    def _set_buttons_enabled(self, enabled: bool):
        has_repo = bool(self._project and is_git_repo(self._project.path))
        for b in (self._refresh_btn, self._pull_btn, self._fetch_btn, self._stash_btn):
            b.setEnabled(enabled and has_repo)

    def _show_empty_state(self, empty: bool):
        self._info_group.setVisible(not empty)
        self._actions_group.setVisible(not empty)
        self._result_frame.setVisible(not empty and not self._result_label.text() == "")
        self._empty_label.setVisible(empty)

    # ---------- Theme ----------

    def apply_theme(self, mode: ThemeMode):
        c = ThemeManager.instance().get_colors(mode)
        group_style = (
            f"QGroupBox {{ color: {c['text_secondary']}; font-weight: bold; "
            f"border: 1px solid {c['border']}; border-radius: 8px; margin-top: 10px; }} "
            f"QGroupBox::title {{ subcontrol-origin: margin; left: 12px; padding: 0 4px; }}"
        )
        self._info_group.setStyleSheet(group_style)
        self._actions_group.setStyleSheet(group_style)

        git_btn_style = (
            f"QPushButton {{ background-color: {c['primary']}; color: white; font-weight: 600; "
            f"border: none; border-radius: 8px; padding: 8px 14px; }} "
            f"QPushButton:hover {{ background-color: {c['primary_hover']}; }} "
            f"QPushButton:disabled {{ background-color: {c['border']}; color: {c['text_muted']}; }}"
        )
        for b in (self._refresh_btn, self._pull_btn, self._fetch_btn, self._stash_btn):
            b.setStyleSheet(git_btn_style)

        self._branch_label.setStyleSheet(f"color: {c['text_primary']}; border: none;")
        self._commit_label.setStyleSheet(f"color: {c['text_secondary']}; border: none;")
        self._empty_label.setStyleSheet(f"color: {c['text_muted']}; font-size: 13px; border: none;")
        self._result_frame.setStyleSheet(
            f"QFrame {{ background-color: {c['bg_surface']}; border: 1px solid {c['border']}; border-radius: 8px; }}"
        )
        self.refresh_info() if self._project else None
```

- [ ] **Step 2: Syntax verification**

Run: `.venv\Scripts\python -c "from PySide6.QtWidgets import QApplication; a=QApplication([]); from app.ui.widgets.git_panel import GitPanel; print('OK')"`
Expected: `OK`

---

### Task 10: MonitorPanel

**Files:**
- Create: `app/ui/widgets/monitor_panel.py`

**Interfaces:**
- Consumes: `Project`, `ServerState`.
- Produces: `MonitorPanel(QWidget)`:
  - Signals: `refresh_requested()`, `kill_requested(int)` (pid)
  - `set_projects(projects: list[Project])`
  - `update_port_rows(rows: list[dict])` — dicts: `{index, name, port, state}` donde state ∈ `"free"|"ours"|"foreign"`, más `owner_name: str, owner_pid: int` cuando foreign
  - `update_resources(rows: list[dict])` — `{name, pid, children, cpu, rss}`
  - `set_polling_enabled(bool)`
  - `apply_theme(mode)`
  - Auto-refresh: QTimer 3000ms que emite `refresh_requested` SOLO cuando el panel es visible (showEvent/hideEvent controlan) y polling habilitado.

- [ ] **Step 1: Write implementation**

```python
# app/ui/widgets/monitor_panel.py
from typing import Dict, List, Optional
from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QGroupBox, QPushButton,
    QCheckBox, QProgressBar, QScrollArea, QFrame, QSizePolicy
)
from PySide6.QtCore import Signal, Qt, QSize, QTimer
from PySide6.QtGui import QFont
from app.models.project import Project
from app.ui.icons import get_icon
from app.ui.theme import ThemeManager, ThemeMode, ui_font, mono_font

POLL_INTERVAL_MS = 3000


def _clear_layout(layout):
    while layout.count():
        item = layout.takeAt(0)
        w = item.widget()
        if w is not None:
            w.deleteLater()
        elif item.layout() is not None:
            _clear_layout(item.layout())


class PortRow(QFrame):
    kill_requested = Signal(int)

    def __init__(self, row: dict, parent=None):
        super().__init__(parent)
        self.setObjectName("portRow")
        lay = QHBoxLayout(self)
        lay.setContentsMargins(12, 8, 12, 8)
        lay.setSpacing(10)

        self._name_lbl = QLabel(row["name"])
        self._name_lbl.setFont(ui_font(13, QFont.Weight.DemiBold))
        self._name_lbl.setFixedWidth(150)
        lay.addWidget(self._name_lbl)

        self._port_lbl = QLabel(f":{row['port']}")
        self._port_lbl.setFont(mono_font(12))
        lay.addWidget(self._port_lbl)
        lay.addStretch()

        self._status_lbl = QLabel()
        lay.addWidget(self._status_lbl)

        self._kill_btn = QPushButton(" Kill")
        self._kill_btn.setIcon(get_icon("trash", "#ffffff"))
        self._kill_btn.setIconSize(QSize(13, 13))
        self._kill_btn.setFixedSize(80, 28)
        self._kill_btn.clicked.connect(lambda: self.kill_requested.emit(row.get("owner_pid", 0)))
        lay.addWidget(self._kill_btn)

        self.apply_row(row)

    def apply_row(self, row: dict):
        c = ThemeManager.instance().get_colors()
        state = row.get("state")
        if state == "free":
            self._status_lbl.setText("✓ Free")
            self._status_lbl.setStyleSheet(f"color: {c['text_muted']}; font-size: 12px;")
            self._kill_btn.hide()
        elif state == "ours":
            self._status_lbl.setText(f"◉ Used by '{row.get('owner_name', '')}'")
            self._status_lbl.setStyleSheet(
                f"color: {c['primary']}; background-color: {c['primary_light']}; "
                f"border-radius: 10px; padding: 3px 10px; font-size: 11px; font-weight: 600;"
            )
            self._kill_btn.hide()
        else:  # foreign
            self._status_lbl.setText(
                f"⚠ {row.get('owner_name', '?')} (PID {row.get('owner_pid', '?')})"
            )
            self._status_lbl.setStyleSheet(
                f"color: {c['warning']}; background-color: {c['warning_bg']}; "
                f"border-radius: 10px; padding: 3px 10px; font-size: 11px; font-weight: 600;"
            )
            self._kill_btn.show()


class ResourceRow(QFrame):
    def __init__(self, row: dict, parent=None):
        super().__init__(parent)
        self.setObjectName("resRow")
        lay = QHBoxLayout(self)
        lay.setContentsMargins(12, 8, 12, 8)
        lay.setSpacing(10)

        self._name_lbl = QLabel(row["name"])
        self._name_lbl.setFont(ui_font(13, QFont.Weight.DemiBold))
        self._name_lbl.setFixedWidth(150)
        lay.addWidget(self._name_lbl)

        self._pid_lbl = QLabel(f"PID {row['pid']}" + (f" (+{row['children']})" if row.get("children") else ""))
        self._pid_lbl.setFont(mono_font(11))
        self._pid_lbl.setFixedWidth(110)
        lay.addWidget(self._pid_lbl)

        self._bar = QProgressBar()
        self._bar.setRange(0, 100)
        self._bar.setTextVisible(False)
        self._bar.setFixedHeight(8)
        lay.addWidget(self._bar, stretch=1)

        self._usage_lbl = QLabel()
        self._usage_lbl.setFont(mono_font(12))
        self._usage_lbl.setFixedWidth(170)
        lay.addWidget(self._usage_lbl)

        self.apply_row(row)

    def apply_row(self, row: dict):
        c = ThemeManager.instance().get_colors()
        cpu = float(row.get("cpu", 0.0))
        rss = float(row.get("rss", 0.0))
        display_cpu = min(cpu, 100.0)
        self._bar.setValue(int(display_cpu))
        if display_cpu >= 80:
            color = c["danger"]
        elif display_cpu >= 50:
            color = c["warning"]
        else:
            color = c["success"]
        self._bar.setStyleSheet(
            f"QProgressBar {{ background-color: {c['bg_elevated']}; border: none; border-radius: 4px; }} "
            f"QProgressBar::chunk {{ background-color: {color}; border-radius: 4px; }}"
        )
        self._usage_lbl.setText(f"{cpu:>5.1f}%  {rss:>7.1f} MB")


class MonitorPanel(QWidget):
    refresh_requested = Signal()
    kill_requested = Signal(int)  # pid

    def __init__(self, parent=None):
        super().__init__(parent)
        self._projects: List[Project] = []
        self._polling_enabled = True

        self._timer = QTimer(self)
        self._timer.setInterval(POLL_INTERVAL_MS)
        self._timer.timeout.connect(lambda: self.refresh_requested.emit())

        self._setup_ui()
        self.apply_theme(ThemeManager.instance().mode)

    def _setup_ui(self):
        outer = QVBoxLayout(self)
        outer.setContentsMargins(14, 14, 14, 14)
        outer.setSpacing(12)

        # Toolbar
        bar = QHBoxLayout()
        self._refresh_btn = QPushButton(" Refresh")
        self._refresh_btn.setIcon(get_icon("restart", "#ffffff"))
        self._refresh_btn.setIconSize(QSize(14, 14))
        self._refresh_btn.setFixedHeight(34)
        self._refresh_btn.clicked.connect(self.refresh_requested.emit)
        bar.addWidget(self._refresh_btn)

        self._auto_cb = QCheckBox("Auto-refresh (3s)")
        self._auto_cb.setChecked(True)
        self._auto_cb.toggled.connect(self._on_auto_toggled)
        bar.addWidget(self._auto_cb)
        bar.addStretch()

        self._summary_lbl = QLabel("")
        bar.addWidget(self._summary_lbl)
        outer.addLayout(bar)

        # Scrollable content
        scroll = QScrollArea()
        scroll.setWidgetResizable(True)
        scroll.setFrameShape(QFrame.Shape.NoFrame)
        content = QWidget()
        content_layout = QVBoxLayout(content)
        content_layout.setContentsMargins(0, 0, 0, 0)
        content_layout.setSpacing(12)

        self._ports_group = QGroupBox("Configured Ports")
        self._ports_layout = QVBoxLayout(self._ports_group)
        self._ports_layout.setContentsMargins(14, 16, 14, 14)
        self._ports_layout.setSpacing(6)
        content_layout.addWidget(self._ports_group)

        self._res_group = QGroupBox("Resources (Running Servers)")
        res_outer = QVBoxLayout(self._res_group)
        res_outer.setContentsMargins(14, 16, 14, 14)
        self._disabled_lbl = QLabel("Resource polling disabled in Settings.")
        self._disabled_lbl.setAlignment(Qt.AlignmentFlag.AlignCenter)
        res_outer.addWidget(self._disabled_lbl)
        self._res_layout = QVBoxLayout()
        self._res_layout.setSpacing(6)
        res_outer.addLayout(self._res_layout)
        content_layout.addWidget(self._res_group)
        content_layout.addStretch()

        scroll.setWidget(content)
        outer.addWidget(scroll, stretch=1)

    # ---------- Visibility-gated polling ----------

    def showEvent(self, event):
        self._update_timer_state()
        super().showEvent(event)

    def hideEvent(self, event):
        self._timer.stop()
        super().hideEvent(event)

    def set_polling_enabled(self, enabled: bool):
        self._polling_enabled = enabled
        self._disabled_lbl.setVisible(not enabled)
        self._update_timer_state()

    def _on_auto_toggled(self, checked: bool):
        self._update_timer_state()

    def _update_timer_state(self):
        should_run = (
            self.isVisible()
            and self._polling_enabled
            and self._auto_cb.isChecked()
        )
        if should_run and not self._timer.isActive():
            self.refresh_requested.emit()  # refresh inmediato al activarse
            self._timer.start()
        elif not should_run and self._timer.isActive():
            self._timer.stop()

    # ---------- Data ----------

    def set_projects(self, projects: List[Project]):
        self._projects = projects

    def update_port_rows(self, rows: List[dict]):
        _clear_layout(self._ports_layout)
        if not rows:
            empty = QLabel("No projects with servers configured.")
            empty.setAlignment(Qt.AlignmentFlag.AlignCenter)
            self._ports_layout.addWidget(empty)
            return
        for row in rows:
            port_row = PortRow(row)
            port_row.kill_requested.connect(self.kill_requested.emit)
            self._ports_layout.addWidget(port_row)
        conflicts = sum(1 for r in rows if r.get("state") == "foreign")
        running = sum(1 for r in rows if r.get("state") == "ours")
        self._summary_lbl.setText(f"{len(rows)} ports · {running} in use · {conflicts} conflict(s)")

    def update_resources(self, rows: List[dict]):
        _clear_layout(self._res_layout)
        if not rows:
            empty = QLabel("No servers running.")
            empty.setAlignment(Qt.AlignmentFlag.AlignCenter)
            self._res_layout.addWidget(empty)
            return
        for row in rows:
            self._res_layout.addWidget(ResourceRow(row))

    # ---------- Theme ----------

    def apply_theme(self, mode: ThemeMode):
        c = ThemeManager.instance().get_colors(mode)
        group_style = (
            f"QGroupBox {{ color: {c['text_secondary']}; font-weight: bold; "
            f"border: 1px solid {c['border']}; border-radius: 8px; margin-top: 10px; }} "
            f"QGroupBox::title {{ subcontrol-origin: margin; left: 12px; padding: 0 4px; }}"
        )
        self._ports_group.setStyleSheet(group_style)
        self._res_group.setStyleSheet(group_style)
        self._refresh_btn.setStyleSheet(
            f"QPushButton {{ background-color: {c['primary']}; color: white; font-weight: 600; "
            f"border: none; border-radius: 8px; padding: 6px 14px; }} "
            f"QPushButton:hover {{ background-color: {c['primary_hover']}; }}"
        )
        for cb in (self._auto_cb,):
            cb.setStyleSheet(f"QCheckBox {{ color: {c['text_secondary']}; font-size: 12px; }}")
        self._summary_lbl.setStyleSheet(f"color: {c['text_muted']}; font-size: 12px;")
        self._disabled_lbl.setStyleSheet(f"color: {c['warning']}; font-size: 12px;")
        self._scroll_widget_style(c)

    def _scroll_widget_style(self, c):
        scroll = self.findChild(QScrollArea)
        if scroll:
            scroll.setStyleSheet(f"QScrollArea {{ background-color: transparent; border: none; }}")
        for frame in self.findChildren(QFrame):
            if frame.objectName() in ("portRow", "resRow"):
                frame.setStyleSheet(
                    f"QFrame#{frame.objectName()} {{ background-color: {c['bg_surface']}; "
                    f"border: 1px solid {c['border']}; border-radius: 8px; }}"
                )
```

- [ ] **Step 2: Syntax verification**

Run: `.venv\Scripts\python -c "from PySide6.QtWidgets import QApplication; a=QApplication([]); from app.ui.widgets.monitor_panel import MonitorPanel; print('OK')" `
Expected: `OK`

---

### Task 11: EvidencePanel

**Files:**
- Create: `app/ui/widgets/evidence_panel.py`

**Interfaces:**
- Consumes: `scan_evidence`, `find_html_report`, `EvidenceFile`.
- Produces: `EvidencePanel(QWidget)`:
  - Signal: `trace_open_requested(str)` (path del .zip)
  - `set_project(project: Optional[Project])` — re-escanea
  - `apply_theme(mode)`
  - Thumbnails lazy: primer lote 20 sync, resto por QTimer en lotes de 20.

- [ ] **Step 1: Write implementation**

```python
# app/ui/widgets/evidence_panel.py
import os
from typing import List, Optional
from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QPushButton, QListWidget,
    QListWidgetItem, QSplitter, QMenu, QFrame
)
from PySide6.QtCore import Signal, Qt, QSize, QTimer, QUrl
from PySide6.QtGui import QPixmap, QAction, QDesktopServices
from app.models.project import Project
from app.utils.evidence import scan_evidence, find_html_report, EvidenceFile
from app.ui.icons import get_icon
from app.ui.theme import ThemeManager, ThemeMode, ui_font

THUMB_SIZE = 180
BATCH_SIZE = 20


class EvidencePanel(QWidget):
    trace_open_requested = Signal(str)  # ruta absoluta del .zip

    def __init__(self, parent=None):
        super().__init__(parent)
        self._project: Optional[Project] = None
        self._files: List[EvidenceFile] = []
        self._pending_thumbs: List[tuple] = []  # (QListWidgetItem, EvidenceFile)
        self._thumb_timer = QTimer(self)
        self._thumb_timer.setInterval(30)
        self._thumb_timer.timeout.connect(self._load_thumb_batch)
        self._setup_ui()
        self.apply_theme(ThemeManager.instance().mode)

    def _setup_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(14, 14, 14, 14)
        layout.setSpacing(10)

        # Toolbar
        bar = QHBoxLayout()
        self._refresh_btn = QPushButton(" Refresh")
        self._refresh_btn.setIcon(get_icon("restart", "#ffffff"))
        self._refresh_btn.setIconSize(QSize(14, 14))
        self._refresh_btn.setFixedHeight(34)
        self._refresh_btn.clicked.connect(self.reload)
        bar.addWidget(self._refresh_btn)

        self._report_btn = QPushButton(" Open HTML Report")
        self._report_btn.setIcon(get_icon("report", "#ffffff"))
        self._report_btn.setIconSize(QSize(14, 14))
        self._report_btn.setFixedHeight(34)
        self._report_btn.clicked.connect(self._open_report)
        bar.addWidget(self._report_btn)

        bar.addStretch()
        self._count_lbl = QLabel("")
        bar.addWidget(self._count_lbl)
        layout.addLayout(bar)

        # Splitter gallery | preview
        self._splitter = QSplitter(Qt.Orientation.Horizontal)

        self._gallery = QListWidget()
        self._gallery.setViewMode(QListWidget.ViewMode.IconMode)
        self._gallery.setIconSize(QSize(THUMB_SIZE, THUMB_SIZE))
        self._gallery.setResizeMode(QListWidget.ResizeMode.Adjust)
        self._gallery.setMovement(QListWidget.Movement.Static)
        self._gallery.setSpacing(10)
        self._gallery.setContextMenuPolicy(Qt.ContextMenuPolicy.CustomContextMenu)
        self._gallery.customContextMenuRequested.connect(self._context_menu)
        self._gallery.itemDoubleClicked.connect(self._on_item_activated)
        self._splitter.addWidget(self._gallery)

        preview_container = QFrame()
        preview_layout = QVBoxLayout(preview_container)
        preview_layout.setContentsMargins(0, 0, 0, 0)
        self._preview = QLabel("Select an image to preview")
        self._preview.setAlignment(Qt.AlignmentFlag.AlignCenter)
        self._preview.setMinimumWidth(280)
        preview_layout.addWidget(self._preview)
        self._splitter.addWidget(preview_container)
        self._splitter.setStretchFactor(0, 3)
        self._splitter.setStretchFactor(1, 2)

        layout.addWidget(self._splitter, stretch=1)

        # Empty state
        self._empty_label = QLabel("No evidence found.\nRun Playwright tests to generate screenshots, videos and traces.")
        self._empty_label.setAlignment(Qt.AlignmentFlag.AlignCenter)
        layout.addWidget(self._empty_label)

    # ---------- Data loading ----------

    def set_project(self, project: Optional[Project]):
        self._project = project
        self.reload()

    def reload(self):
        self._gallery.clear()
        self._preview.setText("Select an image to preview")
        self._pending_thumbs.clear()
        self._thumb_timer.stop()
        self._files = []

        if not self._project or not os.path.isdir(self._project.path):
            self._update_visibility()
            return

        self._files = scan_evidence(self._project.path)

        image_icon = get_icon("image", "#64748b")
        film_icon = get_icon("film", "#64748b")
        archive_icon = get_icon("archive", "#f59e0b")

        for f in self._files[:BATCH_SIZE]:
            self._add_item(f, image_icon, film_icon, archive_icon, load_thumb=True)
        for f in self._files[BATCH_SIZE:]:
            self._add_item(f, image_icon, film_icon, archive_icon, load_thumb=False)

        if self._files:
            self._thumb_timer.start()

        report = find_html_report(self._project.path) if self._project else None
        self._report_path = report
        self._report_btn.setEnabled(bool(report))

        self._count_lbl.setText(f"{len(self._files)} artifact(s)")
        self._update_visibility()

    def _add_item(self, f: EvidenceFile, image_icon, film_icon, archive_icon, load_thumb: bool):
        short = f.rel_path.replace("\\", "/")
        if len(short) > 42:
            short = "…" + short[-41:]
        item = QListWidgetItem(short)
        item.setData(Qt.ItemDataRole.UserRole, f)
        item.setToolTip(f.rel_path)
        if f.kind == "image":
            if load_thumb:
                pix = QPixmap(f.path)
                if not pix.isNull():
                    item.setIcon(pix.scaled(THUMB_SIZE, THUMB_SIZE, Qt.AspectRatioMode.KeepAspectRatio, Qt.TransformationMode.SmoothTransformation))
                    self._gallery.addItem(item)
                    return
            item.setIcon(image_icon)
        elif f.kind == "video":
            item.setIcon(film_icon)
        else:
            item.setIcon(archive_icon)
        self._gallery.addItem(item)
        if f.kind == "image" and not load_thumb:
            self._pending_thumbs.append((item, f))

    def _load_thumb_batch(self):
        batch = self._pending_thumbs[:BATCH_SIZE]
        del self._pending_thumbs[:BATCH_SIZE]
        for item, f in batch:
            pix = QPixmap(f.path)
            if pix.isNull():
                continue
            item.setIcon(pix.scaled(THUMB_SIZE, THUMB_SIZE, Qt.AspectRatioMode.KeepAspectRatio, Qt.TransformationMode.SmoothTransformation))
        if not self._pending_thumbs:
            self._thumb_timer.stop()

    # ---------- Interactions ----------

    def _current_file(self) -> Optional[EvidenceFile]:
        item = self._gallery.currentItem()
        return item.data(Qt.ItemDataRole.UserRole) if item else None

    def _on_item_activated(self, item: QListWidgetItem):
        f: EvidenceFile = item.data(Qt.ItemDataRole.UserRole)
        if f.kind == "image":
            pix = QPixmap(f.path)
            if not pix.isNull():
                self._preview.setPixmap(pix.scaled(
                    self._preview.width() - 8, self._preview.height() - 8,
                    Qt.AspectRatioMode.KeepAspectRatio, Qt.TransformationMode.SmoothTransformation
                ))
        elif f.kind == "video":
            QDesktopServices.openUrl(QUrl.fromLocalFile(f.path))
        else:
            self.trace_open_requested.emit(f.path)

    def _context_menu(self, pos):
        item = self._gallery.itemAt(pos)
        if not item:
            return
        f: EvidenceFile = item.data(Qt.ItemDataRole.UserRole)
        menu = QMenu(self)
        open_act = QAction(get_icon("external-link"), "Open Externally", self)
        open_act.triggered.connect(lambda: QDesktopServices.openUrl(QUrl.fromLocalFile(f.path)))
        menu.addAction(open_act)

        folder_act = QAction(get_icon("folder"), "Open Containing Folder", self)
        folder_act.triggered.connect(
            lambda: QDesktopServices.openUrl(QUrl.fromLocalFile(f.test_dir))
        )
        menu.addAction(folder_act)

        if f.kind == "trace":
            menu.addSeparator()
            trace_act = QAction(get_icon("bug"), "Show Trace Viewer", self)
            trace_act.triggered.connect(lambda: self.trace_open_requested.emit(f.path))
            menu.addAction(trace_act)

        menu.exec(self._gallery.mapToGlobal(pos))

    def _open_report(self):
        if getattr(self, "_report_path", None):
            QDesktopServices.openUrl(QUrl.fromLocalFile(self._report_path))

    def _update_visibility(self):
        has = len(self._files) > 0
        self._splitter.setVisible(has)
        self._empty_label.setVisible(not has)

    # ---------- Theme ----------

    def apply_theme(self, mode: ThemeMode):
        c = ThemeManager.instance().get_colors(mode)
        self._refresh_btn.setStyleSheet(
            f"QPushButton {{ background-color: {c['primary']}; color: white; font-weight: 600; "
            f"border: none; border-radius: 8px; padding: 6px 14px; }} "
            f"QPushButton:hover {{ background-color: {c['primary_hover']}; }} "
            f"QPushButton:disabled {{ background-color: {c['border']}; color: {c['text_muted']}; }}"
        )
        self._report_btn.setStyleSheet(self._refresh_btn.styleSheet())
        self._count_lbl.setStyleSheet(f"color: {c['text_muted']}; font-size: 12px;")
        self._gallery.setStyleSheet(
            f"QListWidget {{ background-color: {c['bg_surface']}; border: 1px solid {c['border']}; "
            f"border-radius: 8px; outline: none; padding: 8px; color: {c['text_primary']}; font-size: 11px; }} "
            f"QListWidget::item:selected {{ background-color: {c['bg_active']}; border-radius: 6px; }}"
        )
        self._preview.setStyleSheet(
            f"QLabel {{ background-color: {c['terminal_bg']}; color: {c['text_muted']}; "
            f"border: 1px solid {c['border']}; border-radius: 8px; }}"
        )
        self._empty_label.setStyleSheet(f"color: {c['text_muted']}; font-size: 13px;")
```

- [ ] **Step 2: Syntax verification**

Run: `.venv\Scripts\python -c "from PySide6.QtWidgets import QApplication; a=QApplication([]); from app.ui.widgets.evidence_panel import EvidencePanel; print('OK')" `
Expected: `OK`

---

### Task 12: Chips de filtro de estado en sidebar

**Files:**
- Modify: `app/ui/widgets/project_list.py`

**Interfaces:**
- Consumes: `_states_cache` existente, `_filter_projects(query)` existente.
- Produces: `self._status_filter: str` ∈ `"all"|"running"|"stopped"`; chips All/Running/Stopped bajo la búsqueda; refiltro automático en `update_status`.

- [ ] **Step 1: Insertar chips en `_setup_ui`**

Después de `layout.addWidget(self._search_input)` (línea ~62), insertar:

```python
        # Status filter chips (All / Running / Stopped)
        self._status_filter = "all"
        filter_row = QHBoxLayout()
        filter_row.setSpacing(6)
        self._chip_buttons = {}
        for key, label in (("all", "All"), ("running", "Running"), ("stopped", "Stopped")):
            chip = QPushButton(label)
            chip.setCheckable(True)
            chip.setFixedHeight(26)
            chip.clicked.connect(lambda _c, k=key: self._on_status_chip(k))
            filter_row.addWidget(chip, stretch=1)
            self._chip_buttons[key] = chip
        self._chip_buttons["all"].setChecked(True)
        layout.addLayout(filter_row)
```

- [ ] **Step 2: Handler + predicado combinado**

Agregar métodos (después de `_filter_projects`):

```python
    def _on_status_chip(self, key: str):
        self._status_filter = key
        for k, chip in self._chip_buttons.items():
            chip.setChecked(k == key)
        self._filter_projects(self._search_input.text())

    def _matches_status_filter(self, state: ServerState) -> bool:
        if self._status_filter == "running":
            return state == ServerState.RUNNING
        if self._status_filter == "stopped":
            return state != ServerState.RUNNING
        return True
```

Modificar el comprehension de `matching_indices` en `_filter_projects` (líneas ~140-143):

ANTES:
```python
        matching_indices = [
            idx for idx, project in enumerate(self._projects_cache)
            if not query or query in project.name.lower() or query in project.path.lower()
        ]
```
DESPUÉS:
```python
        matching_indices = [
            idx for idx, project in enumerate(self._projects_cache)
            if (not query or query in project.name.lower() or query in project.path.lower())
            and self._matches_status_filter(self._states_cache.get(idx, ServerState.STOPPED))
        ]
```

- [ ] **Step 3: Refiltro automático en `update_status`**

Al final de `update_status` (después del `break` del for-loop, fuera del loop), agregar:

```python
        if self._status_filter != "all":
            self._filter_projects(self._search_input.text())
```

- [ ] **Step 4: Estilos en `apply_theme`**

Al final de `apply_theme`, agregar:

```python
        chip_checked = (
            f"QPushButton {{ background-color: {c['primary_light']}; color: {c['primary']}; "
            f"font-size: 11px; font-weight: 700; border: 1px solid {c['primary']}; border-radius: 13px; padding: 2px; }}"
        )
        chip_unchecked = (
            f"QPushButton {{ background-color: {c['bg_elevated']}; color: {c['text_secondary']}; "
            f"font-size: 11px; font-weight: 600; border: 1px solid {c['border']}; border-radius: 13px; padding: 2px; }}"
            f"QPushButton:hover {{ border-color: {c['border_focus']}; }}"
        )
        for key, chip in self._chip_buttons.items():
            chip.setStyleSheet(chip_checked if chip.isChecked() else chip_unchecked)
```

- [ ] **Step 5: Verificar suite**

Run: `.venv\Scripts\python -m pytest tests/ -v`
Expected: PASS (sin regresiones; sidebar no tiene tests unitarios, verificación manual viene en Task 15)

---

### Task 13: Checkbox "Errors only" en LogPanel

**Files:**
- Modify: `app/ui/widgets/log_panel.py`

**Interfaces:**
- Produces: `self._errors_only: bool`; checkbox entre Wrap Lines y Auto-scroll; aplica a ambos usos (Logs + App Log).

- [ ] **Step 1: Crear checkbox en `_setup_ui`**

Entre el bloque `self._wrap_cb` (línea ~43) y `self._autoscroll_btn` (línea ~45), insertar:

```python
        self._errors_only = False
        self._errors_only_cb = QCheckBox("Errors only")
        self._errors_only_cb.setChecked(False)
        self._errors_only_cb.toggled.connect(self._toggle_errors_only)
        top_bar.addWidget(self._errors_only_cb)
```

- [ ] **Step 2: Gate en append_log**

En `append_log`, cambiar la condición (línea ~127):

ANTES:
```python
            query = self._filter_input.text().strip().lower()
            if not query or query in formatted.lower():
                self._text_edit.appendPlainText(formatted)
```
DESPUÉS:
```python
            query = self._filter_input.text().strip().lower()
            if (not query or query in formatted.lower()) and not self._errors_only:
                self._text_edit.appendPlainText(formatted)
```

- [ ] **Step 3: Predicado combinado en `_apply_filter`**

Cambiar (línea ~176):

ANTES:
```python
        for formatted, is_error in self._raw_lines:
            if not query or query in formatted.lower():
```
DESPUÉS:
```python
        for formatted, is_error in self._raw_lines:
            if (not query or query in formatted.lower()) and (not self._errors_only or is_error):
```

- [ ] **Step 4: Toggle handler + estilo**

Agregar método (junto a `_toggle_wrap`):

```python
    def _toggle_errors_only(self, checked: bool):
        self._errors_only = checked
        self._apply_filter(self._filter_input.text())
```

Y en `apply_theme`, junto al estilo de `_wrap_cb` (línea ~95):

```python
        self._errors_only_cb.setStyleSheet(f"color: {c['text_secondary']}; font-size: 12px; font-weight: 500;")
```

- [ ] **Step 5: Verificar suite**

Run: `.venv\Scripts\python -m pytest tests/ -v`
Expected: PASS

---

### Task 14: Label de uptime en ServerPanel

**Files:**
- Modify: `app/ui/widgets/server_panel.py`

**Interfaces:**
- Produces: función módulo-level `format_uptime(seconds: Optional[float]) -> str` (`None/negativo → "—"`, formato `1h 23m 45s` / `4m 12s` / `45s`); método `set_uptime(seconds: Optional[float])`; reset automático en `update_state(STOPPED)`.

- [ ] **Step 1: Función formateadora + import**

Arriba del class `ServerPanel`, agregar:

```python
from typing import Optional


def format_uptime(seconds: Optional[float]) -> str:
    """Formats elapsed seconds as '1h 23m 45s' / '4m 12s' / '45s'. None → '—'."""
    if seconds is None or seconds < 0:
        return "—"
    seconds = int(seconds)
    hours, rem = divmod(seconds, 3600)
    minutes, secs = divmod(rem, 60)
    if hours:
        return f"{hours}h {minutes}m {secs}s"
    if minutes:
        return f"{minutes}m {secs}s"
    return f"{secs}s"
```

- [ ] **Step 2: Label en `_setup_ui`**

Después de `info_layout.addWidget(self._status_label)` (línea ~63):

```python
        self._uptime_label = QLabel("Uptime: —")
        self._uptime_label.setFont(ui_font(12))
        info_layout.addWidget(self._uptime_label)
```

- [ ] **Step 3: Métodos públicos**

Agregar junto a `update_state`:

```python
    def set_uptime(self, seconds: Optional[float]):
        self._uptime_label.setText(f"Uptime: {format_uptime(seconds)}")

    def _clear_uptime(self):
        self.set_uptime(None)
```

Y dentro de `update_state`, en el bloque `if state == ServerState.STOPPED:` (línea ~188) agregar:

```python
        if state == ServerState.STOPPED:
            self._clear_uptime()
            self._sync_port_btn.setVisible(False)
```

Estilo en `apply_theme` (junto a `_path_label` implícito — agregar al final):

```python
        self._uptime_label.setStyleSheet(f"color: {c['text_secondary']}; font-size: 12px;")
```

- [ ] **Step 4: Verificar suite**

Run: `.venv\Scripts\python -m pytest tests/ -v`
Expected: PASS

---

### Task 15: Integración en MainWindow

**Files:**
- Modify: `app/ui/main_window.py`

**Interfaces:**
- Consumes: TODO lo anterior.
- Produces: tabs finales en orden: Server / Playwright / Evidence / Scripts / Git / Monitor / Logs / App Log. Menú File → Settings (Ctrl+,). Helper `_notify()`. Timer uptime 1s. Orquestación Monitor. Trace viewer runner.

- [ ] **Step 1: Imports**

Agregar al bloque de imports de `app.ui.*`:

```python
from app.config.settings import AppSettings, KEY_POLLING_ENABLED, KEY_TOASTS_ENABLED
from app.ui.settings_dialog import SettingsDialog
from app.ui.widgets.evidence_panel import EvidencePanel
from app.ui.widgets.git_panel import GitPanel
from app.ui.widgets.monitor_panel import MonitorPanel
from app.ui.widgets.toast import ToastManager, ToastLevel
from app.process import monitor as sysmon
```

- [ ] **Step 2: Estado en `__init__`**

Después de `self.theme_manager = ThemeManager.instance()`:

```python
        self.app_settings = AppSettings.instance()
```

Después de `self._is_force_exit: bool = False`:

```python
        self._toast_manager: Optional[ToastManager] = None  # creado en _setup_ui
        self._trace_runner = None
        self._uptime_timer: Optional[QTimer] = None
```

Agregar `QTimer` al import existente de `QtCore` (ya está: `Qt, QUrl, QSize, QProcess` → añadir `QTimer`).

- [ ] **Step 3: Tabs en `_setup_ui`**

Reemplazar el bloque de tabs (líneas ~180-192):

ANTES:
```python
        self._tab_widget.addTab(self._server_panel, get_icon("server", "#6366f1"), "  Server  ")
        self._tab_widget.addTab(self._playwright_panel, get_icon("flask", "#6366f1"), "  Playwright  ")
        self._tab_widget.addTab(self._scripts_panel, get_icon("cpu-bolt", "#6366f1"), "  Scripts  ")
        self._tab_widget.addTab(self._log_panel, get_icon("terminal", "#6366f1"), "  Logs  ")
```
DESPUÉS:
```python
        self._evidence_panel = EvidencePanel()
        self._git_panel = GitPanel()
        self._monitor_panel = MonitorPanel()

        self._tab_widget.addTab(self._server_panel, get_icon("server", "#6366f1"), "  Server  ")
        self._tab_widget.addTab(self._playwright_panel, get_icon("flask", "#6366f1"), "  Playwright  ")
        self._tab_widget.addTab(self._evidence_panel, get_icon("image", "#6366f1"), "  Evidence  ")
        self._tab_widget.addTab(self._scripts_panel, get_icon("cpu-bolt", "#6366f1"), "  Scripts  ")
        self._tab_widget.addTab(self._git_panel, get_icon("git-branch", "#6366f1"), "  Git  ")
        self._tab_widget.addTab(self._monitor_panel, get_icon("activity", "#6366f1"), "  Monitor  ")
        self._tab_widget.addTab(self._log_panel, get_icon("terminal", "#6366f1"), "  Logs  ")
```

- [ ] **Step 4: Conexiones paneles nuevos (final de `_setup_ui`, junto a las existentes ~línea 217)**

```python
        self._git_panel.output.connect(
            lambda msg, err: self._append_project_log(self._current_index, msg, is_error=err)
        )
        self._git_panel.command_finished.connect(self._on_git_command_finished)
        self._monitor_panel.refresh_requested.connect(self._refresh_monitor_data)
        self._monitor_panel.kill_requested.connect(self._on_monitor_kill)
        self._evidence_panel.trace_open_requested.connect(self._open_trace_viewer)
```

Y al final de `_setup_ui` (antes del status bar), crear toast manager + uptime timer:

```python
        self._toast_manager = ToastManager(self)

        self._uptime_timer = QTimer(self)
        self._uptime_timer.setInterval(1000)
        self._uptime_timer.timeout.connect(self._update_uptime)
        self._uptime_timer.start()
```

- [ ] **Step 5: Menú Settings**

En `_setup_menu_and_toolbar`, dentro de File Menu, después de `file_menu.addAction(remove_action)`:

```python
        settings_action = QAction(get_icon("settings", "#94a3b8"), "&Settings...", self)
        settings_action.setShortcut("Ctrl+,")
        settings_action.setStatusTip("Open application settings (Ctrl+,)")
        settings_action.triggered.connect(self._on_open_settings)
        file_menu.addAction(settings_action)
```

- [ ] **Step 6: Handlers nuevos (bloque al final de la clase, antes de closeEvent)**

```python
    # ---------------- Settings ----------------

    def _on_open_settings(self):
        dialog = SettingsDialog(self)
        dialog.exec()
        # Los toggles persisten/aplican en vivo vía AppSettings.setting_changed

    # ---------------- Notifications routing ----------------

    def _notify(self, title: str, message: str, level: ToastLevel = ToastLevel.INFO):
        """Visible window → in-app toast. Hidden/minimized → tray notification."""
        if self.isVisible():
            self._toast_manager.show(title, message, level)
        else:
            is_error = level in (ToastLevel.ERROR, ToastLevel.WARNING)
            self._tray.show_notification(title, message, is_error=is_error)

    # ---------------- Uptime ----------------

    def _update_uptime(self):
        idx = self._current_index
        sm = self._server_managers.get(idx) if idx >= 0 else None
        if sm and sm.state == ServerState.RUNNING and sm.started_at:
            self._server_panel.set_uptime(time.time() - sm.started_at)
        else:
            self._server_panel.set_uptime(None)

    # ---------------- Git ----------------

    def _on_git_command_finished(self, name: str, exit_code: int):
        project = self.config_manager.get_project(self._current_index)
        pname = project.name if project else "project"
        if exit_code == 0:
            self._status_bar.showMessage(f"Git {name.lower()} completed for '{pname}'", 4000)
        else:
            self._notify(pname, f"Git {name.lower()} failed (exit {exit_code})", ToastLevel.ERROR)

    # ---------------- Monitor ----------------

    def _refresh_monitor_data(self):
        projects = self.config_manager.get_projects()

        # Ports
        port_rows = []
        running_ports: dict[int, str] = {}  # port → project name (de managers RUNNING)
        for idx, sm in self._server_managers.items():
            if sm.state == ServerState.RUNNING and sm.active_port > 0:
                proj = projects[idx] if idx < len(projects) else None
                running_ports[sm.active_port] = proj.name if proj else f"#{idx}"

        for idx, project in enumerate(projects):
            if not project.server.enabled or project.server.port <= 0:
                continue
            port = project.server.port
            if port in running_ports:
                port_rows.append({"index": idx, "name": project.name, "port": port,
                                  "state": "ours", "owner_name": running_ports[port]})
                continue
            owner = sysmon.get_port_owner(port)
            if owner:
                port_rows.append({"index": idx, "name": project.name, "port": port,
                                  "state": "foreign", "owner_name": owner.name, "owner_pid": owner.pid})
            else:
                port_rows.append({"index": idx, "name": project.name, "port": port, "state": "free"})
        self._monitor_panel.update_port_rows(port_rows)

        # Resources
        res_rows = []
        if self.app_settings.get(KEY_POLLING_ENABLED, True):
            for idx, sm in self._server_managers.items():
                if sm.state != ServerState.RUNNING:
                    continue
                pid = sm._runner.pid() if hasattr(sm, "_runner") else 0
                if pid <= 0:
                    continue
                usage = sysmon.get_process_tree_usage(pid)
                proj = projects[idx] if idx < len(projects) else None
                if usage:
                    res_rows.append({
                        "name": proj.name if proj else f"#{idx}",
                        "pid": usage.pid, "children": usage.children,
                        "cpu": usage.cpu_percent, "rss": usage.rss_mb,
                    })
        self._monitor_panel.update_resources(res_rows)

    def _on_monitor_kill(self, pid: int):
        if pid <= 0:
            return
        reply = QMessageBox.question(
            self, "Kill Process",
            f"Force-terminate process tree with PID {pid}?\n\nThis cannot be undone.",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
            QMessageBox.StandardButton.No,
        )
        if reply != QMessageBox.StandardButton.Yes:
            return
        ok, msg = sysmon.kill_tree(pid)
        project = self.config_manager.get_project(self._current_index)
        pname = project.name if project else "System"
        if ok:
            self._notify(pname, f"Process tree {pid} terminated", ToastLevel.SUCCESS)
            self._append_project_log(self._current_index, f"⚠ Killed process tree PID {pid}: {msg}")
        else:
            self._notify(pname, f"Failed killing PID {pid}: {msg}", ToastLevel.ERROR)
            self._append_project_log(self._current_index, f"Failed killing PID {pid}: {msg}", is_error=True)
        self._refresh_monitor_data()

    # ---------------- Evidence / Trace viewer ----------------

    def _open_trace_viewer(self, trace_path: str):
        project = self.config_manager.get_project(self._current_index)
        if not project:
            return
        if self._trace_runner is not None and self._trace_runner.is_running():
            self._append_project_log(self._current_index, "Trace viewer already open", is_error=True)
            return
        from app.process.runner import ProcessRunner
        self._trace_runner = ProcessRunner(self)
        self._trace_runner.output_ready.connect(
            lambda m: self._append_project_log(self._current_index, m)
        )
        self._trace_runner.error_ready.connect(
            lambda m: self._append_project_log(self._current_index, m, is_error=True)
        )
        self._append_project_log(self._current_index, f"🔍 Opening trace viewer: {trace_path}")
        self._trace_runner.start(f'npx playwright show-trace "{trace_path}"', project.path)
        self._status_bar.showMessage("Opening Playwright Trace Viewer...", 4000)
```

Agregar `import time` arriba (con los imports stdlib).

- [ ] **Step 7: Routing de notificaciones existentes → `_notify`**

Reemplazar en `_on_server_state_changed` (líneas ~750-758):

ANTES:
```python
                if state == ServerState.RUNNING:
                    sm = self._server_managers.get(idx)
                    port = sm.active_port if sm else project.server.port
                    self._tray.show_notification(project.name, f"Server running on port {port}")
                else:
                    self._tray.show_notification(project.name, "Server failed to start", is_error=True)
```
DESPUÉS:
```python
                if state == ServerState.RUNNING:
                    sm = self._server_managers.get(idx)
                    port = sm.active_port if sm else project.server.port
                    self._notify(project.name, f"Server running on port {port}", ToastLevel.SUCCESS)
                else:
                    self._notify(project.name, "Server failed to start", ToastLevel.ERROR)
```

En `_on_playwright_state_changed` (líneas ~814-820):

ANTES:
```python
        if state in (PlaywrightState.FAILED, PlaywrightState.ERROR):
            project = self.config_manager.get_project(idx)
            if project:
                msg = "Test run failed" if state == PlaywrightState.FAILED else "Playwright error occurred"
                self._tray.show_notification(project.name, msg, is_error=True)
```
DESPUÉS:
```python
        if state in (PlaywrightState.FAILED, PlaywrightState.PASSED, PlaywrightState.ERROR):
            project = self.config_manager.get_project(idx)
            if project:
                if state == PlaywrightState.PASSED:
                    self._notify(project.name, "All tests passed ✓", ToastLevel.SUCCESS)
                elif state == PlaywrightState.FAILED:
                    self._notify(project.name, "Test run failed", ToastLevel.ERROR)
                else:
                    self._notify(project.name, "Playwright error occurred", ToastLevel.ERROR)
```

En `_on_script_finished` (línea ~1052):

ANTES:
```python
                self._tray.show_notification(
                    project.name, f"Script '{name}' exited with code {exit_code}", is_error=True
                )
```
DESPUÉS:
```python
                self._notify(project.name, f"Script '{name}' exited with code {exit_code}", ToastLevel.ERROR)
```

En `_on_config_error` (línea ~571): agregar tras el showMessage:
```python
        self._notify("Configuration Notice", message, ToastLevel.WARNING)
```

En `_on_port_mismatch` (tras el showMessage de la línea ~795):
```python
            project = self.config_manager.get_project(idx)
            if project:
                self._notify(project.name, f"Port changed: running on :{det_port} (configured :{cfg_port})", ToastLevel.WARNING)
```

- [ ] **Step 8: Selección de proyecto → nuevos paneles**

En `_on_project_selected`, rama vacía (`if not project or index < 0:`, línea ~580), agregar antes del `return`:

```python
            self._git_panel.set_project(None)
            self._evidence_panel.set_project(None)
```

En la rama con proyecto (después de `self._scripts_panel.set_controls_enabled(True)`, línea ~610):

```python
        self._git_panel.set_project(project)
        self._evidence_panel.set_project(project)
        self._refresh_monitor_data()
```

- [ ] **Step 9: projects_changed → monitor**

En `_on_projects_changed` (línea ~550), después de `self._sidebar.set_projects(projects)`:

```python
        self._monitor_panel.set_projects(projects)
```

- [ ] **Step 10: Tema → nuevos paneles**

En `_on_theme_changed` (líneas ~448-453), después de `self._log_panel.apply_theme(mode)`:

```python
        self._git_panel.apply_theme(mode)
        self._monitor_panel.apply_theme(mode)
        self._evidence_panel.apply_theme(mode)
```

- [ ] **Step 11: resizeEvent + cierre**

Override nuevo (antes de closeEvent):

```python
    def resizeEvent(self, event):
        super().resizeEvent(event)
        if self._toast_manager:
            self._toast_manager.reposition()
```

En `closeEvent`, en la rama de salida real (después de los loops de stop, antes de `self._tray.hide()`):

```python
        if self._trace_runner is not None and self._trace_runner.is_running():
            self._trace_runner.stop()
```

- [ ] **Step 12: Polling setting → panel**

En `__init__`, después de `self._setup_tray()` (y antes de conectar señales), suscribirse:

```python
        self.app_settings.setting_changed.connect(self._on_setting_changed)
```

Handler (junto a los otros handlers):

```python
    def _on_setting_changed(self, key: str, value):
        if key == KEY_POLLING_ENABLED:
            self._monitor_panel.set_polling_enabled(bool(value))
            if not value:
                self._monitor_panel.update_resources([])
```

Inicializar estado del panel tras crear tabs (al final de `_setup_ui`, junto al toast):

```python
        self._monitor_panel.set_polling_enabled(AppSettings.instance().get(KEY_POLLING_ENABLED, True))
        self._monitor_panel.set_projects(self.config_manager.get_projects())
```

- [ ] **Step 13: Verificación completa**

Run: `.venv\Scripts\python -m pytest tests/ -v`
Expected: PASS (incluye `test_main_window.py` existente)

Smoke de arranque GUI (manual): `.venv\Scripts\python main.py` → verificar 8 tabs, abrir Settings, chips de sidebar funcionan, Errors only filtra, uptime corre con servidor activo.

---

### Task 16: README + verificación final

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Actualizar Características**

Agregar a la lista de features:

```markdown
- **Tab Git:** Rama, estado dirty, ahead/behind, último commit y acciones Pull/Fetch/Stash con salida en vivo en Logs.
- **Tab Monitor:** Estado de puertos configurados (libre/ocupado/proceso ajeno con Kill) y CPU/RAM por árbol de proceso de cada servidor (auto-refresh 3s, configurable en Settings).
- **Tab Evidence:** Galería de screenshots, videos y traces de `test-results/` con preview, apertura externa y visor de traces integrado.
- **Toasts in-app:** Notificaciones visuales apilables cuando la ventana está visible; bandeja del sistema cuando está minimizada.
- **Uptime:** Tiempo encendido de cada servidor en el panel Server.
- **Filtros:** Sidebar con chips All/Running/Stopped y logs con modo "Errors only".
- **Settings (Ctrl+,):** Preferencias persistentes de la aplicación (polling de recursos, toasts).
```

- [ ] **Step 2: Actualizar estructura del proyecto**

En el árbol de estructura, reflejar archivos nuevos:

```
│   ├── config/
│   │   ├── manager.py
│   │   └── settings.py           # Ajustes globales (QSettings)
│   ├── process/
│   │   ├── runner.py
│   │   └── monitor.py            # psutil: puertos, kill, uso CPU/RAM
...
│       ├── project_list.py     # Sidebar + búsqueda + chips de estado
│       ├── server_panel.py     # Start/Stop/Restart + uptime
│       ├── playwright_panel.py
│       ├── scripts_panel.py
│       ├── git_panel.py          # Acciones git seguras
│       ├── monitor_panel.py      # Puertos + recursos
│       ├── evidence_panel.py     # Galería test-results
│       ├── log_panel.py        # Logs + errors-only
│       └── toast.py              # Notificaciones in-app
```

- [ ] **Step 3: Suite completa verde**

Run: `.venv\Scripts\python -m pytest tests/ -v`
Expected: ALL PASS (existentes + test_settings + test_system_monitor + test_evidence + test_git)

- [ ] **Step 4: Checklist manual**

Ejecutar `run.bat` (o `.venv\Scripts\python main.py`) y verificar:
1. 8 tabs presentes en orden: Server, Playwright, Evidence, Scripts, Git, Monitor, Logs, App Log.
2. Ctrl+, abre Settings; apagar polling persiste tras reiniciar app.
3. Chips sidebar: Running muestra solo servidores activos; combina con búsqueda.
4. Errors only en Logs y App Log filtra correctamente; combinable con texto.
5. Iniciar un servidor → uptime corre en ServerPanel; toast success aparece; badge tray NO suena (ventana visible).
6. Minimizar a bandeja → detener/reiniciar servidor → llega notificación de bandeja.
7. Tab Git en proyecto repo: branch/dirty/ahead-behind correctos; Pull en repo sin upstream muestra error limpio en Logs + toast.
8. Tab Monitor: puertos listados; iniciar servidor marca "Used by"; ocupar puerto con proceso ajeno muestra Kill; Kill pide confirmación.
9. Tab Resources del Monitor: CPU/RAM aparecen desde el segundo refresh (baseline psutil).
10. Tab Evidence en proyecto con corridas previas: thumbnails cargan progresivamente; doble clic previewa; .zip ofrece Show Trace; HTML Report abre navegador.
11. Cambiar tema → todos los tabs nuevos re-colorean correctamente.

---

## Self-Review (ejecutado al escribir el plan)

1. **Spec coverage:** §3.1→Tasks 1+8, §3.2→Tasks 2+10+15(steps 4,6,12), §3.3→Tasks 4+9+15(step 6), §3.4→Tasks 3+11+15(step 6), §3.5→Tasks 7+15(steps 6-7,11), §3.6→Tasks 5+14+15(steps 4,6), §3.7→Task 12, §3.8→Task 13, §6 testing→tasks individuales+16, README→16. ✅ Sin gaps.
2. **Placeholders:** Ningún TBD/TODO; todos los steps tienen código o comando exacto.
3. **Type consistency:** `get_port_owner`→`PortOwner(pid,name)` usado en `_refresh_monitor_data` como `owner.name/owner.pid` ✅; `kill_tree(pid)->(bool,str)` usado igual ✅; `get_process_tree_usage`→`ProcessUsage(cpu_percent,rss_mb,children)` mapeado a claves `cpu/rss/children` del dict que consume `ResourceRow.apply_row` ✅; `KEY_POLLING_ENABLED/KEY_TOASTS_ENABLED` nombres idénticos en Tasks 1,7,8,15 ✅; `started_at` propiedad consumida por `_update_uptime` ✅; señal `trace_open_requested(str)` emitida por EvidencePanel y conectada en Task 15 ✅; `format_uptime` definida en Task 14 usada ahí mismo ✅.
