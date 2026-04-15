#!/usr/bin/env python3
"""Authentication manager."""

import json
from pathlib import Path
from typing import Optional

from PyQt6.QtCore import QObject, pyqtSignal

from models import User


class AuthManager(QObject):
    authChanged = pyqtSignal(bool)

    def __init__(self, api_client):
        super().__init__()
        self.api = api_client
        self._access_token: Optional[str] = None
        self._refresh_token: Optional[str] = None
        self._user: Optional[User] = None

        self._load_tokens()

    def _get_token_path(self) -> Path:
        config_dir = Path.home() / '.config' / 'clipshare'
        config_dir.mkdir(parents=True, exist_ok=True)
        return config_dir / 'tokens.json'

    def _load_tokens(self):
        token_path = self._get_token_path()
        if token_path.exists():
            try:
                with open(token_path, 'r') as f:
                    data = json.load(f)
                    self._access_token = data.get('access_token')
                    self._refresh_token = data.get('refresh_token')
                    self._user = User.from_dict(data.get('user'))

                if self._access_token:
                    self.api.set_token(self._access_token)
                    self.authChanged.emit(True)
            except Exception:
                pass

    def _save_tokens(self):
        token_path = self._get_token_path()
        data = {
            'access_token': self._access_token,
            'refresh_token': self._refresh_token,
            'user': self._user.to_dict() if self._user else None,
        }
        try:
            with open(token_path, 'w') as f:
                json.dump(data, f)
        except Exception:
            pass

    def is_authenticated(self) -> bool:
        return self._access_token is not None

    def get_user(self) -> Optional[User]:
        return self._user

    def login(self, email: str) -> bool:
        return self.api.login(email)

    def verify_token(self, token: str) -> bool:
        result = self.api.verify_token(token)
        if result:
            self._access_token = result.get('access_token')
            self._refresh_token = result.get('refresh_token')
            self._user = User.from_dict(result.get('user'))
            self._save_tokens()
            self.authChanged.emit(True)
            return True
        return False

    def refresh(self) -> bool:
        if not self._refresh_token:
            return False

        if self.api.refresh_token(self._refresh_token):
            self._access_token = self.api.access_token
            self._refresh_token = self.api.refresh_token_value
            self._save_tokens()
            return True
        return False

    def logout(self):
        self._access_token = None
        self._refresh_token = None
        self._user = None

        token_path = self._get_token_path()
        if token_path.exists():
            token_path.unlink()

        self.authChanged.emit(False)