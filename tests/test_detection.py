# -*- coding: utf-8 -*-
import os
import json
import pytest
from app.utils.detection import detect_project_config, extract_port_from_log


def test_extract_port_from_log():
    assert extract_port_from_log("Local: http://localhost:5173/") == 5173
    assert extract_port_from_log("  ➜  Local:   http://localhost:5174/") == 5174
    assert extract_port_from_log("ready in 250ms on http://127.0.0.1:3000") == 3000
    assert extract_port_from_log("Serving at http://0.0.0.0:8000") == 8000
    assert extract_port_from_log("No port in this line") is None
    assert extract_port_from_log("") is None


def test_detect_vite_project(tmp_path):
    pkg_json = tmp_path / "package.json"
    pkg_json.write_text(json.dumps({
        "name": "my-vite-app",
        "scripts": {"dev": "vite"},
        "devDependencies": {"vite": "^5.0.0", "@playwright/test": "^1.40.0"}
    }), encoding="utf-8")

    config = detect_project_config(str(tmp_path))
    assert config["name"] == "My Vite App"
    assert config["port"] == 5173
    assert config["server_command"] == "npm run dev"
    assert config["playwright_enabled"] is True


def test_detect_nextjs_project(tmp_path):
    pkg_json = tmp_path / "package.json"
    pkg_json.write_text(json.dumps({
        "name": "nextjs-blog",
        "scripts": {"dev": "next dev"},
        "dependencies": {"next": "14.0.0"}
    }), encoding="utf-8")

    config = detect_project_config(str(tmp_path))
    assert config["name"] == "Nextjs Blog"
    assert config["port"] == 3000
    assert config["server_command"] == "npm run dev"
    assert config["playwright_enabled"] is False


def test_detect_env_port_override(tmp_path):
    pkg_json = tmp_path / "package.json"
    pkg_json.write_text(json.dumps({
        "name": "custom-app",
        "scripts": {"dev": "vite"},
        "devDependencies": {"vite": "^5.0.0"}
    }), encoding="utf-8")

    env_file = tmp_path / ".env"
    env_file.write_text("PORT=8080\n", encoding="utf-8")

    config = detect_project_config(str(tmp_path))
    assert config["port"] == 8080
    assert config["url"] == "http://localhost:8080"


def test_detect_avoid_existing_ports(tmp_path):
    pkg_json = tmp_path / "package.json"
    pkg_json.write_text(json.dumps({
        "name": "app-2",
        "scripts": {"dev": "vite"},
        "devDependencies": {"vite": "^5.0.0"}
    }), encoding="utf-8")

    # Pass 5173 and 5174 as already existing ports
    config = detect_project_config(str(tmp_path), existing_ports=[5173, 5174])
    assert config["port"] == 5175
    assert config["url"] == "http://localhost:5175"

