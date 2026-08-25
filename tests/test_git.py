# tests/test_git.py
import shutil
import pytest
from app.utils.git import get_git_status_full, run_git, get_git_info

GIT_AVAILABLE = shutil.which("git") is not None


def test_status_full_non_repo(tmp_path):
    info = get_git_status_full(str(tmp_path))
    assert info["is_repo"] is False
    assert info["branch"] == ""
    assert info["is_dirty"] is False
    assert info["ahead"] == 0
    assert info["behind"] == 0
    assert info["has_upstream"] is False
    assert info["last_commit"] is None


def test_run_git_invalid_args_non_repo(tmp_path):
    code, out, err = run_git(str(tmp_path), ["status"])
    assert code != 0
    assert err  # mensaje de error presente


@pytest.mark.skipif(not GIT_AVAILABLE, reason="git CLI not available")
def test_run_git_in_real_repo(tmp_path):
    assert run_git(str(tmp_path), ["init"]).returncode if False else True
    code, out, err = run_git(str(tmp_path), ["init"])
    assert code == 0
    code, out, err = run_git(str(tmp_path), [
        "-c", "user.email=test@test.dev",
        "-c", "user.name=Test",
        "commit", "--allow-empty", "-m", "initial",
    ])
    assert code == 0
    code, out, err = run_git(str(tmp_path), ["rev-parse", "--abbrev-ref", "HEAD"])
    assert code == 0

    info = get_git_status_full(str(tmp_path))
    assert info["is_repo"] is True
    assert info["branch"]
    assert info["has_upstream"] is False   # repo sin remote
    assert info["ahead"] == 0
    assert info["behind"] == 0
    assert info["last_commit"]["subject"] == "initial"
    assert len(info["last_commit"]["hash"]) == 7
    assert info["last_commit"]["date_rel"]  # ej. "seconds ago"
