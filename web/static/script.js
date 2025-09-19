// Global variables
let currentJobId = null;
let websocket = null;
let allResults = [];
let hostAnalysis = [];
let selectedHosts = [];
let retirementRecords = [];

// Initialize the application
document.addEventListener('DOMContentLoaded', function() {
    initializeApp();
});

function initializeApp() {
    // Set default date to today
    const today = new Date().toISOString().split('T')[0];
    document.getElementById('testDate').value = today;
    
    // Load current configuration
    loadConfiguration();
    
    // Setup event listeners
    setupEventListeners();
    
    // Connect to WebSocket
    connectWebSocket();
}

function setupEventListeners() {
    // Configuration form
    document.getElementById('configForm').addEventListener('submit', handleConfigSubmit);
    
    // Control buttons
    document.getElementById('testModeBtn').addEventListener('click', openTestModal);
    document.getElementById('applyModeBtn').addEventListener('click', openApplyModal);
    document.getElementById('exportBtn').addEventListener('click', handleExport);
    document.getElementById('clearBtn').addEventListener('click', clearResults);
    
    // Test modal
    document.getElementById('startTestBtn').addEventListener('click', startTestMode);
    
    // Apply modal
    document.getElementById('performBackupBtn').addEventListener('click', openBackupModal);
    document.getElementById('skipBackupBtn').addEventListener('click', openApplyConfigModal);
    
    // Backup modal
    document.getElementById('executeBackupBtn').addEventListener('click', executeBackup);
    document.getElementById('cancelBackupBtn').addEventListener('click', closeAllModals);
    
    // Apply config modal
    document.getElementById('startApplyBtn').addEventListener('click', startApplyMode);
    document.getElementById('backToBackupBtn').addEventListener('click', backToBackupModal);
    
    // Host selection modal
    document.getElementById('selectAllHosts').addEventListener('change', handleSelectAllRecommended);
    document.getElementById('selectAllBtn').addEventListener('click', selectAllHosts);
    document.getElementById('deselectAllBtn').addEventListener('click', deselectAllHosts);
    document.getElementById('executeRetirementBtn').addEventListener('click', executeRetirement);
    document.getElementById('cancelRetirementBtn').addEventListener('click', closeAllModals);
    
    // Congratulations modal
    document.getElementById('exportReportBtn').addEventListener('click', exportReport);
    document.getElementById('closeCongratulationsBtn').addEventListener('click', closeAllModals);
    
    // Modal close handlers
    document.querySelectorAll('.close').forEach(closeBtn => {
        closeBtn.addEventListener('click', closeAllModals);
    });
    
    // Apply modal
    document.getElementById('startApplyBtn').addEventListener('click', startApplyMode);
    
    // Host selection modal
    document.getElementById('selectAllHosts').addEventListener('change', handleSelectAllRecommended);
    document.getElementById('selectAllBtn').addEventListener('click', selectAllHosts);
    document.getElementById('deselectAllBtn').addEventListener('click', deselectAllHosts);
    document.getElementById('executeRetirementBtn').addEventListener('click', executeRetirement);
    document.getElementById('cancelRetirementBtn').addEventListener('click', closeAllModals);
    
    // Congratulations modal
    document.getElementById('exportPDFBtn').addEventListener('click', exportPDFReport);
    document.getElementById('undoChangesBtn').addEventListener('click', undoChanges);
    document.getElementById('closeCongratulationsBtn').addEventListener('click', closeAllModals);
    
    // Modal click outside to close
    document.querySelectorAll('.modal').forEach(modal => {
        modal.addEventListener('click', function(e) {
            if (e.target === this) closeAllModals();
        });
    });
    
    // Search and filter
    document.getElementById('searchInput').addEventListener('input', filterResults);
    document.getElementById('pingFilter').addEventListener('change', filterResults);
}

function loadConfiguration() {
    fetch('/api/config')
        .then(response => response.json())
        .then(config => {
            document.getElementById('hostname').value = config.hostname || '';
            document.getElementById('port').value = config.port || 8501;
            document.getElementById('apiKey').value = config.apiKey || '';
            
            // Enable buttons if configuration is valid
            const isValid = config.hostname && config.apiKey && config.apiKey !== '';
            document.getElementById('testModeBtn').disabled = !isValid;
            document.getElementById('applyModeBtn').disabled = !isValid;
        })
        .catch(error => {
            console.error('Error loading configuration:', error);
            showToast('Error loading configuration', 'error');
        });
}

function handleConfigSubmit(e) {
    e.preventDefault();
    
    const formData = new FormData(e.target);
    const config = {
        hostname: formData.get('hostname'),
        apiKey: formData.get('apiKey'),
        port: parseInt(formData.get('port'))
    };
    
    // Validate configuration
    if (!config.hostname) {
        showToast('Hostname is required', 'error');
        return;
    }
    
    if (!config.apiKey || config.apiKey.length < 400) {
        showToast('API key must be at least 400 characters', 'error');
        return;
    }
    
    // Save configuration
    fetch('/api/config', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(config)
    })
    .then(response => response.json())
    .then(data => {
        showToast('Configuration saved successfully!', 'success');
        document.getElementById('testModeBtn').disabled = false;
        document.getElementById('applyModeBtn').disabled = false;
    })
    .catch(error => {
        console.error('Error saving configuration:', error);
        showToast('Error saving configuration', 'error');
    });
}

function openTestModal() {
    document.getElementById('testModal').style.display = 'block';
}

function openApplyModal() {
    document.getElementById('applyModal').style.display = 'block';
}

function closeAllModals() {
    document.querySelectorAll('.modal').forEach(modal => {
        modal.style.display = 'none';
    });
}

function startTestMode() {
    const date = document.getElementById('testDate').value;
    if (!date) {
        showToast('Please select a date', 'error');
        return;
    }
    
    closeTestModal();
    showLoadingOverlay();
    
    fetch('/api/test', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ date: date })
    })
    .then(response => response.json())
    .then(data => {
        currentJobId = data.jobId;
        showProgressSection();
        hideLoadingOverlay();
        showToast('Analysis started', 'success');
    })
    .catch(error => {
        console.error('Error starting test mode:', error);
        hideLoadingOverlay();
        showToast('Error starting analysis', 'error');
    });
}

function startApplyMode() {
    const date = document.getElementById('applyDate').value;
    if (!date) {
        showToast('Please select a date', 'error');
        return;
    }
    
    closeAllModals();
    showLoadingOverlay();
    
    fetch('/api/apply', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ date: date })
    })
    .then(response => response.json())
    .then(data => {
        currentJobId = data.jobId;
        showProgressSection();
        hideLoadingOverlay();
        showToast('Host analysis started', 'success');
    })
    .catch(error => {
        console.error('Error starting apply mode:', error);
        hideLoadingOverlay();
        showToast('Error starting host analysis', 'error');
    });
}

function handleExport() {
    if (!currentJobId) {
        showToast('No results to export', 'warning');
        return;
    }
    
    const link = document.createElement('a');
    link.href = `/api/export/${currentJobId}`;
    link.download = `lrcleaner_results_${currentJobId}.csv`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    
    showToast('Export started', 'success');
}

function clearResults() {
    allResults = [];
    updateResultsTable();
    document.getElementById('resultsSummary').textContent = '';
    currentJobId = null;
    hideProgressSection();
    showToast('Results cleared', 'success');
}

function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;
    
    websocket = new WebSocket(wsUrl);
    
    websocket.onopen = function() {
        console.log('WebSocket connected');
    };
    
    websocket.onmessage = function(event) {
        const job = JSON.parse(event.data);
        updateJobProgress(job);
    };
    
    websocket.onclose = function() {
        console.log('WebSocket disconnected');
        // Reconnect after 5 seconds
        setTimeout(connectWebSocket, 5000);
    };
    
    websocket.onerror = function(error) {
        console.error('WebSocket error:', error);
    };
}

function updateJobProgress(job) {
    if (job.id === currentJobId) {
        // Update progress bar
        const progressFill = document.getElementById('progressFill');
        const progressText = document.getElementById('progressText');
        const jobStatus = document.getElementById('jobStatus');
        
        progressFill.style.width = `${job.progress}%`;
        progressText.textContent = job.message;
        
        if (job.results) {
            allResults = job.results;
            updateResultsTable();
            updateResultsSummary();
        }
        
        if (job.hostAnalysis) {
            hostAnalysis = job.hostAnalysis;
            if (job.status === 'completed') {
                showHostSelectionModal();
            }
        }
        
        if (job.retirementRecords) {
            retirementRecords = job.retirementRecords;
            if (job.status === 'completed') {
                showCongratulationsModal();
            }
        }
        
        if (job.status === 'completed') {
            jobStatus.textContent = `Completed at ${new Date(job.endTime).toLocaleString()}`;
            document.getElementById('exportBtn').disabled = false;
        } else if (job.status === 'error') {
            jobStatus.textContent = `Error: ${job.error}`;
            showToast(`Analysis failed: ${job.error}`, 'error');
        }
    }
}

function updateResultsTable() {
    const tbody = document.getElementById('resultsBody');
    tbody.innerHTML = '';
    
    if (allResults.length === 0) {
        tbody.innerHTML = '<tr><td colspan="5" class="no-results">No results yet. Run Test Mode to analyze log sources.</td></tr>';
        return;
    }
    
    allResults.forEach(result => {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td>${result.id}</td>
            <td>${result.hostId}</td>
            <td>${result.hostName}</td>
            <td>${formatDate(result.maxLogDate)}</td>
            <td class="ping-${result.pingResult.toLowerCase()}">${result.pingResult}</td>
        `;
        tbody.appendChild(row);
    });
}

function updateResultsSummary() {
    const summary = document.getElementById('resultsSummary');
    if (allResults.length === 0) {
        summary.textContent = '';
        return;
    }
    
    const successCount = allResults.filter(r => r.pingResult === 'Success').length;
    const failureCount = allResults.filter(r => r.pingResult === 'Failure').length;
    const unknownCount = allResults.filter(r => r.pingResult === 'Unknown').length;
    
    summary.innerHTML = `
        <strong>Summary:</strong> ${allResults.length} total sources | 
        <span class="ping-success">${successCount} Success</span> | 
        <span class="ping-failure">${failureCount} Failure</span> | 
        <span class="ping-unknown">${unknownCount} Unknown</span>
    `;
}

function filterResults() {
    const searchTerm = document.getElementById('searchInput').value.toLowerCase();
    const pingFilter = document.getElementById('pingFilter').value;
    
    const rows = document.querySelectorAll('#resultsBody tr');
    
    rows.forEach(row => {
        const cells = row.querySelectorAll('td');
        if (cells.length === 0) return; // Skip "no results" row
        
        const text = Array.from(cells).map(cell => cell.textContent.toLowerCase()).join(' ');
        const pingResult = cells[4].textContent;
        
        const matchesSearch = text.includes(searchTerm);
        const matchesPing = !pingFilter || pingResult === pingFilter;
        
        row.style.display = (matchesSearch && matchesPing) ? '' : 'none';
    });
}

function showProgressSection() {
    document.getElementById('progressSection').style.display = 'block';
}

function hideProgressSection() {
    document.getElementById('progressSection').style.display = 'none';
}

function showLoadingOverlay() {
    document.getElementById('loadingOverlay').style.display = 'flex';
}

function hideLoadingOverlay() {
    document.getElementById('loadingOverlay').style.display = 'none';
}

function showToast(message, type = 'info') {
    const container = document.getElementById('toastContainer');
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.textContent = message;
    
    container.appendChild(toast);
    
    // Auto remove after 5 seconds
    setTimeout(() => {
        if (toast.parentNode) {
            toast.parentNode.removeChild(toast);
        }
    }, 5000);
}

function formatDate(dateString) {
    if (!dateString) return '';
    const date = new Date(dateString);
    return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
}

// Handle page visibility change to reconnect WebSocket
document.addEventListener('visibilitychange', function() {
    if (!document.hidden && (!websocket || websocket.readyState === WebSocket.CLOSED)) {
        connectWebSocket();
    }
});

// Apply Mode Functions
function openApplyModal() {
    document.getElementById('applyModal').style.display = 'block';
}

function openBackupModal() {
    closeAllModals();
    document.getElementById('backupModal').style.display = 'block';
}

function openApplyConfigModal() {
    closeAllModals();
    document.getElementById('applyConfigModal').style.display = 'block';
}

function backToBackupModal() {
    closeAllModals();
    document.getElementById('applyModal').style.display = 'block';
}

function executeBackup() {
    const password = document.getElementById('backupPassword').value;
    const location = document.getElementById('backupLocation').value;
    
    if (!password) {
        showToast('Please enter the password for logrhythmadmin', 'error');
        return;
    }
    
    closeAllModals();
    showLoadingOverlay();
    
    fetch('/api/backup', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ 
            password: password,
            location: location || 'C:\\LogRhythm\\Backup'
        })
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            showToast('Database backup completed successfully', 'success');
            openApplyConfigModal();
        } else {
            showToast(data.error || 'Backup failed', 'error');
        }
        hideLoadingOverlay();
    })
    .catch(error => {
        console.error('Error performing backup:', error);
        hideLoadingOverlay();
        showToast('Error performing database backup', 'error');
    });
}

function startApplyMode() {
    const date = document.getElementById('applyDate').value;
    if (!date) {
        showToast('Please select a date', 'error');
        return;
    }
    
    closeAllModals();
    showLoadingOverlay();
    
    fetch('/api/apply', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ date: date })
    })
    .then(response => response.json())
    .then(data => {
        currentJobId = data.jobId;
        showProgressSection();
        hideLoadingOverlay();
        showToast('Host analysis started', 'success');
    })
    .catch(error => {
        console.error('Error starting apply mode:', error);
        hideLoadingOverlay();
        showToast('Error starting host analysis', 'error');
    });
}

function closeAllModals() {
    document.querySelectorAll('.modal').forEach(modal => {
        modal.style.display = 'none';
    });
}

// Host Selection Functions
function showHostSelectionModal() {
    populateHostList();
    updateHostSummary();
    document.getElementById('hostSelectionModal').style.display = 'block';
}

function populateHostList() {
    const hostList = document.getElementById('hostList');
    hostList.innerHTML = '';
    
    hostAnalysis.forEach(host => {
        const hostItem = document.createElement('div');
        hostItem.className = 'host-item';
        hostItem.dataset.hostId = host.hostId;
        
        hostItem.innerHTML = `
            <input type="checkbox" class="host-checkbox" data-host-id="${host.hostId}">
            <div class="host-info">
                <div class="host-name">
                    ${host.hostName}
                    <span class="host-recommended ${host.recommended ? 'recommended' : 'not-recommended'}">
                        ${host.recommended ? 'Recommended' : 'Not Recommended'}
                    </span>
                </div>
                <div class="host-details">
                    <div class="host-detail">
                        <i class="fas fa-server"></i>
                        <span>${host.logSourceCount} log sources</span>
                    </div>
                    <div class="host-detail">
                        <i class="fas fa-clock"></i>
                        <span>Last log: ${formatDate(host.maxLogDate)}</span>
                    </div>
                    <div class="host-detail">
                        <i class="fas fa-wifi"></i>
                        <span class="ping-${host.pingResult.toLowerCase()}">${host.pingResult}</span>
                    </div>
                </div>
            </div>
        `;
        
        // Add click handler for the checkbox
        const checkbox = hostItem.querySelector('.host-checkbox');
        checkbox.addEventListener('change', function() {
            updateHostSelection(host.hostId, this.checked);
            updateHostSummary();
        });
        
        hostList.appendChild(hostItem);
    });
}

function updateHostSelection(hostId, selected) {
    if (selected) {
        if (!selectedHosts.includes(hostId)) {
            selectedHosts.push(hostId);
        }
    } else {
        selectedHosts = selectedHosts.filter(id => id !== hostId);
    }
    
    // Update visual state
    const hostItem = document.querySelector(`[data-host-id="${hostId}"]`);
    if (hostItem) {
        hostItem.classList.toggle('selected', selected);
    }
    
    // Update execute button
    document.getElementById('executeRetirementBtn').disabled = selectedHosts.length === 0;
}

function updateHostSummary() {
    const summary = document.getElementById('hostSummary');
    const totalHosts = hostAnalysis.length;
    const recommendedHosts = hostAnalysis.filter(h => h.recommended).length;
    const totalLogSources = selectedHosts.reduce((total, hostId) => {
        const host = hostAnalysis.find(h => h.hostId === hostId);
        return total + (host ? host.logSourceCount : 0);
    }, 0);
    
    summary.innerHTML = `
        <strong>Summary:</strong> ${totalHosts} total hosts | 
        ${recommendedHosts} recommended | 
        ${selectedHosts.length} selected | 
        ${totalLogSources} log sources will be retired
    `;
}

function handleSelectAllRecommended() {
    const selectAll = document.getElementById('selectAllHosts').checked;
    const recommendedHosts = hostAnalysis.filter(h => h.recommended);
    
    recommendedHosts.forEach(host => {
        const checkbox = document.querySelector(`[data-host-id="${host.hostId}"]`);
        if (checkbox) {
            checkbox.checked = selectAll;
            updateHostSelection(host.hostId, selectAll);
        }
    });
    
    updateHostSummary();
}

function selectAllHosts() {
    hostAnalysis.forEach(host => {
        const checkbox = document.querySelector(`[data-host-id="${host.hostId}"]`);
        if (checkbox && !checkbox.checked) {
            checkbox.checked = true;
            updateHostSelection(host.hostId, true);
        }
    });
    updateHostSummary();
}

function deselectAllHosts() {
    selectedHosts.forEach(hostId => {
        const checkbox = document.querySelector(`[data-host-id="${hostId}"]`);
        if (checkbox) {
            checkbox.checked = false;
            updateHostSelection(hostId, false);
        }
    });
    updateHostSummary();
}

function executeRetirement() {
    if (selectedHosts.length === 0) {
        showToast('Please select at least one host', 'warning');
        return;
    }
    
    // Confirm action
    const totalLogSources = selectedHosts.reduce((total, hostId) => {
        const host = hostAnalysis.find(h => h.hostId === hostId);
        return total + (host ? host.logSourceCount : 0);
    }, 0);
    
    if (!confirm(`Are you sure you want to retire ${selectedHosts.length} hosts with ${totalLogSources} log sources? This action cannot be undone.`)) {
        return;
    }
    
    closeAllModals();
    showLoadingOverlay();
    
    fetch('/api/apply/execute', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ selectedHosts: selectedHosts })
    })
    .then(response => response.json())
    .then(data => {
        currentJobId = data.jobId;
        showProgressSection();
        hideLoadingOverlay();
        showToast('Retirement process started', 'success');
    })
    .catch(error => {
        console.error('Error executing retirement:', error);
        hideLoadingOverlay();
        showToast('Error starting retirement process', 'error');
    });
}

// Congratulations Modal Functions
function showCongratulationsModal() {
    populateRetirementSummary();
    document.getElementById('congratulationsModal').style.display = 'block';
}

function populateRetirementSummary() {
    const summary = document.getElementById('retirementSummary');
    
    // Group by host
    const hostMap = {};
    retirementRecords.forEach(record => {
        if (!hostMap[record.hostName]) {
            hostMap[record.hostName] = [];
        }
        hostMap[record.hostName].push(record);
    });
    
    const totalHosts = Object.keys(hostMap).length;
    const totalLogSources = retirementRecords.length;
    const completionTime = new Date().toLocaleString();
    
    summary.innerHTML = `
        <h5>Retirement Summary</h5>
        <div class="summary-stats">
            <div class="stat-item">
                <span class="stat-number">${totalHosts}</span>
                <span class="stat-label">Hosts Retired</span>
            </div>
            <div class="stat-item">
                <span class="stat-number">${totalLogSources}</span>
                <span class="stat-label">Log Sources Retired</span>
            </div>
            <div class="stat-item">
                <span class="stat-number">${completionTime}</span>
                <span class="stat-label">Completed</span>
            </div>
        </div>
        <div class="host-breakdown">
            <h6>Host Breakdown:</h6>
            <ul>
                ${Object.entries(hostMap).map(([hostName, records]) => 
                    `<li><strong>${hostName}</strong>: ${records.length} log sources</li>`
                ).join('')}
            </ul>
        </div>
    `;
}

function exportReport() {
    if (!currentJobId) {
        showToast('No retirement records to export', 'warning');
        return;
    }
    
    // Create download link
    const link = document.createElement('a');
    link.href = `/api/export/pdf/${currentJobId}`;
    link.download = `LRCleaner_Report_${currentJobId}.txt`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    
    showToast('Report downloaded successfully', 'success');
}

// Host Selection Functions
