import os
from enum import Enum
from typing import Dict, Any, Optional
from PySide6.QtCore import QObject, Signal, QSettings
from PySide6.QtGui import QFont


class ThemeMode(Enum):
    DARK = "dark"
    OLED = "oled"   # true-black variant of DARK for OLED panels
    LIGHT = "light"


# Order used by toggle_theme() / toolbar cycle button.
THEME_CYCLE = [ThemeMode.LIGHT, ThemeMode.DARK, ThemeMode.OLED]


# ---------------------------------------------------------------------------
# Typography
# 'Segoe UI Variable Text' is the cut designed for small UI sizes (better
# spacing/hinting at 12-14px than the 'Display' cut). Falls back gracefully.
# ---------------------------------------------------------------------------
FONT_FAMILY_UI = "'Segoe UI Variable Text', 'Segoe UI', 'Noto Sans', -apple-system, system-ui, sans-serif"
FONT_FAMILIES_UI = ["Segoe UI Variable Text", "Segoe UI", "Noto Sans"]
FONT_FAMILY_MONO = "'Cascadia Code', 'Consolas', 'DejaVu Sans Mono', monospace"
FONT_FAMILIES_MONO = ["Cascadia Code", "Consolas", "DejaVu Sans Mono"]

BASE_FONT_SIZE_PX = 13   # body text
SMALL_FONT_SIZE_PX = 12  # badges, buttons, status bar
MONO_FONT_SIZE_PX = 13   # terminal / logs


def ui_font(size_px: int = BASE_FONT_SIZE_PX,
            weight: QFont.Weight = QFont.Weight.Normal) -> QFont:
    """Theme-aware UI font using pixel size (matches px units in stylesheets)."""
    f = QFont()
    f.setFamilies(FONT_FAMILIES_UI)
    f.setPixelSize(size_px)
    f.setWeight(weight)
    f.setStyleStrategy(QFont.StyleStrategy.PreferAntialias)
    return f


def mono_font(size_px: int = MONO_FONT_SIZE_PX) -> QFont:
    """Theme-aware monospace font for terminal/log output."""
    f = QFont()
    f.setFamilies(FONT_FAMILIES_MONO)
    f.setPixelSize(size_px)
    f.setStyleStrategy(QFont.StyleStrategy.PreferAntialias)
    return f


class ThemeManager(QObject):
    theme_changed = Signal(object)  # ThemeMode

    _instance = None

    def __init__(self, parent=None):
        super().__init__(parent)
        self._settings = QSettings("LocalDevManager", "Theme")
        saved_mode = self._settings.value("mode", "dark")
        try:
            self._mode = ThemeMode(saved_mode)
        except ValueError:
            self._mode = ThemeMode.DARK

    @classmethod
    def instance(cls, parent=None) -> 'ThemeManager':
        if cls._instance is None:
            cls._instance = cls(parent)
        return cls._instance

    @property
    def mode(self) -> ThemeMode:
        return self._mode

    @property
    def is_dark(self) -> bool:
        """True for both DARK and OLED (dark-family modes)."""
        return self._mode != ThemeMode.LIGHT

    @property
    def is_oled(self) -> bool:
        return self._mode == ThemeMode.OLED

    def set_theme(self, mode: ThemeMode):
        if self._mode != mode:
            self._mode = mode
            self._settings.setValue("mode", mode.value)
            self.theme_changed.emit(mode)

    def toggle_theme(self) -> ThemeMode:
        """Cycle LIGHT -> DARK -> OLED -> LIGHT."""
        idx = THEME_CYCLE.index(self._mode)
        new_mode = THEME_CYCLE[(idx + 1) % len(THEME_CYCLE)]
        self.set_theme(new_mode)
        return new_mode

    def get_colors(self, mode: Optional[ThemeMode] = None) -> Dict[str, str]:
        m = mode or self._mode
        if m == ThemeMode.OLED:
            # True-black palette. Text/accent roles use brighter steps (-400)
            # than DARK so contrast on pure black stays >= 4.5:1 (WCAG AA).
            return {
                "bg_base": "#000000",
                "bg_surface": "#0b0d13",
                "bg_card": "#12151f",
                "bg_elevated": "#181c29",
                "bg_active": "#232940",
                "border": "#2a3049",
                "border_focus": "#818cf8",
                "border_subtle": "#151827",
                "primary": "#818cf8",
                "primary_hover": "#a5b0fc",
                "primary_light": "rgba(129, 140, 248, 0.16)",
                "success": "#34d399",
                "success_bg": "rgba(52, 211, 153, 0.14)",
                "warning": "#fbbf24",
                "warning_bg": "rgba(251, 191, 36, 0.14)",
                "danger": "#f87171",
                "danger_bg": "rgba(248, 113, 113, 0.14)",
                "info": "#60a5fa",
                "text_primary": "#ffffff",
                "text_secondary": "#9fb0c4",
                "text_muted": "#7e8aa0",
                "terminal_bg": "#000000",
                "terminal_fg": "#e8edf5",
                "badge_bg": "#141828",
                "icon_color": "#9fb0c4",
                "icon_active": "#818cf8",
            }
        if m == ThemeMode.DARK:
            return {
                "bg_base": "#0c0e17",
                "bg_surface": "#141724",
                "bg_card": "#1a1d2e",
                "bg_elevated": "#22263d",
                "bg_active": "#2b304c",
                "border": "#252a3f",
                "border_focus": "#6366f1",
                "border_subtle": "#1e2238",
                "primary": "#6366f1",
                "primary_hover": "#4f46e5",
                "primary_light": "rgba(99, 102, 241, 0.15)",
                "success": "#10b981",
                "success_bg": "rgba(16, 185, 129, 0.15)",
                "warning": "#f59e0b",
                "warning_bg": "rgba(245, 158, 11, 0.15)",
                "danger": "#ef4444",
                "danger_bg": "rgba(239, 68, 68, 0.15)",
                "info": "#3b82f6",
                "text_primary": "#f8fafc",
                "text_secondary": "#94a3b8",
                "text_muted": "#64748b",
                "terminal_bg": "#090a10",
                "terminal_fg": "#e2e8f0",
                "badge_bg": "#1c2033",
                "icon_color": "#94a3b8",
                "icon_active": "#6366f1",
            }
        else:
            return {
                "bg_base": "#f4f6f9",
                "bg_surface": "#ffffff",
                "bg_card": "#ffffff",
                "bg_elevated": "#ffffff",
                "bg_active": "#eef2ff",
                "border": "#e2e8f0",
                "border_focus": "#4f46e5",
                "border_subtle": "#edf2f7",
                "primary": "#4f46e5",
                "primary_hover": "#4338ca",
                "primary_light": "rgba(79, 70, 229, 0.1)",
                "success": "#059669",
                "success_bg": "rgba(5, 150, 105, 0.1)",
                "warning": "#d97706",
                "warning_bg": "rgba(217, 119, 6, 0.1)",
                "danger": "#dc2626",
                "danger_bg": "rgba(220, 38, 38, 0.1)",
                "info": "#2563eb",
                "text_primary": "#0f172a",
                "text_secondary": "#334155",
                "text_muted": "#64748b",
                "terminal_bg": "#f8fafc",
                "terminal_fg": "#1e293b",
                "badge_bg": "#f1f5f9",
                "icon_color": "#64748b",
                "icon_active": "#4f46e5",
            }

    def get_main_stylesheet(self, mode: Optional[ThemeMode] = None) -> str:
        c = self.get_colors(mode)
        res_dir = os.path.join(os.path.dirname(__file__), "resources").replace("\\", "/")
        up_icon = f"{res_dir}/arrow_up.svg"
        down_icon = f"{res_dir}/arrow_down.svg"

        return f"""
            QMainWindow {{
                background-color: {c["bg_base"]};
                color: {c["text_primary"]};
            }}
            QWidget {{
                background-color: {c["bg_base"]};
                color: {c["text_primary"]};
                font-family: {FONT_FAMILY_UI};
                font-size: {BASE_FONT_SIZE_PX}px;
                font-weight: 400;
            }}
            #headerCard {{
                background-color: {c["bg_surface"]};
                border: 1px solid {c["border"]};
                border-radius: 10px;
            }}
            #projectTitle {{
                color: {c["text_primary"]};
                font-size: 17px;
                font-weight: 700;
                letter-spacing: -0.3px;
            }}
            #projectSubtitle {{
                color: {c["text_muted"]};
                font-size: 12px;
            }}
            QSplitter::handle {{
                background-color: {c["border_subtle"]};
            }}
            QMenuBar {{
                background-color: {c["bg_surface"]};
                color: {c["text_secondary"]};
                border-bottom: 1px solid {c["border"]};
                padding: 4px 6px;
                font-weight: 500;
            }}
            QMenuBar::item {{
                background: transparent;
                padding: 5px 12px;
                border-radius: 6px;
            }}
            QMenuBar::item:selected {{
                background-color: {c["bg_elevated"]};
                color: {c["text_primary"]};
            }}
            QMenu {{
                background-color: {c["bg_surface"]};
                color: {c["text_primary"]};
                border: 1px solid {c["border"]};
                border-radius: 8px;
                padding: 6px;
            }}
            QMenu::item {{
                padding: 6px 24px;
                border-radius: 6px;
                font-weight: 500;
            }}
            QMenu::item:selected {{
                background-color: {c["bg_active"]};
                color: {c["primary"]};
            }}
            QMenu::separator {{
                height: 1px;
                background-color: {c["border"]};
                margin: 4px 8px;
            }}
            QToolBar {{
                background-color: {c["bg_surface"]};
                border-bottom: 1px solid {c["border"]};
                padding: 4px 8px;
                spacing: 8px;
            }}
            QToolBar QToolButton {{
                background: transparent;
                color: {c["text_secondary"]};
                border-radius: 6px;
                padding: 4px 10px;
                font-weight: 500;
            }}
            QToolBar QToolButton:hover {{
                background-color: {c["bg_elevated"]};
                color: {c["text_primary"]};
            }}
            QToolButton {{
                background-color: {c["bg_elevated"]};
                color: {c["text_primary"]};
                border: 1px solid {c["border"]};
                border-radius: 6px;
                padding: 4px;
            }}
            QToolButton:hover {{
                background-color: {c["bg_active"]};
                border-color: {c["border_focus"]};
            }}
            QToolButton:disabled {{
                background-color: {c["bg_base"]};
                color: {c["text_muted"]};
                border-color: {c["border"]};
            }}
            QToolTip {{
                background-color: {c["bg_elevated"]};
                color: {c["text_primary"]};
                border: 1px solid {c["border"]};
                border-radius: 4px;
                padding: 4px 8px;
                font-size: 12px;
            }}
            QStatusBar {{
                background-color: {c["bg_surface"]};
                color: {c["text_muted"]};
                border-top: 1px solid {c["border"]};
                padding: 3px 10px;
                font-size: 12px;
            }}
            QTabWidget::pane {{
                border: 1px solid {c["border"]};
                border-radius: 10px;
                background-color: {c["bg_surface"]};
                top: -1px;
            }}
            QTabBar::tab {{
                background-color: transparent;
                color: {c["text_muted"]};
                padding: 8px 18px;
                border-top-left-radius: 8px;
                border-top-right-radius: 8px;
                margin-right: 4px;
                font-weight: 600;
                border-bottom: 2px solid transparent;
            }}
            QTabBar::tab:selected {{
                background-color: {c["bg_surface"]};
                color: {c["primary"]};
                border-bottom: 2px solid {c["primary"]};
            }}
            QTabBar::tab:hover:!selected {{
                background-color: {c["bg_elevated"]};
                color: {c["text_primary"]};
            }}
            QLabel {{
                background-color: transparent;
            }}
            QGroupBox {{
                border: 1px solid {c["border"]};
                border-radius: 8px;
                margin-top: 10px;
                padding-top: 16px;
                font-weight: 600;
                color: {c["text_primary"]};
                background-color: {c["bg_surface"]};
            }}
            QGroupBox::title {{
                subcontrol-origin: margin;
                subcontrol-position: top left;
                padding: 0 8px;
                left: 12px;
                color: {c["primary"]};
            }}
            QPushButton {{
                background-color: {c["bg_elevated"]};
                color: {c["text_primary"]};
                border: 1px solid {c["border"]};
                border-radius: 6px;
                padding: 6px 14px;
                font-weight: 600;
                font-size: 12px;
            }}
            QPushButton:hover {{
                background-color: {c["bg_active"]};
                border-color: {c["border_focus"]};
            }}
            QPushButton:pressed {{
                background-color: {c["bg_surface"]};
            }}
            QPushButton:disabled {{
                background-color: {c["bg_base"]};
                color: {c["text_muted"]};
                border-color: {c["border"]};
            }}
            QLineEdit {{
                background-color: {c["bg_elevated"]};
                color: {c["text_primary"]};
                border: 1px solid {c["border"]};
                border-radius: 6px;
                padding: 6px 10px;
                selection-background-color: {c["primary"]};
            }}
            QLineEdit:focus {{
                border: 1px solid {c["border_focus"]};
                background-color: {c["bg_surface"]};
            }}
            QSpinBox {{
                background-color: {c["bg_elevated"]};
                color: {c["text_primary"]};
                border: 1px solid {c["border"]};
                border-radius: 6px;
                padding: 4px 28px 4px 8px;
                min-height: 24px;
            }}
            QSpinBox:focus {{
                border: 1px solid {c["border_focus"]};
            }}
            QSpinBox::up-button {{
                subcontrol-origin: border;
                subcontrol-position: top right;
                width: 22px;
                height: 14px;
                border-left: 1px solid {c["border"]};
                border-bottom: 1px solid {c["border"]};
                border-top-right-radius: 6px;
                background-color: {c["bg_elevated"]};
            }}
            QSpinBox::up-button:hover {{
                background-color: {c["bg_active"]};
            }}
            QSpinBox::up-arrow {{
                image: url('{up_icon}');
                width: 9px;
                height: 9px;
            }}
            QSpinBox::down-button {{
                subcontrol-origin: border;
                subcontrol-position: bottom right;
                width: 22px;
                height: 14px;
                border-left: 1px solid {c["border"]};
                border-bottom-right-radius: 6px;
                background-color: {c["bg_elevated"]};
            }}
            QSpinBox::down-button:hover {{
                background-color: {c["bg_active"]};
            }}
            QSpinBox::down-arrow {{
                image: url('{down_icon}');
                width: 9px;
                height: 9px;
            }}
            QScrollBar:vertical {{
                background-color: transparent;
                width: 10px;
                margin: 0px;
                border-radius: 5px;
            }}
            QScrollBar::handle:vertical {{
                background-color: {c["border"]};
                border-radius: 4px;
                min-height: 30px;
            }}
            QScrollBar::handle:vertical:hover {{
                background-color: {c["text_muted"]};
            }}
            QScrollBar::handle:vertical:pressed {{
                background-color: {c["primary"]};
            }}
            QScrollBar::add-line:vertical, QScrollBar::sub-line:vertical {{
                height: 0px;
            }}
            QScrollBar::add-page:vertical, QScrollBar::sub-page:vertical {{
                background: none;
            }}
            QScrollBar:horizontal {{
                background-color: transparent;
                height: 10px;
                margin: 0px;
                border-radius: 5px;
            }}
            QScrollBar::handle:horizontal {{
                background-color: {c["border"]};
                border-radius: 4px;
                min-width: 30px;
            }}
            QScrollBar::handle:horizontal:hover {{
                background-color: {c["text_muted"]};
            }}
            QScrollBar::handle:horizontal:pressed {{
                background-color: {c["primary"]};
            }}
            QScrollBar::add-line:horizontal, QScrollBar::sub-line:horizontal {{
                width: 0px;
            }}
            QScrollBar::add-page:horizontal, QScrollBar::sub-page:horizontal {{
                background: none;
            }}
            QScrollBar::corner {{
                background: transparent;
            }}
        """

    def get_dialog_stylesheet(self, mode: Optional[ThemeMode] = None) -> str:
        c = self.get_colors(mode)
        res_dir = os.path.join(os.path.dirname(__file__), "resources").replace("\\", "/")
        up_icon = f"{res_dir}/arrow_up.svg"
        down_icon = f"{res_dir}/arrow_down.svg"

        return f"""
            QDialog {{
                background-color: {c["bg_base"]};
                color: {c["text_primary"]};
            }}
            QTabWidget::pane {{
                border: 1px solid {c["border"]};
                border-radius: 8px;
                background-color: {c["bg_surface"]};
            }}
            QTabBar::tab {{
                background-color: {c["bg_elevated"]};
                color: {c["text_muted"]};
                padding: 8px 18px;
                border: 1px solid {c["border"]};
                border-bottom: none;
                border-top-left-radius: 8px;
                border-top-right-radius: 8px;
                font-weight: 600;
            }}
            QTabBar::tab:selected {{
                background-color: {c["bg_surface"]};
                color: {c["primary"]};
                border-top: 2px solid {c["primary"]};
            }}
            QLineEdit {{
                background-color: {c["bg_elevated"]};
                color: {c["text_primary"]};
                border: 1px solid {c["border"]};
                border-radius: 6px;
                padding: 6px 10px;
                min-height: 22px;
            }}
            QLineEdit:focus {{
                border: 1px solid {c["border_focus"]};
            }}
            QSpinBox {{
                background-color: {c["bg_elevated"]};
                color: {c["text_primary"]};
                border: 1px solid {c["border"]};
                border-radius: 6px;
                padding: 4px 28px 4px 8px;
                min-height: 24px;
            }}
            QSpinBox:focus {{
                border: 1px solid {c["border_focus"]};
            }}
            QSpinBox::up-button {{
                subcontrol-origin: border;
                subcontrol-position: top right;
                width: 22px;
                height: 14px;
                border-left: 1px solid {c["border"]};
                border-bottom: 1px solid {c["border"]};
                border-top-right-radius: 6px;
                background-color: {c["bg_elevated"]};
            }}
            QSpinBox::up-arrow {{
                image: url('{up_icon}');
                width: 9px;
                height: 9px;
            }}
            QSpinBox::down-button {{
                subcontrol-origin: border;
                subcontrol-position: bottom right;
                width: 22px;
                height: 14px;
                border-left: 1px solid {c["border"]};
                border-bottom-right-radius: 6px;
                background-color: {c["bg_elevated"]};
            }}
            QSpinBox::down-arrow {{
                image: url('{down_icon}');
                width: 9px;
                height: 9px;
            }}
            QCheckBox {{
                color: {c["text_primary"]};
                spacing: 8px;
                font-weight: 500;
            }}
            QCheckBox::indicator {{
                width: 16px;
                height: 16px;
                border: 1px solid {c["border"]};
                border-radius: 4px;
                background-color: {c["bg_elevated"]};
            }}
            QCheckBox::indicator:hover {{
                border: 1px solid {c["primary"]};
            }}
            QCheckBox::indicator:checked {{
                background-color: {c["primary"]};
                border: 1px solid {c["primary"]};
                image: url('{res_dir}/check.svg');
            }}
            QLabel {{
                color: {c["text_secondary"]};
                background-color: transparent;
            }}
            QPushButton {{
                background-color: {c["bg_elevated"]};
                color: {c["text_primary"]};
                border: 1px solid {c["border"]};
                border-radius: 6px;
                padding: 6px 16px;
                font-weight: 600;
                min-height: 24px;
            }}
            QPushButton:hover {{
                background-color: {c["bg_active"]};
                border-color: {c["border_focus"]};
            }}
            QPushButton:pressed {{
                background-color: {c["bg_surface"]};
            }}
        """
