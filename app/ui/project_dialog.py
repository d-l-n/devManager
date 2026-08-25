# -*- coding: utf-8 -*-
import os
from typing import Optional
from PySide6.QtWidgets import (
    QDialog, QVBoxLayout, QHBoxLayout, QFormLayout,
    QLineEdit, QSpinBox, QCheckBox, QPushButton,
    QFileDialog, QMessageBox, QTabWidget, QWidget,
    QGroupBox, QLabel, QAbstractSpinBox
)
from PySide6.QtCore import Qt
from app.models.project import Project, ServerConfig, PlaywrightConfig
from app.ui.icons import get_icon
from app.utils.detection import detect_project_config
from app.ui.theme import ThemeManager


class ProjectDialog(QDialog):

    def __init__(self, project: Optional[Project] = None, existing_ports: Optional[list] = None, parent=None):
        super().__init__(parent)
        self._is_edit = project is not None
        self._existing_ports = existing_ports or []
        self._original_port = project.server.port if project and project.server else 0
        self._original_pinned = project.pinned if project else False
        self.setWindowTitle("Edit Project" if self._is_edit else "Add Project")
        self.setWindowIcon(get_icon("edit" if self._is_edit else "plus"))
        self.setMinimumWidth(540)
        self.setStyleSheet(self._get_stylesheet())
        self._setup_ui()

        if project:
            self._populate(project)

    def _setup_ui(self):
        layout = QVBoxLayout(self)
        layout.setSpacing(14)
        layout.setContentsMargins(16, 16, 16, 16)

        tabs = QTabWidget()

        # --- General Tab ---
        general_tab = QWidget()
        general_layout = QFormLayout(general_tab)
        general_layout.setSpacing(10)
        general_layout.setContentsMargins(12, 12, 12, 12)

        self._name_edit = QLineEdit()
        self._name_edit.setPlaceholderText("e.g. MPoints Tracker")
        general_layout.addRow("Name:", self._name_edit)

        path_layout = QHBoxLayout()
        self._path_edit = QLineEdit()
        self._path_edit.setPlaceholderText("e.g. D:/Projects/my-project")
        browse_btn = QPushButton(" Browse...")
        browse_btn.setIcon(get_icon("folder", "#c0caf5"))
        browse_btn.setFixedWidth(100)
        browse_btn.clicked.connect(self._browse_path)

        detect_btn = QPushButton(" Detectar Automáticamente")
        detect_btn.setIcon(get_icon("search", "#e0af68"))
        detect_btn.setFixedWidth(120)
        detect_btn.setToolTip("Auto-detect project settings from folder contents")
        detect_btn.clicked.connect(self._auto_detect)

        path_layout.addWidget(self._path_edit)
        path_layout.addWidget(browse_btn)
        path_layout.addWidget(detect_btn)
        general_layout.addRow("Path:", path_layout)

        tabs.addTab(general_tab, get_icon("folder"), "General")

        # --- Server Tab ---
        server_tab = QWidget()
        server_layout = QFormLayout(server_tab)
        server_layout.setSpacing(10)
        server_layout.setContentsMargins(12, 12, 12, 12)

        self._server_enabled = QCheckBox("Enable server management")
        self._server_enabled.setChecked(True)
        server_layout.addRow("", self._server_enabled)

        self._server_command = QLineEdit("npm run dev")
        server_layout.addRow("Command:", self._server_command)

        self._server_port = QSpinBox()
        self._server_port.setRange(0, 65535)
        self._server_port.setSingleStep(1)
        self._server_port.setValue(5173)
        self._server_port.setButtonSymbols(QAbstractSpinBox.ButtonSymbols.UpDownArrows)
        server_layout.addRow("Port:", self._server_port)

        self._server_url = QLineEdit("http://localhost:5173")
        server_layout.addRow("URL:", self._server_url)

        timeout_layout = QHBoxLayout()
        self._server_timeout = QSpinBox()
        self._server_timeout.setRange(1000, 120000)
        self._server_timeout.setSingleStep(1000)
        self._server_timeout.setValue(15000)
        self._server_timeout.setButtonSymbols(QAbstractSpinBox.ButtonSymbols.UpDownArrows)
        timeout_layout.addWidget(self._server_timeout)
        timeout_layout.addWidget(QLabel("ms"))
        timeout_layout.addStretch()
        server_layout.addRow("Startup Timeout:", timeout_layout)

        tabs.addTab(server_tab, get_icon("server"), "Server")

        # --- Playwright Tab ---
        pw_tab = QWidget()
        pw_layout = QFormLayout(pw_tab)
        pw_layout.setSpacing(10)
        pw_layout.setContentsMargins(12, 12, 12, 12)

        self._pw_enabled = QCheckBox("Enable Playwright integration")
        self._pw_enabled.setChecked(True)
        pw_layout.addRow("", self._pw_enabled)

        self._pw_command = QLineEdit("npx playwright test")
        pw_layout.addRow("Test Command:", self._pw_command)

        self._pw_ui_command = QLineEdit("npx playwright test --ui")
        pw_layout.addRow("UI Command:", self._pw_ui_command)

        self._pw_debug_command = QLineEdit("npx playwright test --debug")
        pw_layout.addRow("Debug Command:", self._pw_debug_command)

        self._pw_report_command = QLineEdit("npx playwright show-report")
        pw_layout.addRow("Report Command:", self._pw_report_command)

        tabs.addTab(pw_tab, get_icon("flask"), "Playwright")

        layout.addWidget(tabs)

        # --- Auto-detect status label ---
        self._status_label = QLabel("")
        self._status_label.setStyleSheet(self._status_idle_style())
        layout.addWidget(self._status_label)

        # --- Buttons ---
        button_layout = QHBoxLayout()
        button_layout.addStretch()

        ok_btn = QPushButton("OK")
        ok_btn.setFixedWidth(90)
        ok_btn.setDefault(True)
        ok_btn.clicked.connect(self.accept)

        cancel_btn = QPushButton("Cancel")
        cancel_btn.setFixedWidth(90)
        cancel_btn.clicked.connect(self.reject)

        button_layout.addWidget(ok_btn)
        button_layout.addWidget(cancel_btn)

        layout.addLayout(button_layout)

    def _populate(self, project: Project):
        self._name_edit.setText(project.name)
        self._path_edit.setText(project.path)

        self._server_enabled.setChecked(project.server.enabled)
        self._server_command.setText(project.server.command)
        self._server_port.setValue(project.server.port)
        self._server_url.setText(project.server.url)
        self._server_timeout.setValue(project.server.startup_timeout)

        self._pw_enabled.setChecked(project.playwright.enabled)
        self._pw_command.setText(project.playwright.command)
        self._pw_ui_command.setText(project.playwright.ui_command)
        self._pw_debug_command.setText(project.playwright.debug_command)
        self._pw_report_command.setText(project.playwright.report_command)

    def get_project(self) -> Project:
        server = ServerConfig(
            enabled=self._server_enabled.isChecked(),
            command=self._server_command.text().strip(),
            port=self._server_port.value(),
            url=self._server_url.text().strip(),
            startup_timeout=self._server_timeout.value()
        )
        playwright = PlaywrightConfig(
            enabled=self._pw_enabled.isChecked(),
            command=self._pw_command.text().strip(),
            ui_command=self._pw_ui_command.text().strip(),
            debug_command=self._pw_debug_command.text().strip(),
            report_command=self._pw_report_command.text().strip()
        )
        return Project(
            name=self._name_edit.text().strip(),
            path=self._path_edit.text().strip(),
            server=server,
            playwright=playwright,
            pinned=self._original_pinned
        )

    def accept(self):
        if self._validate():
            super().accept()

    def _validate(self) -> bool:
        project = self.get_project()
        errors = project.validate()

        if errors:
            QMessageBox.warning(self, "Error de Validación", "\n".join(errors))
            return False
        return True

    def _browse_path(self):
        path = QFileDialog.getExistingDirectory(self, "Select Project Directory")
        if path:
            self._path_edit.setText(path)
            # Auto-detect only when adding a new project (not editing)
            if not self._is_edit:
                self._auto_detect()

    def _auto_detect(self):
        """Inspect the project folder and auto-fill form fields."""
        path = self._path_edit.text().strip()
        if not path or not os.path.isdir(path):
            QMessageBox.information(
                self, "Detectar Automáticamente",
                "Seleccionar una carpeta de proyecto válida primero"
            )
            return

        ports_to_avoid = list(self._existing_ports)
        if self._is_edit and self._original_port in ports_to_avoid:
            ports_to_avoid.remove(self._original_port)

        detected = detect_project_config(path, existing_ports=ports_to_avoid)

        # Fill name only if currently empty
        if not self._name_edit.text().strip():
            self._name_edit.setText(detected["name"])

        self._server_command.setText(detected["server_command"])
        self._server_port.setValue(detected["port"])
        self._server_url.setText(detected["url"])

        if detected["playwright_enabled"]:
            self._pw_enabled.setChecked(True)

        self._status_label.setText(
            f"✓ Detected: port {detected['port']}, "
            f"command \"{detected['server_command']}\""
            + (", Playwright found" if detected["playwright_enabled"] else "")
        )
        self._status_label.setStyleSheet(self._status_success_style())

    def _get_stylesheet(self) -> str:
        return ThemeManager.instance().get_dialog_stylesheet()

    def _status_idle_style(self) -> str:
        c = ThemeManager.instance().get_colors()
        return f"color: {c['text_muted']}; font-size: 12px; padding: 4px 0;"

    def _status_success_style(self) -> str:
        c = ThemeManager.instance().get_colors()
        return f"color: {c['success']}; font-size: 12px; padding: 4px 0;"
