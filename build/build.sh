#!/bin/bash

# LRCleaner Build Script for Cross-Platform Compilation
# This script builds LRCleaner for Windows, macOS, and Linux

set -e

echo "LRCleaner Cross-Platform Build Script"
echo "====================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Resolve absolute paths so cd doesn't break relative references
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SRC_DIR="$ROOT_DIR/src"
BUILD_DIR="$ROOT_DIR/dist"
VERSION=$(date +"%Y%m%d_%H%M%S")

# Clean previous builds
echo -e "${YELLOW}Cleaning previous builds...${NC}"
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# Download dependencies
echo -e "${YELLOW}Downloading dependencies...${NC}"
cd "$SRC_DIR"
go mod tidy

# Build for different platforms
echo -e "${YELLOW}Building for multiple platforms...${NC}"

# Windows (amd64)
echo -e "${BLUE}Building for Windows (amd64)...${NC}"
mkdir -p "$BUILD_DIR/Windows"
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$BUILD_DIR/Windows/LRCleaner_amd64.exe" .

# Windows (arm64)
echo -e "${BLUE}Building for Windows (arm64)...${NC}"
GOOS=windows GOARCH=arm64 go build -ldflags="-s -w" -o "$BUILD_DIR/Windows/LRCleaner_arm64.exe" .

# macOS (amd64)
echo -e "${BLUE}Building for macOS (amd64)...${NC}"
mkdir -p "$BUILD_DIR/macOS"
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o "$BUILD_DIR/macOS/LRCleaner_amd64" .

# macOS (arm64 - Apple Silicon)
echo -e "${BLUE}Building for macOS (arm64)...${NC}"
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o "$BUILD_DIR/macOS/LRCleaner_arm64" .

# Linux (amd64)
echo -e "${BLUE}Building for Linux (amd64)...${NC}"
mkdir -p "$BUILD_DIR/GNU Linux"
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$BUILD_DIR/GNU Linux/LRCleaner_amd64" .

# Linux (arm64)
echo -e "${BLUE}Building for Linux (arm64)...${NC}"
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o "$BUILD_DIR/GNU Linux/LRCleaner_arm64" .

# Create release packages
echo -e "${YELLOW}Creating release packages...${NC}"

# Windows package (staging dir uses _pkg_ prefix to avoid case-collision on macOS HFS+)
mkdir -p "$BUILD_DIR/_pkg_windows"
cp "$BUILD_DIR/Windows/LRCleaner_amd64.exe" "$BUILD_DIR/_pkg_windows/LRCleaner.exe"
cp "$BUILD_DIR/Windows/LRCleaner_arm64.exe" "$BUILD_DIR/_pkg_windows/LRCleaner_arm64.exe"
cp "$ROOT_DIR/README.md" "$BUILD_DIR/_pkg_windows/"
(cd "$BUILD_DIR/_pkg_windows" && zip -r "../LRCleaner_Windows_${VERSION}.zip" .)

# macOS package
mkdir -p "$BUILD_DIR/_pkg_macos"
cp "$BUILD_DIR/macOS/LRCleaner_amd64" "$BUILD_DIR/_pkg_macos/LRCleaner"
cp "$BUILD_DIR/macOS/LRCleaner_arm64" "$BUILD_DIR/_pkg_macos/LRCleaner_arm64"
cp "$ROOT_DIR/README.md" "$BUILD_DIR/_pkg_macos/"
tar -czf "$BUILD_DIR/LRCleaner_macOS_${VERSION}.tar.gz" -C "$BUILD_DIR/_pkg_macos" .

# Linux package
mkdir -p "$BUILD_DIR/_pkg_linux"
cp "$BUILD_DIR/GNU Linux/LRCleaner_amd64" "$BUILD_DIR/_pkg_linux/LRCleaner"
cp "$BUILD_DIR/GNU Linux/LRCleaner_arm64" "$BUILD_DIR/_pkg_linux/LRCleaner_arm64"
cp "$ROOT_DIR/README.md" "$BUILD_DIR/_pkg_linux/"
tar -czf "$BUILD_DIR/LRCleaner_Linux_${VERSION}.tar.gz" -C "$BUILD_DIR/_pkg_linux" .

# Show file sizes
echo -e "${GREEN}Build completed successfully!${NC}"
echo -e "${YELLOW}File sizes:${NC}"
echo -e "${BLUE}Windows:${NC}"
ls -lh "$BUILD_DIR/Windows/"*.exe 2>/dev/null | awk '{print $5, $9}'
echo -e "${BLUE}macOS:${NC}"
ls -lh "$BUILD_DIR/macOS/"* 2>/dev/null | awk '{print $5, $9}'
echo -e "${BLUE}GNU Linux:${NC}"
ls -lh "$BUILD_DIR/GNU Linux/"* 2>/dev/null | awk '{print $5, $9}'

echo -e "${YELLOW}Release packages:${NC}"
ls -lh "$BUILD_DIR/"*.zip "$BUILD_DIR/"*.tar.gz 2>/dev/null | awk '{print $5, $9}'

echo -e "${GREEN}All builds completed!${NC}"
echo -e "${BLUE}Executables are in: $BUILD_DIR${NC}"
echo -e "${BLUE}Release packages are ready for distribution${NC}"
