from typing import Optional
from PySide6.QtCore import QObject, QProcess, Signal, QProcessEnvironment
import os
import re

# Regex to match ANSI escape sequences (colors, formatting, etc.)
_ANSI_ESCAPE_RE = re.compile(r'\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?\x07')


def strip_ansi(text: str) -> str:
    """Remove all ANSI escape codes from text."""
    return _ANSI_ESCAPE_RE.sub('', text)

class ProcessRunner(QObject):
    output_ready = Signal(str)
    error_ready = Signal(str)
    process_started = Signal()
    process_finished = Signal(int, str)
    process_error = Signal(str)

    def __init__(self, parent=None):
        super().__init__(parent)
        self._process: Optional[QProcess] = None

    def start(self, command: str, working_dir: str, extra_env: Optional[dict] = None):
        if self.is_running():
            self.process_error.emit("Process is already running.")
            return

        self._process = QProcess(self)
        self._process.setWorkingDirectory(working_dir)

        # Set environment variables
        env = QProcessEnvironment.systemEnvironment()
        if extra_env:
            for k, v in extra_env.items():
                env.insert(str(k), str(v))
        self._process.setProcessEnvironment(env)

        # Connect signals
        self._process.readyReadStandardOutput.connect(self._on_stdout)
        self._process.readyReadStandardError.connect(self._on_stderr)
        self._process.started.connect(self._on_started)
        self._process.finished.connect(self._on_finished)
        self._process.errorOccurred.connect(self._on_error)

        # Separate channels for stdout and stderr
        self._process.setProcessChannelMode(QProcess.ProcessChannelMode.SeparateChannels)

        # Execute via cmd.exe on Windows
        self._process.start('cmd.exe', ['/d', '/s', '/c', command])

    def stop(self):
        if not self.is_running():
            return
            
        pid = self.pid()
        if pid:
            kill_proc = QProcess()
            kill_proc.start('taskkill', ['/T', '/F', '/PID', str(pid)])
            kill_proc.waitForFinished(5000)
            
        if self._process:
            self._process.kill()
            self._process.waitForFinished(2000)

    def is_running(self) -> bool:
        return self._process is not None and self._process.state() == QProcess.ProcessState.Running

    def pid(self) -> int:
        if self._process:
            return self._process.processId()
        return 0

    def _on_started(self):
        self.process_started.emit()

    def _on_stdout(self):
        if not self._process:
            return
        data = self._process.readAllStandardOutput().data()
        try:
            text = data.decode('utf-8', errors='replace')
            for line in text.splitlines():
                if line:
                    self.output_ready.emit(strip_ansi(line))
        except Exception as e:
            self.error_ready.emit(f"Failed to decode stdout: {e}")

    def _on_stderr(self):
        if not self._process:
            return
        data = self._process.readAllStandardError().data()
        try:
            text = data.decode('utf-8', errors='replace')
            for line in text.splitlines():
                if line:
                    self.error_ready.emit(strip_ansi(line))
        except Exception as e:
            self.error_ready.emit(f"Failed to decode stderr: {e}")

    def _on_finished(self, exit_code: int, exit_status: QProcess.ExitStatus):
        status_desc = "NormalExit" if exit_status == QProcess.ExitStatus.NormalExit else "CrashExit"
        self.process_finished.emit(exit_code, status_desc)
        if self._process:
            self._process.deleteLater()
            self._process = None

    def _on_error(self, error: QProcess.ProcessError):
        error_map = {
            QProcess.ProcessError.FailedToStart: "Failed to start",
            QProcess.ProcessError.Crashed: "Crashed",
            QProcess.ProcessError.Timedout: "Timed out",
            QProcess.ProcessError.ReadError: "Read error",
            QProcess.ProcessError.WriteError: "Write error",
            QProcess.ProcessError.UnknownError: "Unknown error"
        }
        err_desc = error_map.get(error, "Unknown error")
        self.process_error.emit(err_desc)
