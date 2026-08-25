import pytest
from app.models.project import (
    Project, ServerConfig, PlaywrightConfig,
    ServerState, PlaywrightState
)


def test_server_state_values():
    assert hasattr(ServerState, "STOPPED")
    assert hasattr(ServerState, "STARTING")
    assert hasattr(ServerState, "RUNNING")
    assert hasattr(ServerState, "STOPPING")
    assert hasattr(ServerState, "ERROR")


def test_playwright_state_values():
    assert hasattr(PlaywrightState, "IDLE")
    assert hasattr(PlaywrightState, "STARTING")
    assert hasattr(PlaywrightState, "RUNNING")
    assert hasattr(PlaywrightState, "PASSED")
    assert hasattr(PlaywrightState, "FAILED")
    assert hasattr(PlaywrightState, "ERROR")


def test_server_config_defaults():
    config = ServerConfig()
    assert config.enabled is True
    assert config.command == "npm run dev"
    assert config.port == 5173
    assert config.url == "http://localhost:5173"
    assert config.startup_timeout == 15000


def test_playwright_config_defaults():
    config = PlaywrightConfig()
    assert config.enabled is True
    assert config.command == "npx playwright test"
    assert config.ui_command == "npx playwright test --ui"
    assert config.debug_command == "npx playwright test --debug"
    assert config.report_command == "npx playwright show-report"


def test_server_config_to_dict():
    config = ServerConfig(command="yarn dev", port=8080)
    data = config.to_dict()
    assert data["command"] == "yarn dev"
    assert data["port"] == 8080


def test_server_config_from_dict():
    data = {"command": "npm start", "port": 5000}
    config = ServerConfig.from_dict(data)
    assert config.command == "npm start"
    assert config.port == 5000


def test_server_config_from_dict_missing_keys():
    config = ServerConfig.from_dict({})
    assert config.command == "npm run dev"
    assert config.port == 5173
    assert config.enabled is True


def test_playwright_config_to_dict():
    config = PlaywrightConfig(
        command="pytest",
        ui_command="pytest --ui",
        debug_command="pytest --debug",
        report_command="pytest show-report"
    )
    data = config.to_dict()
    assert data["command"] == "pytest"
    assert data["ui_command"] == "pytest --ui"
    assert data["debug_command"] == "pytest --debug"
    assert data["report_command"] == "pytest show-report"


def test_playwright_config_from_dict():
    data = {
        "command": "test",
        "ui_command": "ui",
        "debug_command": "debug",
        "report_command": "report"
    }
    config = PlaywrightConfig.from_dict(data)
    assert config.command == "test"
    assert config.ui_command == "ui"
    assert config.debug_command == "debug"
    assert config.report_command == "report"


def test_project_creation():
    server = ServerConfig(command="npm start", port=4000)
    playwright = PlaywrightConfig(command="npm test")
    project = Project(name="Test", path="/path/to/test", server=server, playwright=playwright)
    assert project.name == "Test"
    assert project.path == "/path/to/test"
    assert project.server.port == 4000
    assert project.playwright.command == "npm test"


def test_project_to_dict():
    project = Project(name="Proj", path="/path")
    data = project.to_dict()
    assert data["name"] == "Proj"
    assert data["path"] == "/path"
    assert "server" in data
    assert "playwright" in data


def test_project_from_dict():
    data = {
        "name": "App",
        "path": "C:/app",
        "server": {"command": "serve", "port": 80},
        "playwright": {"command": "run test"}
    }
    project = Project.from_dict(data)
    assert project.name == "App"
    assert project.path == "C:/app"
    assert project.server.command == "serve"
    assert project.server.port == 80
    assert project.playwright.command == "run test"


def test_project_from_dict_minimal():
    data = {"name": "Min", "path": "/min"}
    project = Project.from_dict(data)
    assert project.name == "Min"
    assert project.path == "/min"
    assert project.server.command == "npm run dev"
    assert project.playwright.command == "npx playwright test"


def test_project_validate_valid():
    project = Project(name="Valid", path="/valid")
    errors = project.validate()
    assert len(errors) == 0


def test_project_validate_empty_name():
    project = Project(name="", path="/path")
    errors = project.validate()
    assert len(errors) > 0
    assert any("name" in e.lower() for e in errors)


def test_project_validate_empty_path():
    project = Project(name="Name", path="")
    errors = project.validate()
    assert len(errors) > 0
    assert any("path" in e.lower() for e in errors)


def test_project_pinned_defaults_and_serialization():
    project = Project(name="Pinned App", path="/pinned")
    assert project.pinned is False
    data = project.to_dict()
    assert data["pinned"] is False

    project.pinned = True
    data = project.to_dict()
    assert data["pinned"] is True

    restored = Project.from_dict(data)
    assert restored.pinned is True
