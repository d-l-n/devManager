from typing import List, Optional
from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QListWidget, QListWidgetItem,
    QPushButton, QHBoxLayout, QLabel, QLineEdit, QMenu, QToolButton
)
from PySide6.QtCore import Signal, Qt, QSize, QPoint
from PySide6.QtGui import QFont, QAction
from app.models.project import ServerState, Project
from app.ui.icons import get_icon
from app.ui.theme import ThemeManager, ThemeMode, ui_font


class ProjectListWidget(QWidget):
    project_selected = Signal(int)
    add_requested = Signal()
    edit_requested = Signal(int)
    remove_requested = Signal(int)

    # Global (project-independent) views
    monitor_requested = Signal()
    applog_requested = Signal()

    # Context menu action signals
    start_requested = Signal(int)
    stop_requested = Signal(int)
    restart_requested = Signal(int)
    open_url_requested = Signal(int)
    open_folder_requested = Signal(int)
    open_terminal_requested = Signal(int)
    open_vscode_requested = Signal(int)
    open_opencode_requested = Signal(int)
    run_tests_requested = Signal(int)
    toggle_pin_requested = Signal(int)

    def __init__(self, parent=None):
        super().__init__(parent)
        self._projects_cache: List[Project] = []
        self._states_cache: dict[int, ServerState] = {}
        self._setup_ui()
        self.apply_theme(ThemeManager.instance().mode)

    def _setup_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(10, 12, 10, 12)
        layout.setSpacing(10)

        # Header with dynamic counter + icon actions (Add / Edit / Remove)
        header_layout = QHBoxLayout()
        self._header_label = QLabel("PROJECTS")
        self._header_label.setFont(ui_font(11, QFont.Weight.Bold))

        self._count_badge = QLabel("0")
        self._count_badge.setAlignment(Qt.AlignmentFlag.AlignCenter)

        header_layout.addWidget(self._header_label)
        header_layout.addStretch()
        header_layout.addWidget(self._count_badge)

        header_layout.addSpacing(6)
        self._add_btn = QToolButton()
        self._add_btn.setIcon(get_icon("plus", "#6366f1"))
        self._add_btn.setIconSize(QSize(15, 15))
        self._add_btn.setToolTip("Add Project (Ctrl+N)")
        self._add_btn.setAutoRaise(True)

        self._edit_btn = QToolButton()
        self._edit_btn.setIcon(get_icon("edit", "#94a3b8"))
        self._edit_btn.setIconSize(QSize(15, 15))
        self._edit_btn.setToolTip("Edit Selected Project (Ctrl+E or double-click)")
        self._edit_btn.setAutoRaise(True)
        self._edit_btn.setEnabled(False)

        self._remove_btn = QToolButton()
        self._remove_btn.setIcon(get_icon("trash", "#94a3b8"))
        self._remove_btn.setIconSize(QSize(15, 15))
        self._remove_btn.setToolTip("Remove Selected Project (Del)")
        self._remove_btn.setAutoRaise(True)
        self._remove_btn.setEnabled(False)

        for btn in (self._add_btn, self._edit_btn, self._remove_btn):
            btn.setFixedSize(26, 26)
            header_layout.addWidget(btn)

        self._add_btn.clicked.connect(lambda: self.add_requested.emit())
        self._edit_btn.clicked.connect(self._on_edit)
        self._remove_btn.clicked.connect(self._on_remove)

        layout.addLayout(header_layout)

        # Search Bar
        self._search_input = QLineEdit()
        self._search_input.setPlaceholderText("Filter projects... (Ctrl+F)")
        self._search_input.setClearButtonEnabled(True)
        self._search_input.addAction(get_icon("search", "#64748b"), QLineEdit.ActionPosition.LeadingPosition)
        self._search_input.textChanged.connect(self._filter_projects)
        layout.addWidget(self._search_input)

        # Status filter chips (All / Running / Stopped)
        self._status_filter = "all"
        filter_row = QHBoxLayout()
        filter_row.setSpacing(6)
        self._chip_buttons = {}
        for key, label in (("all", "All"), ("running", "Running"), ("stopped", "Stopped")):
            chip = QPushButton(label)
            chip.setCheckable(True)
            chip.setFixedHeight(26)
            chip.clicked.connect(lambda _c, k=key: self._on_status_chip(k))
            filter_row.addWidget(chip, stretch=1)
            self._chip_buttons[key] = chip
        self._chip_buttons["all"].setChecked(True)
        layout.addLayout(filter_row)

        # Projects List
        self._list = QListWidget()
        self._list.setFont(ui_font(13, QFont.Weight.Normal))
        self._list.setIconSize(QSize(14, 14))
        self._list.currentRowChanged.connect(self._on_selection_changed)
        self._list.itemDoubleClicked.connect(self._on_item_double_clicked)
        self._list.setContextMenuPolicy(Qt.ContextMenuPolicy.CustomContextMenu)
        self._list.customContextMenuRequested.connect(self._show_context_menu)
        layout.addWidget(self._list)

        # Global (project-independent) views
        global_row = QHBoxLayout()
        global_row.setSpacing(6)

        self._monitor_btn = QPushButton(" Monitor")
        self._monitor_btn.setIcon(get_icon("activity", "#6366f1"))
        self._monitor_btn.setIconSize(QSize(14, 14))
        self._monitor_btn.setFixedHeight(32)
        self._monitor_btn.setToolTip("Global port & resource monitor (Ctrl+Alt+M)")
        self._monitor_btn.clicked.connect(self.monitor_requested.emit)
        global_row.addWidget(self._monitor_btn, stretch=1)

        self._applog_btn = QPushButton(" App Log")
        self._applog_btn.setIcon(get_icon("info", "#6366f1"))
        self._applog_btn.setIconSize(QSize(14, 14))
        self._applog_btn.setFixedHeight(32)
        self._applog_btn.setToolTip("Application log (Ctrl+Alt+L)")
        self._applog_btn.clicked.connect(self.applog_requested.emit)
        global_row.addWidget(self._applog_btn, stretch=1)

        layout.addLayout(global_row)

    def apply_theme(self, mode: ThemeMode):
        c = ThemeManager.instance().get_colors(mode)
        
        self._header_label.setStyleSheet(f"color: {c['primary']}; letter-spacing: 0.8px; font-weight: bold;")
        self._count_badge.setStyleSheet(
            f"background-color: {c['badge_bg']}; color: {c['text_secondary']}; font-size: 12px; "
            f"font-weight: 600; border-radius: 9px; padding: 2px 8px; border: 1px solid {c['border']};"
        )
        self._search_input.setStyleSheet(
            f"QLineEdit {{ background-color: {c['bg_surface']}; color: {c['text_primary']}; border: 1px solid {c['border']}; "
            f"border-radius: 8px; padding: 6px 10px; font-size: 12px; }} "
            f"QLineEdit:focus {{ border: 1px solid {c['primary']}; }}"
        )
        self._list.setStyleSheet(
            f"QListWidget {{ background-color: {c['bg_surface']}; border: 1px solid {c['border']}; "
            f"border-radius: 8px; outline: none; padding: 4px; }} "
            f"QListWidget::item {{ padding: 8px 12px; min-height: 32px; color: {c['text_primary']}; border-radius: 6px; margin-bottom: 2px; }} "
            f"QListWidget::item:selected {{ background-color: {c['bg_active']}; color: {c['primary']}; font-weight: bold; border-left: 3px solid {c['primary']}; }} "
            f"QListWidget::item:hover:!selected {{ background-color: {c['bg_elevated']}; }}"
        )

        icon_color = c["icon_color"]
        self._add_btn.setIcon(get_icon("plus", c["primary"]))
        self._edit_btn.setIcon(get_icon("edit", icon_color))
        self._remove_btn.setIcon(get_icon("trash", icon_color))

        btn_style = (
            f"QToolButton {{ background-color: transparent; border: 1px solid transparent; border-radius: 6px; padding: 1px; }}"
            f"QToolButton:hover {{ background-color: {c['primary_light']}; border-color: {c['primary']}; }}"
            f"QToolButton:pressed {{ background-color: {c['bg_active']}; }}"
            f"QToolButton:disabled {{ color: {c['text_muted']}; }}"
        )
        self._add_btn.setStyleSheet(btn_style)
        self._edit_btn.setStyleSheet(btn_style)
        self._remove_btn.setStyleSheet(btn_style)

        global_btn_style = (
            f"QPushButton {{ background-color: {c['bg_elevated']}; color: {c['text_secondary']}; "
            f"font-size: 12px; font-weight: 600; border: 1px solid {c['border']}; border-radius: 8px; padding: 5px 10px; }} "
            f"QPushButton:hover {{ background-color: {c['bg_active']}; color: {c['primary']}; border-color: {c['primary']}; }}"
        )
        self._monitor_btn.setStyleSheet(global_btn_style)
        self._applog_btn.setStyleSheet(global_btn_style)
        self._monitor_btn.setIcon(get_icon("activity", c["primary"]))
        self._applog_btn.setIcon(get_icon("info", c["primary"]))

        chip_checked = (
            f"QPushButton {{ background-color: {c['primary_light']}; color: {c['primary']}; "
            f"font-size: 11px; font-weight: 700; border: 1px solid {c['primary']}; border-radius: 13px; padding: 2px; }}"
        )
        chip_unchecked = (
            f"QPushButton {{ background-color: {c['bg_elevated']}; color: {c['text_secondary']}; "
            f"font-size: 11px; font-weight: 600; border: 1px solid {c['border']}; border-radius: 13px; padding: 2px; }}"
            f"QPushButton:hover {{ border-color: {c['border_focus']}; }}"
        )
        for key, chip in self._chip_buttons.items():
            chip.setStyleSheet(chip_checked if chip.isChecked() else chip_unchecked)

    def set_projects(self, projects: list[Project]):
        self._projects_cache = projects
        self._count_badge.setText(str(len(projects)))
        self._filter_projects(self._search_input.text())

    def _filter_projects(self, query: str = ""):
        query = query.strip().lower()
        currently_selected_idx = self.get_selected_index()
        self._list.clear()

        # Filter and sort pinned projects to the top
        matching_indices = [
            idx for idx, project in enumerate(self._projects_cache)
            if (not query or query in project.name.lower() or query in project.path.lower())
            and self._matches_status_filter(self._states_cache.get(idx, ServerState.STOPPED))
        ]
        sorted_indices = sorted(matching_indices, key=lambda i: not self._projects_cache[i].pinned)

        for idx in sorted_indices:
            project = self._projects_cache[idx]
            display_name = f"📌 {project.name}" if project.pinned else project.name
            item = QListWidgetItem(display_name)
            if project.pinned:
                item.setToolTip(f"{project.name} (Pinned to top)")
            state = self._states_cache.get(idx, ServerState.STOPPED)
            item.setIcon(get_icon(self._get_status_icon(state)))
            item.setData(Qt.ItemDataRole.UserRole, idx)
            self._list.addItem(item)

        # Restore selection
        if self._list.count() > 0:
            found = False
            for r in range(self._list.count()):
                if self._list.item(r).data(Qt.ItemDataRole.UserRole) == currently_selected_idx:
                    self._list.setCurrentRow(r)
                    found = True
                    break
            if not found:
                self._list.setCurrentRow(0)
        else:
            self._on_selection_changed(-1)

    def _on_status_chip(self, key: str):
        self._status_filter = key
        for k, chip in self._chip_buttons.items():
            chip.setChecked(k == key)
        # Reapply chip visual styles since Qt stylesheets don't support :checked dynamically
        c = ThemeManager.instance().get_colors()
        chip_checked = (
            f"QPushButton {{ background-color: {c['primary_light']}; color: {c['primary']}; "
            f"font-size: 11px; font-weight: 700; border: 1px solid {c['primary']}; border-radius: 13px; padding: 2px; }}"
        )
        chip_unchecked = (
            f"QPushButton {{ background-color: {c['bg_elevated']}; color: {c['text_secondary']}; "
            f"font-size: 11px; font-weight: 600; border: 1px solid {c['border']}; border-radius: 13px; padding: 2px; }}"
            f"QPushButton:hover {{ border-color: {c['border_focus']}; }}"
        )
        for chip in self._chip_buttons.values():
            chip.setStyleSheet(chip_checked if chip.isChecked() else chip_unchecked)
        self._filter_projects(self._search_input.text())

    def _matches_status_filter(self, state: ServerState) -> bool:
        if self._status_filter == "running":
            return state == ServerState.RUNNING
        if self._status_filter == "stopped":
            return state != ServerState.RUNNING
        return True

    def update_status(self, index: int, state: ServerState):
        self._states_cache[index] = state
        for row in range(self._list.count()):
            item = self._list.item(row)
            if item.data(Qt.ItemDataRole.UserRole) == index:
                item.setIcon(get_icon(self._get_status_icon(state)))
                break

        if self._status_filter != "all":
            self._filter_projects(self._search_input.text())

    def _get_status_icon(self, state: ServerState) -> str:
        status_map = {
            ServerState.STOPPED: "status-stopped",
            ServerState.STARTING: "status-starting",
            ServerState.RUNNING: "status-running",
            ServerState.STOPPING: "status-starting",
            ServerState.ERROR: "status-error",
        }
        return status_map.get(state, "status-stopped")

    def get_selected_index(self) -> int:
        item = self._list.currentItem()
        if item:
            return item.data(Qt.ItemDataRole.UserRole)
        return -1

    def focus_search(self):
        self._search_input.setFocus()
        self._search_input.selectAll()

    def _on_selection_changed(self, row: int):
        has_selection = row >= 0 and self._list.count() > 0
        self._edit_btn.setEnabled(has_selection)
        self._remove_btn.setEnabled(has_selection)
        idx = self.get_selected_index()
        self.project_selected.emit(idx)

    def _on_item_double_clicked(self, item: QListWidgetItem):
        idx = item.data(Qt.ItemDataRole.UserRole)
        if idx >= 0:
            self.edit_requested.emit(idx)

    def _on_edit(self):
        index = self.get_selected_index()
        if index >= 0:
            self.edit_requested.emit(index)

    def _on_remove(self):
        index = self.get_selected_index()
        if index >= 0:
            self.remove_requested.emit(index)

    def _show_context_menu(self, pos: QPoint):
        item = self._list.itemAt(pos)
        if not item:
            return
        
        idx = item.data(Qt.ItemDataRole.UserRole)
        if idx < 0 or idx >= len(self._projects_cache):
            return

        project = self._projects_cache[idx]
        state = self._states_cache.get(idx, ServerState.STOPPED)

        menu = QMenu(self)

        # Pin / Unpin
        pin_title = "Unpin from Top" if project.pinned else "Pin to Top"
        pin_icon = "pin-filled" if project.pinned else "pin"
        pin_act = QAction(get_icon(pin_icon, "#f59e0b" if project.pinned else "#94a3b8"), pin_title, self)
        pin_act.triggered.connect(lambda: self.toggle_pin_requested.emit(idx))
        menu.addAction(pin_act)

        menu.addSeparator()

        # Server actions
        start_act = QAction(get_icon("play-filled", "#10b981"), "Start Server", self)
        start_act.setEnabled(state != ServerState.RUNNING)
        start_act.triggered.connect(lambda: self.start_requested.emit(idx))
        menu.addAction(start_act)

        stop_act = QAction(get_icon("stop", "#ef4444"), "Stop Server", self)
        stop_act.setEnabled(state == ServerState.RUNNING or state == ServerState.STARTING)
        stop_act.triggered.connect(lambda: self.stop_requested.emit(idx))
        menu.addAction(stop_act)

        restart_act = QAction(get_icon("restart", "#3b82f6"), "Restart Server", self)
        restart_act.triggered.connect(lambda: self.restart_requested.emit(idx))
        menu.addAction(restart_act)

        menu.addSeparator()

        # Tools & URLs
        if project.server.url:
            url_act = QAction(get_icon("external-link"), "Open in Browser", self)
            url_act.triggered.connect(lambda: self.open_url_requested.emit(idx))
            menu.addAction(url_act)

        folder_act = QAction(get_icon("folder"), "Open in File Explorer", self)
        folder_act.triggered.connect(lambda: self.open_folder_requested.emit(idx))
        menu.addAction(folder_act)

        terminal_act = QAction(get_icon("terminal"), "Open in Terminal", self)
        terminal_act.triggered.connect(lambda: self.open_terminal_requested.emit(idx))
        menu.addAction(terminal_act)

        vscode_act = QAction(get_icon("sidebar-code"), "Open in VS Code", self)
        vscode_act.triggered.connect(lambda: self.open_vscode_requested.emit(idx))
        menu.addAction(vscode_act)

        opencode_act = QAction(get_icon("terminal-circle"), "Open in OpenCode", self)
        opencode_act.triggered.connect(lambda: self.open_opencode_requested.emit(idx))
        menu.addAction(opencode_act)

        if project.playwright.enabled:
            test_act = QAction(get_icon("flask"), "Run Playwright Tests", self)
            test_act.triggered.connect(lambda: self.run_tests_requested.emit(idx))
            menu.addAction(test_act)

        menu.addSeparator()

        edit_act = QAction(get_icon("edit"), "Edit Project...", self)
        edit_act.triggered.connect(lambda: self.edit_requested.emit(idx))
        menu.addAction(edit_act)

        remove_act = QAction(get_icon("trash"), "Remove Project", self)
        remove_act.triggered.connect(lambda: self.remove_requested.emit(idx))
        menu.addAction(remove_act)

        menu.exec(self._list.mapToGlobal(pos))
