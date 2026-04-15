#!/usr/bin/env python3
"""HTTP client for ClipShare API."""

import os
import json
import requests
from typing import Optional, Dict, Any

from models import Clip, Share


class ClipShareApiClient:
    """Client for ClipShare REST API."""

    def __init__(self, base_url: str):
        self.base_url = base_url
        if base_url.endswith('/api/v1') or base_url.endswith('/api/v1/'):
            self.server_url = base_url.rsplit('/api/v1', 1)[0]
        else:
            self.server_url = base_url
        self.session = requests.Session()
        self.access_token: Optional[str] = None
        self.refresh_token_value: Optional[str] = None

    def set_token(self, token: str):
        self.access_token = token
        self.session.headers['Authorization'] = f'Bearer {token}'

    def login(self, email: str) -> bool:
        try:
            resp = self.session.post(f"{self.base_url}/auth/login", json={'email': email}, timeout=15)
            return resp.status_code == 200
        except requests.exceptions.ConnectionError:
            print("Connection error: Could not reach server")
            return False
        except requests.exceptions.Timeout:
            print("Request timed out")
            return False
        except Exception as e:
            print(f"Login request failed: {e}")
            return False

    def verify_token(self, token: str) -> Optional[Dict[str, Any]]:
        try:
            resp = self.session.post(f"{self.base_url}/auth/verify", json={'token': token}, timeout=15)
            if resp.status_code == 200:
                data = resp.json()
                self.access_token = data.get('access_token')
                self.refresh_token_value = data.get('refresh_token')
                self.session.headers['Authorization'] = f'Bearer {self.access_token}'
                return data
            return None
        except requests.exceptions.ConnectionError:
            print("Connection error: Could not reach server")
            return None
        except requests.exceptions.Timeout:
            print("Request timed out")
            return None
        except Exception as e:
            print(f"Token verification failed: {e}")
            return None

    def refresh_token(self, refresh_token: str) -> bool:
        try:
            resp = self.session.post(
                f"{self.base_url}/auth/refresh",
                json={'refresh_token': refresh_token},
                timeout=15,
            )
            if resp.status_code == 200:
                data = resp.json()
                new_access = data.get('access_token')
                new_refresh = data.get('refresh_token', refresh_token)
                if new_access:
                    self.access_token = new_access
                    self.refresh_token_value = new_refresh
                    self.session.headers['Authorization'] = f'Bearer {new_access}'
                return True
            return False
        except Exception as e:
            print(f"Token refresh failed: {e}")
            return False

    def get_clips(self, page: int = 1, per_page: int = 50) -> Optional[Dict]:
        try:
            print(f"[API] GET {self.base_url}/clips")
            resp = self.session.get(
                f"{self.base_url}/clips",
                params={'page': page, 'per_page': per_page},
                timeout=15,
            )
            print(f"[API] Response: status={resp.status_code}")
            if resp.status_code == 200:
                data = resp.json()
                clips = data.get('clips', [])
                for clip in clips:
                    view_url = clip.get('view_url', '')
                    if view_url and view_url.startswith('/'):
                        clip['view_url'] = f"{self.server_url}{view_url}"
                return data
            else:
                print(f"[API] Error body: {resp.text[:200]}")
            return None
        except Exception as e:
            print(f"Get clips failed: {e}")
            import traceback
            traceback.print_exc()
            return None

    def get_clip(self, clip_id: str) -> Optional[Dict]:
        try:
            resp = self.session.get(f"{self.base_url}/clips/{clip_id}", timeout=15)
            if resp.status_code == 200:
                data = resp.json()
                clip_data = data.get('clip', data)
                view_url = clip_data.get('view_url', '')
                if view_url and view_url.startswith('/'):
                    data['view_url'] = f"{self.server_url}{view_url}"
                    if 'clip' in data:
                        data['clip']['view_url'] = data['view_url']
                return data
            return None
        except Exception as e:
            print(f"Get clip failed: {e}")
            return None

    def upload_clip(
        self,
        file_path: str,
        title: str,
        description: str = "",
        is_public: bool = True,
        allow_comments: bool = True,
        trim_start: float = 0,
        trim_end: float = 0,
        duration: float = 0,
        width: int = 0,
        height: int = 0,
    ) -> Optional[Dict]:
        try:
            with open(file_path, 'rb') as f:
                files = {'file': (os.path.basename(file_path), f, 'video/mp4')}
                resp = self.session.post(
                    f"{self.base_url}/clips/upload", files=files, timeout=300,
                )

            if resp.status_code != 200:
                print(f"Upload failed: {resp.text}")
                return None

            upload_data = resp.json()
            clip_id = upload_data['clip']['id']

            finalize_data = {
                'title': title,
                'description': description or None,
                'original_filename': os.path.basename(file_path),
                'file_size_bytes': os.path.getsize(file_path),
                'duration_seconds': int(duration),
                'width': width,
                'height': height,
                'is_public': is_public,
                'allow_comments': allow_comments,
                'trim_start_seconds': trim_start,
                'trim_end_seconds': trim_end,
            }

            resp = self.session.post(
                f"{self.base_url}/clips/{clip_id}/finalize",
                json=finalize_data,
                timeout=30,
            )
            if resp.status_code == 200:
                return resp.json()
            return None

        except FileNotFoundError:
            print(f"File not found: {file_path}")
            return None
        except Exception as e:
            print(f"Upload failed: {e}")
            return None

    def delete_clip(self, clip_id: str) -> bool:
        try:
            resp = self.session.delete(f"{self.base_url}/clips/{clip_id}", timeout=15)
            return resp.status_code == 200
        except Exception as e:
            print(f"Delete failed: {e}")
            return False

    def create_share(self, clip_id: str, password: str = None, max_views: int = None) -> Optional[Dict]:
        try:
            data = {}
            if password:
                data['password'] = password
            if max_views:
                data['max_views'] = max_views

            resp = self.session.post(
                f"{self.base_url}/clips/{clip_id}/shares",
                json=data or {},
                timeout=15,
            )
            if resp.status_code == 200:
                return resp.json()
            return None
        except Exception as e:
            print(f"Create share failed: {e}")
            return None