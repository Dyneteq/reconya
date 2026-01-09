// Device modal - cyberpunk style
function loadDeviceModal(deviceId) {
    fetch('/api/devices/' + deviceId + '/modal', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(data) {
            var content = document.getElementById('device-modal-content');
            if (content) {
                content.textContent = '';
                renderDeviceModal(content, data.device);
                showModal('deviceModal');
            }
        })
        .catch(function(e) { console.error(e); });
}

function renderDeviceModal(container, d) {
    var wrap = document.createElement('div');
    wrap.style.cssText = 'padding: 20px; font-family: "Share Tech Mono", monospace; color: #0f0;';

    // Header
    var header = document.createElement('div');
    header.style.cssText = 'display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; padding-bottom: 15px; border-bottom: 1px solid #0f03;';

    var title = document.createElement('div');
    title.style.cssText = 'display: flex; align-items: center; gap: 12px;';

    var dot = document.createElement('span');
    dot.style.cssText = 'width: 12px; height: 12px; border-radius: 50%; ' +
        (d.status === 'online' ? 'background: #0f0; box-shadow: 0 0 10px #0f0;' : 'background: #f00; box-shadow: 0 0 10px #f00;');
    title.appendChild(dot);

    var ip = document.createElement('span');
    ip.style.cssText = 'font-size: 1.4rem; font-weight: bold; text-shadow: 0 0 10px #0f0;';
    ip.textContent = d.ipv4;
    title.appendChild(ip);

    header.appendChild(title);

    var closeBtn = document.createElement('button');
    closeBtn.style.cssText = 'background: transparent; border: 1px solid #0f0; color: #0f0; padding: 6px 12px; cursor: pointer; font-family: inherit;';
    closeBtn.textContent = '[X] CLOSE';
    closeBtn.onclick = function() { closeModal('deviceModal'); };
    closeBtn.onmouseover = function() { this.style.background = '#0f0'; this.style.color = '#000'; };
    closeBtn.onmouseout = function() { this.style.background = 'transparent'; this.style.color = '#0f0'; };
    header.appendChild(closeBtn);

    wrap.appendChild(header);

    // Info section
    var info = document.createElement('div');
    info.style.cssText = 'margin-bottom: 20px;';

    var infoTitle = document.createElement('div');
    infoTitle.style.cssText = 'font-size: 11px; text-transform: uppercase; letter-spacing: 2px; margin-bottom: 12px; color: #0f0;';
    infoTitle.textContent = '> TARGET_INFO';
    info.appendChild(infoTitle);

    var grid = document.createElement('div');
    grid.style.cssText = 'display: grid; grid-template-columns: 1fr 1fr; gap: 8px; font-size: 13px;';

    function addRow(label, value, color) {
        var row = document.createElement('div');
        var l = document.createElement('span');
        l.style.color = '#060';
        l.textContent = label + ': ';
        row.appendChild(l);
        var v = document.createElement('span');
        v.style.color = color || '#0f0';
        v.textContent = value || '-';
        row.appendChild(v);
        grid.appendChild(row);
    }

    addRow('IP_ADDR', d.ipv4);
    addRow('STATUS', d.status ? d.status.toUpperCase() : '-');
    addRow('MAC_ADDR', d.mac);
    addRow('VENDOR', d.vendor);
    addRow('HOSTNAME', d.hostname || d.name);
    if (d.os && d.os.name) addRow('OS', d.os.name);
    if (d.LastSeenOnlineAt) addRow('LAST_SEEN', new Date(d.LastSeenOnlineAt).toLocaleString());

    info.appendChild(grid);
    wrap.appendChild(info);

    // Ports section
    if (d.ports && d.ports.length > 0) {
        var openPorts = d.ports.filter(function(p) { return p.state === 'open' || p.state === 'filtered'; });
        if (openPorts.length > 0) {
            var ports = document.createElement('div');

            var portsTitle = document.createElement('div');
            portsTitle.style.cssText = 'font-size: 11px; text-transform: uppercase; letter-spacing: 2px; margin-bottom: 12px; color: #f00;';
            portsTitle.textContent = '> OPEN_PORTS [' + openPorts.length + ']';
            ports.appendChild(portsTitle);

            var portsList = document.createElement('div');
            portsList.style.cssText = 'background: rgba(255,0,0,0.05); border: 1px solid #f003; padding: 12px; max-height: 150px; overflow-y: auto;';

            openPorts.forEach(function(p) {
                var row = document.createElement('div');
                row.style.cssText = 'display: flex; justify-content: space-between; padding: 4px 0; font-size: 12px;';

                var left = document.createElement('span');
                left.style.color = '#f00';
                left.textContent = (p.number || p.Port) + '/' + (p.protocol || p.Protocol) + ' ' + (p.service || p.Service || '');
                row.appendChild(left);

                var right = document.createElement('span');
                right.style.cssText = 'text-transform: uppercase; font-size: 10px; color: ' + (p.state === 'open' ? '#f00' : '#ff0');
                right.textContent = p.state;
                row.appendChild(right);

                portsList.appendChild(row);
            });

            ports.appendChild(portsList);
            wrap.appendChild(ports);
        }
    }

    container.appendChild(wrap);
}

function deleteDevice(deviceId, deviceIP) {
    if (confirm('DELETE TARGET ' + deviceIP + '?')) {
        fetch('/api/devices/' + deviceId, { method: 'DELETE', credentials: 'include' })
            .then(function(r) { if (r.ok && typeof loadDeviceTable === 'function') loadDeviceTable(); })
            .catch(function(e) { console.error(e); });
    }
}

window.loadDeviceModal = loadDeviceModal;
window.deleteDevice = deleteDevice;
