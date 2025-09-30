package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	_ "github.com/microsoft/go-mssqldb"
)

// Configuration structure
type Config struct {
	Hostname           string         `json:"hostname"`
	APIKey             string         `json:"apiKey"`
	Port               int            `json:"port"`
	ExcludedLogSources []string       `json:"excludedLogSources"`
	Rollback           RollbackConfig `json:"rollback"`
}

// LogRhythm API structures
type LogSource struct {
	ID                interface{}   `json:"id"` // Can be string or number
	Name              string        `json:"name"`
	RecordStatus      string        `json:"recordStatus"`
	MaxLogDate        string        `json:"maxLogDate"`
	Host              Host          `json:"host"`
	LogSourceType     LogSourceType `json:"logSourceType"`
	SystemMonitorID   interface{}   `json:"systemMonitorId"`   // Collection host ID
	SystemMonitorName string        `json:"systemMonitorName"` // Collection host name
	Recommended       bool          `json:"recommended"`
}

type Host struct {
	ID   interface{} `json:"id"` // Can be string or number
	Name string      `json:"name"`
}

type LogSourceType struct {
	Name string `json:"name"`
}

type AnalysisResult struct {
	ID            interface{} `json:"id"`     // Can be string or number
	HostID        interface{} `json:"hostId"` // Can be string or number
	HostName      string      `json:"hostName"`
	Name          string      `json:"name"`          // Log source name
	LogSourceType string      `json:"logSourceType"` // Log source type name
	MaxLogDate    string      `json:"maxLogDate"`
	PingResult    string      `json:"pingResult"`
}

type HostAnalysis struct {
	HostID         interface{} `json:"hostId"` // Can be string or number
	HostName       string      `json:"hostName"`
	LogSourceCount int         `json:"logSourceCount"`
	MaxLogDate     string      `json:"maxLogDate"`
	PingResult     string      `json:"pingResult"`
	Recommended    bool        `json:"recommended"`
	LogSources     []LogSource `json:"logSources"`
}

type CollectionHostAnalysis struct {
	SystemMonitorID   interface{} `json:"systemMonitorId"` // Can be string or number
	SystemMonitorName string      `json:"systemMonitorName"`
	LogSourceCount    int         `json:"logSourceCount"`
	PingResult        string      `json:"pingResult"`
	Recommended       bool        `json:"recommended"`
	LogSources        []LogSource `json:"logSources"`
}

type ApplyRequest struct {
	SelectedHosts []string `json:"selectedHosts"`
}

type BackupRequest struct {
	Password string `json:"password"`
	Location string `json:"location"`
}

type RetirementRecord struct {
	LogSourceID    interface{} `json:"logSourceId"` // Can be string or number
	HostID         interface{} `json:"hostId"`      // Can be string or number
	HostName       string      `json:"hostName"`
	OriginalName   string      `json:"originalName"`
	RetiredName    string      `json:"retiredName"`
	OriginalStatus string      `json:"originalStatus"`
	RetiredStatus  string      `json:"retiredStatus"`
	Timestamp      time.Time   `json:"timestamp"`
}

// Rollback data structures
type RollbackData struct {
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	OperationType string    `json:"operationType"` // "retirement", "host_retirement", etc.
	User          string    `json:"user"`          // Who performed the operation
	Description   string    `json:"description"`   // Human-readable description

	// Log Source Changes
	LogSourceChanges []LogSourceRollback `json:"logSourceChanges"`

	// Host Changes
	HostChanges []HostRollback `json:"hostChanges"`

	// System Monitor Changes
	SystemMonitorChanges []SystemMonitorRollback `json:"systemMonitorChanges"`

	// Metadata
	JobID          string `json:"jobId"`
	BackupLocation string `json:"backupLocation,omitempty"`
	Checksum       string `json:"checksum"` // For integrity verification
}

type LogSourceRollback struct {
	LogSourceID     interface{} `json:"logSourceId"`
	HostID          interface{} `json:"hostId"`
	HostName        string      `json:"hostName"`
	OriginalName    string      `json:"originalName"`
	OriginalStatus  string      `json:"originalStatus"`
	CurrentName     string      `json:"currentName"`
	CurrentStatus   string      `json:"currentStatus"`
	SystemMonitorID interface{} `json:"systemMonitorId,omitempty"`
}

type HostRollback struct {
	HostID              interface{}      `json:"hostId"`
	HostName            string           `json:"hostName"`
	OriginalName        string           `json:"originalName"`
	OriginalStatus      string           `json:"originalStatus"`
	OriginalIdentifiers []HostIdentifier `json:"originalIdentifiers"`
	RetiredIdentifiers  []HostIdentifier `json:"retiredIdentifiers"` // Only identifiers that were actually retired
	CurrentName         string           `json:"currentName"`
	CurrentStatus       string           `json:"currentStatus"`
}

type SystemMonitorRollback struct {
	SystemMonitorID     interface{} `json:"systemMonitorId"`
	SystemMonitorName   string      `json:"systemMonitorName"`
	OriginalStatus      string      `json:"originalStatus"`
	OriginalLicenseType string      `json:"originalLicenseType"`
	CurrentStatus       string      `json:"currentStatus"`
	CurrentLicenseType  string      `json:"currentLicenseType"`
}

type HostIdentifier struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type RollbackConfig struct {
	Enabled           bool   `json:"enabled"`
	RetentionDays     int    `json:"retentionDays"`
	MaxRollbackPoints int    `json:"maxRollbackPoints"`
	AutoBackup        bool   `json:"autoBackup"`
	BackupLocation    string `json:"backupLocation"`
	ChecksumAlgorithm string `json:"checksumAlgorithm"`
}

type JobStatus struct {
	ID                     string                   `json:"id"`
	Status                 string                   `json:"status"`
	Progress               int                      `json:"progress"`
	Message                string                   `json:"message"`
	Results                []AnalysisResult         `json:"results,omitempty"`
	HostAnalysis           []HostAnalysis           `json:"hostAnalysis,omitempty"`
	CollectionHostAnalysis []CollectionHostAnalysis `json:"collectionHostAnalysis,omitempty"`
	RetirementRecords      []RetirementRecord       `json:"retirementRecords,omitempty"`
	Error                  string                   `json:"error,omitempty"`
	StartTime              time.Time                `json:"startTime"`
	EndTime                *time.Time               `json:"endTime,omitempty"`
}

// Global variables
var (
	config                *Config
	httpClient            *http.Client
	removedIdentifiersMap = make(map[string][]HostIdentifier) // Track removed identifiers by host ID
	jobs                  = make(map[string]*JobStatus)
	jobsMutex             sync.RWMutex
	upgrader              = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for local development
		},
	}
	// WebSocket connection management
	wsConnections = make(map[*websocket.Conn]bool)
	wsMutex       sync.RWMutex
	// Rollback management
	rollbackHistory = make(map[string]*RollbackData)
	rollbackMutex   sync.RWMutex
)

func findAvailablePort() int {
	fmt.Println("LRCleaner - LogRhythm Log Source Management Tool")
	fmt.Println("================================================")
	fmt.Println()

	// Check for command line argument first
	if len(os.Args) > 1 {
		if port, err := strconv.Atoi(os.Args[1]); err == nil && port > 0 && port <= 65535 {
			if isPortAvailable(port) {
				fmt.Printf("Using port %d from command line argument\n", port)
				return port
			} else {
				fmt.Printf("Port %d from command line is not available, finding alternative...\n", port)
			}
		}
	}

	// Try default port 8080 first
	if isPortAvailable(8080) {
		fmt.Println("Using default port 8080")
		return 8080
	}

	// If 8080 is not available, find an available port starting from 8000
	fmt.Println("Port 8080 is in use, searching for available port...")

	for port := 8000; port <= 8500; port++ {
		if isPortAvailable(port) {
			fmt.Printf("Found available port: %d\n", port)
			return port
		}
	}

	// This should never happen, but just in case
	fmt.Println("No available ports found, using 8080 anyway")
	return 8080
}

func isPortAvailable(port int) bool {
	address := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

func openBrowser(url string) {
	// Wait a moment for the server to start
	time.Sleep(2 * time.Second)

	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		fmt.Printf("Please open your browser and navigate to: %s\n", url)
		return
	}

	if err != nil {
		fmt.Printf("Could not automatically open browser. Please navigate to: %s\n", url)
	} else {
		fmt.Printf("Opening browser to: %s\n", url)
	}
}

func loadRollbackFiles() {
	rollbackDir := config.Rollback.BackupLocation
	if rollbackDir == "" {
		rollbackDir = "./rollback/"
	}

	// Check if rollback directory exists
	if _, err := os.Stat(rollbackDir); os.IsNotExist(err) {
		log.Printf("Rollback directory does not exist: %s", rollbackDir)
		return
	}

	// Read all JSON files in the rollback directory
	files, err := filepath.Glob(filepath.Join(rollbackDir, "*.json"))
	if err != nil {
		log.Printf("Error reading rollback directory: %v", err)
		return
	}

	log.Printf("Found %d rollback files to load", len(files))

	loadedCount := 0
	for _, file := range files {
		// Read the file
		data, err := os.ReadFile(file)
		if err != nil {
			log.Printf("Error reading rollback file %s: %v", file, err)
			continue
		}

		// Parse the rollback data
		var rollbackData RollbackData
		if err := json.Unmarshal(data, &rollbackData); err != nil {
			log.Printf("Error parsing rollback file %s: %v", file, err)
			continue
		}

		// Verify checksum if present
		if rollbackData.Checksum != "" {
			expectedChecksum := calculateChecksum(data)
			if rollbackData.Checksum != expectedChecksum {
				log.Printf("Checksum mismatch for rollback file %s, skipping", file)
				continue
			}
		}

		// Store in memory
		rollbackMutex.Lock()
		rollbackHistory[rollbackData.ID] = &rollbackData
		rollbackMutex.Unlock()

		loadedCount++
		log.Printf("Loaded rollback file: %s (ID: %s)", file, rollbackData.ID)
	}

	log.Printf("Successfully loaded %d rollback files", loadedCount)
}

func main() {
	// Find available port
	port := findAvailablePort()

	// Initialize configuration
	config = loadConfig()

	// Load existing rollback files
	loadRollbackFiles()

	// Setup HTTP client with custom transport
	httpClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 30 * time.Second,
	}

	// Setup routes
	router := mux.NewRouter()

	// Static files (embedded web UI)
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static/"))))
	router.HandleFunc("/", serveIndex)

	// API routes
	api := router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/config", handleConfig).Methods("GET", "POST")
	api.HandleFunc("/test-connection", handleTestConnection).Methods("POST")
	api.HandleFunc("/test", handleTestMode).Methods("POST")
	api.HandleFunc("/backup", handleBackup).Methods("POST")
	api.HandleFunc("/apply", handleApplyMode).Methods("POST")
	api.HandleFunc("/apply/execute", handleExecuteApply).Methods("POST")
	api.HandleFunc("/collection-hosts/retire", handleRetireCollectionHosts).Methods("POST")
	api.HandleFunc("/export/{jobId}", handleExport).Methods("GET")
	api.HandleFunc("/export/pdf/{jobId}", handleExportPDF).Methods("GET")
	api.HandleFunc("/jobs/{jobId}", handleJobStatus).Methods("GET")
	api.HandleFunc("/ws", handleWebSocket)

	// Rollback API routes
	api.HandleFunc("/rollback/history", handleRollbackHistory).Methods("GET")
	api.HandleFunc("/rollback/{rollbackId}", handleRollbackDetails).Methods("GET")
	api.HandleFunc("/rollback/{rollbackId}/execute", handleExecuteRollback).Methods("POST")
	api.HandleFunc("/rollback/{rollbackId}", handleDeleteRollback).Methods("DELETE")

	// Start server
	server := &http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		url := fmt.Sprintf("http://localhost:%d", port)
		fmt.Printf("LRCleaner starting on %s\n", url)

		// Open browser automatically
		go openBrowser(url)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Handle shutdown gracefully
	go func() {
		<-quit
		fmt.Println("\n🛑 Shutdown signal received. Gracefully shutting down LRCleaner...")
		fmt.Println("   - Closing WebSocket connections...")
		fmt.Println("   - Stopping HTTP server...")
		fmt.Println("   - Please wait...")

		// Close all WebSocket connections
		wsMutex.Lock()
		for conn := range wsConnections {
			conn.Close()
		}
		wsConnections = make(map[*websocket.Conn]bool)
		wsMutex.Unlock()

		// Graceful shutdown with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			fmt.Printf("⚠️  Server forced to shutdown: %v\n", err)
		} else {
			fmt.Println("✅ LRCleaner stopped gracefully")
		}

		os.Exit(0)
	}()

	// Keep the main goroutine alive
	select {}
}

func loadConfig() *Config {
	config := &Config{
		Hostname: "localhost",
		Port:     8501,
		ExcludedLogSources: []string{
			"Open Collector",
			"Echo",
			"AI Engine",
			"LogRhythm System",
		},
		Rollback: RollbackConfig{
			Enabled:           true,
			RetentionDays:     30,
			MaxRollbackPoints: 10,
			AutoBackup:        true,
			BackupLocation:    "./rollback/",
			ChecksumAlgorithm: "sha256",
		},
	}

	// Try to load from file
	if data, err := os.ReadFile("config.json"); err == nil {
		if err := json.Unmarshal(data, config); err != nil {
			log.Printf("Warning: Failed to parse config.json: %v", err)
		}
	}

	return config
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./web/index.html")
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config)
	case "POST":
		var newConfig Config
		if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		config = &newConfig

		// Save to file
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			log.Printf("Error marshaling config: %v", err)
			http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile("config.json", data, 0644); err != nil {
			log.Printf("Error writing config file: %v", err)
			http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}
}

func handleTestConnection(w http.ResponseWriter, r *http.Request) {
	var testConfig Config
	if err := json.NewDecoder(r.Body).Decode(&testConfig); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if testConfig.Hostname == "" || testConfig.APIKey == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Hostname and API key are required",
		})
		return
	}

	// Test the connection by making a simple API call
	url := fmt.Sprintf("https://%s:%d/lr-admin-api/logsources?count=1&offset=0",
		testConfig.Hostname, testConfig.Port)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to create request: %v", err),
		})
		return
	}

	req.Header.Set("Authorization", "Bearer "+testConfig.APIKey)

	// Create a temporary client for testing
	testClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 10 * time.Second,
	}

	resp, err := testClient.Do(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Connection failed: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Connection successful! LogRhythm API is accessible.",
		})
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("API returned status code: %d", resp.StatusCode),
		})
	}
}

func handleTestMode(w http.ResponseWriter, r *http.Request) {
	log.Println("Test mode request received")

	var request struct {
		Date string `json:"date"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decoding test mode request: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("Test mode request - Date: %s", request.Date)

	// Parse date
	selectedDate, err := time.Parse("2006-01-02", request.Date)
	if err != nil {
		log.Printf("Error parsing date: %v", err)
		http.Error(w, "Invalid date format", http.StatusBadRequest)
		return
	}

	// Create job
	jobID := fmt.Sprintf("test_%d", time.Now().Unix())
	job := &JobStatus{
		ID:        jobID,
		Status:    "running",
		Progress:  0,
		Message:   "Starting analysis...",
		StartTime: time.Now(),
	}

	log.Printf("Created test job: %s", jobID)

	jobsMutex.Lock()
	jobs[jobID] = job
	jobsMutex.Unlock()

	// Start analysis in background
	log.Printf("Starting background analysis for job: %s", jobID)
	go analyzeLogSources(jobID, selectedDate)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"jobId": jobID})
	log.Printf("Test mode response sent - JobID: %s", jobID)
}

func handleBackup(w http.ResponseWriter, r *http.Request) {
	var req BackupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Password == "" {
		http.Error(w, "Password is required", http.StatusBadRequest)
		return
	}

	// Perform SQL backup
	success, err := performSQLBackup(req.Password, req.Location)
	if err != nil {
		log.Printf("Backup error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": success,
		"message": "Database backup completed successfully",
	})
}

func performSQLBackup(password, location string) (bool, error) {
	// SQL Server connection string
	// Assuming LogRhythmEMDB is on localhost with default instance
	connectionString := fmt.Sprintf("server=localhost;user id=logrhythmadmin;password=%s;database=LogRhythmEMDB;encrypt=disable", password)

	// Open database connection
	db, err := sql.Open("mssql", connectionString)
	if err != nil {
		return false, fmt.Errorf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		return false, fmt.Errorf("failed to ping database: %v", err)
	}

	// Create backup filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupFile := fmt.Sprintf("%s\\LogRhythmEMDB_backup_%s.bak", location, timestamp)

	// Execute backup command
	backupQuery := fmt.Sprintf("BACKUP DATABASE [LogRhythmEMDB] TO DISK = '%s' WITH FORMAT, INIT, NAME = 'LogRhythmEMDB Full Backup', SKIP, NOREWIND, NOUNLOAD, STATS = 10", backupFile)

	_, err = db.Exec(backupQuery)
	if err != nil {
		return false, fmt.Errorf("failed to execute backup: %v", err)
	}

	log.Printf("Database backup completed successfully: %s", backupFile)
	return true, nil
}

func handleApplyMode(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Date string `json:"date"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Parse date
	selectedDate, err := time.Parse("2006-01-02", request.Date)
	if err != nil {
		http.Error(w, "Invalid date format", http.StatusBadRequest)
		return
	}

	// Create job
	jobID := fmt.Sprintf("apply_%d", time.Now().Unix())
	job := &JobStatus{
		ID:        jobID,
		Status:    "running",
		Progress:  0,
		Message:   "Analyzing hosts for retirement...",
		StartTime: time.Now(),
	}

	jobsMutex.Lock()
	jobs[jobID] = job
	jobsMutex.Unlock()

	// Start analysis in background
	go analyzeHostsForRetirement(jobID, selectedDate)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"jobId": jobID})
}

func handleExecuteApply(w http.ResponseWriter, r *http.Request) {
	var request ApplyRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(request.SelectedHosts) == 0 {
		http.Error(w, "No hosts selected", http.StatusBadRequest)
		return
	}

	// Create job
	jobID := fmt.Sprintf("execute_%d", time.Now().Unix())
	job := &JobStatus{
		ID:        jobID,
		Status:    "running",
		Progress:  0,
		Message:   "Starting retirement process...",
		StartTime: time.Now(),
	}

	jobsMutex.Lock()
	jobs[jobID] = job
	jobsMutex.Unlock()

	// Start retirement in background
	go executeRetirement(jobID, request.SelectedHosts)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"jobId": jobID})
}

func handleRetireCollectionHosts(w http.ResponseWriter, r *http.Request) {
	var request struct {
		SelectedCollectionHosts []string `json:"selectedCollectionHosts"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(request.SelectedCollectionHosts) == 0 {
		http.Error(w, "No collection hosts selected", http.StatusBadRequest)
		return
	}

	// For now, just log the selected collection hosts
	// In a real implementation, you would retire the collection hosts here
	log.Printf("Collection hosts selected for retirement: %v", request.SelectedCollectionHosts)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Collection host retirement initiated for %d hosts", len(request.SelectedCollectionHosts)),
	})
}

func handleExportPDF(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["jobId"]

	jobsMutex.RLock()
	job, exists := jobs[jobID]
	jobsMutex.RUnlock()

	if !exists {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	if len(job.RetirementRecords) == 0 {
		http.Error(w, "No retirement records to export", http.StatusBadRequest)
		return
	}

	// Generate text report content
	reportContent := generateTextReport(job)

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"LRCleaner_Report_%s.txt\"", jobID))
	w.Write(reportContent)
}

func handleExport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["jobId"]

	log.Printf("Export request for job ID: %s", jobID)

	jobsMutex.RLock()
	job, exists := jobs[jobID]
	jobsMutex.RUnlock()

	if !exists {
		log.Printf("Job not found: %s", jobID)
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	log.Printf("Job found: %s", jobID)
	log.Printf("Results count: %d", len(job.Results))
	log.Printf("HostAnalysis count: %d", len(job.HostAnalysis))
	log.Printf("CollectionHostAnalysis count: %d", len(job.CollectionHostAnalysis))

	// Check which results field has data
	var resultsToExport []AnalysisResult
	if len(job.Results) > 0 {
		resultsToExport = job.Results
		log.Printf("Using job.Results for export")
	} else if len(job.HostAnalysis) > 0 {
		// Convert HostAnalysis to AnalysisResult format
		log.Printf("Converting HostAnalysis to export format")
		for _, host := range job.HostAnalysis {
			// For host analysis, we need to get the log sources from each host
			for _, logSource := range host.LogSources {
				resultsToExport = append(resultsToExport, AnalysisResult{
					ID:            logSource.ID,
					HostID:        host.HostID,
					HostName:      host.HostName,
					Name:          logSource.Name,
					LogSourceType: logSource.LogSourceType.Name,
					MaxLogDate:    host.MaxLogDate,
					PingResult:    host.PingResult,
				})
			}
		}
	} else {
		log.Printf("No results to export for job: %s", jobID)
		http.Error(w, "No results to export", http.StatusBadRequest)
		return
	}

	if len(resultsToExport) == 0 {
		log.Printf("No results to export for job: %s", jobID)
		http.Error(w, "No results to export", http.StatusBadRequest)
		return
	}

	// Generate CSV with all log source details
	csv := "LogSourceID,HostID,HostName,LogSourceName,LogSourceType,MaxLogDate,PingResult\n"
	for _, result := range resultsToExport {
		csv += fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s\n",
			idToString(result.ID),
			idToString(result.HostID),
			result.HostName,
			result.Name,
			result.LogSourceType,
			result.MaxLogDate,
			result.PingResult)
	}

	log.Printf("Generated CSV for job %s, length: %d bytes", jobID, len(csv))
	previewLength := 200
	if len(csv) < previewLength {
		previewLength = len(csv)
	}
	log.Printf("CSV preview: %s", csv[:previewLength])

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"LRCleaner_Results_%s.csv\"", jobID))
	w.Write([]byte(csv))
}

func handleJobStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["jobId"]

	jobsMutex.RLock()
	job, exists := jobs[jobID]
	jobsMutex.RUnlock()

	if !exists {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// Broadcast job update to all WebSocket connections
func broadcastJobUpdate(job *JobStatus) {
	wsMutex.RLock()
	defer wsMutex.RUnlock()

	log.Printf("Broadcasting job update for job %s to %d WebSocket connections", job.ID, len(wsConnections))

	for conn := range wsConnections {
		if err := conn.WriteJSON(job); err != nil {
			log.Printf("Error broadcasting job update via WebSocket: %v", err)
			// Remove failed connection
			delete(wsConnections, conn)
			conn.Close()
		} else {
			log.Printf("Successfully broadcasted job update to WebSocket connection")
		}
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("WebSocket connection attempt from %s", r.RemoteAddr)
	log.Printf("WebSocket request headers: %v", r.Header)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
		return
	}
	defer conn.Close()

	log.Printf("WebSocket upgrade successful for %s", r.RemoteAddr)

	// Register connection
	wsMutex.Lock()
	wsConnections[conn] = true
	log.Printf("WebSocket connection registered. Total connections: %d", len(wsConnections))
	wsMutex.Unlock()

	// Send current job statuses
	jobsMutex.RLock()
	for _, job := range jobs {
		if err := conn.WriteJSON(job); err != nil {
			log.Printf("Error sending job status via WebSocket: %v", err)
			break
		}
	}
	jobsMutex.RUnlock()

	// Keep connection alive
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket connection closed:", err)
			// Remove connection
			wsMutex.Lock()
			delete(wsConnections, conn)
			wsMutex.Unlock()
			break
		}
	}
}

func analyzeLogSources(jobID string, selectedDate time.Time) {
	log.Printf("Starting analyzeLogSources for job: %s, date: %s", jobID, selectedDate.Format("2006-01-02"))

	jobsMutex.Lock()
	job := jobs[jobID]
	jobsMutex.Unlock()

	defer func() {
		jobsMutex.Lock()
		now := time.Now()
		job.EndTime = &now
		if job.Status == "running" {
			job.Status = "completed"
		}
		jobsMutex.Unlock()
		log.Printf("Completed analyzeLogSources for job: %s", jobID)
	}()

	// Get all log sources
	log.Printf("Getting all log sources for job: %s", jobID)
	allLogSources, err := getAllLogSources()
	if err != nil {
		log.Printf("Error getting log sources for job %s: %v", jobID, err)
		jobsMutex.Lock()
		job.Status = "error"
		job.Error = err.Error()
		jobsMutex.Unlock()
		return
	}

	log.Printf("Retrieved %d log sources for job: %s", len(allLogSources), jobID)

	// Update progress
	jobsMutex.Lock()
	job.Progress = 25
	job.Message = fmt.Sprintf("Found %d log sources. Filtering...", len(allLogSources))
	jobsMutex.Unlock()

	// Filter log sources
	var filteredSources []LogSource
	for _, ls := range allLogSources {
		// Check date
		if maxLogDate, err := time.Parse(time.RFC3339, ls.MaxLogDate); err == nil {
			if maxLogDate.After(selectedDate) {
				continue
			}
		}

		// Check if already retired
		if ls.RecordStatus == "Retired" {
			continue
		}

		// Check excluded sources
		excluded := false
		sourceType := ls.LogSourceType.Name
		sourceName := ls.Name
		hostName := ls.Host.Name

		// Exclude LogRhythm system monitor agents
		if strings.HasPrefix(sourceType, "LogRhythm") {
			excluded = true
		}

		// Exclude echo hosts and log sources
		if containsIgnoreCase(hostName, "echo") || containsIgnoreCase(sourceName, "echo") {
			excluded = true
		}

		// Check config excluded sources
		if !excluded {
			for _, pattern := range config.ExcludedLogSources {
				if containsIgnoreCase(sourceType, pattern) {
					excluded = true
					break
				}
			}
		}

		if !excluded {
			filteredSources = append(filteredSources, ls)
		}
	}

	// Update progress
	jobsMutex.Lock()
	job.Progress = 50
	job.Message = "Testing host connectivity..."
	jobsMutex.Unlock()

	// Collect unique hostnames for concurrent ping testing
	hostnameSet := make(map[string]bool)
	for _, ls := range filteredSources {
		hostnameSet[ls.Host.Name] = true
	}

	uniqueHostnames := make([]string, 0, len(hostnameSet))
	for hostname := range hostnameSet {
		uniqueHostnames = append(uniqueHostnames, hostname)
	}

	log.Printf("Testing connectivity to %d unique hosts concurrently...", len(uniqueHostnames))

	// Update progress
	jobsMutex.Lock()
	job.Progress = 50
	job.Message = fmt.Sprintf("Testing connectivity to %d hosts...", len(uniqueHostnames))
	jobsMutex.Unlock()

	// Perform concurrent ping testing
	pingResults := pingHostsConcurrent(uniqueHostnames)

	// Update progress
	jobsMutex.Lock()
	job.Progress = 75
	job.Message = "Processing results..."
	jobsMutex.Unlock()

	// Analyze each source using the ping results
	var results []AnalysisResult
	for i, ls := range filteredSources {
		// Update progress
		jobsMutex.Lock()
		job.Progress = 75 + (i * 25 / len(filteredSources))
		job.Message = fmt.Sprintf("Processing %s...", ls.Host.Name)
		jobsMutex.Unlock()

		// Get ping result from our concurrent test
		pingResult := pingResults[ls.Host.Name]

		// Create result
		result := AnalysisResult{
			ID:            ls.ID,
			HostID:        ls.Host.ID,
			HostName:      ls.Host.Name,
			Name:          ls.Name,
			LogSourceType: ls.LogSourceType.Name,
			MaxLogDate:    ls.MaxLogDate,
			PingResult:    pingResult,
		}

		results = append(results, result)
	}

	// Update job with results
	jobsMutex.Lock()
	job.Results = results
	jobsMutex.Unlock()

	// Broadcast the update to WebSocket clients
	broadcastJobUpdate(job)

	// Complete
	jobsMutex.Lock()
	job.Progress = 100
	job.Message = fmt.Sprintf("Analysis complete. Found %d sources.", len(results))
	job.Status = "completed"
	now := time.Now()
	job.EndTime = &now
	jobsMutex.Unlock()

	// Broadcast the completion to WebSocket clients
	broadcastJobUpdate(job)

	// Log summary
	successCount := 0
	failureCount := 0
	unknownCount := 0
	for _, result := range results {
		switch result.PingResult {
		case "Success":
			successCount++
		case "Failure":
			failureCount++
		default:
			unknownCount++
		}
	}
	log.Printf("Test Mode Analysis Complete:")
	log.Printf("  Total log sources analyzed: %d", len(results))
	log.Printf("  Successful pings: %d", successCount)
	log.Printf("  Failed pings: %d", failureCount)
	log.Printf("  Unknown ping results: %d", unknownCount)
}

func getAllLogSources() ([]LogSource, error) {
	var allSources []LogSource
	offset := 0
	count := 1000

	for {
		url := fmt.Sprintf("https://%s:%d/lr-admin-api/logsources?count=%d&offset=%d",
			config.Hostname, config.Port, count, offset)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+config.APIKey)

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
		}

		// Try to decode as object first (expected format)
		var response struct {
			Count int         `json:"count"`
			Items []LogSource `json:"items"`
		}

		// Read the response body
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		// Log first 200 characters of response for debugging
		responsePreview := string(bodyBytes)
		if len(responsePreview) > 200 {
			responsePreview = responsePreview[:200] + "..."
		}
		log.Printf("API Response preview: %s", responsePreview)

		// Try to decode as array format first (LogRhythm API returns arrays)
		var directArray []LogSource
		if err := json.Unmarshal(bodyBytes, &directArray); err != nil {
			log.Printf("Failed to decode as array format, trying object format: %v", err)
			// If that fails, try to decode as object format
			if err := json.Unmarshal(bodyBytes, &response); err != nil {
				return nil, fmt.Errorf("failed to decode response as array or object: %v", err)
			}
			log.Printf("Successfully decoded as object format, got %d items", len(response.Items))
			// Use object format
			allSources = append(allSources, response.Items...)
			if len(response.Items) < count {
				break
			}
		} else {
			log.Printf("Successfully decoded as array format, got %d items", len(directArray))
			// Use direct array
			allSources = append(allSources, directArray...)
			if len(directArray) < count {
				break
			}
		}

		offset += count
	}

	return allSources, nil
}

// Fast ping implementation for single host
func pingHostFast(hostname string) string {
	// Test most common ports with short timeouts
	ports := []string{"443", "80", "22", "3389"}

	for _, port := range ports {
		conn, err := net.DialTimeout("tcp", hostname+":"+port, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return "Success"
		}
	}
	return "Failure"
}

// Concurrent ping testing for multiple hosts
func pingHostsConcurrent(hostnames []string) map[string]string {
	results := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrent connections to avoid overwhelming the system
	maxConcurrent := 50
	semaphore := make(chan struct{}, maxConcurrent)

	log.Printf("Starting concurrent ping test for %d hosts (max %d concurrent)", len(hostnames), maxConcurrent)
	startTime := time.Now()

	for _, hostname := range hostnames {
		wg.Add(1)
		go func(host string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := pingHostFast(host)

			mu.Lock()
			results[host] = result
			mu.Unlock()
		}(hostname)
	}

	wg.Wait()
	duration := time.Since(startTime)

	successCount := 0
	for _, result := range results {
		if result == "Success" {
			successCount++
		}
	}

	log.Printf("Ping test completed in %v: %d/%d hosts reachable (%.1f%% success rate)",
		duration, successCount, len(hostnames), float64(successCount)/float64(len(hostnames))*100)

	return results
}

// Legacy function for backward compatibility
func pingHost(hostname string) string {
	return pingHostFast(hostname)
}

func containsIgnoreCase(s, substr string) bool {
	// Simple case-insensitive contains check
	// Convert both strings to lowercase for comparison
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}

// Helper function to convert interface{} ID to string
func idToString(id interface{}) string {
	switch v := id.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', 0, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func analyzeHostsForRetirement(jobID string, selectedDate time.Time) {
	jobsMutex.Lock()
	job := jobs[jobID]
	jobsMutex.Unlock()

	defer func() {
		jobsMutex.Lock()
		now := time.Now()
		job.EndTime = &now
		if job.Status == "running" {
			job.Status = "completed"
		}
		jobsMutex.Unlock()

		// Broadcast the completion to WebSocket clients
		broadcastJobUpdate(job)
	}()

	// Get all log sources
	allLogSources, err := getAllLogSources()
	if err != nil {
		jobsMutex.Lock()
		job.Status = "error"
		job.Error = err.Error()
		jobsMutex.Unlock()
		return
	}

	// Update progress
	jobsMutex.Lock()
	job.Progress = 25
	job.Message = fmt.Sprintf("Found %d log sources. Analyzing hosts...", len(allLogSources))
	jobsMutex.Unlock()

	// Broadcast the update to WebSocket clients
	broadcastJobUpdate(job)

	// Filter by date and excluded sources
	var filteredSources []LogSource
	for _, ls := range allLogSources {
		// Check date
		if maxLogDate, err := time.Parse(time.RFC3339, ls.MaxLogDate); err == nil {
			if maxLogDate.After(selectedDate) {
				continue
			}
		}

		// Check if already retired
		if ls.RecordStatus == "Retired" {
			continue
		}

		// Check excluded sources
		excluded := false
		sourceType := ls.LogSourceType.Name
		sourceName := ls.Name
		hostName := ls.Host.Name

		// Exclude LogRhythm system monitor agents
		if strings.HasPrefix(sourceType, "LogRhythm") {
			excluded = true
		}

		// Exclude echo hosts and log sources
		if containsIgnoreCase(hostName, "echo") || containsIgnoreCase(sourceName, "echo") {
			excluded = true
		}

		// Check config excluded sources
		if !excluded {
			for _, pattern := range config.ExcludedLogSources {
				if containsIgnoreCase(sourceType, pattern) {
					excluded = true
					break
				}
			}
		}

		if !excluded {
			filteredSources = append(filteredSources, ls)
		}
	}

	// Group by host
	hostMap := make(map[string]*HostAnalysis)
	for _, ls := range filteredSources {
		hostID := idToString(ls.Host.ID)
		hostName := ls.Host.Name

		if hostMap[hostID] == nil {
			hostMap[hostID] = &HostAnalysis{
				HostID:     ls.Host.ID, // Keep original interface{} type
				HostName:   hostName,
				LogSources: []LogSource{},
			}
		}

		hostMap[hostID].LogSources = append(hostMap[hostID].LogSources, ls)
		hostMap[hostID].LogSourceCount++

		// Update max log date
		if hostMap[hostID].MaxLogDate == "" || ls.MaxLogDate < hostMap[hostID].MaxLogDate {
			hostMap[hostID].MaxLogDate = ls.MaxLogDate
		}
	}

	// Collect unique hostnames for concurrent ping testing
	hostnames := make([]string, 0, len(hostMap))
	for _, host := range hostMap {
		hostnames = append(hostnames, host.HostName)
	}

	log.Printf("Testing connectivity to %d hosts concurrently...", len(hostnames))

	// Update progress
	jobsMutex.Lock()
	job.Progress = 50
	job.Message = fmt.Sprintf("Testing connectivity to %d hosts...", len(hostnames))
	jobsMutex.Unlock()

	// Broadcast the update to WebSocket clients
	broadcastJobUpdate(job)

	// Perform concurrent ping testing
	pingResults := pingHostsConcurrent(hostnames)

	// Update progress
	jobsMutex.Lock()
	job.Progress = 75
	job.Message = "Processing host analysis..."
	jobsMutex.Unlock()

	// Broadcast the update to WebSocket clients
	broadcastJobUpdate(job)

	// Analyze each host using the ping results
	var hostAnalysis []HostAnalysis
	hostCount := 0
	for _, host := range hostMap {
		hostCount++
		log.Printf("Host analysis %d/%d: Processing host %s (%d log sources)",
			hostCount, len(hostMap), host.HostName, host.LogSourceCount)

		// Get ping result from our concurrent test
		host.PingResult = pingResults[host.HostName]

		// Determine recommendations for each log source
		recommendedLogSources := 0
		for i := range host.LogSources {
			ls := &host.LogSources[i]

			// Only recommend log source if ping fails (host is not reachable)
			// If host is pingable but has old logs, recommend troubleshooting instead
			ls.Recommended = (host.PingResult == "Failure")

			if ls.Recommended {
				recommendedLogSources++
				log.Printf("  → Log source %s is RECOMMENDED for retirement (host not pingable)", ls.Name)
			} else if host.PingResult == "Success" && ls.MaxLogDate != "" && parseTime(ls.MaxLogDate).Before(selectedDate) {
				log.Printf("  → Log source %s has old logs but host is pingable - recommend troubleshooting", ls.Name)
			} else {
				log.Printf("  → Log source %s is NOT recommended for retirement", ls.Name)
			}
		}

		// Only recommend host if ALL log sources are recommended
		host.Recommended = (recommendedLogSources > 0) && (recommendedLogSources == len(host.LogSources))

		// Log host recommendation
		if host.Recommended {
			log.Printf("  → Host %s is RECOMMENDED for retirement (all %d log sources recommended)", host.HostName, recommendedLogSources)
		} else {
			log.Printf("  → Host %s is NOT recommended for retirement (%d/%d log sources recommended)", host.HostName, recommendedLogSources, len(host.LogSources))
		}

		hostAnalysis = append(hostAnalysis, *host)
	}

	// Update job with host analysis
	jobsMutex.Lock()
	job.Progress = 100
	job.Message = fmt.Sprintf("Host analysis complete. Found %d hosts.", len(hostAnalysis))
	job.HostAnalysis = hostAnalysis
	jobsMutex.Unlock()

	// Broadcast the update to WebSocket clients
	broadcastJobUpdate(job)

	// Log summary
	recommendedCount := 0
	for _, host := range hostAnalysis {
		if host.Recommended {
			recommendedCount++
		}
	}
	log.Printf("Apply Mode Host Analysis Complete:")
	log.Printf("  Total hosts analyzed: %d", len(hostAnalysis))
	log.Printf("  Recommended for retirement: %d", recommendedCount)
	log.Printf("  Not recommended: %d", len(hostAnalysis)-recommendedCount)
}

func analyzeCollectionHosts(jobID string) []CollectionHostAnalysis {
	log.Printf("Starting collection host analysis for job: %s", jobID)

	// Get all log sources
	allLogSources, err := getAllLogSources()
	if err != nil {
		log.Printf("Error getting log sources for collection host analysis: %v", err)
		return nil
	}

	// Group by collection host (system monitor)
	collectionHostMap := make(map[string]*CollectionHostAnalysis)
	for _, ls := range allLogSources {
		if ls.SystemMonitorID == nil || ls.SystemMonitorName == "" {
			continue // Skip log sources without collection host info
		}

		collectionHostID := idToString(ls.SystemMonitorID)
		collectionHostName := ls.SystemMonitorName

		if collectionHostMap[collectionHostID] == nil {
			collectionHostMap[collectionHostID] = &CollectionHostAnalysis{
				SystemMonitorID:   ls.SystemMonitorID,
				SystemMonitorName: collectionHostName,
				LogSources:        []LogSource{},
			}
		}

		collectionHostMap[collectionHostID].LogSources = append(collectionHostMap[collectionHostID].LogSources, ls)
		collectionHostMap[collectionHostID].LogSourceCount++
	}

	// Test connectivity to collection hosts
	collectionHostnames := make([]string, 0, len(collectionHostMap))
	for _, ch := range collectionHostMap {
		collectionHostnames = append(collectionHostnames, ch.SystemMonitorName)
	}

	log.Printf("Testing connectivity to %d collection hosts...", len(collectionHostnames))
	pingResults := pingHostsConcurrent(collectionHostnames)

	// Analyze each collection host
	var collectionHostAnalysis []CollectionHostAnalysis
	for _, ch := range collectionHostMap {
		ch.PingResult = pingResults[ch.SystemMonitorName]

		// Recommend for retirement if:
		// 1. Ping fails AND has zero log sources, OR
		// 2. Has zero log sources (regardless of ping status)
		ch.Recommended = (ch.PingResult == "Failure" && ch.LogSourceCount == 0) ||
			(ch.LogSourceCount == 0)

		if ch.Recommended {
			log.Printf("Collection host %s is RECOMMENDED for retirement (Ping: %s, Log Sources: %d)",
				ch.SystemMonitorName, ch.PingResult, ch.LogSourceCount)
		}

		collectionHostAnalysis = append(collectionHostAnalysis, *ch)
	}

	log.Printf("Collection Host Analysis Complete:")
	log.Printf("  Total collection hosts analyzed: %d", len(collectionHostAnalysis))
	recommendedCount := 0
	for _, ch := range collectionHostAnalysis {
		if ch.Recommended {
			recommendedCount++
		}
	}
	log.Printf("  Recommended for retirement: %d", recommendedCount)

	// If there are recommended collection hosts, process them for retirement
	if recommendedCount > 0 {
		log.Printf("Processing recommended collection hosts for retirement...")
		retiredCount := 0

		for _, ch := range collectionHostAnalysis {
			if ch.Recommended {
				log.Printf("Processing collection host %s (ID: %s) for retirement...", ch.SystemMonitorName, idToString(ch.SystemMonitorID))

				// First unlicense the system monitor
				if unlicenseSystemMonitor(ch.SystemMonitorID) {
					log.Printf("  ✓ Successfully unlicensed collection host: %s", ch.SystemMonitorName)

					// Then retire the system monitor
					if retireSystemMonitor(ch.SystemMonitorID) {
						retiredCount++
						log.Printf("  ✓ Successfully retired collection host: %s", ch.SystemMonitorName)
					} else {
						log.Printf("  ✗ Failed to retire collection host: %s", ch.SystemMonitorName)
					}
				} else {
					log.Printf("  ✗ Failed to unlicense collection host: %s", ch.SystemMonitorName)
				}
			}
		}

		log.Printf("Collection Host Retirement Summary:")
		log.Printf("  Collection hosts processed: %d", recommendedCount)
		log.Printf("  Collection hosts successfully retired: %d", retiredCount)
	}

	return collectionHostAnalysis
}

func executeRetirement(jobID string, selectedHosts []string) {
	jobsMutex.Lock()
	job := jobs[jobID]
	jobsMutex.Unlock()

	// Create rollback data before starting retirement
	rollbackData := createRollbackData(jobID, selectedHosts)
	if rollbackData != nil {
		saveRollbackData(rollbackData)
	}

	defer func() {
		jobsMutex.Lock()
		now := time.Now()
		job.EndTime = &now
		if job.Status == "running" {
			job.Status = "completed"
		}
		jobsMutex.Unlock()
	}()

	// Get the host analysis from the previous job
	var hostAnalysis []HostAnalysis
	jobsMutex.RLock()
	for _, otherJob := range jobs {
		if len(otherJob.HostAnalysis) > 0 {
			hostAnalysis = otherJob.HostAnalysis
			break
		}
	}
	jobsMutex.RUnlock()

	if len(hostAnalysis) == 0 {
		jobsMutex.Lock()
		job.Status = "error"
		job.Error = "No host analysis found. Please run Apply Mode analysis first."
		jobsMutex.Unlock()
		return
	}

	// Create map of selected hosts
	selectedMap := make(map[string]bool)
	for _, hostID := range selectedHosts {
		selectedMap[hostID] = true
	}

	// Find hosts to retire
	var hostsToRetire []HostAnalysis
	for _, host := range hostAnalysis {
		if selectedMap[idToString(host.HostID)] {
			hostsToRetire = append(hostsToRetire, host)
		}
	}

	// Process each host
	processedLogSources := 0
	var retirementRecords []RetirementRecord

	for i, host := range hostsToRetire {
		jobsMutex.Lock()
		job.Progress = (i * 100) / len(hostsToRetire)
		job.Message = fmt.Sprintf("Retiring host %s (%d log sources)...", host.HostName, host.LogSourceCount)
		jobsMutex.Unlock()

		log.Printf("Retirement %d/%d: Processing host %s (%d log sources)",
			i+1, len(hostsToRetire), host.HostName, host.LogSourceCount)

		// Retire all log sources for this host
		for j, logSource := range host.LogSources {
			log.Printf("  → Retiring log source %d/%d: %s",
				j+1, len(host.LogSources), logSource.Name)
			// Create retirement record before making changes
			record := RetirementRecord{
				LogSourceID:    logSource.ID,
				HostID:         host.HostID,
				HostName:       host.HostName,
				OriginalName:   logSource.Name,
				RetiredName:    logSource.Name + " Retired by LRCleaner",
				OriginalStatus: logSource.RecordStatus,
				RetiredStatus:  "Retired",
				Timestamp:      time.Now(),
			}

			// Check if log source is already retired
			if logSource.RecordStatus == "Retired" {
				log.Printf("    ⚠ Log source %s is already retired, skipping", logSource.Name)
				continue
			}

			// Update via API (the function now handles getting, modifying, and putting the log source)
			success := updateLogSource(logSource.ID)
			if success {
				processedLogSources++
				retirementRecords = append(retirementRecords, record)
				log.Printf("    ✓ Successfully retired: %s", logSource.Name)
			} else {
				log.Printf("    ✗ Failed to retire: %s", logSource.Name)
			}
		}
	}

	// Complete
	jobsMutex.Lock()
	job.Progress = 100
	job.Message = fmt.Sprintf("Retirement complete. Processed %d log sources across %d hosts.", processedLogSources, len(hostsToRetire))
	job.RetirementRecords = retirementRecords
	jobsMutex.Unlock()

	// Log summary
	totalAttempted := 0
	for _, host := range hostsToRetire {
		totalAttempted += len(host.LogSources)
	}
	log.Printf("Retirement Process Complete:")
	log.Printf("  Hosts processed: %d", len(hostsToRetire))
	log.Printf("  Log sources successfully retired: %d", processedLogSources)
	log.Printf("  Total log sources attempted: %d", totalAttempted)

	// Retire system monitor agents that have no remaining active log sources
	log.Printf("Checking system monitor agents for retirement...")
	retiredAgents := 0
	uniqueAgents := make(map[string]bool)

	// Collect unique system monitor agent IDs from the retirement records
	for _, record := range retirementRecords {
		// Find the system monitor ID for this log source
		for _, host := range hostsToRetire {
			for _, logSource := range host.LogSources {
				if idToString(logSource.ID) == idToString(record.LogSourceID) && logSource.SystemMonitorID != nil {
					agentID := idToString(logSource.SystemMonitorID)
					uniqueAgents[agentID] = true
					log.Printf("Found system monitor agent %s for retired log source %s", agentID, record.OriginalName)
					break
				}
			}
		}
	}

	log.Printf("Total unique system monitor agents from retirement records: %d", len(uniqueAgents))

	// Check each unique system monitor agent for retirement
	for agentID := range uniqueAgents {
		log.Printf("Checking system monitor agent %s for retirement...", agentID)

		// Check if agent has any remaining active log sources
		hasActiveLogSources := checkAgentHasActiveLogSources(agentID)
		log.Printf("System monitor agent %s active log sources check result: %t", agentID, hasActiveLogSources)

		if !hasActiveLogSources {
			log.Printf("System monitor agent %s has no active log sources, proceeding with retirement...", agentID)

			// Retire the system monitor agent
			if retireSystemMonitor(agentID) {
				retiredAgents++
				log.Printf("  ✓ Successfully retired system monitor agent: %s", agentID)
			} else {
				log.Printf("  ✗ Failed to retire system monitor agent: %s", agentID)
			}
		} else {
			log.Printf("System monitor agent %s still has active log sources, skipping agent retirement", agentID)
		}
	}

	log.Printf("System Monitor Agent Retirement Summary:")
	log.Printf("  Agents checked: %d", len(uniqueAgents))
	log.Printf("  Agents successfully retired: %d", retiredAgents)

	// Check and retire hosts that have no remaining active log sources
	log.Printf("Checking hosts for retirement...")
	retiredHosts := 0
	uniqueHosts := make(map[string]bool)

	// Collect unique host IDs from the retirement records
	for _, record := range retirementRecords {
		hostID := idToString(record.HostID)
		uniqueHosts[hostID] = true
		log.Printf("Found retirement record for host %s (log source: %s)", hostID, record.OriginalName)
	}

	log.Printf("Total unique hosts from retirement records: %d", len(uniqueHosts))

	// Check each unique host for retirement
	for hostID := range uniqueHosts {
		log.Printf("=== HOST RETIREMENT PROCESS STARTING ===")
		log.Printf("Checking host %s for retirement...", hostID)

		// Check if host has any remaining active log sources
		hasActiveLogSources := checkHostHasActiveLogSources(hostID)
		log.Printf("Host %s active log sources check result: %t", hostID, hasActiveLogSources)

		if !hasActiveLogSources {
			log.Printf("Host %s has no active log sources, proceeding with retirement...", hostID)

			// Step 1: Find and retire any associated system monitor agents FIRST
			log.Printf("=== STEP 1: AGENT RETIREMENT ===")
			systemMonitorID := getSystemMonitorIDForHost(hostID, retirementRecords, hostsToRetire)
			if systemMonitorID != "" {
				log.Printf("Found system monitor agent %s associated with host %s", systemMonitorID, hostID)
				log.Printf("DEBUG: About to retire agent %s before retiring host %s", systemMonitorID, hostID)

				// First unlicense the system monitor
				log.Printf("DEBUG: Calling unlicenseSystemMonitor for agent %s", systemMonitorID)
				if unlicenseSystemMonitor(systemMonitorID) {
					log.Printf("  ✓ Successfully unlicensed system monitor agent: %s", systemMonitorID)

					// Then retire the system monitor
					log.Printf("DEBUG: Calling retireSystemMonitor for agent %s", systemMonitorID)
					if retireSystemMonitor(systemMonitorID) {
						log.Printf("  ✓ Successfully retired system monitor agent: %s", systemMonitorID)
						log.Printf("DEBUG: Agent %s retirement completed successfully", systemMonitorID)
					} else {
						log.Printf("  ✗ Failed to retire system monitor agent: %s", systemMonitorID)
						log.Printf("DEBUG: Agent %s retirement failed, but continuing with host retirement", systemMonitorID)
					}
				} else {
					log.Printf("  ✗ Failed to unlicense system monitor agent: %s", systemMonitorID)
					log.Printf("DEBUG: Agent %s unlicensing failed, but continuing with host retirement", systemMonitorID)
				}
			} else {
				log.Printf("No system monitor agent found for host %s", hostID)
				log.Printf("DEBUG: No agent retirement needed for host %s", hostID)
			}

			// Step 2: Remove identifiers from host
			log.Printf("=== STEP 2: HOST IDENTIFIER REMOVAL ===")
			log.Printf("DEBUG: About to remove identifiers from host %s", hostID)
			log.Printf("Removing identifiers from host %s...", hostID)

			// Step 3: Retire the host
			log.Printf("=== STEP 3: HOST RETIREMENT ===")
			log.Printf("DEBUG: About to retire host %s", hostID)
			success, removedIdentifiers := updateHost(hostID)
			if success {
				retiredHosts++
				// Store the removed identifiers for rollback data
				removedIdentifiersMap[idToString(hostID)] = removedIdentifiers
				log.Printf("  ✓ Successfully retired host: %s", hostID)
				log.Printf("  ✓ Removed %d identifiers from host: %s", len(removedIdentifiers), hostID)
				log.Printf("DEBUG: Host %s retirement completed successfully", hostID)
			} else {
				log.Printf("  ✗ Failed to retire host: %s", hostID)
				log.Printf("DEBUG: Host %s retirement failed", hostID)
			}
			log.Printf("=== HOST RETIREMENT PROCESS COMPLETED ===")
		} else {
			log.Printf("Host %s still has active log sources, skipping host retirement", hostID)
		}
	}

	log.Printf("Host Retirement Summary:")
	log.Printf("  Hosts checked: %d", len(uniqueHosts))
	log.Printf("  Hosts successfully retired: %d", retiredHosts)

	// Analyze collection hosts after retirement
	log.Printf("Analyzing collection hosts after retirement...")
	collectionHostAnalysis := analyzeCollectionHosts(jobID)

	// Update job with collection host analysis
	jobsMutex.Lock()
	job.CollectionHostAnalysis = collectionHostAnalysis
	jobsMutex.Unlock()

	// Broadcast the collection host analysis update
	broadcastJobUpdate(job)
}

func parseTime(timeStr string) time.Time {
	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t
	}
	return time.Time{}
}

func getSystemMonitorIDForHost(hostID interface{}, retirementRecords []RetirementRecord, hostsToRetire []HostAnalysis) string {
	// Find the system monitor ID for this host by looking at the retirement records
	log.Printf("DEBUG: Searching for system monitor ID for host %s", hostID)
	log.Printf("DEBUG: Checking %d retirement records", len(retirementRecords))

	for _, record := range retirementRecords {
		log.Printf("DEBUG: Checking retirement record for host %s (log source: %s)", idToString(record.HostID), record.OriginalName)
		if idToString(record.HostID) == hostID {
			// Find the system monitor ID for this log source
			for _, host := range hostsToRetire {
				for _, logSource := range host.LogSources {
					if idToString(logSource.ID) == idToString(record.LogSourceID) && logSource.SystemMonitorID != nil {
						systemMonitorID := idToString(logSource.SystemMonitorID)
						log.Printf("DEBUG: Found system monitor ID %s for host %s via log source %s", systemMonitorID, hostID, record.OriginalName)
						return systemMonitorID
					}
				}
			}
		}
	}
	log.Printf("DEBUG: No system monitor ID found for host %s", hostID)
	return ""
}

func checkAgentHasActiveLogSources(agentID interface{}) bool {
	// Check if the system monitor agent has any remaining active log sources (excluding LogRhythm and echo sources)
	url := fmt.Sprintf("https://%s:%d/lr-admin-api/logsources?systemMonitorId=%s&recordStatus=active", config.Hostname, config.Port, idToString(agentID))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Error creating request to check agent %s log sources: %v", idToString(agentID), err)
		return true // Assume it has log sources if we can't check
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Error checking log sources for agent %s: %v", idToString(agentID), err)
		return true // Assume it has log sources if we can't check
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to check log sources for agent %s, status: %d", idToString(agentID), resp.StatusCode)
		return true // Assume it has log sources if we can't check
	}

	// Parse the response - try array format first
	var allLogSources []LogSource
	if err := json.NewDecoder(resp.Body).Decode(&allLogSources); err != nil {
		// Try object format
		var responseObj map[string]interface{}
		resp.Body.Close()

		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			return true
		}
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err = httpClient.Do(req)
		if err != nil {
			return true
		}
		defer resp.Body.Close()

		if err := json.NewDecoder(resp.Body).Decode(&responseObj); err != nil {
			return true
		}

		if items, ok := responseObj["items"].([]interface{}); ok {
			// Convert items to LogSource structs
			for _, item := range items {
				if itemMap, ok := item.(map[string]interface{}); ok {
					var ls LogSource
					if lsBytes, err := json.Marshal(itemMap); err == nil {
						if err := json.Unmarshal(lsBytes, &ls); err == nil {
							allLogSources = append(allLogSources, ls)
						}
					}
				}
			}
		}
	}

	// Apply the same filtering logic as in analyzeHostsForRetirement
	var filteredLogSources []LogSource
	for _, ls := range allLogSources {
		// Check if already retired
		if ls.RecordStatus == "Retired" {
			continue
		}

		// Check excluded sources
		excluded := false
		sourceType := ls.LogSourceType.Name
		sourceName := ls.Name
		hostName := ls.Host.Name

		// Exclude LogRhythm system monitor agents
		if strings.HasPrefix(sourceType, "LogRhythm") {
			excluded = true
		}

		// Exclude echo hosts and log sources
		if containsIgnoreCase(hostName, "echo") || containsIgnoreCase(sourceName, "echo") {
			excluded = true
		}

		// Check config excluded sources
		if !excluded {
			for _, pattern := range config.ExcludedLogSources {
				if containsIgnoreCase(sourceType, pattern) {
					excluded = true
					break
				}
			}
		}

		if !excluded {
			filteredLogSources = append(filteredLogSources, ls)
		}
	}

	hasActiveLogSources := len(filteredLogSources) > 0
	log.Printf("System monitor agent %s has %d total log sources, %d filterable log sources (after excluding LogRhythm/echo)",
		idToString(agentID), len(allLogSources), len(filteredLogSources))
	return hasActiveLogSources
}

func checkHostHasActiveLogSources(hostID interface{}) bool {
	// Check if the host has any remaining active log sources (excluding LogRhythm and echo sources)
	url := fmt.Sprintf("https://%s:%d/lr-admin-api/logsources?hostId=%s&recordStatus=active", config.Hostname, config.Port, idToString(hostID))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Error creating request to check host %s log sources: %v", idToString(hostID), err)
		return true // Assume it has log sources if we can't check
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Error checking log sources for host %s: %v", idToString(hostID), err)
		return true // Assume it has log sources if we can't check
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to check log sources for host %s, status: %d", idToString(hostID), resp.StatusCode)
		return true // Assume it has log sources if we can't check
	}

	// Parse the response - try array format first
	var allLogSources []LogSource
	if err := json.NewDecoder(resp.Body).Decode(&allLogSources); err != nil {
		// Try object format
		var responseObj map[string]interface{}
		resp.Body.Close()

		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			return true
		}
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err = httpClient.Do(req)
		if err != nil {
			return true
		}
		defer resp.Body.Close()

		if err := json.NewDecoder(resp.Body).Decode(&responseObj); err != nil {
			return true
		}

		if items, ok := responseObj["items"].([]interface{}); ok {
			// Convert items to LogSource structs
			for _, item := range items {
				if itemMap, ok := item.(map[string]interface{}); ok {
					var ls LogSource
					if lsBytes, err := json.Marshal(itemMap); err == nil {
						if err := json.Unmarshal(lsBytes, &ls); err == nil {
							allLogSources = append(allLogSources, ls)
						}
					}
				}
			}
		}
	}

	// Apply the same filtering logic as in analyzeHostsForRetirement
	var filteredLogSources []LogSource
	for _, ls := range allLogSources {
		// Check if already retired
		if ls.RecordStatus == "Retired" {
			continue
		}

		// Check excluded sources
		excluded := false
		sourceType := ls.LogSourceType.Name
		sourceName := ls.Name
		hostName := ls.Host.Name

		// Exclude LogRhythm system monitor agents
		if strings.HasPrefix(sourceType, "LogRhythm") {
			excluded = true
		}

		// Exclude echo hosts and log sources
		if containsIgnoreCase(hostName, "echo") || containsIgnoreCase(sourceName, "echo") {
			excluded = true
		}

		// Check config excluded sources
		if !excluded {
			for _, pattern := range config.ExcludedLogSources {
				if containsIgnoreCase(sourceType, pattern) {
					excluded = true
					break
				}
			}
		}

		if !excluded {
			filteredLogSources = append(filteredLogSources, ls)
		}
	}

	hasActiveLogSources := len(filteredLogSources) > 0
	log.Printf("Host %s has %d total log sources, %d filterable log sources (after excluding LogRhythm/echo)",
		idToString(hostID), len(allLogSources), len(filteredLogSources))
	return hasActiveLogSources
}

func unlicenseSystemMonitor(systemMonitorID interface{}) bool {
	log.Printf("DEBUG: Starting unlicenseSystemMonitor for agent %s", idToString(systemMonitorID))
	// First, GET the system monitor to get the complete object
	getURL := fmt.Sprintf("https://%s:%d/lr-admin-api/agents/%s", config.Hostname, config.Port, idToString(systemMonitorID))
	log.Printf("DEBUG: GET URL for agent %s: %s", idToString(systemMonitorID), getURL)

	req, err := http.NewRequest("GET", getURL, nil)
	if err != nil {
		log.Printf("Error creating GET request for system monitor %s: %v", idToString(systemMonitorID), err)
		return false
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Error getting system monitor %s: %v", idToString(systemMonitorID), err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to get system monitor %s, status: %d", idToString(systemMonitorID), resp.StatusCode)
		return false
	}

	// Parse the response
	var systemMonitor map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&systemMonitor); err != nil {
		log.Printf("Error decoding system monitor %s: %v", idToString(systemMonitorID), err)
		return false
	}

	// Modify the system monitor object to unlicense it
	// Set recordStatusName to "Unlicensed" (this is the LogRhythm way to unlicense)
	log.Printf("DEBUG: Setting recordStatusName to 'Unlicensed' for agent %s", idToString(systemMonitorID))
	systemMonitor["recordStatusName"] = "Unlicensed"

	// PUT the updated system monitor back
	putURL := fmt.Sprintf("https://%s:%d/lr-admin-api/agents/%s", config.Hostname, config.Port, idToString(systemMonitorID))
	log.Printf("DEBUG: PUT URL for agent %s: %s", idToString(systemMonitorID), putURL)

	jsonData, err := json.Marshal(systemMonitor)
	if err != nil {
		log.Printf("Error marshaling updated system monitor %s: %v", idToString(systemMonitorID), err)
		return false
	}
	log.Printf("DEBUG: PUT payload for agent %s: %s", idToString(systemMonitorID), string(jsonData))

	req, err = http.NewRequest("PUT", putURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating PUT request for system monitor %s: %v", idToString(systemMonitorID), err)
		return false
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = httpClient.Do(req)
	if err != nil {
		log.Printf("Error unlicensing system monitor %s: %v", idToString(systemMonitorID), err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("Successfully unlicensed system monitor %s", idToString(systemMonitorID))
		log.Printf("DEBUG: Agent %s unlicensing completed successfully", idToString(systemMonitorID))
		return true
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("Failed to unlicense system monitor %s, status: %d", idToString(systemMonitorID), resp.StatusCode)
		log.Printf("DEBUG: Agent %s unlicensing failed with response: %s", idToString(systemMonitorID), string(bodyBytes))
		return false
	}
}

func retireSystemMonitor(systemMonitorID interface{}) bool {
	log.Printf("DEBUG: Starting retireSystemMonitor for agent %s", idToString(systemMonitorID))
	// First, GET the system monitor to get the complete object
	getURL := fmt.Sprintf("https://%s:%d/lr-admin-api/agents/%s", config.Hostname, config.Port, idToString(systemMonitorID))
	log.Printf("DEBUG: GET URL for agent %s: %s", idToString(systemMonitorID), getURL)

	req, err := http.NewRequest("GET", getURL, nil)
	if err != nil {
		log.Printf("Error creating GET request for system monitor %s: %v", idToString(systemMonitorID), err)
		return false
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Error getting system monitor %s: %v", idToString(systemMonitorID), err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to get system monitor %s, status: %d", idToString(systemMonitorID), resp.StatusCode)
		return false
	}

	// Parse the response
	var systemMonitor map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&systemMonitor); err != nil {
		log.Printf("Error decoding system monitor %s: %v", idToString(systemMonitorID), err)
		return false
	}

	// Check if system monitor is already retired
	if recordStatusName, ok := systemMonitor["recordStatusName"].(string); ok && recordStatusName == "Retired" {
		log.Printf("System monitor %s is already retired, skipping retirement", idToString(systemMonitorID))
		return true // Success - already retired
	}

	// Modify the system monitor object to retire it
	// Set recordStatusName to "Retired" and licenseType to "None"
	log.Printf("DEBUG: Setting recordStatusName to 'Retired' and licenseType to 'None' for agent %s", idToString(systemMonitorID))
	systemMonitor["recordStatusName"] = "Retired"
	systemMonitor["licenseType"] = "None"

	// PUT the updated system monitor back
	putURL := fmt.Sprintf("https://%s:%d/lr-admin-api/agents/%s", config.Hostname, config.Port, idToString(systemMonitorID))
	log.Printf("DEBUG: PUT URL for agent %s: %s", idToString(systemMonitorID), putURL)

	jsonData, err := json.Marshal(systemMonitor)
	if err != nil {
		log.Printf("Error marshaling updated system monitor %s: %v", idToString(systemMonitorID), err)
		return false
	}
	log.Printf("DEBUG: PUT payload for agent %s: %s", idToString(systemMonitorID), string(jsonData))

	req, err = http.NewRequest("PUT", putURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating PUT request for system monitor %s: %v", idToString(systemMonitorID), err)
		return false
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = httpClient.Do(req)
	if err != nil {
		log.Printf("Error updating system monitor %s: %v", idToString(systemMonitorID), err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("Successfully retired system monitor %s", idToString(systemMonitorID))
		log.Printf("DEBUG: Agent %s retirement completed successfully", idToString(systemMonitorID))
		return true
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("Failed to update system monitor %s, status: %d", idToString(systemMonitorID), resp.StatusCode)
		log.Printf("DEBUG: Agent %s retirement failed with response: %s", idToString(systemMonitorID), string(bodyBytes))
		return false
	}
}

func removeHostIdentifiers(hostID interface{}) []HostIdentifier {
	// First, GET the host to get all identifiers
	getURL := fmt.Sprintf("https://%s:%d/lr-admin-api/hosts/%s", config.Hostname, config.Port, idToString(hostID))

	req, err := http.NewRequest("GET", getURL, nil)
	if err != nil {
		log.Printf("Error creating GET request for host %s: %v", idToString(hostID), err)
		return []HostIdentifier{}
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Error getting host %s: %v", idToString(hostID), err)
		return []HostIdentifier{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to get host %s, status: %d", idToString(hostID), resp.StatusCode)
		return []HostIdentifier{}
	}

	// Parse the response to get identifiers
	var host map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&host); err != nil {
		log.Printf("Error decoding host %s: %v", idToString(hostID), err)
		return []HostIdentifier{}
	}

	// Check if host has identifiers (note: it's "hostIdentifiers" not "identifiers")
	hostIdentifiers, hasIdentifiers := host["hostIdentifiers"]
	if !hasIdentifiers {
		log.Printf("Host %s has no identifiers to remove", idToString(hostID))
		return []HostIdentifier{} // Success - no identifiers to remove
	}

	identifiersArray, ok := hostIdentifiers.([]interface{})
	if !ok || len(identifiersArray) == 0 {
		log.Printf("Host %s has no identifiers to remove", idToString(hostID))
		return []HostIdentifier{} // Success - no identifiers to remove
	}

	// Find IPAddress identifiers to remove (skip those already retired)
	var ipAddressIdentifiers []map[string]interface{}
	for _, identifier := range identifiersArray {
		identifierMap, ok := identifier.(map[string]interface{})
		if !ok {
			continue
		}

		identifierType, hasType := identifierMap["type"]
		if !hasType {
			continue
		}

		// Only remove IPAddress identifiers
		if identifierType == "IPAddress" {
			// Skip IP addresses that already have a dateRetired (already retired)
			if dateRetired, hasDateRetired := identifierMap["dateRetired"]; hasDateRetired && dateRetired != nil {
				log.Printf("Skipping IP address %v (already retired on %v)", identifierMap["value"], dateRetired)
				continue
			}
			ipAddressIdentifiers = append(ipAddressIdentifiers, identifierMap)
		}
	}

	if len(ipAddressIdentifiers) == 0 {
		log.Printf("Host %s has no IPAddress identifiers to remove", idToString(hostID))
		return []HostIdentifier{} // Success - no IPAddress identifiers to remove
	}

	log.Printf("Host %s has %d IPAddress identifiers to remove", idToString(hostID), len(ipAddressIdentifiers))

	// Create payload for removing IPAddress identifiers with their actual IP address values
	var hostIdentifiersPayload []map[string]interface{}
	var removedIdentifiers []HostIdentifier
	for _, identifier := range ipAddressIdentifiers {
		ipAddress, hasValue := identifier["value"]
		if hasValue {
			hostIdentifiersPayload = append(hostIdentifiersPayload, map[string]interface{}{
				"type":  "IPAddress",
				"value": ipAddress, // Use the actual IP address value, not the hostIdentifierId
			})
			// Track the identifiers that will be removed for rollback
			removedIdentifiers = append(removedIdentifiers, HostIdentifier{
				Type:  "IPAddress",
				Value: ipAddress.(string),
			})
		}
	}

	payload := map[string]interface{}{
		"hostIdentifiers": hostIdentifiersPayload,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling identifier removal payload for host %s: %v", idToString(hostID), err)
		return []HostIdentifier{}
	}

	// DELETE the IPAddress identifiers
	deleteURL := fmt.Sprintf("https://%s:%d/lr-admin-api/hosts/%s/identifiers",
		config.Hostname, config.Port, idToString(hostID))

	log.Printf("DELETE URL: %s", deleteURL)
	log.Printf("DELETE Payload: %s", string(jsonData))

	req, err = http.NewRequest("DELETE", deleteURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating DELETE request for host identifiers %s: %v", idToString(hostID), err)
		return []HostIdentifier{}
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = httpClient.Do(req)
	if err != nil {
		log.Printf("Error removing IPAddress identifiers from host %s: %v", idToString(hostID), err)
		return []HostIdentifier{}
	}
	defer resp.Body.Close()

	// Read response body for debugging
	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("DELETE Response Status: %d", resp.StatusCode)
	log.Printf("DELETE Response Body: %s", string(bodyBytes))

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		log.Printf("Successfully removed %d IPAddress identifiers from host %s", len(removedIdentifiers), idToString(hostID))
		return removedIdentifiers
	} else {
		log.Printf("Failed to remove IPAddress identifiers from host %s, status: %d", idToString(hostID), resp.StatusCode)
		return []HostIdentifier{} // Return empty list on failure
	}
}

func updateHost(hostID interface{}) (bool, []HostIdentifier) {
	// First, remove all identifiers from the host
	removedIdentifiers := removeHostIdentifiers(hostID)
	if len(removedIdentifiers) == 0 {
		log.Printf("No identifiers were removed from host %s", idToString(hostID))
	}

	// Use the correct LogRhythm API for retiring hosts
	// Based on the example, we need to use the specific host endpoint
	hostURL := fmt.Sprintf("https://%s:%d/lr-admin-api/hosts/%s", config.Hostname, config.Port, idToString(hostID))

	// GET the host first to get the complete object
	getReq, err := http.NewRequest("GET", hostURL, nil)
	if err != nil {
		log.Printf("Error creating GET request for host %s: %v", idToString(hostID), err)
		return false, []HostIdentifier{}
	}

	getReq.Header.Set("Authorization", "Bearer "+config.APIKey)
	getReq.Header.Set("Content-Type", "application/json")

	getResp, err := httpClient.Do(getReq)
	if err != nil {
		log.Printf("Error getting host %s: %v", idToString(hostID), err)
		return false, []HostIdentifier{}
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		log.Printf("Failed to get host %s, status: %d", idToString(hostID), getResp.StatusCode)
		return false, []HostIdentifier{}
	}

	// Parse the response
	var host map[string]interface{}
	if err := json.NewDecoder(getResp.Body).Decode(&host); err != nil {
		log.Printf("Error decoding host %s: %v", idToString(hostID), err)
		return false, []HostIdentifier{}
	}

	// Check if host is already retired
	if recordStatusName, ok := host["recordStatusName"].(string); ok && recordStatusName == "Retired" {
		log.Printf("Host %s is already retired, skipping retirement", idToString(hostID))
		return true, []HostIdentifier{} // Success - already retired
	}

	// Update the recordStatusName to "Retired"
	host["recordStatusName"] = "Retired"

	// Also update the name to indicate retirement
	if name, ok := host["name"].(string); ok {
		// Check if already retired to prevent duplicate "Retired by LRCleaner" additions
		if !strings.Contains(name, "Retired by LRCleaner") {
			host["name"] = name + " Retired by LRCleaner"
		}
	}

	// Remove fields that are not allowed in PUT request
	delete(host, "hostRoles")
	delete(host, "hostIdentifiers")

	// PUT the updated host back
	jsonData, err := json.Marshal(host)
	if err != nil {
		log.Printf("Error marshaling updated host %s: %v", idToString(hostID), err)
		return false, []HostIdentifier{}
	}

	log.Printf("PUT Request Data for host %s: %s", idToString(hostID), string(jsonData))

	req, err := http.NewRequest("PUT", hostURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating PUT request for host %s: %v", idToString(hostID), err)
		return false, []HostIdentifier{}
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Error updating host %s: %v", idToString(hostID), err)
		return false, []HostIdentifier{}
	}
	defer resp.Body.Close()

	// Read response body for debugging
	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("PUT Response Status: %d", resp.StatusCode)
	log.Printf("PUT Response Body: %s", string(bodyBytes))

	if resp.StatusCode == http.StatusOK {
		log.Printf("Successfully retired host %s", idToString(hostID))
		return true, removedIdentifiers
	} else {
		log.Printf("Failed to update host %s, status: %d", idToString(hostID), resp.StatusCode)
		return false, []HostIdentifier{}
	}
}

func updateLogSource(logSourceID interface{}) bool {
	// First, GET the log source to get the complete object
	getURL := fmt.Sprintf("https://%s:%d/lr-admin-api/logsources/%s", config.Hostname, config.Port, idToString(logSourceID))

	req, err := http.NewRequest("GET", getURL, nil)
	if err != nil {
		log.Printf("Error creating GET request for log source %s: %v", idToString(logSourceID), err)
		return false
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Error getting log source %s: %v", idToString(logSourceID), err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to get log source %s, status: %d", idToString(logSourceID), resp.StatusCode)
		return false
	}

	// Parse the response
	var logSource map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&logSource); err != nil {
		log.Printf("Error decoding log source %s: %v", idToString(logSourceID), err)
		return false
	}

	// Modify the log source object
	if name, ok := logSource["name"].(string); ok {
		// Check if already retired to prevent duplicate "Retired by LRCleaner" additions
		if !strings.Contains(name, "Retired by LRCleaner") {
			logSource["name"] = name + " Retired by LRCleaner"
		}
	}

	// Only set status to Retired if not already retired
	if recordStatus, ok := logSource["recordStatus"].(string); ok {
		if recordStatus != "Retired" {
			logSource["recordStatus"] = "Retired"
		}
	} else {
		logSource["recordStatus"] = "Retired"
	}

	// PUT the updated log source back
	putURL := fmt.Sprintf("https://%s:%d/lr-admin-api/logsources/%s", config.Hostname, config.Port, idToString(logSourceID))

	jsonData, err := json.Marshal(logSource)
	if err != nil {
		log.Printf("Error marshaling updated log source %s: %v", idToString(logSourceID), err)
		return false
	}

	req, err = http.NewRequest("PUT", putURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating PUT request for log source %s: %v", idToString(logSourceID), err)
		return false
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = httpClient.Do(req)
	if err != nil {
		log.Printf("Error updating log source %s: %v", idToString(logSourceID), err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("Successfully retired log source %s", idToString(logSourceID))
		return true
	} else {
		log.Printf("Failed to update log source %s, status: %d", idToString(logSourceID), resp.StatusCode)
		return false
	}
}

func generateTextReport(job *JobStatus) []byte {
	// Generate a simple text-based report

	report := "LRCleaner Retirement Report\n"
	report += "==========================\n\n"
	report += fmt.Sprintf("Job ID: %s\n", job.ID)
	report += fmt.Sprintf("Completed: %s\n", job.EndTime.Format("2006-01-02 15:04:05"))
	report += fmt.Sprintf("Total Log Sources Retired: %d\n\n", len(job.RetirementRecords))
	report += "Retirement Summary:\n"
	report += "==================\n\n"

	// Group by host
	hostMap := make(map[string][]RetirementRecord)
	for _, record := range job.RetirementRecords {
		hostMap[record.HostName] = append(hostMap[record.HostName], record)
	}

	for hostName, records := range hostMap {
		report += fmt.Sprintf("Host: %s (%d log sources)\n", hostName, len(records))
		report += "----------------------------------------\n"

		for _, record := range records {
			report += fmt.Sprintf("  Log Source ID: %s\n", idToString(record.LogSourceID))
			report += fmt.Sprintf("  Original Name: %s\n", record.OriginalName)
			report += fmt.Sprintf("  Retired Name: %s\n", record.RetiredName)
			report += fmt.Sprintf("  Status: %s -> %s\n", record.OriginalStatus, record.RetiredStatus)
			report += fmt.Sprintf("  Timestamp: %s\n\n", record.Timestamp.Format("2006-01-02 15:04:05"))
		}
		report += "\n"
	}

	report += "Backup Recommendation:\n"
	report += "=====================\n"
	report += "Before making any changes, it is recommended to backup the LogRhythmEMDB database:\n\n"
	report += "1. Stop LogRhythm services\n"
	report += "2. Backup the LogRhythmEMDB database\n"
	report += "3. Document the backup location and timestamp\n"
	report += "4. Restart LogRhythm services\n\n"
	report += "This backup will allow you to restore the system to its previous state if needed.\n\n"
	report += fmt.Sprintf("Generated by LRCleaner on %s\n", time.Now().Format("2006-01-02 15:04:05"))

	return []byte(report)
}

// Rollback Functions

func createRollbackData(jobID string, selectedHosts []string) *RollbackData {
	if !config.Rollback.Enabled {
		return nil
	}

	// Get the host analysis from the previous job
	var hostAnalysis []HostAnalysis
	jobsMutex.RLock()
	for _, otherJob := range jobs {
		if len(otherJob.HostAnalysis) > 0 {
			hostAnalysis = otherJob.HostAnalysis
			break
		}
	}
	jobsMutex.RUnlock()

	if len(hostAnalysis) == 0 {
		log.Printf("No host analysis found for rollback data creation")
		return nil
	}

	// Create map of selected hosts
	selectedMap := make(map[string]bool)
	for _, hostID := range selectedHosts {
		selectedMap[hostID] = true
	}

	// Find hosts to retire
	var hostsToRetire []HostAnalysis
	for _, host := range hostAnalysis {
		if selectedMap[idToString(host.HostID)] {
			hostsToRetire = append(hostsToRetire, host)
		}
	}

	rollbackID := fmt.Sprintf("rollback_%d", time.Now().Unix())
	rollbackData := &RollbackData{
		ID:            rollbackID,
		Timestamp:     time.Now(),
		OperationType: "retirement",
		User:          "system", // TODO: Get actual user
		Description:   fmt.Sprintf("Retirement of %d hosts with %d log sources", len(hostsToRetire), getTotalLogSources(hostsToRetire)),
		JobID:         jobID,
		Checksum:      "", // Will be calculated when saving
	}

	// Capture log source changes
	for _, host := range hostsToRetire {
		for _, logSource := range host.LogSources {
			logSourceChange := LogSourceRollback{
				LogSourceID:     logSource.ID,
				HostID:          host.HostID,
				HostName:        host.HostName,
				OriginalName:    logSource.Name,
				OriginalStatus:  logSource.RecordStatus,
				CurrentName:     logSource.Name,         // Will be updated after retirement
				CurrentStatus:   logSource.RecordStatus, // Will be updated after retirement
				SystemMonitorID: logSource.SystemMonitorID,
			}
			rollbackData.LogSourceChanges = append(rollbackData.LogSourceChanges, logSourceChange)
		}
	}

	// Capture host changes
	for _, host := range hostsToRetire {
		// Get original host data
		originalHostData := getHostData(host.HostID)

		// Get the actually removed identifiers from the retirement process
		removedIdentifiers := removedIdentifiersMap[idToString(host.HostID)]
		if removedIdentifiers == nil {
			removedIdentifiers = []HostIdentifier{} // Default to empty if not found
		}

		hostChange := HostRollback{
			HostID:              host.HostID,
			HostName:            host.HostName,
			OriginalName:        originalHostData["name"].(string),
			OriginalStatus:      originalHostData["recordStatusName"].(string),
			OriginalIdentifiers: extractHostIdentifiers(originalHostData),
			RetiredIdentifiers:  removedIdentifiers,                            // Only the identifiers that were actually removed
			CurrentName:         originalHostData["name"].(string),             // Will be updated after retirement
			CurrentStatus:       originalHostData["recordStatusName"].(string), // Will be updated after retirement
		}
		rollbackData.HostChanges = append(rollbackData.HostChanges, hostChange)
	}

	// Clear the removed identifiers map after creating rollback data
	removedIdentifiersMap = make(map[string][]HostIdentifier)

	return rollbackData
}

func getTotalLogSources(hosts []HostAnalysis) int {
	total := 0
	for _, host := range hosts {
		total += host.LogSourceCount
	}
	return total
}

func getHostData(hostID interface{}) map[string]interface{} {
	url := fmt.Sprintf("https://%s:%d/lr-admin-api/hosts/%s", config.Hostname, config.Port, idToString(hostID))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Error creating request for host %s: %v", idToString(hostID), err)
		return make(map[string]interface{})
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Error getting host %s: %v", idToString(hostID), err)
		return make(map[string]interface{})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to get host %s, status: %d", idToString(hostID), resp.StatusCode)
		return make(map[string]interface{})
	}

	var hostData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&hostData); err != nil {
		log.Printf("Error decoding host %s: %v", idToString(hostID), err)
		return make(map[string]interface{})
	}

	return hostData
}

func extractHostIdentifiers(hostData map[string]interface{}) []HostIdentifier {
	var identifiers []HostIdentifier

	if hostIdentifiers, ok := hostData["hostIdentifiers"].([]interface{}); ok {
		for _, identifier := range hostIdentifiers {
			if identifierMap, ok := identifier.(map[string]interface{}); ok {
				if identifierType, hasType := identifierMap["type"].(string); hasType {
					if identifierValue, hasValue := identifierMap["value"].(string); hasValue {
						identifiers = append(identifiers, HostIdentifier{
							Type:  identifierType,
							Value: identifierValue,
						})
					}
				}
			}
		}
	}

	return identifiers
}

// extractRetiredIPIdentifiers extracts only IP address identifiers that were actually retired
// and removes duplicates
func extractRetiredIPIdentifiers(hostData map[string]interface{}) []HostIdentifier {
	var identifiers []HostIdentifier
	seenValues := make(map[string]bool) // Track seen values to remove duplicates

	if hostIdentifiers, ok := hostData["hostIdentifiers"].([]interface{}); ok {
		for _, identifier := range hostIdentifiers {
			if identifierMap, ok := identifier.(map[string]interface{}); ok {
				if identifierType, hasType := identifierMap["type"].(string); hasType {
					if identifierValue, hasValue := identifierMap["value"].(string); hasValue {
						// Only include IPAddress identifiers
						if identifierType == "IPAddress" {
							// Skip duplicates
							if !seenValues[identifierValue] {
								seenValues[identifierValue] = true
								identifiers = append(identifiers, HostIdentifier{
									Type:  identifierType,
									Value: identifierValue,
								})
							}
						}
					}
				}
			}
		}
	}

	return identifiers
}

func saveRollbackData(rollbackData *RollbackData) {
	// Create rollback directory if it doesn't exist
	rollbackDir := config.Rollback.BackupLocation
	if err := os.MkdirAll(rollbackDir, 0755); err != nil {
		log.Printf("Error creating rollback directory: %v", err)
		return
	}

	// Calculate checksum
	jsonData, err := json.Marshal(rollbackData)
	if err != nil {
		log.Printf("Error marshaling rollback data: %v", err)
		return
	}

	rollbackData.Checksum = calculateChecksum(jsonData)

	// Save to file
	filename := fmt.Sprintf("LRCleaner_rollback_%s_%s.json",
		rollbackData.Timestamp.Format("20060102_150405"),
		rollbackData.OperationType)
	filepath := filepath.Join(rollbackDir, filename)

	if err := os.WriteFile(filepath, jsonData, 0644); err != nil {
		log.Printf("Error saving rollback data: %v", err)
		return
	}

	// Store in memory
	rollbackMutex.Lock()
	rollbackHistory[rollbackData.ID] = rollbackData
	rollbackMutex.Unlock()

	log.Printf("Rollback data saved: %s", filepath)
}

func calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// Rollback API Handlers

func handleRollbackHistory(w http.ResponseWriter, r *http.Request) {
	rollbackMutex.RLock()
	defer rollbackMutex.RUnlock()

	var history []map[string]interface{}
	for _, rollback := range rollbackHistory {
		history = append(history, map[string]interface{}{
			"id":             rollback.ID,
			"timestamp":      rollback.Timestamp,
			"operation":      rollback.OperationType,
			"description":    rollback.Description,
			"user":           rollback.User,
			"logSources":     len(rollback.LogSourceChanges),
			"hosts":          len(rollback.HostChanges),
			"systemMonitors": len(rollback.SystemMonitorChanges),
		})
	}

	// Sort by timestamp (newest first)
	sort.Slice(history, func(i, j int) bool {
		return history[i]["timestamp"].(time.Time).After(history[j]["timestamp"].(time.Time))
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

func handleRollbackDetails(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	rollbackID := vars["rollbackId"]

	rollbackMutex.RLock()
	rollback, exists := rollbackHistory[rollbackID]
	rollbackMutex.RUnlock()

	if !exists {
		http.Error(w, "Rollback not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rollback)
}

func handleExecuteRollback(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	rollbackID := vars["rollbackId"]

	rollbackMutex.RLock()
	rollback, exists := rollbackHistory[rollbackID]
	rollbackMutex.RUnlock()

	if !exists {
		http.Error(w, "Rollback not found", http.StatusNotFound)
		return
	}

	// Execute rollback
	success := executeRollback(rollback)

	if success {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Rollback executed successfully"})
	} else {
		http.Error(w, "Rollback execution failed", http.StatusInternalServerError)
	}
}

func handleDeleteRollback(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	rollbackID := vars["rollbackId"]

	rollbackMutex.Lock()
	defer rollbackMutex.Unlock()

	if rollback, exists := rollbackHistory[rollbackID]; exists {
		// Delete file
		filename := fmt.Sprintf("LRCleaner_rollback_%s_%s.json",
			rollback.Timestamp.Format("20060102_150405"),
			rollback.OperationType)
		filepath := filepath.Join(config.Rollback.BackupLocation, filename)
		os.Remove(filepath)

		// Remove from memory
		delete(rollbackHistory, rollbackID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Rollback deleted"})
	} else {
		http.Error(w, "Rollback not found", http.StatusNotFound)
	}
}

func executeRollback(rollback *RollbackData) bool {
	log.Printf("Executing rollback: %s", rollback.ID)

	success := true

	// Rollback log sources
	for _, logSourceChange := range rollback.LogSourceChanges {
		if !rollbackLogSource(logSourceChange) {
			success = false
			log.Printf("Failed to rollback log source: %s", logSourceChange.LogSourceID)
		}
	}

	// Rollback hosts
	for _, hostChange := range rollback.HostChanges {
		if !rollbackHost(hostChange) {
			success = false
			log.Printf("Failed to rollback host: %s", hostChange.HostID)
		}
	}

	// Rollback system monitors
	for _, systemMonitorChange := range rollback.SystemMonitorChanges {
		if !rollbackSystemMonitor(systemMonitorChange) {
			success = false
			log.Printf("Failed to rollback system monitor: %s", systemMonitorChange.SystemMonitorID)
		}
	}

	if success {
		log.Printf("Rollback completed successfully: %s", rollback.ID)
	} else {
		log.Printf("Rollback completed with errors: %s", rollback.ID)
	}

	return success
}

func rollbackLogSource(change LogSourceRollback) bool {
	// Get current log source data
	url := fmt.Sprintf("https://%s:%d/lr-admin-api/logsources/%s", config.Hostname, config.Port, idToString(change.LogSourceID))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Error creating GET request for log source %s: %v", idToString(change.LogSourceID), err)
		return false
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Error getting log source %s: %v", idToString(change.LogSourceID), err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to get log source %s, status: %d", idToString(change.LogSourceID), resp.StatusCode)
		return false
	}

	var logSource map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&logSource); err != nil {
		log.Printf("Error decoding log source %s: %v", idToString(change.LogSourceID), err)
		return false
	}

	// Restore original values
	logSource["name"] = change.OriginalName
	logSource["recordStatus"] = change.OriginalStatus

	// Remove "Retired by LRCleaner" suffix if present
	if strings.Contains(logSource["name"].(string), "Retired by LRCleaner") {
		logSource["name"] = strings.Replace(logSource["name"].(string), " Retired by LRCleaner", "", 1)
	}

	// PUT the updated log source back
	jsonData, err := json.Marshal(logSource)
	if err != nil {
		log.Printf("Error marshaling updated log source %s: %v", idToString(change.LogSourceID), err)
		return false
	}

	req, err = http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating PUT request for log source %s: %v", idToString(change.LogSourceID), err)
		return false
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = httpClient.Do(req)
	if err != nil {
		log.Printf("Error updating log source %s: %v", idToString(change.LogSourceID), err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("Successfully rolled back log source %s", idToString(change.LogSourceID))
		return true
	} else {
		log.Printf("Failed to rollback log source %s, status: %d", idToString(change.LogSourceID), resp.StatusCode)
		return false
	}
}

func rollbackHost(change HostRollback) bool {
	// Get current host data
	url := fmt.Sprintf("https://%s:%d/lr-admin-api/hosts/%s", config.Hostname, config.Port, idToString(change.HostID))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Error creating GET request for host %s: %v", idToString(change.HostID), err)
		return false
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Error getting host %s: %v", idToString(change.HostID), err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to get host %s, status: %d", idToString(change.HostID), resp.StatusCode)
		return false
	}

	var host map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&host); err != nil {
		log.Printf("Error decoding host %s: %v", idToString(change.HostID), err)
		return false
	}

	// Restore original values
	host["name"] = change.OriginalName
	host["recordStatusName"] = change.OriginalStatus

	// Remove "Retired by LRCleaner" suffix if present
	if strings.Contains(host["name"].(string), "Retired by LRCleaner") {
		host["name"] = strings.Replace(host["name"].(string), " Retired by LRCleaner", "", 1)
	}

	// Remove hostIdentifiers from the main host update request
	// We'll add them back using the correct API endpoint after the host is updated
	delete(host, "hostIdentifiers")

	// Remove fields that are not allowed in PUT request or might cause validation issues
	delete(host, "hostRoles")
	delete(host, "id")
	delete(host, "createdDate")
	delete(host, "lastUpdatedDate")
	delete(host, "lastUpdatedBy")
	delete(host, "createdBy")
	delete(host, "recordStatus")

	// Ensure we have the required fields with correct values
	host["name"] = change.OriginalName
	host["recordStatusName"] = change.OriginalStatus

	// PUT the updated host back
	jsonData, err := json.Marshal(host)
	if err != nil {
		log.Printf("Error marshaling updated host %s: %v", idToString(change.HostID), err)
		return false
	}

	// Log the data being sent for debugging
	log.Printf("Rolling back host %s with data: %s", idToString(change.HostID), string(jsonData))

	req, err = http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating PUT request for host %s: %v", idToString(change.HostID), err)
		return false
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = httpClient.Do(req)
	if err != nil {
		log.Printf("Error updating host %s: %v", idToString(change.HostID), err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("Successfully updated host %s", idToString(change.HostID))

		// Now restore the retired identifiers using the correct API endpoint
		if len(change.RetiredIdentifiers) > 0 {
			if restoreHostIdentifiers(change.HostID, change.RetiredIdentifiers) {
				log.Printf("Successfully restored %d identifiers for host %s", len(change.RetiredIdentifiers), idToString(change.HostID))
			} else {
				log.Printf("Failed to restore identifiers for host %s, but host was updated successfully", idToString(change.HostID))
			}
		}

		log.Printf("Successfully rolled back host %s", idToString(change.HostID))
		return true
	} else {
		// Read the response body to get more details about the error
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			log.Printf("Failed to rollback host %s, status: %d (could not read response body)", idToString(change.HostID), resp.StatusCode)
		} else {
			log.Printf("Failed to rollback host %s, status: %d, response: %s", idToString(change.HostID), resp.StatusCode, string(body))
		}
		return false
	}
}

// restoreHostIdentifiers adds back the retired identifiers to a host
func restoreHostIdentifiers(hostID interface{}, identifiers []HostIdentifier) bool {
	if len(identifiers) == 0 {
		return true // Nothing to restore
	}

	// Create payload for adding identifiers
	var hostIdentifiersPayload []map[string]interface{}
	for _, identifier := range identifiers {
		hostIdentifiersPayload = append(hostIdentifiersPayload, map[string]interface{}{
			"type":  identifier.Type,
			"value": identifier.Value,
		})
	}

	payload := map[string]interface{}{
		"hostIdentifiers": hostIdentifiersPayload,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling host identifiers payload for host %s: %v", idToString(hostID), err)
		return false
	}

	// Use the correct LogRhythm API endpoint for updating host identifiers
	url := fmt.Sprintf("https://%s:%d/lr-admin-api/hosts/%s/identifiers", config.Hostname, config.Port, idToString(hostID))

	log.Printf("POST URL: %s", url)
	log.Printf("POST Payload: %s", string(jsonData))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating POST request for host identifiers %s: %v", idToString(hostID), err)
		return false
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Error adding identifiers to host %s: %v", idToString(hostID), err)
		return false
	}
	defer resp.Body.Close()

	// Read response body for debugging
	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("POST Response Status: %d", resp.StatusCode)
	log.Printf("POST Response Body: %s", string(bodyBytes))

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		log.Printf("Successfully added %d identifiers to host %s", len(identifiers), idToString(hostID))
		return true
	} else {
		log.Printf("Failed to add identifiers to host %s, status: %d", idToString(hostID), resp.StatusCode)
		return false
	}
}

func rollbackSystemMonitor(change SystemMonitorRollback) bool {
	// Get current system monitor data
	url := fmt.Sprintf("https://%s:%d/lr-admin-api/agents/%s", config.Hostname, config.Port, idToString(change.SystemMonitorID))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Error creating GET request for system monitor %s: %v", idToString(change.SystemMonitorID), err)
		return false
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Error getting system monitor %s: %v", idToString(change.SystemMonitorID), err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to get system monitor %s, status: %d", idToString(change.SystemMonitorID), resp.StatusCode)
		return false
	}

	var systemMonitor map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&systemMonitor); err != nil {
		log.Printf("Error decoding system monitor %s: %v", idToString(change.SystemMonitorID), err)
		return false
	}

	// Restore original values
	systemMonitor["recordStatusName"] = change.OriginalStatus
	systemMonitor["licenseType"] = change.OriginalLicenseType

	// PUT the updated system monitor back
	jsonData, err := json.Marshal(systemMonitor)
	if err != nil {
		log.Printf("Error marshaling updated system monitor %s: %v", idToString(change.SystemMonitorID), err)
		return false
	}

	req, err = http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating PUT request for system monitor %s: %v", idToString(change.SystemMonitorID), err)
		return false
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = httpClient.Do(req)
	if err != nil {
		log.Printf("Error updating system monitor %s: %v", idToString(change.SystemMonitorID), err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("Successfully rolled back system monitor %s", idToString(change.SystemMonitorID))
		return true
	} else {
		log.Printf("Failed to rollback system monitor %s, status: %d", idToString(change.SystemMonitorID), resp.StatusCode)
		return false
	}
}
