#!/bin/bash
set -e

cd "$(dirname "$0")"

echo "Building ClipShare desktop app for production..."

# Download ffmpeg binaries if not already present
if [ ! -f "bin/ffmpeg" ] && [ ! -f "bin/ffmpeg.exe" ]; then
  echo "ffmpeg not found in bin/, downloading..."
  ./scripts/download-ffmpeg.sh
fi

# Build the Wails app
env -u GTK_EXE_PREFIX \
    -u GTK_IM_MODULE_FILE \
    -u GTK_PATH \
    -u GIO_MODULE_DIR \
    -u LOCPATH \
    -u GSETTINGS_SCHEMA_DIR \
    -u GDK_BACKEND \
    wails build -tags webkit2_41

# Copy ffmpeg binaries alongside the built binary
BUILD_DIR="build"
if [ -d "$BUILD_DIR" ]; then
  if [ -f "bin/ffmpeg" ]; then
    cp bin/ffmpeg bin/ffprobe "$BUILD_DIR/"
    echo "Copied ffmpeg to $BUILD_DIR/"
  elif [ -f "bin/ffmpeg.exe" ]; then
    cp bin/ffmpeg.exe bin/ffprobe.exe "$BUILD_DIR/"
    echo "Copied ffmpeg.exe to $BUILD_DIR/"
  fi
fi

echo "Build complete: $(ls -la build/ 2>/dev/null | tail -5)"