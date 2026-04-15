# ClipShare Qt Desktop Application

A Qt6-based desktop application for ClipShare with native video playback and FFmpeg integration.

## Architecture

- **Python 3.10+** with PyQt6
- **FFmpeg** bundled as static binary (no system dependency)
- **QMediaPlayer** for video preview
- **QProcess** for FFmpeg trimming/processing

## Directory Structure

```
apps/desktop-qt/
├── src/
│   ├── main.py              # Application entry point
│   ├── config.py            # Configuration management
│   ├── api_client.py        # HTTP client for Go backend
│   ├── auth_manager.py      # Authentication handling
│   ├── ffmpeg_wrapper.py    # FFmpeg process wrapper
│   ├── widgets/
│   │   ├── main_window.py    # Main window
│   │   ├── video_player.py   # Video player with trim controls
│   │   ├── editor_page.py    # Clip editing page
│   │   ├── library_page.py   # Clip library browser
│   │   ├── login_dialog.py   # Auth dialog
│   │   └── share_dialog.py   # Share creation dialog
│   └── models/
│       ├── clip.py           # Clip data model
│       ├── share.py          # Share data model
│       └── user.py           # User data model
├── ffmpeg/                  # Bundled FFmpeg binaries
│   └── ffmpeg               # Static FFmpeg binary
├── resources/               # App resources
│   ├── icons/
│   ├── styles/
│   └── fonts/
├── requirements.txt
├── setup.py                 # Build/packaging script
├── build.py                 # Build script with FFmpeg download
└── README.md
```

## FFmpeg Bundling

The application includes a static FFmpeg binary downloaded at build time:
- Linux: `https://johnvansickle.com/ffmpeg/` (static builds)
- Windows: `https://www.gyan.dev/ffmpeg/builds/` (static builds)
- macOS: `https://evermeet.cx/ffmpeg/` (static builds)

FFmpeg is invoked via QProcess, enabling:
- Fast hardware-accelerated encoding (VAAPI, NVENC, etc.)
- No browser/WASM limitations
- Direct file system access
- Streaming upload capability

## Development

```bash
# Setup
python -m venv venv
source venv/bin/activate
pip install -r requirements.txt

# Download FFmpeg
python build.py --download-ffmpeg

# Run
python src/main.py

# Build executable
python setup.py build
```

## Features

- Native video playback (no codec issues)
- Client-side trimming with FFmpeg
- Hardware-accelerated encoding when available
- Proper progress reporting
- Background upload with resume capability