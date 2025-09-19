// Scan control functionality
function renderScanControlFromData(data) {
    const networks = data.networks || [];
    const scanState = data.scanState || {};
    const isScanning = scanState.is_running && !scanState.is_stopping;
    const isStopping = scanState.is_stopping;
    let selectedNetwork = scanState.selected_network;
    
    // Auto-select the first detected network if none is selected and we have detected networks
    if (!selectedNetwork && window.detectedNetworks && window.detectedNetworks.length > 0) {
        const detectedCidr = window.detectedNetworks[0].cidr;
        const matchingNetwork = networks.find(n => n.cidr === detectedCidr);
        if (matchingNetwork) {
            selectedNetwork = matchingNetwork.id;
            // Automatically select this network on the server side
            selectNetwork(matchingNetwork.id);
        }
    }
    
    // Find selected network info
    let selectedNetworkName = 'Target Network';
    let selectedNetworkCidr = '';
    
    if (selectedNetwork) {
        let network;
        // Handle both cases: selectedNetwork as object or as ID
    if (typeof selectedNetwork === 'object' && selectedNetwork.id) {
            network = selectedNetwork;
        } else {
            network = networks.find(n => n.id === selectedNetwork);
        }
        
        if (network) {
            selectedNetworkName = network.name;
            selectedNetworkCidr = network.cidr;
        }
    }
    
    return `
        <div id="scan-control-content">
            ${isStopping ? `
                <!-- Stopping State -->
                <div class="mb-2">
                    <div class="text-sm text-yellow-400 mb-2">${selectedNetworkName}${selectedNetworkCidr ? ` (${selectedNetworkCidr})` : ''}</div>
                    <div class="mb-3">
                        <select id="network-selector" disabled class="w-full px-2 py-1.5 text-xs rounded focus:outline-none focus:ring-1 focus:ring-green-500 opacity-50 cursor-not-allowed" style="background: var(--bg-tertiary); color: var(--text-primary); border: 1px solid var(--border-color);">
                            <option value="">${!selectedNetwork ? 'Select Network to Start' : 'Target Network'}</option>
                            ${networks.map(network => `
                                <option value="${network.id}" ${selectedNetwork === network.id ? 'selected' : ''}>
                                    ${network.name} (${network.cidr})
                                </option>
                            `).join('')}
                        </select>
                    </div>
                    <div class="mb-4">
                        <div class="w-full px-3 py-2 bg-yellow-600 text-white text-xs rounded text-center">
                            Stopping...
                        </div>
                    </div>
                </div>
                <div class="bg-yellow-600 h-2 rounded-full mb-4">
                    <div class="h-full bg-yellow-400 rounded-full animate-pulse"></div>
                </div>
                
                <div class="grid grid-cols-3 gap-4 text-center">
                    <div>
                        <div class="text-green-400 text-sm">Scans</div>
                        <div class="text-white font-bold">${scanState.scan_count || 0}</div>
                    </div>
                    <div>
                        <div class="text-green-400 text-sm">Runtime</div>
                        <div id="scan-runtime" class="text-white font-bold font-mono" data-start-time="${scanState.start_time || ''}">${formatScanTime(scanState.start_time)}</div>
                    </div>
                    <div>
                        <div class="text-green-400 text-sm">Status</div>
                        <div class="text-yellow-400 font-bold">Stopping</div>
                    </div>
                </div>
            ` : isScanning ? `
                <!-- Scanning State -->
                <div class="mb-2">
                    <div class="text-sm text-green-400 mb-2">${selectedNetworkName}${selectedNetworkCidr ? ` (${selectedNetworkCidr})` : ''}</div>
                    <div class="mb-3">
                        <select id="network-selector" disabled class="w-full px-2 py-1.5 text-xs rounded focus:outline-none focus:ring-1 focus:ring-green-500 opacity-50 cursor-not-allowed" style="background: var(--bg-tertiary); color: var(--text-primary); border: 1px solid var(--border-color);">
                            <option value="">${!selectedNetwork ? 'Select Network to Start' : 'Target Network'}</option>
                            ${networks.map(network => `
                                <option value="${network.id}" ${selectedNetwork === network.id ? 'selected' : ''}>
                                    ${network.name} (${network.cidr})
                                </option>
                            `).join('')}
                        </select>
                    </div>
                    <div class="mb-4">
                        <button onclick="stopScan()" class="w-full px-3 py-2 bg-red-600 hover:bg-red-700 text-white text-xs rounded transition-colors">
                            Stop Scan
                        </button>
                    </div>
                </div>
                <div class="bg-green-600 h-2 rounded-full mb-4 overflow-hidden relative">
                    <div class="absolute inset-0 scan-progress-animation"></div>
                </div>
                
                <div class="grid grid-cols-3 gap-4 text-center">
                    <div>
                        <div class="text-green-400 text-sm">Scans</div>
                        <div class="text-white font-bold">${scanState.scan_count || 0}</div>
                    </div>
                    <div>
                        <div class="text-green-400 text-sm">Runtime</div>
                        <div id="scan-runtime" class="text-white font-bold font-mono" data-start-time="${scanState.start_time || ''}">${formatScanTime(scanState.start_time)}</div>
                    </div>
                    <div>
                        <div class="text-green-400 text-sm">Last Scan</div>
                        <div class="text-white font-bold">${scanState.last_scan_time ? 'Recent' : 'Never'}</div>
                    </div>
                </div>
            ` : `
                <!-- Ready to Scan State -->
                <div class="mb-2">
                    <div class="text-sm text-gray-400 mb-2">${selectedNetworkName}${selectedNetworkCidr ? ` (${selectedNetworkCidr})` : ''}</div>
                    <div class="mb-3">
                        <select id="network-selector" class="w-full px-2 py-1.5 text-xs rounded focus:outline-none focus:ring-1 focus:ring-green-500" style="background: var(--bg-tertiary); color: var(--text-primary); border: 1px solid var(--border-color);">
                            <option value="">${!selectedNetwork ? 'Select Network to Start' : 'Target Network'}</option>
                            ${networks.map(network => `
                                <option value="${network.id}" ${selectedNetwork === network.id ? 'selected' : ''}>
                                    ${network.name} (${network.cidr})
                                </option>
                            `).join('')}
                        </select>
                    </div>
                    <div class="mb-4">
                        <button id="start-scan-btn" onclick="startScan()" ${!selectedNetwork ? 'disabled' : ''} class="w-full px-3 py-2 bg-green-600 hover:bg-green-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white text-xs rounded transition-colors">
                            Start Scan
                        </button>
                    </div>
                </div>
                <div class="bg-green-600 h-2 rounded-full mb-4"></div>
                
                <div class="grid grid-cols-3 gap-4 text-center">
                    <div>
                        <div class="text-green-400 text-sm">Scans</div>
                        <div class="text-white font-bold">${scanState.scan_count || scanState.total_scans || 0}</div>
                    </div>
                    <div>
                        <div class="text-green-400 text-sm">Runtime</div>
                        <div class="text-white font-bold">00:00:00</div>
                    </div>
                    <div>
                        <div class="text-green-400 text-sm">Last Scan</div>
                        <div class="text-white font-bold">${scanState.last_scan_time ? 'Recent' : 'Never'}</div>
                    </div>
                </div>
            `}
        </div>
        
        <style>
        @keyframes scan-progress {
            0% { 
                transform: translateX(-100%); 
                opacity: 0.5;
            }
            50% { 
                opacity: 1;
            }
            100% { 
                transform: translateX(100%); 
                opacity: 0.5;
            }
        }
        
        @keyframes pulse-glow {
            0%, 100% { 
                box-shadow: 0 0 5px rgba(34, 197, 94, 0.5);
            }
            50% { 
                box-shadow: 0 0 20px rgba(34, 197, 94, 0.8), 0 0 30px rgba(34, 197, 94, 0.4);
            }
        }
        
        .scan-progress-animation {
            animation: scan-progress 1.5s ease-in-out infinite;
            background: linear-gradient(90deg, 
                transparent 0%, 
                rgba(34, 197, 94, 0.6) 20%, 
                rgba(34, 197, 94, 1) 50%, 
                rgba(34, 197, 94, 0.6) 80%, 
                transparent 100%);
            width: 60%;
            height: 100%;
        }
        
        .bg-green-600.overflow-hidden {
            animation: pulse-glow 2s ease-in-out infinite;
            border-radius: 9999px;
        }
        </style>
    `;
}

function formatScanTime(startTime) {
    if (!startTime) return '00:00:00';
    
    const start = new Date(startTime);
    const now = new Date();
    const diff = Math.floor((now - start) / 1000);
    const hours = Math.floor(diff / 3600);
    const minutes = Math.floor((diff % 3600) / 60);
    const seconds = diff % 60;
    
    return `${hours.toString().padStart(2,'0')}:${minutes.toString().padStart(2,'0')}:${seconds.toString().padStart(2,'0')}`;
}

function loadScanControl(showSpinner = true) {
    const scanControlContainer = document.getElementById('scan-control-container');
    if (!scanControlContainer) {
        return;
    }
    
    
    if (showSpinner) {
        scanControlContainer.innerHTML = `
            <div class="p-4 flex items-center justify-center" style="background: var(--bg-secondary);">
                <div class="animate-spin rounded-full h-8 w-8 border-2 border-primary border-t-transparent">
                    <span class="sr-only">Loading scan control...</span>
                </div>
            </div>
        `;
    }
    
    fetch('/api/scan/control', { credentials: 'include' })
        .then(response => {
            const contentType = response.headers.get('content-type');
            if (contentType && contentType.includes('application/json')) {
                return response.json();
            }
            return response.text();
        })
        .then(data => {
            if (typeof data === 'object') {
                const html = renderScanControlFromData(data);
                scanControlContainer.innerHTML = html;
            } else {
                scanControlContainer.innerHTML = data;
            }
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
    
    // Network selector change handler
    const networkSelector = document.getElementById('network-selector');
    if (networkSelector) {
        networkSelector.addEventListener('change', function() {
            const startBtn = document.getElementById('start-scan-btn');
            if (startBtn) {
                startBtn.disabled = this.value === '';
            }
        });
    }
    
    // Start scan button handler
    const startScanBtn = document.getElementById('start-scan-btn');
    if (startScanBtn) {
        startScanBtn.addEventListener('click', function() {
            startScan();
        });
    } else {
        // Fallback to onclick if button exists but ID doesn't work
        const allButtons = document.querySelectorAll('button');
        allButtons.forEach(btn => {
            if (btn.textContent.includes('Start Scan')) {
                btn.addEventListener('click', function() {
                    startScan();
                });
            }
        });
    }
    
    // Stop scan button handler
    const stopScanBtn = document.querySelector('button[onclick*="stopScan"]');
    if (stopScanBtn) {
        stopScanBtn.addEventListener('click', function(e) {
            e.preventDefault();
            stopScan();
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
        body: `network-id=${networkId}`,
        credentials: 'include'
    })
    .then(response => {
        if (response.ok) {
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
        body: formData,
        credentials: 'include'
    })
    .then(response => {
        const contentType = response.headers.get('content-type');
        if (contentType && contentType.includes('application/json')) {
            return response.json();
        }
        return response.text();
    })
    .then(data => {
        if (typeof data === 'object') {
            // Handle JSON response
            const scanControlContainer = document.getElementById('scan-control-container');
            if (scanControlContainer) {
                scanControlContainer.innerHTML = renderScanControlFromData(data);
                // Set up event listeners after content is loaded
                setTimeout(() => {
                    setupScanControlEventListeners();
                    manageScanRuntime();
                }, 100);
            }
        } else {
            // Handle HTML response
            document.getElementById('scan-control-content').outerHTML = data;
            manageScanRuntime();
        }
        // Refresh network map and devices
        if (typeof window.loadNetworkMap === 'function') {
            window.loadNetworkMap();
        }
        if (typeof window.loadDevices === 'function') {
            window.loadDevices(false);
        }
        
        // Reload scan control to show updated state
        setTimeout(() => {
            loadScanControl(false);
        }, 500);
    })
    .catch(error => {
        console.error('Failed to start scan:', error);
        alert('Failed to start scan: ' + error.message);
    });
}

function stopScan() {
    // Immediately show stopping state while we wait for server response
    const scanControlContainer = document.getElementById('scan-control-container');
    if (scanControlContainer) {
        // Get current scan control data to modify
        fetch('/api/scan/control', { credentials: 'include' })
            .then(response => response.json())
            .then(currentData => {
                // Force stopping state in the data
                const stoppingData = {
                    ...currentData,
                    scanState: {
                        ...currentData.scanState,
                        is_stopping: true,
                        is_running: true // Keep running true so it shows as stopping, not stopped
                    }
                };
                scanControlContainer.innerHTML = renderScanControlFromData(stoppingData);
                setTimeout(() => {
                    setupScanControlEventListeners();
                    manageScanRuntime();
                }, 100);
            })
            .catch(error => {
                console.error('Error getting current scan state:', error);
            });
    }
    
    fetch('/api/scan/stop', {
        method: 'POST',
        credentials: 'include'
    })
    .then(response => {
        if (response.status === 409) {
            // 409 Conflict - no active scan to stop
            alert('No active scan is currently running.');
            // Reload scan control to sync with server state
            setTimeout(() => {
                loadScanControl(false);
            }, 100);
            return null;
        }
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        
        const contentType = response.headers.get('content-type');
        if (contentType && contentType.includes('application/json')) {
            return response.json();
        }
        return response.text();
    })
    .then(data => {
        if (data === null) return; // Handle 409 case
        
        // Start polling to wait for scan to fully stop
        pollForScanStop();
        
        if (typeof data === 'object') {
            // Handle JSON response - update UI to showing stopping state
            const scanControlContainer = document.getElementById('scan-control-container');
            if (scanControlContainer) {
                scanControlContainer.innerHTML = renderScanControlFromData(data);
                // Set up event listeners after content is loaded
                setTimeout(() => {
                    setupScanControlEventListeners();
                    manageScanRuntime();
                }, 100);
            }
        } else {
            // Handle HTML response
            document.getElementById('scan-control-content').outerHTML = data;
            manageScanRuntime();
        }
    })
    .catch(error => {
        console.error('Failed to stop scan:', error);
        alert('Failed to stop scan: ' + error.message);
        // Reload scan control to restore proper state on error
        setTimeout(() => {
            loadScanControl(false);
        }, 100);
    });
}

// Poll the server to wait for the scan to fully stop
function pollForScanStop() {
    const maxPolls = 30; // Maximum 30 seconds of polling
    let pollCount = 0;
    
    const pollInterval = setInterval(() => {
        pollCount++;
        
        fetch('/api/scan/control', { credentials: 'include' })
            .then(response => response.json())
            .then(data => {
                const scanState = data.scanState || {};
                const isStopping = scanState.is_stopping;
                const isRunning = scanState.is_running;
                
                // If scan is no longer stopping and not running, it's fully stopped
                if (!isStopping && !isRunning) {
                    clearInterval(pollInterval);
                    
                    // Update UI to stopped state
                    const scanControlContainer = document.getElementById('scan-control-container');
                    if (scanControlContainer) {
                        scanControlContainer.innerHTML = renderScanControlFromData(data);
                        setTimeout(() => {
                            setupScanControlEventListeners();
                            manageScanRuntime();
                        }, 100);
                    }
                    
                    // Refresh network map and devices
                    if (typeof window.loadNetworkMap === 'function') {
                        window.loadNetworkMap();
                    }
                    if (typeof window.loadDevices === 'function') {
                        window.loadDevices(false);
                    }
                }
                // If we've polled for too long, give up and refresh
                else if (pollCount >= maxPolls) {
                    clearInterval(pollInterval);
                    console.warn('Scan stop polling timed out, refreshing scan control');
                    loadScanControl(false);
                }
            })
            .catch(error => {
                console.error('Error polling scan status:', error);
                clearInterval(pollInterval);
                // Refresh scan control on error
                loadScanControl(false);
            });
    }, 1000); // Poll every second
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
                body: 'network-id=' + encodeURIComponent(event.target.value),
                credentials: 'include'
            }).then(response => {
                if (response.ok) {
                    // Refresh network map using vanilla JS
                    if (typeof window.loadNetworkMap === 'function') {
                        window.loadNetworkMap();
                    }
                    
                    if (typeof window.loadDevices === 'function') {
                        window.loadDevices(false);
                    }
                }
            })
            .catch(error => {
                console.error('Error selecting network via dropdown:', error);
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
        body: `network-id=${networkId}`,
        credentials: 'include'
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
window.renderScanControlFromData = renderScanControlFromData;
window.formatScanTime = formatScanTime;
window.loadScanControl = loadScanControl;
window.setupScanControlEventListeners = setupScanControlEventListeners;
window.selectNetworkOption = selectNetworkOption;
window.selectNetwork = selectNetwork;
window.startScan = startScan;
window.stopScan = stopScan;
window.pollForScanStop = pollForScanStop;
window.manageScanRuntime = manageScanRuntime;
window.updateScanRuntime = updateScanRuntime;