// Global variables
let currentJobId = null;
let websocket = null;
let hostGroups = {};
let allResults = [];
let hostAnalysis = [];
let selectedHosts = [];
let selectedLogSources = [];
let retirementRecords = [];
let collectionHostAnalysis = [];
let selectedCollectionHosts = [];

// Debug: Check if script is loading
console.log('LRCleaner script loaded - version 2');

// Sidebar functionality
function toggleSidebar() {
    const sidebar = document.getElementById('sidebar');
    const mainContent = document.getElementById('mainContent');
    
    if (sidebar && mainContent) {
        // Check if we're on mobile
        const isMobile = window.innerWidth <= 768;
        
        if (isMobile) {
            // On mobile, toggle open/closed state
            sidebar.classList.toggle('open');
        } else {
            // On desktop, toggle collapsed/expanded state
            sidebar.classList.toggle('collapsed');
        }
        
        // Update toggle button icon
        const toggleIcon = sidebar.querySelector('.sidebar-toggle i');
        if (toggleIcon) {
            if (sidebar.classList.contains('collapsed') || sidebar.classList.contains('open')) {
                toggleIcon.style.transform = 'rotate(180deg)';
            } else {
                toggleIcon.style.transform = 'rotate(0deg)';
            }
        }
    }
}

function navigateToSection(navId) {
    // Remove active class from all nav links
    const navLinks = document.querySelectorAll('.nav-link');
    navLinks.forEach(link => link.classList.remove('active'));
    
    // Add active class to clicked link
    const activeLink = document.getElementById(navId);
    if (activeLink) {
        activeLink.classList.add('active');
    }
    
    // Close sidebar on mobile after navigation
    const isMobile = window.innerWidth <= 768;
    if (isMobile) {
        const sidebar = document.getElementById('sidebar');
        if (sidebar) {
            sidebar.classList.remove('open');
        }
    }
    
    // Handle navigation based on navId
    switch(navId) {
        case 'analyzeNav':
            // Show analysis section (default view)
            showAnalysisSection();
            break;
        case 'exportNav':
            // Handle export functionality
            handleExport();
            break;
        case 'retireNav':
            // Handle retirement functionality
            handleRetirement();
            break;
        case 'rollbackNav':
            // Handle rollback functionality
            handleRollback();
            break;
        case 'settingsNav':
            // Show settings section
            showSettingsSection();
            break;
        default:
            console.log('Unknown navigation:', navId);
    }
}

function showAnalysisSection() {
    // Hide settings section and show analysis section
    const settingsSection = document.getElementById('settingsSection');
    const analysisSection = document.getElementById('analysisSection');
    const controlSection = document.querySelector('.control-section');
    const resultsSection = document.querySelector('.results-section');
    
    if (settingsSection) settingsSection.style.display = 'none';
    if (analysisSection) analysisSection.style.display = 'block';
    if (controlSection) controlSection.style.display = 'block';
    if (resultsSection) resultsSection.style.display = 'block';
    
    console.log('Showing analysis section');
}

function showSettingsSection() {
    // Hide analysis section and show settings section
    const settingsSection = document.getElementById('settingsSection');
    const analysisSection = document.getElementById('analysisSection');
    const rollbackSection = document.getElementById('rollbackSection');
    const controlSection = document.querySelector('.control-section');
    const resultsSection = document.querySelector('.results-section');
    
    if (analysisSection) analysisSection.style.display = 'none';
    if (rollbackSection) rollbackSection.style.display = 'none';
    if (controlSection) controlSection.style.display = 'none';
    if (resultsSection) resultsSection.style.display = 'none';
    if (settingsSection) settingsSection.style.display = 'block';
    
    console.log('Showing settings section');
}

function showRollbackSection() {
    // Hide other sections and show rollback section
    const settingsSection = document.getElementById('settingsSection');
    const analysisSection = document.getElementById('analysisSection');
    const rollbackSection = document.getElementById('rollbackSection');
    const controlSection = document.querySelector('.control-section');
    const resultsSection = document.querySelector('.results-section');
    
    if (analysisSection) analysisSection.style.display = 'none';
    if (settingsSection) settingsSection.style.display = 'none';
    if (controlSection) controlSection.style.display = 'none';
    if (resultsSection) resultsSection.style.display = 'none';
    if (rollbackSection) rollbackSection.style.display = 'block';
    
    console.log('Showing rollback section');
}

function handleExport() {
    // TODO: Implement export functionality
    console.log('Export functionality not yet implemented');
    alert('Export functionality will be implemented soon!');
}

function handleRetirement() {
    // TODO: Implement retirement functionality
    console.log('Retirement functionality not yet implemented');
    alert('Retirement functionality will be implemented soon!');
}

function handleRollback() {
    // Show rollback section
    showRollbackSection();
    // Load rollback history
    loadRollbackHistory();
}

function handleBackupAcknowledge() {
    const backupStatus = document.getElementById('backupStatus');
    if (backupStatus) {
        backupStatus.innerHTML = '<i class="fas fa-check-circle"></i> Database backup acknowledged. You may proceed with retirement operations.';
        backupStatus.className = 'status-message success';
    }
    
    // Store acknowledgment in localStorage
    localStorage.setItem('backupAcknowledged', 'true');
    
    console.log('Database backup acknowledged');
}

function handleBackupSkip() {
    const backupStatus = document.getElementById('backupStatus');
    if (backupStatus) {
        backupStatus.innerHTML = '<i class="fas fa-exclamation-triangle"></i> Database backup skipped. Proceeding without backup is not recommended.';
        backupStatus.className = 'status-message error';
    }
    
    // Store skip in localStorage
    localStorage.setItem('backupSkipped', 'true');
    
    console.log('Database backup skipped');
}

// Initialize the application
document.addEventListener('DOMContentLoaded', function() {
    initializeApp();
});

function initializeApp() {
    // Set default date to today
    const today = new Date().toISOString().split('T')[0];
    const testDateElement = document.getElementById('testDate');
    if (testDateElement) {
        testDateElement.value = today;
    }
    
    // Set default date to 90 days ago for main date picker
    const ninetyDaysAgo = new Date();
    ninetyDaysAgo.setDate(ninetyDaysAgo.getDate() - 90);
    const mainDateElement = document.getElementById('mainDate');
    if (mainDateElement) {
        mainDateElement.value = ninetyDaysAgo.toISOString().split('T')[0];
    }
    
    // Show analysis section by default
    showAnalysisSection();
    
    // Load current configuration
    loadConfiguration();
    
    // Setup event listeners
    setupEventListeners();
    
    // Connect to WebSocket
    console.log('About to connect to WebSocket...');
    connectWebSocket();
}

function setupEventListeners() {
    // Sidebar toggle
    const sidebarToggle = document.getElementById('sidebarToggle');
    if (sidebarToggle) {
        sidebarToggle.addEventListener('click', toggleSidebar);
    }
    
    // Navigation links
    const navLinks = document.querySelectorAll('.nav-link');
    navLinks.forEach(link => {
        link.addEventListener('click', function(e) {
            e.preventDefault();
            const navId = this.id;
            navigateToSection(navId);
        });
    });
    
    // Settings button (if it exists)
    const settingsBtn = document.getElementById('settingsBtn');
    if (settingsBtn) {
        settingsBtn.addEventListener('click', openSettingsModal);
    }
    
    // Backup buttons
    const backupAcknowledgeBtn = document.getElementById('backupAcknowledgeBtn');
    if (backupAcknowledgeBtn) {
        backupAcknowledgeBtn.addEventListener('click', handleBackupAcknowledge);
    }
    
    const backupSkipBtn = document.getElementById('backupSkipBtn');
    if (backupSkipBtn) {
        backupSkipBtn.addEventListener('click', handleBackupSkip);
    }
    
    // Settings section doesn't have a close button - it's shown/hidden via navigation
    
    // Specific close button for test modal
    const testModal = document.getElementById('testModal');
    if (testModal) {
        const closeBtn = testModal.querySelector('.close');
        if (closeBtn) {
            console.log('Setting up close button for test modal');
            closeBtn.addEventListener('click', function(e) {
                console.log('Test modal close button clicked!');
                e.preventDefault();
                e.stopPropagation();
                closeAllModals();
            });
        } else {
            console.log('Close button not found in test modal');
        }
    } else {
        console.log('Test modal not found');
    }
    
    // Apply modal was removed - no longer needed
    
    // Configuration form
    document.getElementById('configForm').addEventListener('submit', handleConfigSubmit);
    
    // Test connection button
    document.getElementById('testConnectionBtn').addEventListener('click', handleTestConnection);
    
    // Control buttons
    const testModeBtn = document.getElementById('testModeBtn');
    if (testModeBtn) testModeBtn.addEventListener('click', openTestModal);
    
    const applyModeBtn = document.getElementById('applyModeBtn');
    if (applyModeBtn) applyModeBtn.addEventListener('click', startApplyMode);
    
    const applyBtn = document.getElementById('applyBtn');
    if (applyBtn) applyBtn.addEventListener('click', handleApply);
    
    const exportBtn = document.getElementById('exportBtn');
    if (exportBtn) exportBtn.addEventListener('click', handleExport);
    
    const clearBtn = document.getElementById('clearBtn');
    if (clearBtn) clearBtn.addEventListener('click', clearResults);
    
    // Test modal
    const startTestBtn = document.getElementById('startTestBtn');
    if (startTestBtn) startTestBtn.addEventListener('click', startTestMode);
    
    // Apply modal
    const performBackupBtn = document.getElementById('performBackupBtn');
    if (performBackupBtn) performBackupBtn.addEventListener('click', openBackupModal);
    
    const skipBackupBtn = document.getElementById('skipBackupBtn');
    if (skipBackupBtn) skipBackupBtn.addEventListener('click', openApplyConfigModal);
    
    // Backup modal
    const executeBackupBtn = document.getElementById('executeBackupBtn');
    if (executeBackupBtn) executeBackupBtn.addEventListener('click', executeBackup);
    
    const cancelBackupBtn = document.getElementById('cancelBackupBtn');
    if (cancelBackupBtn) cancelBackupBtn.addEventListener('click', closeAllModals);
    
    // Apply config modal
    const startApplyBtn = document.getElementById('startApplyBtn');
    if (startApplyBtn) startApplyBtn.addEventListener('click', startApplyMode);
    
    const backToBackupBtn = document.getElementById('backToBackupBtn');
    if (backToBackupBtn) backToBackupBtn.addEventListener('click', backToBackupModal);
    
    // Host selection modal
    const selectVisibleBtn = document.getElementById('selectVisibleBtn');
    if (selectVisibleBtn) selectVisibleBtn.addEventListener('click', selectVisibleHosts);
    
    const selectAllHostsBtn = document.getElementById('selectAllHostsBtn');
    if (selectAllHostsBtn) selectAllHostsBtn.addEventListener('click', selectAllHosts);
    
    const deselectAllHostsBtn = document.getElementById('deselectAllHostsBtn');
    if (deselectAllHostsBtn) deselectAllHostsBtn.addEventListener('click', deselectAllHosts);
    
    const executeRetirementBtn = document.getElementById('executeRetirementBtn');
    if (executeRetirementBtn) executeRetirementBtn.addEventListener('click', executeRetirement);
    
    const cancelRetirementBtn = document.getElementById('cancelRetirementBtn');
    if (cancelRetirementBtn) cancelRetirementBtn.addEventListener('click', cancelRetirement);
    
    // Collection host modal
    const selectAllCollectionHosts = document.getElementById('selectAllCollectionHosts');
    if (selectAllCollectionHosts) selectAllCollectionHosts.addEventListener('change', handleSelectAllRecommendedCollectionHosts);
    
    const selectAllCollectionHostsBtn = document.getElementById('selectAllCollectionHostsBtn');
    if (selectAllCollectionHostsBtn) selectAllCollectionHostsBtn.addEventListener('click', selectAllCollectionHosts);
    
    const deselectAllCollectionHostsBtn = document.getElementById('deselectAllCollectionHostsBtn');
    if (deselectAllCollectionHostsBtn) deselectAllCollectionHostsBtn.addEventListener('click', deselectAllCollectionHosts);
    
    const executeCollectionHostRetirementBtn = document.getElementById('executeCollectionHostRetirementBtn');
    if (executeCollectionHostRetirementBtn) executeCollectionHostRetirementBtn.addEventListener('click', executeCollectionHostRetirement);
    
    const cancelCollectionHostRetirementBtn = document.getElementById('cancelCollectionHostRetirementBtn');
    if (cancelCollectionHostRetirementBtn) cancelCollectionHostRetirementBtn.addEventListener('click', closeAllModals);
    
    // Host selection modal filters
    const hostSearchInput = document.getElementById('hostSearchInput');
    if (hostSearchInput) hostSearchInput.addEventListener('input', filterHostResults);
    
    const hostPingFilter = document.getElementById('hostPingFilter');
    if (hostPingFilter) hostPingFilter.addEventListener('change', filterHostResults);
    
    const hostLogSourceTypeFilter = document.getElementById('hostLogSourceTypeFilter');
    if (hostLogSourceTypeFilter) hostLogSourceTypeFilter.addEventListener('change', filterHostResults);
    
    const hostLogSourceNameFilter = document.getElementById('hostLogSourceNameFilter');
    if (hostLogSourceNameFilter) hostLogSourceNameFilter.addEventListener('change', filterHostResults);
    
    const clearHostFiltersBtn = document.getElementById('clearHostFiltersBtn');
    if (clearHostFiltersBtn) clearHostFiltersBtn.addEventListener('click', clearHostFilters);
    
    // Congratulations modal
    const exportPDFBtn = document.getElementById('exportPDFBtn');
    if (exportPDFBtn) exportPDFBtn.addEventListener('click', exportPDFReport);
    
    const undoChangesBtn = document.getElementById('undoChangesBtn');
    if (undoChangesBtn) undoChangesBtn.addEventListener('click', undoChanges);
    
    const closeCongratulationsBtn = document.getElementById('closeCongratulationsBtn');
    if (closeCongratulationsBtn) closeCongratulationsBtn.addEventListener('click', closeAllModals);
    
    // Modal close handlers - using event delegation for better reliability
    document.addEventListener('click', function(e) {
        if (e.target.classList.contains('close')) {
            console.log('Close button clicked via delegation:', e.target);
            e.preventDefault();
            e.stopPropagation();
            
            // Find which modal this close button belongs to
            const modal = e.target.closest('.modal');
            if (modal) {
                console.log('Closing modal via delegation:', modal.id);
                modal.style.display = 'none';
            } else {
                console.log('Modal not found, using closeAllModals');
                closeAllModals();
            }
        }
    });
    
    // Also try the original approach as backup
    document.querySelectorAll('.close').forEach(closeBtn => {
        closeBtn.addEventListener('click', function(e) {
            console.log('Close button clicked via direct listener:', e.target);
            e.preventDefault();
            e.stopPropagation();
            closeAllModals();
        });
    });
    
    // Modal click outside to close
    document.querySelectorAll('.modal').forEach(modal => {
        modal.addEventListener('click', function(e) {
            if (e.target === this) {
                console.log('Clicked outside modal:', modal.id);
                if (modal.id === 'settingsSection') {
                    console.log('Closing settings section via click outside');
                    modal.style.display = 'none';
                } else {
                    closeAllModals();
                }
            }
        });
    });
    
    // Escape key to close modals - using multiple approaches for reliability
    document.addEventListener('keydown', function(e) {
        if (e.key === 'Escape' || e.keyCode === 27) {
            console.log('Escape key pressed - checking for open modals');
            e.preventDefault();
            e.stopPropagation();
            
            // Check if settings section is open
            const settingsSection = document.getElementById('settingsSection');
            if (settingsSection && settingsSection.style.display === 'block') {
                console.log('Settings section is open - closing it');
                settingsSection.style.display = 'none';
                console.log('Settings section closed via escape key');
            } else {
                console.log('No settings section open - using closeAllModals');
                closeAllModals();
            }
        }
    });
    
    // Additional escape key handler for better compatibility
    window.addEventListener('keydown', function(e) {
        if (e.key === 'Escape' || e.keyCode === 27) {
            console.log('Window escape key pressed');
            e.preventDefault();
            e.stopPropagation();
            
            const settingsSection = document.getElementById('settingsSection');
            if (settingsSection && settingsSection.style.display === 'block') {
                console.log('Closing settings section via window escape handler');
                settingsSection.style.display = 'none';
            }
        }
    });
    
    // Search and filter
    const searchInput = document.getElementById('searchInput');
    if (searchInput) searchInput.addEventListener('input', filterResults);
    
    const pingFilter = document.getElementById('pingFilter');
    if (pingFilter) pingFilter.addEventListener('change', filterResults);
    
    const logSourceTypeFilter = document.getElementById('logSourceTypeFilter');
    if (logSourceTypeFilter) logSourceTypeFilter.addEventListener('change', filterResults);
    
    const logSourceNameFilter = document.getElementById('logSourceNameFilter');
    if (logSourceNameFilter) logSourceNameFilter.addEventListener('change', filterResults);
    
    const clearFiltersBtn = document.getElementById('clearFiltersBtn');
    if (clearFiltersBtn) clearFiltersBtn.addEventListener('click', clearFilters);
    
    // Rollback controls
    const refreshRollbackBtn = document.getElementById('refreshRollbackBtn');
    if (refreshRollbackBtn) refreshRollbackBtn.addEventListener('click', loadRollbackHistory);
    
    const cleanupRollbackBtn = document.getElementById('cleanupRollbackBtn');
    if (cleanupRollbackBtn) cleanupRollbackBtn.addEventListener('click', cleanupRollbackHistory);
    
    // Rollback configuration form
    const rollbackConfigForm = document.getElementById('rollbackConfigForm');
    if (rollbackConfigForm) rollbackConfigForm.addEventListener('submit', handleRollbackConfigSubmit);
}

function loadConfiguration() {
    console.log('loadConfiguration called - fetching /api/config');
    fetch('/api/config')
        .then(response => {
            console.log('loadConfiguration response status:', response.status);
            if (!response.ok) {
                console.log('HTTP error status:', response.status);
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            return response.json();
        })
        .then(config => {
            if (!config) {
                console.log('No configuration data - showing settings modal');
                showSettingsSection();
                return;
            }
            
            document.getElementById('hostname').value = config.hostname || '';
            document.getElementById('port').value = config.port || 8501;
            document.getElementById('apiKey').value = config.apiKey || '';
            
            // Check if configuration is valid (has real values, not just defaults)
            const isValid = config.hostname && 
                           config.hostname !== 'localhost' && 
                           config.apiKey && 
                           config.apiKey !== '';
            console.log('Configuration loaded:', { 
                hostname: config.hostname, 
                hasApiKey: !!config.apiKey, 
                isValid,
                isDefaultHostname: config.hostname === 'localhost'
            });
            
            if (isValid) {
                // Show main content and hide settings section
                console.log('Showing main content - valid config found');
                showMainContent();
                const applyModeBtn = document.getElementById('applyModeBtn');
                if (applyModeBtn) applyModeBtn.disabled = false;
            } else {
                // Show settings section if no valid config (including default values)
                console.log('Showing settings section - no valid config (default or empty values)');
                showSettingsSection();
                const applyModeBtn = document.getElementById('applyModeBtn');
                if (applyModeBtn) applyModeBtn.disabled = true;
            }
        })
        .catch(error => {
            console.error('Error loading configuration:', error);
            showToast('Error loading configuration', 'error');
            // Show settings modal on error
            showSettingsSection();
        });
}

let isSubmittingConfig = false;

function handleConfigSubmit(e) {
    e.preventDefault();
    e.stopPropagation();
    
    // Prevent double submission
    if (isSubmittingConfig) {
        console.log('Configuration submission already in progress, ignoring duplicate');
        return;
    }
    
    isSubmittingConfig = true;
    
    const formData = new FormData(e.target);
    const config = {
        hostname: formData.get('hostname'),
        apiKey: formData.get('apiKey'),
        port: parseInt(formData.get('port'))
    };
    
    // Validate configuration
    if (!config.hostname) {
        showToast('Hostname is required', 'error');
        isSubmittingConfig = false;
        return;
    }
    
    if (!config.apiKey || config.apiKey.length < 10) {
        showToast('API key must be at least 10 characters', 'error');
        isSubmittingConfig = false;
        return;
    }
    
    // Save configuration
    console.log('Sending configuration save request:', config);
    fetch('/api/config', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(config)
    })
    .then(response => {
        console.log('Configuration save response status:', response.status);
        console.log('Configuration save response headers:', response.headers);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        // Check if response has content
        const contentType = response.headers.get('content-type');
        console.log('Response content-type:', contentType);
        
        if (contentType && contentType.includes('application/json')) {
            return response.json();
        } else {
            // If not JSON, just return success
            console.log('Response is not JSON, treating as success');
            return { status: 'success' };
        }
    })
    .then(data => {
        console.log('Configuration save successful:', data);
        showToast('Configuration saved successfully!', 'success');
        const applyModeBtn = document.getElementById('applyModeBtn');
        if (applyModeBtn) applyModeBtn.disabled = false;
        showMainContent();
        isSubmittingConfig = false;
    })
    .catch(error => {
        console.error('Error saving configuration:', error);
        showToast('Error saving configuration', 'error');
        isSubmittingConfig = false;
    });
}

function handleTestConnection() {
    const formData = new FormData(document.getElementById('configForm'));
    const config = {
        hostname: formData.get('hostname'),
        apiKey: formData.get('apiKey'),
        port: parseInt(formData.get('port'))
    };
    
    // Validate configuration
    if (!config.hostname) {
        showToast('Please enter a hostname first', 'error');
        return;
    }
    
    if (!config.apiKey) {
        showToast('Please enter an API key first', 'error');
        return;
    }
    
    // Show loading state
    const testBtn = document.getElementById('testConnectionBtn');
    const originalText = testBtn.innerHTML;
    testBtn.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Testing...';
    testBtn.disabled = true;
    
    // Test connection
    fetch('/api/test-connection', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(config)
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            showToast(data.message, 'success');
            // Update status message in the modal
            const statusDiv = document.getElementById('configStatus');
            statusDiv.innerHTML = `<div class="status-success"><i class="fas fa-check-circle"></i> ${data.message}</div>`;
        } else {
            showToast(data.error || 'Connection test failed', 'error');
            // Update status message in the modal
            const statusDiv = document.getElementById('configStatus');
            statusDiv.innerHTML = `<div class="status-error"><i class="fas fa-exclamation-circle"></i> ${data.error || 'Connection test failed'}</div>`;
        }
    })
    .catch(error => {
        console.error('Error testing connection:', error);
        showToast('Error testing connection', 'error');
        // Update status message in the modal
        const statusDiv = document.getElementById('configStatus');
        statusDiv.innerHTML = `<div class="status-error"><i class="fas fa-exclamation-circle"></i> Error testing connection</div>`;
    })
    .finally(() => {
        // Restore button state
        testBtn.innerHTML = originalText;
        testBtn.disabled = false;
    });
}

function showMainContent() {
    console.log('showMainContent called');
    document.getElementById('mainContent').style.display = 'block';
    const settingsSection = document.getElementById('settingsSection');
    if (settingsSection) {
        settingsSection.style.display = 'none';
    }
    console.log('Main content should now be visible');
}

function hideMainContent() {
    document.getElementById('mainContent').style.display = 'none';
}

function showSettingsSection() {
    const settingsSection = document.getElementById('settingsSection');
    if (settingsSection) {
        settingsSection.style.display = 'block';
        // Hide other sections
        const analysisSection = document.getElementById('analysisSection');
        const controlSection = document.querySelector('.control-section');
        const resultsSection = document.querySelector('.results-section');
        
        if (analysisSection) analysisSection.style.display = 'none';
        if (controlSection) controlSection.style.display = 'none';
        if (resultsSection) resultsSection.style.display = 'none';
    } else {
        console.log('Settings section not found');
    }
}


function openTestModal() {
    document.getElementById('testModal').style.display = 'block';
}

function openApplyModal() {
    document.getElementById('applyModal').style.display = 'block';
}

function closeAllModals() {
    console.log('closeAllModals called');
    const modals = document.querySelectorAll('.modal');
    console.log('Found modals to close:', modals.length);
    
    modals.forEach(modal => {
        console.log('Closing modal:', modal.id, 'Current display:', modal.style.display);
        modal.style.display = 'none';
        console.log('Modal closed:', modal.id);
    });
    
    // Ensure main content is visible when closing modals (unless we're in initial setup)
    const hostname = document.getElementById('hostname').value;
    const apiKey = document.getElementById('apiKey').value;
    console.log('Config check - hostname:', hostname, 'apiKey length:', apiKey ? apiKey.length : 0);
    
    if (hostname && apiKey && apiKey !== '') {
        console.log('Showing main content');
        showMainContent();
    } else {
        console.log('Not showing main content - no valid config');
    }
}

function startTestMode() {
    console.log('startTestMode called');
    const date = document.getElementById('testDate').value;
    console.log('Selected date:', date);
    
    if (!date) {
        showToast('Please select a date', 'error');
        return;
    }
    
    closeAllModals();
    hideHostSelectionControls();
    showLoadingOverlay();
    console.log('Starting test mode request...');
    
    fetch('/api/test', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ date: date })
    })
    .then(response => {
        console.log('Response received:', response.status, response.statusText);
        return response.json();
    })
    .then(data => {
        console.log('Response data:', data);
        currentJobId = data.jobId;
        console.log('Current job ID set to:', currentJobId);
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
    const date = document.getElementById('mainDate').value;
    if (!date) {
        showToast('Please select a date', 'error');
        return;
    }
    
    closeAllModals();
    hideHostSelectionControls();
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

function handleApply() {
    // TODO: Implement apply functionality
    console.log('Apply functionality not yet implemented');
    showToast('Apply functionality will be implemented soon!', 'info');
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

function exportPDFReport() {
    if (!currentJobId) {
        showToast('No report to export', 'error');
        return;
    }
    
    const url = `/api/export/pdf/${currentJobId}`;
    window.open(url, '_blank');
    showToast('PDF report exported', 'success');
}

function undoChanges() {
    if (!currentJobId) {
        showToast('No changes to undo', 'error');
        return;
    }
    
    // This would typically make an API call to undo the retirement changes
    showToast('Undo functionality not yet implemented', 'info');
}

// Collection Host Management Functions
function showCollectionHostModal() {
    if (collectionHostAnalysis.length === 0) {
        showToast('No collection hosts to display', 'info');
        return;
    }
    
    populateCollectionHostList();
    document.getElementById('collectionHostModal').style.display = 'block';
}

function populateCollectionHostList() {
    const list = document.getElementById('collectionHostList');
    list.innerHTML = '';
    
    collectionHostAnalysis.forEach(host => {
        const hostItem = document.createElement('div');
        hostItem.className = 'host-item';
        hostItem.setAttribute('data-collection-host-id', host.systemMonitorId);
        
        const isRecommended = host.recommended;
        const isSelected = selectedCollectionHosts.includes(host.systemMonitorId);
        
        hostItem.innerHTML = `
            <div class="host-info">
                <label class="checkbox-label">
                    <input type="checkbox" ${isSelected ? 'checked' : ''} 
                           onchange="updateCollectionHostSelection('${host.systemMonitorId}', this.checked)">
                    <span class="checkmark"></span>
                </label>
                <div class="host-details">
                    <div class="host-name">${host.systemMonitorName}</div>
                    <div class="host-meta">
                        <span class="ping-status ping-${(host.pingResult || 'unknown').toLowerCase()}">${host.pingResult || 'Unknown'}</span>
                        <span class="log-source-count">${host.logSourceCount} log sources</span>
                        ${isRecommended ? '<span class="recommended-badge">Recommended</span>' : ''}
                    </div>
                </div>
            </div>
        `;
        
        if (isSelected) {
            hostItem.classList.add('selected');
        }
        
        list.appendChild(hostItem);
    });
    
    updateCollectionHostSummary();
}

function updateCollectionHostSelection(collectionHostId, selected) {
    if (selected) {
        if (!selectedCollectionHosts.includes(collectionHostId)) {
            selectedCollectionHosts.push(collectionHostId);
        }
    } else {
        selectedCollectionHosts = selectedCollectionHosts.filter(id => id !== collectionHostId);
    }
    
    // Update visual state
    const hostItem = document.querySelector(`[data-collection-host-id="${collectionHostId}"]`);
    if (hostItem) {
        hostItem.classList.toggle('selected', selected);
    }
    
    // Update execute button
    document.getElementById('executeCollectionHostRetirementBtn').disabled = selectedCollectionHosts.length === 0;
    updateCollectionHostSummary();
}

function updateCollectionHostSummary() {
    const summary = document.getElementById('collectionHostSummary');
    const totalHosts = collectionHostAnalysis.length;
    const recommendedHosts = collectionHostAnalysis.filter(h => h.recommended).length;
    const totalLogSources = selectedCollectionHosts.reduce((total, hostId) => {
        const host = collectionHostAnalysis.find(h => h.systemMonitorId === hostId);
        return total + (host ? host.logSourceCount : 0);
    }, 0);
    
    summary.innerHTML = `
        <strong>Summary:</strong> ${totalHosts} total collection hosts | 
        ${recommendedHosts} recommended | 
        ${selectedCollectionHosts.length} selected | 
        ${totalLogSources} log sources will be retired
    `;
}

function handleSelectAllRecommendedCollectionHosts() {
    const selectAll = document.getElementById('selectAllCollectionHosts').checked;
    const recommendedHosts = collectionHostAnalysis.filter(h => h.recommended);
    
    recommendedHosts.forEach(host => {
        updateCollectionHostSelection(host.systemMonitorId, selectAll);
    });
}

function selectAllCollectionHosts() {
    collectionHostAnalysis.forEach(host => {
        updateCollectionHostSelection(host.systemMonitorId, true);
    });
}

function deselectAllCollectionHosts() {
    selectedCollectionHosts = [];
    collectionHostAnalysis.forEach(host => {
        updateCollectionHostSelection(host.systemMonitorId, false);
    });
}

function executeCollectionHostRetirement() {
    if (selectedCollectionHosts.length === 0) {
        showToast('Please select at least one collection host', 'warning');
        return;
    }
    
    // Confirm action
    const totalLogSources = selectedCollectionHosts.reduce((total, hostId) => {
        const host = collectionHostAnalysis.find(h => h.systemMonitorId === hostId);
        return total + (host ? host.logSourceCount : 0);
    }, 0);
    
    if (!confirm(`Are you sure you want to retire ${selectedCollectionHosts.length} collection hosts with ${totalLogSources} log sources? This action cannot be undone.`)) {
        return;
    }
    
    closeAllModals();
    showLoadingOverlay();
    
    fetch('/api/collection-hosts/retire', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            selectedCollectionHosts: selectedCollectionHosts
        })
    })
    .then(response => response.json())
    .then(data => {
        hideLoadingOverlay();
        showToast(data.message || 'Collection host retirement initiated', 'success');
    })
    .catch(error => {
        hideLoadingOverlay();
        console.error('Error retiring collection hosts:', error);
        showToast('Error retiring collection hosts', 'error');
    });
}

// Apply Mode Host Management Functions
function toggleApplyHostDetails(hostId) {
    const detailsRow = document.getElementById(hostId);
    const icon = document.getElementById(`apply-icon-${hostId}`);
    
    if (detailsRow.style.display === 'none') {
        detailsRow.style.display = 'block';
        icon.textContent = '▼';
    } else {
        detailsRow.style.display = 'none';
        icon.textContent = '▶';
    }
}

function populateHostFilterDropdowns(uniqueLogSourceTypes, uniqueLogSourceNames) {
    // Populate log source type filter
    const logSourceTypeFilter = document.getElementById('hostLogSourceTypeFilter');
    if (logSourceTypeFilter) {
        // Clear existing options except the first one
        logSourceTypeFilter.innerHTML = '<option value="">All Log Source Types</option>';
        Array.from(uniqueLogSourceTypes).sort().forEach(type => {
            const option = document.createElement('option');
            option.value = type;
            option.textContent = type;
            logSourceTypeFilter.appendChild(option);
        });
    }
    
    // Populate log source name filter
    const logSourceNameFilter = document.getElementById('hostLogSourceNameFilter');
    if (logSourceNameFilter) {
        // Clear existing options except the first one
        logSourceNameFilter.innerHTML = '<option value="">All Log Source Names</option>';
        Array.from(uniqueLogSourceNames).sort().forEach(name => {
            const option = document.createElement('option');
            option.value = name;
            option.textContent = name;
            logSourceNameFilter.appendChild(option);
        });
    }
}

function filterHostResults() {
    const searchTerm = document.getElementById('hostSearchInput').value.toLowerCase();
    const pingFilter = document.getElementById('hostPingFilter').value;
    const logSourceTypeFilter = document.getElementById('hostLogSourceTypeFilter').value;
    const logSourceNameFilter = document.getElementById('hostLogSourceNameFilter').value;
    
    const hostItems = document.querySelectorAll('#hostList .host-item');
    
    hostItems.forEach(item => {
        // Handle host summary rows (expandable rows)
        if (item.classList.contains('host-summary-row')) {
            const hostName = item.querySelector('.host-name')?.textContent.toLowerCase() || '';
            const pingStatus = item.querySelector('.ping-status')?.textContent || '';
            const logSourceCount = item.querySelector('.log-source-count')?.textContent.toLowerCase() || '';
            
            // Get the host data for this row
            const hostId = item.getAttribute('data-host-id');
            const host = hostAnalysis.find(h => h.hostId === hostId);
            
            const matchesSearch = hostName.includes(searchTerm) || logSourceCount.includes(searchTerm);
            const matchesPing = !pingFilter || pingStatus === pingFilter;
            
            // Check if any log source in this host matches the type/name filters
            let matchesLogSourceType = !logSourceTypeFilter;
            let matchesLogSourceName = !logSourceNameFilter;
            
            if (host && host.logSources) {
                if (logSourceTypeFilter) {
                    matchesLogSourceType = host.logSources.some(source => source.logSourceType === logSourceTypeFilter);
                }
                if (logSourceNameFilter) {
                    matchesLogSourceName = host.logSources.some(source => source.name === logSourceNameFilter);
                }
            }
            
            const shouldShow = matchesSearch && matchesPing && matchesLogSourceType && matchesLogSourceName;
            item.style.display = shouldShow ? '' : 'none';
            
            // Also hide/show the corresponding details row
            const detailsRow = item.nextElementSibling;
            if (detailsRow && detailsRow.classList.contains('host-details-row')) {
                detailsRow.style.display = shouldShow ? 'none' : 'none'; // Always hide details when filtering
            }
        }
        // Handle host details rows (expandable content)
        else if (item.classList.contains('host-details-row')) {
            // Details rows are controlled by their parent summary row
            // They should be hidden when filtering is active
            item.style.display = 'none';
        }
    });
}

function clearHostFilters() {
    document.getElementById('hostSearchInput').value = '';
    document.getElementById('hostPingFilter').value = '';
    document.getElementById('hostLogSourceTypeFilter').value = '';
    document.getElementById('hostLogSourceNameFilter').value = '';
    filterHostResults();
}

function toggleHostDetails(hostId) {
    const detailsRow = document.getElementById(hostId);
    const icon = document.getElementById(`icon-${hostId}`);
    
    if (detailsRow.style.display === 'none') {
        detailsRow.style.display = 'table-row';
        icon.textContent = '▼';
    } else {
        detailsRow.style.display = 'none';
        icon.textContent = '▶';
    }
}

function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/ws`;
    console.log('Connecting to WebSocket:', wsUrl);
    
    websocket = new WebSocket(wsUrl);
    
    websocket.onopen = function() {
        console.log('WebSocket connected successfully');
        console.log('WebSocket ready state:', websocket.readyState);
    };
    
    websocket.onmessage = function(event) {
        console.log('WebSocket message received:', event.data);
        const job = JSON.parse(event.data);
        console.log('Parsed job data:', job);
        updateJobProgress(job);
    };
    
    websocket.onclose = function() {
        console.log('WebSocket disconnected');
        // Reconnect after 5 seconds
        setTimeout(connectWebSocket, 5000);
    };
    
    websocket.onerror = function(error) {
        console.error('WebSocket error:', error);
        console.error('WebSocket error details:', {
            type: error.type,
            target: error.target,
            readyState: websocket.readyState,
            url: websocket.url
        });
    };
}

function updateJobProgress(job) {
    console.log('updateJobProgress called with job:', job);
    console.log('Current job ID:', currentJobId);
    console.log('Job ID from message:', job.id);
    if (job.id === currentJobId) {
        console.log('Job ID matches current job ID:', currentJobId);
        // Update progress bar
        const progressFill = document.getElementById('progressFill');
        const progressText = document.getElementById('progressText');
        const jobStatus = document.getElementById('jobStatus');
        
        progressFill.style.width = `${job.progress}%`;
        progressText.textContent = job.message;
        
        if (job.results) {
            console.log('Job has results:', job.results.length, 'items');
            allResults = job.results;
            updateResultsTable();
            updateResultsSummary();
        } else {
            console.log('Job has no results property');
        }
        
        if (job.hostAnalysis) {
            console.log('Job has host analysis:', job.hostAnalysis.length, 'hosts');
            console.log('Job status:', job.status);
            hostAnalysis = job.hostAnalysis;
            if (job.status === 'completed') {
                console.log('Job is completed, showing host selection controls in results');
                showHostSelectionControls();
                updateResultsTableForApplyMode();
            } else {
                console.log('Job not completed yet, status:', job.status);
            }
        }
        
        if (job.collectionHostAnalysis) {
            console.log('Job has collection host analysis:', job.collectionHostAnalysis.length, 'collection hosts');
            collectionHostAnalysis = job.collectionHostAnalysis;
            // Show collection host modal if there are recommended collection hosts
            const recommendedCollectionHosts = collectionHostAnalysis.filter(h => h.recommended);
            if (recommendedCollectionHosts.length > 0) {
                showCollectionHostModal();
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
    console.log('updateResultsTable called with', allResults.length, 'results');
    const tbody = document.getElementById('resultsBody');
    if (!tbody) {
        console.error('resultsBody element not found!');
        return;
    }
    tbody.innerHTML = '';
    
    if (allResults.length === 0) {
        console.log('No results to display');
        tbody.innerHTML = '<tr><td colspan="5" class="no-results">No results yet. Run Test Mode to analyze log sources.</td></tr>';
        return;
    }
    
    // Group results by host and collect unique filter values
    hostGroups = {};
    const uniqueLogSourceTypes = new Set();
    const uniqueLogSourceNames = new Set();
    
    allResults.forEach(result => {
        if (!hostGroups[result.hostName]) {
            hostGroups[result.hostName] = {
                hostName: result.hostName,
                pingResult: result.pingResult,
                hostId: result.hostId,
                logSources: []
            };
        }
        hostGroups[result.hostName].logSources.push(result);
        
        // Collect unique values for filters
        if (result.logSourceType) {
            uniqueLogSourceTypes.add(result.logSourceType);
        }
        if (result.name) {
            uniqueLogSourceNames.add(result.name);
        }
    });
    
    // Populate filter dropdowns
    populateFilterDropdowns(uniqueLogSourceTypes, uniqueLogSourceNames);
    
    // Create expandable rows for each host
    Object.values(hostGroups).forEach((hostGroup, index) => {
        const hostId = `host-${index}`;
        const isExpanded = false;
        
        // Create host summary row
        const summaryRow = document.createElement('tr');
        summaryRow.className = 'host-summary-row';
        summaryRow.innerHTML = `
            <td colspan="5" class="host-summary-cell">
                <div class="host-summary-content" onclick="toggleHostDetails('${hostId}')">
                    <span class="expand-icon" id="icon-${hostId}">▶</span>
                    <span class="host-name">${hostGroup.hostName}</span>
                    <span class="ping-status ping-${(hostGroup.pingResult || 'unknown').toLowerCase()}">${hostGroup.pingResult || 'Unknown'}</span>
                    <span class="log-source-count">${hostGroup.logSources.length} log source${hostGroup.logSources.length !== 1 ? 's' : ''}</span>
                </div>
            </td>
        `;
        tbody.appendChild(summaryRow);
        
        // Create expandable details container
        const detailsRow = document.createElement('tr');
        detailsRow.id = hostId;
        detailsRow.className = 'host-details-row';
        detailsRow.style.display = 'none';
        
        const detailsCell = document.createElement('td');
        detailsCell.colSpan = 5;
        detailsCell.className = 'host-details-cell';
        
        const detailsTable = document.createElement('table');
        detailsTable.className = 'log-sources-table';
        detailsTable.innerHTML = `
            <thead>
                <tr>
                    <th>Log Source ID</th>
                    <th>Log Source Name</th>
                    <th>Log Source Type</th>
                    <th>Last Log Message</th>
                    <th>Ping Result</th>
                </tr>
            </thead>
            <tbody>
                ${hostGroup.logSources.map(source => `
                    <tr>
                        <td class="log-source-id">${source.id}</td>
                        <td class="log-source-name">${source.name || 'N/A'}</td>
                        <td class="log-source-type">${source.logSourceType?.name || source.logSourceType || 'N/A'}</td>
                        <td class="last-log-date">${formatDate(source.maxLogDate)}</td>
                        <td class="ping-result ping-${(hostGroup.pingResult || 'unknown').toLowerCase()}">${hostGroup.pingResult || 'Unknown'}</td>
                    </tr>
                `).join('')}
            </tbody>
        `;
        
        detailsCell.appendChild(detailsTable);
        detailsRow.appendChild(detailsCell);
        tbody.appendChild(detailsRow);
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
    const logSourceTypeFilter = document.getElementById('logSourceTypeFilter').value;
    const logSourceNameFilter = document.getElementById('logSourceNameFilter').value;
    
    const rows = document.querySelectorAll('#resultsBody tr');
    
    rows.forEach(row => {
        // Skip "no results" row
        if (row.querySelector('.no-results')) {
            return;
        }
        
        // Handle host summary rows (expandable rows)
        if (row.classList.contains('host-summary-row')) {
            const hostName = row.querySelector('.host-name')?.textContent.toLowerCase() || '';
            const pingStatus = row.querySelector('.ping-status')?.textContent || '';
            const logSourceCount = row.querySelector('.log-source-count')?.textContent.toLowerCase() || '';
            
            // Get the host group data for this row
            const hostId = row.querySelector('.host-summary-content')?.onclick?.toString().match(/toggleHostDetails\('(host-\d+)'\)/)?.[1];
            const hostGroup = getHostGroupByHostId(hostId);
            
            const matchesSearch = hostName.includes(searchTerm) || logSourceCount.includes(searchTerm);
            const matchesPing = !pingFilter || pingStatus === pingFilter;
            
            // Check if any log source in this host matches the type/name filters
            let matchesLogSourceType = !logSourceTypeFilter;
            let matchesLogSourceName = !logSourceNameFilter;
            
            if (hostGroup && hostGroup.logSources) {
                if (logSourceTypeFilter) {
                    matchesLogSourceType = hostGroup.logSources.some(source => source.logSourceType === logSourceTypeFilter);
                }
                if (logSourceNameFilter) {
                    matchesLogSourceName = hostGroup.logSources.some(source => source.name === logSourceNameFilter);
                }
            }
            
            const shouldShow = matchesSearch && matchesPing && matchesLogSourceType && matchesLogSourceName;
            row.style.display = shouldShow ? '' : 'none';
            
            // Also hide/show the corresponding details row
            const detailsRow = row.nextElementSibling;
            if (detailsRow && detailsRow.classList.contains('host-details-row')) {
                detailsRow.style.display = shouldShow ? 'none' : 'none'; // Always hide details when filtering
            }
        }
        // Handle host details rows (expandable content)
        else if (row.classList.contains('host-details-row')) {
            // Details rows are controlled by their parent summary row
            // They should be hidden when filtering is active
            row.style.display = 'none';
        }
    });
}

function populateFilterDropdowns(uniqueLogSourceTypes, uniqueLogSourceNames) {
    // Populate log source type filter
    const logSourceTypeFilter = document.getElementById('logSourceTypeFilter');
    logSourceTypeFilter.innerHTML = '<option value="">All Log Source Types</option>';
    
    Array.from(uniqueLogSourceTypes).sort().forEach(type => {
        const option = document.createElement('option');
        option.value = type;
        option.textContent = type;
        logSourceTypeFilter.appendChild(option);
    });
    
    // Populate log source name filter
    const logSourceNameFilter = document.getElementById('logSourceNameFilter');
    logSourceNameFilter.innerHTML = '<option value="">All Log Source Names</option>';
    
    Array.from(uniqueLogSourceNames).sort().forEach(name => {
        const option = document.createElement('option');
        option.value = name;
        option.textContent = name;
        logSourceNameFilter.appendChild(option);
    });
}

function getHostGroupByHostId(hostId) {
    if (!hostId) return null;
    
    // Find the host group by matching the hostId pattern
    const hostIndex = parseInt(hostId.replace('host-', ''));
    const hostNames = Object.keys(hostGroups);
    if (hostIndex >= 0 && hostIndex < hostNames.length) {
        const hostName = hostNames[hostIndex];
        return hostGroups[hostName];
    }
    return null;
}

function clearFilters() {
    document.getElementById('searchInput').value = '';
    document.getElementById('pingFilter').value = '';
    document.getElementById('logSourceTypeFilter').value = '';
    document.getElementById('logSourceNameFilter').value = '';
    filterResults();
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

// Host Selection Functions
function showHostSelectionControls() {
    // Show the host selection controls in the results section
    const controls = document.getElementById('hostSelectionControls');
    controls.style.display = 'block';
    
    // Wait for the DOM to update before trying to access the element
    setTimeout(() => {
        const summary = document.getElementById('hostSummaryResults');
        if (summary) {
            updateHostSummary();
        } else {
            console.error('hostSummaryResults element not found after showing controls');
        }
    }, 50);
}

function hideHostSelectionControls() {
    // Hide the host selection controls in the results section
    document.getElementById('hostSelectionControls').style.display = 'none';
    selectedHosts = []; // Clear selected hosts
}

function cancelRetirement() {
    // Cancel retirement and hide host selection controls
    hideHostSelectionControls();
    showToast('Retirement cancelled', 'info');
}

function updateResultsTableForApplyMode() {
    // Update the results table to show hosts with checkboxes for selection
    console.log('updateResultsTableForApplyMode called with', hostAnalysis.length, 'hosts');
    const tbody = document.getElementById('resultsBody');
    if (!tbody) {
        console.error('resultsBody element not found!');
        return;
    }
    tbody.innerHTML = '';
    
    if (hostAnalysis.length === 0) {
        console.log('No hosts to display');
        tbody.innerHTML = '<tr><td colspan="6" class="no-results">No hosts found for retirement.</td></tr>';
        return;
    }
    
    // Group hosts and collect unique filter values
    const uniqueLogSourceTypes = new Set();
    const uniqueLogSourceNames = new Set();
    
    hostAnalysis.forEach(host => {
        // Collect unique values for filters
        host.logSources.forEach(logSource => {
            if (logSource.logSourceType) {
                uniqueLogSourceTypes.add(logSource.logSourceType);
            }
            if (logSource.name) {
                uniqueLogSourceNames.add(logSource.name);
            }
        });
    });
    
    // Populate filter dropdowns
    populateFilterDropdowns(uniqueLogSourceTypes, uniqueLogSourceNames);
    
    // Create expandable rows for each host
    hostAnalysis.forEach((host, index) => {
        const hostId = `apply-host-${index}`;
        const isRecommended = host.recommended;
        const isSelected = selectedHosts.includes(host.hostId);
        
        // Create host summary row
        const summaryRow = document.createElement('tr');
        summaryRow.className = 'host-summary-row';
        summaryRow.setAttribute('data-host-id', host.hostId);
        summaryRow.innerHTML = `
            <td class="host-checkbox-cell">
                <label class="checkbox-label">
                    <input type="checkbox" ${isSelected ? 'checked' : ''} 
                           onchange="updateHostSelection('${host.hostId}', this.checked)">
                    <span class="checkmark"></span>
                </label>
            </td>
            <td colspan="4" class="host-summary-cell">
                <div class="host-summary-content" onclick="toggleHostDetails('${hostId}')">
                    <span class="expand-icon" id="icon-${hostId}">▶</span>
                    <span class="host-name">${host.hostName}</span>
                    <span class="ping-status ping-${(host.pingResult || 'unknown').toLowerCase()}">${host.pingResult || 'Unknown'}</span>
                    <span class="log-source-count">${host.logSourceCount} log source${host.logSourceCount !== 1 ? 's' : ''}</span>
                    <span class="max-log-date">Last log: ${formatDate(host.maxLogDate)}</span>
                    ${isRecommended ? '<span class="recommended-badge">Recommended</span>' : ''}
                </div>
            </td>
        `;
        tbody.appendChild(summaryRow);
        
        // Create expandable details container
        const detailsRow = document.createElement('tr');
        detailsRow.id = hostId;
        detailsRow.className = 'host-details-row';
        detailsRow.style.display = 'none';
        
        const detailsCell = document.createElement('td');
        detailsCell.colSpan = 6;
        detailsCell.className = 'host-details-cell';
        
        const detailsTable = document.createElement('table');
        detailsTable.className = 'log-sources-table';
        detailsTable.innerHTML = `
            <thead>
                <tr>
                    <th style="width: 40px;">Select</th>
                    <th>Log Source ID</th>
                    <th>Log Source Name</th>
                    <th>Log Source Type</th>
                    <th>Last Log Message</th>
                    <th>Ping Result</th>
                </tr>
            </thead>
            <tbody>
                ${host.logSources.map(source => `
                    <tr>
                        <td class="log-source-checkbox-cell">
                            <label class="checkbox-label">
                                <input type="checkbox" 
                                       onchange="updateLogSourceSelection('${source.id}', this.checked)">
                                <span class="checkmark"></span>
                            </label>
                        </td>
                        <td class="log-source-id">${source.id}</td>
                        <td class="log-source-name">${source.name || 'N/A'}</td>
                        <td class="log-source-type">${source.logSourceType?.name || source.logSourceType || 'N/A'}</td>
                        <td class="last-log-date">${source.maxLogDate === '1899-12-31T17:00:00Z' || source.maxLogDate === '1899-12-31T17:00:00.000Z' ? 
                            '<span style="color: #ff6b6b; font-weight: bold;">NEVER RECEIVED LOGS</span>' : 
                            formatDate(source.maxLogDate)}</td>
                        <td class="ping-result ping-${(host.pingResult || 'unknown').toLowerCase()}">${host.pingResult || 'Unknown'}</td>
                    </tr>
                `).join('')}
            </tbody>
        `;
        
        detailsCell.appendChild(detailsTable);
        detailsRow.appendChild(detailsCell);
        tbody.appendChild(detailsRow);
    });
}

function populateHostList() {
    const hostList = document.getElementById('hostList');
    hostList.innerHTML = '';
    
    // Group hosts and collect unique filter values
    const uniqueLogSourceTypes = new Set();
    const uniqueLogSourceNames = new Set();
    
    hostAnalysis.forEach(host => {
        // Collect unique values for filters
        host.logSources.forEach(logSource => {
            if (logSource.logSourceType) {
                uniqueLogSourceTypes.add(logSource.logSourceType);
            }
            if (logSource.name) {
                uniqueLogSourceNames.add(logSource.name);
            }
        });
    });
    
    // Populate filter dropdowns
    populateHostFilterDropdowns(uniqueLogSourceTypes, uniqueLogSourceNames);
    
    // Create expandable rows for each host
    hostAnalysis.forEach((host, index) => {
        const hostId = `apply-host-${index}`;
        const isRecommended = host.recommended;
        const isSelected = selectedHosts.includes(host.hostId);
        
        // Create host summary row
        const summaryRow = document.createElement('div');
        summaryRow.className = 'host-item host-summary-row';
        summaryRow.setAttribute('data-host-id', host.hostId);
        summaryRow.innerHTML = `
            <div class="host-info">
                <label class="checkbox-label">
                    <input type="checkbox" ${isSelected ? 'checked' : ''} 
                           onchange="updateHostSelection('${host.hostId}', this.checked)">
                    <span class="checkmark"></span>
                </label>
                <div class="host-details" onclick="toggleApplyHostDetails('${hostId}')">
                    <div class="host-summary-content">
                        <span class="expand-icon" id="apply-icon-${hostId}">▶</span>
                        <span class="host-name">${host.hostName}</span>
                        <span class="ping-status ping-${(host.pingResult || 'unknown').toLowerCase()}">${host.pingResult || 'Unknown'}</span>
                        <span class="log-source-count">${host.logSourceCount} log source${host.logSourceCount !== 1 ? 's' : ''}</span>
                        <span class="max-log-date">Last log: ${formatDate(host.maxLogDate)}</span>
                        ${isRecommended ? '<span class="recommended-badge">Recommended</span>' : ''}
                </div>
                </div>
            </div>
        `;
        
        if (isSelected) {
            summaryRow.classList.add('selected');
        }
        
        hostList.appendChild(summaryRow);
        
        // Create expandable details container
        const detailsRow = document.createElement('div');
        detailsRow.id = hostId;
        detailsRow.className = 'host-details-row';
        detailsRow.style.display = 'none';
        
        const detailsContent = document.createElement('div');
        detailsContent.className = 'host-details-content';
        
        const detailsTable = document.createElement('table');
        detailsTable.className = 'log-sources-table';
        detailsTable.innerHTML = `
            <thead>
                <tr>
                    <th>Log Source ID</th>
                    <th>Log Source Name</th>
                    <th>Log Source Type</th>
                    <th>Last Log Message</th>
                    <th>Ping Result</th>
                </tr>
            </thead>
            <tbody>
                ${host.logSources.map(source => `
                    <tr>
                        <td class="log-source-id">${source.id}</td>
                        <td class="log-source-name">${source.name || 'N/A'}</td>
                        <td class="log-source-type">${source.logSourceType?.name || source.logSourceType || 'N/A'}</td>
                        <td class="last-log-date">${formatDate(source.maxLogDate)}</td>
                        <td class="ping-result ping-${(host.pingResult || 'unknown').toLowerCase()}">${host.pingResult || 'Unknown'}</td>
                    </tr>
                `).join('')}
            </tbody>
        `;
        
        detailsContent.appendChild(detailsTable);
        detailsRow.appendChild(detailsContent);
        hostList.appendChild(detailsRow);
    });
    
    updateHostSummary();
}

function updateHostSelection(hostId, selected) {
    console.log('updateHostSelection called with:', hostId, selected);
    console.log('Current selectedHosts:', selectedHosts);
    
    if (selected) {
        if (!selectedHosts.includes(hostId)) {
            selectedHosts.push(hostId);
        }
    } else {
        selectedHosts = selectedHosts.filter(id => id !== hostId);
    }
    
    console.log('Updated selectedHosts:', selectedHosts);
    
    // Update visual state
    const hostItem = document.querySelector(`[data-host-id="${hostId}"]`);
    if (hostItem) {
        hostItem.classList.toggle('selected', selected);
    }
    
    // Update execute button - always enabled
    const executeBtn = document.getElementById('executeRetirementBtn');
    if (executeBtn) {
        executeBtn.disabled = false; // Always enabled
        console.log('Execute button always enabled, selected hosts count:', selectedHosts.length);
    } else {
        console.error('Execute button element not found!');
    }
    
    // Update host summary
    updateHostSummary();
}

function updateLogSourceSelection(logSourceId, selected) {
    console.log('updateLogSourceSelection called with:', logSourceId, selected);
    
    if (selected) {
        if (!selectedLogSources.includes(logSourceId)) {
            selectedLogSources.push(logSourceId);
        }
    } else {
        selectedLogSources = selectedLogSources.filter(id => id !== logSourceId);
    }
    
    console.log('Updated selectedLogSources:', selectedLogSources);
    
    // Update execute button state
    const executeBtn = document.getElementById('executeRetirementBtn');
    if (executeBtn) {
        executeBtn.disabled = (selectedHosts.length === 0 && selectedLogSources.length === 0);
    }
    
    // Update host summary
    updateHostSummary();
}

function updateHostSummary() {
    const summary = document.getElementById('hostSummaryResults');
    
    // Debug logging
    console.log('updateHostSummary called');
    console.log('hostAnalysis length:', hostAnalysis.length);
    console.log('selectedHosts length:', selectedHosts.length);
    console.log('hostAnalysis data:', hostAnalysis);
    console.log('Summary element found:', !!summary);
    
    if (!summary) {
        console.error('hostSummaryResults element not found - element may not be in DOM yet');
        return;
    }
    
    if (!hostAnalysis || hostAnalysis.length === 0) {
        summary.innerHTML = '<strong>Summary:</strong> No host analysis data available';
        return;
    }
    
    const totalHosts = hostAnalysis.length;
    const recommendedHosts = hostAnalysis.filter(h => h.recommended).length;
    const totalLogSources = selectedHosts.reduce((total, hostId) => {
        const host = hostAnalysis.find(h => String(h.hostId) === String(hostId));
        return total + (host ? host.logSourceCount : 0);
    }, 0);
    
    summary.innerHTML = `
        <strong>Summary:</strong> ${totalHosts} total hosts | 
        ${recommendedHosts} recommended | 
        ${selectedHosts.length} hosts selected | 
        ${selectedLogSources.length} log sources selected | 
        ${totalLogSources + selectedLogSources.length} total items will be retired
    `;
}

function selectVisibleHosts() {
    console.log('selectVisibleHosts called');
    // Get all currently visible host rows (not hidden by filters)
    const visibleHostRows = document.querySelectorAll('.host-summary-row:not([style*="display: none"])');
    console.log('Found visible host rows:', visibleHostRows.length);
    
    visibleHostRows.forEach(row => {
        const hostId = row.getAttribute('data-host-id');
        console.log('Processing host with ID:', hostId);
        if (hostId) {
            // Select the host
            const checkbox = row.querySelector('input[type="checkbox"]');
            if (checkbox && !checkbox.checked) {
                console.log('Selecting host:', hostId);
                checkbox.checked = true;
                updateHostSelection(hostId, true);
            }
            
            // Also select all log sources under this host
            const host = hostAnalysis.find(h => String(h.hostId) === String(hostId));
            if (host && host.logSources) {
                console.log('Found host with log sources:', host.logSources.length);
                host.logSources.forEach(logSource => {
                    if (logSource.recommended && !selectedLogSources.includes(String(logSource.id))) {
                        selectedLogSources.push(String(logSource.id));
                        console.log('Added log source to selection:', logSource.id);
                    }
                });
            }
        }
    });
    
    console.log('Final selectedHosts:', selectedHosts);
    console.log('Final selectedLogSources:', selectedLogSources);
    updateHostSummary();
}

function selectAllHosts() {
    console.log('selectAllHosts called');
    hostAnalysis.forEach(host => {
        // Find the row with the host ID, then find the checkbox within it
        const hostRow = document.querySelector(`[data-host-id="${host.hostId}"]`);
        if (hostRow) {
            const checkbox = hostRow.querySelector('input[type="checkbox"]');
            if (checkbox && !checkbox.checked) {
                console.log('Selecting host:', host.hostId);
                checkbox.checked = true;
                updateHostSelection(host.hostId, true);
            }
        } else {
            console.log('Host row not found for ID:', host.hostId);
        }
        
        // Also select all log sources under this host (both recommended and non-recommended)
        if (host.logSources) {
            host.logSources.forEach(logSource => {
                if (!selectedLogSources.includes(String(logSource.id))) {
                    selectedLogSources.push(String(logSource.id));
                    console.log('Added log source to selection:', logSource.id);
                }
                
                // Also check the log source checkbox in the UI
                const logSourceCheckbox = document.querySelector(`input[onchange*="updateLogSourceSelection('${logSource.id}'"]`);
                if (logSourceCheckbox && !logSourceCheckbox.checked) {
                    logSourceCheckbox.checked = true;
                    console.log('Checked log source checkbox:', logSource.id);
                }
            });
        }
    });
    console.log('Final selectedHosts:', selectedHosts);
    console.log('Final selectedLogSources:', selectedLogSources);
    updateHostSummary();
}

function deselectAllHosts() {
    console.log('deselectAllHosts called');
    console.log('Current selectedHosts:', selectedHosts);
    console.log('Current selectedLogSources:', selectedLogSources);
    
    // Deselect all host checkboxes
    selectedHosts.forEach(hostId => {
        const hostRow = document.querySelector(`[data-host-id="${hostId}"]`);
        if (hostRow) {
            const checkbox = hostRow.querySelector('input[type="checkbox"]');
            if (checkbox) {
                checkbox.checked = false;
                console.log('Unchecked host checkbox:', hostId);
            }
        }
    });
    
    // Deselect all log source checkboxes
    const allLogSourceCheckboxes = document.querySelectorAll('input[onchange*="updateLogSourceSelection"]');
    allLogSourceCheckboxes.forEach(checkbox => {
        checkbox.checked = false;
    });
    
    // Clear all selections
    selectedHosts = [];
    selectedLogSources = [];
    
    // Update visual state for all host items
    const hostItems = document.querySelectorAll('[data-host-id]');
    hostItems.forEach(item => {
        item.classList.remove('selected');
    });
    
    // Update execute button state
    const executeBtn = document.getElementById('executeRetirementBtn');
    if (executeBtn) {
        executeBtn.disabled = true;
    }
    
    console.log('After clearing - selectedHosts:', selectedHosts);
    console.log('After clearing - selectedLogSources:', selectedLogSources);
    
    updateHostSummary();
}

function executeRetirement() {
    if (selectedHosts.length === 0 && selectedLogSources.length === 0) {
        showToast('Please select at least one host or log source to retire', 'warning');
        return;
    }
    
    // Confirm action
    const totalLogSourcesFromHosts = selectedHosts.reduce((total, hostId) => {
        const host = hostAnalysis.find(h => String(h.hostId) === String(hostId));
        return total + (host ? host.logSourceCount : 0);
    }, 0);
    
    const totalItems = selectedHosts.length + selectedLogSources.length;
    const totalLogSources = totalLogSourcesFromHosts + selectedLogSources.length;
    
    let confirmMessage = `Are you sure you want to retire ${totalItems} items?`;
    if (selectedHosts.length > 0) {
        confirmMessage += `\n- ${selectedHosts.length} hosts (${totalLogSourcesFromHosts} log sources)`;
    }
    if (selectedLogSources.length > 0) {
        confirmMessage += `\n- ${selectedLogSources.length} individual log sources`;
    }
    confirmMessage += `\n\nThis action cannot be undone.`;
    
    if (!confirm(confirmMessage)) {
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

// Rollback Functions

function loadRollbackHistory() {
    console.log('Loading rollback history...');
    
    fetch('/api/rollback/history')
        .then(response => response.json())
        .then(history => {
            console.log('Rollback history loaded:', history);
            displayRollbackHistory(history);
        })
        .catch(error => {
            console.error('Error loading rollback history:', error);
            showToast('Error loading rollback history', 'error');
        });
}

function displayRollbackHistory(history) {
    const rollbackHistoryDiv = document.getElementById('rollbackHistory');
    
    if (!history || history.length === 0) {
        rollbackHistoryDiv.innerHTML = `
            <div class="no-rollbacks">
                <i class="fas fa-info-circle"></i>
                <p>No rollback points available</p>
                <small>Rollback points are created automatically when retirement operations are performed</small>
            </div>
        `;
        return;
    }
    
    rollbackHistoryDiv.innerHTML = history.map(rollback => `
        <div class="rollback-item" data-rollback-id="${rollback.id}">
            <div class="rollback-header">
                <div class="rollback-info">
                    <h4>${rollback.description}</h4>
                    <div class="rollback-meta">
                        <span class="rollback-date">
                            <i class="fas fa-clock"></i> ${new Date(rollback.timestamp).toLocaleString()}
                        </span>
                        <span class="rollback-user">
                            <i class="fas fa-user"></i> ${rollback.user}
                        </span>
                    </div>
                </div>
                <div class="rollback-stats">
                    <div class="stat-item">
                        <span class="stat-number">${rollback.logSources}</span>
                        <span class="stat-label">Log Sources</span>
                    </div>
                    <div class="stat-item">
                        <span class="stat-number">${rollback.hosts}</span>
                        <span class="stat-label">Hosts</span>
                    </div>
                    <div class="stat-item">
                        <span class="stat-number">${rollback.systemMonitors}</span>
                        <span class="stat-label">System Monitors</span>
                    </div>
                </div>
            </div>
            <div class="rollback-actions">
                <button class="btn btn-primary btn-sm" onclick="previewRollback('${rollback.id}')">
                    <i class="fas fa-eye"></i> Preview
                </button>
                <button class="btn btn-warning btn-sm" onclick="executeRollback('${rollback.id}')">
                    <i class="fas fa-undo"></i> Execute Rollback
                </button>
                <button class="btn btn-danger btn-sm" onclick="deleteRollback('${rollback.id}')">
                    <i class="fas fa-trash"></i> Delete
                </button>
            </div>
        </div>
    `).join('');
}

function previewRollback(rollbackId) {
    console.log('Previewing rollback:', rollbackId);
    
    fetch(`/api/rollback/${rollbackId}`)
        .then(response => response.json())
        .then(rollback => {
            console.log('Rollback details:', rollback);
            showRollbackPreview(rollback);
        })
        .catch(error => {
            console.error('Error loading rollback details:', error);
            showToast('Error loading rollback details', 'error');
        });
}

function showRollbackPreview(rollback) {
    const previewContent = `
        <div class="rollback-preview">
            <h3>Rollback Preview: ${rollback.description}</h3>
            <div class="preview-section">
                <h4>Log Sources (${rollback.logSourceChanges.length})</h4>
                <div class="preview-list">
                    ${rollback.logSourceChanges.slice(0, 5).map(change => `
                        <div class="preview-item">
                            <strong>${change.hostName}</strong> - ${change.originalName}
                            <div class="change-details">
                                <span class="original">${change.originalStatus}</span> → 
                                <span class="current">${change.currentStatus}</span>
                            </div>
                        </div>
                    `).join('')}
                    ${rollback.logSourceChanges.length > 5 ? `<div class="preview-more">... and ${rollback.logSourceChanges.length - 5} more</div>` : ''}
                </div>
            </div>
            <div class="preview-section">
                <h4>Hosts (${rollback.hostChanges.length})</h4>
                <div class="preview-list">
                    ${rollback.hostChanges.slice(0, 5).map(change => `
                        <div class="preview-item">
                            <strong>${change.hostName}</strong>
                            <div class="change-details">
                                <span class="original">${change.originalStatus}</span> → 
                                <span class="current">${change.currentStatus}</span>
                            </div>
                        </div>
                    `).join('')}
                    ${rollback.hostChanges.length > 5 ? `<div class="preview-more">... and ${rollback.hostChanges.length - 5} more</div>` : ''}
                </div>
            </div>
        </div>
    `;
    
    // Create and show modal
    const modal = document.createElement('div');
    modal.className = 'modal';
    modal.innerHTML = `
        <div class="modal-content">
            <div class="modal-header">
                <h3><i class="fas fa-eye"></i> Rollback Preview</h3>
                <span class="close">&times;</span>
            </div>
            <div class="modal-body">
                ${previewContent}
            </div>
            <div class="modal-actions">
                <button class="btn btn-secondary" onclick="closeAllModals()">
                    <i class="fas fa-times"></i> Close
                </button>
            </div>
        </div>
    `;
    
    document.body.appendChild(modal);
    modal.style.display = 'block';
}

function executeRollback(rollbackId) {
    if (!confirm('Are you sure you want to execute this rollback? This will undo the retirement changes and cannot be undone.')) {
        return;
    }
    
    console.log('Executing rollback:', rollbackId);
    showLoadingOverlay();
    
    fetch(`/api/rollback/${rollbackId}/execute`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        }
    })
    .then(response => response.json())
    .then(data => {
        hideLoadingOverlay();
        if (data.status === 'success') {
            showToast('Rollback executed successfully', 'success');
            loadRollbackHistory(); // Refresh the history
        } else {
            showToast('Rollback execution failed', 'error');
        }
    })
    .catch(error => {
        hideLoadingOverlay();
        console.error('Error executing rollback:', error);
        showToast('Error executing rollback', 'error');
    });
}

function deleteRollback(rollbackId) {
    if (!confirm('Are you sure you want to delete this rollback point? This action cannot be undone.')) {
        return;
    }
    
    console.log('Deleting rollback:', rollbackId);
    
    fetch(`/api/rollback/${rollbackId}`, {
        method: 'DELETE'
    })
    .then(response => response.json())
    .then(data => {
        if (data.status === 'success') {
            showToast('Rollback deleted successfully', 'success');
            loadRollbackHistory(); // Refresh the history
        } else {
            showToast('Failed to delete rollback', 'error');
        }
    })
    .catch(error => {
        console.error('Error deleting rollback:', error);
        showToast('Error deleting rollback', 'error');
    });
}

function cleanupRollbackHistory() {
    if (!confirm('Are you sure you want to cleanup old rollback points? This will delete rollback points older than the retention period.')) {
        return;
    }
    
    console.log('Cleaning up rollback history...');
    showLoadingOverlay();
    
    // This would typically call a cleanup API endpoint
    // For now, just refresh the history
    setTimeout(() => {
        hideLoadingOverlay();
        showToast('Rollback cleanup completed', 'success');
        loadRollbackHistory();
    }, 1000);
}

function handleRollbackConfigSubmit(e) {
    e.preventDefault();
    e.stopPropagation();
    
    const formData = new FormData(e.target);
    const rollbackConfig = {
        enabled: formData.get('rollbackEnabled') === 'on',
        retentionDays: parseInt(formData.get('rollbackRetentionDays')),
        maxRollbackPoints: parseInt(formData.get('rollbackMaxPoints')),
        autoBackup: formData.get('rollbackAutoBackup') === 'on',
        backupLocation: formData.get('rollbackBackupLocation')
    };
    
    // Validate configuration
    if (rollbackConfig.retentionDays < 1) {
        showToast('Retention days must be at least 1', 'error');
        return;
    }
    
    if (rollbackConfig.maxRollbackPoints < 1) {
        showToast('Maximum rollback points must be at least 1', 'error');
        return;
    }
    
    // Save configuration
    console.log('Saving rollback configuration:', rollbackConfig);
    fetch('/api/config', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            rollback: rollbackConfig
        })
    })
    .then(response => response.json())
    .then(data => {
        console.log('Rollback configuration saved:', data);
        showToast('Rollback configuration saved successfully!', 'success');
    })
    .catch(error => {
        console.error('Error saving rollback configuration:', error);
        showToast('Error saving rollback configuration', 'error');
    });
}

// Host Selection Functions
