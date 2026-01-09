function loadSensors() {
    const container = document.getElementById('sensors-container');
    if (!container) return;

    fetch('/api/sensors')
        .then(response => response.json())
        .then(sensors => renderSensorsTable(container, sensors))
        .catch(error => {
            console.error('Error loading sensors:', error);
            container.textContent = 'Error loading sensors';
        });
}

function renderSensorsTable(container, sensors) {
    container.textContent = '';

    if (!sensors || sensors.length === 0) {
        const emptyDiv = document.createElement('div');
        emptyDiv.className = 'p-8 text-center text-gray-400';
        emptyDiv.textContent = 'No sensors configured yet. Click "Add Sensor" to create one.';
        container.appendChild(emptyDiv);
        return;
    }

    const grid = document.createElement('div');
    grid.className = 'flex flex-col gap-4';

    sensors.forEach(sensor => {
        const isOnline = sensor.status === 'online';
        const isScanning = sensor.scan_status === 'running';
        const statusClass = isOnline ? 'text-green-500' : sensor.status === 'offline' ? 'text-red-500' : 'text-yellow-500';
        const statusDot = isOnline ? 'bg-green-500' : sensor.status === 'offline' ? 'bg-red-500' : 'bg-yellow-500';
        const statusDisplay = sensor.status ? sensor.status.toUpperCase() : 'UNKNOWN';
        const devicesTotal = sensor.devices_total || 0;
        const devicesOnline = sensor.devices_online || 0;
        const devices = sensor.devices || [];

        let scanButton = '';
        if (!isOnline) {
            scanButton = '<span class="text-gray-500 text-xs">Offline</span>';
        } else if (isScanning) {
            scanButton = '<button onclick="stopSensorScanPage(\'' + sensor.id + '\')" class="px-3 py-1.5 bg-red-600 hover:bg-red-700 text-white text-xs rounded inline-flex items-center"><svg class="animate-spin h-3 w-3 mr-1" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>Stop Scan</button>';
        } else {
            scanButton = '<button onclick="startSensorScanPage(\'' + sensor.id + '\')" class="px-3 py-1.5 bg-green-600 hover:bg-green-700 text-white text-xs rounded">Start Scan</button>';
        }

        let deviceGrid = '';
        if (devices.length > 0) {
            deviceGrid = '<div class="flex flex-wrap gap-1">';
            devices.forEach(device => {
                const lastOctet = device.ip.split('.').pop();
                let boxClass = 'w-8 h-6 rounded text-xs font-mono flex items-center justify-center ';
                if (device.status === 'online') {
                    boxClass += 'bg-green-600 text-white';
                } else if (device.status === 'idle') {
                    boxClass += 'bg-green-400 text-green-900';
                } else {
                    boxClass += 'bg-gray-700 text-gray-400';
                }
                deviceGrid += '<div class="' + boxClass + '" title="' + device.ip + ' - ' + device.status + '">' + lastOctet + '</div>';
            });
            deviceGrid += '</div>';
        } else {
            deviceGrid = '<div class="text-gray-500 text-xs">No devices found</div>';
        }

        const recentLogs = sensor.recent_logs || [];
        let logsHtml = '';
        if (recentLogs.length > 0) {
            logsHtml = '<div class="space-y-1 max-h-24 overflow-y-auto">';
            recentLogs.slice(0, 5).forEach(log => {
                logsHtml += '<div class="text-xs flex justify-between"><span class="text-gray-300 truncate">' + escapeHtml(log.description) + '</span><span class="text-gray-500 ml-2">' + log.created_at + '</span></div>';
            });
            logsHtml += '</div>';
        } else {
            logsHtml = '<div class="text-gray-500 text-xs">No recent logs</div>';
        }

        const card = document.createElement('div');
        card.className = 'bg-gray-800 rounded-lg p-4 border border-gray-700';
        card.innerHTML =
            '<div class="flex items-center justify-between mb-3">' +
                '<div class="flex items-center">' +
                    '<div class="w-2.5 h-2.5 rounded-full ' + statusDot + ' mr-2"></div>' +
                    '<span class="font-medium text-white text-lg">' + escapeHtml(sensor.name) + '</span>' +
                    (isScanning ? '<svg class="animate-spin h-4 w-4 ml-2 text-green-500" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>' : '') +
                '</div>' +
                '<span class="text-gray-500 text-xs">' + statusDisplay + '</span>' +
            '</div>' +
            '<div class="grid grid-cols-5 gap-4 mb-4">' +
                '<div>' +
                    '<div class="text-gray-500 text-xs">Sensor IP</div>' +
                    '<div class="text-gray-200 font-mono text-sm">' + (sensor.ip || '-') + '</div>' +
                '</div>' +
                '<div>' +
                    '<div class="text-gray-500 text-xs">Public IP</div>' +
                    '<div class="text-gray-200 font-mono text-sm">' + (sensor.public_ip || '-') + '</div>' +
                '</div>' +
                '<div>' +
                    '<div class="text-gray-500 text-xs">Location</div>' +
                    '<div class="text-gray-200 text-sm">' + (sensor.geolocation ? (sensor.geolocation.city + ', ' + sensor.geolocation.country_code) : '-') + '</div>' +
                '</div>' +
                '<div>' +
                    '<div class="text-gray-500 text-xs">Network</div>' +
                    '<div class="text-gray-200 font-mono text-sm">' + (sensor.network_cidr || '-') + '</div>' +
                '</div>' +
                '<div>' +
                    '<div class="text-gray-500 text-xs">Devices</div>' +
                    '<div class="text-gray-200"><span class="text-green-400 font-medium">' + devicesOnline + '</span> / ' + devicesTotal + '</div>' +
                '</div>' +
            '</div>' +
            '<div class="grid grid-cols-4 gap-4">' +
                '<div class="col-span-3">' +
                    '<div class="text-gray-500 text-xs mb-2">Device Map</div>' +
                    deviceGrid +
                '</div>' +
                '<div>' +
                    '<div class="text-gray-500 text-xs mb-2">Recent Activity</div>' +
                    logsHtml +
                '</div>' +
            '</div>' +
            '<div class="border-t border-gray-700 mt-4 pt-3 flex items-center justify-between">' +
                '<button onclick="copyToken(\'' + sensor.token + '\')" class="text-green-500 hover:text-green-400 text-xs font-mono">Copy Token</button>' +
                '<div class="flex items-center gap-3">' +
                    scanButton +
                    '<button onclick="deleteSensor(\'' + sensor.id + '\', \'' + escapeHtml(sensor.name) + '\')" class="text-red-500 hover:text-red-400 text-xs">Delete</button>' +
                '</div>' +
            '</div>';
        grid.appendChild(card);
    });

    container.appendChild(grid);
}

function startSensorScanPage(sensorId) {
    fetch('/api/sensors/' + sensorId + '/start-scan', { method: 'POST', credentials: 'include' })
        .then(response => {
            if (!response.ok) throw new Error('Failed to start scan');
            loadSensors();
        })
        .catch(error => alert('Failed to start scan: ' + error.message));
}

function stopSensorScanPage(sensorId) {
    fetch('/api/sensors/' + sensorId + '/stop-scan', { method: 'POST', credentials: 'include' })
        .then(response => {
            if (!response.ok) throw new Error('Failed to stop scan');
            loadSensors();
        })
        .catch(error => alert('Failed to stop scan: ' + error.message));
}

function openCreateSensorModal() {
    const modal = document.getElementById('modal');
    const modalContent = document.getElementById('modal-content');
    if (!modal || !modalContent) return;
    
    modalContent.innerHTML = '<div class="flex justify-between items-center mb-4 pb-3 border-b border-green-600"><h5 class="text-xl font-bold text-green-500">Create Sensor</h5><button type="button" class="text-gray-400 hover:text-white text-xl" onclick="hideSensorModal()">X</button></div><form onsubmit="createSensor(event)"><div class="mb-4"><label class="block text-gray-300 text-sm mb-2">Sensor Name</label><input type="text" id="sensor-name" class="w-full bg-gray-700 text-white rounded px-3 py-2" placeholder="e.g., Office Scanner" required></div><div class="flex justify-end gap-2 pt-3 border-t border-gray-700"><button type="button" class="border border-gray-500 text-gray-300 px-4 py-2 rounded text-sm" onclick="hideSensorModal()">Cancel</button><button type="submit" class="bg-green-600 text-white px-4 py-2 rounded text-sm">Create</button></div></form>';
    showSensorModal(modal);
    setTimeout(() => document.getElementById('sensor-name').focus(), 100);
}

function createSensor(event) {
    event.preventDefault();
    const name = document.getElementById('sensor-name').value.trim();
    if (!name) return;

    fetch('/api/sensors', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: name })
    })
    .then(response => response.json())
    .then(sensor => {
        hideSensorModal();
        showSensorToken(sensor);
        loadSensors();
    })
    .catch(error => alert('Failed to create sensor: ' + error.message));
}

function showSensorToken(sensor) {
    const modal = document.getElementById('modal');
    const modalContent = document.getElementById('modal-content');
    if (!modal || !modalContent) return;
    
    modalContent.innerHTML = '<div class="flex justify-between items-center mb-4 pb-3 border-b border-green-600"><h5 class="text-xl font-bold text-green-500">Sensor Created</h5><button type="button" class="text-gray-400 hover:text-white" onclick="hideSensorModal()">X</button></div><p class="text-gray-300 mb-4">Sensor <strong>' + escapeHtml(sensor.name) + '</strong> created!</p><div class="bg-gray-900 rounded p-4 mb-4"><label class="block text-gray-400 text-xs mb-2">Token (copy this)</label><code class="text-green-400 text-sm break-all">' + sensor.token + '</code><button onclick="copyToken(\'' + sensor.token + '\')" class="ml-2 bg-green-600 text-white px-2 py-1 rounded text-xs">Copy</button></div><div class="bg-blue-900 rounded p-3 text-sm text-blue-300"><p>Start agent with:</p><code class="block mt-1 text-green-400">./reconya agent primary --server http://SERVER:3000 --token ' + sensor.token + '</code></div><div class="flex justify-end pt-3"><button onclick="hideSensorModal()" class="bg-green-600 text-white px-4 py-2 rounded">Done</button></div>';
    showSensorModal(modal);
}

function deleteSensor(id, name) {
    if (!confirm('Delete sensor "' + name + '"?')) return;
    fetch('/api/sensors/' + id, { method: 'DELETE' })
        .then(() => loadSensors())
        .catch(error => alert('Failed to delete: ' + error.message));
}

function copyToken(token) {
    navigator.clipboard.writeText(token).then(() => alert('Token copied!')).catch(() => alert('Copy failed'));
}

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

document.addEventListener('DOMContentLoaded', function() {
    if (document.getElementById('sensors-container')) {
        loadSensors();
        setInterval(loadSensors, 5000);
    }
});

window.loadSensors = loadSensors;
window.openCreateSensorModal = openCreateSensorModal;
window.createSensor = createSensor;
window.deleteSensor = deleteSensor;
window.copyToken = copyToken;
window.startSensorScanPage = startSensorScanPage;
window.stopSensorScanPage = stopSensorScanPage;

function showSensorModal(modal) {
    if (!modal) return;
    if (typeof showModal === 'function') {
        showModal(modal.id);
    } else {
        modal.classList.remove('hidden');
    }
}

function hideSensorModal() {
    const modal = document.getElementById('modal');
    if (!modal) return;
    if (typeof closeModal === 'function') {
        closeModal(modal.id);
    } else {
        modal.classList.add('hidden');
    }
}

window.hideSensorModal = hideSensorModal;
