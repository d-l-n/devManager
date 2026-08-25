# -*- coding: utf-8 -*-
from typing import List, Dict, Optional
from PySide6.QtWidgets import QSystemTrayIcon, QMenu
from PySide6.QtGui import QAction, QIcon
from PySide6.QtCore import Signal, QObject
from app.models.project import Project, ServerState
from app.ui.icons import get_icon


class DevManagerTray(QSystemTrayIcon):
    show_window_requested = Signal()
    start_all_requested = Signal()
    stop_all_requested = Signal()
    exit_app_requested = Signal()
    start_server_requested = Signal(int)
    stop_server_requested = Signal(int)
    open_url_requested = Signal(int)

    def __init__(self, parent=None):
        super().__init__(parent)
        self.setIcon(get_icon("cpu-bolt", "#6366f1"))
        self.setToolTip("Local Dev Manager")
        self._menu = QMenu()
        self.setContextMenu(self._menu)

        self.activated.connect(self._on_activated)
        self._build_menu([], {}, {})

    def _on_activated(self, reason: QSystemTrayIcon.ActivationReason):
        if reason in (QSystemTrayIcon.ActivationReason.Trigger, QSystemTrayIcon.ActivationReason.DoubleClick):
            self.show_window_requested.emit()

    def update_projects(
        self,
        projects: List[Project],
        server_states: Dict[int, ServerState],
        active_urls: Dict[int, str]
    ):
        self._build_menu(projects, server_states, active_urls)

    def _build_menu(
        self,
        projects: List[Project],
        server_states: Dict[int, ServerState],
        active_urls: Dict[int, str]
    ):
        self._menu.clear()

        # Header action
        title_act = QAction(get_icon("cpu-bolt", "#6366f1"), "Local Dev Manager", self)
        title_act.setEnabled(False)
        self._menu.addAction(title_act)
        self._menu.addSeparator()

        # Projects section
        if projects:
            for idx, p in enumerate(projects):
                state = server_states.get(idx, ServerState.STOPPED)
                is_running = state == ServerState.RUNNING
                status_icon = "status-running" if is_running else ("status-starting" if state == ServerState.STARTING else "status-stopped")
                
                project_menu = self._menu.addMenu(get_icon(status_icon), p.name)

                # Start / Stop server
                if is_running:
                    stop_act = QAction(get_icon("stop", "#ef4444"), f"Stop Server (:{p.server.port})", self)
                    stop_act.triggered.connect(lambda _, i=idx: self.stop_server_requested.emit(i))
                    project_menu.addAction(stop_act)
                else:
                    start_act = QAction(get_icon("play-filled", "#10b981"), f"Start Server (:{p.server.port})", self)
                    start_act.triggered.connect(lambda _, i=idx: self.start_server_requested.emit(i))
                    project_menu.addAction(start_act)

                # Open URL
                url = active_urls.get(idx) or (p.server.url if p.server.enabled else "")
                if url:
                    url_act = QAction(get_icon("external-link"), f"Open {url}", self)
                    url_act.triggered.connect(lambda _, i=idx: self.open_url_requested.emit(i))
                    project_menu.addAction(url_act)

            self._menu.addSeparator()

        # Global actions
        start_all_act = QAction(get_icon("play-filled", "#10b981"), "Start All Servers", self)
        start_all_act.triggered.connect(self.start_all_requested.emit)
        self._menu.addAction(start_all_act)

        stop_all_act = QAction(get_icon("stop", "#ef4444"), "Stop All Servers", self)
        stop_all_act.triggered.connect(self.stop_all_requested.emit)
        self._menu.addAction(stop_all_act)

        self._menu.addSeparator()

        show_act = QAction("Show / Focus Window", self)
        show_act.triggered.connect(self.show_window_requested.emit)
        self._menu.addAction(show_act)

        exit_act = QAction("Exit Application", self)
        exit_act.triggered.connect(self.exit_app_requested.emit)
        self._menu.addAction(exit_act)

    def show_notification(self, title: str, message: str, is_error: bool = False):
        """Displays a native system tray notification."""
        icon_type = QSystemTrayIcon.MessageIcon.Critical if is_error else QSystemTrayIcon.MessageIcon.Information
        self.showMessage(title, message, icon_type, 3500)
