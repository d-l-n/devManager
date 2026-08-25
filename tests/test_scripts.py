import json
import pytest
from app.models.project import Project
from app.utils.detection import detect_package_manager, get_project_scripts
from app.scripts.manager import ScriptManager
from app.ui.widgets.scripts_panel import ScriptsPanel


@pytest.fixture(scope='session')
def qapp():
    from PySide6.QtWidgets import QApplication
    app = QApplication.instance() or QApplication([])
    return app


def test_detect_package_manager(tmp_path):
    assert detect_package_manager(str(tmp_path)) == "npm"

    (tmp_path / "pnpm-lock.yaml").write_text("", encoding='utf-8')
    assert detect_package_manager(str(tmp_path)) == "pnpm"

    (tmp_path / "pnpm-lock.yaml").unlink()
    (tmp_path / "yarn.lock").write_text("", encoding='utf-8')
    assert detect_package_manager(str(tmp_path)) == "yarn"

    (tmp_path / "yarn.lock").unlink()
    (tmp_path / "bun.lockb").write_text("", encoding='utf-8')
    assert detect_package_manager(str(tmp_path)) == "bun"


def test_get_project_scripts(tmp_path):
    pkg_json = tmp_path / "package.json"
    pkg_json.write_text(json.dumps({
        "scripts": {
            "build": "vite build",
            "lint": "eslint .",
            "test": "vitest"
        }
    }), encoding='utf-8')

    scripts = get_project_scripts(str(tmp_path))
    assert "build" in scripts
    assert "lint" in scripts
    assert "test" in scripts
    assert scripts["build"] == "npm run build"
    assert scripts["lint"] == "npm run lint"
    assert scripts["test"] == "npm test"


def test_script_manager_lifecycle(qapp, tmp_path):
    project = Project(name="Script Proj", path=str(tmp_path))
    manager = ScriptManager(project)

    assert not manager.is_running()
    assert manager.active_script_name is None

    # Test running a simple echo command
    outputs = []
    manager.log_output.connect(lambda msg, err: outputs.append(msg))
    
    manager.run_script("echo_test", "cmd.exe /c echo Hello from script")
    assert manager.is_running()
    assert manager.active_script_name == "echo_test"

    # Wait briefly or stop
    manager.stop()
    assert not manager.is_running()


def test_scripts_panel_ui(qapp, tmp_path):
    pkg_json = tmp_path / "package.json"
    pkg_json.write_text(json.dumps({
        "scripts": {
            "build": "vite build",
            "lint": "eslint ."
        }
    }), encoding='utf-8')

    project = Project(name="UI Proj", path=str(tmp_path))
    panel = ScriptsPanel()
    panel.set_project(project)
    panel.set_controls_enabled(True)

    assert len(panel._rows) == 2
    assert "build" in panel._rows
    assert "lint" in panel._rows

    # Test filter
    panel._filter_rows("build")
    assert not panel._rows["build"].isHidden()
    assert panel._rows["lint"].isHidden()

    # Test custom command signal
    custom_calls = []
    panel.run_script_requested.connect(lambda name, cmd: custom_calls.append((name, cmd)))
    panel._custom_input.setText("npm run custom-test")
    panel._on_run_custom()
    assert len(custom_calls) == 1
    assert custom_calls[0] == ("custom", "npm run custom-test")

    panel.close()
