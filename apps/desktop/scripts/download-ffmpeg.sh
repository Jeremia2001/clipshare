#!/bin/bash
# Download ffmpeg/ffprobe static binaries for bundling with ClipShare desktop releases.
# Usage: ./scripts/download-ffmpeg.sh [platform]
# Platforms: linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, windows-amd64
# If no platform is given, defaults to the current OS/arch.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DESKTOP_DIR="$(dirname "$SCRIPT_DIR")"
FFMPEG_DIR="$DESKTOP_DIR/bin"

# Determine platform
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
  linux) ;;
  darwin) ;;
  windows) ;;
  mingw*|msys*|cygwin*) OS="windows" ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

PLATFORM="${1:-${OS}-${ARCH}}"

case "$PLATFORM" in
  linux-amd64)
    FFMPEG_URL="https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz"
    FFMPEG_ARCHIVE="ffmpeg-amd64-static.tar.xz"
    STRIP_PATH="ffmpeg-*-static"
    BINS="ffmpeg ffprobe"
    EXTRACT_CMD="tar xf"
    ;;
  linux-arm64)
    FFMPEG_URL="https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-arm64-static.tar.xz"
    FFMPEG_ARCHIVE="ffmpeg-arm64-static.tar.xz"
    STRIP_PATH="ffmpeg-*-static"
    BINS="ffmpeg ffprobe"
    EXTRACT_CMD="tar xf"
    ;;
  darwin-amd64|darwin-arm64)
    BREW_PREFIX="$(brew --prefix 2>/dev/null || echo /usr/local)"
    echo "For macOS, install ffmpeg via Homebrew:"
    echo "  brew install ffmpeg"
    echo ""
    echo "For bundling in releases, you can download from: https://evermeet.cx/ffmpeg/"
    echo "  curl -L -o $FFMPEG_DIR/ffmpeg https://evermeet.cx/ffmpeg/getrelease/ffmpeg"
    echo "  curl -L -o $FFMPEG_DIR/ffprobe https://evermeet.cx/ffmpeg/getrelease/ffprobe"
    mkdir -p "$FFMPEG_DIR"
    if command -v ffmpeg &>/dev/null; then
      cp "$(command -v ffmpeg)" "$FFMPEG_DIR/ffmpeg"
      cp "$(command -v ffprobe)" "$FFMPEG_DIR/ffprobe"
      chmod +x "$FFMPEG_DIR/ffmpeg" "$FFMPEG_DIR/ffprobe"
      echo "Copied system ffmpeg to $FFMPEG_DIR"
    else
      echo "ffmpeg not found. Install it with: brew install ffmpeg"
      exit 1
    fi
    exit 0
    ;;
  windows-amd64)
    FFMPEG_URL="https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip"
    FFMPEG_ARCHIVE="ffmpeg-essentials.zip"
    EXTRACT_CMD="unzip"
    ;;
  *)
    echo "Unknown platform: $PLATFORM"
    echo "Supported: linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, windows-amd64"
    exit 1
    ;;
esac

mkdir -p "$FFMPEG_DIR"

echo "Downloading ffmpeg for $PLATFORM..."
echo "  URL: $FFMPEG_URL"

TMPDIR="$(mktemp -d)"
trap "rm -rf '$TMPDIR'" EXIT

cd "$TMPDIR"

if [[ "$FFMPEG_URL" == *.tar.xz ]]; then
  curl -L -o archive.tar.xz "$FFMPEG_URL"
  tar xf archive.tar.xz
  # Find the binaries in the extracted directory
  FFMPEG_BIN="$(find . -name ffmpeg -type f | head -1)"
  FFPROBE_BIN="$(find . -name ffprobe -type f | head -1)"
  if [ -z "$FFMPEG_BIN" ] || [ -z "$FFPROBE_BIN" ]; then
    echo "ERROR: Could not find ffmpeg/ffprobe in archive"
    exit 1
  fi
  cp "$FFMPEG_BIN" "$FFMPEG_DIR/ffmpeg"
  cp "$FFPROBE_BIN" "$FFMPEG_DIR/ffprobe"
  chmod +x "$FFMPEG_DIR/ffmpeg" "$FFMPEG_DIR/ffprobe"
elif [[ "$FFMPEG_URL" == *.zip ]]; then
  curl -L -o archive.zip "$FFMPEG_URL"
  unzip -q archive.zip
  FFMPEG_BIN="$(find . -name ffmpeg.exe -type f | head -1)"
  FFPROBE_BIN="$(find . -name ffprobe.exe -type f | head -1)"
  if [ -z "$FFMPEG_BIN" ] || [ -z "$FFPROBE_BIN" ]; then
    echo "ERROR: Could not find ffmpeg/ffprobe in archive"
    exit 1
  fi
  cp "$FFMPEG_BIN" "$FFMPEG_DIR/ffmpeg.exe"
  cp "$FFPROBE_BIN" "$FFMPEG_DIR/ffprobe.exe"
fi

echo "ffmpeg downloaded to $FFMPEG_DIR"
ls -la "$FFMPEG_DIR"
"$FFMPEG_DIR/ffmpeg" -version | head -3
echo ""
echo "Done! ffmpeg is ready at: $FFMPEG_DIR/ffmpeg"