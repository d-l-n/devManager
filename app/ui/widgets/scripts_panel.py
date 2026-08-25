# -*- coding: utf-8 -*-
from typing import Dict, Optional
from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QPushButton,
    QLabel, QGroupBox, QLineEdit, QScrollArea, QFrame,
    QSizePolicy
)
from PySide6.QtCore import Signal, Qt, QSize
from PySide6.QtGui import QFont
from app.models.project import Project
from app.ui.icons import get_icon
from app.ui.theme import ThemeManager, ThemeMode, ui_font, mono_font
from app.utils.detection import get_project_scripts


class ScriptRow(QFrame):
    run_requested = Signal(str, str)  # (name, command)

    def __init__(self, name: str, command: str, parent=None):
        super().__init__(parent)
        self._name = name
        self._command = command
        self._is_running = False
        self._setup_ui()
        self.apply_theme(ThemeManager.instance().mode)

    def _setup_ui(self):
        self.setObjectName("scriptRow")
        layout = QHBoxLayout(self)
        layout.setContentsMargins(12, 8, 12, 8)
        layout.setSpacing(10)

        # Name label
        self._name_label = QLabel(self._name)
        self._name_label.setFont(ui_font(13, QFont.Weight.DemiBold))
        self._name_label.setFixedWidth(140)
        layout.addWidget(self._name_label)

        # Command label
        self._cmd_label = QLabel(self._command)
        self._cmd_label.setFont(mono_font(11))
        self._cmd_label.setSizePolicy(QSizePolicy.Policy.Expanding, QSizePolicy.Policy.Preferred)
        layout.addWidget(self._cmd_label, stretch=1)

        # Status badge
        self._status_badge = QLabel("Idle")
        self._status_badge.setAlignment(Qt.AlignmentFlag.AlignCenter)
        self._status_badge.setFixedWidth(80)
        self._status_badge.setFixedHeight(24)
        layout.addWidget(self._status_badge)

        # Run button
        self._run_btn = QPushButton(" Run")
        self._run_btn.setIcon(get_icon("play-filled", "#ffffff"))
        self._run_btn.setIconSize(QSize(13, 13))
        self._run_btn.setFixedSize(74, 30)
        self._run_btn.clicked.connect(lambda: self.run_requested.emit(self._name, self._command))
        layout.addWidget(self._run_btn)

    def apply_theme(self, mode: ThemeMode):
        c = ThemeManager.instance().get_colors(mode)
        self.setStyleSheet(
            f"QFrame#scriptRow {{ background-color: {c['bg_surface']}; border: 1px solid {c['border']}; border-radius: 8px; }}"
        )
        self._name_label.setStyleSheet(f"color: {c['text_primary']}; font-weight: 600;")
        self._cmd_label.setStyleSheet(f"color: {c['text_muted']};")
        self._run_btn.setStyleSheet(
            f"QPushButton {{ background-color: {c['primary']}; color: #ffffff; border: none; border-radius: 6px; font-weight: 600; font-size: 12px; }} "
            f"QPushButton:hover {{ background-color: {c['primary_hover']}; }} "
            f"QPushButton:disabled {{ background-color: {c['bg_elevated']}; color: {c['text_muted']}; }}"
        )
        self._update_badge_style(c)

    def set_running(self, running: bool):
        self._is_running = running
        c = ThemeManager.instance().get_colors()
        self._update_badge_style(c)

    def _update_badge_style(self, c: dict):
        if self._is_running:
            self._status_badge.setText("Running...")
            self._status_badge.setStyleSheet(
                f"background-color: {c['warning_bg']}; color: {c['warning']}; border-radius: 6px; font-size: 11px; font-weight: 600;"
            )
            self._run_btn.setEnabled(False)
        else:
            self._status_badge.setText("Idle")
            self._status_badge.setStyleSheet(
                f"background-color: {c['bg_elevated']}; color: {c['text_muted']}; border-radius: 6px; font-size: 11px; font-weight: 500;"
            )
            self._run_btn.setEnabled(True)


class ScriptsPanel(QWidget):
    run_script_requested = Signal(str, str)  # (script_name, command)
    stop_script_requested = Signal()

    def __init__(self, parent=None):
        super().__init__(parent)
        self._project: Optional[Project] = None
        self._scripts: Dict[str, str] = {}
        self._rows: Dict[str, ScriptRow] = {}
        self._active_script: Optional[str] = None
        self._setup_ui()
        self.apply_theme(ThemeManager.instance().mode)
        self.set_controls_enabled(False)

    def _setup_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(14, 14, 14, 14)
        layout.setSpacing(12)

        # Top Action & Status Bar
        top_group = QGroupBox("Active Script Execution")
        top_layout = QHBoxLayout(top_group)
        top_layout.setContentsMargins(14, 14, 14, 14)
        top_layout.setSpacing(10)

        self._active_label = QLabel("No script running")
        self._active_label.setFont(ui_font(13, QFont.Weight.DemiBold))
        top_layout.addWidget(self._active_label, stretch=1)

        self._stop_btn = QPushButton(" Stop Script")
        self._stop_btn.setIcon(get_icon("stop", "#ffffff"))
        self._stop_btn.setIconSize(QSize(14, 14))
        self._stop_btn.setFixedHeight(34)
        self._stop_btn.setEnabled(False)
        self._stop_btn.clicked.connect(self.stop_script_requested.emit)
        top_layout.addWidget(self._stop_btn)

        layout.addWidget(top_group)

        # Custom Command Runner Box
        custom_group = QGroupBox("Run Custom Command")
        custom_layout = QHBoxLayout(custom_group)
        custom_layout.setContentsMargins(14, 14, 14, 14)
        custom_layout.setSpacing(8)

        self._custom_input = QLineEdit()
        self._custom_input.setPlaceholderText("e.g. npx prisma studio / pip install -r requirements.txt")
        self._custom_input.setFont(mono_font(12))
        self._custom_input.returnPressed.connect(self._on_run_custom)
        custom_layout.addWidget(self._custom_input, stretch=1)

        self._custom_run_btn = QPushButton(" Run Custom")
        self._custom_run_btn.setIcon(get_icon("play-filled", "#ffffff"))
        self._custom_run_btn.setIconSize(QSize(14, 14))
        self._custom_run_btn.setFixedHeight(34)
        self._custom_run_btn.clicked.connect(self._on_run_custom)
        custom_layout.addWidget(self._custom_run_btn)

        layout.addWidget(custom_group)

        # Scripts List Group
        self._scripts_group = QGroupBox("Project Scripts (package.json / config)")
        scripts_vbox = QVBoxLayout(self._scripts_group)
        scripts_vbox.setContentsMargins(14, 14, 14, 14)
        scripts_vbox.setSpacing(8)

        # Search Bar
        self._search_input = QLineEdit()
        self._search_input.setPlaceholderText("Filter scripts...")
        self._search_input.setClearButtonEnabled(True)
        self._search_input.addAction(get_icon("search", "#64748b"), QLineEdit.ActionPosition.LeadingPosition)
        self._search_input.textChanged.connect(self._filter_rows)
        scripts_vbox.addWidget(self._search_input)

        # Scroll area for script rows
        self._scroll = QScrollArea()
        self._scroll.setWidgetResizable(True)
        self._scroll.setFrameShape(QFrame.Shape.NoFrame)

        self._container = QWidget()
        self._container_layout = QVBoxLayout(self._container)
        self._container_layout.setContentsMargins(0, 0, 0, 0)
        self._container_layout.setSpacing(6)
        self._container_layout.addStretch()

        self._scroll.setWidget(self._container)
        scripts_vbox.addWidget(self._scroll, stretch=1)

        self._empty_label = QLabel("No scripts found in package.json")
        self._empty_label.setAlignment(Qt.AlignmentFlag.AlignCenter)
        self._empty_label.setFont(ui_font(13, QFont.Weight.Normal))
        self._empty_label.hide()
        scripts_vbox.addWidget(self._empty_label)

        layout.addWidget(self._scripts_group, stretch=1)

    def apply_theme(self, mode: ThemeMode):
        c = ThemeManager.instance().get_colors(mode)
        
        self._active_label.setStyleSheet(f"color: {c['text_primary']};")
        self._stop_btn.setStyleSheet(
            f"QPushButton {{ background-color: {c['danger']}; color: #ffffff; border: none; border-radius: 6px; font-weight: 600; padding: 6px 14px; }} "
            f"QPushButton:hover {{ background-color: {c['danger']}cc; }} "
            f"QPushButton:disabled {{ background-color: {c['bg_elevated']}; color: {c['text_muted']}; }}"
        )
        self._custom_input.setStyleSheet(
            f"QLineEdit {{ background-color: {c['bg_surface']}; color: {c['text_primary']}; border: 1px solid {c['border']}; "
            f"border-radius: 6px; padding: 6px 10px; font-size: 12px; }} "
            f"QLineEdit:focus {{ border: 1px solid {c['primary']}; }}"
        )
        self._custom_run_btn.setStyleSheet(
            f"QPushButton {{ background-color: {c['primary']}; color: #ffffff; border: none; border-radius: 6px; font-weight: 600; padding: 6px 14px; font-size: 12px; }} "
            f"QPushButton:hover {{ background-color: {c['primary_hover']}; }} "
            f"QPushButton:disabled {{ background-color: {c['bg_elevated']}; color: {c['text_muted']}; }}"
        )
        self._search_input.setStyleSheet(
            f"QLineEdit {{ background-color: {c['bg_surface']}; color: {c['text_primary']}; border: 1px solid {c['border']}; "
            f"border-radius: 6px; padding: 5px 10px; font-size: 12px; }} "
            f"QLineEdit:focus {{ border: 1px solid {c['primary']}; }}"
        )
        self._empty_label.setStyleSheet(f"color: {c['text_muted']}; padding: 20px;")

        for row in self._rows.values():
            row.apply_theme(mode)

    def set_project(self, project: Optional[Project]):
        self._project = project
        self._reload_scripts()

    def set_controls_enabled(self, enabled: bool):
        self._custom_input.setEnabled(enabled)
        self._custom_run_btn.setEnabled(enabled)
        self._search_input.setEnabled(enabled)
        for row in self._rows.values():
            row.setEnabled(enabled)

    def set_script_running(self, script_name: Optional[str], is_running: bool):
        self._active_script = script_name if is_running else None
        c = ThemeManager.instance().get_colors()

        if is_running and script_name:
            self._active_label.setText(f"Running script:  ⚡ {script_name}")
            self._active_label.setStyleSheet(f"color: {c['warning']}; font-weight: bold;")
            self._stop_btn.setEnabled(True)
        else:
            self._active_label.setText("No script running")
            self._active_label.setStyleSheet(f"color: {c['text_muted']};")
            self._stop_btn.setEnabled(False)

        for name, row in self._rows.items():
            row.set_running(is_running and name == script_name)

    def _reload_scripts(self):
        # Clear existing rows
        for row in self._rows.values():
            row.deleteLater()
        self._rows.clear()

        if not self._project:
            self._empty_label.setText("Select a project to view scripts")
            self._empty_label.show()
            self._scripts = {}
            return

        self._scripts = get_project_scripts(self._project.path)

        if not self._scripts:
            self._empty_label.setText(f"No scripts declared in '{self._project.name}'")
            self._empty_label.show()
            return

        self._empty_label.hide()
        for name, cmd in self._scripts.items():
            row = ScriptRow(name, cmd)
            row.run_requested.connect(self.run_script_requested.emit)
            self._container_layout.insertWidget(self._container_layout.count() - 1, row)
            self._rows[name] = row

        self._filter_rows(self._search_input.text())

    def _filter_rows(self, query: str = ""):
        query = query.strip().lower()
        visible_count = 0
        for name, row in self._rows.items():
            cmd = self._scripts.get(name, "")
            matches = not query or query in name.lower() or query in cmd.lower()
            row.setVisible(matches)
            if matches:
                visible_count += 1

        if visible_count == 0 and self._scripts:
            self._empty_label.setText(f"No scripts match '{query}'")
            self._empty_label.show()
        elif self._scripts:
            self._empty_label.hide()

    def _on_run_custom(self):
        cmd = self._custom_input.text().strip()
        if cmd:
            self.run_script_requested.emit("custom", cmd)
