// Scan control functionality

function updateNavbarScanIndicator() {
    // navbar scan indicator removed
}

function renderScanControlFromData(data) {
    const networks = data.networks || [];
    const scanState = data.scanState || {};
    const isScanning = scanState.is_running && !scanState.is_stopping;
    const isStopping = scanState.is_stopping;

    updateNavbarScanIndicator(isScanning, isStopping);

    // Priority order for selecting network:
    // 1. Current network (if scanning)
    // 2. Selected network (if not scanning)
    // 3. Only one network available - auto-select it
    // 4. First detected network (if none selected)
    let selectedNetwork = null;

    if (scanState.current_network) {
        selectedNetwork = scanState.current_network;
    } else if (scanState.selected_network) {
        selectedNetwork = scanState.selected_network;
    } else if (networks.length === 1) {
        selectedNetwork = networks[0].id;
    } else if (window.detectedNetworks && window.detectedNetworks.length > 0) {
        // Auto-select the first detected network if none is selected
        const detectedCidr = window.detectedNetworks[0].cidr;
        const matchingNetwork = networks.find(n => n.cidr === detectedCidr);
        if (matchingNetwork) {
            selectedNetwork = matchingNetwork.id;
        }
    }
    
    // Normalize selectedNetwork to always be an ID string
    if (selectedNetwork && typeof selectedNetwork === 'object' && selectedNetwork.id) {
        selectedNetwork = selectedNetwork.id;
    }

    // Find selected network info
    let selectedNetworkName = 'Target Network';
    let selectedNetworkCidr = '';

    if (selectedNetwork) {
        const network = networks.find(n => n.id === selectedNetwork);
        if (network) {
            selectedNetworkName = network.name;
            selectedNetworkCidr = network.cidr;
        }
    }
    
    const networkOptions = networks.map(n =>
        `<option value="${n.id}" ${selectedNetwork === n.id ? 'selected' : ''}>${n.name} (${n.cidr})</option>`
    ).join('');

    const onchange = !isScanning && !isStopping
        ? 'onchange="scanControlNetworkSelected(this.value)"'
        : '';

    const selectEl = (disabled) => `
        <select id="network-selector" ${disabled ? 'disabled' : ''} ${onchange}
                style="width:100%;padding:5px 8px;font-family:'Roboto Condensed',sans-serif;font-size:11px;
                       color:rgba(16,185,129,${disabled ? '0.35' : '0.65'});
                       background:rgba(7,13,23,0.55);border:1px solid rgba(16,185,129,${disabled ? '0.10' : '0.18'});
                       border-radius:4px;outline:none;margin-bottom:10px;${disabled ? 'cursor:not-allowed;' : ''}">
            <option value="">${!selectedNetwork ? 'Select Network' : 'Target Network'}</option>
            ${networkOptions}
        </select>`;

    const divider = `<div style="border-top:1px solid rgba(16,185,129,0.12);margin:10px 0;"></div>`;

    const statsRow = (col1label, col1val, col2label, col2val, col2id, col2attr, col3label, col3val, col3style) => `
        <div style="display:grid;grid-template-columns:repeat(3,1fr);gap:8px;text-align:center;">
            <div>
                <div class="hud-sublabel">${col1label}</div>
                <div class="hud-value">${col1val}</div>
            </div>
            <div>
                <div class="hud-sublabel">${col2label}</div>
                <div ${col2id ? `id="${col2id}"` : ''} class="hud-value" style="font-size:12px;" ${col2attr || ''}>${col2val}</div>
            </div>
            <div>
                <div class="hud-sublabel">${col3label}</div>
                <div class="hud-value" style="font-size:11px;${col3style || ''}">${col3val}</div>
            </div>
        </div>`;

    // Buttons always wire onclick; visual enabled/disabled state is cosmetic only
    const hudBtn = (label, id, onclick, enabled) => `
        <div ${id ? `id="${id}"` : ''} onclick="${onclick}"
             style="width:100%;padding:7px 12px;font-family:'Orbitron',monospace;font-size:8px;
                    letter-spacing:0.12em;text-transform:uppercase;text-align:center;border-radius:4px;
                    margin-bottom:10px;box-sizing:border-box;
                    color:rgba(16,185,129,${enabled ? '0.70' : '0.28'});
                    background:rgba(16,185,129,${enabled ? '0.07' : '0.02'});
                    border:1px solid rgba(16,185,129,${enabled ? '0.22' : '0.10'});
                    cursor:${enabled ? 'pointer' : 'not-allowed'};"
             ${enabled ? `onmouseover="this.style.background='rgba(16,185,129,0.13)';this.style.borderColor='rgba(16,185,129,0.40)';this.style.color='#10b981';"
                         onmouseout="this.style.background='rgba(16,185,129,0.07)';this.style.borderColor='rgba(16,185,129,0.22)';this.style.color='rgba(16,185,129,0.70)';"` : ''}>
            ${label}
        </div>`;

    return `
        <div id="scan-control-content">
            <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:10px;">
                <div class="hud-label" style="margin-bottom:0;">Scan Control</div>
                ${isScanning ? `<div style="display:flex;align-items:center;gap:5px;">
                    <div style="width:5px;height:5px;border-radius:50%;background:#10b981;flex-shrink:0;"></div>
                    <span style="font-family:'Orbitron',monospace;font-size:7px;letter-spacing:0.1em;color:#10b981;text-transform:uppercase;">Active</span>
                </div>` : ''}
                ${isStopping ? `<span style="font-family:'Orbitron',monospace;font-size:7px;letter-spacing:0.1em;color:rgba(16,185,129,0.40);text-transform:uppercase;">Stopping</span>` : ''}
            </div>
            ${isStopping ? `
                ${selectEl(true)}
                ${hudBtn('Stopping…', '', '', false)}
                ${divider}
                ${statsRow('Scans', scanState.scan_count || 0,
                           'Runtime', formatScanTime(scanState.start_time), 'scan-runtime', `data-start-time="${scanState.start_time || ''}"`,
                           'Status', 'Stopping', '')}
            ` : isScanning ? `
                ${selectEl(true)}
                ${hudBtn('Stop Scan', 'stop-scan-btn', 'stopScan()', true)}
                ${divider}
                ${statsRow('Scans', scanState.scan_count || 0,
                           'Runtime', formatScanTime(scanState.start_time), 'scan-runtime', `data-start-time="${scanState.start_time || ''}"`,
                           'Status', 'Active', 'color:#10b981;')}
            ` : `
                ${selectEl(false)}
                ${hudBtn('Start Scan', 'start-scan-btn', 'startScan()', !!selectedNetwork)}
                ${divider}
                ${statsRow('Scans', scanState.scan_count || scanState.total_scans || 0,
                           'Runtime', '00:00:00', '', '',
                           'Last', scanState.last_scan_time ? 'Recent' : 'Never', '')}
            `}
        </div>
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

function scanControlNetworkSelected(val) {
    var btn = document.getElementById('start-scan-btn');
    if (!btn) return;
    var enabled = val !== '';
    btn.style.cursor = enabled ? 'pointer' : 'not-allowed';
    btn.style.color = 'rgba(16,185,129,' + (enabled ? '0.70' : '0.28') + ')';
    btn.style.background = 'rgba(16,185,129,' + (enabled ? '0.07' : '0.02') + ')';
    btn.style.borderColor = 'rgba(16,185,129,' + (enabled ? '0.22' : '0.10') + ')';
    if (enabled) {
        btn.onmouseover = function() {
            this.style.background = 'rgba(16,185,129,0.13)';
            this.style.borderColor = 'rgba(16,185,129,0.40)';
            this.style.color = '#10b981';
        };
        btn.onmouseout = function() {
            this.style.background = 'rgba(16,185,129,0.07)';
            this.style.borderColor = 'rgba(16,185,129,0.22)';
            this.style.color = 'rgba(16,185,129,0.70)';
        };
    } else {
        btn.onmouseover = null;
        btn.onmouseout = null;
    }
}
window.scanControlNetworkSelected = scanControlNetworkSelected;

// Cache to prevent duplicate concurrent requests
let scanControlLoading = false;

function loadScanControl(showSpinner = true) {
    const scanControlContainer = document.getElementById('scan-control-container');
    if (!scanControlContainer) {
        return;
    }

    // Prevent duplicate concurrent requests
    if (scanControlLoading) {
        console.log('Scan control already loading, skipping duplicate request');
        return;
    }

    scanControlLoading = true;
    
    // Add timeout to prevent hanging requests
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 5000); // 5 second timeout

    fetch('/api/scan/control', {
        credentials: 'include',
        signal: controller.signal
    })
        .then(response => {
            clearTimeout(timeoutId);
            const contentType = response.headers.get('content-type');
            if (contentType && contentType.includes('application/json')) {
                return response.json();
            }
            return response.text();
        })
        .then(data => {
            scanControlLoading = false;
            if (typeof data === 'object') {
                const html = renderScanControlFromData(data);
                scanControlContainer.innerHTML = html;
                // Auto-persist single network selection server-side
                const networks = data.networks || [];
                const scanState = data.scanState || {};
                if (!scanState.selected_network && !scanState.current_network && networks.length === 1) {
                    selectNetwork(networks[0].id);
                }
            } else {
                scanControlContainer.innerHTML = data;
            }
            setTimeout(() => {
                setupScanControlEventListeners();
                manageScanRuntime(); // Initialize runtime management
            }, 100);
        })
        .catch(error => {
            scanControlLoading = false;
            clearTimeout(timeoutId);
            console.error('Error loading scan control:', error);
            const errorMsg = error.name === 'AbortError' ? 'Request timed out' : 'Failed to load scan control';
            scanControlContainer.innerHTML = `<div class="p-4 text-red-500 text-center" style="background: var(--bg-secondary);">${errorMsg}</div>`;
        });
}

function setupScanControlEventListeners() {
    // onchange is wired inline on the select via scanControlNetworkSelected()
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

// Navbar scan indicator polling (works on all pages)
var navbarScanPollInterval = null;

function startNavbarScanPolling() {
    // Update immediately
    refreshNavbarScanIndicator();
    // Poll every 5 seconds
    if (!navbarScanPollInterval) {
        navbarScanPollInterval = setInterval(refreshNavbarScanIndicator, 5000);
    }
}

function refreshNavbarScanIndicator() {
    fetch('/api/scan/status', { credentials: 'include' })
        .then(function(response) { return response.json(); })
        .then(function(scanState) {
            var isScanning = scanState.is_running && !scanState.is_stopping;
            var isStopping = scanState.is_stopping;
            updateNavbarScanIndicator(isScanning, isStopping);
        })
        .catch(function() {});
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
window.startNavbarScanPolling = startNavbarScanPolling;