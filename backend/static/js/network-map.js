// Network map functionality
function loadNetworkMap() {
    const networkMapContainer = document.getElementById('network-map');
    if (!networkMapContainer) return;
    
    const loadingDiv = document.getElementById('network-map-loading');
    const contentDiv = document.getElementById('network-map-content');
    const emptyDiv = document.getElementById('network-map-empty');
    
    // Only show loading on first load (when content is empty)
    const isFirstLoad = !contentDiv || contentDiv.innerHTML.trim() === '';
    
    if (isFirstLoad && loadingDiv && contentDiv && emptyDiv) {
        loadingDiv.classList.remove('hidden');
        contentDiv.classList.add('hidden');
        emptyDiv.classList.add('hidden');
    }
    
    fetch('/api/network-map')
        .then(response => response.json())
        .then(data => {
            renderNetworkMap(data);
        })
        .catch(error => {
            console.error('Error loading network map:', error);
            showNetworkMapEmpty();
        });
}

function renderNetworkMap(data) {
    const loadingEl = document.getElementById('network-map-loading');
    const contentEl = document.getElementById('network-map-content');
    const emptyEl = document.getElementById('network-map-empty');
    
    // Only proceed if elements exist (we're on the right page)
    if (!loadingEl || !contentEl || !emptyEl) {
        return;
    }
    
    if (!data.BaseIP || !data.IPRange) {
        showNetworkMapEmpty();
        return;
    }
    
    // Hide loading and empty states
    loadingEl.classList.add('hidden');
    emptyEl.classList.add('hidden');
    contentEl.classList.remove('hidden');
    
    // Build the network map grid
    let html = '<div class="flex flex-wrap gap-1 justify-start">';
    
    data.IPRange.forEach(ipPart => {
        const fullIP = `${data.BaseIP}.${ipPart}`;
        const device = data.Devices[fullIP];
        
        let buttonClass = 'w-5 h-5 rounded-sm transition-all duration-200 hover:scale-110 cursor-pointer ';
        let title = fullIP;
        
        if (device) {
            title += ` - ${device.status}`;
            if (device.status === 'online') {
                buttonClass += 'bg-green-600 border-2 border-green-500';
            } else if (device.status === 'idle') {
                buttonClass += 'bg-green-300 border border-green-400 opacity-70';
            } else if (device.status === 'offline') {
                buttonClass += 'bg-gray-700 border border-gray-600 opacity-25';
            } else {
                buttonClass += 'bg-gray-700 border border-gray-600 opacity-25';
            }
        } else {
            buttonClass += 'bg-gray-700 border border-gray-600 opacity-30';
            title += ' - Available IP';
        }
        
        html += `<button type="button" class="${buttonClass}" title="${title}" data-ip="${fullIP}" data-status="${device ? device.status : 'empty'}" onclick="handleNetworkMapClick('${fullIP}', ${device ? device.id : 'null'})"></button>`;
    });
    
    html += '</div>';
    contentEl.innerHTML = html;
}

function showNetworkMapEmpty() {
    const networkMapContainer = document.getElementById('network-map');
    const loadingDiv = document.getElementById('network-map-loading');
    const contentDiv = document.getElementById('network-map-content');
    const emptyDiv = document.getElementById('network-map-empty');
    
    if (!networkMapContainer) return;
    
    // Use proper HTML structure
    if (loadingDiv && contentDiv && emptyDiv) {
        loadingDiv.classList.add('hidden');
        contentDiv.classList.add('hidden');
        emptyDiv.classList.remove('hidden');
    } else {
        // Fallback
        networkMapContainer.innerHTML = `
            <div class="flex flex-col items-center justify-center h-50 text-gray-400">
                <i class="bi bi-wifi-off text-4xl mb-2"></i>
                <span>No devices detected</span>
                <small class="text-gray-500">Start a scan to populate the network map</small>
            </div>
        `;
    }
}

function getStatusColor(status) {
    switch (status) {
        case 'online': return 'bg-green-500';
        case 'offline': return 'bg-red-500';
        case 'idle': return 'bg-yellow-500';
        default: return 'bg-gray-500';
    }
}

function handleNetworkMapClick(ip, deviceId) {
    if (deviceId && deviceId !== 'null') {
        // Load device modal
        loadDeviceModal(deviceId);
    } else {
        // Load new scan modal
        loadNewScanModal(ip);
    }
}

function loadNewScanModal(ip) {
    const modalContent = document.getElementById('device-modal-content');
    if (modalContent) {
        modalContent.innerHTML = renderNewScanModal({}, ip);
        showModal('deviceModal');
    }
}

function renderNewScanModal(data, ip) {
    return `
        <div class="p-6">
            <!-- Header -->
            <div class="flex justify-between items-center mb-4 pb-3 border-b border-gray-600">
                <h3 class="text-xl font-bold text-green-500">Scan IP Address</h3>
                <button type="button" class="text-gray-400 hover:text-white text-xl" onclick="closeModal('deviceModal')">
                    <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                    </svg>
                </button>
            </div>
            
            <!-- Content -->
            <div class="mb-4">
                <p class="text-gray-300 mb-4">No device found at this IP address. Would you like to scan it?</p>
                <div class="bg-gray-900 rounded p-3 mb-4">
                    <div class="text-green-400 text-lg text-center">${ip}</div>
                </div>
            </div>
            
            <!-- Actions -->
            <div class="flex justify-end gap-3">
                <button type="button" class="px-4 py-2 text-gray-400 border border-gray-600 rounded hover:bg-gray-700 transition-colors" onclick="closeModal('deviceModal')">
                    Cancel
                </button>
                <button type="button" class="px-4 py-2 bg-green-600 hover:bg-green-700 text-white rounded transition-colors" 
                        onclick="scanSingleIP('${ip}'); closeModal('deviceModal')">
                    Scan IP
                </button>
            </div>
        </div>
    `;
}

function scanSingleIP(ip) {
    const formData = new FormData();
    formData.append('ip', ip);
    
    fetch('/api/scan/single', {
        method: 'POST',
        body: formData
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            // Refresh devices and network map
            if (typeof window.loadDevices === 'function') {
                window.loadDevices(false);
            }
            loadNetworkMap();
        } else {
            alert(data.error || 'Failed to scan IP');
        }
    })
    .catch(error => {
        console.error('Failed to scan IP:', error);
        alert('Failed to scan IP');
    });
}

function startNetworkMapUpdates() {
    if (window.networkMapInterval) {
        clearInterval(window.networkMapInterval);
    }
    
    // Update immediately, then every 10 seconds
    loadNetworkMap();
    window.networkMapInterval = setInterval(loadNetworkMap, 10000);
}

function stopNetworkMapUpdates() {
    if (window.networkMapInterval) {
        clearInterval(window.networkMapInterval);
        window.networkMapInterval = null;
    }
}

// Make functions available globally
window.loadNetworkMap = loadNetworkMap;
window.renderNetworkMap = renderNetworkMap;
window.showNetworkMapEmpty = showNetworkMapEmpty;
window.getStatusColor = getStatusColor;
window.handleNetworkMapClick = handleNetworkMapClick;
window.startNetworkMapUpdates = startNetworkMapUpdates;
window.stopNetworkMapUpdates = stopNetworkMapUpdates;
window.loadNewScanModal = loadNewScanModal;
window.renderNewScanModal = renderNewScanModal;
window.scanSingleIP = scanSingleIP;