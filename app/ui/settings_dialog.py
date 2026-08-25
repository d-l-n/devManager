# app/ui/settings_dialog.py
import os
from PySide6.QtWidgets import (
    QDialog, QVBoxLayout, QGroupBox, QCheckBox, QDialogButtonBox, QLabel
)
from PySide6.QtCore import Qt
from PySide6.QtGui import QFont
from app.config.settings import AppSettings, KEY_POLLING_ENABLED, KEY_TOASTS_ENABLED
from app.ui.icons import get_icon
from app.ui.theme import ThemeManager, ui_font


class SettingsDialog(QDialog):
    """Application settings. Toggles persist immediately via AppSettings."""

    def __init__(self, parent=None):
        super().__init__(parent)
        self.setWindowTitle("Settings")
        self.setMinimumWidth(440)
        self._settings = AppSettings.instance()

        layout = QVBoxLayout(self)
        layout.setContentsMargins(18, 18, 18, 14)
        layout.setSpacing(14)

        # --- Monitors ---
        monitors_group = QGroupBox("Monitors")
        mon_layout = QVBoxLayout(monitors_group)
        mon_layout.setContentsMargins(14, 16, 14, 14)
        mon_layout.setSpacing(8)

        self._polling_cb = QCheckBox("Enable resource polling (CPU / RAM)")
        self._polling_cb.setChecked(self._settings.get(KEY_POLLING_ENABLED, True))
        self._polling_cb.setToolTip("When off, the Monitor tab stops auto-refreshing resources. Manual refresh still works.")
        self._polling_cb.toggled.connect(
            lambda v: self._settings.set(KEY_POLLING_ENABLED, bool(v))
        )
        mon_layout.addWidget(self._polling_cb)

        polling_hint = QLabel("Polling runs every 3 seconds while the Monitor tab is visible.")
        polling_hint.setObjectName("hintLabel")
        mon_layout.addWidget(polling_hint)
        layout.addWidget(monitors_group)

        # --- Notifications ---
        notif_group = QGroupBox("Notifications")
        notif_layout = QVBoxLayout(notif_group)
        notif_layout.setContentsMargins(14, 16, 14, 14)
        notif_layout.setSpacing(8)

        self._toasts_cb = QCheckBox("Enable in-app toast notifications")
        self._toasts_cb.setChecked(self._settings.get(KEY_TOASTS_ENABLED, True))
        self._toasts_cb.setToolTip("When off, notifications go only to the system tray.")
        self._toasts_cb.toggled.connect(
            lambda v: self._settings.set(KEY_TOASTS_ENABLED, bool(v))
        )
        notif_layout.addWidget(self._toasts_cb)
        layout.addWidget(notif_group)

        # --- Buttons ---
        buttons = QDialogButtonBox(QDialogButtonBox.StandardButton.Ok)
        buttons.accepted.connect(self.accept)
        layout.addWidget(buttons)

        self.apply_theme()

    def apply_theme(self):
        c = ThemeManager.instance().get_colors()
        res_dir = os.path.join(os.path.dirname(__file__), "resources").replace("\\", "/")
        hint_style = f"QLabel#hintLabel {{ color: {c['text_muted']}; font-size: 11px; }}"
        cb_style = (
            f"QCheckBox {{ color: {c['text_primary']}; font-size: 13px; spacing: 8px; }} "
            f"QCheckBox::indicator {{ width: 16px; height: 16px; border: 1px solid {c['border']}; border-radius: 4px; background-color: {c['bg_elevated']}; }} "
            f"QCheckBox::indicator:hover {{ border: 1px solid {c['primary']}; }} "
            f"QCheckBox::indicator:checked {{ background-color: {c['primary']}; border: 1px solid {c['primary']}; image: url('{res_dir}/check.svg'); }}"
        )
        for group in self.findChildren(QGroupBox):
            group.setStyleSheet(
                f"QGroupBox {{ color: {c['text_secondary']}; font-weight: bold; "
                f"border: 1px solid {c['border']}; border-radius: 8px; margin-top: 10px; }} "
                f"QGroupBox::title {{ subcontrol-origin: margin; left: 12px; padding: 0 4px; }} "
                f"{cb_style} {hint_style}"
            )
