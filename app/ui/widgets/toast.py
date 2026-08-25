# app/ui/widgets/toast.py
"""Stacked toast notifications overlaid on the main window (bottom-right)."""
from enum import Enum
from PySide6.QtWidgets import QWidget, QFrame, QHBoxLayout, QVBoxLayout, QLabel, QGraphicsOpacityEffect
from PySide6.QtCore import Qt, QTimer, QPropertyAnimation, QEasingCurve
from PySide6.QtGui import QFont
from app.config.settings import AppSettings, KEY_TOASTS_ENABLED
from app.ui.icons import get_icon
from app.ui.theme import ThemeManager, ui_font

TOAST_WIDTH = 340
TOAST_SPACING = 8
MARGIN = 16
MAX_VISIBLE = 3


class ToastLevel(Enum):
    SUCCESS = "success"
    INFO = "info"
    WARNING = "warning"
    ERROR = "error"


_LEVEL_STYLE = {
    ToastLevel.SUCCESS: ("success", "check"),
    ToastLevel.INFO: ("info", "info"),
    ToastLevel.WARNING: ("warning", "bell"),
    ToastLevel.ERROR: ("danger", "bug"),
}


class ToastFrame(QFrame):
    """Single toast card. Click dismisses immediately."""

    def __init__(self, title: str, message: str, level: ToastLevel, duration_ms: int, on_close, parent=None):
        super().__init__(parent)
        self._on_close = on_close
        self.setFixedWidth(TOAST_WIDTH)
        self.setCursor(Qt.CursorShape.PointingHandCursor)
        self.mousePressEvent = lambda _e: self.close_now()

        color_key, icon_name = _LEVEL_STYLE[level]
        c = ThemeManager.instance().get_colors()
        accent = c[color_key]

        self.setStyleSheet(
            f"QFrame {{ background-color: {c['bg_card']}; border: 1px solid {accent}; "
            f"border-left: 4px solid {accent}; border-radius: 8px; }}"
        )

        row = QHBoxLayout(self)
        row.setContentsMargins(12, 10, 12, 10)
        row.setSpacing(10)

        icon_label = QLabel()
        icon_label.setPixmap(get_icon(icon_name, accent).pixmap(18, 18))
        icon_label.setFixedWidth(20)
        icon_label.setStyleSheet("border: none;")
        row.addWidget(icon_label, 0, Qt.AlignmentFlag.AlignTop)

        col = QVBoxLayout()
        col.setSpacing(2)
        title_lbl = QLabel(title)
        title_lbl.setFont(ui_font(13, QFont.Weight.DemiBold))
        msg_lbl = QLabel(message)
        msg_lbl.setFont(ui_font(12))
        msg_lbl.setWordWrap(True)
        for lbl in (title_lbl, msg_lbl):
            lbl.setStyleSheet(f"border: none; color: {c['text_primary']};")
        col.addWidget(title_lbl)
        col.addWidget(msg_lbl)
        row.addLayout(col, stretch=1)

        self._effect = QGraphicsOpacityEffect(self)
        self.setGraphicsEffect(self._effect)
        self._anim = QPropertyAnimation(self._effect, b"opacity", self)
        self._anim.setDuration(220)
        self._anim.setEasingCurve(QEasingCurve.Type.OutCubic)

        QTimer.singleShot(duration_ms, self.close_now)

    def appear(self):
        self._anim.stop()
        self._anim.setStartValue(0.0)
        self._anim.setEndValue(1.0)
        self._anim.start()

    def close_now(self):
        self._anim.stop()
        self._anim.setStartValue(self._effect.opacity())
        self._anim.setEndValue(0.0)
        try:
            self._anim.finished.disconnect()
        except RuntimeError:
            pass
        self._anim.finished.connect(self._finish_close)
        self._anim.start()

    def _finish_close(self):
        self._on_close(self)
        self.deleteLater()


class ToastManager(QWidget):
    """Container stacking up to MAX_VISIBLE toasts at the main window's bottom-right."""

    def __init__(self, main_window: QWidget):
        super().__init__(main_window)
        self._host = main_window
        self._toasts: list[ToastFrame] = []
        self.setAttribute(Qt.WidgetAttribute.WA_TranslucentBackground)
        self.setAttribute(Qt.WidgetAttribute.WA_ShowWithoutActivating)

        self._layout = QVBoxLayout(self)
        self._layout.setContentsMargins(0, 0, 0, 0)
        self._layout.setSpacing(TOAST_SPACING)
        self._layout.setAlignment(Qt.AlignmentFlag.AlignBottom | Qt.AlignmentFlag.AlignRight)
        self.reposition()

    def show(self, title: str, message: str, level: ToastLevel = ToastLevel.INFO, duration_ms: int = 4000):
        if not AppSettings.instance().get(KEY_TOASTS_ENABLED, True):
            return
        toast = ToastFrame(title, message, level, duration_ms, self._remove, self)
        self._toasts.append(toast)
        self._layout.addWidget(toast)
        while len(self._toasts) > MAX_VISIBLE:
            oldest = self._toasts.pop(0)
            oldest.close_now()
        toast.appear()
        self.reposition()
        self.raise_()
        self.show_normal()

    def show_normal(self):
        QWidget.setVisible(self, True)

    def _remove(self, toast: ToastFrame):
        if toast in self._toasts:
            self._toasts.remove(toast)
        self.reposition()
        if not self._toasts:
            QWidget.setVisible(self, False)

    def reposition(self):
        if not self._host:
            return
        host_w = self._host.width()
        host_h = self._host.height()
        height = sum(t.sizeHint().height() for t in self._toasts) + TOAST_SPACING * max(0, len(self._toasts) - 1)
        height = max(height, 60)
        self.setGeometry(
            host_w - TOAST_WIDTH - MARGIN * 2,
            host_h - height - MARGIN * 2,
            TOAST_WIDTH + MARGIN,
            height + MARGIN,
        )
