# -*- coding: utf-8 -*-
# app/utils/evidence.py
"""Filesystem scanner for Playwright test artifacts (screenshots, videos, traces)."""
import os
from dataclasses import dataclass
from typing import List, Optional

IMAGE_EXTS = {".png", ".jpg", ".jpeg"}
VIDEO_EXTS = {".webm"}
TRACE_EXTS = {".zip"}
SKIP_DIRS = {"node_modules", ".git", "venv", ".venv", "__pycache__"}


@dataclass
class EvidenceFile:
    path: str
    rel_path: str
    kind: str          # "image" | "video" | "trace"
    mtime: float
    test_dir: str      # carpeta contenedora inmediata


def _classify(ext: str) -> Optional[str]:
    if ext in IMAGE_EXTS:
        return "image"
    if ext in VIDEO_EXTS:
        return "video"
    if ext in TRACE_EXTS:
        return "trace"
    return None


def scan_evidence(project_path: str, max_items: int = 200) -> List[EvidenceFile]:
    results_root = os.path.join(project_path, "test-results")
    found: List[EvidenceFile] = []
    if not os.path.isdir(results_root):
        return found

    for dirpath, dirnames, filenames in os.walk(results_root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        for fname in filenames:
            ext = os.path.splitext(fname)[1].lower()
            kind = _classify(ext)
            if not kind:
                continue
            full = os.path.join(dirpath, fname)
            try:
                mtime = os.path.getmtime(full)
            except OSError:
                continue
            found.append(EvidenceFile(
                path=full,
                rel_path=os.path.relpath(full, project_path),
                kind=kind,
                mtime=mtime,
                test_dir=dirpath,
            ))

    found.sort(key=lambda f: f.mtime, reverse=True)
    return found[:max_items]


def find_html_report(project_path: str) -> Optional[str]:
    candidate = os.path.join(project_path, "playwright-report", "index.html")
    return candidate if os.path.isfile(candidate) else None
