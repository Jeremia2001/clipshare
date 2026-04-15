#!/usr/bin/env python3
"""Editor page for clip editing."""

import os
from pathlib import Path

from PyQt6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel,
    QPushButton, QLineEdit, QTextEdit, QCheckBox,
    QFileDialog, QMessageBox, QProgressBar, QFrame,
    QSizePolicy, QScrollArea
)
from PyQt6.QtCore import Qt, QThread, pyqtSignal, QMimeData
from PyQt6.QtGui import QDragEnterEvent, QDropEvent, QFont

from widgets.video_player import VideoPlayer
from ffmpeg_wrapper import FFmpegWorker
from api_client import ClipShareApiClient
from config import Config
from theme import (
    BG_DARK, BG_CARD, BG_PANEL, FOREST_700, FOREST_600, FOREST_800,
    MOSS_400, MOSS_500, TEXT_PRIMARY, TEXT_SECONDARY, TEXT_MUTED,
    EARTH_500, BORDER, FONT_FAMILY,
)


class UploadWorker(QThread):
    progress = pyqtSignal(str)
    finished = pyqtSignal(bool, str)

    def __init__(
        self,
        api: ClipShareApiClient,
        file_path: str,
        title: str,
        description: str,
        is_public: bool,
        allow_comments: bool,
        duration: float,
        width: int,
        height: int,
        trim_start: float,
        trim_end: float,
    ):
        super().__init__()
        self.api = api
        self.file_path = file_path
        self.title = title
        self.description = description
        self.is_public = is_public
        self.allow_comments = allow_comments
        self.duration = duration
        self.width = width
        self.height = height
        self.trim_start = trim_start
        self.trim_end = trim_end

    def run(self):
        self.progress.emit("Uploading...")
        result = self.api.upload_clip(
            self.file_path,
            self.title,
            self.description,
            self.is_public,
            self.allow_comments,
            self.trim_start,
            self.trim_end,
            self.duration,
            self.width,
            self.height,
        )
        if result:
            clip_id = result.get('clip', result).get('id', '') if isinstance(result.get('clip'), dict) else result.get('id', '')
            self.finished.emit(True, clip_id)
        else:
            self.finished.emit(False, "Upload failed")


class EditorPage(QWidget):
    clipUploaded = pyqtSignal(str)

    def __init__(self, api: ClipShareApiClient, config: Config):
        super().__init__()
        self.api = api
        self.config = config
        self.current_file = None
        self.trimmed_file = None
        self.ffmpeg_worker = None
        self.upload_worker = None
        self._drag_over = False

        self._setup_ui()
        self.setAcceptDrops(True)

    def _setup_ui(self):
        layout = QVBoxLayout(self)
        layout.setSpacing(0)
        layout.setContentsMargins(0, 0, 0, 0)

        scroll = QScrollArea()
        scroll.setWidgetResizable(True)
        scroll.setFrameShape(QFrame.Shape.NoFrame)
        scroll.setHorizontalScrollBarPolicy(Qt.ScrollBarPolicy.ScrollBarAlwaysOff)

        content = QWidget()
        content_layout = QVBoxLayout(content)
        content_layout.setSpacing(20)
        content_layout.setContentsMargins(32, 24, 32, 24)

        header = QHBoxLayout()
        title = QLabel("Clip Editor")
        title.setObjectName("pageTitle")
        header.addWidget(title)
        header.addStretch()

        self.new_btn = QPushButton("New Clip")
        self.new_btn.setObjectName("ghostBtn")
        self.new_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        self.new_btn.clicked.connect(self._on_new_clip)
        header.addWidget(self.new_btn)

        content_layout.addLayout(header)

        # Drop zone
        self.drop_zone = QFrame()
        self.drop_zone.setObjectName("dropZone")
        self.drop_zone.setMinimumHeight(300)
        drop_layout = QVBoxLayout(self.drop_zone)
        drop_layout.setAlignment(Qt.AlignmentFlag.AlignCenter)
        drop_layout.setSpacing(16)

        drop_icon = QLabel("Drop a video file here")
        drop_icon.setObjectName("dropIcon")
        drop_icon.setAlignment(Qt.AlignmentFlag.AlignCenter)
        drop_layout.addWidget(drop_icon)

        drop_hint = QLabel("or click the button below to select a file")
        drop_hint.setObjectName("dropHint")
        drop_hint.setAlignment(Qt.AlignmentFlag.AlignCenter)
        drop_layout.addWidget(drop_hint)

        self.select_btn = QPushButton("Select Video File")
        self.select_btn.setObjectName("dropSelectBtn")
        self.select_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        self.select_btn.clicked.connect(self._on_select_file)
        drop_layout.addWidget(self.select_btn, alignment=Qt.AlignmentFlag.AlignCenter)

        self.format_hint = QLabel("Supports MP4, MOV, WebM, MKV, AVI")
        self.format_hint.setObjectName("formatHint")
        self.format_hint.setAlignment(Qt.AlignmentFlag.AlignCenter)
        drop_layout.addWidget(self.format_hint)

        content_layout.addWidget(self.drop_zone)

        # Editor (hidden initially)
        self.editor_widget = QWidget()
        editor_layout = QVBoxLayout(self.editor_widget)
        editor_layout.setSpacing(16)

        player_frame = QFrame()
        player_frame.setObjectName("playerFrame")
        player_layout = QVBoxLayout(player_frame)
        player_layout.setContentsMargins(0, 0, 0, 0)

        self.player = VideoPlayer()
        self.player.durationChanged.connect(self._on_duration_changed)
        self.player.trimChanged.connect(self._on_trim_changed)
        player_layout.addWidget(self.player)

        editor_layout.addWidget(player_frame)

        # Details section
        details_frame = QFrame()
        details_frame.setObjectName("detailsFrame")
        details = QVBoxLayout(details_frame)
        details.setSpacing(12)
        details.setContentsMargins(16, 16, 16, 16)

        title_label = QLabel("Clip Details")
        title_label.setObjectName("sectionLabel")
        details.addWidget(title_label)

        self.title_input = QLineEdit()
        self.title_input.setPlaceholderText("Give your clip a title...")
        self.title_input.setMinimumHeight(40)
        details.addWidget(self.title_input)

        self.desc_input = QTextEdit()
        self.desc_input.setPlaceholderText("Add a description (optional)...")
        self.desc_input.setMaximumHeight(80)
        details.addWidget(self.desc_input)

        options = QHBoxLayout()
        options.setSpacing(20)

        self.public_check = QCheckBox("Public")
        self.public_check.setChecked(True)
        options.addWidget(self.public_check)

        self.comments_check = QCheckBox("Allow Comments")
        self.comments_check.setChecked(True)
        options.addWidget(self.comments_check)

        options.addStretch()
        details.addLayout(options)

        editor_layout.addWidget(details_frame)

        # Actions
        actions = QHBoxLayout()
        actions.setSpacing(12)

        self.upload_btn = QPushButton("Upload & Save")
        self.upload_btn.setObjectName("primaryBtn")
        self.upload_btn.setMinimumHeight(44)
        self.upload_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        self.upload_btn.clicked.connect(self._on_upload)
        actions.addWidget(self.upload_btn)

        self.cancel_btn = QPushButton("Cancel")
        self.cancel_btn.setObjectName("ghostBtn")
        self.cancel_btn.setMinimumHeight(44)
        self.cancel_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        self.cancel_btn.clicked.connect(self._on_cancel)
        actions.addWidget(self.cancel_btn)

        actions.addStretch()
        editor_layout.addLayout(actions)

        # Progress
        self.progress_label = QLabel("")
        self.progress_label.setObjectName("progressLabel")
        editor_layout.addWidget(self.progress_label)

        self.progress_bar = QProgressBar()
        self.progress_bar.setRange(0, 0)
        self.progress_bar.hide()
        editor_layout.addWidget(self.progress_bar)

        self.editor_widget.hide()
        content_layout.addWidget(self.editor_widget)

        content_layout.addStretch()

        scroll.setWidget(content)
        layout.addWidget(scroll, stretch=1)

        self.setStyleSheet(f"""
            QLabel#pageTitle {{
                color: {TEXT_PRIMARY};
                font-size: 22px;
                font-weight: 700;
            }}
            QPushButton#primaryBtn {{
                background-color: {FOREST_700};
                color: white;
                border: none;
                border-radius: 8px;
                padding: 12px 28px;
                font-size: 14px;
                font-weight: 600;
            }}
            QPushButton#primaryBtn:hover {{
                background-color: {FOREST_600};
            }}
            QPushButton#primaryBtn:disabled {{
                background-color: #555;
                color: #999;
            }}
            QPushButton#ghostBtn {{
                background-color: transparent;
                color: {TEXT_SECONDARY};
                border: 1px solid {BORDER};
                border-radius: 8px;
                padding: 10px 16px;
                font-size: 13px;
            }}
            QPushButton#ghostBtn:hover {{
                background-color: {BG_CARD};
                color: {TEXT_PRIMARY};
                border-color: {TEXT_SECONDARY};
            }}
            QFrame#dropZone {{
                background-color: {BG_CARD};
                border: 2px dashed {FOREST_600};
                border-radius: 16px;
            }}
            QFrame#dropZone[hover="true"] {{
                background-color: {FOREST_800};
                border-color: {MOSS_400};
            }}
            QLabel#dropIcon {{
                color: {MOSS_400};
                font-size: 18px;
                font-weight: 600;
                background: transparent;
                border: none;
            }}
            QLabel#dropHint {{
                color: {TEXT_MUTED};
                font-size: 13px;
                background: transparent;
                border: none;
            }}
            QLabel#formatHint {{
                color: {TEXT_MUTED};
                font-size: 11px;
                background: transparent;
                border: none;
            }}
            QPushButton#dropSelectBtn {{
                background-color: {FOREST_700};
                color: white;
                border: none;
                border-radius: 8px;
                padding: 12px 28px;
                font-size: 14px;
                font-weight: 600;
            }}
            QPushButton#dropSelectBtn:hover {{
                background-color: {FOREST_600};
            }}
            QFrame#playerFrame {{
                background-color: #111;
                border-radius: 12px;
                border: 1px solid #222;
            }}
            QFrame#detailsFrame {{
                background-color: {BG_CARD};
                border: 1px solid {BORDER};
                border-radius: 10px;
            }}
            QLabel#sectionLabel {{
                color: {TEXT_PRIMARY};
                font-size: 14px;
                font-weight: 600;
            }}
            QLabel#progressLabel {{
                color: {MOSS_400};
                font-size: 13px;
            }}
        """)

    def dragEnterEvent(self, event: QDragEnterEvent):
        if event.mimeData().hasUrls():
            event.acceptProposedAction()
            self._drag_over = True
            self.drop_zone.setProperty("hover", True)
            self.drop_zone.style().unpolish(self.drop_zone)
            self.drop_zone.style().polish(self.drop_zone)

    def dragLeaveEvent(self, event):
        self._drag_over = False
        self.drop_zone.setProperty("hover", False)
        self.drop_zone.style().unpolish(self.drop_zone)
        self.drop_zone.style().polish(self.drop_zone)

    def dropEvent(self, event: QDropEvent):
        self._drag_over = False
        self.drop_zone.setProperty("hover", False)
        self.drop_zone.style().unpolish(self.drop_zone)
        self.drop_zone.style().polish(self.drop_zone)
        urls = event.mimeData().urls()
        if urls:
            filepath = urls[0].toLocalFile()
            if self._is_video_file(filepath):
                self._load_file(filepath)

    def _is_video_file(self, filepath: str) -> bool:
        ext = Path(filepath).suffix.lower()
        return ext in {'.mp4', '.mov', '.webm', '.mkv', '.avi', '.m4v'}

    def _on_select_file(self):
        filepath, _ = QFileDialog.getOpenFileName(
            self,
            "Select Video File",
            "",
            "Video Files (*.mp4 *.mov *.webm *.mkv *.avi *.m4v)"
        )
        if filepath:
            self._load_file(filepath)

    def _load_file(self, filepath: str):
        self.current_file = filepath
        self.trimmed_file = None

        self.drop_zone.hide()
        self.editor_widget.show()

        title = Path(filepath).stem.replace('_', ' ').replace('-', ' ').title()
        self.title_input.setText(title)

        self.player.load(filepath)
        self.player.play()

    def _on_duration_changed(self, duration_ms: int):
        self._update_progress(f"Duration: {duration_ms/1000:.1f}s")

    def _on_trim_changed(self, start_sec: float, end_sec: float):
        duration = end_sec - start_sec
        self._update_progress(f"Trim: {duration:.1f}s")

    def _on_upload(self):
        if not self.current_file:
            return

        title = self.title_input.text().strip()
        if not title:
            QMessageBox.warning(self, "Missing Title", "Please enter a clip title.")
            return

        trim_start, trim_end = self.player.get_trim_range()
        duration = self.player._duration / 1000.0

        is_trimmed = trim_start > 0.5 or trim_end < duration - 0.5

        if is_trimmed:
            self._start_trimming(title, trim_start, trim_end)
        else:
            self._start_upload(self.current_file, title, 0, duration)

    def _start_trimming(self, title: str, start: float, end: float):
        self._set_ui_enabled(False)
        self.progress_bar.show()
        self._update_progress("Preparing trim...")

        ffmpeg_path = self.config.get_ffmpeg_cmd()
        self.ffmpeg_worker = FFmpegWorker(ffmpeg_path, self.current_file, start, end)

        self.ffmpeg_worker.progress.connect(self._update_progress)
        self.ffmpeg_worker.finished.connect(
            lambda success, output, error: self._on_trim_finished(success, output, error, title, start, end)
        )

        self.ffmpeg_worker.start()

    def _on_trim_finished(self, success: bool, output_path: str, error: str, title: str, start: float, end: float):
        worker = self.ffmpeg_worker
        self.ffmpeg_worker = None
        if worker is not None:
            worker.wait(3000)

        if success:
            self.trimmed_file = output_path
            self._start_upload(output_path, title, start, end)
        else:
            self._set_ui_enabled(True)
            self.progress_bar.hide()
            self._update_progress(f"Trim failed: {error}")
            QMessageBox.critical(self, "Trim Failed", f"Failed to trim video:\n{error}")

    def _start_upload(self, filepath: str, title: str, trim_start: float, trim_end: float):
        self._update_progress("Uploading...")

        duration = self.player._duration / 1000.0
        width, height = self.player.get_resolution()
        if width == 0 or height == 0:
            width, height = 1920, 1080

        self.upload_worker = UploadWorker(
            self.api,
            filepath,
            title,
            self.desc_input.toPlainText(),
            self.public_check.isChecked(),
            self.comments_check.isChecked(),
            duration,
            width,
            height,
            trim_start,
            trim_end,
        )

        self.upload_worker.progress.connect(self._update_progress)
        self.upload_worker.finished.connect(self._on_upload_finished)
        self.upload_worker.start()

    def _on_upload_finished(self, success: bool, clip_id: str):
        worker = self.upload_worker
        self.upload_worker = None
        if worker is not None:
            worker.wait(3000)

        self.progress_bar.hide()

        if self.trimmed_file and os.path.exists(self.trimmed_file):
            os.unlink(self.trimmed_file)
            self.trimmed_file = None

        if success:
            self._update_progress("Upload complete!")
            self.clipUploaded.emit(clip_id)
            self._on_new_clip()
        else:
            self._set_ui_enabled(True)
            self._update_progress(f"Upload failed: {clip_id}")
            QMessageBox.critical(self, "Upload Failed", f"Failed to upload:\n{clip_id}")

    def _on_cancel(self):
        if self.ffmpeg_worker and self.ffmpeg_worker.isRunning():
            self.ffmpeg_worker.terminate()
            self.ffmpeg_worker.wait()

        if self.upload_worker and self.upload_worker.isRunning():
            self.upload_worker.terminate()
            self.upload_worker.wait()

        self._on_new_clip()

    def enter_viewer_mode(self, title: str = ""):
        self.current_file = None
        self.upload_btn.hide()
        self.cancel_btn.setText("Close")

    def enter_editor_mode(self):
        self.upload_btn.show()
        self.cancel_btn.setText("Cancel")

    def _on_new_clip(self):
        self.current_file = None
        self.player.stop()

        if self.trimmed_file and os.path.exists(self.trimmed_file):
            os.unlink(self.trimmed_file)
        self.trimmed_file = None

        self.title_input.clear()
        self.desc_input.clear()
        self.progress_label.clear()
        self.progress_bar.hide()

        self._set_ui_enabled(True)
        self.editor_widget.hide()
        self.drop_zone.show()

    def _set_ui_enabled(self, enabled: bool):
        self.upload_btn.setEnabled(enabled)
        self.select_btn.setEnabled(enabled)
        self.title_input.setEnabled(enabled)
        self.desc_input.setEnabled(enabled)
        self.public_check.setEnabled(enabled)
        self.comments_check.setEnabled(enabled)

    def _update_progress(self, message: str):
        self.progress_label.setText(message)