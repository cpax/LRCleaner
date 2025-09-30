# LRCleaner - LogRhythm Log Source Management Tool

A cross-platform tool for managing LogRhythm log sources with a modern web interface. Analyze and retire log sources with real-time progress tracking.

## Features

- **🌐 Web Interface**: Responsive browser-based UI
- **🚀 Cross-Platform**: Windows, macOS, and Linux support
- **⚡ High Performance**: Handles 10,000+ log sources efficiently
- **📊 Real-time Progress**: Live updates via WebSocket
- **🔍 Analysis Mode**: Test connectivity and identify retirement candidates
- **✅ Retirement Mode**: Retire log sources with optional database backup
- **📁 Export**: CSV export of results
- **🔧 Configuration**: Secure API credential storage

## Quick Start

### Download & Run

1. Download the executable for your platform from `dist/`
2. Run: `./LRCleaner` (Linux/macOS) or `LRCleaner.exe` (Windows)
3. Enter port number (default: 8080) or press Enter
4. Browser opens automatically to the web interface
5. Configure LogRhythm hostname and API key
6. Start analyzing log sources

### Build from Source

**Prerequisites:** Go 1.21+

```bash
# Clone repository
git clone <repository-url>
cd LRCleaner

# Download dependencies
cd src
go mod tidy

# Build
go build -o ../dist/LRCleaner .

# Run
cd ..
./dist/LRCleaner
```

### Cross-Platform Build

```bash
# Linux/macOS
./build/build.sh

# Windows
build\build.bat
```

## Usage

### Configuration

1. Launch application and open `http://localhost:8080`
2. Enter LogRhythm details:
   - **Hostname**: LogRhythm server hostname/IP
   - **API Key**: LogRhythm API key (10+ characters)
   - **Port**: LogRhythm API port (default: 8501)
3. Click "Save Configuration"

Configuration is stored in `config.json` in the executable directory.

### Analysis Mode

1. Click "Analysis" in the sidebar
2. Select cutoff date for log analysis
3. Click "Start Analysis"
4. Monitor real-time progress
5. Review results and export to CSV

**What it does:**
- Fetches all active log sources from LogRhythm
- Filters sources by selected date
- Excludes LogRhythm system sources
- Tests host connectivity with ping
- Displays results in sortable table

### Retirement Mode

1. Click "Operations" in the sidebar
2. Choose backup option (recommended: perform backup)
3. Select cutoff date
4. Click "Analyze Hosts"
5. Review host recommendations
6. Select hosts to retire
7. Click "Execute Retirement"

**What it does:**
- Optional SQL backup of LogRhythmEMDB
- Analyzes hosts and log sources
- Tests host connectivity
- Recommends hosts for retirement
- Retires all log sources for selected hosts
- Updates log source names and status

## Architecture

```
Web Browser ←→ Go Backend ←→ LogRhythm API
     ↓              ↓              ↓
HTML/CSS/JS    HTTP Server    REST API
WebSocket      Background     Authentication
Real-time UI   Processing     Data Access
```

**Backend (Go):**
- HTTP server with REST API
- WebSocket for real-time updates
- Concurrent processing with goroutines
- JSON configuration management

**Frontend (Web):**
- HTML5/CSS3/JavaScript
- Responsive design
- Real-time progress updates

## Performance

- **1,000 log sources**: ~1-2 minutes
- **10,000 log sources**: ~10-15 minutes
- **100,000 log sources**: ~1-2 hours

*Performance depends on network latency and LogRhythm server performance.*

## File Structure

```
LRCleaner/
├── src/                    # Source code
│   ├── main.go            # Main application
│   ├── go.mod             # Go module
│   └── web/               # Web interface
│       ├── index.html     # Main HTML
│       └── static/        # CSS/JS assets
├── build/                 # Build scripts
│   ├── build.sh          # Cross-platform build
│   └── build.bat         # Windows build
├── dist/                  # Built executables
├── docs/                  # Documentation
└── tests/                 # Test files
```

## API Endpoints

- `GET /api/config` - Get configuration
- `POST /api/config` - Update configuration
- `POST /api/analyze` - Start analysis
- `GET /api/jobs/{jobId}` - Get job status
- `GET /ws` - WebSocket connection

## Troubleshooting

**Common Issues:**

1. **"Configuration is invalid"**
   - Verify API key is 10+ characters
   - Check hostname is accessible

2. **"Failed to connect to LogRhythm API"**
   - Verify hostname/IP address
   - Check network connectivity
   - Ensure LogRhythm API is accessible

3. **"Analysis failed"**
   - Check browser console for errors
   - Verify API key permissions
   - Ensure LogRhythm server is responding

**Network Requirements:**
- Outbound HTTPS (port 443) to LogRhythm server
- Outbound ICMP for ping tests
- LogRhythm API (port 8501)
- Local web server (port 8080)

## Development

**Prerequisites:** Go 1.21+, modern browser, Git

**Development Mode:**
```bash
cd src
go run main.go
```

**Build:**
```bash
# Current platform
go build -o ../dist/LRCleaner .

# Specific platform
GOOS=windows GOARCH=amd64 go build -o ../dist/LRCleaner.exe .

# Optimized
go build -ldflags="-s -w" -o ../dist/LRCleaner .
```

## Security Notes

- API keys stored in plain text in config file
- SSL verification disabled for LogRhythm API calls
- Web server runs on localhost only
- No authentication required (local use)

## License

See LICENSE file for details.