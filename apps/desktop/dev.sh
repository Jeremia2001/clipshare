#!/bin/bash
set -e

cd "$(dirname "$0")"

echo "Starting ClipShare desktop app in dev mode..."
echo "Stripping snap-injected env vars to prevent WebKit conflicts..."

env -u GTK_EXE_PREFIX \
    -u GTK_IM_MODULE_FILE \
    -u GTK_PATH \
    -u GIO_MODULE_DIR \
    -u LOCPATH \
    -u GSETTINGS_SCHEMA_DIR \
    -u GDK_BACKEND \
    ENV=development \
    CLIPSHARE_DEV=1 \
    wails dev -tags webkit2_41