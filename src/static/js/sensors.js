// Sensors management - uses same patterns as existing JS files

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

    const table = document.createElement('table');
    table.className = 'w-full text-sm';
    
    const thead = document.createElement('thead');
    thead.innerHTML = '<tr class="border-b border-gray-700"><th class="text-left p-3 text-green-500">Name</th><th class="text-left p-3 text-green-500">Status</th><th class="text-left p-3 text-green-500">IP</th><th class="text-left p-3 text-green-500">Network</th><th class="text-left p-3 text-green-500">Last Seen</th><th class="text-left p-3 text-green-500">Token</th><th class="text-right p-3 text-green-500">Actions</th></tr>';
    table.appendChild(thead);

    const tbody = document.createElement('tbody');
    sensors.forEach(sensor => {
        const row = document.createElement('tr');
        row.className = 'border-b border-gray-700 hover:bg-gray-750';
        
        const isScanning = sensor.scan_status === 'running';
        const statusClass = sensor.status === 'online' ? 'text-green-500' : sensor.status === 'offline' ? 'text-red-500' : 'text-yellow-500';
        const statusDisplay = isScanning ? '<span class="inline-flex items-center"><span class="relative flex h-2 w-2 mr-2"><span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span><span class="relative inline-flex rounded-full h-2 w-2 bg-green-500"></span></span>SCANNING</span>' : sensor.status.toUpperCase();
        const statusAnimClass = isScanning ? statusClass + ' animate-pulse' : statusClass;
        const lastSeen = sensor.last_seen_at ? new Date(sensor.last_seen_at).toLocaleString() : 'Never';
        const token = sensor.token ? sensor.token.substring(0, 12) + '...' : '-';
        
        row.innerHTML = '<td class="p-3"><span class="font-medium text-white">' + escapeHtml(sensor.name) + '</span></td>' +
            '<td class="p-3 ' + statusAnimClass + '">' + statusDisplay + '</td>' +
            '<td class="p-3 text-gray-300">' + (sensor.ip || '-') + '</td>' +
            '<td class="p-3 text-gray-300">' + (sensor.network_cidr || '-') + '</td>' +
            '<td class="p-3 text-gray-400 text-xs">' + lastSeen + '</td>' +
            '<td class="p-3"><button onclick="copyToken(\'' + sensor.token + '\')" class="text-green-500 hover:text-green-400 text-xs">' + token + '</button></td>' +
            '<td class="p-3 text-right"><button onclick="deleteSensor(\'' + sensor.id + '\', \'' + escapeHtml(sensor.name) + '\')" class="text-red-500 hover:text-red-400 p-1">Delete</button></td>';
        tbody.appendChild(row);
    });
    table.appendChild(tbody);
    container.appendChild(table);
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
        setInterval(loadSensors, 30000);
    }
});

window.loadSensors = loadSensors;
window.openCreateSensorModal = openCreateSensorModal;
window.createSensor = createSensor;
window.deleteSensor = deleteSensor;
window.copyToken = copyToken;

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
