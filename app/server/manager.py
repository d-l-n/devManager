# -*- coding: utf-8 -*-
from typing import Optional
import time
from PySide6.QtCore import QObject, Signal, QTimer
from app.models.project import Project, ServerState
from app.process.runner import ProcessRunner
from app.utils.ports import PortChecker, is_port_open, build_server_command
from app.utils.detection import extract_port_from_log


class ServerManager(QObject):
    state_changed = Signal(object)  # ServerState
    log_output = Signal(str)
    ready = Signal()
    port_detected = Signal(int, str)  # (port, url)
    port_mismatch = Signal(int, int, str)  # (configured_port, detected_port, active_url)

    def __init__(self, project: Project, parent=None):
        super().__init__(parent)
        self._project = project
        self._state = ServerState.STOPPED
        self._started_at: Optional[float] = None
        self._runner = ProcessRunner(self)
        self._port_checker: Optional[PortChecker] = None
        self._active_port: int = project.server.port
        self._active_url: str = project.server.url
        self._is_mismatch: bool = False
        self._port_was_already_occupied: bool = False
        
        self._runner.process_started.connect(self._on_runner_started)
        self._runner.output_ready.connect(self._on_runner_output)
        self._runner.error_ready.connect(self._on_runner_error)
        self._runner.process_finished.connect(self._on_runner_finished)
        self._runner.process_error.connect(self._on_runner_error)

    @property
    def state(self) -> ServerState:
        return self._state

    @property
    def project(self) -> Project:
        return self._project
        
    @project.setter
    def project(self, value: Project):
        self.update_project(value)

    @property
    def active_port(self) -> int:
        return self._active_port

    @property
    def active_url(self) -> str:
        return self._active_url

    @property
    def is_port_mismatch(self) -> bool:
        return self._is_mismatch

    @property
    def started_at(self) -> Optional[float]:
        """Epoch timestamp captured when the server entered RUNNING. None if stopped."""
        return self._started_at

    def start(self):
        if self._state not in (ServerState.STOPPED, ServerState.ERROR):
            return
            
        if not self._project.server.enabled:
            self.log_output.emit('Server not enabled')
            return
            
        self._set_state(ServerState.STARTING)
        self._active_port = self._project.server.port
        self._active_url = self._project.server.url
        self._is_mismatch = False
        
        configured_port = self._project.server.port
        host = self._project.server.url or '127.0.0.1'
        if '://' in host:
            host = host.split('://')[-1].split('/')[0].split(':')[0]

        # Check if configured port is already occupied before our server even starts
        if configured_port > 0 and is_port_open(host, configured_port):
            self._port_was_already_occupied = True
            self.log_output.emit(
                f"⚠️ [Port Notice] Configured port {configured_port} is already in use by another process. "
                f"The server may bind to a dynamic fallback port."
            )
        else:
            self._port_was_already_occupied = False
        
        raw_cmd = self._project.server.command
        cmd = build_server_command(raw_cmd, configured_port) if configured_port > 0 else raw_cmd
        cwd = self._project.path
        self.log_output.emit(f'Starting server: {cmd} in {cwd}')
        
        extra_env = {}
        if configured_port > 0:
            extra_env["PORT"] = str(configured_port)
            extra_env["VITE_PORT"] = str(configured_port)
            extra_env["SERVER_PORT"] = str(configured_port)

        self._runner.start(cmd, cwd, extra_env=extra_env)

    def stop(self):
        if self._state == ServerState.STOPPED:
            return
            
        self._set_state(ServerState.STOPPING)
        self._is_mismatch = False
        
        if self._port_checker:
            self._port_checker.stop()
            self._port_checker.deleteLater()
            self._port_checker = None
            
        self._runner.stop()
        self._started_at = None
        self._set_state(ServerState.STOPPED)

    def restart(self):
        if self._state in (ServerState.RUNNING, ServerState.STARTING, ServerState.ERROR):
            self.stop()
            QTimer.singleShot(500, self.start)
        else:
            self.start()

    def update_project(self, project: Project):
        self._project = project
        self._active_port = project.server.port
        self._active_url = project.server.url
        self._is_mismatch = False

    def _set_state(self, new_state: ServerState):
        if self._state != new_state:
            if new_state == ServerState.RUNNING and self._started_at is None:
                self._started_at = time.time()
            self._state = new_state
            self.state_changed.emit(new_state)

    def _enter_running(self):
        if self._started_at is None:
            self._started_at = time.time()
        self._set_state(ServerState.RUNNING)

    def _on_runner_started(self):
        port = self._project.server.port
        # If the port was already occupied before we started, we should not rely on PortChecker
        # for that port because it would immediately report ready for the WRONG process.
        if port > 0 and not self._port_was_already_occupied:
            host = self._project.server.url or '127.0.0.1'
            if '://' in host:
                host = host.split('://')[-1].split('/')[0].split(':')[0]
            
            timeout = self._project.server.startup_timeout or 30000
            self._port_checker = PortChecker(host, port, timeout, 250, self)
            self._port_checker.port_ready.connect(self._on_port_ready)
            self._port_checker.check_timeout.connect(self._on_port_timeout)
            self._port_checker.start()
        elif port <= 0:
            self._enter_running()
            self.ready.emit()

    def _on_port_ready(self):
        self._enter_running()
        self.ready.emit()
        self.log_output.emit(f'Server is ready on port {self._active_port}')

    def _on_port_timeout(self):
        self._set_state(ServerState.ERROR)
        self.log_output.emit(f'Server startup timeout after {self._project.server.startup_timeout}ms')

    def _on_runner_output(self, text: str):
        self.log_output.emit(text)

        # Dynamic port detection from live log output
        detected = extract_port_from_log(text)
        if detected:
            configured_port = self._project.server.port
            has_changed = (self._active_port != detected)
            is_different_from_config = (configured_port > 0 and detected != configured_port)

            self._active_port = detected
            self._active_url = f"http://localhost:{detected}"

            if is_different_from_config and not self._is_mismatch:
                self._is_mismatch = True
                self.port_mismatch.emit(configured_port, detected, self._active_url)
                self.log_output.emit(
                    f"⚠️ [PORT MISMATCH] Server started on port {detected}, differing from configured port {configured_port}"
                )
                self.log_output.emit(f"🔗 Active URL redirected to: {self._active_url}")
            elif has_changed:
                self.log_output.emit(f"[Auto-Detect] Server bound to active port {detected}")

            self.port_detected.emit(detected, self._active_url)

            if self._state == ServerState.STARTING:
                if self._port_checker:
                    self._port_checker.stop()
                    self._port_checker.deleteLater()
                    self._port_checker = None
                self._enter_running()
                self.ready.emit()

    def _on_runner_error(self, text: str):
        self.log_output.emit(f'[ERROR] {text}')

    def _on_runner_finished(self, exit_code: int, status: str):
        if self._port_checker:
            self._port_checker.stop()
            self._port_checker.deleteLater()
            self._port_checker = None

        self._started_at = None
        if exit_code == 0:
            self._set_state(ServerState.STOPPED)
        else:
            self._set_state(ServerState.ERROR)
            
        self.log_output.emit(f'Server process finished with exit code {exit_code} ({status})')
