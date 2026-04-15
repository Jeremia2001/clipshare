#!/usr/bin/env python3
"""Centralized theme constants and stylesheet for ClipShare Qt app."""

# Forest palette (primary)
FOREST_50 = "#f0fdf4"
FOREST_100 = "#dcfce7"
FOREST_200 = "#bbf7d0"
FOREST_300 = "#86efac"
FOREST_400 = "#4ade80"
FOREST_500 = "#22c55e"
FOREST_600 = "#40916c"
FOREST_700 = "#2d6a4f"
FOREST_800 = "#1b4332"
FOREST_900 = "#14532d"

# Moss palette (secondary accents, success)
MOSS_400 = "#74c69d"
MOSS_500 = "#52b788"
MOSS_600 = "#2d6a4f"

# Sand palette (text, borders)
SAND_300 = "#d8f3dc"
SAND_400 = "#b7b7a4"
SAND_500 = "#a5a58d"
SAND_600 = "#6b705c"

# Earth palette (accents, warnings)
EARTH_400 = "#bc6c25"
EARTH_500 = "#9b2226"
EARTH_600 = "#7f1d1d"

# Neutrals
BG_DARK = "#121212"
BG_PANEL = "#1a1a1a"
BG_CARD = "#1e1e1e"
BG_CARD_HOVER = "#252525"
BG_INPUT = "#1a1a1a"
BORDER = "#333"
BORDER_FOCUS = "#40916c"
TEXT_PRIMARY = "#d8f3dc"
TEXT_SECONDARY = "#b7b7a4"
TEXT_MUTED = "#777"
DISABLED_BG = "#555"
DISABLED_TEXT = "#999"
ERROR = "#9b2226"
ERROR_HOVER = "#7f1d1d"

FONT_FAMILY = '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif'
FONT_MONO = '"SF Mono", "Cascadia Code", "Fira Code", Consolas, monospace'

SIDEBAR_WIDTH = 220

STYLESHEET = f"""
QMainWindow {{
    background-color: {BG_DARK};
}}
QWidget {{
    font-family: {FONT_FAMILY};
}}
QLabel {{
    color: {TEXT_PRIMARY};
}}
QPushButton {{
    background-color: {FOREST_700};
    color: white;
    border: none;
    padding: 8px 16px;
    border-radius: 6px;
    font-size: 13px;
    min-height: 20px;
}}
QPushButton:hover {{
    background-color: {FOREST_600};
}}
QPushButton:pressed {{
    background-color: {MOSS_500};
}}
QPushButton:disabled {{
    background-color: {DISABLED_BG};
    color: {DISABLED_TEXT};
}}
QPushButton[class="danger"] {{
    background-color: {ERROR};
}}
QPushButton[class="danger"]:hover {{
    background-color: {ERROR_HOVER};
}}
QPushButton[class="ghost"] {{
    background-color: transparent;
    color: {TEXT_SECONDARY};
    border: 1px solid {BORDER};
}}
QPushButton[class="ghost"]:hover {{
    background-color: {BG_CARD};
    color: {TEXT_PRIMARY};
    border-color: {TEXT_SECONDARY};
}}
QLineEdit, QTextEdit, QPlainTextEdit {{
    background-color: {BG_INPUT};
    color: {TEXT_PRIMARY};
    border: 1px solid {BORDER};
    border-radius: 6px;
    padding: 8px 12px;
    font-size: 13px;
    selection-background-color: {FOREST_700};
}}
QLineEdit:focus, QTextEdit:focus, QPlainTextEdit:focus {{
    border-color: {BORDER_FOCUS};
}}
QCheckBox {{
    color: {TEXT_PRIMARY};
    spacing: 8px;
    font-size: 13px;
}}
QCheckBox::indicator {{
    width: 18px;
    height: 18px;
    border-radius: 4px;
    border: 2px solid {SAND_600};
    background-color: {BG_INPUT};
}}
QCheckBox::indicator:checked {{
    background-color: {FOREST_600};
    border-color: {FOREST_600};
}}
QCheckBox::indicator:hover {{
    border-color: {MOSS_400};
}}
QProgressBar {{
    border: none;
    background-color: {BG_PANEL};
    border-radius: 4px;
    height: 6px;
    min-height: 6px;
    max-height: 6px;
}}
QProgressBar::chunk {{
    background-color: {FOREST_600};
    border-radius: 4px;
}}
QScrollBar:vertical {{
    background-color: {BG_DARK};
    width: 8px;
    border: none;
}}
QScrollBar::handle:vertical {{
    background-color: {BORDER};
    border-radius: 4px;
    min-height: 30px;
}}
QScrollBar::handle:vertical:hover {{
    background-color: {SAND_500};
}}
QScrollBar::add-line:vertical, QScrollBar::sub-line:vertical {{
    height: 0px;
}}
QScrollBar:horizontal {{
    background-color: {BG_DARK};
    height: 8px;
    border: none;
}}
QScrollBar::handle:horizontal {{
    background-color: {BORDER};
    border-radius: 4px;
    min-width: 30px;
}}
QScrollBar::handle:horizontal:hover {{
    background-color: {SAND_500};
}}
QScrollBar::add-line:horizontal, QScrollBar::sub-line:horizontal {{
    width: 0px;
}}
QListWidget {{
    background-color: {BG_DARK};
    border: none;
    outline: none;
}}
QListWidget::item {{
    background-color: {BG_CARD};
    border-radius: 8px;
    margin: 4px 2px;
    padding: 4px;
}}
QListWidget::item:selected {{
    background-color: {FOREST_800};
}}
QListWidget::item:hover {{
    background-color: {BG_CARD_HOVER};
}}
QMessageBox {{
    background-color: {BG_PANEL};
}}
QMessageBox QLabel {{
    color: {TEXT_PRIMARY};
}}
QMessageBox QPushButton {{
    min-width: 80px;
}}
QToolTip {{
    background-color: {BG_CARD};
    color: {TEXT_PRIMARY};
    border: 1px solid {BORDER};
    border-radius: 4px;
    padding: 4px 8px;
}}
QSplitter::handle {{
    background-color: {BORDER};
}}
QGroupBox {{
    color: {TEXT_PRIMARY};
    border: 1px solid {BORDER};
    border-radius: 8px;
    margin-top: 12px;
    padding-top: 16px;
    font-weight: bold;
}}
QGroupBox::title {{
    subcontrol-origin: margin;
    subcontrol-position: top left;
    padding: 0 8px;
    color: {MOSS_400};
}}
"""


def sidebar_stylesheet():
    return f"""
    QFrame {{
        background-color: {BG_PANEL};
        border-right: 1px solid {BORDER};
    }}
    """


def nav_button_stylesheet(checked=False):
    base = f"""
    QPushButton {{
        background-color: transparent;
        color: {TEXT_SECONDARY};
        border: none;
        border-radius: 8px;
        padding: 10px 16px;
        text-align: left;
        font-size: 14px;
        font-weight: normal;
        min-height: 20px;
    }}
    QPushButton:hover {{
        background-color: {FOREST_800};
        color: {TEXT_PRIMARY};
    }}
    """
    if checked:
        base += f"""
        QPushButton {{
            background-color: {FOREST_700};
            color: white;
            font-weight: 600;
        }}
        """
    return base


def card_stylesheet(hover=False):
    return f"""
    QWidget {{
        background-color: {BG_CARD_HOVER if hover else BG_CARD};
        border: 1px solid {BORDER};
        border-radius: 10px;
    }}
    QWidget:hover {{
        background-color: {BG_CARD_HOVER};
        border-color: {SAND_600};
    }}
    """


def format_duration(seconds):
    if seconds <= 0:
        return "0:00"
    m, s = divmod(int(seconds), 60)
    if m >= 60:
        h, m = divmod(m, 60)
        return f"{h}:{m:02d}:{s:02d}"
    return f"{m}:{s:02d}"


def format_size(bytes_size):
    if bytes_size <= 0:
        return "0 B"
    for unit in ["B", "KB", "MB", "GB"]:
        if bytes_size < 1024:
            return f"{bytes_size:.1f} {unit}"
        bytes_size /= 1024
    return f"{bytes_size:.1f} TB"