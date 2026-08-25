# -*- coding: utf-8 -*-
# tests/test_evidence.py
import os
import time
from app.utils.evidence import scan_evidence, find_html_report, EvidenceFile


def _make_file(root, rel, mtime_offset=0):
    p = root / rel
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_bytes(b"x")
    ts = time.time() - mtime_offset
    os.utime(p, (ts, ts))
    return p


def test_scan_orders_by_mtime_desc(tmp_path):
    _make_file(tmp_path, "test-results/old/a.png", mtime_offset=100)
    _make_file(tmp_path, "test-results/new/b.png", mtime_offset=10)
    _make_file(tmp_path, "test-results/newest/c.webm", mtime_offset=0)

    files = scan_evidence(str(tmp_path))
    names = [f.rel_path.replace("\\", "/") for f in files]
    assert names.index("test-results/newest/c.webm") < names.index("test-results/new/b.png")
    assert names.index("test-results/new/b.png") < names.index("test-results/old/a.png")


def test_scan_kinds(tmp_path):
    _make_file(tmp_path, "test-results/a.png")
    _make_file(tmp_path, "test-results/b.jpeg")
    _make_file(tmp_path, "test-results/sub/c.webm")
    _make_file(tmp_path, "test-results/d.zip")

    files = scan_evidence(str(tmp_path))
    kinds = {f.rel_path.replace("\\", "/"): f.kind for f in files}
    assert kinds["test-results/a.png"] == "image"
    assert kinds["test-results/b.jpeg"] == "image"
    assert kinds["test-results/sub/c.webm"] == "video"
    assert kinds["test-results/d.zip"] == "trace"


def test_scan_ignores_node_modules_and_git(tmp_path):
    _make_file(tmp_path, "node_modules/pkg/shot.png")
    _make_file(tmp_path, ".git/hooks/hook.png")
    _make_file(tmp_path, "test-results/real.png")

    files = scan_evidence(str(tmp_path))
    rels = [f.rel_path.replace("\\", "/") for f in files]
    assert rels == ["test-results/real.png"]


def test_scan_respects_max_items(tmp_path):
    for i in range(10):
        _make_file(tmp_path, f"test-results/s{i}.png", mtime_offset=i)
    files = scan_evidence(str(tmp_path), max_items=5)
    assert len(files) == 5
    # Los 5 más recientes
    assert all(f.rel_path.endswith(f"s{i}.png") for i, f in enumerate(files))


def test_scan_empty_project(tmp_path):
    assert scan_evidence(str(tmp_path)) == []
    assert find_html_report(str(tmp_path)) is None


def test_find_html_report(tmp_path):
    report = _make_file(tmp_path, "playwright-report/index.html")
    assert find_html_report(str(tmp_path)) == str(report)
