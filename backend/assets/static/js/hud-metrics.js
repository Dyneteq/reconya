// Dashboard HUD tile updates (network range, public IP, found/online counts,
// saturation). Split out of the old dashboard.js, which otherwise rendered
// only into containers that are permanently display:none.
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
    const networkRangeEl = document.getElementById('network-range');
    const publicIpEl = document.getElementById('public-ip');
    const devicesFoundEl = document.getElementById('devices-found');
    const devicesOnlineEl = document.getElementById('devices-online');

    if (networkRangeEl) networkRangeEl.textContent = data.networkRange || 'N/A';

    // Update public IP with location as tooltip if available
    if (publicIpEl) {
        // Empty when public IP lookup is disabled (the default) — match the
        // template's own placeholder rather than showing a bare "N/A".
        publicIpEl.textContent = data.publicIP || '—';
        if (data.location) {
            publicIpEl.setAttribute('title', data.location);
            publicIpEl.style.cursor = 'help';
        } else {
            publicIpEl.removeAttribute('title');
            publicIpEl.style.cursor = 'default';
        }
    }

    if (devicesFoundEl) devicesFoundEl.textContent = data.devicesFound || 0;
    if (devicesOnlineEl) devicesOnlineEl.textContent = data.devicesOnline || 0;

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

window.loadDashboardMetrics = loadDashboardMetrics;
window.updateDashboardMetrics = updateDashboardMetrics;
window.updateNetworkSaturation = updateNetworkSaturation;
