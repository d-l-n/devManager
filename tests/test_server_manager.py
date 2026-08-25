# -*- coding: utf-8 -*-
import pytest
from app.models.project import Project, ServerConfig, PlaywrightConfig, ServerState
from app.server.manager import ServerManager


def test_server_manager_init():
    proj = Project(
        name="Test",
        path=".",
        server=ServerConfig(enabled=True, port=5173, url="http://localhost:5173"),
        playwright=PlaywrightConfig()
    )
    sm = ServerManager(proj)
    assert sm.state == ServerState.STOPPED
    assert sm.active_port == 5173
    assert sm.active_url == "http://localhost:5173"
    assert sm.is_port_mismatch is False


def test_server_manager_detect_mismatch_output():
    proj = Project(
        name="Test App",
        path=".",
        server=ServerConfig(enabled=True, port=5173, url="http://localhost:5173"),
        playwright=PlaywrightConfig()
    )
    sm = ServerManager(proj)
    mismatches = []
    ports = []
    
    sm.port_mismatch.connect(lambda cfg, det, url: mismatches.append((cfg, det, url)))
    sm.port_detected.connect(lambda port, url: ports.append((port, url)))
    
    # Simulate server starting and Vite outputting fallback port 5174
    sm._on_runner_output("  ➜  Local:   http://localhost:5174/")
    
    assert sm.active_port == 5174
    assert sm.active_url == "http://localhost:5174"
    assert sm.is_port_mismatch is True
    assert len(mismatches) == 1
    assert mismatches[0] == (5173, 5174, "http://localhost:5174")
    assert len(ports) == 1
    assert ports[0] == (5174, "http://localhost:5174")


def test_server_manager_matching_port():
    proj = Project(
        name="Test App",
        path=".",
        server=ServerConfig(enabled=True, port=3000, url="http://localhost:3000"),
        playwright=PlaywrightConfig()
    )
    sm = ServerManager(proj)
    mismatches = []
    
    sm.port_mismatch.connect(lambda cfg, det, url: mismatches.append((cfg, det, url)))
    
    sm._on_runner_output("ready - started server on 0.0.0.0:3000, url: http://localhost:3000")
    
    assert sm.active_port == 3000
    assert sm.is_port_mismatch is False
    assert len(mismatches) == 0


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
