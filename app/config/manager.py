# -*- coding: utf-8 -*-
import json
import os
import shutil
from typing import Optional, List
from PySide6.QtCore import QObject, Signal
from app.models.project import Project
from app.utils.ports import is_port_open


class ConfigManager(QObject):
    projects_changed = Signal()
    config_error = Signal(str)

    def __init__(self, config_path: str, parent=None):
        super().__init__(parent)
        self._config_path = config_path
        self._projects: list[Project] = []
        self.load()

    def load(self):
        """Load projects from the config file."""
        if not os.path.exists(self._config_path):
            self._projects = []
            self.save()
            self.projects_changed.emit()
            return

        try:
            with open(self._config_path, 'r', encoding='utf-8') as f:
                data = json.load(f)
            
            projects_data = data.get('projects', [])
            self._projects = [Project.from_dict(p) for p in projects_data]
            self.projects_changed.emit()
            
        except json.JSONDecodeError as e:
            backup_path = self._config_path + '.bak'
            try:
                shutil.copy2(self._config_path, backup_path)
            except IOError:
                pass # Fail silently on backup if it can't be created
                
            self._projects = []
            self.save()
            self.config_error.emit(f"Config file corrupted. Backed up to {backup_path} and created a new empty config.")
            self.projects_changed.emit()
            
        except Exception as e:
            self.config_error.emit(f"Failed to load config: {str(e)}")

    def save(self):
        """Save current projects to the config file."""
        try:
            # Ensure directory exists
            config_dir = os.path.dirname(self._config_path)
            if config_dir:
                os.makedirs(config_dir, exist_ok=True)
            
            data = {
                'projects': [p.to_dict() for p in self._projects]
            }
            with open(self._config_path, 'w', encoding='utf-8') as f:
                json.dump(data, f, indent=2)
        except Exception as e:
            self.config_error.emit(f"Failed to save config: {str(e)}")

    def get_projects(self) -> list[Project]:
        """Return a copy of the projects list."""
        return list(self._projects)

    def get_project(self, index: int) -> Optional[Project]:
        """Return a project by its index, or None if invalid."""
        if 0 <= index < len(self._projects):
            return self._projects[index]
        return None

    def add_project(self, project: Project):
        """Add a new project."""
        self._projects.append(project)
        self.save()
        self.projects_changed.emit()

    def update_project(self, index: int, project: Project):
        """Update an existing project at the given index."""
        if 0 <= index < len(self._projects):
            self._projects[index] = project
            self.save()
            self.projects_changed.emit()

    def remove_project(self, index: int):
        """Remove a project from the list by its index."""
        if 0 <= index < len(self._projects):
            self._projects.pop(index)
            self.save()
            self.projects_changed.emit()

    def toggle_pin_project(self, index: int):
        """Toggle the pinned state of a project at the given index."""
        if 0 <= index < len(self._projects):
            self._projects[index].pinned = not self._projects[index].pinned
            self.save()
            self.projects_changed.emit()

    def project_count(self) -> int:
        """Return the number of projects."""
        return len(self._projects)

    def get_configured_ports(self) -> List[int]:
        """Return a list of all ports currently configured across projects."""
        return [p.server.port for p in self._projects if p.server and p.server.port > 0]

    def get_next_available_port(self, base_port: int = 5173) -> int:
        """
        Finds the next free port starting from base_port that is neither
        configured in an existing project nor currently open on localhost.
        """
        configured = set(self.get_configured_ports())
        port = max(1024, base_port)
        while port <= 65535:
            if port not in configured and not is_port_open("127.0.0.1", port):
                return port
            port += 1
        return base_port

    def auto_assign_unique_ports(self, start_port: int = 5173) -> int:
        """
        Reassign unique sequential ports to all projects that share ports.
        Returns the number of projects updated.
        """
        used_ports = set()
        changed_count = 0
        current_port = start_port

        for p in self._projects:
            if not p.server.enabled:
                continue
            # If port is already claimed or <= 0
            if p.server.port in used_ports or p.server.port <= 0:
                while current_port in used_ports or is_port_open("127.0.0.1", current_port):
                    current_port += 1
                p.server.port = current_port
                p.server.url = f"http://localhost:{current_port}"
                used_ports.add(current_port)
                current_port += 1
                changed_count += 1
            else:
                used_ports.add(p.server.port)

        if changed_count > 0:
            self.save()
            self.projects_changed.emit()
            
        return changed_count
