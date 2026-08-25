# -*- coding: utf-8 -*-
# tests/test_settings.py
import pytest
from PySide6.QtCore import QSettings
from app.config.settings import AppSettings, KEY_POLLING_ENABLED, KEY_TOASTS_ENABLED


@pytest.fixture()
def qapp():
    from PySide6.QtWidgets import QApplication
    app = QApplication.instance() or QApplication([])
    return app


@pytest.fixture()
def settings(qapp, tmp_path):
    s = AppSettings(QSettings(str(tmp_path / "test_settings.ini"), QSettings.Format.IniFormat))
    yield s
    s._settings.clear()


def test_defaults(settings):
    assert settings.get(KEY_POLLING_ENABLED, True) is True
    assert settings.get(KEY_TOASTS_ENABLED, True) is True
    assert settings.get("nonexistent/key", "fallback") == "fallback"


def test_set_get_roundtrip(settings):
    settings.set(KEY_POLLING_ENABLED, False)
    assert settings.get(KEY_POLLING_ENABLED, True) is False
    settings.set(KEY_TOASTS_ENABLED, False)
    assert settings.get(KEY_TOASTS_ENABLED, True) is False


def test_signal_emitted_on_change(settings):
    received = []
    settings.setting_changed.connect(lambda k, v: received.append((k, v)))
    settings.set(KEY_POLLING_ENABLED, False)
    assert received == [(KEY_POLLING_ENABLED, False)]


def test_signal_not_emitted_when_same_value(settings):
    received = []
    settings.setting_changed.connect(lambda k, v: received.append((k, v)))
    settings.set(KEY_POLLING_ENABLED, True)   # default ya es True
    assert received == []


def test_bool_coercion_from_string(settings):
    # QSettings ini puede devolver "true"/"false" como string
    assert settings._to_bool("true") is True
    assert settings._to_bool("false") is False
    assert settings._to_bool(True) is True
    assert settings._to_bool(False) is False
