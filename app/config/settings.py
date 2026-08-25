# -*- coding: utf-8 -*-
# app/config/settings.py
from typing import Any, Optional
from PySide6.QtCore import QObject, Signal, QSettings

KEY_POLLING_ENABLED = "monitor/polling_enabled"
KEY_TOASTS_ENABLED = "notifications/toasts_enabled"

# Effective defaults used by set() to suppress signals for no-op writes.
KNOWN_DEFAULTS: dict[str, Any] = {
    KEY_POLLING_ENABLED: True,
    KEY_TOASTS_ENABLED: True,
}


class AppSettings(QObject):
    """Application-wide settings backed by QSettings (Windows registry).

    Singleton mirroring ThemeManager.instance(). Inject a QSettings for tests.
    """

    setting_changed = Signal(str, object)  # (key, value)

    _instance = None

    def __init__(self, settings: Optional[QSettings] = None, parent=None):
        super().__init__(parent)
        self._settings = settings or QSettings("LocalDevManager", "Settings")
        self._cache: dict[str, Any] = {}

    @classmethod
    def instance(cls, parent=None) -> 'AppSettings':
        if cls._instance is None:
            cls._instance = cls(parent=parent)
        return cls._instance

    def _to_bool(self, value: Any) -> bool:
        if isinstance(value, bool):
            return value
        if isinstance(value, str):
            return value.strip().lower() in ("true", "1", "yes", "on")
        return bool(value)

    def get(self, key: str, default: Any = None) -> Any:
        if key in self._cache:
            return self._cache[key]
        raw = self._settings.value(key, default)
        if isinstance(default, bool) or isinstance(raw, str) and isinstance(default, bool):
            value = self._to_bool(raw)
        else:
            value = raw
        self._cache[key] = value
        return value

    def set(self, key: str, value: Any):
        current = self.get(key, KNOWN_DEFAULTS.get(key))
        self._settings.setValue(key, value)
        self._settings.sync()
        self._cache[key] = value
        if current != value:
            self.setting_changed.emit(key, value)
