#!/usr/bin/env python3
"""Main application window."""

from PyQt6.QtWidgets import (
    QMainWindow, QWidget, QVBoxLayout, QHBoxLayout,
    QPushButton, QLabel, QStackedWidget, QFrame, QSizePolicy,
    QMessageBox, QSpacerItem, QApplication, QLineEdit, QDialog
)
from PyQt6.QtCore import Qt, QSize, QTimer
from PyQt6.QtGui import QAction, QDesktopServices
from PyQt6.QtCore import QUrl

from theme import (
    BG_DARK, BG_PANEL, BG_CARD, FOREST_700, FOREST_600, FOREST_800,
    MOSS_400, MOSS_500, TEXT_PRIMARY, TEXT_SECONDARY, TEXT_MUTED,
    EARTH_500, EARTH_600, BORDER, FONT_FAMILY, FONT_MONO, STYLESHEET,
    SIDEBAR_WIDTH,
)
from widgets.editor_page import EditorPage
from widgets.library_page import LibraryPage
from widgets.settings_page import SettingsPage


class ShareDialog(QDialog):
    def __init__(self, share_url: str, parent=None):
        super().__init__(parent)
        self.share_url = share_url
        self.setWindowTitle("Share Clip")
        self.setFixedWidth(500)
        self.setWindowFlags(self.windowFlags() & ~Qt.WindowType.WindowContextHelpButtonHint)
        self._setup_ui()
        self._apply_styles()

    def _setup_ui(self):
        layout = QVBoxLayout(self)
        layout.setSpacing(0)
        layout.setContentsMargins(28, 28, 28, 24)

        title = QLabel("Share Clip")
        title.setObjectName("shareTitle")
        layout.addWidget(title)

        layout.addSpacing(6)

        subtitle = QLabel("Anyone with this link can watch the clip in their browser.")
        subtitle.setObjectName("shareSubtitle")
        subtitle.setWordWrap(True)
        layout.addWidget(subtitle)

        layout.addSpacing(20)

        url_row = QHBoxLayout()
        url_row.setSpacing(8)

        self.url_field = QLineEdit(self.share_url)
        self.url_field.setReadOnly(True)
        self.url_field.setObjectName("urlField")
        self.url_field.setMinimumHeight(40)
        url_row.addWidget(self.url_field, stretch=1)

        self.copy_btn = QPushButton("Copy")
        self.copy_btn.setObjectName("copyBtn")
        self.copy_btn.setFixedWidth(80)
        self.copy_btn.setMinimumHeight(40)
        self.copy_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        self.copy_btn.clicked.connect(self._copy_url)
        url_row.addWidget(self.copy_btn)

        layout.addLayout(url_row)

        layout.addSpacing(20)

        btn_row = QHBoxLayout()
        btn_row.setSpacing(8)

        open_btn = QPushButton("Open in Browser")
        open_btn.setObjectName("openBtn")
        open_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        open_btn.clicked.connect(self._open_browser)
        btn_row.addWidget(open_btn)

        btn_row.addStretch()

        done_btn = QPushButton("Done")
        done_btn.setObjectName("doneBtn")
        done_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        done_btn.clicked.connect(self.accept)
        btn_row.addWidget(done_btn)

        layout.addLayout(btn_row)

    def _copy_url(self):
        QApplication.clipboard().setText(self.share_url)
        self.copy_btn.setText("Copied!")
        self.copy_btn.setProperty("copied", True)
        self.copy_btn.style().unpolish(self.copy_btn)
        self.copy_btn.style().polish(self.copy_btn)
        QTimer.singleShot(2000, self._reset_copy_btn)

    def _reset_copy_btn(self):
        self.copy_btn.setText("Copy")
        self.copy_btn.setProperty("copied", False)
        self.copy_btn.style().unpolish(self.copy_btn)
        self.copy_btn.style().polish(self.copy_btn)

    def _open_browser(self):
        QDesktopServices.openUrl(QUrl(self.share_url))

    def _apply_styles(self):
        self.setStyleSheet(f"""
            QDialog {{
                background-color: {BG_PANEL};
            }}
            QLabel#shareTitle {{
                color: {TEXT_PRIMARY};
                font-size: 18px;
                font-weight: 700;
                background: transparent;
            }}
            QLabel#shareSubtitle {{
                color: {TEXT_SECONDARY};
                font-size: 13px;
                background: transparent;
            }}
            QLineEdit#urlField {{
                background-color: #111;
                color: {TEXT_PRIMARY};
                border: 1px solid {BORDER};
                border-radius: 8px;
                padding: 8px 14px;
                font-size: 12px;
                font-family: monospace;
                selection-background-color: {FOREST_700};
            }}
            QPushButton#copyBtn {{
                background-color: {FOREST_700};
                color: white;
                border: none;
                border-radius: 8px;
                font-size: 13px;
                font-weight: 600;
            }}
            QPushButton#copyBtn:hover {{
                background-color: {FOREST_600};
            }}
            QPushButton#copyBtn[copied="true"] {{
                background-color: {MOSS_500};
            }}
            QPushButton#openBtn {{
                background-color: transparent;
                color: {TEXT_SECONDARY};
                border: 1px solid {BORDER};
                border-radius: 8px;
                padding: 9px 16px;
                font-size: 13px;
            }}
            QPushButton#openBtn:hover {{
                background-color: {BG_CARD};
                color: {TEXT_PRIMARY};
                border-color: {TEXT_SECONDARY};
            }}
            QPushButton#doneBtn {{
                background-color: {FOREST_700};
                color: white;
                border: none;
                border-radius: 8px;
                padding: 9px 24px;
                font-size: 13px;
                font-weight: 600;
            }}
            QPushButton#doneBtn:hover {{
                background-color: {FOREST_600};
            }}
        """)


class MainWindow(QMainWindow):
    def __init__(self, api, config, auth_manager):
        super().__init__()
        self.api = api
        self.config = config
        self.auth_manager = auth_manager

        self.setWindowTitle("ClipShare")
        self.setMinimumSize(1100, 700)
        self.resize(1400, 900)

        self._current_page = 0
        self._setup_ui()
        self._apply_styles()

    def _setup_ui(self):
        central = QWidget()
        self.setCentralWidget(central)

        main_layout = QHBoxLayout(central)
        main_layout.setContentsMargins(0, 0, 0, 0)
        main_layout.setSpacing(0)

        sidebar = self._create_sidebar()
        main_layout.addWidget(sidebar)

        self.stack = QStackedWidget()

        self.library_page = LibraryPage(self.api)
        self.library_page.playClip.connect(self._on_play_clip)
        self.library_page.createShare.connect(self._on_create_share)
        self.library_page.newClipRequested.connect(lambda: self._show_page(1))
        self.stack.addWidget(self.library_page)

        self.editor_page = EditorPage(self.api, self.config)
        self.editor_page.clipUploaded.connect(self._on_clip_uploaded)
        self.stack.addWidget(self.editor_page)

        self.settings_page = SettingsPage(self.api, self.config, self.auth_manager)
        self.stack.addWidget(self.settings_page)

        main_layout.addWidget(self.stack, stretch=1)

        self._create_menu()
        self._setup_status_bar()
        self._show_page(0)

    def _setup_status_bar(self):
        sb = self.statusBar()
        sb.setObjectName("appStatusBar")
        sb.setSizeGripEnabled(False)

    def _create_sidebar(self) -> QFrame:
        sidebar = QFrame()
        sidebar.setObjectName("sidebar")
        sidebar.setFixedWidth(SIDEBAR_WIDTH)

        layout = QVBoxLayout(sidebar)
        layout.setContentsMargins(12, 20, 12, 16)
        layout.setSpacing(0)

        logo = QLabel("ClipShare")
        logo.setObjectName("sidebarLogo")
        logo.setAlignment(Qt.AlignmentFlag.AlignCenter)
        layout.addWidget(logo)

        tagline = QLabel("Record · Edit · Share")
        tagline.setObjectName("sidebarTagline")
        tagline.setAlignment(Qt.AlignmentFlag.AlignCenter)
        layout.addWidget(tagline)

        layout.addSpacing(28)

        self.nav_buttons = []

        nav_items = [
            ("Library", "◈", 0),
            ("New Clip", "+", 1),
            ("Settings", "⚙", 2),
        ]

        for label, icon, index in nav_items:
            btn = self._make_nav_button(icon, label, index)
            layout.addWidget(btn)
            layout.addSpacing(2)
            self.nav_buttons.append(btn)

        layout.addStretch()

        sep = QFrame()
        sep.setFrameShape(QFrame.Shape.HLine)
        sep.setObjectName("sidebarSep")
        layout.addWidget(sep)

        layout.addSpacing(12)

        user_container = QWidget()
        user_container.setObjectName("userContainer")
        user_layout = QHBoxLayout(user_container)
        user_layout.setContentsMargins(4, 0, 4, 0)
        user_layout.setSpacing(10)

        avatar = QLabel("U")
        avatar.setObjectName("userAvatar")
        avatar.setAlignment(Qt.AlignmentFlag.AlignCenter)
        avatar.setFixedSize(34, 34)
        user_layout.addWidget(avatar)

        name_layout = QVBoxLayout()
        name_layout.setSpacing(1)

        self.user_label = QLabel("Not signed in")
        self.user_label.setObjectName("userName")
        self.user_label.setWordWrap(True)
        name_layout.addWidget(self.user_label)

        user_role = QLabel("Member")
        user_role.setObjectName("userRole")
        name_layout.addWidget(user_role)

        user_layout.addLayout(name_layout, stretch=1)

        layout.addWidget(user_container)
        layout.addSpacing(8)

        logout_btn = QPushButton("Sign Out")
        logout_btn.setObjectName("logoutBtn")
        logout_btn.setCursor(Qt.CursorShape.PointingHandCursor)
        logout_btn.clicked.connect(self._on_logout)
        layout.addWidget(logout_btn)

        return sidebar

    def _make_nav_button(self, icon: str, label: str, index: int) -> QPushButton:
        btn = QPushButton(f"  {icon}  {label}")
        btn.setObjectName("navBtn")
        btn.setCheckable(True)
        btn.setMinimumHeight(42)
        btn.setCursor(Qt.CursorShape.PointingHandCursor)
        btn.clicked.connect(lambda _, idx=index: self._show_page(idx))
        return btn

    def _create_menu(self):
        menubar = self.menuBar()
        menubar.setObjectName("menuBar")

        file_menu = menubar.addMenu("File")

        new_action = QAction("New Clip", self)
        new_action.setShortcut("Ctrl+N")
        new_action.triggered.connect(lambda: self._show_page(1))
        file_menu.addAction(new_action)

        file_menu.addSeparator()

        quit_action = QAction("Quit", self)
        quit_action.setShortcut("Ctrl+Q")
        quit_action.triggered.connect(self.close)
        file_menu.addAction(quit_action)

        view_menu = menubar.addMenu("View")

        library_action = QAction("Library", self)
        library_action.setShortcut("Ctrl+1")
        library_action.triggered.connect(lambda: self._show_page(0))
        view_menu.addAction(library_action)

        editor_action = QAction("Editor", self)
        editor_action.setShortcut("Ctrl+2")
        editor_action.triggered.connect(lambda: self._show_page(1))
        view_menu.addAction(editor_action)

        settings_action = QAction("Settings", self)
        settings_action.setShortcut("Ctrl+,")
        settings_action.triggered.connect(lambda: self._show_page(2))
        view_menu.addAction(settings_action)

    def _apply_styles(self):
        self.setStyleSheet(STYLESHEET + f"""
            QMenuBar {{
                background-color: {BG_PANEL};
                color: {TEXT_SECONDARY};
                border-bottom: 1px solid {BORDER};
                padding: 2px;
            }}
            QMenuBar::item:selected {{
                background-color: {FOREST_800};
                color: {TEXT_PRIMARY};
            }}
            QMenu {{
                background-color: {BG_PANEL};
                color: {TEXT_PRIMARY};
                border: 1px solid {BORDER};
            }}
            QMenu::item:selected {{
                background-color: {FOREST_800};
            }}
            QStatusBar#appStatusBar {{
                background-color: {FOREST_800};
                color: {MOSS_400};
                font-size: 13px;
                padding: 4px 12px;
                border-top: 1px solid {BORDER};
            }}
            QStatusBar#appStatusBar::item {{
                border: none;
            }}
            QFrame#sidebar {{
                background-color: {BG_PANEL};
                border-right: 1px solid {BORDER};
            }}
            QLabel#sidebarLogo {{
                color: {MOSS_400};
                font-size: 20px;
                font-weight: 800;
                letter-spacing: -0.5px;
                background: transparent;
            }}
            QLabel#sidebarTagline {{
                color: {TEXT_MUTED};
                font-size: 11px;
                margin-top: 2px;
                background: transparent;
            }}
            QPushButton#navBtn {{
                background-color: transparent;
                color: {TEXT_SECONDARY};
                border: none;
                border-radius: 8px;
                padding: 10px 12px;
                text-align: left;
                font-size: 14px;
                font-weight: normal;
            }}
            QPushButton#navBtn:hover {{
                background-color: {FOREST_800};
                color: {TEXT_PRIMARY};
            }}
            QPushButton#navBtn:checked {{
                background-color: {FOREST_700};
                color: white;
                font-weight: 600;
            }}
            QFrame#sidebarSep {{
                color: {BORDER};
                max-height: 1px;
                margin: 0px 8px;
                background-color: {BORDER};
            }}
            QWidget#userContainer {{
                background: transparent;
            }}
            QLabel#userAvatar {{
                background-color: {FOREST_800};
                color: {MOSS_400};
                border-radius: 17px;
                font-size: 14px;
                font-weight: 700;
                border: 1px solid {FOREST_700};
            }}
            QLabel#userName {{
                color: {TEXT_PRIMARY};
                font-size: 12px;
                font-weight: 600;
                background: transparent;
            }}
            QLabel#userRole {{
                color: {TEXT_MUTED};
                font-size: 11px;
                background: transparent;
            }}
            QPushButton#logoutBtn {{
                background-color: transparent;
                color: {TEXT_MUTED};
                border: 1px solid {BORDER};
                border-radius: 6px;
                padding: 6px 12px;
                font-size: 12px;
            }}
            QPushButton#logoutBtn:hover {{
                background-color: rgba(155, 34, 38, 0.15);
                color: {EARTH_500};
                border-color: {EARTH_500};
            }}
        """)

    def update_user_label(self):
        user = self.auth_manager.get_user()
        if user and user.email:
            self.user_label.setText(user.email)
            avatar = user.email[0].upper()
            self.findChild(QLabel, "userAvatar").setText(avatar)

    def _show_page(self, index: int):
        self._current_page = index
        self.stack.setCurrentIndex(index)
        for i, btn in enumerate(self.nav_buttons):
            btn.setChecked(i == index)
        if index == 0:
            self.library_page.load_clips_async()
        elif index == 1:
            self.editor_page.enter_editor_mode()

    def _on_play_clip(self, clip_id: str, view_url: str):
        if view_url:
            if not view_url.startswith("http"):
                view_url = f"{self.api.server_url}{view_url}" if view_url.startswith("/") else f"{self.api.server_url}/{view_url}"
            token = self.api.access_token
            if token:
                sep = "&" if "?" in view_url else "?"
                view_url = f"{view_url}{sep}token={token}"
            self.editor_page.enter_viewer_mode()
            self.editor_page.player.load_url(view_url)
            self.editor_page.player.play()
            self.editor_page.drop_zone.hide()
            self.editor_page.editor_widget.show()
            self._show_page(1)
        else:
            QMessageBox.information(self, "No Preview", "This clip doesn't have a preview available yet.")

    def _on_create_share(self, clip_id: str):
        result = self.api.create_share(clip_id)
        if result:
            share_code = result.get('share_code', '')
            share_url = f"{self.config.get_api_base_url()}/s/{share_code}"
            dlg = ShareDialog(share_url, parent=self)
            dlg.exec()
        else:
            QMessageBox.critical(self, "Error", "Failed to create share link.")

    def _on_clip_uploaded(self, clip_id: str):
        self._show_page(0)
        self.statusBar().showMessage("  Clip uploaded successfully!", 4000)

    def _on_logout(self):
        reply = QMessageBox.question(
            self,
            "Sign Out",
            "Are you sure you want to sign out?",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
            QMessageBox.StandardButton.No,
        )
        if reply == QMessageBox.StandardButton.Yes:
            self.auth_manager.logout()
            self.close()
