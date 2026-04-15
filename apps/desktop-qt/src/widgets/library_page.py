#!/usr/bin/env python3
"""Clip library browser page."""

from datetime import datetime, timezone

from PyQt6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel,
    QPushButton, QListWidget, QListWidgetItem, QMessageBox,
    QFrame, QSizePolicy, QApplication, QGridLayout, QScrollArea
)
from PyQt6.QtCore import Qt, pyqtSignal, QThread, QSize
from PyQt6.QtGui import QFont

from theme import (
    BG_DARK, BG_CARD, BG_CARD_HOVER, BG_PANEL, FOREST_700, FOREST_600,
    FOREST_800, MOSS_400, MOSS_500, TEXT_PRIMARY, TEXT_SECONDARY,
    TEXT_MUTED, EARTH_500, BORDER, FONT_FAMILY, FONT_MONO,
)
from models import Clip


def _relative_time(iso_string: str) -> str:
    """Return a human-readable relative time string from an ISO timestamp."""
    if not iso_string:
        return ""
    try:
        dt = datetime.fromisoformat(iso_string.replace('Z', '+00:00'))
        diff = datetime.now(timezone.utc) - dt
        s = int(diff.total_seconds())
        if s < 60:
            return "just now"
        if s < 3600:
            m = s // 60
            return f"{m}m ago"
        if s < 86400:
            h = s // 3600
            return f"{h}h ago"
        if s < 7 * 86400:
            d = s // 86400
            return f"{d}d ago"
        return dt.strftime("%b %d")
    except Exception:
        return ""


class LoadClipsWorker(QThread):
    finished = pyqtSignal(object, str)

    def __init__(self, api):
        super().__init__()
        self.api = api

    def run(self):
        try:
            result = self.api.get_clips()
            if result:
                clips_data = result.get('clips', [])
                clips = [Clip.from_dict(c) for c in clips_data]
                self.finished.emit(clips, "")
            else:
                self.finished.emit(None, "Failed to load clips")
        except Exception as e:
            import traceback
            traceback.print_exc()
            self.finished.emit(None, str(e))


class ClipCard(QFrame):
    playClicked = pyqtSignal(str, str)
    shareClicked = pyqtSignal(str)
    deleteClicked = pyqtSignal(str)

    def __init__(self, clip: Clip, parent=None):
        super().__init__(parent)
        self.clip = clip
        self.setObjectName("clipCard")
        self.setCursor(Qt.CursorShape.PointingHandCursor)
        self.setFixedHeight(148)
        self._setup_ui(clip)

    def _setup_ui(self, clip: Clip):
        layout = QHBoxLayout(self)
        layout.setContentsMargins(0, 0, 12, 0)
        layout.setSpacing(0)

        # Thumbnail
        thumb = QFrame()
        thumb.setObjectName("clipThumb")
        thumb.setFixedSize(148, 148)
        thumb_layout = QVBoxLayout(thumb)
        thumb_layout.setContentsMargins(0, 0, 0, 8)
        thumb_layout.setSpacing(0)
        thumb_layout.setAlignment(Qt.AlignmentFlag.AlignCenter)

        thumb_layout.addStretch()

        icon = QLabel("▶")
        icon.setObjectName("thumbIcon")
        icon.setAlignment(Qt.AlignmentFlag.AlignCenter)
        thumb_layout.addWidget(icon)

        thumb_layout.addStretch()

        if clip.duration_seconds > 0:
            dur = QLabel(clip.duration_display)
            dur.setObjectName("thumbDuration")
            dur.setAlignment(Qt.AlignmentFlag.AlignCenter)
            thumb_layout.addWidget(dur, alignment=Qt.AlignmentFlag.AlignBottom | Qt.AlignmentFlag.AlignRight)

        layout.addWidget(thumb)

        # Info
        info = QVBoxLayout()
        info.setSpacing(3)
        info.setContentsMargins(14, 12, 0, 12)

        title = QLabel(clip.title)
        title.setObjectName("clipTitle")
        title.setWordWrap(True)
        title.setMaximumHeight(44)
        info.addWidget(title)

        # Meta row: size · resolution · visibility
        meta_parts = [p for p in [clip.size_display, clip.resolution_display] if p]
        visibility = "Public" if clip.is_public else "Private"
        meta_parts.append(visibility)
        meta_line = QLabel("  ·  ".join(meta_parts))
        meta_line.setObjectName("clipMeta")
        info.addWidget(meta_line)

        # Timestamp
        rel_time = _relative_time(clip.created_at)
        if rel_time:
            time_label = QLabel(rel_time)
            time_label.setObjectName("clipTime")
            info.addWidget(time_label)

        if clip.description:
            desc = QLabel(clip.description)
            desc.setObjectName("clipDesc")
            desc.setWordWrap(True)
            desc.setMaximumHeight(32)
            info.addWidget(desc)

        info.addStretch()

        btn_row = QHBoxLayout()
        btn_row.setSpacing(6)

        play_btn = QPushButton("▶  Play")
        play_btn.setObjectName("actionBtn")
        play_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        play_btn.clicked.connect(lambda: self.playClicked.emit(self.clip.id, self.clip.view_url))
        btn_row.addWidget(play_btn)

        share_btn = QPushButton("Share")
        share_btn.setObjectName("ghostBtn")
        share_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        share_btn.clicked.connect(lambda: self.shareClicked.emit(self.clip.id))
        btn_row.addWidget(share_btn)

        delete_btn = QPushButton("Delete")
        delete_btn.setObjectName("dangerBtn")
        delete_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        delete_btn.clicked.connect(lambda: self.deleteClicked.emit(self.clip.id))
        btn_row.addWidget(delete_btn)

        btn_row.addStretch()

        info.addLayout(btn_row)
        layout.addLayout(info, stretch=1)

        self.setStyleSheet(f"""
            QFrame#clipCard {{
                background-color: {BG_CARD};
                border: 1px solid {BORDER};
                border-radius: 10px;
            }}
            QFrame#clipCard:hover {{
                background-color: {BG_CARD_HOVER};
                border-color: {FOREST_700};
            }}
            QFrame#clipThumb {{
                background-color: #0d0d0d;
                border-top-left-radius: 9px;
                border-bottom-left-radius: 9px;
                border-right: 1px solid #222;
            }}
            QLabel#thumbIcon {{
                color: {FOREST_700};
                font-size: 26px;
                background: transparent;
                border: none;
            }}
            QLabel#thumbDuration {{
                color: {TEXT_PRIMARY};
                font-size: 10px;
                font-family: {FONT_MONO};
                background-color: rgba(0,0,0,0.6);
                padding: 2px 5px;
                border-radius: 3px;
                border: none;
                margin: 0 4px;
            }}
            QLabel#clipTitle {{
                color: {TEXT_PRIMARY};
                font-size: 14px;
                font-weight: 600;
                background: transparent;
                border: none;
            }}
            QLabel#clipMeta {{
                color: {TEXT_SECONDARY};
                font-size: 11px;
                background: transparent;
                border: none;
            }}
            QLabel#clipTime {{
                color: {TEXT_MUTED};
                font-size: 11px;
                background: transparent;
                border: none;
            }}
            QLabel#clipDesc {{
                color: {TEXT_MUTED};
                font-size: 11px;
                background: transparent;
                border: none;
            }}
            QPushButton#actionBtn {{
                background-color: {FOREST_700};
                color: white;
                border: none;
                border-radius: 6px;
                padding: 5px 12px;
                font-size: 12px;
                max-height: 28px;
            }}
            QPushButton#actionBtn:hover {{
                background-color: {FOREST_600};
            }}
            QPushButton#ghostBtn {{
                background-color: transparent;
                color: {TEXT_SECONDARY};
                border: 1px solid {BORDER};
                border-radius: 6px;
                padding: 5px 12px;
                font-size: 12px;
                max-height: 28px;
            }}
            QPushButton#ghostBtn:hover {{
                background-color: {BG_CARD_HOVER};
                color: {TEXT_PRIMARY};
                border-color: {TEXT_SECONDARY};
            }}
            QPushButton#dangerBtn {{
                background-color: transparent;
                color: {TEXT_MUTED};
                border: 1px solid {BORDER};
                border-radius: 6px;
                padding: 5px 12px;
                font-size: 12px;
                max-height: 28px;
            }}
            QPushButton#dangerBtn:hover {{
                background-color: rgba(155, 34, 38, 0.12);
                color: {EARTH_500};
                border-color: {EARTH_500};
            }}
        """)


class LibraryPage(QWidget):
    playClip = pyqtSignal(str, str)
    createShare = pyqtSignal(str)
    newClipRequested = pyqtSignal()

    def __init__(self, api):
        super().__init__()
        self.api = api
        self.clips = []
        self._load_worker = None
        self._setup_ui()

    def _setup_ui(self):
        layout = QVBoxLayout(self)
        layout.setSpacing(0)
        layout.setContentsMargins(0, 0, 0, 0)

        # Header bar
        header_bar = QWidget()
        header_bar.setObjectName("headerBar")
        header_layout = QHBoxLayout(header_bar)
        header_layout.setContentsMargins(24, 16, 24, 16)
        header_layout.setSpacing(12)

        title = QLabel("My Clips")
        title.setObjectName("pageTitle")
        header_layout.addWidget(title)

        self.clip_count = QLabel("")
        self.clip_count.setObjectName("clipCount")
        header_layout.addWidget(self.clip_count)

        header_layout.addStretch()

        self.refresh_btn = QPushButton("Refresh")
        self.refresh_btn.setObjectName("ghostBtn")
        self.refresh_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        self.refresh_btn.clicked.connect(self.load_clips_async)
        header_layout.addWidget(self.refresh_btn)

        new_btn = QPushButton("+ New Clip")
        new_btn.setObjectName("primaryBtn")
        new_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        new_btn.clicked.connect(lambda: self.newClipRequested.emit())
        header_layout.addWidget(new_btn)

        layout.addWidget(header_bar)

        # Divider
        div = QFrame()
        div.setFrameShape(QFrame.Shape.HLine)
        div.setObjectName("headerDivider")
        layout.addWidget(div)

        # Scroll area
        scroll = QScrollArea()
        scroll.setWidgetResizable(True)
        scroll.setFrameShape(QFrame.Shape.NoFrame)
        scroll.setHorizontalScrollBarPolicy(Qt.ScrollBarPolicy.ScrollBarAlwaysOff)

        scroll_content = QWidget()
        self.cards_layout = QVBoxLayout(scroll_content)
        self.cards_layout.setSpacing(10)
        self.cards_layout.setAlignment(Qt.AlignmentFlag.AlignTop)
        self.cards_layout.setContentsMargins(24, 16, 24, 16)

        scroll.setWidget(scroll_content)
        layout.addWidget(scroll, stretch=1)

        # Empty state (overlaid in scroll content when no clips)
        self.empty_widget = QWidget()
        empty_layout = QVBoxLayout(self.empty_widget)
        empty_layout.setAlignment(Qt.AlignmentFlag.AlignCenter)
        empty_layout.setSpacing(10)

        empty_icon = QLabel("▶")
        empty_icon.setObjectName("emptyIcon")
        empty_icon.setAlignment(Qt.AlignmentFlag.AlignCenter)
        empty_layout.addWidget(empty_icon)

        empty_title = QLabel("No clips yet")
        empty_title.setObjectName("emptyTitle")
        empty_title.setAlignment(Qt.AlignmentFlag.AlignCenter)
        empty_layout.addWidget(empty_title)

        empty_hint = QLabel('Click  "+ New Clip"  to upload your first video')
        empty_hint.setObjectName("emptyHint")
        empty_hint.setAlignment(Qt.AlignmentFlag.AlignCenter)
        empty_layout.addWidget(empty_hint)

        self.empty_widget.hide()
        self.cards_layout.addWidget(self.empty_widget)

        # Loading label
        self.loading_label = QLabel("Loading clips…")
        self.loading_label.setObjectName("loadingLabel")
        self.loading_label.setAlignment(Qt.AlignmentFlag.AlignCenter)
        self.loading_label.hide()
        self.cards_layout.addWidget(self.loading_label)

        self.setStyleSheet(f"""
            QWidget#headerBar {{
                background-color: {BG_PANEL};
            }}
            QFrame#headerDivider {{
                color: {BORDER};
                max-height: 1px;
                background-color: {BORDER};
            }}
            QLabel#pageTitle {{
                color: {TEXT_PRIMARY};
                font-size: 20px;
                font-weight: 700;
                background: transparent;
            }}
            QLabel#clipCount {{
                color: {TEXT_MUTED};
                font-size: 13px;
                background: transparent;
                padding-top: 3px;
            }}
            QPushButton#primaryBtn {{
                background-color: {FOREST_700};
                color: white;
                border: none;
                border-radius: 8px;
                padding: 9px 18px;
                font-size: 13px;
                font-weight: 600;
            }}
            QPushButton#primaryBtn:hover {{
                background-color: {FOREST_600};
            }}
            QPushButton#ghostBtn {{
                background-color: transparent;
                color: {TEXT_SECONDARY};
                border: 1px solid {BORDER};
                border-radius: 8px;
                padding: 9px 14px;
                font-size: 13px;
            }}
            QPushButton#ghostBtn:hover {{
                background-color: {BG_CARD};
                color: {TEXT_PRIMARY};
                border-color: {TEXT_SECONDARY};
            }}
            QLabel#emptyIcon {{
                color: {FOREST_800};
                font-size: 52px;
                background: transparent;
            }}
            QLabel#emptyTitle {{
                color: {TEXT_SECONDARY};
                font-size: 17px;
                font-weight: 600;
                background: transparent;
            }}
            QLabel#emptyHint {{
                color: {TEXT_MUTED};
                font-size: 13px;
                background: transparent;
            }}
            QLabel#loadingLabel {{
                color: {MOSS_400};
                font-size: 14px;
                background: transparent;
            }}
            QScrollArea {{
                background-color: {BG_DARK};
                border: none;
            }}
        """)

    def load_clips_async(self):
        if self._load_worker and self._load_worker.isRunning():
            return

        self.empty_widget.hide()
        self.loading_label.show()
        self.clip_count.setText("")
        self._clear_cards()

        self._load_worker = LoadClipsWorker(self.api)
        self._load_worker.finished.connect(self._on_clips_loaded)
        self._load_worker.start()

    def _clear_cards(self):
        for card in self.findChildren(ClipCard):
            card.deleteLater()

    def _on_clips_loaded(self, clips, error):
        self.loading_label.hide()

        if clips is not None:
            self.clips = clips
            if not clips:
                self.empty_widget.show()
                self.clip_count.setText("")
            else:
                self.empty_widget.hide()
                n = len(clips)
                self.clip_count.setText(f"{n} clip{'s' if n != 1 else ''}")
                for clip in clips:
                    card = ClipCard(clip)
                    card.playClicked.connect(self._on_play)
                    card.shareClicked.connect(self._on_share)
                    card.deleteClicked.connect(self._on_delete)
                    self.cards_layout.addWidget(card)
        else:
            err_label = QLabel(f"Failed to load clips: {error}" if error else "Failed to load clips")
            err_label.setStyleSheet(f"color: {EARTH_500}; font-size: 14px; background: transparent;")
            err_label.setAlignment(Qt.AlignmentFlag.AlignCenter)
            self.cards_layout.addWidget(err_label)

    def _on_play(self, clip_id: str, view_url: str):
        if not view_url:
            clip_data = self.api.get_clip(clip_id)
            if clip_data:
                view_url = clip_data.get('view_url', '')
                if not view_url:
                    clip_obj = clip_data.get('clip', clip_data)
                    view_url = Clip.from_dict(clip_obj).view_url
        self.playClip.emit(clip_id, view_url or "")

    def _on_share(self, clip_id: str):
        self.createShare.emit(clip_id)

    def _on_delete(self, clip_id: str):
        reply = QMessageBox.question(
            self,
            "Delete Clip",
            "Are you sure you want to delete this clip?\nThis cannot be undone.",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
            QMessageBox.StandardButton.No,
        )
        if reply == QMessageBox.StandardButton.Yes:
            QApplication.setOverrideCursor(Qt.CursorShape.WaitCursor)
            ok = self.api.delete_clip(clip_id)
            QApplication.restoreOverrideCursor()
            if ok:
                self.load_clips_async()
            else:
                QMessageBox.critical(self, "Error", "Failed to delete clip.")
