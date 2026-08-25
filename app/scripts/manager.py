from typing import Optional
from PySide6.QtCore import QObject, Signal
from app.models.project import Project
from app.process.runner import ProcessRunner


class ScriptManager(QObject):
    script_started = Signal(str)  # script_name
    script_finished = Signal(str, int)  # script_name, exit_code
    log_output = Signal(str, bool)  # message, is_error

    def __init__(self, project: Project, parent=None):
        super().__init__(parent)
        self._project = project
        self._runner = ProcessRunner(self)
        self._active_name: Optional[str] = None
        self._active_command: Optional[str] = None

        self._runner.process_started.connect(self._on_started)
        self._runner.output_ready.connect(lambda msg: self.log_output.emit(f"[{self._active_name}] {msg}", False))
        self._runner.error_ready.connect(lambda msg: self.log_output.emit(f"[{self._active_name}] {msg}", True))
        self._runner.process_finished.connect(self._on_finished)
        self._runner.process_error.connect(lambda msg: self.log_output.emit(f"[{self._active_name}] ERROR: {msg}", True))

    @property
    def project(self) -> Project:
        return self._project

    def update_project(self, project: Project):
        self._project = project

    def is_running(self) -> bool:
        return self._runner.is_running()

    @property
    def active_script_name(self) -> Optional[str]:
        return self._active_name if self.is_running() else None

    def run_script(self, name: str, command: str):
        if self.is_running():
            self.log_output.emit(f"⚠️ Cannot start '{name}': script '{self._active_name}' is already running.", True)
            return

        self._active_name = name
        self._active_command = command
        self.log_output.emit(f"⚡ Starting script '{name}': {command}", False)
        self._runner.start(command, self._project.path)

    def stop(self):
        if self.is_running():
            name = self._active_name or "Script"
            self.log_output.emit(f"🛑 Stopping script '{name}'...", False)
            self._runner.stop()

    def _on_started(self):
        if self._active_name:
            self.script_started.emit(self._active_name)

    def _on_finished(self, exit_code: int, status_desc: str):
        finished_name = self._active_name or "Script"
        msg = f"✓ Script '{finished_name}' finished (code {exit_code})" if exit_code == 0 else f"✕ Script '{finished_name}' exited with code {exit_code} ({status_desc})"
        self.log_output.emit(msg, exit_code != 0)
        self.script_finished.emit(finished_name, exit_code)
        self._active_name = None
        self._active_command = None
