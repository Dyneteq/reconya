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
        .then(response => response.json())
        .then(data => {
            updateDashboardMetrics(data);
        })
        .catch(error => {
            console.error('Error loading dashboard metrics:', error);
        });
}

function updateDashboardMetrics(data) {
    // Update various dashboard metrics
    const networkRangeEl = document.getElementById('network-range');
    const publicIpEl = document.getElementById('public-ip');
    const devicesFoundEl = document.getElementById('devices-found');
    const devicesOnlineEl = document.getElementById('devices-online');
    
    if (networkRangeEl) networkRangeEl.textContent = data.networkRange || 'N/A';

    // Update public IP with location as tooltip if available
    if (publicIpEl) {
        let ipText = data.publicIP || 'N/A';
        console.log('Dashboard data:', data);
        console.log('Location:', data.location);
        publicIpEl.textContent = ipText;
        if (data.location && data.location !== '') {
            console.log('Setting location tooltip to:', data.location);
            publicIpEl.setAttribute('title', data.location);
            publicIpEl.style.cursor = 'help';
        } else {
            console.log('No location data');
            publicIpEl.removeAttribute('title');
            publicIpEl.style.cursor = 'default';
        }
    }

    if (devicesFoundEl) devicesFoundEl.textContent = data.devicesFound || 0;
    if (devicesOnlineEl) devicesOnlineEl.textContent = data.devicesOnline || 0;
    
    // Calculate and update saturation
    updateNetworkSaturation(data.networkRange, data.devicesOnline || 0);
}

function updateNetworkSaturation(networkRange, devicesOnline) {
    if (networkRange && networkRange.includes('/')) {
        const cidrParts = networkRange.split('/');
        if (cidrParts.length === 2) {
            const cidr = parseInt(cidrParts[1]);
            if (!isNaN(cidr) && cidr >= 8 && cidr <= 30) {
                const totalAddresses = Math.pow(2, 32 - cidr) - 2;
                if (totalAddresses > 0) {
                    const saturation = ((devicesOnline / totalAddresses) * 100).toFixed(1);
                    const saturationNum = parseFloat(saturation);
                    const saturationEl = document.getElementById('network-saturation');
                    const progressEl = document.getElementById('saturation-progress');
                    if (saturationEl) saturationEl.textContent = saturation + '%';
                    if (progressEl) progressEl.style.width = saturationNum + '%';
                    return;
                }
            }
        }
    }
    
    // Reset to 0% if calculation fails
    const saturationEl = document.getElementById('network-saturation');
    const progressEl = document.getElementById('saturation-progress');
    if (saturationEl) saturationEl.textContent = '0%';
    if (progressEl) progressEl.style.width = '0%';
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