#!/usr/bin/env python3
"""Video player widget with trim controls."""

from PyQt6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QPushButton,
    QLabel, QStyle, QFrame, QSizePolicy
)
from PyQt6.QtCore import Qt, QUrl, pyqtSignal, QTimer
from PyQt6.QtGui import QPainter, QColor, QPen, QFont, QLinearGradient
from PyQt6.QtMultimedia import QMediaPlayer, QAudioOutput, QMediaMetaData
from PyQt6.QtMultimediaWidgets import QVideoWidget

from theme import (
    BG_DARK, BG_PANEL, FOREST_700, FOREST_600, FOREST_800,
    MOSS_400, MOSS_500, TEXT_PRIMARY, TEXT_SECONDARY, TEXT_MUTED,
    BORDER, FONT_MONO,
)


class VideoPlayer(QWidget):
    positionChanged = pyqtSignal(int)
    durationChanged = pyqtSignal(int)
    trimChanged = pyqtSignal(float, float)

    def __init__(self, parent=None):
        super().__init__(parent)
        self._duration = 0
        self._trim_start = 0.0
        self._trim_end = 0.0
        self._video_width = 0
        self._video_height = 0

        self._setup_ui()
        self._setup_player()

    def _setup_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(0, 0, 0, 8)
        layout.setSpacing(8)

        self.video_widget = QVideoWidget()
        self.video_widget.setMinimumHeight(280)
        self.video_widget.setSizePolicy(QSizePolicy.Policy.Expanding, QSizePolicy.Policy.Expanding)
        self.video_widget.setStyleSheet("background-color: #000; border-radius: 10px;")
        layout.addWidget(self.video_widget, stretch=1)

        controls = QHBoxLayout()
        controls.setSpacing(12)
        controls.setContentsMargins(8, 4, 8, 4)

        self.play_btn = QPushButton()
        self.play_btn.setIcon(self.style().standardIcon(QStyle.StandardPixmap.SP_MediaPlay))
        self.play_btn.setFixedSize(36, 36)
        self.play_btn.setObjectName("playBtn")
        self.play_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        self.play_btn.clicked.connect(self.toggle_play)
        controls.addWidget(self.play_btn)

        self.time_label = QLabel("0:00 / 0:00")
        self.time_label.setObjectName("timeLabel")
        controls.addWidget(self.time_label)

        controls.addStretch()

        self.trim_label = QLabel("Trim: 0:00 - 0:00")
        self.trim_label.setObjectName("trimLabel")
        controls.addWidget(self.trim_label)

        layout.addLayout(controls)

        self.trim_bar = TrimBar()
        self.trim_bar.positionChanged.connect(self._on_trim_position_changed)
        self.trim_bar.startChanged.connect(self._on_trim_start_changed)
        self.trim_bar.endChanged.connect(self._on_trim_end_changed)
        self.trim_bar.seekRequested.connect(self._on_seek_requested)
        layout.addWidget(self.trim_bar)

        self.setStyleSheet(f"""
            QPushButton#playBtn {{
                background-color: {FOREST_700};
                border-radius: 18px;
                color: white;
                border: none;
            }}
            QPushButton#playBtn:hover {{
                background-color: {FOREST_600};
            }}
            QLabel#timeLabel {{
                color: {TEXT_SECONDARY};
                font-family: {FONT_MONO};
                font-size: 12px;
                background: transparent;
                border: none;
            }}
            QLabel#trimLabel {{
                color: {MOSS_400};
                font-family: {FONT_MONO};
                font-size: 12px;
                background: transparent;
                border: none;
            }}
        """)

    def _setup_player(self):
        self.audio_output = QAudioOutput()
        self.audio_output.setVolume(1.0)

        self.player = QMediaPlayer()
        self.player.setAudioOutput(self.audio_output)
        self.player.setVideoOutput(self.video_widget)

        self.player.positionChanged.connect(self._on_position_changed)
        self.player.durationChanged.connect(self._on_duration_changed)
        self.player.playbackStateChanged.connect(self._on_state_changed)
        self.player.metaDataChanged.connect(self._on_metadata_changed)

        self.update_timer = QTimer(self)
        self.update_timer.timeout.connect(self._update_ui)
        self.update_timer.start(100)

    def load(self, filepath: str):
        self.player.setSource(QUrl.fromLocalFile(filepath))
        self._trim_start = 0.0
        self._trim_end = 0.0

    def load_url(self, url: str):
        self.player.setSource(QUrl(url))
        self._trim_start = 0.0
        self._trim_end = 0.0

    def play(self):
        self.player.play()

    def pause(self):
        self.player.pause()

    def toggle_play(self):
        if self.player.playbackState() == QMediaPlayer.PlaybackState.PlayingState:
            self.player.pause()
        else:
            self.player.play()

    def stop(self):
        self.player.stop()

    def seek(self, position_ms: int):
        self.player.setPosition(position_ms)

    def get_trim_range(self) -> tuple:
        return (self._trim_start, self._trim_end)

    def get_resolution(self) -> tuple:
        return (self._video_width, self._video_height)

    def _on_position_changed(self, position):
        self.positionChanged.emit(position)
        self.trim_bar.set_position(position)
        if self._trim_end > self._trim_start:
            if position >= int(self._trim_end * 1000):
                self.seek(int(self._trim_start * 1000))

    def _on_duration_changed(self, duration):
        self._duration = duration
        self.durationChanged.emit(duration)
        self.trim_bar.set_duration(duration)
        self._update_time_labels()
        if duration > 0:
            self._trim_end = duration / 1000.0
            self.trimChanged.emit(self._trim_start, self._trim_end)

    def _on_state_changed(self, state):
        if state == QMediaPlayer.PlaybackState.PlayingState:
            self.play_btn.setIcon(self.style().standardIcon(QStyle.StandardPixmap.SP_MediaPause))
        else:
            self.play_btn.setIcon(self.style().standardIcon(QStyle.StandardPixmap.SP_MediaPlay))

    def _on_metadata_changed(self):
        meta = self.player.metaData()
        if meta:
            res = meta.value(QMediaMetaData.Key.Resolution)
            if res is not None:
                self._video_width = res.width()
                self._video_height = res.height()

    def _on_seek_requested(self, position_ms: int):
        self.seek(position_ms)

    def _on_trim_start_changed(self, start_ms: int):
        self._trim_start = start_ms / 1000.0
        self._update_time_labels()
        self.trimChanged.emit(self._trim_start, self._trim_end)
        self.seek(start_ms)

    def _on_trim_end_changed(self, end_ms: int):
        self._trim_end = end_ms / 1000.0
        self._update_time_labels()
        self.trimChanged.emit(self._trim_start, self._trim_end)

    def _on_trim_position_changed(self, position_ms: int):
        pass

    def _update_ui(self):
        if self._duration > 0:
            self._update_time_labels()

    def _update_time_labels(self):
        current = self.player.position()
        self.time_label.setText(
            f"{self._format_time(current)} / {self._format_time(self._duration)}"
        )
        self.trim_label.setText(
            f"Trim: {self._format_time(int(self._trim_start * 1000))} - "
            f"{self._format_time(int(self._trim_end * 1000))}"
        )

    @staticmethod
    def _format_time(ms: int) -> str:
        seconds = ms // 1000
        m, s = divmod(seconds, 60)
        if m >= 60:
            h, m = divmod(m, 60)
            return f"{h}:{m:02d}:{s:02d}"
        return f"{m}:{s:02d}"


class TrimBar(QWidget):
    positionChanged = pyqtSignal(int)
    startChanged = pyqtSignal(int)
    endChanged = pyqtSignal(int)
    seekRequested = pyqtSignal(int)

    HANDLE_WIDTH = 8
    MIN_SELECTION = 1000  # 1 second minimum trim

    def __init__(self, parent=None):
        super().__init__(parent)
        self._duration = 0
        self._position = 0
        self._start = 0
        self._end = 0
        self._dragging = None
        self._hover_zone = None

        self.setMinimumHeight(48)
        self.setMaximumHeight(56)
        self.setCursor(Qt.CursorShape.PointingHandCursor)

    def set_duration(self, duration_ms: int):
        self._duration = duration_ms
        self._end = duration_ms
        self.update()

    def set_position(self, position_ms: int):
        self._position = position_ms
        self.update()

    def paintEvent(self, event):
        if self._duration == 0:
            painter = QPainter(self)
            painter.fillRect(self.rect(), QColor(BG_PANEL))
            painter.setPen(QColor(TEXT_MUTED))
            painter.setFont(QFont("sans-serif", 11))
            painter.drawText(self.rect(), Qt.AlignmentFlag.AlignCenter, "No video loaded")
            painter.end()
            return

        painter = QPainter(self)
        painter.setRenderHint(QPainter.RenderHint.Antialiasing)

        w = self.width()
        h = self.height()
        bar_y = 10
        bar_h = h - 20

        painter.fillRect(0, 0, w, h, QColor(BG_DARK))

        painter.setPen(Qt.PenStyle.NoPen)
        painter.setBrush(QColor("#111"))
        painter.drawRoundedRect(0, bar_y, w, bar_h, 4, 4)

        start_x = int((self._start / self._duration) * w)
        end_x = int((self._end / self._duration) * w)
        pos_x = int((self._position / self._duration) * w)

        if end_x > start_x:
            gradient = QLinearGradient(start_x, 0, end_x, 0)
            gradient.setColorAt(0, QColor(FOREST_800))
            gradient.setColorAt(0.5, QColor(FOREST_700))
            gradient.setColorAt(1, QColor(FOREST_800))
            painter.setPen(Qt.PenStyle.NoPen)
            painter.setBrush(gradient)
            painter.drawRoundedRect(start_x, bar_y, end_x - start_x, bar_h, 4, 4)

        handle_color = QColor(MOSS_400)
        handle_hover_color = QColor(MOSS_500)

        for hx, zone in [(start_x, 'start'), (end_x, 'end')]:
            color = handle_hover_color if self._hover_zone == zone else handle_color
            painter.setPen(Qt.PenStyle.NoPen)
            painter.setBrush(color)
            painter.drawRoundedRect(hx - self.HANDLE_WIDTH // 2, bar_y - 2, self.HANDLE_WIDTH, bar_h + 4, 3, 3)

            grip_y_center = bar_y + bar_h // 2
            for dy in [-3, 0, 3]:
                painter.setPen(QColor("#1a1a1a"))
                painter.drawLine(hx - 2, grip_y_center + dy, hx + 2, grip_y_center + dy)

        pen_color = QColor(255, 255, 255, 200)
        painter.setPen(QPen(pen_color, 2))
        painter.drawLine(pos_x, bar_y - 2, pos_x, bar_y + bar_h + 2)

        painter.end()

    def _get_time_from_x(self, x: int) -> int:
        return int((x / self.width()) * self._duration)

    def _get_x_from_time(self, time_ms: int) -> int:
        return int((time_ms / self._duration) * self.width()) if self._duration else 0

    def mousePressEvent(self, event):
        if self._duration == 0:
            return
        x = event.pos().x()
        start_x = self._get_x_from_time(self._start)
        end_x = self._get_x_from_time(self._end)

        handle_radius = self.HANDLE_WIDTH + 4
        if abs(x - start_x) < handle_radius:
            self._dragging = 'start'
        elif abs(x - end_x) < handle_radius:
            self._dragging = 'end'
        else:
            self._dragging = 'position'
            time_ms = self._get_time_from_x(x)
            self.seekRequested.emit(time_ms)
        self.update()

    def mouseMoveEvent(self, event):
        if self._dragging is None or self._duration == 0:
            x = event.pos().x()
            start_x = self._get_x_from_time(self._start)
            end_x = self._get_x_from_time(self._end)
            handle_radius = self.HANDLE_WIDTH + 4
            if abs(x - start_x) < handle_radius:
                self._hover_zone = 'start'
                self.setCursor(Qt.CursorShape.SizeHorCursor)
            elif abs(x - end_x) < handle_radius:
                self._hover_zone = 'end'
                self.setCursor(Qt.CursorShape.SizeHorCursor)
            else:
                self._hover_zone = None
                self.setCursor(Qt.CursorShape.PointingHandCursor)
            self.update()
            return

        x = event.pos().x()
        time_ms = self._get_time_from_x(x)

        if self._dragging == 'start':
            time_ms = max(0, min(time_ms, self._end - self.MIN_SELECTION))
            self._start = time_ms
            self.startChanged.emit(time_ms)
        elif self._dragging == 'end':
            time_ms = max(self._start + self.MIN_SELECTION, min(time_ms, self._duration))
            self._end = time_ms
            self.endChanged.emit(time_ms)

        self.update()

    def mouseReleaseEvent(self, event):
        self._dragging = None