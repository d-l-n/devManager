# -*- coding: utf-8 -*-
import socket
import pytest
from app.utils.ports import is_port_open, build_server_command


def test_port_open():
    server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server.bind(('127.0.0.1', 0))
    server.listen(1)
    port = server.getsockname()[1]
    
    assert is_port_open('127.0.0.1', port) == True
    server.close()


def test_port_closed():
    server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server.bind(('127.0.0.1', 0))
    port = server.getsockname()[1]
    server.close()
    
    assert is_port_open('127.0.0.1', port) == False


def test_port_zero():
    assert is_port_open('127.0.0.1', 0) == False


def test_build_server_command_npm():
    assert build_server_command("npm run dev", 5174) == "npm run dev -- --port 5174"
    assert build_server_command("npm start", 5174) == "npm start -- --port 5174"


def test_build_server_command_pnpm():
    assert build_server_command("pnpm dev", 5174) == "pnpm dev --port 5174"
    assert build_server_command("pnpm run dev", 5174) == "pnpm run dev --port 5174"


def test_build_server_command_yarn():
    assert build_server_command("yarn dev", 5174) == "yarn dev --port 5174"


def test_build_server_command_bun():
    assert build_server_command("bun dev", 5174) == "bun dev --port 5174"
    assert build_server_command("bun run dev", 5174) == "bun run dev --port 5174"


def test_build_server_command_vite():
    assert build_server_command("vite", 5174) == "vite --port 5174"
    assert build_server_command("npx vite", 5174) == "npx vite --port 5174"


def test_build_server_command_next():
    assert build_server_command("next dev", 3001) == "next dev -p 3001"


def test_build_server_command_already_has_port():
    # If user already wrote --port or -p, do not duplicate
    assert build_server_command("npm run dev -- --port 5173", 5174) == "npm run dev -- --port 5173"
    assert build_server_command("pnpm dev --port 5173", 5174) == "pnpm dev --port 5173"
    assert build_server_command("next dev -p 3000", 3001) == "next dev -p 3000"
