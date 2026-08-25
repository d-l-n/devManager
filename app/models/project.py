from enum import Enum, auto
from dataclasses import dataclass, field, asdict
from typing import Optional

class ServerState(Enum):
    STOPPED = auto()
    STARTING = auto()
    RUNNING = auto()
    STOPPING = auto()
    ERROR = auto()

class PlaywrightState(Enum):
    IDLE = auto()
    STARTING = auto()
    RUNNING = auto()
    PASSED = auto()
    FAILED = auto()
    ERROR = auto()

@dataclass
class ServerConfig:
    enabled: bool = True
    command: str = 'npm run dev'
    port: int = 5173
    url: str = 'http://localhost:5173'
    startup_timeout: int = 15000

    def to_dict(self) -> dict:
        return asdict(self)

    @classmethod
    def from_dict(cls, data: dict) -> 'ServerConfig':
        return cls(
            enabled=data.get('enabled', True),
            command=data.get('command', 'npm run dev'),
            port=data.get('port', 5173),
            url=data.get('url', 'http://localhost:5173'),
            startup_timeout=data.get('startup_timeout', 15000)
        )

@dataclass
class PlaywrightConfig:
    enabled: bool = True
    command: str = 'npx playwright test'
    ui_command: str = 'npx playwright test --ui'
    debug_command: str = 'npx playwright test --debug'
    report_command: str = 'npx playwright show-report'

    def to_dict(self) -> dict:
        return asdict(self)

    @classmethod
    def from_dict(cls, data: dict) -> 'PlaywrightConfig':
        return cls(
            enabled=data.get('enabled', True),
            command=data.get('command', 'npx playwright test'),
            ui_command=data.get('ui_command', 'npx playwright test --ui'),
            debug_command=data.get('debug_command', 'npx playwright test --debug'),
            report_command=data.get('report_command', 'npx playwright show-report')
        )

@dataclass
class Project:
    name: str
    path: str
    server: ServerConfig = field(default_factory=ServerConfig)
    playwright: PlaywrightConfig = field(default_factory=PlaywrightConfig)
    pinned: bool = False

    def to_dict(self) -> dict:
        return {
            'name': self.name,
            'path': self.path,
            'server': self.server.to_dict(),
            'playwright': self.playwright.to_dict(),
            'pinned': self.pinned
        }

    @classmethod
    def from_dict(cls, data: dict) -> 'Project':
        return cls(
            name=data.get('name', ''),
            path=data.get('path', ''),
            server=ServerConfig.from_dict(data.get('server', {})),
            playwright=PlaywrightConfig.from_dict(data.get('playwright', {})),
            pinned=data.get('pinned', False)
        )

    def validate(self) -> list[str]:
        errors = []
        if not self.name or not self.name.strip():
            errors.append('Project name cannot be empty')
        if not self.path or not self.path.strip():
            errors.append('Project path cannot be empty')
        return errors
