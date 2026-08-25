from typing import Optional
from PySide6.QtCore import QObject, Signal
from app.models.project import Project, PlaywrightState, ServerState
from app.process.runner import ProcessRunner
from app.server.manager import ServerManager

class PlaywrightManager(QObject):
    state_changed = Signal(object)  # PlaywrightState
    log_output = Signal(str)
    tests_finished = Signal(int)

    def __init__(self, project: Project, server_manager: ServerManager, parent=None):
        super().__init__(parent)
        self._project = project
        self._server_manager = server_manager
        self._state = PlaywrightState.IDLE
        
        self._server_manager.state_changed.connect(self._on_server_state_changed)
        
        self._runner = ProcessRunner(self)
        self._runner.output_ready.connect(self._on_runner_output)
        self._runner.error_ready.connect(self._on_runner_error)
        self._runner.process_finished.connect(self._on_runner_finished)
        self._runner.process_error.connect(self._on_runner_error)
        
        self._report_runner = ProcessRunner(self)
        self._report_runner.output_ready.connect(self._on_runner_output)
        self._report_runner.error_ready.connect(self._on_runner_error)
        self._report_runner.process_error.connect(self._on_runner_error)
        
        self._pending_command: Optional[str] = None

    @property
    def state(self) -> PlaywrightState:
        return self._state

    def update_project(self, project: Project):
        self._project = project

    def run_tests(self):
        self._execute_or_wait_for_server(self._project.playwright.command)

    def run_ui(self):
        self._execute_or_wait_for_server(self._project.playwright.ui_command)

    def run_debug(self):
        self._execute_or_wait_for_server(self._project.playwright.debug_command)

    def show_report(self):
        cmd = self._project.playwright.report_command
        if not cmd:
            self.log_output.emit("No report command configured")
            return
            
        self.log_output.emit(f"Executing report command: {cmd}")
        self._report_runner.start(cmd, self._project.path)

    def stop(self):
        if self._runner.is_running():
            self._runner.stop()
        self._pending_command = None
        self._set_state(PlaywrightState.IDLE)

    def _execute_or_wait_for_server(self, command: str):
        if not command:
            self.log_output.emit("Command is empty")
            return
            
        if self._state == PlaywrightState.RUNNING:
            self.log_output.emit('Playwright is already running')
            return
            
        if not self._project.playwright.enabled:
            self.log_output.emit('Playwright is not enabled')
            return

        if self._project.server.enabled and self._server_manager.state != ServerState.RUNNING:
            self._pending_command = command
            self._set_state(PlaywrightState.STARTING)
            
            try:
                self._server_manager.ready.disconnect(self._on_server_ready)
            except RuntimeError:
                pass
                
            self._server_manager.ready.connect(self._on_server_ready)
            self._server_manager.start()
            self.log_output.emit('Waiting for server to be ready...')
        else:
            self._run_command(command)

    def _on_server_ready(self):
        try:
            self._server_manager.ready.disconnect(self._on_server_ready)
        except RuntimeError:
            pass
            
        if self._pending_command is not None:
            cmd = self._pending_command
            self._pending_command = None
            self._run_command(cmd)

    def _on_server_state_changed(self, server_state: ServerState):
        if self._pending_command is not None and server_state in (ServerState.ERROR, ServerState.STOPPED):
            try:
                self._server_manager.ready.disconnect(self._on_server_ready)
            except RuntimeError:
                pass
            self._pending_command = None
            self._set_state(PlaywrightState.ERROR)
            self.log_output.emit('Server failed to start or timed out. Playwright execution cancelled.')

    def _run_command(self, command: str):
        self._set_state(PlaywrightState.RUNNING)
        self.log_output.emit(f"Running command: {command}")
        
        extra_env = {}
        if self._project.server.enabled and self._server_manager.active_url:
            extra_env["BASE_URL"] = self._server_manager.active_url
            extra_env["PLAYWRIGHT_TEST_BASE_URL"] = self._server_manager.active_url
            if self._server_manager.active_port > 0:
                extra_env["PORT"] = str(self._server_manager.active_port)
                
        self._runner.start(command, self._project.path, extra_env=extra_env)

    def _set_state(self, new_state: PlaywrightState):
        if self._state != new_state:
            self._state = new_state
            self.state_changed.emit(new_state)

    def _on_runner_output(self, text: str):
        self.log_output.emit(text)

    def _on_runner_error(self, text: str):
        self.log_output.emit(f'[ERROR] {text}')

    def _on_runner_finished(self, exit_code: int, status: str):
        if exit_code == 0:
            self._set_state(PlaywrightState.PASSED)
        else:
            self._set_state(PlaywrightState.FAILED)
            
        self.tests_finished.emit(exit_code)
        self.log_output.emit(f"Playwright process finished with exit code {exit_code} ({status})")
