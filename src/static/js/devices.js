// Device functionality
function loadDevices(showSpinner = true) {
    console.log('loadDevices called with showSpinner:', showSpinner);
    const devicesContainer = document.getElementById('devices-container');
    if (!devicesContainer) {
        console.log('devices-container not found');
        return;
    }
    
    if (showSpinner) {
        devicesContainer.innerHTML = `
            <div class="flex justify-center items-center h-25">
                <div class="animate-spin rounded-full h-8 w-8 border-2 border-primary border-t-transparent">
                    <span class="sr-only">Loading devices...</span>
                </div>
            </div>
        `;
    }
    
    console.log('Making fetch request to /api/devices');
    fetch('/api/devices', { credentials: 'include' })
        .then(response => {
            console.log('Response status:', response.status);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            return response.json();
        })
        .then(data => {
            console.log('Devices data received:', data);
            renderDeviceGrid(data.devices || []);
        })
        .catch(error => {
            console.error('Error loading devices:', error);
            devicesContainer.innerHTML = '<div class="text-red-500 text-center py-4">Failed to load devices</div>';
        });
}

function showNoNetworkSelected() {
    const devicesContainer = document.getElementById('devices-container');
    if (!devicesContainer) return;
    
    devicesContainer.innerHTML = `
        <div class="text-center py-8 text-gray-400">
            <svg class="w-12 h-12 mx-auto mb-3 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"></path>
            </svg>
            <p class="text-sm">Select a network to view devices</p>
        </div>
    `;
}

function renderDeviceGrid(devices) {
    console.log('renderDeviceGrid called with devices:', devices);
    const devicesContainer = document.getElementById('devices-container');
    if (!devicesContainer) {
        console.log('devices-container not found in renderDeviceGrid');
        return;
    }

    // Debug: log first device to see all fields
    if (devices && devices.length > 0) {
        console.log('Sample device data (first device):', JSON.stringify(devices[0], null, 2));
    }

    console.log('Devices length:', devices ? devices.length : 'null/undefined');
    if (!devices || devices.length === 0) {
        console.log('No devices found, showing empty message');
        devicesContainer.innerHTML = `
            <div class="text-center py-8 text-gray-400">
                <i class="ti ti-router text-4xl mb-2 block"></i>
                <p>No devices found</p>
                <p class="text-sm text-gray-500">Start a network scan to discover devices</p>
            </div>
        `;
        return;
    }

    // Filter devices:
    // 1. Must have been seen online at least once
    // 2. If offline, must have been seen within the last hour
    const now = new Date();
    const oneHourAgo = new Date(now.getTime() - 60 * 60 * 1000);

    const onlineDevices = devices.filter(device => {
        // Try different field name variations
        const lastSeenOnline = device.LastSeenOnlineAt || device.last_seen_online_at;

        // Must have been seen online at least once
        if (!lastSeenOnline && device.status !== 'online') {
            return false;
        }

        // If device is offline, check if it was seen within last hour
        if (device.status === 'offline') {
            if (!lastSeenOnline) {
                return false; // No last seen timestamp, hide it
            }
            const lastSeen = new Date(lastSeenOnline);
            if (lastSeen < oneHourAgo) {
                console.log(`Hiding offline device ${device.ipv4}, last seen ${lastSeen.toLocaleString()}`);
                return false; // Offline for more than 1 hour, hide it
            }
        }

        return true;
    });

    console.log('Filtered to', onlineDevices.length, 'devices that have been online (from', devices.length, 'total)');

    if (onlineDevices.length === 0) {
        console.log('No online devices found, showing empty message');
        devicesContainer.innerHTML = `
            <div class="text-center py-8 text-gray-400">
                <i class="ti ti-router text-4xl mb-2 block"></i>
                <p>No devices have been seen online</p>
                <p class="text-sm text-gray-500">Start a network scan to discover devices</p>
            </div>
        `;
        return;
    }

    console.log('Rendering', onlineDevices.length, 'online devices');
    
    let gridHTML = '<div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5 gap-6">';

    onlineDevices.forEach(device => {
        // Count ports
        let openPorts = 0;
        let filteredPorts = 0;
        if (device.ports && Array.isArray(device.ports)) {
            device.ports.forEach(port => {
                if (port.state === 'open') openPorts++;
                else if (port.state === 'filtered') filteredPorts++;
            });
        }
        
        // Check if device is currently being port scanned
        // Try both snake_case and PascalCase field names
        const portScanStarted = device.port_scan_started_at || device.PortScanStartedAt;
        const portScanEnded = device.port_scan_ended_at || device.PortScanEndedAt;
        const isPortScanning = portScanStarted && !portScanEnded;

        // Debug log for first device to see field names
        if (devices.indexOf(device) === 0) {
            console.log('Device fields check:', {
                ip: device.ipv4,
                port_scan_started_at: device.port_scan_started_at,
                PortScanStartedAt: device.PortScanStartedAt,
                port_scan_ended_at: device.port_scan_ended_at,
                PortScanEndedAt: device.PortScanEndedAt,
                isPortScanning: isPortScanning
            });
        }

        gridHTML += `
            <div class="rounded p-4 transition-colors cursor-pointer relative min-h-[150px] flex flex-col"
                 style="background: var(--bg-secondary);"
                 onmouseover="this.style.background='var(--bg-tertiary)'"
                 onmouseout="this.style.background='var(--bg-secondary)'"
                 onclick="loadDeviceModal('${device.id}')">
                ${openPorts > 0 ? `<div class="absolute top-2 right-2 w-2 h-2 bg-red-500 rounded-full" title="${openPorts} open ports"></div>` : ''}
                ${filteredPorts > 0 ? `<div class="absolute top-2 right-${openPorts > 0 ? '5' : '2'} w-2 h-2 bg-yellow-500 rounded-full" title="${filteredPorts} filtered ports"></div>` : ''}

                ${isPortScanning ? `
                    <div class="absolute top-2 left-2">
                        <i class="ti ti-scan text-blue-400 text-lg animate-pulse" title="Port scanning in progress"></i>
                    </div>
                ` : ''}

                <div class="text-white font-semibold text-xl mb-1 break-all">${device.ipv4}</div>
                ${device.mac ? `<div class="text-gray-600 text-xs mb-4">${device.mac}</div>` : '<div class="mb-4"></div>'}
                <div class="flex-1 flex items-end justify-between">
                    <div class="text-gray-600 text-xs truncate opacity-60">${(device.hostname || device.name) ? (device.hostname || device.name) : ''}</div>
                </div>
            </div>
        `;
    });
    
    gridHTML += '</div>';
    devicesContainer.innerHTML = gridHTML;
}

function loadDeviceModal(deviceId) {
    fetch(`/api/devices/${deviceId}/modal`, { credentials: 'include' })
        .then(response => response.json())
        .then(data => {
            const modalContent = document.getElementById('device-modal-content');
            if (modalContent) {
                modalContent.innerHTML = renderDeviceModal(data.device, data.screenshotsEnabled);
                showModal('deviceModal');
            }
        })
        .catch(error => {
            console.error('Error loading device modal:', error);
        });
}

function renderDeviceModal(device, screenshotsEnabled = false) {
    return `
        <div class="p-6">
            <!-- Header -->
            <div class="flex justify-between items-center mb-4 pb-3" style="border-bottom: 1px solid var(--border-color);">
                <div class="flex items-center">
                    <div class="w-4 h-4 rounded-full mr-3 ${getStatusColor(device.status)}"></div>
                    <h3 class="device-ip" style="color: var(--text-primary);">${device.ipv4}</h3>
                    ${device.name || device.hostname ? `<span class="text-lg ml-3" style="color: var(--text-secondary);">- ${device.name || device.hostname}</span>` : ''}
                </div>
                <button type="button" class="text-xl transition-colors" style="color: var(--text-muted);" onmouseover="this.style.color='var(--text-primary)'" onmouseout="this.style.color='var(--text-muted)'" onclick="closeModal('deviceModal')">
                    <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                    </svg>
                </button>
            </div>
            
            <!-- Device Information -->
            <div class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-6">
                <div>
                    <h4 class="text-green-500 font-semibold mb-3">Device Info</h4>
                    <div class="space-y-3 text-sm">
                        <div><span style="color: var(--text-muted);">IP Address:</span> <span class="device-ip" style="color: var(--text-primary); font-size: 1rem;">${device.ipv4}</span></div>
                        ${device.mac ? `<div><span style="color: var(--text-muted);">MAC Address:</span> <span class="text-blue-400">${device.mac}</span></div>` : ''}
                        ${device.hostname ? `<div><span style="color: var(--text-muted);">Hostname:</span> <span style="color: var(--text-primary);">${device.hostname}</span></div>` : ''}
                        <div><span style="color: var(--text-muted);">Status:</span> <span class="px-2 py-1 rounded text-xs ${getStatusBadgeColor(device.status)}">${device.status}</span></div>
                        ${device.LastSeenOnlineAt ? `<div><span style="color: var(--text-muted);">Last Seen:</span> <span style="color: var(--text-primary);">${formatLogTime(device.LastSeenOnlineAt)}</span></div>` : ''}
                    </div>
                </div>
                
                ${device.os ? `
                    <div>
                        <h4 class="text-green-500 font-semibold mb-2">Operating System</h4>
                        <div class="space-y-2 text-sm">
                            <div><span style="color: var(--text-muted);">OS:</span> <span style="color: var(--text-primary);">${device.os.name || 'Unknown'}</span></div>
                            ${device.os.version ? `<div><span style="color: var(--text-muted);">Version:</span> <span style="color: var(--text-primary);">${device.os.version}</span></div>` : ''}
                            ${device.os.cpe ? `<div><span style="color: var(--text-muted);">CPE:</span> <span class="text-xs" style="color: var(--text-secondary);">${device.os.cpe}</span></div>` : ''}
                        </div>
                    </div>
                ` : ''}
            </div>
            
            <!-- Editable Fields -->
            <div class="mb-6 space-y-4">
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <!-- Editable Name Field -->
                    <div>
                        <label class="text-green-500 font-semibold block mb-2">Device Name</label>
                        <input type="text" 
                               id="device-name-${device.id}" 
                               value="${device.name || ''}" 
                               placeholder="Enter device name"
                               class="px-4 py-3 rounded w-full focus:border-green-500 focus:outline-none focus:ring-1 focus:ring-green-500 transition-colors"
                               style="background: var(--bg-tertiary); color: var(--text-primary); border: 1px solid var(--border-color);">
                    </div>
                </div>
                
                <!-- Editable Comment Field - Full Width -->
                <div>
                    <label class="text-green-500 font-semibold block mb-2">Comments & Notes</label>
                    <textarea id="device-comment-${device.id}" 
                              placeholder="Add comments, notes, or observations about this device..."
                              rows="6"
                              class="px-4 py-3 rounded w-full focus:border-green-500 focus:outline-none focus:ring-1 focus:ring-green-500 resize-y transition-colors"
                              style="background: var(--bg-tertiary); color: var(--text-primary); border: 1px solid var(--border-color);">${device.comment || ''}</textarea>
                </div>
            </div>
            
            <!-- Ports -->
            ${device.ports && device.ports.length > 0 ? `
                <div class="mb-6">
                    <h4 class="text-green-500 font-semibold mb-3">Open Ports</h4>
                    <div class="rounded p-3 max-h-32 overflow-y-auto" style="background: var(--bg-tertiary); border: 1px solid var(--border-color);">
                        <div class="space-y-1">
                            ${device.ports.filter(port => port.state === 'open' || port.state === 'filtered').map(port => `
                                <div class="flex items-center justify-between text-xs py-1">
                                    <div class="flex items-center space-x-2">
                                        <span class="text-green-400 font-medium">${port.number || port.Port}/${port.protocol || port.Protocol}</span>
                                        <span style="color: var(--text-secondary);">${port.service || port.Service || 'unknown'}</span>
                                    </div>
                                    <span class="text-xs font-bold uppercase ${port.state === 'open' ? 'text-red-500' : port.state === 'filtered' ? 'text-yellow-500' : 'text-gray-500'}">${port.state}</span>
                                </div>
                            `).join('')}
                        </div>
                    </div>
                </div>
            ` : ''}
            
            <!-- Actions -->
            <div class="flex justify-between items-center pt-4" style="border-top: 1px solid var(--border-color);">
                <button type="button" class="px-4 py-2 rounded transition-colors" style="color: var(--text-muted); border: 1px solid var(--border-color); background: transparent;" onmouseover="this.style.background='var(--bg-tertiary)'" onmouseout="this.style.background='transparent'" onclick="closeModal('deviceModal')">
                    Close
                </button>
                <div class="flex gap-3">
                    <button type="button" class="px-4 py-2 bg-green-500 hover:bg-green-600 text-white rounded transition-colors" onclick="saveDeviceChanges('${device.id}')">
                        Save Changes
                    </button>
                    <button type="button" class="px-3 py-2 rounded transition-colors" style="color: #ef4444; border: 1px solid #ef4444; background: transparent;" onmouseover="this.style.background='#ef4444'; this.style.color='white';" onmouseout="this.style.background='transparent'; this.style.color='#ef4444';" onclick="deleteDevice('${device.id}', '${device.ipv4}'); closeModal('deviceModal')" title="Delete Device">
                        <i class="ti ti-trash"></i> Delete
                    </button>
                </div>
            </div>
        </div>
    `;
}

function getStatusBadgeColor(status) {
    switch (status) {
        case 'online': return 'bg-green-500 text-white';
        case 'offline': return 'bg-red-500 text-white';
        case 'idle': return 'bg-yellow-500 text-black';
        default: return 'bg-gray-500 text-white';
    }
}

function formatLogTime(dateString) {
    if (!dateString) return 'Never';
    try {
        const date = new Date(dateString);
        return date.toLocaleString();
    } catch (error) {
        return 'Invalid date';
    }
}

function saveDeviceChanges(deviceId) {
    const nameInput = document.getElementById(`device-name-${deviceId}`);
    const commentInput = document.getElementById(`device-comment-${deviceId}`);

    if (!nameInput || !commentInput) {
        console.error('Device input fields not found');
        return;
    }

    const data = {
        name: nameInput.value.trim(),
        comment: commentInput.value.trim()
    };

    fetch(`/api/devices/${deviceId}`, {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
        credentials: 'include'
    })
    .then(response => response.json())
    .then(result => {
        if (result.success) {
            // Close the modal
            closeModal('deviceModal');

            // Refresh the devices list
            if (typeof loadDevices === 'function') {
                loadDevices(false);
            }

            // Refresh device list table if it exists
            if (typeof loadDeviceList === 'function') {
                loadDeviceList();
            }

            // Refresh network map if it exists
            if (typeof window.loadNetworkMap === 'function') {
                window.loadNetworkMap();
            }
        } else {
            alert('Failed to save device changes: ' + (result.error || 'Unknown error'));
        }
    })
    .catch(error => {
        console.error('Error saving device changes:', error);
        alert('Failed to save device changes');
    });
}

function deleteDevice(deviceId, deviceIP) {
    if (confirm(`Are you sure you want to delete device ${deviceIP}? This action cannot be undone.`)) {
        fetch(`/api/devices/${deviceId}`, {
            method: 'DELETE',
            credentials: 'include'
        })
        .then(response => {
            if (response.ok) {
                loadDevices(); // Reload the device list
            } else {
                alert('Failed to delete device');
            }
        })
        .catch(error => {
            console.error('Error deleting device:', error);
            alert('Failed to delete device');
        });
    }
}

function loadDeviceList() {
    const targetEl = document.getElementById('device-list-container');
    if (targetEl) {
        targetEl.innerHTML = '<div class="flex items-center justify-center py-8"><div class="animate-spin rounded-full h-8 w-8 border-2 border-green-500 border-t-transparent"></div><span class="ml-3 text-gray-400">Loading devices...</span></div>';
        
        fetch('/api/device-list', { credentials: 'include' })
            .then(response => response.json())
            .then(data => {
                targetEl.innerHTML = renderDeviceTable(data.devices || []);
            })
            .catch(error => {
                console.error('Error loading device list:', error);
                targetEl.innerHTML = '<div class="text-red-400">Failed to load devices</div>';
            });
    }
}

function renderDeviceTable(devices) {
    if (!devices || devices.length === 0) {
        return '<div class="text-center text-gray-400 py-8">No devices found</div>';
    }
    
    return `
        <div class="rounded overflow-hidden" style="background: var(--bg-secondary);">
            <table class="w-full">
                <thead style="background: var(--bg-primary);">
                    <tr>
                        <th class="px-4 py-3 text-left text-green-500">IP Address</th>
                        <th class="px-4 py-3 text-left text-green-500">Name</th>
                        <th class="px-4 py-3 text-left text-green-500">Network Info</th>
                        <th class="px-4 py-3 text-left text-green-500">Status</th>
                        <th class="px-4 py-3 text-left text-green-500">Ports</th>
                        <th class="px-4 py-3 text-left text-green-500">Actions</th>
                    </tr>
                </thead>
                <tbody>
                    ${devices.map(device => `
                        <tr class="cursor-pointer" style="transition: background 0.2s;" onmouseover="this.style.background='var(--bg-tertiary)'" onmouseout="this.style.background='';" onclick="loadDeviceModal('${device.id}')">
                            <td class="px-4 py-3">
                                <div class="flex items-center">
                                    <div class="w-3 h-3 rounded-full mr-3 ${getStatusColor(device.status)}"></div>
                                    <div class="device-ip" style="font-size: 1.1rem;">${device.ipv4}</div>
                                </div>
                            </td>
                            <td class="px-4 py-3">
                                ${device.name || device.hostname ? `<div class="text-gray-300">${device.name || device.hostname}</div>` : '<span class="text-gray-500">-</span>'}
                            </td>
                            <td class="px-4 py-3">
                                ${device.mac ? `<div class="text-blue-400 text-sm">${device.mac}</div>` : ''}
                            </td>
                            <td class="px-4 py-3">
                                <span class="px-2 py-1 rounded text-xs ${getStatusBadgeColor(device.status)}">${device.status}</span>
                            </td>
                            <td class="px-4 py-3">
                                ${device.ports && device.ports.length > 0 ? 
                                    `<div class="text-sm text-gray-400">${device.ports.filter(p => p.state === 'open').length} open</div>` 
                                    : '<span class="text-gray-500">-</span>'
                                }
                            </td>
                            <td class="px-4 py-3">
                                <button class="px-2 py-1 text-red-400 rounded text-sm hover:bg-red-400 hover:text-white transition-colors" 
                                        onclick="event.stopPropagation(); deleteDevice('${device.id}', '${device.ipv4}')"
                                        title="Delete">
                                    <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                                        <path d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"/>
                                    </svg>
                                </button>
                            </td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        </div>
    `;
}

// Make functions available globally
window.loadDevices = loadDevices;
window.loadDeviceList = loadDeviceList;
window.renderDeviceGrid = renderDeviceGrid;
window.renderDeviceTable = renderDeviceTable;
window.loadDeviceModal = loadDeviceModal;
window.renderDeviceModal = renderDeviceModal;
window.getStatusBadgeColor = getStatusBadgeColor;
window.formatLogTime = formatLogTime;
window.saveDeviceChanges = saveDeviceChanges;
window.deleteDevice = deleteDevice;