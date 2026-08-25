import os
from datetime import datetime
from typing import List, Tuple
from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QPlainTextEdit, QPushButton,
    QHBoxLayout, QLineEdit, QLabel, QApplication, QCheckBox,
    QFileDialog, QMessageBox
)
from PySide6.QtGui import QTextCharFormat, QColor, QTextCursor
from PySide6.QtCore import Qt, QSize, QTimer
from app.ui.icons import get_icon
from app.ui.theme import ThemeManager, ThemeMode, mono_font, FONT_FAMILY_MONO


class LogPanel(QWidget):
    MAX_LINES = 5000

    def __init__(self, parent=None):
        super().__init__(parent)
        self._raw_lines: List[Tuple[str, bool]] = []  # (formatted_line, is_error)
        self._auto_scroll = True
        self._setup_ui()
        self.apply_theme(ThemeManager.instance().mode)

    def _setup_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(14, 14, 14, 14)
        layout.setSpacing(8)

        # Top Control Bar (Search & Quick Actions)
        top_bar = QHBoxLayout()
        top_bar.setSpacing(6)

        self._filter_input = QLineEdit()
        self._filter_input.setPlaceholderText("Filter in logs... (Ctrl+F)")
        self._filter_input.setClearButtonEnabled(True)
        self._filter_input.addAction(get_icon("search", "#64748b"), QLineEdit.ActionPosition.LeadingPosition)
        self._filter_input.textChanged.connect(self._apply_filter)
        top_bar.addWidget(self._filter_input, stretch=1)

        self._wrap_cb = QCheckBox("Wrap Lines")
        self._wrap_cb.setChecked(False)
        self._wrap_cb.toggled.connect(self._toggle_wrap)
        top_bar.addWidget(self._wrap_cb)

        self._errors_only = False
        self._errors_only_cb = QCheckBox("Errors only")
        self._errors_only_cb.setChecked(False)
        self._errors_only_cb.toggled.connect(self._toggle_errors_only)
        top_bar.addWidget(self._errors_only_cb)

        self._autoscroll_btn = QPushButton("Auto-scroll: ON")
        self._autoscroll_btn.setCheckable(True)
        self._autoscroll_btn.setChecked(True)
        self._autoscroll_btn.setFixedHeight(30)
        self._autoscroll_btn.toggled.connect(self._toggle_autoscroll)
        top_bar.addWidget(self._autoscroll_btn)

        self._copy_btn = QPushButton(" Copy All")
        self._copy_btn.setIcon(get_icon("copy", "#94a3b8"))
        self._copy_btn.setIconSize(QSize(14, 14))
        self._copy_btn.setFixedHeight(30)
        self._copy_btn.setToolTip("Copy all console output to clipboard")
        self._copy_btn.clicked.connect(self._copy_all)
        top_bar.addWidget(self._copy_btn)

        self._save_btn = QPushButton(" Save")
        self._save_btn.setIcon(get_icon("save", "#94a3b8"))
        self._save_btn.setIconSize(QSize(14, 14))
        self._save_btn.setFixedHeight(30)
        self._save_btn.setToolTip("Export console logs to a text or log file")
        self._save_btn.clicked.connect(self._save_logs)
        top_bar.addWidget(self._save_btn)

        self._clear_btn = QPushButton(" Clear")
        self._clear_btn.setIcon(get_icon("trash", "#94a3b8"))
        self._clear_btn.setIconSize(QSize(14, 14))
        self._clear_btn.setFixedHeight(30)
        self._clear_btn.clicked.connect(self.clear)
        top_bar.addWidget(self._clear_btn)

        layout.addLayout(top_bar)

        # Terminal Editor
        self._text_edit = QPlainTextEdit()
        self._text_edit.setReadOnly(True)
        self._text_edit.setFont(mono_font())
        self._text_edit.setLineWrapMode(QPlainTextEdit.LineWrapMode.NoWrap)
        self._text_edit.setMaximumBlockCount(self.MAX_LINES)
        self._text_edit.verticalScrollBar().valueChanged.connect(self._on_scroll)

        layout.addWidget(self._text_edit, stretch=1)

    def apply_theme(self, mode: ThemeMode):
        c = ThemeManager.instance().get_colors(mode)
        res_dir = os.path.join(os.path.dirname(__file__), "..", "resources").replace("\\", "/")

        self._filter_input.setStyleSheet(
            f"QLineEdit {{ background-color: {c['bg_surface']}; color: {c['text_primary']}; border: 1px solid {c['border']}; "
            f"border-radius: 6px; padding: 5px 10px; font-size: 12px; }} "
            f"QLineEdit:focus {{ border: 1px solid {c['primary']}; }}"
        )
        self._wrap_cb.setStyleSheet(
            f"QCheckBox {{ color: {c['text_secondary']}; font-size: 12px; font-weight: 500; spacing: 8px; }} "
            f"QCheckBox::indicator {{ width: 16px; height: 16px; border: 1px solid {c['border']}; border-radius: 4px; background-color: {c['bg_elevated']}; }} "
            f"QCheckBox::indicator:hover {{ border: 1px solid {c['primary']}; }} "
            f"QCheckBox::indicator:checked {{ background-color: {c['primary']}; border: 1px solid {c['primary']}; image: url('{res_dir}/check.svg'); }}"
        )
        self._errors_only_cb.setStyleSheet(
            f"QCheckBox {{ color: {c['text_secondary']}; font-size: 12px; font-weight: 500; spacing: 8px; }} "
            f"QCheckBox::indicator {{ width: 16px; height: 16px; border: 1px solid {c['border']}; border-radius: 4px; background-color: {c['bg_elevated']}; }} "
            f"QCheckBox::indicator:hover {{ border: 1px solid {c['primary']}; }} "
            f"QCheckBox::indicator:checked {{ background-color: {c['primary']}; border: 1px solid {c['primary']}; image: url('{res_dir}/check.svg'); }}"
        )
        
        self._autoscroll_btn.setStyleSheet(
            f"QPushButton {{ background-color: {c['bg_elevated']}; color: {c['text_secondary']}; font-size: 12px; font-weight: 600; border-radius: 6px; padding: 4px 10px; border: 1px solid {c['border']}; }} "
            f"QPushButton:checked {{ background-color: {c['primary_light']}; color: {c['primary']}; border: 1px solid {c['primary']}; }}"
        )
        
        self._text_edit.setStyleSheet(
            f"QPlainTextEdit {{ background-color: {c['terminal_bg']}; color: {c['terminal_fg']}; "
            f"border: 1px solid {c['border']}; border-radius: 8px; padding: 8px; font-family: {FONT_FAMILY_MONO}; }}"
        )

        icon_color = c["icon_color"]
        self._copy_btn.setIcon(get_icon("copy", icon_color))
        self._save_btn.setIcon(get_icon("save", icon_color))
        self._clear_btn.setIcon(get_icon("trash", icon_color))

    def focus_search(self):
        """Focus and select the search/filter input."""
        self._filter_input.setFocus()
        self._filter_input.selectAll()

    def append_log(self, text: str):
        timestamp = datetime.now().strftime("[%H:%M:%S]")
        lines = text.splitlines() if text else [""]
        for i, line in enumerate(lines):
            formatted = f"{timestamp} {line}" if i == 0 else f"           {line}"
            self._raw_lines.append((formatted, False))
            if len(self._raw_lines) > self.MAX_LINES:
                self._raw_lines.pop(0)

            # Check filter
            query = self._filter_input.text().strip().lower()
            if (not query or query in formatted.lower()) and not self._errors_only:
                self._text_edit.appendPlainText(formatted)

        if self._auto_scroll:
            self._scroll_to_bottom()

    def append_error(self, text: str):
        timestamp = datetime.now().strftime("[%H:%M:%S]")
        lines = text.splitlines() if text else [""]

        c = ThemeManager.instance().get_colors()
        error_format = QTextCharFormat()
        error_format.setForeground(QColor(c["danger"]))

        for i, line in enumerate(lines):
            formatted = f"{timestamp} {line}" if i == 0 else f"           {line}"
            self._raw_lines.append((formatted, True))
            if len(self._raw_lines) > self.MAX_LINES:
                self._raw_lines.pop(0)

            query = self._filter_input.text().strip().lower()
            if not query or query in formatted.lower():
                cursor = self._text_edit.textCursor()
                cursor.movePosition(QTextCursor.MoveOperation.End)
                cursor.insertBlock()
                cursor.setCharFormat(error_format)
                cursor.insertText(formatted)
                self._text_edit.setTextCursor(cursor)

        if self._auto_scroll:
            self._scroll_to_bottom()

    def clear(self):
        self._raw_lines.clear()
        self._text_edit.clear()

    def _apply_filter(self, query: str):
        query = query.strip().lower()
        self._text_edit.clear()

        c = ThemeManager.instance().get_colors()
        error_format = QTextCharFormat()
        error_format.setForeground(QColor(c["danger"]))
        default_format = QTextCharFormat()
        default_format.setForeground(QColor(c["terminal_fg"]))

        cursor = self._text_edit.textCursor()
        for formatted, is_error in self._raw_lines:
            if (not query or query in formatted.lower()) and (not self._errors_only or is_error):
                cursor.movePosition(QTextCursor.MoveOperation.End)
                cursor.insertBlock()
                cursor.setCharFormat(error_format if is_error else default_format)
                cursor.insertText(formatted)

        self._text_edit.setTextCursor(cursor)
        if self._auto_scroll:
            self._scroll_to_bottom()

    def _copy_all(self):
        full_text = "\n".join(line for line, _ in self._raw_lines)
        if full_text:
            QApplication.clipboard().setText(full_text)
            self._copy_btn.setText(" Copied!")
            QTimer.singleShot(1800, lambda: self._copy_btn.setText(" Copy All"))

    def _save_logs(self):
        full_text = "\n".join(line for line, _ in self._raw_lines)
        if not full_text:
            QMessageBox.information(self, "No Logs", "There are no logs to export.")
            return

        file_path, _ = QFileDialog.getSaveFileName(
            self,
            "Save Logs As",
            f"devmanager_logs_{datetime.now().strftime('%Y%m%d_%H%M%S')}.log",
            "Log Files (*.log);;Text Files (*.txt);;All Files (*.*)"
        )
        if file_path:
            try:
                with open(file_path, "w", encoding="utf-8") as f:
                    f.write(full_text + "\n")
                self._save_btn.setText(" Saved!")
                QTimer.singleShot(1800, lambda: self._save_btn.setText(" Save"))
            except Exception as e:
                QMessageBox.critical(self, "Error Saving Logs", f"Failed to save log file:\n{str(e)}")

    def _toggle_wrap(self, checked: bool):
        if checked:
            self._text_edit.setLineWrapMode(QPlainTextEdit.LineWrapMode.WidgetWidth)
        else:
            self._text_edit.setLineWrapMode(QPlainTextEdit.LineWrapMode.NoWrap)

    def _toggle_errors_only(self, checked: bool):
        self._errors_only = checked
        self._apply_filter(self._filter_input.text())

    def _toggle_autoscroll(self, checked: bool):
        self._auto_scroll = checked
        self._autoscroll_btn.setText("Auto-scroll: ON" if checked else "Auto-scroll: OFF")
        if checked:
            self._scroll_to_bottom()

    def _scroll_to_bottom(self):
        scrollbar = self._text_edit.verticalScrollBar()
        scrollbar.setValue(scrollbar.maximum())

    def _on_scroll(self, value: int):
        scrollbar = self._text_edit.verticalScrollBar()
        is_at_bottom = value >= scrollbar.maximum() - 10
        if not is_at_bottom and self._auto_scroll:
            self._auto_scroll = False
            self._autoscroll_btn.setChecked(False)
            self._autoscroll_btn.setText("Auto-scroll: OFF")
        elif is_at_bottom and not self._auto_scroll:
            self._auto_scroll = True
            self._autoscroll_btn.setChecked(True)
            self._autoscroll_btn.setText("Auto-scroll: ON")
