# -*- coding: utf-8 -*-
# app/ui/widgets/evidence_panel.py
import os
from typing import List, Optional
from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QPushButton, QListWidget,
    QListWidgetItem, QSplitter, QMenu, QFrame
)
from PySide6.QtCore import Signal, Qt, QSize, QTimer, QUrl
from PySide6.QtGui import QPixmap, QAction, QDesktopServices
from app.models.project import Project
from app.utils.evidence import scan_evidence, find_html_report, EvidenceFile
from app.ui.icons import get_icon
from app.ui.theme import ThemeManager, ThemeMode, ui_font

THUMB_SIZE = 180
BATCH_SIZE = 20


class EvidencePanel(QWidget):
    trace_open_requested = Signal(str)  # ruta absoluta del .zip

    def __init__(self, parent=None):
        super().__init__(parent)
        self._project: Optional[Project] = None
        self._files: List[EvidenceFile] = []
        self._pending_thumbs: List[tuple] = []  # (QListWidgetItem, EvidenceFile)
        self._thumb_timer = QTimer(self)
        self._thumb_timer.setInterval(30)
        self._thumb_timer.timeout.connect(self._load_thumb_batch)
        self._setup_ui()
        self.apply_theme(ThemeManager.instance().mode)

    def _setup_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(14, 14, 14, 14)
        layout.setSpacing(10)

        # Toolbar
        bar = QHBoxLayout()
        self._refresh_btn = QPushButton(" Refresh")
        self._refresh_btn.setIcon(get_icon("restart", "#ffffff"))
        self._refresh_btn.setIconSize(QSize(14, 14))
        self._refresh_btn.setFixedHeight(34)
        self._refresh_btn.clicked.connect(self.reload)
        bar.addWidget(self._refresh_btn)

        self._report_btn = QPushButton(" Open HTML Report")
        self._report_btn.setIcon(get_icon("report", "#ffffff"))
        self._report_btn.setIconSize(QSize(14, 14))
        self._report_btn.setFixedHeight(34)
        self._report_btn.clicked.connect(self._open_report)
        bar.addWidget(self._report_btn)

        bar.addStretch()
        self._count_lbl = QLabel("")
        bar.addWidget(self._count_lbl)
        layout.addLayout(bar)

        # Splitter gallery | preview
        self._splitter = QSplitter(Qt.Orientation.Horizontal)

        self._gallery = QListWidget()
        self._gallery.setViewMode(QListWidget.ViewMode.IconMode)
        self._gallery.setIconSize(QSize(THUMB_SIZE, THUMB_SIZE))
        self._gallery.setResizeMode(QListWidget.ResizeMode.Adjust)
        self._gallery.setMovement(QListWidget.Movement.Static)
        self._gallery.setSpacing(10)
        self._gallery.setContextMenuPolicy(Qt.ContextMenuPolicy.CustomContextMenu)
        self._gallery.customContextMenuRequested.connect(self._context_menu)
        self._gallery.itemDoubleClicked.connect(self._on_item_activated)
        self._splitter.addWidget(self._gallery)

        preview_container = QFrame()
        preview_layout = QVBoxLayout(preview_container)
        preview_layout.setContentsMargins(0, 0, 0, 0)
        self._preview = QLabel("Select an image to preview")
        self._preview.setAlignment(Qt.AlignmentFlag.AlignCenter)
        self._preview.setMinimumWidth(280)
        preview_layout.addWidget(self._preview)
        self._splitter.addWidget(preview_container)
        self._splitter.setStretchFactor(0, 3)
        self._splitter.setStretchFactor(1, 2)

        layout.addWidget(self._splitter, stretch=1)

        # Empty state
        self._empty_label = QLabel("No evidence found.\nRun Playwright tests to generate screenshots, videos and traces.")
        self._empty_label.setAlignment(Qt.AlignmentFlag.AlignCenter)
        layout.addWidget(self._empty_label)

    # ---------- Data loading ----------

    def set_project(self, project: Optional[Project]):
        self._project = project
        self.reload()

    def reload(self):
        self._gallery.clear()
        self._preview.setText("Select an image to preview")
        self._pending_thumbs.clear()
        self._thumb_timer.stop()
        self._files = []

        if not self._project or not os.path.isdir(self._project.path):
            self._update_visibility()
            return

        self._files = scan_evidence(self._project.path)

        image_icon = get_icon("image", "#64748b")
        film_icon = get_icon("film", "#64748b")
        archive_icon = get_icon("archive", "#f59e0b")

        for f in self._files[:BATCH_SIZE]:
            self._add_item(f, image_icon, film_icon, archive_icon, load_thumb=True)
        for f in self._files[BATCH_SIZE:]:
            self._add_item(f, image_icon, film_icon, archive_icon, load_thumb=False)

        if self._files:
            self._thumb_timer.start()

        report = find_html_report(self._project.path) if self._project else None
        self._report_path = report
        self._report_btn.setEnabled(bool(report))

        self._count_lbl.setText(f"{len(self._files)} artifact(s)")
        self._update_visibility()

    def _add_item(self, f: EvidenceFile, image_icon, film_icon, archive_icon, load_thumb: bool):
        short = f.rel_path.replace("\\", "/")
        if len(short) > 42:
            short = "…" + short[-41:]
        item = QListWidgetItem(short)
        item.setData(Qt.ItemDataRole.UserRole, f)
        item.setToolTip(f.rel_path)
        if f.kind == "image":
            if load_thumb:
                pix = QPixmap(f.path)
                if not pix.isNull():
                    item.setIcon(pix.scaled(THUMB_SIZE, THUMB_SIZE, Qt.AspectRatioMode.KeepAspectRatio, Qt.TransformationMode.SmoothTransformation))
                    self._gallery.addItem(item)
                    return
            item.setIcon(image_icon)
        elif f.kind == "video":
            item.setIcon(film_icon)
        else:
            item.setIcon(archive_icon)
        self._gallery.addItem(item)
        if f.kind == "image" and not load_thumb:
            self._pending_thumbs.append((item, f))

    def _load_thumb_batch(self):
        batch = self._pending_thumbs[:BATCH_SIZE]
        del self._pending_thumbs[:BATCH_SIZE]
        for item, f in batch:
            pix = QPixmap(f.path)
            if pix.isNull():
                continue
            item.setIcon(pix.scaled(THUMB_SIZE, THUMB_SIZE, Qt.AspectRatioMode.KeepAspectRatio, Qt.TransformationMode.SmoothTransformation))
        if not self._pending_thumbs:
            self._thumb_timer.stop()

    # ---------- Interactions ----------

    def _current_file(self) -> Optional[EvidenceFile]:
        item = self._gallery.currentItem()
        return item.data(Qt.ItemDataRole.UserRole) if item else None

    def _on_item_activated(self, item: QListWidgetItem):
        f: EvidenceFile = item.data(Qt.ItemDataRole.UserRole)
        if f.kind == "image":
            pix = QPixmap(f.path)
            if not pix.isNull():
                self._preview.setPixmap(pix.scaled(
                    self._preview.width() - 8, self._preview.height() - 8,
                    Qt.AspectRatioMode.KeepAspectRatio, Qt.TransformationMode.SmoothTransformation
                ))
        elif f.kind == "video":
            QDesktopServices.openUrl(QUrl.fromLocalFile(f.path))
        else:
            self.trace_open_requested.emit(f.path)

    def _context_menu(self, pos):
        item = self._gallery.itemAt(pos)
        if not item:
            return
        f: EvidenceFile = item.data(Qt.ItemDataRole.UserRole)
        menu = QMenu(self)
        open_act = QAction(get_icon("external-link"), "Open Externally", self)
        open_act.triggered.connect(lambda: QDesktopServices.openUrl(QUrl.fromLocalFile(f.path)))
        menu.addAction(open_act)

        folder_act = QAction(get_icon("folder"), "Open Containing Folder", self)
        folder_act.triggered.connect(
            lambda: QDesktopServices.openUrl(QUrl.fromLocalFile(f.test_dir))
        )
        menu.addAction(folder_act)

        if f.kind == "trace":
            menu.addSeparator()
            trace_act = QAction(get_icon("bug"), "Show Trace Viewer", self)
            trace_act.triggered.connect(lambda: self.trace_open_requested.emit(f.path))
            menu.addAction(trace_act)

        menu.exec(self._gallery.mapToGlobal(pos))

    def _open_report(self):
        if getattr(self, "_report_path", None):
            QDesktopServices.openUrl(QUrl.fromLocalFile(self._report_path))

    def _update_visibility(self):
        has = len(self._files) > 0
        self._splitter.setVisible(has)
        self._empty_label.setVisible(not has)

    # ---------- Theme ----------

    def apply_theme(self, mode: ThemeMode):
        c = ThemeManager.instance().get_colors(mode)
        self._refresh_btn.setStyleSheet(
            f"QPushButton {{ background-color: {c['primary']}; color: white; font-weight: 600; "
            f"border: none; border-radius: 8px; padding: 6px 14px; }} "
            f"QPushButton:hover {{ background-color: {c['primary_hover']}; }} "
            f"QPushButton:disabled {{ background-color: {c['border']}; color: {c['text_muted']}; }}"
        )
        self._report_btn.setStyleSheet(self._refresh_btn.styleSheet())
        self._count_lbl.setStyleSheet(f"color: {c['text_muted']}; font-size: 12px;")
        self._gallery.setStyleSheet(
            f"QListWidget {{ background-color: {c['bg_surface']}; border: 1px solid {c['border']}; "
            f"border-radius: 8px; outline: none; padding: 8px; color: {c['text_primary']}; font-size: 11px; }} "
            f"QListWidget::item:selected {{ background-color: {c['bg_active']}; border-radius: 6px; }}"
        )
        self._preview.setStyleSheet(
            f"QLabel {{ background-color: {c['terminal_bg']}; color: {c['text_muted']}; "
            f"border: 1px solid {c['border']}; border-radius: 8px; }}"
        )
        self._empty_label.setStyleSheet(f"color: {c['text_muted']}; font-size: 13px;")
