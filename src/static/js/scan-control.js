function escapeHtmlText(text) {
    if (text === undefined || text === null) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.textContent;
}

const pendingSensors = new Set();
let cachedSensors = [];

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
        const isPending = pendingSensors.has(sensor.id);
        const networkRange = sensor.network_cidr || 'Not connected';

        const statusDot = isOnline
            ? (isScanning ? 'bg-green-500 animate-pulse' : 'bg-green-500')
            : sensor.status === 'pending'
                ? 'bg-yellow-500'
                : 'bg-red-500';

        let actionButton = '';
        if (!isOnline) {
            actionButton = '<span class="text-gray-500 text-xs">Offline</span>';
        } else if (isPending) {
            actionButton = `<span class="text-yellow-400 text-xs flex items-center">
                <svg class="animate-spin h-3 w-3 mr-1" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                ${isScanning ? 'Stopping...' : 'Starting...'}
            </span>`;
        } else if (isScanning) {
            actionButton = '<button onclick="stopSensorScan(\'' + sensor.id + '\')" class="px-2 py-1 bg-red-600 hover:bg-red-700 text-white text-xs rounded transition-colors">Stop</button>';
        } else {
            actionButton = '<button onclick="startSensorScan(\'' + sensor.id + '\')" class="px-2 py-1 bg-green-600 hover:bg-green-700 text-white text-xs rounded transition-colors">Start</button>';
        }

        const statusText = isScanning && !isPending
            ? '<span class="text-green-400 font-mono text-xs">' + escapeHtmlText(networkRange) + '</span>'
            : '<span class="text-gray-500">' + escapeHtmlText(networkRange) + '</span>';

        const statusDotHtml = isScanning && !isPending
            ? '<svg class="animate-spin h-4 w-4 mr-2 flex-shrink-0 text-green-500" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>'
            : '<div class="w-3 h-3 rounded-full ' + statusDot + ' mr-2 flex-shrink-0"></div>';

        return '<div class="flex items-center justify-between py-2 border-b border-gray-700 last:border-0">' +
            '<div class="flex items-center flex-1 min-w-0">' +
                statusDotHtml +
                '<div class="min-w-0">' +
                    '<div class="text-sm text-white truncate">' + escapeHtmlText(sensor.name) + '</div>' +
                    '<div class="text-xs truncate">' + statusText + '</div>' +
                '</div>' +
            '</div>' +
            '<div class="flex-shrink-0 ml-2">' + actionButton + '</div>' +
        '</div>';
    }).join('');

    const scanningSensors = sensors.filter(s => s.scan_status === 'running').length;
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
        </div>
    `;
}

function renderToContainer() {
    const container = document.getElementById('scan-control-container');
    if (!container) return;

    const loadingEl = container.querySelector('#scan-control-loading');
    if (loadingEl) loadingEl.remove();

    const temp = document.createElement('div');
    temp.innerHTML = renderScanControlFromData({ sensors: cachedSensors });
    const newContent = temp.firstElementChild;

    const oldContent = container.querySelector('#scan-control-content');
    if (oldContent) {
        oldContent.replaceWith(newContent);
    } else {
        container.appendChild(newContent);
    }
}

function loadScanControl() {
    const container = document.getElementById('scan-control-container');
    if (!container) return;

    fetch('/api/sensors', { credentials: 'include' })
        .then(response => {
            if (!response.ok) throw new Error('Failed to load sensors');
            return response.json();
        })
        .then(sensors => {
            cachedSensors = sensors;
            renderToContainer();
        })
        .catch(error => {
            console.error('Error loading scan control:', error);
        });
}


const actionStartTimes = new Map();
const MIN_SPINNER_DURATION = 1500;

function pollForStatusChange(sensorId, expectedStatus, attempts = 0) {
    if (attempts > 20) {
        pendingSensors.delete(sensorId);
        actionStartTimes.delete(sensorId);
        loadScanControl();
        return;
    }

    fetch('/api/sensors', { credentials: 'include' })
        .then(response => response.json())
        .then(sensors => {
            const sensor = sensors.find(s => s.id === sensorId);
            if (sensor && sensor.scan_status === expectedStatus) {
                const startTime = actionStartTimes.get(sensorId) || Date.now();
                const elapsed = Date.now() - startTime;
                const remainingDelay = Math.max(0, MIN_SPINNER_DURATION - elapsed);

                setTimeout(() => {
                    pendingSensors.delete(sensorId);
                    actionStartTimes.delete(sensorId);
                    loadScanControl();
                    if (expectedStatus === 'running') {
                        if (typeof window.loadDevices === 'function') {
                            setTimeout(() => window.loadDevices(false), 1000);
                        }
                    }
                }, remainingDelay);
            } else {
                setTimeout(() => pollForStatusChange(sensorId, expectedStatus, attempts + 1), 500);
            }
        })
        .catch(() => {
            setTimeout(() => pollForStatusChange(sensorId, expectedStatus, attempts + 1), 500);
        });
}

function startSensorScan(sensorId) {
    if (!sensorId || pendingSensors.has(sensorId)) return;

    pendingSensors.add(sensorId);
    actionStartTimes.set(sensorId, Date.now());
    renderToContainer();

    fetch('/api/sensors/' + sensorId + '/start-scan', {
        method: 'POST',
        credentials: 'include'
    })
        .then(response => {
            if (!response.ok) throw new Error('Failed to start scan');
            pollForStatusChange(sensorId, 'running');
        })
        .catch(error => {
            console.error('Failed to start scan:', error);
            alert('Failed to start scan');
            pendingSensors.delete(sensorId);
            actionStartTimes.delete(sensorId);
            renderToContainer();
        });
}

function stopSensorScan(sensorId) {
    if (!sensorId || pendingSensors.has(sensorId)) return;

    pendingSensors.add(sensorId);
    actionStartTimes.set(sensorId, Date.now());
    renderToContainer();

    fetch('/api/sensors/' + sensorId + '/stop-scan', {
        method: 'POST',
        credentials: 'include'
    })
        .then(response => {
            if (!response.ok) throw new Error('Failed to stop scan');
            pollForStatusChange(sensorId, 'idle');
        })
        .catch(error => {
            console.error('Failed to stop scan:', error);
            alert('Failed to stop scan');
            pendingSensors.delete(sensorId);
            actionStartTimes.delete(sensorId);
            renderToContainer();
        });
}

window.renderScanControlFromData = renderScanControlFromData;
window.loadScanControl = loadScanControl;
window.startSensorScan = startSensorScan;
window.stopSensorScan = stopSensorScan;
