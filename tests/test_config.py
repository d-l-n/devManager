import json
import os
import pytest
from app.config.manager import ConfigManager
from app.models.project import Project, ServerConfig, PlaywrightConfig


@pytest.fixture(scope='session')
def qapp():
    from PySide6.QtWidgets import QApplication
    app = QApplication.instance() or QApplication([])
    return app


def test_load_empty_config(qapp, tmp_path):
    config_file = tmp_path / "projects.json"
    config_file.write_text('{"projects": []}', encoding='utf-8')
    manager = ConfigManager(str(config_file))
    assert manager.project_count() == 0


def test_load_nonexistent_creates_file(qapp, tmp_path):
    config_file = tmp_path / "new_projects.json"
    manager = ConfigManager(str(config_file))
    assert manager.project_count() == 0
    assert config_file.exists()
    content = json.loads(config_file.read_text(encoding='utf-8'))
    assert "projects" in content


def test_add_project(qapp, tmp_path):
    config_file = tmp_path / "projects.json"
    manager = ConfigManager(str(config_file))

    project = Project("New Project", "/new/path")
    manager.add_project(project)

    assert manager.project_count() == 1
    assert manager.get_project(0).name == "New Project"

    # Verify file updated
    content = json.loads(config_file.read_text(encoding='utf-8'))
    assert len(content["projects"]) == 1
    assert content["projects"][0]["name"] == "New Project"


def test_update_project(qapp, tmp_path):
    config_file = tmp_path / "projects.json"
    manager = ConfigManager(str(config_file))

    project = Project("Proj", "/path")
    manager.add_project(project)

    updated_project = Project("Updated Proj", "/updated/path")
    manager.update_project(0, updated_project)

    assert manager.get_project(0).name == "Updated Proj"

    content = json.loads(config_file.read_text(encoding='utf-8'))
    assert content["projects"][0]["name"] == "Updated Proj"


def test_remove_project(qapp, tmp_path):
    config_file = tmp_path / "projects.json"
    manager = ConfigManager(str(config_file))

    project = Project("Proj", "/path")
    manager.add_project(project)

    assert manager.project_count() == 1
    manager.remove_project(0)
    assert manager.project_count() == 0

    content = json.loads(config_file.read_text(encoding='utf-8'))
    assert len(content["projects"]) == 0


def test_corrupt_json_recovery(qapp, tmp_path):
    config_file = tmp_path / "projects.json"
    config_file.write_text("{corrupt json", encoding='utf-8')

    manager = ConfigManager(str(config_file))
    assert manager.project_count() == 0

    # Check backup created (.bak)
    backup_file = tmp_path / "projects.json.bak"
    assert backup_file.exists()

    # Check new file is valid
    content = json.loads(config_file.read_text(encoding='utf-8'))
    assert content["projects"] == []


def test_get_project_invalid_index(qapp, tmp_path):
    config_file = tmp_path / "projects.json"
    manager = ConfigManager(str(config_file))
    assert manager.get_project(0) is None
    assert manager.get_project(-1) is None


def test_project_count(qapp, tmp_path):
    config_file = tmp_path / "projects.json"
    manager = ConfigManager(str(config_file))
    assert manager.project_count() == 0
    manager.add_project(Project("P1", "/p1"))
    manager.add_project(Project("P2", "/p2"))
    assert manager.project_count() == 2


def test_get_next_available_port(qapp, tmp_path):
    config_file = tmp_path / "projects.json"
    manager = ConfigManager(str(config_file))
    p1 = Project("P1", "/p1", server=ServerConfig(enabled=True, port=5173))
    manager.add_project(p1)
    
    next_port = manager.get_next_available_port(base_port=5173)
    assert next_port >= 5174


def test_auto_assign_unique_ports(qapp, tmp_path):
    config_file = tmp_path / "projects.json"
    manager = ConfigManager(str(config_file))
    
    base = 59200
    p1 = Project("P1", "/p1", server=ServerConfig(enabled=True, port=base, url=f"http://localhost:{base}"))
    p2 = Project("P2", "/p2", server=ServerConfig(enabled=True, port=base, url=f"http://localhost:{base}"))
    p3 = Project("P3", "/p3", server=ServerConfig(enabled=True, port=base, url=f"http://localhost:{base}"))
    
    manager.add_project(p1)
    manager.add_project(p2)
    manager.add_project(p3)
    
    changed = manager.auto_assign_unique_ports(start_port=base)
    assert changed == 2
    
    ports = [p.server.port for p in manager.get_projects()]
    assert len(set(ports)) == 3
    assert ports[0] == base
    assert ports[1] == base + 1
    assert ports[2] == base + 2
    assert manager.get_project(1).server.url == f"http://localhost:{base + 1}"
    assert manager.get_project(2).server.url == f"http://localhost:{base + 2}"


def test_toggle_pin_project(qapp, tmp_path):
    config_file = tmp_path / "projects.json"
    manager = ConfigManager(str(config_file))
    p1 = Project("P1", "/p1", pinned=False)
    manager.add_project(p1)

    assert manager.get_project(0).pinned is False
    manager.toggle_pin_project(0)
    assert manager.get_project(0).pinned is True

    manager.toggle_pin_project(0)
    assert manager.get_project(0).pinned is False
