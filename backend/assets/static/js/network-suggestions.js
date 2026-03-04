window.detectedNetworks = [];
window.currentSuggestionIndex = 0;

const DISMISSED_NETWORKS_KEY = 'reconya_dismissed_networks';

function getDismissedNetworks() {
    try {
        const dismissed = localStorage.getItem(DISMISSED_NETWORKS_KEY);
        return dismissed ? JSON.parse(dismissed) : [];
    } catch (e) {
        console.error('Failed to parse dismissed networks from localStorage:', e);
        return [];
    }
}

function addDismissedNetwork(cidr) {
    try {
        const dismissed = getDismissedNetworks();
        if (!dismissed.includes(cidr)) {
            dismissed.push(cidr);
            localStorage.setItem(DISMISSED_NETWORKS_KEY, JSON.stringify(dismissed));
        }
    } catch (e) {
        console.error('Failed to save dismissed network to localStorage:', e);
    }
}

function isNetworkDismissed(cidr) {
    const dismissed = getDismissedNetworks();
    return dismissed.includes(cidr);
}

function checkForNetworkSuggestions() {
    Promise.all([
        fetch('/api/detected-networks', { credentials: 'include' }).then(r => r.ok ? r.json() : []),
        fetch('/api/networks', { credentials: 'include' }).then(r => r.ok ? r.json() : {networks: []})
    ]).then(([detectedData, networksData]) => {
        const existingNetworks = networksData.networks || [];

        // When there are 0 configured networks, clear any previously dismissed networks
        // so the user always sees the prompt on a fresh start
        if (existingNetworks.length === 0) {
            localStorage.removeItem(DISMISSED_NETWORKS_KEY);
        }

        window.detectedNetworks = (detectedData || []).filter(network => !isNetworkDismissed(network.cidr));

        if (window.detectedNetworks.length > 0) {
            showNetworkAlert(0);
        } else {
            hideNetworkAlert();
        }
    }).catch(error => {
        console.error('Failed to fetch network data:', error);
        hideNetworkAlert();
    });
}

function showNetworkAlert(index) {
    if (index >= window.detectedNetworks.length) {
        hideNetworkAlert();
        return;
    }
    window.currentSuggestionIndex = index;
    const network = window.detectedNetworks[index];

    const alertElement = document.getElementById('no-networks-alert');
    const alertText = document.getElementById('no-networks-alert-text');
    const addBtn = document.getElementById('no-networks-add-btn');

    if (!alertElement || !alertText) return;

    const interfaceText = network.interface_name && network.interface_name !== 'undefined'
        ? ` on interface ${network.interface_name}`
        : '';

    alertText.textContent = `Network ${network.cidr} detected${interfaceText}. Add it to start scanning for devices.`;

    if (addBtn) {
        addBtn.textContent = 'Add Network';
    }

    alertElement.classList.remove('hidden');
}

function hideNetworkAlert() {
    const alertElement = document.getElementById('no-networks-alert');
    if (alertElement) {
        alertElement.classList.add('hidden');
    }
}

function checkIfNetworkExists(cidr) {
    return fetch('/api/networks', { credentials: 'include' })
        .then(response => response.json())
        .then(data => {
            const networks = data.networks || [];
            return networks.some(network => network.cidr === cidr);
        })
        .catch(error => {
            console.error('Failed to check existing networks:', error);
            return false;
        });
}

function createNetworkFromSuggestion() {
    if (!window.detectedNetworks || window.currentSuggestionIndex >= window.detectedNetworks.length) {
        return;
    }
    const network = window.detectedNetworks[window.currentSuggestionIndex];
    fetch('/api/network-suggestion', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: new URLSearchParams({
            'cidr': network.cidr,
            'interface_name': network.interface_name,
            'gateway': network.gateway || '',
            'name': network.name || ''
        }),
        credentials: 'include'
    })
    .then(response => {
        if (!response.ok) {
            return response.text().then(text => {
                throw new Error(`HTTP ${response.status}: ${text}`);
            });
        }
        return response.json();
    })
    .then(data => {
        // Remove the accepted suggestion from the array
        window.detectedNetworks.splice(window.currentSuggestionIndex, 1);
        if (window.detectedNetworks.length > 0) {
            showNetworkAlert(Math.min(window.currentSuggestionIndex, window.detectedNetworks.length - 1));
        } else {
            hideNetworkAlert();
        }
        // Refresh network list and scan control
        if (typeof window.loadNetworkList === 'function') {
            window.loadNetworkList();
        }
        if (typeof window.loadScanControl === 'function') {
            window.loadScanControl();
        }
    })
    .catch(error => {
        console.error('Failed to create network from suggestion:', error);
        alert('Failed to create network: ' + error.message);
    });
}

function dismissNetworkSuggestion() {
    if (window.detectedNetworks && window.detectedNetworks.length > 0 && window.currentSuggestionIndex < window.detectedNetworks.length) {
        const networkCidr = window.detectedNetworks[window.currentSuggestionIndex].cidr;
        addDismissedNetwork(networkCidr);
        window.detectedNetworks.splice(window.currentSuggestionIndex, 1);
    }

    if (window.detectedNetworks && window.detectedNetworks.length > 0) {
        showNetworkAlert(Math.min(window.currentSuggestionIndex, window.detectedNetworks.length - 1));
    } else {
        hideNetworkAlert();
    }
}

// Initialize network suggestion functionality
function initNetworkSuggestions() {
    const alertElement = document.getElementById('no-networks-alert');
    const addBtn = document.getElementById('no-networks-add-btn');
    const dismissBtn = document.getElementById('no-networks-dismiss-btn');

    if (!alertElement) {
        return;
    }

    if (addBtn) {
        addBtn.addEventListener('click', createNetworkFromSuggestion);
    }
    if (dismissBtn) {
        dismissBtn.addEventListener('click', dismissNetworkSuggestion);
    }

    // Check for network suggestions immediately and periodically
    checkForNetworkSuggestions();

    if (window.networkSuggestionInterval) {
        clearInterval(window.networkSuggestionInterval);
    }
    window.networkSuggestionInterval = setInterval(checkForNetworkSuggestions, 15000);
}
