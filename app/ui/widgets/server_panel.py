from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QPushButton,
    QLabel, QGroupBox, QFrame
)
from PySide6.QtCore import Signal, Qt, QSize
from PySide6.QtGui import QFont, QDesktopServices
from PySide6.QtCore import QUrl
from app.models.project import Project, ServerState
from app.ui.icons import get_icon
from app.ui.theme import ThemeManager, ThemeMode, ui_font
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


class ServerPanel(QWidget):
    start_requested = Signal()
    stop_requested = Signal()
    restart_requested = Signal()
    update_port_requested = Signal(int, str)  # (new_port, new_url)

    def __init__(self, parent=None):
        super().__init__(parent)
        self._project = None
        self._active_port: int = 0
        self._active_url: str = ""
        self._is_mismatch: bool = False
        self._configured_port: int = 0
        
        self._setup_ui()
        self.apply_theme(ThemeManager.instance().mode)
        self.set_controls_enabled(False)

    def _setup_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(14, 14, 14, 14)
        layout.setSpacing(14)

        # Server Info Card
        self._info_group = QGroupBox("Server Information")
        info_layout = QVBoxLayout(self._info_group)
        info_layout.setSpacing(8)
        info_layout.setContentsMargins(14, 16, 14, 14)

        self._path_label = QLabel("Path: —")
        self._path_label.setWordWrap(True)
        
        url_row = QHBoxLayout()
        self._url_label = QLabel("URL: —")
        self._url_label.setTextFormat(Qt.TextFormat.RichText)
        self._url_label.setWordWrap(True)
        url_row.addWidget(self._url_label, stretch=1)

        self._sync_port_btn = QPushButton("💾 Save Port to Config")
        self._sync_port_btn.setFixedHeight(26)
        self._sync_port_btn.setToolTip("Save this detected port as the project's default port in projects.json")
        self._sync_port_btn.setVisible(False)
        self._sync_port_btn.clicked.connect(self._on_sync_port_clicked)
        url_row.addWidget(self._sync_port_btn)

        self._status_label = QLabel("Status: Stopped")
        self._status_label.setFont(ui_font(14, QFont.Weight.DemiBold))

        info_layout.addWidget(self._path_label)
        info_layout.addLayout(url_row)
        info_layout.addWidget(self._status_label)

        self._uptime_label = QLabel("Uptime: —")
        self._uptime_label.setFont(ui_font(12))
        info_layout.addWidget(self._uptime_label)

        layout.addWidget(self._info_group)

        # Server Controls
        self._actions_group = QGroupBox("Controls")
        actions_layout = QVBoxLayout(self._actions_group)
        actions_layout.setSpacing(10)
        actions_layout.setContentsMargins(14, 16, 14, 14)

        btn_layout = QHBoxLayout()
        btn_layout.setSpacing(10)

        self._start_btn = QPushButton(" Start Server")
        self._start_btn.setIcon(get_icon("play-filled", "#ffffff"))
        self._start_btn.setIconSize(QSize(16, 16))
        self._start_btn.setMinimumHeight(44)

        self._stop_btn = QPushButton(" Stop")
        self._stop_btn.setIcon(get_icon("stop", "#ffffff"))
        self._stop_btn.setIconSize(QSize(16, 16))
        self._stop_btn.setMinimumHeight(44)

        self._restart_btn = QPushButton(" Restart")
        self._restart_btn.setIcon(get_icon("restart", "#ffffff"))
        self._restart_btn.setIconSize(QSize(16, 16))
        self._restart_btn.setMinimumHeight(44)

        self._start_btn.clicked.connect(self.start_requested.emit)
        self._stop_btn.clicked.connect(self.stop_requested.emit)
        self._restart_btn.clicked.connect(self.restart_requested.emit)

        btn_layout.addWidget(self._start_btn, stretch=2)
        btn_layout.addWidget(self._stop_btn, stretch=1)
        btn_layout.addWidget(self._restart_btn, stretch=1)

        actions_layout.addLayout(btn_layout)

        # Open URL button
        self._open_url_btn = QPushButton(" Open in Web Browser")
        self._open_url_btn.setIcon(get_icon("external-link", "#6366f1"))
        self._open_url_btn.setIconSize(QSize(16, 16))
        self._open_url_btn.setMinimumHeight(38)
        self._open_url_btn.clicked.connect(self._open_url)
        actions_layout.addWidget(self._open_url_btn)

        layout.addWidget(self._actions_group)
        layout.addStretch()

    def apply_theme(self, mode: ThemeMode):
        c = ThemeManager.instance().get_colors(mode)

        self._start_btn.setStyleSheet(
            f"QPushButton {{ background-color: {c['success']}; color: white; font-weight: 700; "
            f"border: none; border-radius: 8px; padding: 8px 16px; }} "
            f"QPushButton:hover {{ background-color: {c['success']}cc; }} "
            f"QPushButton:disabled {{ background-color: {c['border']}; color: {c['text_muted']}; }}"
        )
        self._stop_btn.setStyleSheet(
            f"QPushButton {{ background-color: {c['danger']}; color: white; font-weight: 700; "
            f"border: none; border-radius: 8px; padding: 8px 16px; }} "
            f"QPushButton:hover {{ background-color: {c['danger']}cc; }} "
            f"QPushButton:disabled {{ background-color: {c['border']}; color: {c['text_muted']}; }}"
        )
        self._restart_btn.setStyleSheet(
            f"QPushButton {{ background-color: {c['primary']}; color: white; font-weight: 700; "
            f"border: none; border-radius: 8px; padding: 8px 16px; }} "
            f"QPushButton:hover {{ background-color: {c['primary_hover']}; }} "
            f"QPushButton:disabled {{ background-color: {c['border']}; color: {c['text_muted']}; }}"
        )
        self._sync_port_btn.setStyleSheet(
            f"QPushButton {{ background-color: {c['warning']}; color: white; font-weight: 600; "
            f"font-size: 12px; border-radius: 6px; padding: 4px 10px; border: none; }} "
            f"QPushButton:hover {{ background-color: {c['warning']}cc; }}"
        )
        self._open_url_btn.setIcon(get_icon("external-link", c["primary"]))
        self._uptime_label.setStyleSheet(f"color: {c['text_secondary']}; font-size: 12px;")

    def set_project(self, project: Project):
        self._project = project
        self._path_label.setText(f"<b>Path:</b> {project.path}")
        self._configured_port = project.server.port
        self._active_port = project.server.port
        self._active_url = project.server.url or ""
        self._is_mismatch = False
        self._sync_port_btn.setVisible(False)
        
        url_text = project.server.url or "—"
        self._url_label.setText(f"<b>URL:</b> {url_text}")
        self.set_controls_enabled(project.server.enabled)

    def set_active_port(self, port: int, url: str, is_mismatch: bool = False, configured_port: int = 0):
        self._active_port = port
        self._active_url = url
        self._is_mismatch = is_mismatch
        self._configured_port = configured_port or (self._project.server.port if self._project else 0)

        c = ThemeManager.instance().get_colors()
        if is_mismatch:
            self._url_label.setText(
                f"<b>URL:</b> <span style='color: {c['primary']}; font-weight: bold;'>{url}</span> "
                f"<span style='color: {c['warning']}; font-size: 12px;'>⚠️ (Auto-detected :{port}, configured :{self._configured_port})</span>"
            )
            self._sync_port_btn.setText(f"💾 Save :{port} to Config")
            self._sync_port_btn.setVisible(True)
        else:
            self._url_label.setText(f"<b>URL:</b> {url}")
            self._sync_port_btn.setVisible(False)

    def set_uptime(self, seconds: Optional[float]):
        self._uptime_label.setText(f"Uptime: {format_uptime(seconds)}")

    def _clear_uptime(self):
        self.set_uptime(None)

    def update_state(self, state: ServerState):
        c = ThemeManager.instance().get_colors()
        state_config = {
            ServerState.STOPPED: ("Stopped", c["text_muted"], True, False, True),
            ServerState.STARTING: ("Starting...", c["warning"], False, True, False),
            ServerState.RUNNING: ("Running", c["success"], False, True, True),
            ServerState.STOPPING: ("Stopping...", c["warning"], False, False, False),
            ServerState.ERROR: ("Error", c["danger"], True, False, True),
        }
        text, color, start, stop, restart = state_config.get(
            state, ("Unknown", c["text_muted"], True, False, True)
        )
        self._status_label.setText(f"Status: <span style='color: {color};'>{text}</span>")
        self._start_btn.setEnabled(start)
        self._stop_btn.setEnabled(stop)
        self._restart_btn.setEnabled(restart)

        if state == ServerState.STOPPED:
            self._clear_uptime()
            self._sync_port_btn.setVisible(False)

    def set_controls_enabled(self, enabled: bool):
        self._start_btn.setEnabled(enabled)
        self._stop_btn.setEnabled(False)
        self._restart_btn.setEnabled(enabled)
        self._open_url_btn.setEnabled(enabled)
        if not enabled:
            c = ThemeManager.instance().get_colors()
            self._status_label.setText(f"Status: <span style='color: {c['text_muted']};'>Server disabled</span>")
            self._sync_port_btn.setVisible(False)

    def _open_url(self):
        target_url = self._active_url or (self._project.server.url if self._project else "")
        if target_url:
            QDesktopServices.openUrl(QUrl(target_url))

    def _on_sync_port_clicked(self):
        if self._active_port > 0 and self._active_url:
            self.update_port_requested.emit(self._active_port, self._active_url)
            self._sync_port_btn.setVisible(False)
