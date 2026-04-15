#!/usr/bin/env python3
"""Login dialog for authentication."""

from PyQt6.QtWidgets import (
    QDialog, QVBoxLayout, QHBoxLayout, QLabel,
    QLineEdit, QPushButton, QFrame, QSpacerItem, QSizePolicy
)
from PyQt6.QtCore import Qt

from theme import (
    BG_PANEL, FOREST_700, FOREST_600, MOSS_400, MOSS_500,
    TEXT_PRIMARY, TEXT_SECONDARY, SAND_400, EARTH_500,
    FONT_FAMILY,
)


class LoginDialog(QDialog):
    def __init__(self, auth_manager, parent=None):
        super().__init__(parent)
        self.auth_manager = auth_manager

        self.setWindowTitle("Sign In - ClipShare")
        self.setMinimumWidth(440)
        self.setMinimumHeight(480)
        self.setWindowFlags(self.windowFlags() & ~Qt.WindowType.WindowContextHelpButtonHint)

        self._setup_ui()
        self._apply_styles()

    def _setup_ui(self):
        layout = QVBoxLayout(self)
        layout.setSpacing(0)
        layout.setContentsMargins(48, 48, 48, 32)

        logo = QLabel("ClipShare")
        logo.setObjectName("logoTitle")
        logo.setAlignment(Qt.AlignmentFlag.AlignCenter)
        layout.addWidget(logo)

        tagline = QLabel("Record. Edit. Share.")
        tagline.setObjectName("tagline")
        tagline.setAlignment(Qt.AlignmentFlag.AlignCenter)
        layout.addWidget(tagline)

        layout.addSpacing(36)

        self.email_section = QFrame()
        email_layout = QVBoxLayout(self.email_section)
        email_layout.setSpacing(8)
        email_layout.setContentsMargins(0, 0, 0, 0)

        email_label = QLabel("Email Address")
        email_label.setObjectName("fieldLabel")
        email_layout.addWidget(email_label)

        self.email_input = QLineEdit()
        self.email_input.setPlaceholderText("you@example.com")
        self.email_input.setMinimumHeight(44)
        self.email_input.returnPressed.connect(self._on_submit)
        email_layout.addWidget(self.email_input)

        self.error_label = QLabel("")
        self.error_label.setObjectName("errorLabel")
        self.error_label.setAlignment(Qt.AlignmentFlag.AlignCenter)
        self.error_label.setWordWrap(True)
        email_layout.addWidget(self.error_label)

        self.submit_btn = QPushButton("Send Magic Link")
        self.submit_btn.setObjectName("primaryBtn")
        self.submit_btn.setMinimumHeight(44)
        self.submit_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        self.submit_btn.clicked.connect(self._on_submit)
        email_layout.addWidget(self.submit_btn)

        layout.addWidget(self.email_section)

        self.token_section = QFrame()
        token_layout = QVBoxLayout(self.token_section)
        token_layout.setSpacing(8)
        token_layout.setContentsMargins(0, 0, 0, 0)

        sent_label = QLabel("Check your email for a magic link, then paste the token below.")
        sent_label.setObjectName("infoLabel")
        sent_label.setWordWrap(True)
        sent_label.setAlignment(Qt.AlignmentFlag.AlignCenter)
        token_layout.addWidget(sent_label)

        token_label = QLabel("Token")
        token_label.setObjectName("fieldLabel")
        token_layout.addWidget(token_label)

        self.token_input = QLineEdit()
        self.token_input.setPlaceholderText("Paste your token here...")
        self.token_input.setMinimumHeight(44)
        self.token_input.returnPressed.connect(self._on_verify)
        token_layout.addWidget(self.token_input)

        self.verify_btn = QPushButton("Verify & Sign In")
        self.verify_btn.setObjectName("primaryBtn")
        self.verify_btn.setMinimumHeight(44)
        self.verify_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        self.verify_btn.clicked.connect(self._on_verify)
        token_layout.addWidget(self.verify_btn)

        self.token_section.hide()
        layout.addWidget(self.token_section)

        layout.addSpacing(24)

        layout.addStretch()

        sep = QFrame()
        sep.setFrameShape(QFrame.Shape.HLine)
        sep.setObjectName("separator")
        layout.addWidget(sep)

        layout.addSpacing(16)

        dev_info = QLabel("Running locally? Skip authentication in development mode.")
        dev_info.setObjectName("devInfo")
        dev_info.setAlignment(Qt.AlignmentFlag.AlignCenter)
        dev_info.setWordWrap(True)
        layout.addWidget(dev_info)

        self.dev_btn = QPushButton("Dev Mode (Skip Login)")
        self.dev_btn.setObjectName("devBtn")
        self.dev_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        self.dev_btn.clicked.connect(self._on_dev_mode)
        layout.addWidget(self.dev_btn)

    def _apply_styles(self):
        self.setStyleSheet(f"""
            QDialog {{
                background-color: {BG_PANEL};
            }}
            QLabel#logoTitle {{
                color: {MOSS_400};
                font-size: 32px;
                font-weight: 800;
                font-family: {FONT_FAMILY};
                letter-spacing: -0.5px;
            }}
            QLabel#tagline {{
                color: {TEXT_SECONDARY};
                font-size: 14px;
                margin-top: 4px;
            }}
            QLabel#fieldLabel {{
                color: {TEXT_PRIMARY};
                font-size: 13px;
                font-weight: 600;
            }}
            QLabel#errorLabel {{
                color: {EARTH_500};
                font-size: 12px;
                min-height: 18px;
            }}
            QLabel#infoLabel {{
                color: {MOSS_400};
                font-size: 13px;
                padding: 8px;
                background-color: rgba(45, 106, 79, 0.15);
                border-radius: 6px;
            }}
            QLabel#devInfo {{
                color: {TEXT_SECONDARY};
                font-size: 12px;
            }}
            QPushButton#primaryBtn {{
                background-color: {FOREST_700};
                color: white;
                border: none;
                border-radius: 8px;
                padding: 12px;
                font-size: 14px;
                font-weight: 600;
            }}
            QPushButton#primaryBtn:hover {{
                background-color: {FOREST_600};
            }}
            QPushButton#primaryBtn:pressed {{
                background-color: {MOSS_500};
            }}
            QPushButton#primaryBtn:disabled {{
                background-color: #555;
                color: #999;
            }}
            QPushButton#devBtn {{
                background-color: transparent;
                color: {MOSS_400};
                border: 1px dashed {SAND_400};
                border-radius: 6px;
                padding: 8px 16px;
                font-size: 12px;
            }}
            QPushButton#devBtn:hover {{
                background-color: rgba(45, 106, 79, 0.1);
                border-color: {MOSS_400};
            }}
            QFrame#separator {{
                color: #333;
                max-height: 1px;
            }}
            QLineEdit {{
                background-color: #111;
                color: {TEXT_PRIMARY};
                border: 1px solid #333;
                border-radius: 8px;
                padding: 10px 14px;
                font-size: 14px;
                selection-background-color: {FOREST_700};
            }}
            QLineEdit:focus {{
                border-color: {FOREST_600};
                border-width: 2px;
                padding: 9px 13px;
            }}
        """)

    def _on_submit(self):
        email = self.email_input.text().strip()
        if not email:
            self.error_label.setText("Please enter your email address")
            self.error_label.setStyleSheet(f"color: {EARTH_500};")
            return

        if '@' not in email or '.' not in email:
            self.error_label.setText("Please enter a valid email address")
            self.error_label.setStyleSheet(f"color: {EARTH_500};")
            return

        self.submit_btn.setText("Sending...")
        self.submit_btn.setEnabled(False)

        if self.auth_manager.login(email):
            self.email_section.hide()
            self.token_section.show()
            self.token_input.setFocus()
        else:
            self.error_label.setText("Failed to send magic link. Check your email address or try again.")
            self.error_label.setStyleSheet(f"color: {EARTH_500};")
            self.submit_btn.setText("Send Magic Link")
            self.submit_btn.setEnabled(True)

    def _on_verify(self):
        token = self.token_input.text().strip()
        if not token:
            self.error_label.setText("Please enter the token from your email")
            self.error_label.setStyleSheet(f"color: {EARTH_500};")
            return

        self.verify_btn.setText("Verifying...")
        self.verify_btn.setEnabled(False)

        if self.auth_manager.verify_token(token):
            self.accept()
        else:
            self.error_label.setText("Invalid or expired token. Please try again.")
            self.error_label.setStyleSheet(f"color: {EARTH_500};")
            self.verify_btn.setText("Verify & Sign In")
            self.verify_btn.setEnabled(True)

    def _on_dev_mode(self):
        from models import User
        self.auth_manager._access_token = "dev-token"
        self.auth_manager._user = User(
            id="dev-user", email="dev@localhost", username="dev", is_admin=True
        )
        self.auth_manager.api.set_token("dev-token")
        self.auth_manager._save_tokens()
        self.auth_manager.authChanged.emit(True)
        self.accept()