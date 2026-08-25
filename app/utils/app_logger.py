"""
Application-level logger that captures Python stdout/stderr and Python logging
into a Qt signal so the GUI can display it in an integrated App Log panel.
"""

import sys
import logging
from datetime import datetime
from PySide6.QtCore import QObject, Signal


class _StreamRedirector:
    """Wraps a stream (stdout/stderr) and duplicates output to a callback."""

    def __init__(self, callback, is_error: bool = False, original=None):
        self._callback = callback
        self._is_error = is_error
        self._original = original

    def write(self, text: str):
        if text and text.strip():
            self._callback(text.rstrip("\n\r"), self._is_error)
        # Also write to original stream if available (for debugging / pythonw fallback)
        if self._original:
            try:
                self._original.write(text)
                self._original.flush()
            except Exception:
                pass

    def flush(self):
        if self._original:
            try:
                self._original.flush()
            except Exception:
                pass


class _QtLogHandler(logging.Handler):
    """Routes Python logging records into the AppLogger signal."""

    def __init__(self, callback):
        super().__init__()
        self._callback = callback

    def emit(self, record):
        try:
            msg = self.format(record)
            is_error = record.levelno >= logging.ERROR
            self._callback(msg, is_error)
        except Exception:
            pass


class AppLogger(QObject):
    """
    Singleton-like application logger.
    Captures sys.stdout, sys.stderr and Python logging into Qt signals.
    """

    log_message = Signal(str, bool)  # (message, is_error)

    _instance = None

    def __init__(self, parent=None):
        super().__init__(parent)
        self._messages = []  # List of (timestamp_str, message, is_error)

    @classmethod
    def instance(cls, parent=None):
        if cls._instance is None:
            cls._instance = cls(parent)
        return cls._instance

    def install(self):
        """Redirect stdout, stderr, and Python logging to this logger."""
        # Save originals
        self._orig_stdout = sys.stdout
        self._orig_stderr = sys.stderr

        # Redirect stdout and stderr
        sys.stdout = _StreamRedirector(self._on_output, is_error=False, original=self._orig_stdout)
        sys.stderr = _StreamRedirector(self._on_output, is_error=True, original=self._orig_stderr)

        # Install Python logging handler
        root_logger = logging.getLogger()
        handler = _QtLogHandler(self._on_output)
        handler.setFormatter(logging.Formatter("%(levelname)s [%(name)s] %(message)s"))
        root_logger.addHandler(handler)
        if root_logger.level == logging.NOTSET:
            root_logger.setLevel(logging.INFO)

        self.info("App Logger initialized")

    def _on_output(self, text: str, is_error: bool = False):
        ts = datetime.now().strftime("%H:%M:%S")
        self._messages.append((ts, text, is_error))
        # Cap memory
        if len(self._messages) > 3000:
            self._messages = self._messages[-3000:]
        self.log_message.emit(text, is_error)

    def info(self, msg: str):
        self._on_output(msg, False)

    def error(self, msg: str):
        self._on_output(msg, True)

    def get_history(self):
        """Return all captured messages as list of (timestamp, text, is_error)."""
        return list(self._messages)
