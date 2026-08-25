# -*- coding: utf-8 -*-
from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QPushButton,
    QLabel, QGroupBox
)
from PySide6.QtCore import Signal, Qt, QSize
from PySide6.QtGui import QFont
from app.models.project import Project, PlaywrightState
from app.ui.icons import get_icon
from app.ui.theme import ThemeManager, ThemeMode, ui_font


class PlaywrightPanel(QWidget):
    run_tests_requested = Signal()
    run_ui_requested = Signal()
    run_debug_requested = Signal()
    show_report_requested = Signal()
    stop_requested = Signal()

    def __init__(self, parent=None):
        super().__init__(parent)
        self._project = None
        self._setup_ui()
        self.apply_theme(ThemeManager.instance().mode)
        self.set_controls_enabled(False)

    def _setup_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(14, 14, 14, 14)
        layout.setSpacing(14)

        # Status Group
        self._status_group = QGroupBox("Test Status")
        status_layout = QVBoxLayout(self._status_group)
        status_layout.setContentsMargins(14, 16, 14, 14)

        self._status_label = QLabel("Status: Idle")
        self._status_label.setFont(ui_font(14, QFont.Weight.DemiBold))
        status_layout.addWidget(self._status_label)

        layout.addWidget(self._status_group)

        # Test actions
        self._actions_group = QGroupBox("Test Runners")
        actions_layout = QVBoxLayout(self._actions_group)
        actions_layout.setSpacing(10)
        actions_layout.setContentsMargins(14, 16, 14, 14)

        row1 = QHBoxLayout()
        row1.setSpacing(10)

        self._run_btn = QPushButton(" Run Tests")
        self._run_btn.setIcon(get_icon("flask", "#ffffff"))
        self._run_btn.setIconSize(QSize(16, 16))

        self._ui_btn = QPushButton(" UI Mode")
        self._ui_btn.setIcon(get_icon("monitor", "#ffffff"))
        self._ui_btn.setIconSize(QSize(16, 16))

        self._debug_btn = QPushButton(" Debug Mode")
        self._debug_btn.setIcon(get_icon("bug", "#ffffff"))
        self._debug_btn.setIconSize(QSize(16, 16))

        for btn in (self._run_btn, self._ui_btn, self._debug_btn):
            btn.setMinimumHeight(44)
            row1.addWidget(btn)

        actions_layout.addLayout(row1)

        row2 = QHBoxLayout()
        row2.setSpacing(10)

        self._report_btn = QPushButton(" View Test Report")
        self._report_btn.setIcon(get_icon("report", "#6366f1"))
        self._report_btn.setIconSize(QSize(16, 16))

        self._stop_btn = QPushButton(" Stop Execution")
        self._stop_btn.setIcon(get_icon("stop", "#ffffff"))
        self._stop_btn.setIconSize(QSize(16, 16))

        self._report_btn.setMinimumHeight(42)
        self._stop_btn.setMinimumHeight(42)

        row2.addWidget(self._report_btn, stretch=2)
        row2.addWidget(self._stop_btn, stretch=1)

        actions_layout.addLayout(row2)
        layout.addWidget(self._actions_group)

        # Connect signals
        self._run_btn.clicked.connect(self.run_tests_requested.emit)
        self._ui_btn.clicked.connect(self.run_ui_requested.emit)
        self._debug_btn.clicked.connect(self.run_debug_requested.emit)
        self._report_btn.clicked.connect(self.show_report_requested.emit)
        self._stop_btn.clicked.connect(self.stop_requested.emit)

        layout.addStretch()

    def apply_theme(self, mode: ThemeMode):
        c = ThemeManager.instance().get_colors(mode)

        self._run_btn.setStyleSheet(
            f"QPushButton {{ background-color: {c['success']}; color: white; font-weight: 700; "
            f"border: none; border-radius: 8px; padding: 8px 16px; }} "
            f"QPushButton:hover {{ background-color: {c['success']}cc; }} "
            f"QPushButton:disabled {{ background-color: {c['border']}; color: {c['text_muted']}; }}"
        )
        self._ui_btn.setStyleSheet(
            f"QPushButton {{ background-color: {c['primary']}; color: white; font-weight: 700; "
            f"border: none; border-radius: 8px; padding: 8px 16px; }} "
            f"QPushButton:hover {{ background-color: {c['primary_hover']}; }} "
            f"QPushButton:disabled {{ background-color: {c['border']}; color: {c['text_muted']}; }}"
        )
        self._debug_btn.setStyleSheet(
            f"QPushButton {{ background-color: {c['warning']}; color: white; font-weight: 700; "
            f"border: none; border-radius: 8px; padding: 8px 16px; }} "
            f"QPushButton:hover {{ background-color: {c['warning']}cc; }} "
            f"QPushButton:disabled {{ background-color: {c['border']}; color: {c['text_muted']}; }}"
        )
        self._stop_btn.setStyleSheet(
            f"QPushButton {{ background-color: {c['danger']}; color: white; font-weight: 700; "
            f"border: none; border-radius: 8px; padding: 8px 16px; }} "
            f"QPushButton:hover {{ background-color: {c['danger']}cc; }} "
            f"QPushButton:disabled {{ background-color: {c['border']}; color: {c['text_muted']}; }}"
        )
        self._report_btn.setIcon(get_icon("report", c["primary"]))

    def set_project(self, project: Project):
        self._project = project
        self.set_controls_enabled(project.playwright.enabled)

    def update_state(self, state: PlaywrightState):
        c = ThemeManager.instance().get_colors()
        state_config = {
            PlaywrightState.IDLE: ("Idle", c["text_muted"]),
            PlaywrightState.STARTING: ("Starting...", c["warning"]),
            PlaywrightState.RUNNING: ("Running...", c["primary"]),
            PlaywrightState.PASSED: ("PASSED ✓", c["success"]),
            PlaywrightState.FAILED: ("FAILED ✗", c["danger"]),
            PlaywrightState.ERROR: ("Error", c["danger"]),
        }
        text, color = state_config.get(state, ("Unknown", c["text_muted"]))
        self._status_label.setText(f"Status: <span style='color: {color};'>{text}</span>")

        is_running = state in (PlaywrightState.STARTING, PlaywrightState.RUNNING)
        self._run_btn.setEnabled(not is_running)
        self._ui_btn.setEnabled(not is_running)
        self._debug_btn.setEnabled(not is_running)
        self._stop_btn.setEnabled(is_running)
        self._report_btn.setEnabled(True)

    def set_controls_enabled(self, enabled: bool):
        self._run_btn.setEnabled(enabled)
        self._ui_btn.setEnabled(enabled)
        self._debug_btn.setEnabled(enabled)
        self._report_btn.setEnabled(enabled)
        self._stop_btn.setEnabled(False)
        if not enabled:
            c = ThemeManager.instance().get_colors()
            self._status_label.setText(f"Status: <span style='color: {c['text_muted']};'>Playwright disabled</span>")
