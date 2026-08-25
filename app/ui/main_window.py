# -*- coding: utf-8 -*-
import os
import shutil
import sys
import time
from typing import Optional, Dict, List
from PySide6.QtWidgets import (
    QMainWindow, QWidget, QHBoxLayout, QVBoxLayout,
    QSplitter, QTabWidget, QMenuBar, QMenu,
    QMessageBox, QLabel, QToolBar, QStatusBar,
    QApplication, QFrame, QPushButton, QToolButton, QSizePolicy,
    QSystemTrayIcon
)
from PySide6.QtCore import Qt, QUrl, QSize, QProcess, QTimer
from PySide6.QtGui import QAction, QFont, QIcon, QCloseEvent, QDesktopServices, QKeySequence

from app.config.manager import ConfigManager
from app.models.project import Project, ServerState, PlaywrightState
from app.server.manager import ServerManager
from app.playwright.manager import PlaywrightManager
from app.scripts.manager import ScriptManager
from app.ui.project_dialog import ProjectDialog
from app.ui.widgets.project_list import ProjectListWidget
from app.ui.widgets.server_panel import ServerPanel
from app.ui.widgets.playwright_panel import PlaywrightPanel
from app.ui.widgets.scripts_panel import ScriptsPanel
from app.ui.widgets.log_panel import LogPanel
from app.ui.system_tray import DevManagerTray
from app.ui.icons import get_icon
from app.ui.theme import ThemeManager, ThemeMode, ui_font
from app.utils.app_logger import AppLogger
from app.utils.git import get_git_info
from app.config.settings import AppSettings, KEY_POLLING_ENABLED, KEY_TOASTS_ENABLED
from app.ui.settings_dialog import SettingsDialog
from app.ui.global_panels import GlobalPanelDialog
from app.ui.widgets.evidence_panel import EvidencePanel
from app.ui.widgets.git_panel import GitPanel
from app.ui.widgets.monitor_panel import MonitorPanel
from app.ui.widgets.toast import ToastManager, ToastLevel
from app.process import monitor as sysmon


class MainWindow(QMainWindow):
    def __init__(self, config_manager: ConfigManager, parent=None):
        super().__init__(parent)
        self.config_manager = config_manager
        self.theme_manager = ThemeManager.instance()
        self.app_settings = AppSettings.instance()
        
        self._server_managers: Dict[int, ServerManager] = {}
        self._playwright_managers: Dict[int, PlaywrightManager] = {}
        self._script_managers: Dict[int, ScriptManager] = {}
        self._project_logs: Dict[int, List[tuple[str, bool]]] = {}  # index -> list of (text, is_error)
        self._current_index: int = -1
        self._is_force_exit: bool = False
        self._toast_manager: Optional[ToastManager] = None  # creado en _setup_ui
        self._trace_runner = None
        self._uptime_timer: Optional[QTimer] = None
        self._last_tray_notification: float = 0.0  # cooldown to prevent spam
        self._pending_tray_notify: Optional[tuple[str, str, bool]] = None

        self.setWindowTitle("Local Dev Manager")
        self.setWindowIcon(get_icon("cpu-bolt", "#6366f1"))
        self.resize(1180, 780)
        self.setMinimumSize(940, 620)
        self.setStyleSheet(self.theme_manager.get_main_stylesheet())

        self._setup_ui()
        self._setup_menu_and_toolbar()
        self._setup_shortcuts()
        self._setup_tray()

        self.app_settings.setting_changed.connect(self._on_setting_changed)

        # Connect signals
        self.config_manager.projects_changed.connect(self._on_projects_changed)
        self.config_manager.config_error.connect(self._on_config_error)
        self.theme_manager.theme_changed.connect(self._on_theme_changed)

        # Initial load
        self._rebuild_managers()
        self._load_projects_ui()
        self._update_theme_ui()

    def _setup_ui(self):
        central_widget = QWidget()
        main_layout = QHBoxLayout(central_widget)
        main_layout.setContentsMargins(0, 0, 0, 0)
        main_layout.setSpacing(0)

        splitter = QSplitter(Qt.Orientation.Horizontal)
        splitter.setHandleWidth(2)

        # Left Sidebar
        self._sidebar = ProjectListWidget()
        self._sidebar.setMinimumWidth(250)
        self._sidebar.setMaximumWidth(380)
        self._sidebar.project_selected.connect(self._on_project_selected)
        self._sidebar.add_requested.connect(self._on_add_project)
        self._sidebar.edit_requested.connect(self._on_edit_project)
        self._sidebar.remove_requested.connect(self._on_remove_project)
        
        # Connect sidebar context menu actions
        self._sidebar.start_requested.connect(self._start_server_for_index)
        self._sidebar.stop_requested.connect(self._stop_server_for_index)
        self._sidebar.restart_requested.connect(self._restart_server_for_index)
        self._sidebar.open_url_requested.connect(self._open_url_for_index)
        self._sidebar.open_folder_requested.connect(self._open_folder_for_index)
        self._sidebar.open_terminal_requested.connect(self._open_terminal_for_index)
        self._sidebar.open_vscode_requested.connect(self._open_vscode_for_index)
        self._sidebar.open_opencode_requested.connect(self._open_opencode_for_index)
        self._sidebar.run_tests_requested.connect(self._run_tests_for_index)
        self._sidebar.toggle_pin_requested.connect(self._on_toggle_pin)
        self._sidebar.monitor_requested.connect(self._show_global_monitor)
        self._sidebar.applog_requested.connect(self._show_app_log)

        splitter.addWidget(self._sidebar)

        # Right Detail Area
        detail_container = QWidget()
        detail_layout = QVBoxLayout(detail_container)
        detail_layout.setContentsMargins(16, 16, 16, 16)
        detail_layout.setSpacing(12)

        # Header Info Banner Card
        self._header_card = QFrame()
        self._header_card.setObjectName("headerCard")
        header_vbox = QVBoxLayout(self._header_card)
        header_vbox.setContentsMargins(18, 16, 18, 16)
        header_vbox.setSpacing(10)

        # Top row: Title + Quick actions (icon-only for compactness)
        top_row = QHBoxLayout()
        top_row.setSpacing(6)

        self._title_label = QLabel("No Project Selected")
        self._title_label.setFont(ui_font(17, QFont.Weight.Bold))
        self._title_label.setObjectName("projectTitle")
        top_row.addWidget(self._title_label)
        top_row.addStretch()

        # Quick Explorer, Terminal, VS Code & OpenCode Buttons (icon-only)
        self._explorer_btn = QToolButton()
        self._explorer_btn.setIcon(get_icon("folder", "#6366f1"))
        self._explorer_btn.setIconSize(QSize(16, 16))
        self._explorer_btn.setFixedSize(30, 30)
        self._explorer_btn.setToolTip("Open project folder in File Explorer (Ctrl+O)")
        self._explorer_btn.clicked.connect(self._open_current_folder)
        top_row.addWidget(self._explorer_btn)

        self._terminal_btn = QToolButton()
        self._terminal_btn.setIcon(get_icon("terminal", "#6366f1"))
        self._terminal_btn.setIconSize(QSize(16, 16))
        self._terminal_btn.setFixedSize(30, 30)
        self._terminal_btn.setToolTip("Open Terminal in project folder (Ctrl+` or Ctrl+T)")
        self._terminal_btn.clicked.connect(self._open_current_in_terminal)
        top_row.addWidget(self._terminal_btn)

        self._vscode_btn = QToolButton()
        self._vscode_btn.setIcon(get_icon("sidebar-code", "#6366f1"))
        self._vscode_btn.setIconSize(QSize(16, 16))
        self._vscode_btn.setFixedSize(30, 30)
        self._vscode_btn.setToolTip("Open project in Visual Studio Code (Ctrl+Shift+C)")
        self._vscode_btn.clicked.connect(self._open_current_in_vscode)
        top_row.addWidget(self._vscode_btn)

        self._opencode_btn = QToolButton()
        self._opencode_btn.setIcon(get_icon("terminal-circle", "#6366f1"))
        self._opencode_btn.setIconSize(QSize(16, 16))
        self._opencode_btn.setFixedSize(30, 30)
        self._opencode_btn.setToolTip("Open project in OpenCode (Ctrl+Shift+O)")
        self._opencode_btn.clicked.connect(self._open_current_in_opencode)
        top_row.addWidget(self._opencode_btn)

        header_vbox.addLayout(top_row)

        # Bottom row: Badges (compact, on their own line)
        badges_row = QHBoxLayout()
        badges_row.setSpacing(6)

        self._server_badge = QLabel("Server: Stopped")
        self._server_badge.setObjectName("statusBadge")
        badges_row.addWidget(self._server_badge)

        self._pw_badge = QLabel("Playwright: Idle")
        self._pw_badge.setObjectName("statusBadge")
        badges_row.addWidget(self._pw_badge)

        self._git_badge = QLabel("Git: None")
        self._git_badge.setObjectName("statusBadge")
        badges_row.addWidget(self._git_badge)

        badges_row.addStretch()

        header_vbox.addLayout(badges_row)

        # Subtitle: Path & URL
        self._path_label = QLabel("Select a project from the sidebar to manage servers and tests.")
        self._path_label.setObjectName("projectSubtitle")
        self._path_label.setWordWrap(True)
        self._path_label.setMinimumHeight(18)
        header_vbox.addWidget(self._path_label)

        detail_layout.addWidget(self._header_card)

        # Project tabs
        self._tab_widget = QTabWidget()
        self._server_panel = ServerPanel()
        self._playwright_panel = PlaywrightPanel()
        self._scripts_panel = ScriptsPanel()
        self._log_panel = LogPanel()

        self._evidence_panel = EvidencePanel()
        self._git_panel = GitPanel()

        self._tab_widget.addTab(self._server_panel, get_icon("server", "#6366f1"), "  Server  ")
        self._tab_widget.addTab(self._playwright_panel, get_icon("flask", "#6366f1"), "  Playwright  ")
        self._tab_widget.addTab(self._evidence_panel, get_icon("image", "#6366f1"), "  Evidence  ")
        self._tab_widget.addTab(self._scripts_panel, get_icon("cpu-bolt", "#6366f1"), "  Scripts  ")
        self._tab_widget.addTab(self._git_panel, get_icon("git-branch", "#6366f1"), "  Git  ")
        self._tab_widget.addTab(self._log_panel, get_icon("terminal", "#6366f1"), "  Logs  ")

        # Global (project-independent) panels — Monitor & App Log
        self._monitor_panel = MonitorPanel()
        self._app_log_panel = LogPanel()

        # Connect AppLogger to the App Log panel
        app_logger = AppLogger.instance()
        app_logger.log_message.connect(self._on_app_log_message)
        for ts, text, is_error in app_logger.get_history():
            if is_error:
                self._app_log_panel.append_error(text)
            else:
                self._app_log_panel.append_log(text)

        # Global dialogs (modeless; hide-on-close; single instance each)
        self._monitor_dialog = GlobalPanelDialog("Global Monitor", self._monitor_panel, self)
        self._applog_dialog = GlobalPanelDialog("Application Log", self._app_log_panel, self)

        detail_layout.addWidget(self._tab_widget, stretch=1)

        # Connect Panel actions
        self._server_panel.start_requested.connect(self._on_start_server)
        self._server_panel.stop_requested.connect(self._on_stop_server)
        self._server_panel.restart_requested.connect(self._on_restart_server)
        self._server_panel.update_port_requested.connect(self._on_update_project_port)

        self._playwright_panel.run_tests_requested.connect(self._on_run_tests)
        self._playwright_panel.run_ui_requested.connect(self._on_run_ui)
        self._playwright_panel.run_debug_requested.connect(self._on_run_debug)
        self._playwright_panel.show_report_requested.connect(self._on_show_report)
        self._playwright_panel.stop_requested.connect(self._on_stop_playwright)

        self._scripts_panel.run_script_requested.connect(self._on_run_script)
        self._scripts_panel.stop_script_requested.connect(self._on_stop_script)

        self._git_panel.output.connect(
            lambda msg, err: self._append_project_log(self._current_index, msg, is_error=err)
        )
        self._git_panel.command_finished.connect(self._on_git_command_finished)
        self._monitor_panel.refresh_requested.connect(self._refresh_monitor_data)
        self._monitor_panel.kill_requested.connect(self._on_monitor_kill)
        self._evidence_panel.trace_open_requested.connect(self._open_trace_viewer)

        splitter.addWidget(detail_container)
        splitter.setStretchFactor(0, 0)
        splitter.setStretchFactor(1, 1)

        main_layout.addWidget(splitter)
        self.setCentralWidget(central_widget)

        self._toast_manager = ToastManager(self)

        self._uptime_timer = QTimer(self)
        self._uptime_timer.setInterval(1000)
        self._uptime_timer.timeout.connect(self._update_uptime)
        self._uptime_timer.start()

        self._monitor_panel.set_polling_enabled(AppSettings.instance().get(KEY_POLLING_ENABLED, True))
        self._monitor_panel.set_projects(self.config_manager.get_projects())

        self._status_bar = QStatusBar()
        self.setStatusBar(self._status_bar)
        self._status_bar.showMessage("Ready")

    def _setup_menu_and_toolbar(self):
        menubar = self.menuBar()

        # File Menu
        file_menu = menubar.addMenu("&File")

        add_action = QAction(get_icon("plus", "#6366f1"), "&Add Project...", self)
        add_action.setShortcut("Ctrl+N")
        add_action.triggered.connect(self._on_add_project)
        file_menu.addAction(add_action)

        reload_action = QAction(get_icon("refresh", "#6366f1"), "&Reload Projects", self)
        reload_action.setShortcut("Ctrl+R")
        reload_action.setStatusTip("Reload project configuration from projects.json (Ctrl+R)")
        reload_action.triggered.connect(self._on_reload_config)
        file_menu.addAction(reload_action)

        edit_action = QAction(get_icon("edit", "#94a3b8"), "&Edit Current Project...", self)
        edit_action.setShortcut("Ctrl+E")
        edit_action.triggered.connect(lambda: self._on_edit_project(self._current_index))
        file_menu.addAction(edit_action)

        remove_action = QAction(get_icon("trash", "#ef4444"), "&Remove Current Project", self)
        remove_action.triggered.connect(lambda: self._on_remove_project(self._current_index))
        file_menu.addAction(remove_action)

        settings_action = QAction(get_icon("settings", "#94a3b8"), "&Settings...", self)
        settings_action.setShortcut("Ctrl+,")
        settings_action.setStatusTip("Open application settings (Ctrl+,)")
        settings_action.triggered.connect(self._on_open_settings)
        file_menu.addAction(settings_action)

        file_menu.addSeparator()

        open_folder_act = QAction(get_icon("folder", "#6366f1"), "Open in &Explorer", self)
        open_folder_act.setShortcut("Ctrl+O")
        open_folder_act.triggered.connect(self._open_current_folder)
        file_menu.addAction(open_folder_act)

        open_terminal_act = QAction(get_icon("terminal", "#6366f1"), "Open in &Terminal", self)
        open_terminal_act.setShortcut("Ctrl+`")
        open_terminal_act.setStatusTip("Open Terminal in project root folder (Ctrl+` or Ctrl+Alt+T)")
        open_terminal_act.triggered.connect(self._open_current_in_terminal)
        file_menu.addAction(open_terminal_act)

        open_vscode_act = QAction(get_icon("sidebar-code", "#6366f1"), "Open in &VS Code", self)
        open_vscode_act.setShortcut("Ctrl+Shift+C")
        open_vscode_act.triggered.connect(self._open_current_in_vscode)
        file_menu.addAction(open_vscode_act)

        open_opencode_act = QAction(get_icon("terminal-circle", "#6366f1"), "Open in &OpenCode", self)
        open_opencode_act.setShortcut("Ctrl+Shift+O")
        open_opencode_act.triggered.connect(self._open_current_in_opencode)
        file_menu.addAction(open_opencode_act)

        file_menu.addSeparator()

        restart_app_action = QAction(get_icon("restart", "#f59e0b"), "Restart &Dev Manager", self)
        restart_app_action.setShortcut("Ctrl+Shift+R")
        restart_app_action.setStatusTip("Restart the Dev Manager application (Ctrl+Shift+R)")
        restart_app_action.triggered.connect(self._on_restart_app)
        file_menu.addAction(restart_app_action)

        exit_action = QAction("E&xit", self)
        exit_action.setShortcut("Ctrl+Q")
        exit_action.triggered.connect(self.close)
        file_menu.addAction(exit_action)

        # Servers Menu
        servers_menu = menubar.addMenu("&Servers")

        start_all_action = QAction(get_icon("play-filled", "#10b981"), "Start All Enabled Servers", self)
        start_all_action.triggered.connect(self._on_start_all)
        servers_menu.addAction(start_all_action)

        stop_all_action = QAction(get_icon("stop", "#ef4444"), "Stop All Servers", self)
        stop_all_action.triggered.connect(self._on_stop_all)
        servers_menu.addAction(stop_all_action)

        servers_menu.addSeparator()

        auto_ports_action = QAction(get_icon("edit", "#f59e0b"), "Auto-Assign &Unique Ports to All Projects", self)
        auto_ports_action.triggered.connect(self._on_auto_assign_unique_ports)
        servers_menu.addAction(auto_ports_action)

        # View Menu (Theme switcher)
        view_menu = menubar.addMenu("&View")

        self._dark_theme_act = QAction("Dark Theme", self)
        self._dark_theme_act.setCheckable(True)
        self._dark_theme_act.setChecked(self.theme_manager.mode == ThemeMode.DARK)
        self._dark_theme_act.triggered.connect(lambda: self.theme_manager.set_theme(ThemeMode.DARK))
        view_menu.addAction(self._dark_theme_act)

        self._oled_theme_act = QAction("OLED Theme (True Black)", self)
        self._oled_theme_act.setCheckable(True)
        self._oled_theme_act.setChecked(self.theme_manager.is_oled)
        self._oled_theme_act.triggered.connect(lambda: self.theme_manager.set_theme(ThemeMode.OLED))
        view_menu.addAction(self._oled_theme_act)

        self._light_theme_act = QAction("Light Theme", self)
        self._light_theme_act.setCheckable(True)
        self._light_theme_act.setChecked(self.theme_manager.mode == ThemeMode.LIGHT)
        self._light_theme_act.triggered.connect(lambda: self.theme_manager.set_theme(ThemeMode.LIGHT))
        view_menu.addAction(self._light_theme_act)

        view_menu.addSeparator()

        toggle_theme_act = QAction("Cycle Theme (Light/Dark/OLED)", self)
        toggle_theme_act.setShortcut("Ctrl+Shift+T")
        toggle_theme_act.triggered.connect(self.theme_manager.toggle_theme)
        view_menu.addAction(toggle_theme_act)

        view_menu.addSeparator()

        monitor_act = QAction(get_icon("activity", "#6366f1"), "&Global Monitor", self)
        monitor_act.setShortcut("Ctrl+Alt+M")
        monitor_act.setStatusTip("Open the project-independent port & resource monitor (Ctrl+Alt+M)")
        monitor_act.triggered.connect(self._show_global_monitor)
        view_menu.addAction(monitor_act)

        applog_act = QAction(get_icon("info", "#6366f1"), "Application &Log", self)
        applog_act.setShortcut("Ctrl+Alt+L")
        applog_act.setStatusTip("Open the application log window (Ctrl+Alt+L)")
        applog_act.triggered.connect(self._show_app_log)
        view_menu.addAction(applog_act)

        # Help Menu
        help_menu = menubar.addMenu("&Help")
        about_action = QAction(get_icon("server", "#6366f1"), "&Acerca de Local Dev Manager", self)
        about_action.triggered.connect(self._show_about)
        help_menu.addAction(about_action)

        # Toolbar
        toolbar = QToolBar("Main Controls")
        toolbar.setMovable(False)
        toolbar.setIconSize(QSize(16, 16))
        self.addToolBar(toolbar)

        toolbar.addAction(add_action)
        toolbar.addAction(reload_action)
        toolbar.addSeparator()
        toolbar.addAction(start_all_action)
        toolbar.addAction(stop_all_action)

        # Theme toggle in toolbar
        spacer = QWidget()
        spacer.setSizePolicy(QSizePolicy.Policy.Expanding, QSizePolicy.Policy.Preferred)
        toolbar.addWidget(spacer)

        self._theme_btn = QToolButton()
        self._theme_btn.setToolButtonStyle(Qt.ToolButtonStyle.ToolButtonTextBesideIcon)
        self._theme_btn.clicked.connect(self.theme_manager.toggle_theme)
        self._theme_btn.setToolTip("Cycle Theme: Light/Dark/OLED (Ctrl+Shift+T)")
        toolbar.addWidget(self._theme_btn)

        # Quit button (exits app completely, no tray minimize)
        self._quit_btn = QToolButton()
        self._quit_btn.setIcon(get_icon("power", "#ef4444"))
        self._quit_btn.setToolTip("Quit Application completely (Ctrl+Q)")
        self._quit_btn.clicked.connect(self._force_quit)
        toolbar.addWidget(self._quit_btn)

    def _setup_shortcuts(self):
        """Register convenient power-user keyboard shortcuts."""
        # F5 -> Start/Restart Server
        f5_action = QAction(self)
        f5_action.setShortcut(QKeySequence(Qt.Key.Key_F5))
        f5_action.triggered.connect(self._on_start_server)
        self.addAction(f5_action)

        # Shift+F5 -> Stop Server
        shift_f5 = QAction(self)
        shift_f5.setShortcut(QKeySequence("Shift+F5"))
        shift_f5.triggered.connect(self._on_stop_server)
        self.addAction(shift_f5)

        # Ctrl+` or Ctrl+Alt+T -> Open Terminal
        term_act = QAction(self)
        term_act.setShortcut(QKeySequence("Ctrl+`"))
        term_act.triggered.connect(self._open_current_in_terminal)
        self.addAction(term_act)

        term_act2 = QAction(self)
        term_act2.setShortcut(QKeySequence("Ctrl+Alt+T"))
        term_act2.triggered.connect(self._open_current_in_terminal)
        self.addAction(term_act2)

        # Ctrl+T -> Run Tests
        test_act = QAction(self)
        test_act.setShortcut(QKeySequence("Ctrl+T"))
        test_act.triggered.connect(self._on_run_tests)
        self.addAction(test_act)

        # Ctrl+L -> Clear Logs
        clear_act = QAction(self)
        clear_act.setShortcut(QKeySequence("Ctrl+L"))
        clear_act.triggered.connect(self._log_panel.clear)
        self.addAction(clear_act)

        # Ctrl+F -> Focus Search (smart focus)
        search_act = QAction(self)
        search_act.setShortcut(QKeySequence("Ctrl+F"))
        search_act.triggered.connect(self._on_search_shortcut)
        self.addAction(search_act)

        # Del -> Remove selected project (with confirmation)
        delete_act = QAction(self)
        delete_act.setShortcut(QKeySequence(Qt.Key.Key_Delete))
        delete_act.triggered.connect(lambda: self._on_remove_project(self._current_index))
        self.addAction(delete_act)

    def _setup_tray(self):
        """Initialize system tray icon with context menu and signals."""
        self._tray = DevManagerTray(self)
        self._tray.show_window_requested.connect(self._tray_show_window)
        self._tray.start_all_requested.connect(self._on_start_all)
        self._tray.stop_all_requested.connect(self._on_stop_all)
        self._tray.exit_app_requested.connect(self._tray_force_exit)
        self._tray.start_server_requested.connect(self._start_server_for_index)
        self._tray.stop_server_requested.connect(self._stop_server_for_index)
        self._tray.open_url_requested.connect(self._tray_open_url)
        self._tray.show()

    def _tray_show_window(self):
        """Restore and bring window to front from system tray."""
        self.showNormal()
        self.activateWindow()
        self.raise_()

    def _force_quit(self):
        """Force-quit the application completely (bypass tray, stop all servers)."""
        reply = QMessageBox.question(
            self,
            "Salir de la Aplicación",
            "Confirmar salida de Local Dev Manager\n\nTodos los servidores, ejecutores de pruebas y scripts activos se detendrán",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
            QMessageBox.StandardButton.No
        )
        if reply == QMessageBox.StandardButton.Yes:
            self._is_force_exit = True
            self.close()

    def _tray_force_exit(self):
        """Exit the application completely (not just minimize to tray)."""
        self._is_force_exit = True
        self.close()

    def _tray_open_url(self, index: int):
        """Open a project's active server URL from the tray menu."""
        sm = self._server_managers.get(index)
        if sm and sm.active_url:
            QDesktopServices.openUrl(QUrl(sm.active_url))
        else:
            project = self.config_manager.get_project(index)
            if project and project.server.url:
                QDesktopServices.openUrl(QUrl(project.server.url))

    def _update_tray_menu(self):
        """Refresh the tray context menu with current project states."""
        if not hasattr(self, '_tray'):
            return
        projects = self.config_manager.get_projects()
        server_states = {i: sm.state for i, sm in self._server_managers.items()}
        active_urls = {i: sm.active_url for i, sm in self._server_managers.items() if sm.active_url}
        self._tray.update_projects(projects, server_states, active_urls)

    def _on_theme_changed(self, mode: ThemeMode):
        self.setStyleSheet(self.theme_manager.get_main_stylesheet(mode))
        self._sidebar.apply_theme(mode)
        self._server_panel.apply_theme(mode)
        self._playwright_panel.apply_theme(mode)
        self._scripts_panel.apply_theme(mode)
        self._log_panel.apply_theme(mode)
        self._git_panel.apply_theme(mode)
        self._monitor_panel.apply_theme(mode)
        self._evidence_panel.apply_theme(mode)
        self._app_log_panel.apply_theme(mode)
        self._update_theme_ui()

        # Re-render badges with theme colors
        if self._current_index >= 0:
            self._on_project_selected(self._current_index)
        else:
            self._on_project_selected(-1)

    def _update_theme_ui(self):
        mode = self.theme_manager.mode
        self._dark_theme_act.setChecked(mode == ThemeMode.DARK)
        self._oled_theme_act.setChecked(mode == ThemeMode.OLED)
        self._light_theme_act.setChecked(mode == ThemeMode.LIGHT)

        # Button shows the NEXT mode in the cycle (Light -> Dark -> OLED -> Light).
        if mode == ThemeMode.LIGHT:
            self._theme_btn.setText(" Dark")
            self._theme_btn.setIcon(get_icon("moon", "#6366f1"))
        elif mode == ThemeMode.DARK:
            self._theme_btn.setText(" OLED")
            self._theme_btn.setIcon(get_icon("star", "#f59e0b"))
        else:
            self._theme_btn.setText(" Light")
            self._theme_btn.setIcon(get_icon("sun", "#f59e0b"))

        c = self.theme_manager.get_colors()
        self._explorer_btn.setIcon(get_icon("folder", c["primary"]))
        self._terminal_btn.setIcon(get_icon("terminal", c["primary"]))
        self._vscode_btn.setIcon(get_icon("sidebar-code", c["primary"]))
        self._opencode_btn.setIcon(get_icon("terminal-circle", c["primary"]))

    def _rebuild_managers(self):
        """Build or update managers to match config projects."""
        projects = self.config_manager.get_projects()
        
        # Stop and remove any excess managers
        current_count = len(projects)
        excess_keys = [k for k in self._server_managers.keys() if k >= current_count]
        for k in excess_keys:
            sm = self._server_managers.pop(k, None)
            if sm:
                sm.stop()
            pm = self._playwright_managers.pop(k, None)
            if pm:
                pm.stop()
            sc_m = self._script_managers.pop(k, None)
            if sc_m:
                sc_m.stop()

        # Update or create managers
        for idx, project in enumerate(projects):
            if idx not in self._server_managers:
                sm = ServerManager(project, self)
                pm = PlaywrightManager(project, sm, self)
                sc_m = ScriptManager(project, self)
                
                # Connect handlers with captured index
                self._bind_manager_signals(idx, sm, pm, sc_m)
                
                self._server_managers[idx] = sm
                self._playwright_managers[idx] = pm
                self._script_managers[idx] = sc_m
            else:
                self._server_managers[idx].update_project(project)
                self._playwright_managers[idx].update_project(project)
                if idx in self._script_managers:
                    self._script_managers[idx].update_project(project)

            if idx not in self._project_logs:
                self._project_logs[idx] = []

    def _bind_manager_signals(self, idx: int, sm: ServerManager, pm: PlaywrightManager, sc_m: Optional[ScriptManager] = None):
        sm.state_changed.connect(lambda state, i=idx: self._on_server_state_changed(i, state))
        sm.log_output.connect(lambda msg, i=idx: self._append_project_log(i, msg, is_error=False))
        sm.port_detected.connect(lambda port, url, i=idx: self._on_port_detected(i, port, url))
        sm.port_mismatch.connect(lambda cfg, det, url, i=idx: self._on_port_mismatch(i, cfg, det, url))
        
        pm.state_changed.connect(lambda state, i=idx: self._on_playwright_state_changed(i, state))
        pm.log_output.connect(lambda msg, i=idx: self._append_project_log(i, msg, is_error=False))

        if sc_m:
            sc_m.script_started.connect(lambda name, i=idx: self._on_script_started(i, name))
            sc_m.script_finished.connect(lambda name, code, i=idx: self._on_script_finished(i, name, code))
            sc_m.log_output.connect(lambda msg, is_err, i=idx: self._append_project_log(i, msg, is_error=is_err))

    def _load_projects_ui(self):
        projects = self.config_manager.get_projects()
        self._sidebar.set_projects(projects)
        
        # Restore status indicators
        for idx, sm in self._server_managers.items():
            self._sidebar.update_status(idx, sm.state)

        if projects:
            self._sidebar._list.setCurrentRow(0)
            self._on_project_selected(0)
        else:
            self._on_project_selected(-1)

        self._update_tray_menu()

    def _on_projects_changed(self):
        prev_index = self._current_index
        self._rebuild_managers()
        projects = self.config_manager.get_projects()
        self._sidebar.set_projects(projects)
        self._monitor_panel.set_projects(projects)
        # Monitor es global: se refresca con cambios de config, no con selección
        self._refresh_monitor_data()
        
        # Restore states
        for idx, sm in self._server_managers.items():
            self._sidebar.update_status(idx, sm.state)

        if 0 <= prev_index < len(projects):
            self._sidebar._list.setCurrentRow(prev_index)
            self._on_project_selected(prev_index)
        elif projects:
            self._sidebar._list.setCurrentRow(0)
            self._on_project_selected(0)
        else:
            self._on_project_selected(-1)

        self._update_tray_menu()

    def _on_config_error(self, message: str):
        self._status_bar.showMessage(f"Config Warning: {message}", 6000)
        self._notify("Notificación de Configuración", message, ToastLevel.WARNING)
        QMessageBox.warning(self, "Notificación de Configuración", message)

    def _on_project_selected(self, index: int):
        self._current_index = index
        project = self.config_manager.get_project(index)
        c = self.theme_manager.get_colors()
        
        if not project or index < 0:
            self._title_label.setText("No Project Selected")
            self._path_label.setText("Select a project from the sidebar to manage servers and tests.")
            self._server_badge.setText("Server: None")
            self._server_badge.setStyleSheet(f"background-color: {c['badge_bg']}; color: {c['text_muted']}; border-radius: 12px; padding: 4px 12px; border: 1px solid {c['border']};")
            self._pw_badge.setText("Playwright: None")
            self._pw_badge.setStyleSheet(f"background-color: {c['badge_bg']}; color: {c['text_muted']}; border-radius: 12px; padding: 4px 12px; border: 1px solid {c['border']};")
            self._git_badge.setText("Git: None")
            self._git_badge.setStyleSheet(f"background-color: {c['badge_bg']}; color: {c['text_muted']}; border-radius: 12px; padding: 4px 12px; border: 1px solid {c['border']};")
            self._explorer_btn.setEnabled(False)
            self._terminal_btn.setEnabled(False)
            self._vscode_btn.setEnabled(False)
            self._opencode_btn.setEnabled(False)
            self._server_panel.set_controls_enabled(False)
            self._playwright_panel.set_controls_enabled(False)
            self._scripts_panel.set_controls_enabled(False)
            self._scripts_panel.set_project(None)
            self._git_panel.set_project(None)
            self._evidence_panel.set_project(None)
            self._log_panel.clear()
            return

        self._title_label.setText(project.name)
        self._title_label.setToolTip(project.name)
        path_url = f"📁 {project.path}   |   🔗 {project.server.url if project.server.enabled else 'Server disabled'}"
        self._path_label.setText(path_url)
        self._path_label.setToolTip(path_url)
        self._explorer_btn.setEnabled(True)
        self._terminal_btn.setEnabled(True)
        self._vscode_btn.setEnabled(True)
        self._opencode_btn.setEnabled(True)

        self._server_panel.set_project(project)
        self._playwright_panel.set_project(project)
        self._scripts_panel.set_project(project)
        self._scripts_panel.set_controls_enabled(True)

        self._git_panel.set_project(project)
        self._evidence_panel.set_project(project)

        # Update panels & badges with current manager states
        sm = self._server_managers.get(index)
        if sm:
            self._server_panel.update_state(sm.state)
            self._server_panel.set_active_port(
                sm.active_port, sm.active_url,
                is_mismatch=sm.is_port_mismatch,
                configured_port=project.server.port
            )
            self._update_server_badge(sm.state, sm.active_port, is_mismatch=sm.is_port_mismatch)
        
        pm = self._playwright_managers.get(index)
        if pm:
            self._playwright_panel.update_state(pm.state)
            self._update_pw_badge(pm.state)

        sc_m = self._script_managers.get(index)
        if sc_m and sc_m.is_running():
            self._scripts_panel.set_script_running(sc_m.active_script_name, True)
        else:
            self._scripts_panel.set_script_running(None, False)

        # Git badge
        self._update_git_badge(project)

        # Restore logs for this project
        self._log_panel.clear()
        logs = self._project_logs.get(index, [])
        for line, is_error in logs:
            if is_error:
                self._log_panel.append_error(line)
            else:
                self._log_panel.append_log(line)

    def _update_server_badge(self, state: ServerState, port: int = 0, is_mismatch: bool = False):
        c = self.theme_manager.get_colors()
        if state == ServerState.RUNNING:
            if is_mismatch and port:
                text = f"Server: Running (:{port} ⚠️)"
                bg = c["warning_bg"]
                fg = c["warning"]
            elif port:
                text = f"Server: Running (:{port})"
                bg = c["success_bg"]
                fg = c["success"]
            else:
                text = "Server: Running"
                bg = c["success_bg"]
                fg = c["success"]
        else:
            badge_config = {
                ServerState.STOPPED: ("Server: Stopped", c["badge_bg"], c["text_muted"]),
                ServerState.STARTING: ("Server: Starting...", c["warning_bg"], c["warning"]),
                ServerState.STOPPING: ("Server: Stopping...", c["warning_bg"], c["warning"]),
                ServerState.ERROR: ("Server: Error", c["danger_bg"], c["danger"]),
            }
            text, bg, fg = badge_config.get(state, ("Server: Stopped", c["badge_bg"], c["text_muted"]))

        self._server_badge.setText(text)
        self._server_badge.setStyleSheet(
            f"background-color: {bg}; color: {fg}; font-size: 12px; "
            f"font-weight: 600; border-radius: 12px; padding: 4px 12px; border: 1px solid {fg};"
        )

    def _update_pw_badge(self, state: PlaywrightState):
        c = self.theme_manager.get_colors()
        badge_config = {
            PlaywrightState.IDLE: ("Playwright: Idle", c["badge_bg"], c["text_muted"]),
            PlaywrightState.STARTING: ("Playwright: Starting...", c["warning_bg"], c["warning"]),
            PlaywrightState.RUNNING: ("Playwright: Running", c["primary_light"], c["primary"]),
            PlaywrightState.PASSED: ("Playwright: PASSED ✓", c["success_bg"], c["success"]),
            PlaywrightState.FAILED: ("Playwright: FAILED ✗", c["danger_bg"], c["danger"]),
            PlaywrightState.ERROR: ("Playwright: Error", c["danger_bg"], c["danger"]),
        }
        text, bg, fg = badge_config.get(state, ("Playwright: Idle", c["badge_bg"], c["text_muted"]))
        self._pw_badge.setText(text)
        self._pw_badge.setStyleSheet(
            f"background-color: {bg}; color: {fg}; font-size: 12px; "
            f"font-weight: 600; border-radius: 12px; padding: 4px 12px; border: 1px solid {fg};"
        )

    def _update_git_badge(self, project: Optional[Project] = None):
        """Update the Git branch/status badge in the header card."""
        c = self.theme_manager.get_colors()
        if not project:
            self._git_badge.setText("Git: None")
            self._git_badge.setStyleSheet(
                f"background-color: {c['badge_bg']}; color: {c['text_muted']}; font-size: 12px; "
                f"font-weight: 600; border-radius: 12px; padding: 4px 12px; border: 1px solid {c['border']};"
            )
            return

        info = get_git_info(project.path)
        if not info["is_repo"]:
            self._git_badge.setText("Git: None")
            self._git_badge.setStyleSheet(
                f"background-color: {c['badge_bg']}; color: {c['text_muted']}; font-size: 12px; "
                f"font-weight: 600; border-radius: 12px; padding: 4px 12px; border: 1px solid {c['border']};"
            )
            return

        branch = info["branch"] or "unknown"
        dirty = info["is_dirty"]
        dirty_marker = " •" if dirty else ""
        text = f"🌿 {branch}{dirty_marker}"
        if dirty:
            bg = c["warning_bg"]
            fg = c["warning"]
        else:
            bg = c["success_bg"]
            fg = c["success"]

        self._git_badge.setText(text)
        self._git_badge.setStyleSheet(
            f"background-color: {bg}; color: {fg}; font-size: 12px; "
            f"font-weight: 600; border-radius: 12px; padding: 4px 12px; border: 1px solid {fg};"
        )

    def _append_project_log(self, idx: int, msg: str, is_error: bool = False):
        if idx not in self._project_logs:
            self._project_logs[idx] = []
        self._project_logs[idx].append((msg, is_error))
        
        # Cap memory buffer at 2000 lines per project
        if len(self._project_logs[idx]) > 2000:
            self._project_logs[idx] = self._project_logs[idx][-2000:]

        if self._current_index == idx:
            if is_error:
                self._log_panel.append_error(msg)
            else:
                self._log_panel.append_log(msg)

    def _on_server_state_changed(self, idx: int, state: ServerState):
        self._sidebar.update_status(idx, state)
        self._update_tray_menu()

        # Tray notifications for relevant server transitions
        if state in (ServerState.RUNNING, ServerState.ERROR):
            project = self.config_manager.get_project(idx)
            if project:
                if state == ServerState.RUNNING:
                    sm = self._server_managers.get(idx)
                    port = sm.active_port if sm else project.server.port
                    self._notify(project.name, f"Server running on port {port}", ToastLevel.SUCCESS)
                else:
                    self._notify(project.name, "Server failed to start", ToastLevel.ERROR)

        if self._current_index == idx:
            sm = self._server_managers.get(idx)
            port = sm.active_port if sm else 0
            is_mismatch = sm.is_port_mismatch if sm else False
            project = self.config_manager.get_project(idx)
            cfg_port = project.server.port if project else 0

            self._server_panel.update_state(state)
            if sm:
                self._server_panel.set_active_port(port, sm.active_url, is_mismatch=is_mismatch, configured_port=cfg_port)
            self._update_server_badge(state, port, is_mismatch=is_mismatch)
            self._status_bar.showMessage(f"Server {state.name.lower()}", 3000)

    def _on_port_detected(self, idx: int, port: int, url: str):
        """Handle dynamic port detection from server log output."""
        if self._current_index == idx:
            sm = self._server_managers.get(idx)
            project = self.config_manager.get_project(idx)
            if sm and project:
                is_mismatch = sm.is_port_mismatch
                self._update_server_badge(sm.state, port, is_mismatch=is_mismatch)
                self._server_panel.set_active_port(port, url, is_mismatch=is_mismatch, configured_port=project.server.port)
                self._path_label.setText(f"📁 {project.path}   |   🔗 {url}")
                self._path_label.setToolTip(f"📁 {project.path}   |   🔗 {url}")

    def _on_port_mismatch(self, idx: int, cfg_port: int, det_port: int, url: str):
        """Called when newly started localhost port differs from configured port."""
        if self._current_index == idx:
            sm = self._server_managers.get(idx)
            project = self.config_manager.get_project(idx)
            if sm and project:
                self._update_server_badge(sm.state, det_port, is_mismatch=True)
                self._server_panel.set_active_port(det_port, url, is_mismatch=True, configured_port=cfg_port)
                self._path_label.setText(
                    f"📁 {project.path}   |   🔗 {url}  (⚠️ differs from configured :{cfg_port})"
                )
                self._path_label.setToolTip(
                    f"📁 {project.path}   |   🔗 {url}  (⚠️ differs from configured :{cfg_port})"
                )
            self._status_bar.showMessage(
                f"⚠️ Server started on port {det_port} (configured: {cfg_port})", 6000
            )
            project = self.config_manager.get_project(idx)
            if project:
                self._append_project_log(idx, f"⚠️ Port changed: running on :{det_port} (configured :{cfg_port})", is_error=True)

    def _on_update_project_port(self, new_port: int, new_url: str):
        """Save the auto-detected port & URL into the project configuration."""
        if self._current_index < 0:
            return
        project = self.config_manager.get_project(self._current_index)
        if not project:
            return

        project.server.port = new_port
        project.server.url = new_url
        self.config_manager.update_project(self._current_index, project)

        self._status_bar.showMessage(f"Saved port {new_port} to project configuration", 4000)
        self._append_project_log(self._current_index, f"✓ Project configuration updated to port {new_port} ({new_url})", is_error=False)

    def _on_playwright_state_changed(self, idx: int, state: PlaywrightState):
        # Tray notification on failed test runs
        if state in (PlaywrightState.FAILED, PlaywrightState.PASSED, PlaywrightState.ERROR):
            project = self.config_manager.get_project(idx)
            if project:
                if state == PlaywrightState.PASSED:
                    self._notify(project.name, "All tests passed ✓", ToastLevel.SUCCESS)
                elif state == PlaywrightState.FAILED:
                    self._notify(project.name, "Test run failed", ToastLevel.ERROR)
                else:
                    self._notify(project.name, "Playwright error occurred", ToastLevel.ERROR)

        if self._current_index == idx:
            self._playwright_panel.update_state(state)
            self._update_pw_badge(state)
            self._status_bar.showMessage(f"Playwright: {state.name.lower()}", 3000)

    def _on_app_log_message(self, text: str, is_error: bool):
        """Route application-level log messages to the App Log panel."""
        if is_error:
            self._app_log_panel.append_error(text)
        else:
            self._app_log_panel.append_log(text)

    def _on_search_shortcut(self):
        """Intelligently route Ctrl+F to the active panel search/filter."""
        # App Log dialog abierto tiene prioridad (vista global activa)
        if self._applog_dialog.isVisible():
            self._app_log_panel.focus_search()
            return
        current_tab = self._tab_widget.currentWidget()
        if current_tab == self._log_panel:
            self._log_panel.focus_search()
        else:
            self._sidebar.focus_search()

    def _show_global_monitor(self):
        """Open/raise the project-independent Monitor window."""
        self._refresh_monitor_data()
        self._monitor_dialog.reveal()
        self._status_bar.showMessage("Global Monitor opened", 3000)

    def _show_app_log(self):
        """Open/raise the application log window."""
        self._applog_dialog.reveal()

    def _open_current_folder(self):
        self._open_folder_for_index(self._current_index)

    def _open_current_in_terminal(self):
        self._open_terminal_for_index(self._current_index)

    def _open_terminal_for_index(self, index: int):
        if index < 0:
            return
        project = self.config_manager.get_project(index)
        if not (project and os.path.exists(project.path)):
            return

        wt_bin = shutil.which("wt") or shutil.which("wt.exe")
        powershell_bin = shutil.which("powershell") or shutil.which("pwsh") or "powershell.exe"

        if wt_bin:
            QProcess.startDetached(wt_bin, ["-d", project.path])
        elif os.name == "nt":
            QProcess.startDetached(powershell_bin, ["-NoExit", "-Command", f"Set-Location -LiteralPath '{project.path}'"])
        else:
            for term in ["x-terminal-emulator", "gnome-terminal", "konsole", "alacritty", "kitty", "xterm"]:
                if shutil.which(term):
                    QProcess.startDetached(term, ["--working-directory", project.path])
                    break
            else:
                QProcess.startDetached("xterm", ["-e", f"cd '{project.path}' && bash"])

        self._status_bar.showMessage(f"Opening Terminal in '{project.name}'...", 3000)
        self._append_project_log(index, f"💻 Opening Terminal in {project.path}...", is_error=False)

    def _on_toggle_pin(self, index: int):
        if index < 0:
            return
        self.config_manager.toggle_pin_project(index)
        project = self.config_manager.get_project(index)
        if project:
            status = "pinned to top" if project.pinned else "unpinned"
            self._status_bar.showMessage(f"Project '{project.name}' {status}", 3000)

    def _open_current_in_vscode(self):
        self._open_vscode_for_index(self._current_index)

    def _open_vscode_for_index(self, index: int):
        if index < 0:
            return
        project = self.config_manager.get_project(index)
        if project and os.path.exists(project.path):
            code_bin = shutil.which("code") or "code"
            QProcess.startDetached(code_bin, [project.path])
            self._status_bar.showMessage(f"Opening '{project.name}' in VS Code...", 3000)

    def _open_current_in_opencode(self):
        self._open_opencode_for_index(self._current_index)

    def _open_opencode_for_index(self, index: int):
        if index < 0:
            return
        project = self.config_manager.get_project(index)
        if not (project and os.path.exists(project.path)):
            return

        wt_bin = shutil.which("wt") or shutil.which("wt.exe")
        if wt_bin:
            QProcess.startDetached(wt_bin, ["-d", project.path, "cmd.exe", "/k", "opencode"])
        elif os.name == "nt":
            QProcess.startDetached("cmd.exe", ["/c", "start", "OpenCode", "/d", project.path, "cmd.exe", "/k", "opencode"])
        else:
            for term in ["x-terminal-emulator", "gnome-terminal", "konsole", "alacritty", "kitty", "xterm"]:
                if shutil.which(term):
                    QProcess.startDetached(term, ["--working-directory", project.path, "-e", "opencode"])
                    break
            else:
                QProcess.startDetached("opencode", [], project.path)

        self._status_bar.showMessage(f"Opening OpenCode in '{project.name}'...", 3000)
        self._append_project_log(index, f"🚀 Launching OpenCode in {project.path}...", is_error=False)

    def _open_folder_for_index(self, index: int):
        if index < 0:
            return
        project = self.config_manager.get_project(index)
        if project and os.path.exists(project.path):
            QDesktopServices.openUrl(QUrl.fromLocalFile(project.path))

    def _open_url_for_index(self, index: int):
        if index < 0:
            return
        # Prefer the live-detected URL from the server manager
        sm = self._server_managers.get(index)
        if sm and sm.active_url:
            QDesktopServices.openUrl(QUrl(sm.active_url))
            return
        project = self.config_manager.get_project(index)
        if project and project.server.url:
            QDesktopServices.openUrl(QUrl(project.server.url))

    def _start_server_for_index(self, index: int):
        sm = self._server_managers.get(index)
        if sm:
            sm.start()

    def _stop_server_for_index(self, index: int):
        sm = self._server_managers.get(index)
        if sm:
            sm.stop()

    def _restart_server_for_index(self, index: int):
        sm = self._server_managers.get(index)
        if sm:
            sm.restart()

    def _run_tests_for_index(self, index: int):
        pm = self._playwright_managers.get(index)
        if pm:
            pm.run_tests()

    def _on_add_project(self):
        existing_ports = self.config_manager.get_configured_ports()
        dialog = ProjectDialog(existing_ports=existing_ports, parent=self)
        if dialog.exec():
            new_project = dialog.get_project()
            self.config_manager.add_project(new_project)
            new_idx = self.config_manager.project_count() - 1
            if new_idx >= 0:
                self._sidebar._list.setCurrentRow(new_idx)
                self._on_project_selected(new_idx)

    def _on_edit_project(self, index: int):
        if index < 0:
            return
        project = self.config_manager.get_project(index)
        if not project:
            return
        
        existing_ports = self.config_manager.get_configured_ports()
        dialog = ProjectDialog(project=project, existing_ports=existing_ports, parent=self)
        if dialog.exec():
            updated = dialog.get_project()
            self.config_manager.update_project(index, updated)

    def _on_auto_assign_unique_ports(self):
        count = self.config_manager.auto_assign_unique_ports(start_port=5173)
        if count > 0:
            self._status_bar.showMessage(f"Assigned unique sequential ports to {count} project(s)", 5000)
            QMessageBox.information(
                self, "Puertos Únicos Asignados",
                f"Successfully assigned unique ports to {count} project(s) starting from :5173.\n\n"
                "Los proyectos ahora pueden ejecutarse concurrentemente sin conflictos de puerto."
            )
        else:
            self._status_bar.showMessage("All projects already have unique ports", 3000)

    def _on_remove_project(self, index: int):
        if index < 0:
            return
        project = self.config_manager.get_project(index)
        if not project:
            return

        reply = QMessageBox.question(
            self,
            "Confirmar Eliminación",
            f"Confirmar eliminación de '{project.name}'\ del gestor\n\n(No se eliminarán archivos locales)",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
            QMessageBox.StandardButton.No
        )
        if reply == QMessageBox.StandardButton.Yes:
            sm = self._server_managers.get(index)
            if sm:
                sm.stop()
            pm = self._playwright_managers.get(index)
            if pm:
                pm.stop()
            sc_m = self._script_managers.get(index)
            if sc_m:
                sc_m.stop()
            self.config_manager.remove_project(index)

    def _on_run_script(self, name: str, command: str):
        if self._current_index < 0:
            return
        sc_m = self._script_managers.get(self._current_index)
        if sc_m:
            sc_m.run_script(name, command)

    def _on_stop_script(self):
        if self._current_index < 0:
            return
        sc_m = self._script_managers.get(self._current_index)
        if sc_m:
            sc_m.stop()

    def _on_script_started(self, index: int, name: str):
        if index == self._current_index:
            self._scripts_panel.set_script_running(name, True)
        self._status_bar.showMessage(f"Running script '{name}'...", 3000)

    def _on_script_finished(self, index: int, name: str, exit_code: int):
        if index == self._current_index:
            self._scripts_panel.set_script_running(None, False)
        status_txt = "finished successfully" if exit_code == 0 else f"exited with code {exit_code}"
        self._status_bar.showMessage(f"Script '{name}' {status_txt}", 4000)

        # Tray notification when a script fails
        if exit_code != 0:
            project = self.config_manager.get_project(index)
            if project:
                self._notify(project.name, f"Script '{name}' exited with code {exit_code}", ToastLevel.ERROR)

    def _on_start_server(self):
        self._start_server_for_index(self._current_index)

    def _on_stop_server(self):
        self._stop_server_for_index(self._current_index)

    def _on_restart_server(self):
        self._restart_server_for_index(self._current_index)

    def _on_run_tests(self):
        self._run_tests_for_index(self._current_index)

    def _on_run_ui(self):
        pm = self._playwright_managers.get(self._current_index)
        if pm:
            pm.run_ui()

    def _on_run_debug(self):
        pm = self._playwright_managers.get(self._current_index)
        if pm:
            pm.run_debug()

    def _on_show_report(self):
        pm = self._playwright_managers.get(self._current_index)
        if pm:
            pm.show_report()

    def _on_stop_playwright(self):
        pm = self._playwright_managers.get(self._current_index)
        if pm:
            pm.stop()

    def _on_start_all(self):
        started_count = 0
        for idx, sm in self._server_managers.items():
            project = self.config_manager.get_project(idx)
            if project and project.server.enabled:
                sm.start()
                started_count += 1
        self._status_bar.showMessage(f"Started {started_count} server(s)", 3000)

    def _on_stop_all(self):
        stopped_count = 0
        for sm in self._server_managers.values():
            if sm.state != ServerState.STOPPED:
                sm.stop()
                stopped_count += 1
        for sc_m in self._script_managers.values():
            sc_m.stop()
        self._status_bar.showMessage(f"Stopped {stopped_count} server(s)", 3000)

    def _on_reload_config(self):
        """Reloads projects configuration from disk and refreshes the UI."""
        self.config_manager.load()
        self._status_bar.showMessage("Projects and configuration reloaded from disk", 4000)
        AppLogger.instance().info("Configuration reloaded from disk")

    def _on_restart_app(self):
        """Gracefully stops all servers and restarts the Dev Manager application."""
        reply = QMessageBox.question(
            self,
            "Reiniciar Dev Manager",
            "Confirmar reinicio de Local Dev Manager\n\nTodos los servidores, ejecutores de pruebas y scripts activos se detendrán",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
            QMessageBox.StandardButton.No
        )
        if reply == QMessageBox.StandardButton.Yes:
            for sm in self._server_managers.values():
                sm.stop()
            for pm in self._playwright_managers.values():
                pm.stop()
            for sc_m in self._script_managers.values():
                sc_m.stop()

            QProcess.startDetached(sys.executable, sys.argv)
            QApplication.quit()

    def _show_about(self):
        QMessageBox.about(
            self,
            "Acerca de Local Dev Manager",
            "<h3>⚡ Local Dev Manager</h3>"
            "<p>Una herramienta de escritorio moderna para gestionar servidores de desarrollo local, scripts personalizados y flujos de trabajo de Playwright.</p>"
            "<p>Construido con Python, PySide6, sistema de vectores Reicon (cpu-bolt) y soporte para tema Claro/Oscuro.</p>"
        )

    # ---------------- Settings ----------------

    def _on_open_settings(self):
        dialog = SettingsDialog(self)
        dialog.exec()
        # Los toggles persisten/aplican en vivo vía AppSettings.setting_changed

    # ---------------- Notifications routing ----------------

    def _notify(self, title: str, message: str, level: ToastLevel = ToastLevel.INFO):
        """Visible window → in-app toast. Hidden/minimized → tray notification.
        Tray notifications have a 3-second cooldown to prevent spam."""
        if self.isVisible():
            self._toast_manager.show(title, message, level)
        else:
            now = time.time()
            is_error = level in (ToastLevel.ERROR, ToastLevel.WARNING)
            # Cooldown: skip if same notification within 3s
            if now - self._last_tray_notification < 3.0:
                # Remember the latest one; it will show after cooldown
                self._pending_tray_notify = (title, message, is_error)
                return
            self._last_tray_notification = now
            self._tray.show_notification(title, message, is_error=is_error)
            # Schedule pending notification check
            if self._pending_tray_notify:
                QTimer.singleShot(3200, self._flush_pending_tray_notify)

    def _flush_pending_tray_notify(self):
        if self._pending_tray_notify:
            title, message, is_error = self._pending_tray_notify
            self._pending_tray_notify = None
            self._last_tray_notification = time.time()
            self._tray.show_notification(title, message, is_error=is_error)

    # ---------------- Uptime ----------------

    def _update_uptime(self):
        idx = self._current_index
        sm = self._server_managers.get(idx) if idx >= 0 else None
        if sm and sm.state == ServerState.RUNNING and sm.started_at:
            self._server_panel.set_uptime(time.time() - sm.started_at)
        else:
            self._server_panel.set_uptime(None)

    # ---------------- Git ----------------

    def _on_git_command_finished(self, name: str, exit_code: int, project_name: str = "?"):
        if exit_code == 0:
            self._status_bar.showMessage(f"Git {name.lower()} completed for '{project_name}'", 4000)
        else:
            self._notify(project_name, f"Git {name.lower()} failed (exit {exit_code})", ToastLevel.ERROR)

    # ---------------- Monitor ----------------

    def _refresh_monitor_data(self):
        projects = self.config_manager.get_projects()

        # Ports
        port_rows = []
        running_ports: dict[int, str] = {}  # port → project name (de managers RUNNING)
        for idx, sm in self._server_managers.items():
            if sm.state == ServerState.RUNNING and sm.active_port > 0:
                proj = projects[idx] if idx < len(projects) else None
                running_ports[sm.active_port] = proj.name if proj else f"#{idx}"

        for idx, project in enumerate(projects):
            if not project.server.enabled or project.server.port <= 0:
                continue
            port = project.server.port
            if port in running_ports:
                port_rows.append({"index": idx, "name": project.name, "port": port,
                                  "state": "ours", "owner_name": running_ports[port]})
                continue
            owner = sysmon.get_port_owner(port)
            if owner:
                port_rows.append({"index": idx, "name": project.name, "port": port,
                                  "state": "foreign", "owner_name": owner.name, "owner_pid": owner.pid})
            else:
                port_rows.append({"index": idx, "name": project.name, "port": port, "state": "free"})
        self._monitor_panel.update_port_rows(port_rows)

        # Resources
        res_rows = []
        if self.app_settings.get(KEY_POLLING_ENABLED, True):
            for idx, sm in self._server_managers.items():
                if sm.state != ServerState.RUNNING:
                    continue
                pid = sm._runner.pid() if hasattr(sm, "_runner") else 0
                if pid <= 0:
                    continue
                usage = sysmon.get_process_tree_usage(pid)
                proj = projects[idx] if idx < len(projects) else None
                if usage:
                    res_rows.append({
                        "name": proj.name if proj else f"#{idx}",
                        "pid": usage.pid, "children": usage.children,
                        "cpu": usage.cpu_percent, "rss": usage.rss_mb,
                    })
        self._monitor_panel.update_resources(res_rows)

    def _on_monitor_kill(self, pid: int):
        if pid <= 0:
            return
        reply = QMessageBox.question(
            self, "Terminar Proceso",
            f"Confirmar terminación del árbol de procesos con PID {pid}\n\nEsta acción no se puede deshacer",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
            QMessageBox.StandardButton.No,
        )
        if reply != QMessageBox.StandardButton.Yes:
            return
        ok, msg = sysmon.kill_tree(pid)
        project = self.config_manager.get_project(self._current_index)
        pname = project.name if project else "System"
        if ok:
            self._notify(pname, f"Process tree {pid} terminated", ToastLevel.SUCCESS)
            self._append_project_log(self._current_index, f"⚠ Killed process tree PID {pid}: {msg}")
        else:
            self._notify(pname, f"Failed killing PID {pid}: {msg}", ToastLevel.ERROR)
            self._append_project_log(self._current_index, f"Failed killing PID {pid}: {msg}", is_error=True)
        self._refresh_monitor_data()

    # ---------------- Evidence / Trace viewer ----------------

    def _open_trace_viewer(self, trace_path: str):
        project = self.config_manager.get_project(self._current_index)
        if not project:
            return
        if self._trace_runner is not None and self._trace_runner.is_running():
            self._append_project_log(self._current_index, "Trace viewer already open", is_error=True)
            return
        from app.process.runner import ProcessRunner
        self._trace_runner = ProcessRunner(self)
        self._trace_runner.output_ready.connect(
            lambda m: self._append_project_log(self._current_index, m)
        )
        self._trace_runner.error_ready.connect(
            lambda m: self._append_project_log(self._current_index, m, is_error=True)
        )
        self._append_project_log(self._current_index, f"🔍 Opening trace viewer: {trace_path}")
        self._trace_runner.start(f'npx playwright show-trace "{trace_path}"', project.path)
        self._status_bar.showMessage("Opening Playwright Trace Viewer...", 4000)

    # ---------------- Settings changes ----------------

    def _on_setting_changed(self, key: str, value):
        if key == KEY_POLLING_ENABLED:
            self._monitor_panel.set_polling_enabled(bool(value))
            if not value:
                self._monitor_panel.update_resources([])

    def resizeEvent(self, event):
        super().resizeEvent(event)
        if self._toast_manager:
            self._toast_manager.reposition()

    def closeEvent(self, event: QCloseEvent):
        # Minimize to tray unless a real exit was requested from the tray menu
        # (or the system has no tray available).
        if not self._is_force_exit and QSystemTrayIcon.isSystemTrayAvailable():
            event.ignore()
            self.hide()
            return

        # Real exit: stop all servers, test runners, and background scripts gracefully
        for sm in self._server_managers.values():
            sm.stop()
        for pm in self._playwright_managers.values():
            pm.stop()
        for sc_m in self._script_managers.values():
            sc_m.stop()
        if self._trace_runner is not None and self._trace_runner.is_running():
            self._trace_runner.stop()
        self._tray.hide()
        event.accept()
