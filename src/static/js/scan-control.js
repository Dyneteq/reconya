// Scan control functionality - Sensor-based scanning

function escapeHtmlText(text) {
    if (text === undefined || text === null) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.textContent;
}

function renderScanControlFromData(data) {
    const sensors = data.sensors || [];

    if (sensors.length === 0) {
        return `
            <div id="scan-control-content">
                <div class="text-center py-4">
                    <div class="text-gray-400 mb-3">
                        <svg class="w-8 h-8 mx-auto mb-2 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z"></path>
                        </svg>
                        <p class="text-sm">No sensors configured</p>
                    </div>
                    <a href="/sensors" class="text-green-500 hover:text-green-400 text-xs">Add a sensor</a>
                </div>
            </div>
        `;
    }

    const sensorRows = sensors.map(sensor => {
        const isOnline = sensor.status === 'online';
        const isScanning = sensor.scan_status === 'running';
        const isPending = sensor.scan_status === 'pending';
        const networkRange = sensor.network_cidr || 'Not connected';

        const statusDot = isOnline
            ? 'bg-green-500'
            : sensor.status === 'pending'
                ? 'bg-yellow-500'
                : 'bg-red-500';

        let actionButton = '';
        if (!isOnline) {
            actionButton = '<span class="text-gray-500 text-xs">Offline</span>';
        } else if (isPending) {
            actionButton = '<span class="text-yellow-400 text-xs animate-pulse">Starting...</span>';
        } else if (isScanning) {
            actionButton = '<button onclick="stopSensorScan(\'' + sensor.id + '\', this)" class="px-2 py-1 bg-red-600 hover:bg-red-700 text-white text-xs rounded transition-colors">Stop</button>';
        } else {
            actionButton = '<button onclick="startSensorScan(\'' + sensor.id + '\', this)" class="px-2 py-1 bg-green-600 hover:bg-green-700 text-white text-xs rounded transition-colors">Start</button>';
        }

        return '<div class="flex items-center justify-between py-2 border-b border-gray-700 last:border-0">' +
            '<div class="flex items-center flex-1 min-w-0">' +
                '<div class="w-2 h-2 rounded-full ' + statusDot + ' mr-2 flex-shrink-0"></div>' +
                '<div class="min-w-0">' +
                    '<div class="text-sm text-white truncate">' + escapeHtmlText(sensor.name) + '</div>' +
                    '<div class="text-xs text-gray-500 truncate">' + escapeHtmlText(networkRange) + '</div>' +
                '</div>' +
            '</div>' +
            '<div class="flex-shrink-0 ml-2">' + actionButton + '</div>' +
        '</div>';
    }).join('');

    const scanningSensors = sensors.filter(s => s.scan_status === 'running' || s.scan_status === 'pending').length;
    const onlineSensors = sensors.filter(s => s.status === 'online').length;

    let statusText = '';
    if (scanningSensors > 0) {
        statusText = '<span class="text-green-400">' + scanningSensors + ' scanning</span>';
    } else if (onlineSensors > 0) {
        statusText = '<span>' + onlineSensors + ' online</span>';
    }

    return `
        <div id="scan-control-content">
            <div class="flex items-center justify-between mb-3">
                <h3 class="text-xs font-bold text-green-500">SENSORS</h3>
                <div class="text-xs text-gray-400">${statusText}</div>
            </div>
            <div class="space-y-0 max-h-48 overflow-y-auto">
                ${sensorRows}
            </div>
            <div class="mt-3 pt-3 border-t border-gray-700">
                <a href="/sensors" class="text-green-500 hover:text-green-400 text-xs flex items-center justify-center">
                    <svg class="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"></path>
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
                    </svg>
                    Manage Sensors
                </a>
            </div>
        </div>
    `;
}

// Cache to prevent duplicate concurrent requests
let scanControlLoading = false;

function loadScanControl(showSpinner = true) {
    const scanControlContainer = document.getElementById('scan-control-container');
    if (!scanControlContainer) return;

    if (scanControlLoading) return;
    scanControlLoading = true;

    const loadingEl = document.getElementById('scan-control-loading');
    const contentEl = document.getElementById('scan-control-content');

    if (showSpinner && loadingEl && contentEl) {
        loadingEl.classList.remove('hidden');
        contentEl.classList.add('hidden');
    }

    fetch('/api/sensors', { credentials: 'include' })
        .then(response => {
            if (!response.ok) throw new Error('Failed to load sensors');
            return response.json();
        })
        .then(sensors => {
            scanControlLoading = false;
            scanControlContainer.querySelector('#scan-control-loading')?.classList.add('hidden');
            const content = scanControlContainer.querySelector('#scan-control-content') || scanControlContainer;
            content.classList.remove('hidden');
            content.outerHTML = renderScanControlFromData({ sensors: sensors });
        })
        .catch(error => {
            scanControlLoading = false;
            console.error('Error loading scan control:', error);
            const content = scanControlContainer.querySelector('#scan-control-content') || scanControlContainer;
            content.outerHTML = '<div id="scan-control-content" class="text-center py-4"><div class="text-red-400 text-sm">Failed to load sensors</div></div>';
        });
}

function startSensorScan(sensorId, buttonEl) {
    if (!sensorId) return;
    const originalText = buttonEl.textContent;
    buttonEl.disabled = true;
    buttonEl.textContent = '...';

    fetch('/api/sensors/' + sensorId + '/start-scan', {
        method: 'POST',
        credentials: 'include'
    })
        .then(response => {
            if (!response.ok) return response.text().then(t => { throw new Error(t || 'Failed'); });
        })
        .then(() => {
            loadScanControl(false);
            if (typeof window.loadDevices === 'function') setTimeout(() => window.loadDevices(false), 1000);
            if (typeof window.loadNetworkMap === 'function') setTimeout(() => window.loadNetworkMap(), 1000);
        })
        .catch(error => {
            console.error('Failed to start scan:', error);
            alert('Failed to start scan: ' + error.message);
            buttonEl.disabled = false;
            buttonEl.textContent = originalText;
        });
}

function stopSensorScan(sensorId, buttonEl) {
    if (!sensorId) return;
    const originalText = buttonEl.textContent;
    buttonEl.disabled = true;
    buttonEl.textContent = '...';

    fetch('/api/sensors/' + sensorId + '/stop-scan', {
        method: 'POST',
        credentials: 'include'
    })
        .then(response => {
            if (!response.ok) return response.text().then(t => { throw new Error(t || 'Failed'); });
        })
        .then(() => {
            loadScanControl(false);
        })
        .catch(error => {
            console.error('Failed to stop scan:', error);
            alert('Failed to stop scan: ' + error.message);
            buttonEl.disabled = false;
            buttonEl.textContent = originalText;
        });
}

// Make functions available globally
window.renderScanControlFromData = renderScanControlFromData;
window.loadScanControl = loadScanControl;
window.startSensorScan = startSensorScan;
window.stopSensorScan = stopSensorScan;

// End of scan-control.js - sensor-based implementation
