# app/ui/widgets/monitor_panel.py
import os
from typing import Dict, List, Optional
from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QGroupBox, QPushButton,
    QCheckBox, QProgressBar, QScrollArea, QFrame, QSizePolicy
)
from PySide6.QtCore import Signal, Qt, QSize, QTimer
from PySide6.QtGui import QFont
from app.models.project import Project
from app.ui.icons import get_icon
from app.ui.theme import ThemeManager, ThemeMode, ui_font, mono_font

POLL_INTERVAL_MS = 3000


def _clear_layout(layout):
    while layout.count():
        item = layout.takeAt(0)
        w = item.widget()
        if w is not None:
            w.deleteLater()
        elif item.layout() is not None:
            _clear_layout(item.layout())


class PortRow(QFrame):
    kill_requested = Signal(int)

    def __init__(self, row: dict, parent=None):
        super().__init__(parent)
        self.setObjectName("portRow")
        lay = QHBoxLayout(self)
        lay.setContentsMargins(12, 8, 12, 8)
        lay.setSpacing(10)

        self._name_lbl = QLabel(row["name"])
        self._name_lbl.setFont(ui_font(13, QFont.Weight.DemiBold))
        self._name_lbl.setFixedWidth(150)
        lay.addWidget(self._name_lbl)

        self._port_lbl = QLabel(f":{row['port']}")
        self._port_lbl.setFont(mono_font(12))
        lay.addWidget(self._port_lbl)
        lay.addStretch()

        self._status_lbl = QLabel()
        lay.addWidget(self._status_lbl)

        self._kill_btn = QPushButton(" Kill")
        self._kill_btn.setIcon(get_icon("trash", "#ffffff"))
        self._kill_btn.setIconSize(QSize(13, 13))
        self._kill_btn.setFixedSize(80, 28)
        self._kill_btn.clicked.connect(lambda: self.kill_requested.emit(row.get("owner_pid", 0)))
        lay.addWidget(self._kill_btn)

        self.apply_row(row)

    def apply_row(self, row: dict):
        c = ThemeManager.instance().get_colors()
        state = row.get("state")
        if state == "free":
            self._status_lbl.setText("✓ Free")
            self._status_lbl.setStyleSheet(f"color: {c['text_muted']}; font-size: 12px;")
            self._kill_btn.hide()
        elif state == "ours":
            self._status_lbl.setText(f"◉ Used by '{row.get('owner_name', '')}'")
            self._status_lbl.setStyleSheet(
                f"color: {c['primary']}; background-color: {c['primary_light']}; "
                f"border-radius: 10px; padding: 3px 10px; font-size: 11px; font-weight: 600;"
            )
            self._kill_btn.hide()
        else:  # foreign
            self._status_lbl.setText(
                f"⚠ {row.get('owner_name', '?')} (PID {row.get('owner_pid', '?')})"
            )
            self._status_lbl.setStyleSheet(
                f"color: {c['warning']}; background-color: {c['warning_bg']}; "
                f"border-radius: 10px; padding: 3px 10px; font-size: 11px; font-weight: 600;"
            )
            self._kill_btn.show()


class ResourceRow(QFrame):
    def __init__(self, row: dict, parent=None):
        super().__init__(parent)
        self.setObjectName("resRow")
        lay = QHBoxLayout(self)
        lay.setContentsMargins(12, 8, 12, 8)
        lay.setSpacing(10)

        self._name_lbl = QLabel(row["name"])
        self._name_lbl.setFont(ui_font(13, QFont.Weight.DemiBold))
        self._name_lbl.setFixedWidth(150)
        lay.addWidget(self._name_lbl)

        self._pid_lbl = QLabel(f"PID {row['pid']}" + (f" (+{row['children']})" if row.get("children") else ""))
        self._pid_lbl.setFont(mono_font(11))
        self._pid_lbl.setFixedWidth(110)
        lay.addWidget(self._pid_lbl)

        self._bar = QProgressBar()
        self._bar.setRange(0, 100)
        self._bar.setTextVisible(False)
        self._bar.setFixedHeight(8)
        lay.addWidget(self._bar, stretch=1)

        self._usage_lbl = QLabel()
        self._usage_lbl.setFont(mono_font(12))
        self._usage_lbl.setFixedWidth(170)
        lay.addWidget(self._usage_lbl)

        self.apply_row(row)

    def apply_row(self, row: dict):
        c = ThemeManager.instance().get_colors()
        cpu = float(row.get("cpu", 0.0))
        rss = float(row.get("rss", 0.0))
        display_cpu = min(cpu, 100.0)
        self._bar.setValue(int(display_cpu))
        if display_cpu >= 80:
            color = c["danger"]
        elif display_cpu >= 50:
            color = c["warning"]
        else:
            color = c["success"]
        self._bar.setStyleSheet(
            f"QProgressBar {{ background-color: {c['bg_elevated']}; border: none; border-radius: 4px; }} "
            f"QProgressBar::chunk {{ background-color: {color}; border-radius: 4px; }}"
        )
        self._usage_lbl.setText(f"{cpu:>5.1f}%  {rss:>7.1f} MB")


class MonitorPanel(QWidget):
    refresh_requested = Signal()
    kill_requested = Signal(int)  # pid

    def __init__(self, parent=None):
        super().__init__(parent)
        self._projects: List[Project] = []
        self._polling_enabled = True

        self._timer = QTimer(self)
        self._timer.setInterval(POLL_INTERVAL_MS)
        self._timer.timeout.connect(lambda: self.refresh_requested.emit())

        self._setup_ui()
        self.apply_theme(ThemeManager.instance().mode)

    def _setup_ui(self):
        outer = QVBoxLayout(self)
        outer.setContentsMargins(14, 14, 14, 14)
        outer.setSpacing(12)

        # Toolbar
        bar = QHBoxLayout()
        self._refresh_btn = QPushButton(" Refresh")
        self._refresh_btn.setIcon(get_icon("restart", "#ffffff"))
        self._refresh_btn.setIconSize(QSize(14, 14))
        self._refresh_btn.setFixedHeight(34)
        self._refresh_btn.clicked.connect(self.refresh_requested.emit)
        bar.addWidget(self._refresh_btn)

        self._auto_cb = QCheckBox("Auto-refresh (3s)")
        self._auto_cb.setChecked(True)
        self._auto_cb.toggled.connect(self._on_auto_toggled)
        bar.addWidget(self._auto_cb)
        bar.addStretch()

        self._summary_lbl = QLabel("")
        bar.addWidget(self._summary_lbl)
        outer.addLayout(bar)

        # Scrollable content
        scroll = QScrollArea()
        scroll.setWidgetResizable(True)
        scroll.setFrameShape(QFrame.Shape.NoFrame)
        content = QWidget()
        content_layout = QVBoxLayout(content)
        content_layout.setContentsMargins(0, 0, 0, 0)
        content_layout.setSpacing(12)

        self._ports_group = QGroupBox("Configured Ports")
        self._ports_layout = QVBoxLayout(self._ports_group)
        self._ports_layout.setContentsMargins(14, 16, 14, 14)
        self._ports_layout.setSpacing(6)
        content_layout.addWidget(self._ports_group)

        self._res_group = QGroupBox("Resources (Running Servers)")
        res_outer = QVBoxLayout(self._res_group)
        res_outer.setContentsMargins(14, 16, 14, 14)
        self._disabled_lbl = QLabel("Resource polling disabled in Settings.")
        self._disabled_lbl.setAlignment(Qt.AlignmentFlag.AlignCenter)
        res_outer.addWidget(self._disabled_lbl)
        self._res_layout = QVBoxLayout()
        self._res_layout.setSpacing(6)
        res_outer.addLayout(self._res_layout)
        content_layout.addWidget(self._res_group)
        content_layout.addStretch()

        scroll.setWidget(content)
        outer.addWidget(scroll, stretch=1)

    # ---------- Visibility-gated polling ----------

    def showEvent(self, event):
        self._update_timer_state()
        super().showEvent(event)

    def hideEvent(self, event):
        self._timer.stop()
        super().hideEvent(event)

    def set_polling_enabled(self, enabled: bool):
        self._polling_enabled = enabled
        self._disabled_lbl.setVisible(not enabled)
        self._update_timer_state()

    def _on_auto_toggled(self, checked: bool):
        self._update_timer_state()

    def _update_timer_state(self):
        should_run = (
            self.isVisible()
            and self._polling_enabled
            and self._auto_cb.isChecked()
        )
        if should_run and not self._timer.isActive():
            self.refresh_requested.emit()  # refresh inmediato al activarse
            self._timer.start()
        elif not should_run and self._timer.isActive():
            self._timer.stop()

    # ---------- Data ----------

    def set_projects(self, projects: List[Project]):
        self._projects = projects

    def update_port_rows(self, rows: List[dict]):
        _clear_layout(self._ports_layout)
        if not rows:
            empty = QLabel("No projects with servers configured.")
            empty.setAlignment(Qt.AlignmentFlag.AlignCenter)
            self._ports_layout.addWidget(empty)
            return
        for row in rows:
            port_row = PortRow(row)
            port_row.kill_requested.connect(self.kill_requested.emit)
            self._ports_layout.addWidget(port_row)
        conflicts = sum(1 for r in rows if r.get("state") == "foreign")
        running = sum(1 for r in rows if r.get("state") == "ours")
        self._summary_lbl.setText(f"{len(rows)} ports · {running} in use · {conflicts} conflict(s)")

    def update_resources(self, rows: List[dict]):
        _clear_layout(self._res_layout)
        if not rows:
            empty = QLabel("No servers running.")
            empty.setAlignment(Qt.AlignmentFlag.AlignCenter)
            self._res_layout.addWidget(empty)
            return
        for row in rows:
            self._res_layout.addWidget(ResourceRow(row))

    # ---------- Theme ----------

    def apply_theme(self, mode: ThemeMode):
        c = ThemeManager.instance().get_colors(mode)
        res_dir = os.path.join(os.path.dirname(__file__), "..", "resources").replace("\\", "/")
        group_style = (
            f"QGroupBox {{ color: {c['text_secondary']}; font-weight: bold; "
            f"border: 1px solid {c['border']}; border-radius: 8px; margin-top: 10px; }} "
            f"QGroupBox::title {{ subcontrol-origin: margin; left: 12px; padding: 0 4px; }}"
        )
        self._ports_group.setStyleSheet(group_style)
        self._res_group.setStyleSheet(group_style)
        self._refresh_btn.setStyleSheet(
            f"QPushButton {{ background-color: {c['primary']}; color: white; font-weight: 600; "
            f"border: none; border-radius: 8px; padding: 6px 14px; }} "
            f"QPushButton:hover {{ background-color: {c['primary_hover']}; }}"
        )
        for cb in (self._auto_cb,):
            cb.setStyleSheet(
                f"QCheckBox {{ color: {c['text_secondary']}; font-size: 12px; font-weight: 500; spacing: 8px; }} "
                f"QCheckBox::indicator {{ width: 16px; height: 16px; border: 1px solid {c['border']}; border-radius: 4px; background-color: {c['bg_elevated']}; }} "
                f"QCheckBox::indicator:hover {{ border: 1px solid {c['primary']}; }} "
                f"QCheckBox::indicator:checked {{ background-color: {c['primary']}; border: 1px solid {c['primary']}; image: url('{res_dir}/check.svg'); }}"
            )
        self._summary_lbl.setStyleSheet(f"color: {c['text_muted']}; font-size: 12px;")
        self._disabled_lbl.setStyleSheet(f"color: {c['warning']}; font-size: 12px;")
        self._scroll_widget_style(c)

    def _scroll_widget_style(self, c):
        scroll = self.findChild(QScrollArea)
        if scroll:
            scroll.setStyleSheet(f"QScrollArea {{ background-color: transparent; border: none; }}")
        for frame in self.findChildren(QFrame):
            if frame.objectName() in ("portRow", "resRow"):
                frame.setStyleSheet(
                    f"QFrame#{frame.objectName()} {{ background-color: {c['bg_surface']}; "
                    f"border: 1px solid {c['border']}; border-radius: 8px; }}"
                )
