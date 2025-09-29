# LRCleaner - LogRhythm Log Source Management Tool

A modern, cross-platform tool for managing LogRhythm log sources, built with Go and featuring a beautiful web-based interface. This tool provides both test and apply modes for analyzing and retiring log sources, optimized for handling large-scale log source analysis (10,000+ sources).

## Features

- **🌐 Modern Web Interface**: Beautiful, responsive web UI accessible via browser
- **🚀 Cross-Platform**: Single executable for Windows, macOS, and Linux
- **⚡ High Performance**: Optimized for processing 10,000-100,000+ log sources
- **📊 Real-time Progress**: Live progress updates via WebSocket
- **🔍 Test Mode**: Analyze log sources and test connectivity with ping
- **✅ Apply Mode**: Retire log sources based on CSV input
- **📁 Data Export**: Export results to CSV format
- **🔧 Configuration Management**: Secure storage of API credentials
- **🔄 Background Processing**: Non-blocking operations with concurrent processing
- **📱 Responsive Design**: Works on desktop, tablet, and mobile devices

## Quick Start

### Option 1: Download Pre-built Executable

1. **Download** the appropriate executable for your platform from the releases
2. **Run** the executable: `./LRCleaner` (Linux/macOS) or `LRCleaner.exe` (Windows)
3. **Enter a port number** when prompted (or press Enter for default port 8080)
4. **Browser opens automatically** to the correct URL
5. **Configure** your LogRhythm hostname and API key
6. **Start analyzing** log sources!

### Option 2: Build from Source

#### Prerequisites

- **Go 1.21 or higher** - [Download Go](https://golang.org/dl/)
- **Git** (for cloning the repository)

#### Build Instructions

1. **Clone the repository:**
   ```bash
   git clone <repository-url>
   cd LRCleaner-1
   ```

2. **Download dependencies:**
   ```bash
   go mod tidy
   ```

3. **Build for your platform:**
   ```bash
   # Linux/macOS
   go build -o LRCleaner .
   
   # Windows
   go build -o LRCleaner.exe .
   ```

4. **Run the application:**
   ```bash
   # Linux/macOS
   ./LRCleaner
   
   # Windows
   LRCleaner.exe
   ```

5. **Automatic port selection** - the application finds an available port automatically
6. **Browser opens automatically** to the correct URL

#### Cross-Platform Build

Use the provided build scripts to create executables for all platforms:

```bash
# Linux/macOS
./build.sh

# Windows
build.bat
```

This will create executables for:
- Windows (amd64, arm64)
- macOS (amd64, arm64)
- Linux (amd64, arm64)

## Usage

### Command Line Options

You can specify the port as a command line argument:

```bash
# Try to use port 3000 (will find alternative if not available)
./LRCleaner 3000

# Try to use port 9000 (will find alternative if not available)
./LRCleaner 9000
```

**Automatic Port Selection:**
- If no port is specified, tries port 8080 first
- If 8080 is in use, searches for available ports starting from 3000
- If a command line port is specified but not available, finds an alternative
- Displays the selected port in the console

### Configuration

1. **Launch the application** and open `http://localhost:8080` in your browser
2. **Enter your LogRhythm details:**
   - **Hostname**: Your LogRhythm server hostname/IP
   - **API Key**: Your LogRhythm API key (must be 400+ characters)
   - **Port**: LogRhythm API port (default: 8501)
3. **Click "Save Configuration"**

The configuration is stored securely in `LRCleanerConfig.json` in the same directory as the executable.

### Test Mode (Analysis)

1. **Click "Test Mode"** button
2. **Select a date** for the last log message cutoff
3. **Click "Start Analysis"**
4. **Monitor progress** in real-time
5. **Review results** in the table
6. **Export results** to CSV if needed

**What Test Mode Does:**
- Fetches all active log sources from LogRhythm
- Filters sources by the selected date (sources with no logs after this date)
- Excludes LogRhythm system sources (Open Collector, Echo, AI Engine, etc.)
- Tests connectivity to each host with ping
- Displays results in a sortable, filterable table

### Apply Mode (Retirement)

1. **Click "Apply Mode"** button
2. **Choose backup option**:
   - **Perform Backup**: Enter logrhythmadmin password and backup location
   - **Skip Backup**: Proceed without backup (not recommended)
3. **Select a date** for the last log message cutoff
4. **Click "Analyze Hosts"** to scan for retirement candidates
5. **Review the host list** with recommendations
6. **Select hosts** to retire (individual checkboxes or bulk selection)
7. **Click "Execute Retirement"** to retire all log sources for selected hosts
8. **View congratulations screen** with summary and export options

**What Apply Mode Does:**
- **Optional SQL Backup**: Creates a backup of LogRhythmEMDB before making changes
- Analyzes all hosts and their log sources
- Groups log sources by host for easier management
- Tests host connectivity with ping
- Recommends hosts for retirement based on:
  - Failed ping tests
  - No recent log activity (30+ days)
- Provides a user-friendly interface to select hosts
- Retires all log sources for selected hosts
- Updates log source names to include "Retired by LRCleaner"
- Sets record status to "Retired"
- Shows congratulations screen with detailed summary
- Allows export of retirement report as text file

### SQL Database Backup

**Requirements:**
- LogRhythmEMDB database accessible on localhost
- logrhythmadmin user account with backup permissions
- SQL Server instance running

**Backup Process:**
- Connects to LogRhythmEMDB using provided credentials
- Creates timestamped backup file (.bak format)
- Default location: `C:\LogRhythm\Backup`
- Custom location can be specified
- Backup includes full database with statistics

**Backup File Format:**
- Filename: `LogRhythmEMDB_backup_YYYYMMDD_HHMMSS.bak`
- Location: User-specified directory (default: `C:\LogRhythm\Backup`)

### CSV Export

- **Click "Export Results"** to save analysis results
- **Results include**: ID, HostID, HostName, MaxLogDate, PingResult
- **File format**: CSV with headers

## Architecture

### Backend (Go)
- **HTTP Server**: Embedded web server on port 8080
- **REST API**: RESTful endpoints for all operations
- **WebSocket**: Real-time progress updates
- **Concurrent Processing**: Goroutines for parallel operations
- **Configuration**: JSON-based configuration management

### Frontend (Web)
- **HTML5**: Modern semantic markup
- **CSS3**: Responsive design with gradients and animations
- **JavaScript**: Vanilla JS with WebSocket support
- **Font Awesome**: Beautiful icons
- **Responsive**: Works on all device sizes

### Key Components

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Web Browser   │◄──►│   Go Backend    │◄──►│  LogRhythm API  │
│                 │    │                 │    │                 │
│ • HTML/CSS/JS   │    │ • HTTP Server   │    │ • REST API      │
│ • WebSocket     │    │ • WebSocket     │    │ • Authentication│
│ • Real-time UI  │    │ • Background    │    │ • Data Access   │
└─────────────────┘    │   Processing    │    └─────────────────┘
                       └─────────────────┘
```

## Performance

### Large Scale Processing

The Go version is optimized for handling large numbers of log sources:

- **Concurrent API calls**: Uses goroutines for parallel processing
- **Connection pooling**: Efficient HTTP client with connection reuse
- **Background processing**: Non-blocking operations
- **Real-time updates**: WebSocket for live progress
- **Memory efficient**: Processes data in batches
- **Error handling**: Robust error recovery and logging

### Typical Performance

- **1,000 log sources**: ~1-2 minutes
- **10,000 log sources**: ~10-15 minutes
- **100,000 log sources**: ~1-2 hours

*Performance depends on network latency, LogRhythm server performance, and host ping response times.*

## File Structure

```
LRCleaner-1/
├── main.go                 # Main Go application
├── go.mod                  # Go module definition
├── web/                    # Web UI files
│   ├── index.html         # Main HTML page
│   └── static/            # Static assets
│       ├── style.css      # CSS styles
│       └── script.js      # JavaScript functionality
├── build.sh               # Cross-platform build script (Linux/macOS)
├── build.bat              # Windows build script
├── LRCleaner.ps1          # Original PowerShell script (preserved)
├── README.md              # This file
├── LRCleanerConfig.json   # Configuration file (created on first run)
└── dist/                  # Build output directory
    ├── LRCleaner.exe      # Windows executable
    ├── LRCleaner          # Linux/macOS executable
    └── *.zip, *.tar.gz    # Release packages
```

## Configuration

### Configuration File

The tool creates a `LRCleanerConfig.json` file with the following structure:

```json
{
  "hostname": "your_logrhythm_server",
  "apiKey": "your_api_key_here",
  "port": 8501,
  "excludedLogSources": [
    ".*logrhythm.*",
    ".*Open.*Collector.*",
    ".*echo.*",
    ".*AI Engine.*",
    "LogRhythm.*"
  ]
}
```

### Environment Variables

You can also set configuration via environment variables:

```bash
export LRCLEANER_HOSTNAME="your-server.com"
export LRCLEANER_API_KEY="your-api-key"
export LRCLEANER_PORT="8501"
```

## API Endpoints

The application provides a REST API for programmatic access:

- `GET /api/config` - Get current configuration
- `POST /api/config` - Update configuration
- `POST /api/test` - Start test mode analysis
- `POST /api/apply` - Start apply mode (retire log sources)
- `GET /api/export/{jobId}` - Export results as CSV
- `GET /api/jobs/{jobId}` - Get job status
- `GET /ws` - WebSocket connection for real-time updates

## Troubleshooting

### Common Issues

1. **"Configuration is invalid"**
   - Ensure your API key is at least 400 characters long
   - Verify the hostname is correct and accessible

2. **"Failed to connect to LogRhythm API"**
   - Check your hostname/IP address
   - Verify network connectivity
   - Ensure LogRhythm API is accessible on the specified port

3. **"Analysis failed"**
   - Check the browser console for detailed error messages
   - Verify your API key has the necessary permissions
   - Ensure the LogRhythm server is responding

4. **WebSocket connection issues**
   - The application will automatically reconnect
   - Check if port 8080 is available
   - Ensure no firewall is blocking the connection

### Network Requirements

- **Outbound HTTPS**: Port 443 to LogRhythm server
- **Outbound ICMP**: For ping tests (may be blocked by firewalls)
- **LogRhythm API**: Port 8501 (default)
- **Local Web Server**: Port 8080 (configurable)

### Logs

- **Application logs**: Check the terminal/console output
- **Browser logs**: Open Developer Tools (F12) for detailed error messages
- **Network logs**: Check the Network tab in Developer Tools

## Development

### Prerequisites

- Go 1.21+
- Modern web browser
- Git

### Running in Development

1. **Clone and setup:**
   ```bash
   git clone <repository-url>
   cd LRCleaner-1
   go mod tidy
   ```

2. **Run in development mode:**
   ```bash
   go run main.go
   ```

3. **Open browser** to `http://localhost:8080`

### Building

```bash
# Build for current platform
go build -o LRCleaner .

# Build for specific platform
GOOS=windows GOARCH=amd64 go build -o LRCleaner.exe .

# Build with optimizations
go build -ldflags="-s -w" -o LRCleaner .
```

### Code Structure

- **`main.go`**: Main application with HTTP server and API endpoints
- **`web/`**: Frontend files (HTML, CSS, JavaScript)
- **`go.mod`**: Go module dependencies
- **Build scripts**: Cross-platform compilation

## Security Notes

- **API Key Storage**: API keys are stored in plain text in the config file
- **SSL Verification**: Disabled for LogRhythm API calls (common in enterprise environments)
- **Local Server**: Web server runs on localhost only
- **No Authentication**: Web interface has no authentication (runs locally)

## Comparison with PowerShell Version

| Feature | PowerShell | Go Version |
|---------|------------|------------|
| **Platform** | Windows only | Cross-platform |
| **UI** | Windows Forms | Modern web UI |
| **Performance** | Good | Excellent |
| **Concurrency** | Limited | High (goroutines) |
| **Real-time Updates** | Basic | WebSocket |
| **File Size** | ~5MB | ~10-15MB |
| **Dependencies** | PowerShell | None (static binary) |
| **Maintenance** | Complex | Simple |

## License

See LICENSE file for details.

## Support

For issues or questions:

1. **Check the browser console** for error messages
2. **Verify configuration** is correct
3. **Test network connectivity** to LogRhythm server
4. **Review this README** for common solutions
5. **Check the terminal output** for detailed logs

## Version History

- **v2.0.0**: Go rewrite with web interface
- **v1.0.0**: Original PowerShell script