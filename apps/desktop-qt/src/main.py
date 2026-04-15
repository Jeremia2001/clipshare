#!/usr/bin/env python3
"""Main application entry point."""

import sys
import os

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from PyQt6.QtWidgets import QApplication, QDialog
from PyQt6.QtCore import Qt

from config import Config
from api_client import ClipShareApiClient
from auth_manager import AuthManager
from widgets.main_window import MainWindow
from widgets.login_dialog import LoginDialog
from theme import STYLESHEET


def main():
    if hasattr(Qt, 'AA_EnableHighDpiScaling'):
        QApplication.setAttribute(Qt.ApplicationAttribute.AA_EnableHighDpiScaling, True)
    if hasattr(Qt, 'AA_UseHighDpiPixmaps'):
        QApplication.setAttribute(Qt.ApplicationAttribute.AA_UseHighDpiPixmaps, True)

    app = QApplication(sys.argv)
    app.setApplicationName("ClipShare")
    app.setApplicationVersion("0.2.0")
    app.setStyleSheet(STYLESHEET)

    config = Config()
    api = ClipShareApiClient(config.get_api_base_url())
    auth_manager = AuthManager(api)

    if not auth_manager.is_authenticated():
        login = LoginDialog(auth_manager)
        if login.exec() != QDialog.DialogCode.Accepted:
            return 1

    window = MainWindow(api, config, auth_manager)
    window.show()

    user = auth_manager.get_user()
    if user:
        window.user_label.setText(user.email)
        avatar_label = window.findChild(object, "userAvatar")
        if avatar_label and user.email:
            from PyQt6.QtWidgets import QLabel
            if isinstance(avatar_label, QLabel):
                avatar_label.setText(user.email[0].upper())

    return app.exec()


if __name__ == '__main__':
    sys.exit(main())