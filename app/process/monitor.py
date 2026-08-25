# -*- coding: utf-8 -*-
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


_proc_lock = threading.Lock()
_proc_cache: Dict[int, "psutil.Process"] = {}
_MAX_PROC_CACHE = 128


def _get_cached_process(pid: int):
    with _proc_lock:
        proc = _proc_cache.get(pid)
        if proc is None:
            # Evict stale entries if cache is full
            if len(_proc_cache) >= _MAX_PROC_CACHE:
                stale = [k for k in list(_proc_cache) if not psutil.pid_exists(k)]
                for k in stale[:len(stale) - _MAX_PROC_CACHE // 2]:
                    _proc_cache.pop(k, None)
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
