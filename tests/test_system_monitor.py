# -*- coding: utf-8 -*-
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
