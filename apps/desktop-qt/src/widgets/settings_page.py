#!/usr/bin/env python3
"""Settings page."""

from PyQt6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel,
    QPushButton, QLineEdit, QFrame, QSizePolicy, QMessageBox
)
from PyQt6.QtCore import Qt

from theme import (
    BG_DARK, BG_CARD, FOREST_700, FOREST_600, FOREST_800,
    MOSS_400, TEXT_PRIMARY, TEXT_SECONDARY, TEXT_MUTED,
    EARTH_500, BORDER, FONT_FAMILY,
)


class SettingsPage(QWidget):
    def __init__(self, api, config, auth_manager, parent=None):
        super().__init__(parent)
        self.api = api
        self.config = config
        self.auth_manager = auth_manager
        self._setup_ui()

    def _setup_ui(self):
        layout = QVBoxLayout(self)
        layout.setSpacing(0)
        layout.setContentsMargins(0, 0, 0, 0)

        from PyQt6.QtWidgets import QScrollArea
        scroll = QScrollArea()
        scroll.setWidgetResizable(True)
        scroll.setFrameShape(QFrame.Shape.NoFrame)
        scroll.setHorizontalScrollBarPolicy(Qt.ScrollBarPolicy.ScrollBarAlwaysOff)

        content = QWidget()
        content_layout = QVBoxLayout(content)
        content_layout.setSpacing(24)
        content_layout.setContentsMargins(32, 24, 32, 32)

        title = QLabel("Settings")
        title.setObjectName("pageTitle")
        content_layout.addWidget(title)

        content_layout.addWidget(self._section("Account", [
            self._account_section(),
        ]))

        content_layout.addWidget(self._section("Connection", [
            self._connection_section(),
        ]))

        content_layout.addWidget(self._section("About", [
            self._about_section(),
        ]))

        content_layout.addStretch()

        scroll.setWidget(content)
        layout.addWidget(scroll, stretch=1)

        self.setStyleSheet(f"""
            QLabel#pageTitle {{
                color: {TEXT_PRIMARY};
                font-size: 22px;
                font-weight: 700;
            }}
            QFrame#sectionCard {{
                background-color: {BG_CARD};
                border: 1px solid {BORDER};
                border-radius: 10px;
            }}
            QLabel#sectionTitle {{
                color: {MOSS_400};
                font-size: 13px;
                font-weight: 700;
                text-transform: uppercase;
                letter-spacing: 0.5px;
            }}
        """)

    def _section(self, title: str, widgets: list) -> QFrame:
        card = QFrame()
        card.setObjectName("sectionCard")
        layout = QVBoxLayout(card)
        layout.setSpacing(12)
        layout.setContentsMargins(20, 16, 20, 16)

        label = QLabel(title.upper())
        label.setObjectName("sectionTitle")
        layout.addWidget(label)

        separator = QFrame()
        separator.setFrameShape(QFrame.Shape.HLine)
        separator.setStyleSheet(f"background-color: {BORDER}; max-height: 1px;")
        layout.addWidget(separator)

        for w in widgets:
            layout.addWidget(w)

        return card

    def _account_section(self) -> QWidget:
        widget = QWidget()
        layout = QVBoxLayout(widget)
        layout.setSpacing(10)
        layout.setContentsMargins(0, 0, 0, 0)

        user = self.auth_manager.get_user()
        if user and user.email:
            email_label = QLabel(f"Signed in as {user.email}")
            email_label.setStyleSheet(f"color: {TEXT_PRIMARY}; font-size: 14px;")
            layout.addWidget(email_label)

            if user.is_admin:
                badge = QLabel("Admin")
                badge.setStyleSheet(f"""
                    background-color: {FOREST_800};
                    color: {MOSS_400};
                    padding: 2px 10px;
                    border-radius: 4px;
                    font-size: 11px;
                    font-weight: 700;
                    max-width: 60px;
                """)
                layout.addWidget(badge)

            if self.config.dev_mode:
                dev_badge = QLabel("Dev Mode")
                dev_badge.setStyleSheet(f"""
                    background-color: rgba(188, 108, 37, 0.2);
                    color: #e0a040;
                    padding: 2px 10px;
                    border-radius: 4px;
                    font-size: 11px;
                    font-weight: 700;
                    max-width: 80px;
                """)
                layout.addWidget(dev_badge)

            signout_btn = QPushButton("Sign Out")
            signout_btn.setObjectName("dangerBtn")
            signout_btn.setCursor(Qt.CursorShape.PointingHandCursor)
            signout_btn.clicked.connect(self._on_sign_out)
            layout.addWidget(signout_btn)

            signout_btn.setStyleSheet(f"""
                QPushButton {{
                    background-color: transparent;
                    color: {EARTH_500};
                    border: 1px solid {EARTH_500};
                    border-radius: 6px;
                    padding: 8px 16px;
                    font-size: 12px;
                    max-width: 120px;
                }}
                QPushButton:hover {{
                    background-color: {EARTH_500};
                    color: white;
                }}
            """)
        else:
            not_signed = QLabel("Not signed in")
            not_signed.setStyleSheet(f"color: {TEXT_MUTED}; font-size: 14px;")
            layout.addWidget(not_signed)

        return widget

    def _connection_section(self) -> QWidget:
        widget = QWidget()
        layout = QVBoxLayout(widget)
        layout.setSpacing(10)
        layout.setContentsMargins(0, 0, 0, 0)

        row = QHBoxLayout()
        label = QLabel("API Server")
        label.setStyleSheet(f"color: {TEXT_SECONDARY}; font-size: 13px;")
        label.setMinimumWidth(100)
        row.addWidget(label)

        url_display = QLabel(self.config.api_url)
        url_display.setStyleSheet(f"color: {TEXT_PRIMARY}; font-size: 13px; font-family: monospace;")
        url_display.setTextInteractionFlags(Qt.TextInteractionFlag.TextSelectableByMouse)
        row.addWidget(url_display, stretch=1)
        row.addStretch()

        layout.addLayout(row)

        return widget

    def _about_section(self) -> QWidget:
        widget = QWidget()
        layout = QVBoxLayout(widget)
        layout.setSpacing(8)
        layout.setContentsMargins(0, 0, 0, 0)

        app_label = QLabel("ClipShare Desktop")
        app_label.setStyleSheet(f"color: {TEXT_PRIMARY}; font-size: 14px; font-weight: 600;")
        layout.addWidget(app_label)

        ver_label = QLabel("Version 0.2.0")
        ver_label.setStyleSheet(f"color: {TEXT_MUTED}; font-size: 12px;")
        layout.addWidget(ver_label)

        ffmpeg_label = QLabel(f"FFmpeg: {self.config.get_ffmpeg_cmd()}")
        ffmpeg_label.setStyleSheet(f"color: {TEXT_MUTED}; font-size: 12px;")
        layout.addWidget(ffmpeg_label)

        return widget

    def _on_sign_out(self):
        self.auth_manager.logout()