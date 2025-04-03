#!/bin/bash

# Exit on error
set -e

# Print commands
set -x

# Create a build directory
BUILD_DIR="build"
mkdir -p "$BUILD_DIR"

# Function to build for a specific platform
build_for_platform() {
    local GOOS=$1
    local GOARCH=$2
    local OUTPUT_NAME=$3
    
    echo "Building for $GOOS/$GOARCH..."
    GOOS=$GOOS GOARCH=$GOARCH go build -o "$BUILD_DIR/$OUTPUT_NAME" main.go
    echo "Build complete for $GOOS/$GOARCH"
}

# Build for Apple Silicon (darwin/arm64)
build_for_platform "darwin" "arm64" "xray_projection_render_darwin-arm64"

# Build for Windows (windows/amd64)
build_for_platform "windows" "amd64" "xray_projection_render_windows-amd64.exe"

# Build for Linux (linux/amd64)
build_for_platform "linux" "amd64" "xray_projection_render_linux-amd64"

echo "All builds completed successfully!"
echo "Build artifacts are in the $BUILD_DIR directory" 