import os
import json
import re
from typing import Dict, Any, Optional


def detect_project_config(project_path: str, existing_ports: Optional[list] = None) -> Dict[str, Any]:
    """
    Inspects a project folder and automatically detects:
    - Project Name
    - Package manager (npm, pnpm, yarn, bun)
    - Server command
    - Default Port & URL (avoiding ports in existing_ports)
    - Playwright configuration
    """
    result: Dict[str, Any] = {
        "name": "",
        "server_command": "npm run dev",
        "port": 5173,
        "url": "http://localhost:5173",
        "playwright_enabled": False,
    }

    if not project_path or not os.path.isdir(project_path):
        return result

    # Default name from folder basename
    folder_name = os.path.basename(os.path.abspath(project_path))
    result["name"] = folder_name.replace("-", " ").replace("_", " ").title()

    # Detect package manager
    pkg_mgr = "npm"
    if os.path.exists(os.path.join(project_path, "pnpm-lock.yaml")):
        pkg_mgr = "pnpm"
    elif os.path.exists(os.path.join(project_path, "yarn.lock")):
        pkg_mgr = "yarn"
    elif os.path.exists(os.path.join(project_path, "bun.lockb")):
        pkg_mgr = "bun"

    # 1. Inspect package.json (Node.js ecosystem)
    pkg_json_path = os.path.join(project_path, "package.json")
    if os.path.exists(pkg_json_path):
        try:
            with open(pkg_json_path, "r", encoding="utf-8") as f:
                pkg_data = json.load(f)

            if pkg_data.get("name"):
                result["name"] = pkg_data["name"].replace("-", " ").replace("_", " ").title()

            scripts = pkg_data.get("scripts", {})
            deps = {**pkg_data.get("dependencies", {}), **pkg_data.get("devDependencies", {})}

            # Choose dev script
            if "dev" in scripts:
                result["server_command"] = f"{pkg_mgr} run dev" if pkg_mgr != "pnpm" else "pnpm dev"
            elif "start" in scripts:
                result["server_command"] = f"{pkg_mgr} start"
            elif "serve" in scripts:
                result["server_command"] = f"{pkg_mgr} run serve"

            # Check framework signatures for default ports
            if "astro" in deps:
                result["port"] = 4321
            elif "next" in deps:
                result["port"] = 3000
            elif "nuxt" in deps or "@nuxt/core" in deps:
                result["port"] = 3000
            elif "@angular/core" in deps or "@angular/cli" in deps:
                result["port"] = 4200
            elif "vite" in deps or "@sveltejs/kit" in deps:
                result["port"] = 5173
            elif "react-scripts" in deps or "@remix-run/react" in deps:
                result["port"] = 3000
            elif "vue" in deps and "vite" not in deps:
                result["port"] = 8080

            # Check Playwright in dependencies
            if "@playwright/test" in deps or "playwright" in deps:
                result["playwright_enabled"] = True

        except Exception:
            pass

    # 2. Check Python frameworks if no package.json or if python server files exist
    if os.path.exists(os.path.join(project_path, "manage.py")):
        result["server_command"] = "python manage.py runserver"
        result["port"] = 8000
    elif os.path.exists(os.path.join(project_path, "main.py")) and not os.path.exists(pkg_json_path):
        result["server_command"] = "uvicorn main:app --reload"
        result["port"] = 8000

    # 3. Check for .env files that override port
    for env_file in [".env", ".env.local", ".env.development"]:
        env_path = os.path.join(project_path, env_file)
        if os.path.exists(env_path):
            try:
                with open(env_path, "r", encoding="utf-8") as f:
                    for line in f:
                        m = re.match(r"^(?:PORT|VITE_PORT|SERVER_PORT)\s*=\s*(\d+)", line.strip(), re.IGNORECASE)
                        if m:
                            result["port"] = int(m.group(1))
                            break
            except Exception:
                pass

    # 4. Check for Playwright configuration files
    for pw_config in ["playwright.config.ts", "playwright.config.js", "playwright.config.mjs"]:
        if os.path.exists(os.path.join(project_path, pw_config)):
            result["playwright_enabled"] = True
            break

    # Avoid port collisions with existing configured projects
    if existing_ports:
        used_set = set(existing_ports)
        port = result["port"]
        while port in used_set:
            port += 1
        result["port"] = port

    # Build URL from resolved port
    result["url"] = f"http://localhost:{result['port']}"
    return result


def extract_port_from_log(log_line: str) -> Optional[int]:
    """
    Parses a log line and extracts the port if a localhost/127.0.0.1/network URL is detected.
    Matches lines like:
    - 'Local:   http://localhost:5174/'
    - 'ready on http://127.0.0.1:3000'
    - 'Network: http://192.168.1.5:8080/'
    """
    if not log_line:
        return None

    # Matches http(s)://localhost:PORT or http(s)://127.0.0.1:PORT or http(s)://0.0.0.0:PORT or http(s)://[::1]:PORT
    pattern = r"https?:\/\/(?:localhost|127\.0\.0\.1|0\.0\.0\.0|\[::1\]|\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):(\d{2,5})"
    match = re.search(pattern, log_line, re.IGNORECASE)
    if match:
        try:
            port = int(match.group(1))
            if 0 < port <= 65535:
                return port
        except ValueError:
            pass
    return None


def detect_package_manager(project_path: str) -> str:
    """Detects the package manager (pnpm, yarn, bun, npm) used in a project folder."""
    if not project_path or not os.path.isdir(project_path):
        return "npm"
    if os.path.exists(os.path.join(project_path, "pnpm-lock.yaml")):
        return "pnpm"
    if os.path.exists(os.path.join(project_path, "yarn.lock")):
        return "yarn"
    if os.path.exists(os.path.join(project_path, "bun.lockb")):
        return "bun"
    return "npm"


def get_project_scripts(project_path: str) -> Dict[str, str]:
    """
    Extracts defined scripts from package.json or pyproject.toml in a project directory.
    Returns a dict mapping script_name -> full command to execute (e.g., 'build' -> 'pnpm run build').
    """
    scripts_map: Dict[str, str] = {}
    if not project_path or not os.path.isdir(project_path):
        return scripts_map

    pkg_mgr = detect_package_manager(project_path)
    pkg_json_path = os.path.join(project_path, "package.json")

    if os.path.exists(pkg_json_path):
        try:
            with open(pkg_json_path, "r", encoding="utf-8") as f:
                data = json.load(f)
            raw_scripts = data.get("scripts", {})
            for name, cmd in raw_scripts.items():
                if pkg_mgr == "npm":
                    run_cmd = f"npm run {name}" if name not in ("start", "test") else f"npm {name}"
                elif pkg_mgr == "pnpm":
                    run_cmd = f"pnpm {name}"
                elif pkg_mgr == "yarn":
                    run_cmd = f"yarn {name}"
                elif pkg_mgr == "bun":
                    run_cmd = f"bun run {name}"
                else:
                    run_cmd = f"{pkg_mgr} run {name}"
                scripts_map[name] = run_cmd
        except Exception:
            pass

    return scripts_map
