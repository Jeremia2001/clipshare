#!/usr/bin/env python3
"""FFmpeg worker for ClipShare Qt app."""

import subprocess
import tempfile
import os
from pathlib import Path

from PyQt6.QtCore import QThread, pyqtSignal


class FFmpegWorker(QThread):
    progress = pyqtSignal(str)
    finished = pyqtSignal(bool, str, str)

    def __init__(self, ffmpeg_path: str, input_path: str, start: float, end: float):
        super().__init__()
        self.ffmpeg_path = ffmpeg_path
        self.input_path = input_path
        self.trim_start = start
        self.trim_duration = end - start
        self.output_path = None

    def run(self):
        try:
            suffix = Path(self.input_path).suffix
            fd, self.output_path = tempfile.mkstemp(suffix=suffix, prefix='clipshare_trimmed_')
            os.close(fd)

            cmd = [
                self.ffmpeg_path,
                '-y',
                '-ss', str(self.trim_start),
                '-i', self.input_path,
                '-t', str(self.trim_duration),
                '-c:v', 'libx264',
                '-c:a', 'aac',
                '-preset', 'veryfast',
                '-movflags', '+faststart',
                self.output_path
            ]

            self.progress.emit("Trimming video...")

            process = subprocess.Popen(
                cmd,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                universal_newlines=True
            )

            while process.poll() is None:
                self.progress.emit("Processing...")
                self.msleep(100)

            if process.returncode == 0:
                self.finished.emit(True, self.output_path, "")
            else:
                stderr = process.stderr.read() if process.stderr else "Unknown error"
                self.finished.emit(False, "", stderr)
                if self.output_path and os.path.exists(self.output_path):
                    os.unlink(self.output_path)

        except Exception as e:
            self.finished.emit(False, "", str(e))
            if self.output_path and os.path.exists(self.output_path):
                os.unlink(self.output_path)