#!/usr/bin/env python3
"""Build script with FFmpeg download."""

import os
import sys
import platform
import urllib.request
import tarfile
import zipfile
from pathlib import Path


def get_ffmpeg_url():
    """Get FFmpeg download URL for current platform."""
    system = platform.system().lower()
    machine = platform.machine().lower()
    
    if system == 'linux':
        if 'x86_64' in machine or 'amd64' in machine:
            return "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz"
        elif 'aarch64' in machine or 'arm64' in machine:
            return "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-arm64-static.tar.xz"
    elif system == 'darwin':  # macOS
        return "https://evermeet.cx/ffmpeg/ffmpeg-6.1.1.zip"
    elif system == 'windows':
        return "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip"
    
    return None


def download_ffmpeg(url: str, dest_dir: Path):
    """Download and extract FFmpeg."""
    print(f"Downloading FFmpeg from {url}...")
    
    # Create ffmpeg directory
    ffmpeg_dir = dest_dir / 'ffmpeg'
    ffmpeg_dir.mkdir(parents=True, exist_ok=True)
    
    # Download
    archive_path = ffmpeg_dir / 'ffmpeg_archive'
    urllib.request.urlretrieve(url, archive_path)
    
    print("Extracting FFmpeg...")
    
    # Extract
    if url.endswith('.tar.xz'):
        with tarfile.open(archive_path, 'r:xz') as tar:
            tar.extractall(ffmpeg_dir)
            # Find ffmpeg binary
            for member in tar.getmembers():
                if member.name.endswith('/ffmpeg'):
                    extracted = ffmpeg_dir / member.name
                    extracted.rename(ffmpeg_dir / 'ffmpeg')
                    break
    elif url.endswith('.zip'):
        with zipfile.ZipFile(archive_path, 'r') as zip_ref:
            zip_ref.extractall(ffmpeg_dir)
            # Find ffmpeg binary
            for name in zip_ref.namelist():
                if name.endswith('ffmpeg') or name.endswith('ffmpeg.exe'):
                    extracted = ffmpeg_dir / name
                    extracted.rename(ffmpeg_dir / 'ffmpeg')
                    break
    
    # Make executable on Unix
    if platform.system() != 'Windows':
        (ffmpeg_dir / 'ffmpeg').chmod(0o755)
    
    # Clean up
    archive_path.unlink()
    print(f"FFmpeg installed to {ffmpeg_dir / 'ffmpeg'}")


def main():
    """Main build function."""
    script_dir = Path(__file__).parent.absolute()
    
    if '--download-ffmpeg' in sys.argv:
        url = get_ffmpeg_url()
        if url:
            download_ffmpeg(url, script_dir)
        else:
            print(f"Unsupported platform: {platform.system()} {platform.machine()}")
            sys.exit(1)
    else:
        print("Usage: python build.py --download-ffmpeg")
        print("Downloads FFmpeg binary for the current platform")


if __name__ == '__main__':
    main()