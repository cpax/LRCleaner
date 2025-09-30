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

# Build directory
BUILD_DIR="dist"
VERSION=$(date +"%Y%m%d_%H%M%S")

# Clean previous builds
echo -e "${YELLOW}Cleaning previous builds...${NC}"
rm -rf $BUILD_DIR
mkdir -p $BUILD_DIR

# Download dependencies
echo -e "${YELLOW}Downloading dependencies...${NC}"
cd src
go mod tidy

# Build for different platforms
echo -e "${YELLOW}Building for multiple platforms...${NC}"

# Windows (amd64)
echo -e "${BLUE}Building for Windows (amd64)...${NC}"
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ../$BUILD_DIR/LRCleaner_Windows_amd64.exe .

# Windows (arm64)
echo -e "${BLUE}Building for Windows (arm64)...${NC}"
GOOS=windows GOARCH=arm64 go build -ldflags="-s -w" -o ../$BUILD_DIR/LRCleaner_Windows_arm64.exe .

# macOS (amd64)
echo -e "${BLUE}Building for macOS (amd64)...${NC}"
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o ../$BUILD_DIR/LRCleaner_macOS_amd64 .

# macOS (arm64 - Apple Silicon)
echo -e "${BLUE}Building for macOS (arm64)...${NC}"
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ../$BUILD_DIR/LRCleaner_macOS_arm64 .

# Linux (amd64)
echo -e "${BLUE}Building for Linux (amd64)...${NC}"
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../$BUILD_DIR/LRCleaner_Linux_amd64 .

# Linux (arm64)
echo -e "${BLUE}Building for Linux (arm64)...${NC}"
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ../$BUILD_DIR/LRCleaner_Linux_arm64 .

cd ..

# Create release packages
echo -e "${YELLOW}Creating release packages...${NC}"

# Windows package
mkdir -p $BUILD_DIR/windows
cp $BUILD_DIR/LRCleaner_Windows_amd64.exe $BUILD_DIR/windows/LRCleaner.exe
cp ../docs/README.md $BUILD_DIR/windows/
cd $BUILD_DIR/windows
zip -r ../LRCleaner_Windows_$VERSION.zip .
cd ../..

# macOS package
mkdir -p $BUILD_DIR/macos
cp $BUILD_DIR/LRCleaner_macOS_amd64 $BUILD_DIR/macos/LRCleaner
cp $BUILD_DIR/LRCleaner_macOS_arm64 $BUILD_DIR/macos/LRCleaner_arm64
cp ../docs/README.md $BUILD_DIR/macos/
cd $BUILD_DIR/macos
tar -czf ../LRCleaner_macOS_$VERSION.tar.gz .
cd ../..

# Linux package
mkdir -p $BUILD_DIR/linux
cp $BUILD_DIR/LRCleaner_Linux_amd64 $BUILD_DIR/linux/LRCleaner
cp $BUILD_DIR/LRCleaner_Linux_arm64 $BUILD_DIR/linux/LRCleaner_arm64
cp ../docs/README.md $BUILD_DIR/linux/
cd $BUILD_DIR/linux
tar -czf ../LRCleaner_Linux_$VERSION.tar.gz .
cd ../..

# Show file sizes
echo -e "${GREEN}Build completed successfully!${NC}"
echo -e "${YELLOW}File sizes:${NC}"
ls -lh $BUILD_DIR/*.exe $BUILD_DIR/*_amd64 $BUILD_DIR/*_arm64 2>/dev/null | awk '{print $5, $9}'

echo -e "${YELLOW}Release packages:${NC}"
ls -lh $BUILD_DIR/*.zip $BUILD_DIR/*.tar.gz 2>/dev/null | awk '{print $5, $9}'

echo -e "${GREEN}All builds completed!${NC}"
echo -e "${BLUE}Executables are in the $BUILD_DIR directory${NC}"
echo -e "${BLUE}Release packages are ready for distribution${NC}"
