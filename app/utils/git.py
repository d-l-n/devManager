import os
import subprocess
from typing import Dict, Any, Optional


def is_git_repo(project_path: str) -> bool:
    """Checks if a directory contains a .git folder or is inside a Git working tree."""
    if not project_path or not os.path.isdir(project_path):
        return False
    git_dir = os.path.join(project_path, ".git")
    return os.path.exists(git_dir)


def get_git_info(project_path: str) -> Dict[str, Any]:
    """
    Retrieves Git branch and dirty status for a project directory.
    Returns:
        {
            "is_repo": bool,
            "branch": str,
            "is_dirty": bool,
            "error": Optional[str]
        }
    """
    result: Dict[str, Any] = {
        "is_repo": False,
        "branch": "",
        "is_dirty": False,
        "error": None
    }

    if not is_git_repo(project_path):
        return result

    result["is_repo"] = True

    # 1. Fast read from .git/HEAD
    head_file = os.path.join(project_path, ".git", "HEAD")
    if os.path.isfile(head_file):
        try:
            with open(head_file, "r", encoding="utf-8") as f:
                head_content = f.read().strip()
            if head_content.startswith("ref: refs/heads/"):
                result["branch"] = head_content[len("ref: refs/heads/"):]
            elif len(head_content) >= 7:
                result["branch"] = head_content[:7]  # Detached HEAD SHA
        except Exception:
            pass

    # 2. Check branch & dirty status via git CLI if available
    try:
        startupinfo = None
        if os.name == 'nt':
            startupinfo = subprocess.STARTUPINFO()
            startupinfo.dwFlags |= subprocess.STARTF_USESHOWWINDOW
            startupinfo.wShowWindow = subprocess.SW_HIDE

        # If branch wasn't determined by .git/HEAD (or if .git is a worktree file)
        if not result["branch"]:
            branch_proc = subprocess.run(
                ["git", "rev-parse", "--abbrev-ref", "HEAD"],
                cwd=project_path,
                capture_output=True,
                text=True,
                timeout=1.5,
                startupinfo=startupinfo
            )
            if branch_proc.returncode == 0:
                result["branch"] = branch_proc.stdout.strip()

        # Check dirty status (uncommitted/staged/untracked changes)
        status_proc = subprocess.run(
            ["git", "status", "--porcelain"],
            cwd=project_path,
            capture_output=True,
            text=True,
            timeout=2.0,
            startupinfo=startupinfo
        )
        if status_proc.returncode == 0:
            result["is_dirty"] = bool(status_proc.stdout.strip())

    except Exception as e:
        result["error"] = str(e)

    return result


def run_git(project_path: str, args: list, timeout: float = 10.0):
    """Runs a git command hidden. Returns (returncode, stdout, stderr)."""
    startupinfo = None
    if os.name == 'nt':
        startupinfo = subprocess.STARTUPINFO()
        startupinfo.dwFlags |= subprocess.STARTF_USESHOWWINDOW
        startupinfo.wShowWindow = subprocess.SW_HIDE
    try:
        proc = subprocess.run(
            ["git", *args],
            cwd=project_path,
            capture_output=True,
            text=True,
            timeout=timeout,
            startupinfo=startupinfo,
        )
        return proc.returncode, proc.stdout, proc.stderr
    except FileNotFoundError:
        return -1, "", "git executable not found in PATH"
    except subprocess.TimeoutExpired:
        return -1, "", f"git command timed out after {timeout}s"
    except Exception as e:
        return -1, "", str(e)


def get_git_status_full(project_path: str) -> Dict[str, Any]:
    """Extends get_git_info with ahead/behind counts and last commit metadata."""
    result = get_git_info(project_path)
    result.update({
        "ahead": 0,
        "behind": 0,
        "has_upstream": False,
        "last_commit": None,
    })
    if not result["is_repo"]:
        return result

    code, out, _err = run_git(
        project_path,
        ["rev-list", "--left-right", "--count", "@{upstream}...HEAD"],
        timeout=5.0,
    )
    if code == 0 and "\t" in out:
        left, right = out.strip().split("\t")
        result["has_upstream"] = True
        result["behind"] = int(left) if left.isdigit() else 0
        result["ahead"] = int(right) if right.isdigit() else 0

    code, out, _err = run_git(
        project_path,
        ["log", "-1", "--pretty=format:%h|%s|%cr"],
        timeout=5.0,
    )
    if code == 0 and out.strip():
        parts = out.split("|", 2)
        if len(parts) == 3:
            result["last_commit"] = {
                "hash": parts[0].strip(),
                "subject": parts[1].strip(),
                "date_rel": parts[2].strip(),
            }

    return result
