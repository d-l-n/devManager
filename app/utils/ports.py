# -*- coding: utf-8 -*-
import re
import socket
from typing import Optional
from PySide6.QtCore import QObject, QTimer, Signal


def is_port_open(host: str, port: int) -> bool:
    """Check if a port is open on the given host using a short timeout."""
    if not host or port <= 0 or port > 65535:
        return False
    try:
        with socket.create_connection((host, port), timeout=0.5):
            return True
    except (OSError, socket.error):
        return False


def build_server_command(command: str, port: int) -> str:
    """
    Ensures that the server launch command explicitly binds to the configured port.
    If the command already specifies a port flag, it is returned untouched.
    """
    if not command or port <= 0:
        return command

    # If command already specifies a port flag like --port 1234 or -p 1234, don't duplicate
    if re.search(r'(?:--port|-p)\s+\d+', command):
        return command

    trimmed = command.strip()
    lower = trimmed.lower()

    if lower.startswith("pnpm ") or lower.startswith("pnpm.cmd "):
        return f"{trimmed} --port {port}"
    elif lower.startswith("npm run ") or lower == "npm start":
        return f"{trimmed} -- --port {port}"
    elif lower.startswith("yarn ") or lower.startswith("yarn.cmd "):
        return f"{trimmed} --port {port}"
    elif lower.startswith("bun run ") or lower.startswith("bun dev"):
        return f"{trimmed} --port {port}"
    elif lower.startswith("vite") or lower.startswith("npx vite"):
        return f"{trimmed} --port {port}"
    elif lower.startswith("next dev") or lower.startswith("npx next dev"):
        return f"{trimmed} -p {port}"
    elif lower.startswith("astro dev") or lower.startswith("npx astro dev"):
        return f"{trimmed} --port {port}"
    elif lower.startswith("python manage.py runserver"):
        return f"{trimmed} {port}"
    elif lower.startswith("uvicorn "):
        return f"{trimmed} --port {port}"
    elif lower.startswith("flask run"):
        return f"{trimmed} --port {port}"

    return trimmed


class PortChecker(QObject):
    port_ready = Signal()
    check_timeout = Signal()

    def __init__(self, host: str, port: int, timeout_ms: int = 15000, interval_ms: int = 250, parent=None):
        super().__init__(parent)
        self._host = host
        self._port = port
        self._timeout_ms = timeout_ms
        self._interval_ms = interval_ms
        
        self._interval_timer = QTimer(self)
        self._interval_timer.setInterval(self._interval_ms)
        self._interval_timer.timeout.connect(self._check)
        
        self._timeout_timer = QTimer(self)
        self._timeout_timer.setSingleShot(True)
        self._timeout_timer.setInterval(self._timeout_ms)
        self._timeout_timer.timeout.connect(self._on_timeout)

    def start(self):
        """Start polling the port and start the timeout timer."""
        self._interval_timer.start()
        self._timeout_timer.start()

    def stop(self):
        """Stop all timers."""
        self._interval_timer.stop()
        self._timeout_timer.stop()

    def _check(self):
        """Called by the interval timer to check if the port is open."""
        if is_port_open(self._host, self._port):
            self.stop()
            self.port_ready.emit()

    def _on_timeout(self):
        """Called when the overall check timeout has been reached."""
        self.stop()
        self.check_timeout.emit()
