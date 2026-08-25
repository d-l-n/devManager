# -*- coding: utf-8 -*-
# app/ui/widgets/git_panel.py
from typing import Optional
from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QGroupBox, QPushButton, QFrame
)
from PySide6.QtCore import Signal, QProcess, QSize, Qt
from PySide6.QtGui import QFont
from app.models.project import Project
from app.utils.git import get_git_status_full, is_git_repo
from app.ui.icons import get_icon
from app.ui.theme import ThemeManager, ThemeMode, ui_font, mono_font


class GitPanel(QWidget):
    output = Signal(str, bool)             # (text, is_error) → ruteado a Logs por MainWindow
    command_finished = Signal(str, int, str)    # (cmd_name, exit_code, project_name)

    GIT_COMMANDS = {
        "Pull": ["pull", "--ff-only"],
        "Fetch": ["fetch", "--all", "--prune"],
        "Stash": ["stash"],
    }

    def __init__(self, parent=None):
        super().__init__(parent)
        self._project: Optional[Project] = None
        self._info: dict = {}
        self._process: Optional[QProcess] = None
        self._running_cmd: Optional[str] = None
        self._setup_ui()
        self.apply_theme(ThemeManager.instance().mode)

    def _setup_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(14, 14, 14, 14)
        layout.setSpacing(14)

        # --- Info card ---
        self._info_group = QGroupBox("Repository Status")
        info_layout = QVBoxLayout(self._info_group)
        info_layout.setContentsMargins(14, 16, 14, 14)
        info_layout.setSpacing(8)

        row1 = QHBoxLayout()
        self._branch_label = QLabel("Branch: —")
        self._branch_label.setFont(ui_font(14, QFont.Weight.DemiBold))
        row1.addWidget(self._branch_label)
        row1.addStretch()
        self._sync_label = QLabel("")  # ↑ahead ↓behind
        row1.addWidget(self._sync_label)
        self._dirty_badge = QLabel("")
        row1.addWidget(self._dirty_badge)
        info_layout.addLayout(row1)

        self._commit_label = QLabel("Last commit: —")
        self._commit_label.setFont(mono_font(11))
        self._commit_label.setWordWrap(True)
        info_layout.addWidget(self._commit_label)

        layout.addWidget(self._info_group)

        # --- Actions ---
        self._actions_group = QGroupBox("Actions")
        actions_layout = QHBoxLayout(self._actions_group)
        actions_layout.setContentsMargins(14, 16, 14, 14)
        actions_layout.setSpacing(10)

        self._refresh_btn = QPushButton(" Refresh")
        self._pull_btn = QPushButton(" Pull")
        self._fetch_btn = QPushButton(" Fetch")
        self._stash_btn = QPushButton(" Stash")

        self._refresh_btn.setIcon(get_icon("refresh", "#ffffff"))
        self._pull_btn.setIcon(get_icon("git-pull", "#ffffff"))
        self._fetch_btn.setIcon(get_icon("external-link", "#ffffff"))
        self._stash_btn.setIcon(get_icon("layers", "#ffffff"))
        for b in (self._refresh_btn, self._pull_btn, self._fetch_btn, self._stash_btn):
            b.setIconSize(QSize(14, 14))
            b.setMinimumHeight(36)
            actions_layout.addWidget(b)

        self._refresh_btn.clicked.connect(self.refresh_info)
        self._pull_btn.clicked.connect(lambda: self._run_command("Pull"))
        self._fetch_btn.clicked.connect(lambda: self._run_command("Fetch"))
        self._stash_btn.clicked.connect(lambda: self._run_command("Stash"))

        layout.addWidget(self._actions_group)

        # --- Result strip ---
        self._result_frame = QFrame()
        result_layout = QHBoxLayout(self._result_frame)
        result_layout.setContentsMargins(12, 8, 12, 8)
        self._result_label = QLabel("")
        self._result_label.setFont(ui_font(12))
        self._result_label.setWordWrap(True)
        result_layout.addWidget(self._result_label)
        self._result_frame.hide()
        layout.addWidget(self._result_frame)

        layout.addStretch()

        # Empty state
        self._empty_label = QLabel("Not a git repository.\nOpen a project folder containing a .git directory.")
        self._empty_label.setAlignment(Qt.AlignmentFlag.AlignCenter)
        layout.addWidget(self._empty_label)

    # ---------- Public API ----------

    def set_project(self, project: Optional[Project]):
        self._project = project
        self.refresh_info()

    def refresh_info(self):
        if not self._project or not is_git_repo(self._project.path):
            self._show_empty_state(True)
            return
        self._show_empty_state(False)
        self._info = get_git_status_full(self._project.path)
        c = ThemeManager.instance().get_colors()

        branch = self._info.get("branch") or "unknown"
        dirty = self._info.get("is_dirty", False)
        self._branch_label.setText(f"Branch: {branch}")

        if dirty:
            self._dirty_badge.setText("● Uncommitted changes")
            self._dirty_badge.setStyleSheet(
                f"color: {c['warning']}; background-color: {c['warning_bg']}; "
                f"border-radius: 10px; padding: 3px 10px; font-size: 11px; font-weight: 600;"
            )
        else:
            self._dirty_badge.setText("● Clean")
            self._dirty_badge.setStyleSheet(
                f"color: {c['success']}; background-color: {c['success_bg']}; "
                f"border-radius: 10px; padding: 3px 10px; font-size: 11px; font-weight: 600;"
            )

        if self._info.get("has_upstream"):
            self._sync_label.setText(f"↑ {self._info['ahead']}   ↓ {self._info['behind']}")
            self._sync_label.setStyleSheet(f"color: {c['text_secondary']}; font-weight: 600;")
        else:
            self._sync_label.setText("(no upstream)")
            self._sync_label.setStyleSheet(f"color: {c['text_muted']}; font-size: 11px;")

        lc = self._info.get("last_commit")
        if lc:
            self._commit_label.setText(f"Last commit: {lc['hash']} · {lc['subject']} · {lc['date_rel']}")
        else:
            self._commit_label.setText("Last commit: —")

    # ---------- Command execution ----------

    def _run_command(self, name: str):
        if not self._project or self._process is not None:
            return
        args = self.GIT_COMMANDS[name]
        self._running_cmd = name
        self._set_buttons_enabled(False)
        self._show_result(f"Running: git {args[0]}…", ThemeManager.instance().get_colors()["text_secondary"])

        self._process = QProcess(self)
        self._process.setWorkingDirectory(self._project.path)
        self._process.readyReadStandardOutput.connect(self._on_stdout)
        self._process.readyReadStandardError.connect(self._on_stderr)
        self._process.finished.connect(self._on_finished)
        self._process.start("git", args)

    def _on_stdout(self):
        if self._process:
            text = bytes(self._process.readAllStandardOutput()).decode("utf-8", errors="replace").strip()
            if text:
                self.output.emit(text, False)

    def _on_stderr(self):
        if self._process:
            text = bytes(self._process.readAllStandardError()).decode("utf-8", errors="replace").strip()
            if text:
                self.output.emit(text, True)

    def _on_finished(self, exit_code, _status):
        name = self._running_cmd or "?"
        was_stash = name == "Stash"
        stdout_text = ""
        if self._process:
            stdout_text = bytes(self._process.readAllStandardOutput()).decode("utf-8", errors="replace")
        c = ThemeManager.instance().get_colors()

        if exit_code == 0:
            if was_stash and "No local changes" in stdout_text:
                self._show_result("Nothing to stash — working tree clean.", c["text_secondary"])
            else:
                self._show_result(f"{name} completed successfully.", c["success"])
        else:
            self._show_result(f"{name} failed (exit code {exit_code}). See Logs tab.", c["danger"])

        self.output.emit(f"[git {name.lower()}] exited with code {exit_code}", exit_code != 0)
        self.command_finished.emit(name, exit_code, self._project.name if self._project else "?")

        self._process = None
        self._running_cmd = None
        self._set_buttons_enabled(True)
        self.refresh_info()

    def _show_result(self, text: str, color: str):
        self._result_label.setText(text)
        self._result_label.setStyleSheet(f"color: {color}; border: none;")
        self._result_frame.show()

    def _set_buttons_enabled(self, enabled: bool):
        has_repo = bool(self._project and is_git_repo(self._project.path))
        for b in (self._refresh_btn, self._pull_btn, self._fetch_btn, self._stash_btn):
            b.setEnabled(enabled and has_repo)

    def _show_empty_state(self, empty: bool):
        self._info_group.setVisible(not empty)
        self._actions_group.setVisible(not empty)
        self._result_frame.setVisible(not empty and not self._result_label.text() == "")
        self._empty_label.setVisible(empty)

    # ---------- Theme ----------

    def apply_theme(self, mode: ThemeMode):
        c = ThemeManager.instance().get_colors(mode)
        group_style = (
            f"QGroupBox {{ color: {c['text_secondary']}; font-weight: bold; "
            f"border: 1px solid {c['border']}; border-radius: 8px; margin-top: 10px; }} "
            f"QGroupBox::title {{ subcontrol-origin: margin; left: 12px; padding: 0 4px; }}"
        )
        self._info_group.setStyleSheet(group_style)
        self._actions_group.setStyleSheet(group_style)

        git_btn_style = (
            f"QPushButton {{ background-color: {c['primary']}; color: white; font-weight: 600; "
            f"border: none; border-radius: 8px; padding: 8px 14px; }} "
            f"QPushButton:hover {{ background-color: {c['primary_hover']}; }} "
            f"QPushButton:disabled {{ background-color: {c['border']}; color: {c['text_muted']}; }}"
        )
        for b in (self._refresh_btn, self._pull_btn, self._fetch_btn, self._stash_btn):
            b.setStyleSheet(git_btn_style)

        self._branch_label.setStyleSheet(f"color: {c['text_primary']}; border: none;")
        self._commit_label.setStyleSheet(f"color: {c['text_secondary']}; border: none;")
        self._empty_label.setStyleSheet(f"color: {c['text_muted']}; font-size: 13px; border: none;")
        self._result_frame.setStyleSheet(
            f"QFrame {{ background-color: {c['bg_surface']}; border: 1px solid {c['border']}; border-radius: 8px; }}"
        )
        self.refresh_info() if self._project else None
