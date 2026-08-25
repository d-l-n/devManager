# -*- coding: utf-8 -*-
import pytest
from PySide6.QtWidgets import QApplication
from app.config.manager import ConfigManager
from app.ui.main_window import MainWindow


@pytest.fixture(scope='session')
def qapp():
    app = QApplication.instance() or QApplication([])
    return app


def test_main_window_initialization(qapp, tmp_path):
    config_file = tmp_path / "projects.json"
    config_file.write_text('{"projects": []}', encoding='utf-8')
    config_manager = ConfigManager(str(config_file))
    
    window = MainWindow(config_manager)
    assert window.windowTitle() == "Local Dev Manager"
    assert window._theme_btn is not None
    assert window._sidebar is not None
    assert window._server_panel is not None
    assert window._playwright_panel is not None
    assert window._scripts_panel is not None
    assert window._log_panel is not None
    assert window._app_log_panel is not None
    assert window._opencode_btn is not None
    assert not window._opencode_btn.isEnabled()
    window.close()


def test_main_window_with_project_opencode(qapp, tmp_path, monkeypatch):
    import json
    from PySide6.QtCore import QProcess

    project_dir = tmp_path / "my_project"
    project_dir.mkdir()

    config_data = {
        "projects": [
            {
                "name": "My App",
                "path": str(project_dir),
                "server": {"command": "npm run dev", "port": 3000, "url": "http://localhost:3000", "enabled": True},
                "playwright": {"command": "npx playwright test", "enabled": True}
            }
        ]
    }
    config_file = tmp_path / "projects.json"
    config_file.write_text(json.dumps(config_data), encoding='utf-8')
    config_manager = ConfigManager(str(config_file))

    window = MainWindow(config_manager)
    assert window._opencode_btn.isEnabled()

    # Test detached start call for opencode
    started_calls = []
    def fake_start_detached(prog, args, *rest):
        started_calls.append((prog, args))
        return True

    monkeypatch.setattr(QProcess, "startDetached", fake_start_detached)
    window._open_current_in_opencode()
    assert len(started_calls) == 1

    window.close()


def test_main_window_reload_config(qapp, tmp_path):
    import json

    config_file = tmp_path / "projects.json"
    config_file.write_text('{"projects": []}', encoding='utf-8')
    config_manager = ConfigManager(str(config_file))

    window = MainWindow(config_manager)
    assert window._sidebar._list.count() == 0

    # Simulate modifying projects.json externally
    new_data = {
        "projects": [
            {
                "name": "External Project",
                "path": str(tmp_path),
                "server": {"command": "npm run dev", "port": 3001, "url": "http://localhost:3001", "enabled": True},
                "playwright": {"command": "npx playwright test", "enabled": True}
            }
        ]
    }
    config_file.write_text(json.dumps(new_data), encoding='utf-8')

    # Trigger reload
    window._on_reload_config()
    assert window._sidebar._list.count() == 1
    assert window.config_manager.project_count() == 1
    assert window.config_manager.get_project(0).name == "External Project"

    window.close()


def test_main_window_restart_app(qapp, tmp_path, monkeypatch):
    from PySide6.QtCore import QProcess
    from PySide6.QtWidgets import QMessageBox

    config_file = tmp_path / "projects.json"
    config_file.write_text('{"projects": []}', encoding='utf-8')
    config_manager = ConfigManager(str(config_file))
    window = MainWindow(config_manager)

    restarted = []
    quit_called = []

    def fake_start_detached(prog, args):
        restarted.append((prog, args))
        return True

    monkeypatch.setattr(QProcess, "startDetached", fake_start_detached)
    monkeypatch.setattr(QApplication, "quit", lambda: quit_called.append(True))
    monkeypatch.setattr(QMessageBox, "question", lambda *args, **kwargs: QMessageBox.StandardButton.Yes)

    window._on_restart_app()

    assert len(restarted) == 1
    assert len(quit_called) == 1

    window.close()


def test_main_window_terminal_and_pin(qapp, tmp_path, monkeypatch):
    import json
    from PySide6.QtCore import QProcess

    project_dir = tmp_path / "term_project"
    project_dir.mkdir()

    config_data = {
        "projects": [
            {
                "name": "Term App",
                "path": str(project_dir),
                "server": {"command": "npm run dev", "port": 3000, "url": "http://localhost:3000", "enabled": True},
                "playwright": {"command": "npx playwright test", "enabled": True},
                "pinned": False
            }
        ]
    }
    config_file = tmp_path / "projects.json"
    config_file.write_text(json.dumps(config_data), encoding='utf-8')
    config_manager = ConfigManager(str(config_file))

    window = MainWindow(config_manager)
    assert window._terminal_btn is not None
    assert window._terminal_btn.isEnabled()

    # Test detached start call for terminal
    term_calls = []
    def fake_start_detached(prog, args=None, *rest):
        term_calls.append((prog, args))
        return True

    monkeypatch.setattr(QProcess, "startDetached", fake_start_detached)
    window._open_current_in_terminal()
    assert len(term_calls) == 1

    # Test toggle pin
    assert not window.config_manager.get_project(0).pinned
    window._on_toggle_pin(0)
    assert window.config_manager.get_project(0).pinned
    assert "📌" in window._sidebar._list.item(0).text()

    window.close()


def test_log_panel_save_logs(qapp, tmp_path, monkeypatch):
    from app.ui.widgets.log_panel import LogPanel
    from PySide6.QtWidgets import QFileDialog

    panel = LogPanel()
    panel.append_log("Test message 1")
    panel.append_error("Test error 1")

    out_file = tmp_path / "exported.log"
    monkeypatch.setattr(QFileDialog, "getSaveFileName", lambda *args, **kwargs: (str(out_file), "Log Files (*.log)"))

    panel._save_logs()
    assert out_file.exists()
    content = out_file.read_text(encoding='utf-8')
    assert "Test message 1" in content
    assert "Test error 1" in content
    panel.close()



