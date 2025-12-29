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
    GOOS=$GOOS GOARCH=$GOARCH go build -o "$BUILD_DIR/$OUTPUT_NAME" main.go api.go
    echo "Build complete for $GOOS/$GOARCH"
}

# Function to build shared library for a specific platform
build_shared_library() {
    local GOOS=$1
    local GOARCH=$2
    
    echo "Building shared library for $GOOS/$GOARCH..."
    
    # go build with -buildmode=c-shared outputs both .so/.dylib/.dll and .h files
    # The -o flag sets the base name for both outputs
    # Go automatically adds the correct extension (.so, .dylib, or .dll) and creates a .h file
    OUTPUT_BASE="$BUILD_DIR/libxray_projection_render"
    
    GOOS=$GOOS GOARCH=$GOARCH go build -buildmode=c-shared -o "$OUTPUT_BASE" .
    
    echo "Shared library build complete for $GOOS/$GOARCH"
    echo "  Library: ${OUTPUT_BASE}.* (extension depends on platform)"
    echo "  Header: ${OUTPUT_BASE}.h"
}

# Build for Apple Silicon (darwin/arm64)
build_for_platform "darwin" "arm64" "xray_projection_render_darwin-arm64"

# Build for Windows (windows/amd64)
build_for_platform "windows" "amd64" "xray_projection_render_windows-amd64.exe"

# Build for Linux (linux/amd64)
build_for_platform "linux" "amd64" "xray_projection_render_linux-amd64"

# Build shared library for current platform
echo "Building shared library for current platform..."
CURRENT_OS=$(go env GOOS)
CURRENT_ARCH=$(go env GOARCH)
build_shared_library "$CURRENT_OS" "$CURRENT_ARCH"

echo "All builds completed successfully!"
echo "Build artifacts are in the $BUILD_DIR directory"
echo "Shared library: $BUILD_DIR/$OUTPUT_NAME" 