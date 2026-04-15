#!/usr/bin/env python3
"""Data models for ClipShare."""

from dataclasses import dataclass, field
from typing import Optional
from theme import format_duration, format_size


@dataclass
class User:
    id: str = ""
    email: str = ""
    username: str = ""
    is_admin: bool = False

    @staticmethod
    def from_dict(data: dict) -> 'User':
        if data is None:
            return User()
        return User(
            id=str(data.get('id', '')),
            email=data.get('email', ''),
            username=data.get('username', ''),
            is_admin=data.get('is_admin', False),
        )

    def to_dict(self) -> dict:
        return {
            'id': self.id,
            'email': self.email,
            'username': self.username,
            'is_admin': self.is_admin,
        }


@dataclass
class Clip:
    id: str = ""
    title: str = "Untitled"
    description: str = ""
    status: str = ""
    view_url: str = ""
    thumbnail_url: str = ""
    duration_seconds: float = 0
    file_size_bytes: int = 0
    width: int = 0
    height: int = 0
    is_public: bool = False
    allow_comments: bool = True
    created_at: str = ""
    share_code: str = ""

    @staticmethod
    def from_dict(data: dict) -> 'Clip':
        if data is None:
            return Clip()
        return Clip(
            id=str(data.get('id', '')),
            title=data.get('title', 'Untitled'),
            description=data.get('description', ''),
            status=data.get('status', ''),
            view_url=data.get('view_url', ''),
            thumbnail_url=data.get('thumbnail_url', ''),
            duration_seconds=float(data.get('duration_seconds', 0)),
            file_size_bytes=int(data.get('file_size_bytes', 0)),
            width=int(data.get('width', 0)),
            height=int(data.get('height', 0)),
            is_public=data.get('is_public', False),
            allow_comments=data.get('allow_comments', True),
            created_at=data.get('created_at', ''),
            share_code=data.get('share_code', ''),
        )

    @property
    def duration_display(self) -> str:
        return format_duration(self.duration_seconds)

    @property
    def size_display(self) -> str:
        return format_size(self.file_size_bytes)

    @property
    def resolution_display(self) -> str:
        if self.width and self.height:
            return f"{self.width}x{self.height}"
        return ""

    @property
    def visibility_badge(self) -> str:
        return "Public" if self.is_public else "Private"


@dataclass
class Share:
    id: str = ""
    clip_id: str = ""
    share_code: str = ""
    url: str = ""
    password: str = ""
    max_views: int = 0
    view_count: int = 0
    created_at: str = ""

    @staticmethod
    def from_dict(data: dict) -> 'Share':
        if data is None:
            return Share()
        return Share(
            id=str(data.get('id', '')),
            clip_id=str(data.get('clip_id', '')),
            share_code=data.get('share_code', ''),
            url=data.get('url', ''),
            password=data.get('password', ''),
            max_views=int(data.get('max_views', 0)),
            view_count=int(data.get('view_count', 0)),
            created_at=data.get('created_at', ''),
        )