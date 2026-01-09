// Device modal functionality
function loadDeviceModal(deviceId) {
    fetch('/api/devices/' + deviceId + '/modal', { credentials: 'include' })
        .then(function(response) { return response.json(); })
        .then(function(data) {
            var modalContent = document.getElementById('device-modal-content');
            if (modalContent) {
                modalContent.textContent = '';
                renderDeviceModal(modalContent, data.device);
                showModal('deviceModal');
            }
        })
        .catch(function(error) { console.error('Error loading device modal:', error); });
}

function createSvgIcon(pathD) {
    var svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('class', 'w-6 h-6');
    svg.setAttribute('fill', 'none');
    svg.setAttribute('stroke', 'currentColor');
    svg.setAttribute('viewBox', '0 0 24 24');
    var path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    path.setAttribute('stroke-linecap', 'round');
    path.setAttribute('stroke-linejoin', 'round');
    path.setAttribute('stroke-width', '2');
    path.setAttribute('d', pathD);
    svg.appendChild(path);
    return svg;
}

function renderDeviceModal(container, device) {
    var wrapper = document.createElement('div');
    wrapper.className = 'p-6';

    // Header
    var header = document.createElement('div');
    header.className = 'flex justify-between items-center mb-4 pb-3';
    header.style.borderBottom = '1px solid var(--border-color)';

    var headerLeft = document.createElement('div');
    headerLeft.className = 'flex items-center';

    var statusDot = document.createElement('div');
    statusDot.className = 'w-4 h-4 rounded-full mr-3 ' + (device.status === 'online' ? 'bg-green-500' : device.status === 'offline' ? 'bg-red-500' : 'bg-gray-500');
    headerLeft.appendChild(statusDot);

    var ipText = document.createElement('h3');
    ipText.className = 'text-xl font-mono font-bold';
    ipText.style.color = 'var(--text-primary)';
    ipText.textContent = device.ipv4;
    headerLeft.appendChild(ipText);

    if (device.name || device.hostname) {
        var nameText = document.createElement('span');
        nameText.className = 'text-lg ml-3';
        nameText.style.color = 'var(--text-secondary)';
        nameText.textContent = '- ' + (device.name || device.hostname);
        headerLeft.appendChild(nameText);
    }

    header.appendChild(headerLeft);

    var closeBtn = document.createElement('button');
    closeBtn.className = 'text-xl transition-colors';
    closeBtn.style.color = 'var(--text-muted)';
    closeBtn.appendChild(createSvgIcon('M6 18L18 6M6 6l12 12'));
    closeBtn.onclick = function() { closeModal('deviceModal'); };
    header.appendChild(closeBtn);

    wrapper.appendChild(header);

    // Device info grid
    var grid = document.createElement('div');
    grid.className = 'grid grid-cols-1 md:grid-cols-2 gap-6 mb-6';

    // Left column - Device Info
    var leftCol = document.createElement('div');
    var leftTitle = document.createElement('h4');
    leftTitle.className = 'text-green-500 font-semibold mb-3';
    leftTitle.textContent = 'Device Info';
    leftCol.appendChild(leftTitle);

    var infoList = document.createElement('div');
    infoList.className = 'space-y-2 text-sm';

    function addInfoRow(label, value, valueClass) {
        var row = document.createElement('div');
        var labelSpan = document.createElement('span');
        labelSpan.style.color = 'var(--text-muted)';
        labelSpan.textContent = label + ': ';
        row.appendChild(labelSpan);
        var valueSpan = document.createElement('span');
        valueSpan.className = valueClass || '';
        valueSpan.style.color = valueClass ? '' : 'var(--text-primary)';
        valueSpan.textContent = value;
        row.appendChild(valueSpan);
        infoList.appendChild(row);
    }

    addInfoRow('IP Address', device.ipv4, 'font-mono');
    if (device.mac) addInfoRow('MAC Address', device.mac, 'text-blue-400 font-mono');
    if (device.hostname) addInfoRow('Hostname', device.hostname);
    addInfoRow('Status', device.status);
    if (device.LastSeenOnlineAt) addInfoRow('Last Seen', new Date(device.LastSeenOnlineAt).toLocaleString());

    leftCol.appendChild(infoList);
    grid.appendChild(leftCol);

    // Right column - OS Info
    if (device.os && device.os.name) {
        var rightCol = document.createElement('div');
        var rightTitle = document.createElement('h4');
        rightTitle.className = 'text-green-500 font-semibold mb-3';
        rightTitle.textContent = 'Operating System';
        rightCol.appendChild(rightTitle);

        var osInfo = document.createElement('div');
        osInfo.className = 'space-y-2 text-sm';

        var osRow = document.createElement('div');
        var osLabel = document.createElement('span');
        osLabel.style.color = 'var(--text-muted)';
        osLabel.textContent = 'OS: ';
        osRow.appendChild(osLabel);
        var osValue = document.createElement('span');
        osValue.style.color = 'var(--text-primary)';
        osValue.textContent = device.os.name;
        osRow.appendChild(osValue);
        osInfo.appendChild(osRow);

        rightCol.appendChild(osInfo);
        grid.appendChild(rightCol);
    }

    wrapper.appendChild(grid);

    // Ports
    if (device.ports && device.ports.length > 0) {
        var openPorts = device.ports.filter(function(p) { return p.state === 'open' || p.state === 'filtered'; });
        if (openPorts.length > 0) {
            var portsSection = document.createElement('div');
            portsSection.className = 'mb-6';

            var portsTitle = document.createElement('h4');
            portsTitle.className = 'text-green-500 font-semibold mb-3';
            portsTitle.textContent = 'Open Ports';
            portsSection.appendChild(portsTitle);

            var portsBox = document.createElement('div');
            portsBox.className = 'rounded p-3 max-h-32 overflow-y-auto';
            portsBox.style.background = 'var(--bg-tertiary)';
            portsBox.style.border = '1px solid var(--border-color)';

            openPorts.forEach(function(port) {
                var portRow = document.createElement('div');
                portRow.className = 'flex items-center justify-between text-xs py-1';

                var portInfo = document.createElement('div');
                portInfo.className = 'flex items-center space-x-2';

                var portNum = document.createElement('span');
                portNum.className = 'text-green-400 font-medium';
                portNum.textContent = (port.number || port.Port) + '/' + (port.protocol || port.Protocol);
                portInfo.appendChild(portNum);

                var portService = document.createElement('span');
                portService.style.color = 'var(--text-secondary)';
                portService.textContent = port.service || port.Service || 'unknown';
                portInfo.appendChild(portService);

                portRow.appendChild(portInfo);

                var stateSpan = document.createElement('span');
                stateSpan.className = 'text-xs font-bold uppercase ' + (port.state === 'open' ? 'text-red-500' : 'text-yellow-500');
                stateSpan.textContent = port.state;
                portRow.appendChild(stateSpan);

                portsBox.appendChild(portRow);
            });

            portsSection.appendChild(portsBox);
            wrapper.appendChild(portsSection);
        }
    }

    // Footer
    var footer = document.createElement('div');
    footer.className = 'flex justify-end pt-4';
    footer.style.borderTop = '1px solid var(--border-color)';

    var closeButton = document.createElement('button');
    closeButton.className = 'px-4 py-2 rounded transition-colors';
    closeButton.style.color = 'var(--text-muted)';
    closeButton.style.border = '1px solid var(--border-color)';
    closeButton.textContent = 'Close';
    closeButton.onclick = function() { closeModal('deviceModal'); };
    footer.appendChild(closeButton);

    wrapper.appendChild(footer);
    container.appendChild(wrapper);
}

function deleteDevice(deviceId, deviceIP) {
    if (confirm('Delete device ' + deviceIP + '?')) {
        fetch('/api/devices/' + deviceId, { method: 'DELETE', credentials: 'include' })
            .then(function(response) {
                if (response.ok && typeof loadDeviceTable === 'function') {
                    loadDeviceTable();
                }
            })
            .catch(function(error) { console.error('Error deleting device:', error); });
    }
}

window.loadDeviceModal = loadDeviceModal;
window.deleteDevice = deleteDevice;
