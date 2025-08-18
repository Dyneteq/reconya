// Scan control functionality
function loadScanControl(showSpinner = true) {
    const scanControlContainer = document.getElementById('scan-control-container');
    if (!scanControlContainer) return;
    
    if (showSpinner) {
        scanControlContainer.innerHTML = `
            <div class="p-4 flex items-center justify-center" style="background: var(--bg-secondary);">
                <div class="animate-spin rounded-full h-8 w-8 border-2 border-primary border-t-transparent">
                    <span class="sr-only">Loading scan control...</span>
                </div>
            </div>
        `;
    }
    
    fetch('/api/scan/control')
        .then(response => response.text())
        .then(html => {
            scanControlContainer.innerHTML = html;
            // Set up event listeners after content is loaded
            setTimeout(() => {
                setupScanControlEventListeners();
                manageScanRuntime(); // Initialize runtime management
            }, 100);
        })
        .catch(error => {
            console.error('Error loading scan control:', error);
            scanControlContainer.innerHTML = '<div class="p-4 text-red-500 text-center" style="background: var(--bg-secondary);">Failed to load scan control</div>';
        });
}

function setupScanControlEventListeners() {
    // Network dropdown functionality
    const networkSelect = document.getElementById('networkSelect');
    if (networkSelect) {
        const options = networkSelect.querySelectorAll('.network-option');
        options.forEach(option => {
            option.addEventListener('click', function() {
                selectNetworkOption(this);
            });
        });
    }
}

function selectNetworkOption(element) {
    const networkId = element.getAttribute('data-network-id');
    const networkName = element.textContent.trim();
    
    // Update button text
    const selectButton = document.getElementById('networkSelectButton');
    if (selectButton) {
        selectButton.innerHTML = `
            <span class="truncate">${networkName}</span>
            <i class="bi bi-chevron-down ml-2"></i>
        `;
    }
    
    // Send selection to server
    fetch('/api/scan/select-network', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: `network_id=${networkId}`
    })
    .then(response => {
        if (response.ok) {
            console.log('Network selected successfully');
            // Reload scan control to update state
            setTimeout(() => loadScanControl(false), 100);
        } else {
            console.error('Failed to select network');
        }
    })
    .catch(error => {
        console.error('Error selecting network:', error);
    });
}

// Scan control functions - moved from template to ensure they're always available
function startScan() {
    const networkSelector = document.getElementById('network-selector');
    if (!networkSelector || !networkSelector.value) {
        alert('Please select a network first');
        return;
    }
    
    const formData = new FormData();
    formData.append('network-selector', networkSelector.value);
    
    fetch('/api/scan/start', {
        method: 'POST',
        body: formData
    })
    .then(response => response.text())
    .then(html => {
        document.getElementById('scan-control-content').outerHTML = html;
        manageScanRuntime();
        // Refresh network map and devices
        if (typeof window.loadNetworkMap === 'function') {
            window.loadNetworkMap();
        }
        if (typeof window.loadDevices === 'function') {
            window.loadDevices(false);
        }
    })
    .catch(error => {
        console.error('Failed to start scan:', error);
        alert('Failed to start scan: ' + error.message);
    });
}

function stopScan() {
    fetch('/api/scan/stop', {
        method: 'POST'
    })
    .then(response => response.text())
    .then(html => {
        document.getElementById('scan-control-content').outerHTML = html;
        manageScanRuntime();
        // Refresh network map and devices
        if (typeof window.loadNetworkMap === 'function') {
            window.loadNetworkMap();
        }
        if (typeof window.loadDevices === 'function') {
            window.loadDevices(false);
        }
    })
    .catch(error => {
        console.error('Failed to stop scan:', error);
        alert('Failed to stop scan: ' + error.message);
    });
}

// Runtime update function
function updateScanRuntime() {
    const runtimeEl = document.getElementById('scan-runtime');
    if (runtimeEl) {
        const startTimeStr = runtimeEl.getAttribute('data-start-time');
        if (startTimeStr) {
            const startTime = new Date(startTimeStr);
            const now = new Date();
            const diff = Math.floor((now - startTime) / 1000);
            const hours = Math.floor(diff / 3600);
            const minutes = Math.floor((diff % 3600) / 60);
            const seconds = diff % 60;
            runtimeEl.textContent = 
                `${hours.toString().padStart(2,'0')}:${minutes.toString().padStart(2,'0')}:${seconds.toString().padStart(2,'0')}`;
        } else {
            runtimeEl.textContent = '00:00:00';
        }
    }
}

// Global interval for runtime updates
window.scanRuntimeInterval = window.scanRuntimeInterval || null;

// Start/stop runtime updates based on scan state
function manageScanRuntime() {
    const runtimeEl = document.getElementById('scan-runtime');
    const isScanning = runtimeEl && runtimeEl.getAttribute('data-start-time');
    
    if (isScanning && !window.scanRuntimeInterval) {
        // Start updating runtime
        updateScanRuntime(); // Initial update
        window.scanRuntimeInterval = setInterval(updateScanRuntime, 1000);
    } else if (!isScanning && window.scanRuntimeInterval) {
        // Stop updating runtime
        clearInterval(window.scanRuntimeInterval);
        window.scanRuntimeInterval = null;
    } else if (isScanning) {
        // Just update the runtime if already running
        updateScanRuntime();
    }
}

// Handle network selector changes
function handleNetworkSelectorChange(event) {
    if (event.target.id === 'network-selector') {
        const startBtn = document.getElementById('start-scan-btn');
        if (startBtn) {
            startBtn.disabled = event.target.value === '';
        }
        
        // Update selected network and refresh components
        if (event.target.value !== '') {
            fetch('/api/scan/select-network', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                },
                body: 'network-id=' + encodeURIComponent(event.target.value)
            }).then(() => {
                // Refresh network map and devices only if target elements exist and are proper containers
                const networkMapEl = document.getElementById('network-map');
                const devicesEl = document.getElementById('devices-container');
                
                // Refresh network map using vanilla JS
                if (typeof window.loadNetworkMap === 'function') {
                    window.loadNetworkMap();
                }
                
                if (devicesEl && typeof window.loadDevices === 'function') {
                    window.loadDevices(false);
                }
            });
        }
    }
}

function selectNetwork(networkId) {
    fetch('/api/scan/select-network', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: `network_id=${networkId}`
    })
    .then(response => {
        if (response.ok) {
            loadScanControl(); // Refresh scan control
        }
    })
    .catch(error => {
        console.error('Error selecting network:', error);
    });
}

// Set up event listeners when scan control loads
function setupScanControlEventListeners() {
    // Remove existing listener to avoid duplicates
    document.removeEventListener('change', handleNetworkSelectorChange);
    // Add network selector change listener
    document.addEventListener('change', handleNetworkSelectorChange);
    
    // Initialize runtime management
    manageScanRuntime();
}

// Make functions available globally
window.loadScanControl = loadScanControl;
window.setupScanControlEventListeners = setupScanControlEventListeners;
window.selectNetworkOption = selectNetworkOption;
window.selectNetwork = selectNetwork;
window.startScan = startScan;
window.stopScan = stopScan;
window.manageScanRuntime = manageScanRuntime;
window.updateScanRuntime = updateScanRuntime;