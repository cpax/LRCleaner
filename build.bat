@echo off
REM LRCleaner Build Script for Windows
REM This script builds LRCleaner for Windows

setlocal enabledelayedexpansion

echo LRCleaner Windows Build Script
echo ==============================

REM Check if Go is installed
go version >nul 2>&1
if errorlevel 1 (
    echo Go is not installed or not in PATH
    echo Please install Go and try again
    pause
    exit /b 1
)

REM Build directory
set BUILD_DIR=dist
set VERSION=%date:~-4,4%%date:~-10,2%%date:~-7,2%_%time:~0,2%%time:~3,2%%time:~6,2%
set VERSION=%VERSION: =0%

REM Clean previous builds
echo Cleaning previous builds...
if exist %BUILD_DIR% rmdir /s /q %BUILD_DIR%
mkdir %BUILD_DIR%

REM Download dependencies
echo Downloading dependencies...
go mod tidy
if errorlevel 1 (
    echo Failed to download dependencies
    pause
    exit /b 1
)

REM Build for Windows
echo Building for Windows (amd64)...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o %BUILD_DIR%\LRCleaner.exe .
if errorlevel 1 (
    echo Build failed
    pause
    exit /b 1
)

REM Create Windows package
echo Creating Windows package...
mkdir %BUILD_DIR%\windows
copy %BUILD_DIR%\LRCleaner.exe %BUILD_DIR%\windows\
copy LRCleaner.ps1 %BUILD_DIR%\windows\
copy README.md %BUILD_DIR%\windows\

REM Create ZIP file (requires PowerShell)
powershell -command "Compress-Archive -Path '%BUILD_DIR%\windows\*' -DestinationPath '%BUILD_DIR%\LRCleaner_Windows_%VERSION%.zip' -Force"

REM Show file size
echo.
echo Build completed successfully!
echo File size:
dir %BUILD_DIR%\LRCleaner.exe | findstr LRCleaner.exe

echo.
echo Executable created: %BUILD_DIR%\LRCleaner.exe
echo Package created: %BUILD_DIR%\LRCleaner_Windows_%VERSION%.zip
echo.
pause
