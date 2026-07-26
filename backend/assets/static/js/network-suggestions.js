window.detectedNetworks = [];

function checkForNetworkSuggestions() {
    Promise.all([
        fetch('/api/detected-networks', { credentials: 'include' }).then(r => r.ok ? r.json() : []),
        fetch('/api/networks', { credentials: 'include' }).then(r => r.ok ? r.json() : {networks: []})
    ]).then(([detectedData, networksData]) => {
        const existingNetworks = networksData.networks || [];
        const detected = detectedData || [];

        window.detectedNetworks = detected;

        // Auto-add any detected networks not yet in the database
        const toAdd = detected.filter(n => !existingNetworks.some(e => e.cidr === n.cidr));
        if (toAdd.length > 0) {
            autoAddDetectedNetworks(toAdd);
        }
    }).catch(error => {
        console.error('Failed to fetch network data:', error);
    });
}

function autoAddDetectedNetworks(networks) {
    let index = 0;
    function addNext() {
        if (index >= networks.length) {
            // All networks added — reload scan control to trigger auto-start
            if (typeof window.loadScanControl === 'function') {
                window.loadScanControl(true);
            }
            return;
        }
        const network = networks[index++];
        fetch('/api/network-suggestion', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: new URLSearchParams({
                cidr: network.cidr,
                interface_name: network.interface_name || '',
                gateway: network.gateway || '',
                name: network.name || ''
            }),
            credentials: 'include'
        })
        .then(r => {
            if (r.ok) {
                addNext();
            } else {
                console.error('Failed to auto-add network:', network.cidr);
            }
        })
        .catch(e => console.error('Error auto-adding network:', e));
    }
    addNext();
}

// Initialize network suggestion functionality
function initNetworkSuggestions() {
    // Check for detected networks immediately and periodically
    checkForNetworkSuggestions();

    if (window.networkSuggestionInterval) {
        clearInterval(window.networkSuggestionInterval);
    }
    window.networkSuggestionInterval = setInterval(checkForNetworkSuggestions, 15000);
}
