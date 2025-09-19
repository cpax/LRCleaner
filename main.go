package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	_ "github.com/microsoft/go-mssqldb"
)

// Configuration structure
type Config struct {
	Hostname           string   `json:"hostname"`
	APIKey             string   `json:"apiKey"`
	Port               int      `json:"port"`
	ExcludedLogSources []string `json:"excludedLogSources"`
}

// LogRhythm API structures
type LogSource struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	RecordStatus   string         `json:"recordStatus"`
	MaxLogDate     string         `json:"maxLogDate"`
	Host           Host           `json:"host"`
	LogSourceType  LogSourceType  `json:"logSourceType"`
}

type Host struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type LogSourceType struct {
	Name string `json:"name"`
}

type AnalysisResult struct {
	ID         string `json:"id"`
	HostID     string `json:"hostId"`
	HostName   string `json:"hostName"`
	MaxLogDate string `json:"maxLogDate"`
	PingResult string `json:"pingResult"`
}

type HostAnalysis struct {
	HostID       string `json:"hostId"`
	HostName     string `json:"hostName"`
	LogSourceCount int   `json:"logSourceCount"`
	MaxLogDate   string `json:"maxLogDate"`
	PingResult   string `json:"pingResult"`
	Recommended  bool   `json:"recommended"`
	LogSources   []LogSource `json:"logSources"`
}

type ApplyRequest struct {
	SelectedHosts []string `json:"selectedHosts"`
}

type BackupRequest struct {
	Password string `json:"password"`
	Location string `json:"location"`
}

type RetirementRecord struct {
	LogSourceID   string `json:"logSourceId"`
	HostID        string `json:"hostId"`
	HostName      string `json:"hostName"`
	OriginalName  string `json:"originalName"`
	RetiredName   string `json:"retiredName"`
	OriginalStatus string `json:"originalStatus"`
	RetiredStatus string `json:"retiredStatus"`
	Timestamp     time.Time `json:"timestamp"`
}

type JobStatus struct {
	ID       string    `json:"id"`
	Status   string    `json:"status"`
	Progress int       `json:"progress"`
	Message  string    `json:"message"`
	Results  []AnalysisResult `json:"results,omitempty"`
	HostAnalysis []HostAnalysis `json:"hostAnalysis,omitempty"`
	RetirementRecords []RetirementRecord `json:"retirementRecords,omitempty"`
	Error    string    `json:"error,omitempty"`
	StartTime time.Time `json:"startTime"`
	EndTime   *time.Time `json:"endTime,omitempty"`
}

// Global variables
var (
	config     *Config
	httpClient *http.Client
	jobs       = make(map[string]*JobStatus)
	jobsMutex  sync.RWMutex
	upgrader   = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for local development
		},
	}
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
	
	// If 8080 is not available, find an available port starting from 3000
	fmt.Println("Port 8080 is in use, searching for available port...")
	
	for port := 3000; port <= 65535; port++ {
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

func main() {
	// Find available port
	port := findAvailablePort()
	
	// Initialize configuration
	config = loadConfig()
	
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
	api.HandleFunc("/test", handleTestMode).Methods("POST")
	api.HandleFunc("/backup", handleBackup).Methods("POST")
	api.HandleFunc("/apply", handleApplyMode).Methods("POST")
	api.HandleFunc("/apply/execute", handleExecuteApply).Methods("POST")
	api.HandleFunc("/export/{jobId}", handleExport).Methods("GET")
	api.HandleFunc("/export/pdf/{jobId}", handleExportPDF).Methods("GET")
	api.HandleFunc("/jobs/{jobId}", handleJobStatus).Methods("GET")
	api.HandleFunc("/ws", handleWebSocket)
	
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
	<-quit

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	fmt.Println("LRCleaner stopped")
}

func loadConfig() *Config {
	config := &Config{
		Hostname: "localhost",
		Port:     8500,
		ExcludedLogSources: []string{
			"Open Collector",
			"Echo",
			"AI Engine",
			"LogRhythm System",
		},
	}
	
	// Try to load from file
	if data, err := os.ReadFile("config.json"); err == nil {
		json.Unmarshal(data, config)
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
		data, _ := json.MarshalIndent(config, "", "  ")
		os.WriteFile("config.json", data, 0644)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}
}

func handleTestMode(w http.ResponseWriter, r *http.Request) {
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
	jobID := fmt.Sprintf("test_%d", time.Now().Unix())
	job := &JobStatus{
		ID:        jobID,
		Status:    "running",
		Progress:  0,
		Message:   "Starting analysis...",
		StartTime: time.Now(),
	}
	
	jobsMutex.Lock()
	jobs[jobID] = job
	jobsMutex.Unlock()
	
	// Start analysis in background
	go analyzeLogSources(jobID, selectedDate)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"jobId": jobID})
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
	
	jobsMutex.RLock()
	job, exists := jobs[jobID]
	jobsMutex.RUnlock()
	
	if !exists {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}
	
	if len(job.Results) == 0 {
		http.Error(w, "No results to export", http.StatusBadRequest)
		return
	}
	
	// Generate CSV
	csv := "ID,HostID,HostName,MaxLogDate,PingResult\n"
	for _, result := range job.Results {
		csv += fmt.Sprintf("%s,%s,%s,%s,%s\n", 
			result.ID, result.HostID, result.HostName, result.MaxLogDate, result.PingResult)
	}
	
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

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	defer conn.Close()
	
	// Send current job statuses
	jobsMutex.RLock()
	for _, job := range jobs {
		conn.WriteJSON(job)
	}
	jobsMutex.RUnlock()
	
	// Keep connection alive
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func analyzeLogSources(jobID string, selectedDate time.Time) {
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
		
		// Check excluded sources
		excluded := false
		sourceType := ls.LogSourceType.Name
		sourceName := ls.Name
		
		for _, pattern := range config.ExcludedLogSources {
			if containsIgnoreCase(sourceType, pattern) {
				excluded = true
				break
			}
		}
		
		if !excluded && !containsIgnoreCase(sourceName, "echo") {
			filteredSources = append(filteredSources, ls)
		}
	}
	
	// Update progress
	jobsMutex.Lock()
	job.Progress = 50
	job.Message = "Testing host connectivity..."
	jobsMutex.Unlock()
	
	// Analyze each source
	var results []AnalysisResult
	for i, ls := range filteredSources {
		// Update progress
		jobsMutex.Lock()
		job.Progress = 50 + (i * 50 / len(filteredSources))
		job.Message = fmt.Sprintf("Testing %s...", ls.Host.Name)
		jobsMutex.Unlock()
		
		// Ping test
		pingResult := pingHost(ls.Host.Name)
		
		// Create result
		result := AnalysisResult{
			ID:         ls.ID,
			HostID:     ls.Host.ID,
			HostName:   ls.Host.Name,
			MaxLogDate: ls.MaxLogDate,
			PingResult: pingResult,
		}
		
		results = append(results, result)
	}
	
	// Update job with results
	jobsMutex.Lock()
	job.Results = results
	jobsMutex.Unlock()
	
	// Complete
	jobsMutex.Lock()
	job.Progress = 100
	job.Message = fmt.Sprintf("Analysis complete. Found %d sources.", len(results))
	jobsMutex.Unlock()
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
		
		var response struct {
			Count int         `json:"count"`
			Items []LogSource `json:"items"`
		}
		
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, err
		}
		
		allSources = append(allSources, response.Items...)
		
		if len(response.Items) < count {
			break
		}
		
		offset += count
	}
	
	return allSources, nil
}

func pingHost(hostname string) string {
	// Simple ping implementation - in production you might want to use a proper ping library
	// For now, we'll just return "Unknown" as ping functionality needs to be implemented
	return "Unknown"
}

func containsIgnoreCase(s, substr string) bool {
	// Simple case-insensitive contains check
	// In production, you might want to use regex matching
	return len(s) >= len(substr) && 
		   (s[:len(substr)] == substr || 
		    s[len(s)-len(substr):] == substr)
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
	
	// Filter by date and excluded sources
	var filteredSources []LogSource
	for _, ls := range allLogSources {
		// Check date
		if maxLogDate, err := time.Parse(time.RFC3339, ls.MaxLogDate); err == nil {
			if maxLogDate.After(selectedDate) {
				continue
			}
		}
		
		// Check excluded sources
		excluded := false
		sourceType := ls.LogSourceType.Name
		sourceName := ls.Name
		
		for _, pattern := range config.ExcludedLogSources {
			if containsIgnoreCase(sourceType, pattern) {
				excluded = true
				break
			}
		}
		
		if !excluded && !containsIgnoreCase(sourceName, "echo") {
			filteredSources = append(filteredSources, ls)
		}
	}
	
	// Group by host
	hostMap := make(map[string]*HostAnalysis)
	for _, ls := range filteredSources {
		hostID := ls.Host.ID
		hostName := ls.Host.Name
		
		if hostMap[hostID] == nil {
			hostMap[hostID] = &HostAnalysis{
				HostID:   hostID,
				HostName: hostName,
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
	
	// Update progress
	jobsMutex.Lock()
	job.Progress = 50
	job.Message = "Testing host connectivity..."
	jobsMutex.Unlock()
	
	// Analyze each host
	var hostAnalysis []HostAnalysis
	for _, host := range hostMap {
		// Ping test
		host.PingResult = pingHost(host.HostName)
		
		// Determine if recommended for retirement
		// Recommend if ping fails or no recent logs
		host.Recommended = (host.PingResult == "Failure") || 
						  (host.MaxLogDate != "" && 
						   time.Since(parseTime(host.MaxLogDate)) > 30*24*time.Hour)
		
		hostAnalysis = append(hostAnalysis, *host)
	}
	
	// Update job with host analysis
	jobsMutex.Lock()
	job.Progress = 100
	job.Message = fmt.Sprintf("Host analysis complete. Found %d hosts.", len(hostAnalysis))
	job.HostAnalysis = hostAnalysis
	jobsMutex.Unlock()
}

func executeRetirement(jobID string, selectedHosts []string) {
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
	}()
	
	// Get the host analysis from the previous job
	var hostAnalysis []HostAnalysis
	for _, otherJob := range jobs {
		if len(otherJob.HostAnalysis) > 0 {
			hostAnalysis = otherJob.HostAnalysis
			break
		}
	}
	
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
		if selectedMap[host.HostID] {
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
		
		// Retire all log sources for this host
		for _, logSource := range host.LogSources {
			// Create retirement record before making changes
			record := RetirementRecord{
				LogSourceID:   logSource.ID,
				HostID:        host.HostID,
				HostName:      host.HostName,
				OriginalName:  logSource.Name,
				RetiredName:   logSource.Name + " Retired by LRCleaner",
				OriginalStatus: logSource.RecordStatus,
				RetiredStatus: "Retired",
				Timestamp:     time.Now(),
			}
			
			// Get current log source data
			lsData := logSource
			
			// Update log source
			lsData.Name += " Retired by LRCleaner"
			lsData.RecordStatus = "Retired"
			
			// Update via API
			success := updateLogSource(logSource.ID, lsData)
			if success {
				processedLogSources++
				retirementRecords = append(retirementRecords, record)
			}
		}
	}
	
	// Complete
	jobsMutex.Lock()
	job.Progress = 100
	job.Message = fmt.Sprintf("Retirement complete. Processed %d log sources across %d hosts.", processedLogSources, len(hostsToRetire))
	job.RetirementRecords = retirementRecords
	jobsMutex.Unlock()
}

func parseTime(timeStr string) time.Time {
	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t
	}
	return time.Time{}
}

func updateLogSource(logSourceID string, logSourceData LogSource) bool {
	url := fmt.Sprintf("https://%s:%d/lr-admin-api/logsources/%s", config.Hostname, config.Port, logSourceID)
	
	jsonData, err := json.Marshal(logSourceData)
	if err != nil {
		return false
	}
	
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return false
	}
	
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	return resp.StatusCode == http.StatusOK
}


func generateTextReport(job *JobStatus) []byte {
	// Generate a simple text-based report
	
	report := fmt.Sprintf("LRCleaner Retirement Report\n")
	report += fmt.Sprintf("==========================\n\n")
	report += fmt.Sprintf("Job ID: %s\n", job.ID)
	report += fmt.Sprintf("Completed: %s\n", job.EndTime.Format("2006-01-02 15:04:05"))
	report += fmt.Sprintf("Total Log Sources Retired: %d\n\n", len(job.RetirementRecords))
	report += fmt.Sprintf("Retirement Summary:\n")
	report += fmt.Sprintf("==================\n\n")
	
	// Group by host
	hostMap := make(map[string][]RetirementRecord)
	for _, record := range job.RetirementRecords {
		hostMap[record.HostName] = append(hostMap[record.HostName], record)
	}
	
	for hostName, records := range hostMap {
		report += fmt.Sprintf("Host: %s (%d log sources)\n", hostName, len(records))
		report += fmt.Sprintf("----------------------------------------\n")
		
		for _, record := range records {
			report += fmt.Sprintf("  Log Source ID: %s\n", record.LogSourceID)
			report += fmt.Sprintf("  Original Name: %s\n", record.OriginalName)
			report += fmt.Sprintf("  Retired Name: %s\n", record.RetiredName)
			report += fmt.Sprintf("  Status: %s -> %s\n", record.OriginalStatus, record.RetiredStatus)
			report += fmt.Sprintf("  Timestamp: %s\n\n", record.Timestamp.Format("2006-01-02 15:04:05"))
		}
		report += fmt.Sprintf("\n")
	}
	
	report += fmt.Sprintf("Backup Recommendation:\n")
	report += fmt.Sprintf("=====================\n")
	report += fmt.Sprintf("Before making any changes, it is recommended to backup the LogRhythmEMDB database:\n\n")
	report += fmt.Sprintf("1. Stop LogRhythm services\n")
	report += fmt.Sprintf("2. Backup the LogRhythmEMDB database\n")
	report += fmt.Sprintf("3. Document the backup location and timestamp\n")
	report += fmt.Sprintf("4. Restart LogRhythm services\n\n")
	report += fmt.Sprintf("This backup will allow you to restore the system to its previous state if needed.\n\n")
	report += fmt.Sprintf("Generated by LRCleaner on %s\n", time.Now().Format("2006-01-02 15:04:05"))
	
	return []byte(report)
}
