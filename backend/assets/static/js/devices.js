// Device functionality
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
    var targetEl = document.getElementById('device-list-container');
    if (targetEl) {
        targetEl.textContent = '';
        var spinner = document.createElement('div');
        spinner.className = 'flex items-center justify-center py-8';
        spinner.textContent = 'Loading devices...';
        targetEl.appendChild(spinner);

        fetch('/api/device-list', { credentials: 'include' })
            .then(function(response) { return response.json(); })
            .then(function(data) {
                targetEl.textContent = '';
                var wrapper = document.createElement('div');
                wrapper.innerHTML = renderDeviceTable(data.devices || [], data.networks || {});
                while (wrapper.firstChild) targetEl.appendChild(wrapper.firstChild);
            })
            .catch(function(error) {
                console.error('Error loading device list:', error);
                targetEl.textContent = 'Failed to load devices';
            });
    }
}

function renderDeviceTable(devices, networkMap) {
    networkMap = networkMap || {};

    if (!devices || devices.length === 0) {
        return '<div class="text-center text-gray-400 py-8">No devices found</div>';
    }

    var rows = devices.map(function(device) {
        var networkCidr = (device.network_id && networkMap[device.network_id]) ? networkMap[device.network_id] : '';
        var openPorts = (device.ports && device.ports.length > 0)
            ? device.ports.filter(function(p) { return p.state === 'open'; }).length
            : 0;

        var nameCell = (device.name || device.hostname)
            ? '<div class="text-gray-300">' + (device.name || device.hostname) + '</div>'
            : '<span class="text-gray-500">-</span>';

        var macCell = device.mac
            ? '<div class="text-blue-400 text-sm">' + device.mac + '</div>'
            : '';

        var portsCell = (device.ports && device.ports.length > 0)
            ? '<div class="text-sm text-gray-400">' + openPorts + ' open</div>'
            : '<span class="text-gray-500">-</span>';

        return '<tr class="cursor-pointer" style="transition: background 0.2s;" onmouseover="this.style.background=\'var(--bg-tertiary)\'" onmouseout="this.style.background=\'\';" onclick="loadDeviceModal(\'' + device.id + '\')">'
            + '<td class="px-4 py-3"><div class="flex items-center"><div class="w-3 h-3 rounded-full mr-3 ' + getStatusColor(device.status) + '"></div><div class="device-ip" style="font-size: 1.1rem;">' + device.ipv4 + '</div></div></td>'
            + '<td class="px-4 py-3">' + nameCell + '</td>'
            + '<td class="px-4 py-3">' + macCell + '</td>'
            + '<td class="px-4 py-3"><span class="text-gray-400 text-sm">' + networkCidr + '</span></td>'
            + '<td class="px-4 py-3">' + portsCell + '</td>'
            + '<td class="px-4 py-3 text-right"><button class="px-2 py-1 text-gray-400 rounded text-sm hover:bg-gray-600 hover:text-white transition-colors" onclick="event.stopPropagation(); deleteDevice(\'' + device.id + '\', \'' + device.ipv4 + '\')" title="Delete"><svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24"><path d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"/></svg></button></td>'
            + '</tr>';
    }).join('');

    return '<div class="rounded overflow-hidden" style="background: var(--bg-secondary);">'
        + '<table class="w-full">'
        + '<thead style="background: var(--bg-primary);">'
        + '<tr>'
        + '<th class="px-4 py-3 text-left text-green-500">IP Address</th>'
        + '<th class="px-4 py-3 text-left text-green-500">Name</th>'
        + '<th class="px-4 py-3 text-left text-green-500">Network Info</th>'
        + '<th class="px-4 py-3 text-left text-green-500">Network</th>'
        + '<th class="px-4 py-3 text-left text-green-500">Ports</th>'
        + '<th class="px-4 py-3"></th>'
        + '</tr>'
        + '</thead>'
        + '<tbody>' + rows + '</tbody>'
        + '</table>'
        + '</div>';
}

// Make functions available globally
window.loadDeviceList = loadDeviceList;
window.renderDeviceTable = renderDeviceTable;
window.loadDeviceModal = loadDeviceModal;
window.renderDeviceModal = renderDeviceModal;
window.getStatusBadgeColor = getStatusBadgeColor;
window.formatLogTime = formatLogTime;
window.saveDeviceChanges = saveDeviceChanges;
window.deleteDevice = deleteDevice;

// ── Node dropdown (topology canvas click) ───────────────────────────────────

function showNodeDropdown(deviceId, clientX, clientY) {
    closeNodeDropdown();

    var tip = document.getElementById('network-viz-tip');
    if (tip) tip.style.display = 'none';

    fetch('/api/devices/' + deviceId + '/modal', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(data) {
            var d = data.device;
            var green  = '#10b981';
            var muted  = '#6b7280';
            var dim    = '#9ca3af';
            var bg     = 'rgba(7,13,23,0.97)';
            var border = 'rgba(16,185,129,0.30)';
            var body   = '#d1fae5';
            var red    = '#ef4444';
            var inputBg = 'rgba(255,255,255,0.05)';

            var openPorts = (d.ports || []).filter(function(p) { return p.state === 'open'; }).length;
            var statusColor = d.status === 'online' ? green
                            : d.status === 'idle'   ? '#eab308'
                            : muted;

            function ago(ts) {
                if (!ts) return '';
                var diff = Date.now() - new Date(ts).getTime();
                var m = Math.floor(diff / 60000);
                if (m < 1)  return 'just now';
                if (m < 60) return m + 'm ago';
                var h = Math.floor(m / 60);
                if (h < 24) return h + 'h ago';
                return Math.floor(h / 24) + 'd ago';
            }

            function makeRow(labelText, valueText, valueColor) {
                var row = document.createElement('div');
                var lbl = document.createElement('span');
                lbl.textContent = labelText;
                lbl.style.cssText = 'font-family:\'Orbitron\',monospace;font-size:8px;color:' + muted + ';';
                var val = document.createElement('span');
                val.textContent = valueText;
                if (valueColor) val.style.color = valueColor;
                row.appendChild(lbl);
                row.appendChild(document.createTextNode(' '));
                row.appendChild(val);
                return row;
            }

            function makeFieldGroup(labelText, el) {
                var wrap = document.createElement('div');
                wrap.style.cssText = 'display:flex;flex-direction:column;gap:3px;';
                var lbl = document.createElement('label');
                lbl.textContent = labelText;
                lbl.style.cssText = 'font-family:\'Orbitron\',monospace;font-size:8px;color:' + muted + ';';
                wrap.appendChild(lbl);
                wrap.appendChild(el);
                return wrap;
            }

            // ── Outer container ──────────────────────────────────────────
            var drop = document.createElement('div');
            drop.id = 'node-dropdown';
            drop.style.cssText =
                'position:fixed;z-index:9999;pointer-events:auto;' +
                'background:' + bg + ';border:1px solid ' + border + ';' +
                'border-radius:6px;padding:10px 12px;width:260px;' +
                'box-shadow:0 8px 32px rgba(0,0,0,0.55);' +
                'color:' + body + ';font-family:\'Roboto Mono\',monospace;font-size:10px;line-height:1.7;';

            // ── Header ───────────────────────────────────────────────────
            var header = document.createElement('div');
            header.style.cssText = 'display:flex;justify-content:space-between;align-items:flex-start;margin-bottom:7px;';

            var headerLeft = document.createElement('div');
            headerLeft.style.cssText = 'min-width:0;flex:1;';

            var ipEl = document.createElement('div');
            ipEl.textContent = d.ipv4 || '—';
            ipEl.style.cssText = 'font-family:\'Orbitron\',monospace;font-size:13px;font-weight:bold;color:' + green + ';letter-spacing:0.05em;';
            headerLeft.appendChild(ipEl);

            var displayName = d.name || d.hostname || '';
            var nameDisplay = document.createElement('div');
            nameDisplay.textContent = displayName || '';
            nameDisplay.style.cssText = 'font-size:10px;color:' + dim + ';margin-top:1px;' + (displayName ? '' : 'display:none;');
            headerLeft.appendChild(nameDisplay);

            var headerRight = document.createElement('div');
            headerRight.style.cssText = 'display:flex;align-items:center;gap:4px;flex-shrink:0;padding-left:6px;';

            var editBtn = document.createElement('button');
            editBtn.title = 'Edit';
            editBtn.style.cssText = 'background:none;border:1px solid ' + border + ';cursor:pointer;color:' + green + ';font-size:11px;line-height:1;padding:2px 5px;border-radius:3px;';
            editBtn.textContent = '✎';

            var closeBtn = document.createElement('button');
            closeBtn.textContent = '✕';
            closeBtn.title = 'Close';
            closeBtn.style.cssText = 'background:none;border:none;cursor:pointer;color:' + muted + ';font-size:15px;line-height:1;padding:0;';
            closeBtn.addEventListener('click', function() { window.closeNodeDropdown(); });

            headerRight.appendChild(editBtn);
            headerRight.appendChild(closeBtn);
            header.appendChild(headerLeft);
            header.appendChild(headerRight);
            drop.appendChild(header);

            // ── View section ─────────────────────────────────────────────
            var viewEl = document.createElement('div');
            viewEl.style.cssText = 'border-top:1px solid ' + border + ';padding-top:7px;display:flex;flex-direction:column;gap:3px;';

            var statusRow = document.createElement('div');
            statusRow.style.cssText = 'display:flex;align-items:center;gap:5px;';
            var dot = document.createElement('span');
            dot.style.cssText = 'width:6px;height:6px;border-radius:50%;background:' + statusColor + ';flex-shrink:0;';
            var statusLbl = document.createElement('span');
            statusLbl.textContent = (d.status || 'unknown').toUpperCase();
            statusLbl.style.cssText = 'font-family:\'Orbitron\',monospace;font-size:9px;color:' + statusColor + ';';
            statusRow.appendChild(dot);
            statusRow.appendChild(statusLbl);
            if (openPorts > 0) {
                var portsSpan = document.createElement('span');
                portsSpan.textContent = '· ' + openPorts + ' open port' + (openPorts > 1 ? 's' : '');
                portsSpan.style.cssText = 'color:' + red + ';margin-left:4px;';
                statusRow.appendChild(portsSpan);
            }
            viewEl.appendChild(statusRow);

            if (d.mac)    viewEl.appendChild(makeRow('MAC',  d.mac));
            if (d.vendor) viewEl.appendChild(makeRow('MFR',  d.vendor, dim));
            if (d.os && d.os.name) {
                var osText = d.os.name + (d.os.version ? ' ' + d.os.version : '');
                viewEl.appendChild(makeRow('OS', osText));
            }
            if (d.last_seen_online_at) viewEl.appendChild(makeRow('SEEN', ago(d.last_seen_online_at)));

            var openPortList = (d.ports || []).filter(function(p) { return p.state === 'open'; });
            if (openPortList.length > 0) {
                var portsSection = document.createElement('div');
                portsSection.style.cssText = 'margin-top:4px;';
                var portsLbl = document.createElement('div');
                portsLbl.textContent = 'PORTS';
                portsLbl.style.cssText = 'font-family:\'Orbitron\',monospace;font-size:8px;color:' + muted + ';margin-bottom:4px;';
                portsSection.appendChild(portsLbl);

                var table = document.createElement('table');
                table.style.cssText = 'width:100%;border-collapse:collapse;font-size:9px;';

                openPortList.forEach(function(p) {
                    var num  = p.number   || '';
                    var proto = p.protocol || 'tcp';
                    var svc  = p.service  || '';

                    var isHttp  = svc === 'http'  || num === '80'   || num === '8080' || num === '8000' || num === '8888';
                    var isHttps = svc === 'https' || num === '443'  || num === '8443';
                    var isWeb   = isHttp || isHttps;
                    var scheme  = isHttps ? 'https' : 'http';

                    var tr = document.createElement('tr');
                    tr.style.cssText = 'border-bottom:1px solid ' + border + ';';

                    // Port + protocol
                    var tdPort = document.createElement('td');
                    tdPort.style.cssText = 'padding:3px 6px 3px 0;white-space:nowrap;';
                    var portNum = document.createElement('span');
                    portNum.textContent = num;
                    portNum.style.cssText = 'color:' + red + ';font-weight:bold;';
                    var portProto = document.createElement('span');
                    portProto.textContent = '/' + proto;
                    portProto.style.cssText = 'color:' + muted + ';';
                    tdPort.appendChild(portNum);
                    tdPort.appendChild(portProto);

                    // Service
                    var tdSvc = document.createElement('td');
                    tdSvc.style.cssText = 'padding:3px 6px;color:' + dim + ';width:100%;';
                    tdSvc.textContent = svc || '—';

                    // Link (web ports only)
                    var tdLink = document.createElement('td');
                    tdLink.style.cssText = 'padding:3px 0 3px 4px;white-space:nowrap;';
                    if (isWeb) {
                        var link = document.createElement('a');
                        link.href = scheme + '://' + (d.ipv4 || '') + ':' + num;
                        link.target = '_blank';
                        link.rel = 'noopener noreferrer';
                        link.textContent = '↗';
                        link.style.cssText = 'color:' + green + ';text-decoration:none;font-size:11px;';
                        tdLink.appendChild(link);
                    }

                    tr.appendChild(tdPort);
                    tr.appendChild(tdSvc);
                    tr.appendChild(tdLink);
                    table.appendChild(tr);
                });

                portsSection.appendChild(table);
                viewEl.appendChild(portsSection);
            }

            if (d.comment) {
                var commentRow = document.createElement('div');
                commentRow.style.cssText = 'margin-top:2px;';
                var commentLbl = document.createElement('span');
                commentLbl.textContent = 'NOTE';
                commentLbl.style.cssText = 'font-family:\'Orbitron\',monospace;font-size:8px;color:' + muted + ';display:block;';
                var commentVal = document.createElement('div');
                commentVal.textContent = d.comment;
                commentVal.style.cssText = 'color:' + dim + ';font-size:9px;line-height:1.4;margin-top:1px;white-space:pre-wrap;word-break:break-word;';
                commentRow.appendChild(commentLbl);
                commentRow.appendChild(commentVal);
                viewEl.appendChild(commentRow);
            }

            drop.appendChild(viewEl);

            // ── Edit section (hidden by default) ─────────────────────────
            var editEl = document.createElement('div');
            editEl.style.cssText = 'display:none;border-top:1px solid ' + border + ';padding-top:8px;display:none;flex-direction:column;gap:8px;';

            var nameInput = document.createElement('input');
            nameInput.type = 'text';
            nameInput.value = d.name || '';
            nameInput.placeholder = 'Device name';
            nameInput.style.cssText =
                'width:100%;box-sizing:border-box;padding:4px 7px;border-radius:3px;' +
                'border:1px solid ' + border + ';background:' + inputBg + ';' +
                'color:' + body + ';font-family:\'Roboto Mono\',monospace;font-size:10px;outline:none;';
            editEl.appendChild(makeFieldGroup('NAME', nameInput));

            var commentInput = document.createElement('textarea');
            commentInput.value = d.comment || '';
            commentInput.placeholder = 'Add a note…';
            commentInput.rows = 3;
            commentInput.style.cssText =
                'width:100%;box-sizing:border-box;padding:4px 7px;border-radius:3px;' +
                'border:1px solid ' + border + ';background:' + inputBg + ';' +
                'color:' + body + ';font-family:\'Roboto Mono\',monospace;font-size:10px;' +
                'resize:vertical;outline:none;min-height:54px;';
            editEl.appendChild(makeFieldGroup('NOTE', commentInput));

            // Save / Cancel row
            var formActions = document.createElement('div');
            formActions.style.cssText = 'display:flex;gap:6px;justify-content:flex-end;';

            var cancelBtn = document.createElement('button');
            cancelBtn.textContent = 'Cancel';
            cancelBtn.style.cssText =
                'padding:3px 10px;border-radius:3px;border:1px solid ' + border + ';' +
                'background:none;color:' + muted + ';font-family:\'Roboto Mono\',monospace;font-size:10px;cursor:pointer;';

            var saveBtn = document.createElement('button');
            saveBtn.textContent = 'Save';
            saveBtn.style.cssText =
                'padding:3px 10px;border-radius:3px;border:1px solid ' + green + ';' +
                'background:' + green + ';color:#fff;font-family:\'Roboto Mono\',monospace;font-size:10px;cursor:pointer;';

            formActions.appendChild(cancelBtn);
            formActions.appendChild(saveBtn);
            editEl.appendChild(formActions);
            drop.appendChild(editEl);

            // ── Toggle edit mode ─────────────────────────────────────────
            function enterEdit() {
                viewEl.style.display = 'none';
                editEl.style.display = 'flex';
                editBtn.textContent = '✕';
                editBtn.title = 'Cancel edit';
                nameInput.focus();
                document.removeEventListener('click', _nodeDropdownOutside);
            }

            function exitEdit() {
                editEl.style.display = 'none';
                viewEl.style.display = 'flex';
                editBtn.textContent = '✎';
                editBtn.title = 'Edit';
                setTimeout(function() {
                    document.addEventListener('click', _nodeDropdownOutside);
                }, 0);
            }

            editBtn.addEventListener('click', function() {
                if (editEl.style.display === 'none') enterEdit(); else exitEdit();
            });
            cancelBtn.addEventListener('click', function() {
                nameInput.value    = d.name    || '';
                commentInput.value = d.comment || '';
                exitEdit();
            });

            saveBtn.addEventListener('click', function() {
                saveBtn.textContent = '…';
                saveBtn.disabled = true;
                fetch('/api/devices/' + deviceId, {
                    method: 'PUT',
                    credentials: 'include',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ name: nameInput.value.trim(), comment: commentInput.value.trim() })
                })
                .then(function(r) {
                    if (!r.ok) throw new Error('HTTP ' + r.status);
                    return r.text();
                })
                .then(function() {
                    d.name    = nameInput.value.trim();
                    d.comment = commentInput.value.trim();
                    nameDisplay.textContent = d.name || d.hostname || '';
                    nameDisplay.style.display = (d.name || d.hostname) ? '' : 'none';
                    exitEdit();
                })
                .catch(function() {
                    saveBtn.textContent = 'Save';
                    saveBtn.disabled = false;
                });
            });

            document.body.appendChild(drop);

            // ── Position near cursor ─────────────────────────────────────
            var dw = 260, dh = drop.offsetHeight || 130;
            var vw = window.innerWidth, vh = window.innerHeight;
            var left = clientX + 14;
            var top  = clientY + 14;
            if (left + dw > vw - 8) left = clientX - dw - 14;
            if (top  + dh > vh - 8) top  = clientY - dh - 14;
            drop.style.left = Math.max(4, left) + 'px';
            drop.style.top  = Math.max(4, top)  + 'px';

            setTimeout(function() {
                document.addEventListener('click', _nodeDropdownOutside);
            }, 0);
        })
        .catch(function() {});
}

function _nodeDropdownOutside(e) {
    var drop = document.getElementById('node-dropdown');
    if (drop && !drop.contains(e.target)) {
        window.closeNodeDropdown();
    }
}

function closeNodeDropdown() {
    var drop = document.getElementById('node-dropdown');
    if (drop) drop.remove();
    document.removeEventListener('click', _nodeDropdownOutside);
}

document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') closeNodeDropdown();
});

window.showNodeDropdown  = showNodeDropdown;
window.closeNodeDropdown = closeNodeDropdown;