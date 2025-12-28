// Dashboard functionality
function loadRecentActivity() {
    fetch('/api/event-logs', { credentials: 'include' })
        .then(response => response.json())
        .then(data => {
            renderRecentActivity(data.logs || []);
        })
        .catch(error => {
            console.error('Error loading recent activity:', error);
            // Show error in the UI
            const activityContainer = document.getElementById('activity-log');
            if (activityContainer) {
                activityContainer.innerHTML = `
                    <div class="text-center py-4 text-red-500">
                        <i class="bi bi-exclamation-triangle text-2xl mb-2 block"></i>
                        <p>Failed to load recent activity</p>
                    </div>
                `;
            }
        });
}

function renderRecentActivity(logs) {
    const activityContainer = document.getElementById('activity-log');
    if (!activityContainer) return;
    
    if (!logs || logs.length === 0) {
        activityContainer.innerHTML = `
            <div class="text-center py-4 text-gray-500">
                <i class="bi bi-journal-text text-2xl mb-2 block"></i>
                <p>No recent activity</p>
            </div>
        `;
        return;
    }
    
    let activityHTML = '<div class="space-y-1">';
    
    logs.slice(0, 10).forEach(log => { // Show only latest 10
        // Handle both snake_case and camelCase field names for compatibility
        const logType = log.type || log.Type || 'unknown';
        const logDescription = log.description || log.Description || 'Unknown activity';
        const logCreatedAt = log.created_at || log.CreatedAt || new Date().toISOString();
        
        const activityIcon = getActivityIcon(logType);
        const timeAgo = getTimeAgo(logCreatedAt);
        
        activityHTML += `
            <div class="flex items-center justify-between py-1">
                <div class="flex items-center">
                    <span class="text-xs">${logDescription}</span>
                </div>
                <span class="text-xs text-gray-500">${timeAgo}</span>
            </div>
        `;
    });
    
    activityHTML += '</div>';
    activityContainer.innerHTML = activityHTML;
}

function getActivityIcon(type) {
    switch (type) {
        case 'scan_started': return 'bi-play-circle';
        case 'scan_stopped': return 'bi-stop-circle';
        case 'device_online': return 'bi-check-circle';
        case 'device_offline': return 'bi-x-circle';
        case 'port_scan_started': return 'bi-search';
        case 'port_scan_completed': return 'bi-check2-circle';
        default: return 'bi-info-circle';
    }
}

function getTimeAgo(date) {
    try {
        if (!date) return 'Unknown';
        
        const now = new Date();
        const eventDate = new Date(date);
        
        // Check if the date is valid
        if (isNaN(eventDate.getTime())) {
            return 'Unknown';
        }
        
        const diffMs = now - eventDate;
        
        // Handle negative differences (future dates)
        if (diffMs < 0) return 'Just now';
        
        const diffMins = Math.floor(diffMs / 60000);
        const diffHours = Math.floor(diffMs / 3600000);
        const diffDays = Math.floor(diffMs / 86400000);
        
        if (diffMins < 1) return 'Just now';
        if (diffMins < 60) return `${diffMins}m ago`;
        if (diffHours < 24) return `${diffHours}h ago`;
        if (diffDays < 30) return `${diffDays}d ago`;
        return `${Math.floor(diffDays / 30)}mo ago`;
    } catch (error) {
        console.error('Error calculating time ago:', error);
        return 'Unknown';
    }
}

function loadDashboardMetrics() {
    fetch('/api/dashboard-metrics', { credentials: 'include' })
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to load dashboard metrics');
            }
            return response.json();
        })
        .then(data => {
            updateDashboardMetrics(data);
        })
        .catch(error => {
            console.error('Error loading dashboard metrics:', error);
        });
}

function updateDashboardMetrics(data) {
    const summary = data.summary || {};

    // Update the main metric cards
    const networkCountEl = document.getElementById('network-count');
    const devicesFoundEl = document.getElementById('devices-found');
    const devicesOnlineEl = document.getElementById('devices-online');
    const sensorsOnlineEl = document.getElementById('sensors-online');
    const saturationEl = document.getElementById('network-saturation');

    if (networkCountEl) networkCountEl.textContent = summary.networks || 0;
    if (devicesFoundEl) devicesFoundEl.textContent = summary.devicesFound || 0;
    if (devicesOnlineEl) devicesOnlineEl.textContent = summary.devicesOnline || 0;
    if (sensorsOnlineEl) sensorsOnlineEl.textContent = summary.sensorsOnline || 0;
    if (saturationEl) {
        const sat = typeof summary.saturation === 'number' ? summary.saturation.toFixed(1) : '0';
        saturationEl.textContent = sat + '%';
    }
}

function renderSensorCard(sensor) {
    const statusClass = sensor.status === 'online'
        ? 'bg-green-500'
        : sensor.status === 'offline'
            ? 'bg-red-500'
            : 'bg-yellow-400';
    const scanLabel = sensor.scanStatus === 'running' ? 'Scanning' : 'Idle';
    const badgeClass = sensor.scanStatus === 'running'
        ? 'bg-green-600/20 text-green-400 border border-green-500/50'
        : 'bg-gray-700 text-gray-300 border border-gray-600';

    const saturation = typeof sensor.saturation === 'number'
        ? sensor.saturation.toFixed(1) + '%'
        : '0%';

    const lastSeen = sensor.lastSeenAt ? getTimeAgo(sensor.lastSeenAt) : 'Never';

    const actionButton = sensor.scanStatus === 'running'
        ? `<button class="bg-red-600 hover:bg-red-700 text-white text-xs px-3 py-2 rounded transition-colors" onclick="stopSensorScan('${sensor.id}', this)">Stop Scan</button>`
        : `<button class="bg-green-600 hover:bg-green-700 text-white text-xs px-3 py-2 rounded transition-colors" onclick="startSensorScan('${sensor.id}', this)">Start Scan</button>`;

    return `
        <div class="bg-gray-800 rounded-lg p-5 flex flex-col justify-between h-full border border-gray-700/50 shadow-lg shadow-black/20">
            <div class="flex items-start justify-between mb-4">
                <div>
                    <p class="text-sm font-semibold text-white">${escapeHtml(sensor.name || 'Sensor')}</p>
                    <div class="flex items-center text-xs text-gray-400 mt-1">
                        <span class="flex items-center gap-2">
                            <span class="inline-flex h-2.5 w-2.5 rounded-full ${statusClass}"></span>
                            ${sensor.status ? sensor.status.toUpperCase() : 'UNKNOWN'}
                        </span>
                    </div>
                </div>
                <span class="px-2 py-1 text-[10px] uppercase tracking-widest rounded ${badgeClass}">
                    ${scanLabel}
                </span>
            </div>
            <dl class="grid grid-cols-2 gap-3 text-sm text-gray-300 flex-1">
                <div>
                    <dt class="text-xs text-gray-500">Network Range</dt>
                    <dd class="font-semibold">${escapeHtml(sensor.networkRange || 'N/A')}</dd>
                </div>
                <div>
                    <dt class="text-xs text-gray-500">Public IP</dt>
                    <dd class="font-semibold">${escapeHtml(sensor.publicIP || 'N/A')}</dd>
                </div>
                <div>
                    <dt class="text-xs text-gray-500">Devices Found</dt>
                    <dd class="font-semibold">${sensor.devicesFound ?? 0}</dd>
                </div>
                <div>
                    <dt class="text-xs text-gray-500">Online</dt>
                    <dd class="font-semibold">${sensor.devicesOnline ?? 0}</dd>
                </div>
                <div>
                    <dt class="text-xs text-gray-500">Idle</dt>
                    <dd class="font-semibold">${sensor.devicesIdle ?? 0}</dd>
                </div>
                <div>
                    <dt class="text-xs text-gray-500">Saturation</dt>
                    <dd class="font-semibold">${saturation}</dd>
                </div>
                <div>
                    <dt class="text-xs text-gray-500">Last Seen</dt>
                    <dd class="font-semibold">${lastSeen}</dd>
                </div>
            </dl>
            <div class="flex items-center justify-between mt-5 text-xs text-gray-500">
                <span>${sensor.scanStatus === 'running' && sensor.lastScanStartedAt ? `Started ${getTimeAgo(sensor.lastScanStartedAt)}` : ''}</span>
                ${actionButton}
            </div>
        </div>
    `;
}

function loadEventLogs() {
    const targetEl = document.getElementById('logs-container');
    if (targetEl) {
        targetEl.innerHTML = '<div class="flex items-center justify-center py-8"><div class="animate-spin rounded-full h-8 w-8 border-2 border-green-500 border-t-transparent"></div><span class="ml-3 text-gray-400">Loading logs...</span></div>';
        
        fetch('/api/event-logs', { credentials: 'include' })
            .then(response => response.json())
            .then(data => {
                targetEl.innerHTML = renderEventLogs(data.logs || []);
            })
            .catch(error => {
                console.error('Error loading event logs:', error);
                targetEl.innerHTML = '<div class="text-red-400">Failed to load logs</div>';
            });
    }
}

function renderEventLogs(logs) {
    logsData = logs;
    // Render logs and pagination controls separately
    setTimeout(renderLogsPaginationControls, 0);
    return renderEventLogsPage(logsPage, false);
}

function renderEventLogsPage(page, updatePagination = true) {
    if (!logsData || logsData.length === 0) {
        if (updatePagination) setTimeout(renderLogsPaginationControls, 0);
        return '<div class="text-center text-gray-400 py-8">No logs found</div>';
    }
    const start = (page - 1) * logsPerPage;
    const end = start + logsPerPage;
    const pageLogs = logsData.slice(start, end);
    let html = `<div class="space-y-2">${pageLogs.map(log => `
        <div class="flex items-center justify-between py-2 px-3 border-b border-gray-700 hover:bg-gray-800 rounded">
            <div class="flex items-center">
                <div class="w-2 h-2 rounded-full mr-3 ${getLogLevelColor(log.type)}"></div>
                <span class="text-gray-300 text-sm">${log.description || log.Description}</span>
            </div>
            <span class="text-xs text-gray-500">${formatLogTime(log.created_at || log.CreatedAt)}</span>
        </div>
    `).join('')}</div>`;
    if (updatePagination) setTimeout(renderLogsPaginationControls, 0);
    return html;
}

function escapeHtml(text) {
    if (text === undefined || text === null) {
        return '';
    }
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function renderLogsPaginationControls() {
    const paginationId = 'logs-pagination-controls';
    let paginationEl = document.getElementById(paginationId);
    const totalPages = Math.ceil(logsData.length / logsPerPage);
    if (!paginationEl) {
        paginationEl = document.createElement('div');
        paginationEl.id = paginationId;
        paginationEl.className = 'flex justify-center gap-2 mt-4';
        const logsContainer = document.getElementById('logs-container');
        if (logsContainer && logsContainer.parentNode) {
            logsContainer.parentNode.appendChild(paginationEl);
        }
    }
    if (totalPages > 1) {
        paginationEl.innerHTML = `
            <button class="px-3 py-1 rounded bg-gray-700 text-white" ${logsPage === 1 ? 'disabled' : ''} onclick="changeLogsPage(${logsPage - 1})">Prev</button>
            <span class="px-2 py-1">Page ${logsPage} of ${totalPages}</span>
            <button class="px-3 py-1 rounded bg-gray-700 text-white" ${logsPage === totalPages ? 'disabled' : ''} onclick="changeLogsPage(${logsPage + 1})">Next</button>
        `;
        paginationEl.style.display = '';
    } else {
        paginationEl.innerHTML = '';
        paginationEl.style.display = 'none';
    }
}

function getLogLevelColor(level) {
    switch(level?.toLowerCase()) {
        case 'error': return 'bg-red-500';
        case 'warning': return 'bg-yellow-500';
        case 'info': return 'bg-blue-500';
        case 'success': return 'bg-green-500';
        default: return 'bg-gray-500';
    }
}

function formatLogTime(timestamp) {
    if (!timestamp) return 'Unknown';
    try {
        const date = new Date(timestamp);
        if (isNaN(date.getTime())) return 'Invalid Date';
        return date.toLocaleString();
    } catch (e) {
        return 'Invalid Date';
    }
}

// LOGS PAGE BUTTONS AND PAGINATION

window.refreshLogs = function() {
    if (typeof loadEventLogs === 'function') {
        loadEventLogs();
    }
};

window.clearLogs = function() {
    if (confirm('Are you sure you want to clear all event logs? This action cannot be undone.')) {
        fetch('/api/event-logs/clear', { method: 'POST' })
            .then(response => {
                if (response.ok) {
                    refreshLogs();
                } else {
                    alert('Failed to clear logs');
                }
            })
            .catch(() => alert('Failed to clear logs'));
    }
};

// Pagination for logs
let logsPage = 1;
let logsPerPage = 50;
let logsData = [];

window.changeLogsPage = function(page) {
    if (page < 1) page = 1;
    const totalPages = Math.ceil(logsData.length / logsPerPage);
    if (page > totalPages) page = totalPages;
    logsPage = page;
    const logsContainer = document.getElementById('logs-container');
    if (logsContainer) {
        logsContainer.innerHTML = renderEventLogsPage(logsPage, false);
    }
    renderLogsPaginationControls();
};

// Load public IP
function loadPublicIP() {
    const ipEl = document.getElementById('public-ip-address');
    const locationEl = document.getElementById('public-ip-location');
    if (!ipEl) return;

    fetch('/api/system-status', { credentials: 'include' })
        .then(response => response.json())
        .then(data => {
            const status = data.SystemStatus || data.system_status || {};
            const publicIP = status.public_ip || status.PublicIP || 'N/A';
            const geo = status.Geolocation || status.geolocation;

            ipEl.textContent = publicIP;

            if (locationEl && geo) {
                const parts = [];
                if (geo.city || geo.City) parts.push(geo.city || geo.City);
                if (geo.country || geo.Country) parts.push(geo.country || geo.Country);
                locationEl.textContent = parts.join(', ');
            }
        })
        .catch(error => {
            console.error('Error loading public IP:', error);
            ipEl.textContent = 'Error';
        });
}

// Initialize public IP on load
document.addEventListener('DOMContentLoaded', function() {
    if (document.getElementById('public-ip-address')) {
        loadPublicIP();
    }
});

// Make functions available globally
window.loadRecentActivity = loadRecentActivity;
window.loadEventLogs = loadEventLogs;
window.renderEventLogs = renderEventLogs;
window.getLogLevelColor = getLogLevelColor;
window.renderRecentActivity = renderRecentActivity;
window.getActivityIcon = getActivityIcon;
window.getTimeAgo = getTimeAgo;
window.formatLogTime = formatLogTime;
window.loadDashboardMetrics = loadDashboardMetrics;
window.updateDashboardMetrics = updateDashboardMetrics;
window.updateNetworkSaturation = updateNetworkSaturation;
window.loadPublicIP = loadPublicIP;
