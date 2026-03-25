#!/bin/bash
# VisionLoop Build Script
set -e

echo "Building VisionLoop..."

# Build Go server
echo "Building Go server..."
go build -ldflags="-s -w" -o VisionLoop.exe ./cmd/server

# Build Python detection (optional - requires python environment)
if command -v python3 &> /dev/null; then
    echo "Building Python detection executable..."
    cd detection
    if command -v pyinstaller &> /dev/null; then
        pyinstaller --onefile --name visionloop_det main.py
    else
        echo "PyInstaller not found, skipping detection build"
    fi
    cd ..
fi

echo "Build complete!"
echo "Run VisionLoop.exe to start the server"
