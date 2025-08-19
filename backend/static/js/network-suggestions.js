// Network suggestion functionality
window.detectedNetworks = [];
window.currentSuggestionIndex = 0;

function checkForNetworkSuggestions() {
    fetch('/api/detected-networks', { credentials: 'include' })
        .then(response => {
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            return response.json();
        })
        .then(data => {
            window.detectedNetworks = data || [];
            if (window.detectedNetworks.length > 0) {
                showNetworkSuggestion(0);
            } else {
                console.log('No networks detected, hiding suggestion');
                hideNetworkSuggestion();
            }
        })
        .catch(error => {
            console.error('Failed to fetch detected networks:', error);
        });
}

function showNetworkSuggestion(index) {
    if (index >= window.detectedNetworks.length) {
        hideNetworkSuggestion();
        return;
    }
    window.currentSuggestionIndex = index;
    const network = window.detectedNetworks[index];
    const suggestionElement = document.getElementById('network-suggestions');
    const suggestionTextElement = document.getElementById('suggestion-text');
    
    if (suggestionElement && suggestionTextElement) {
        const interfaceText = network.interface_name && network.interface_name !== 'undefined' 
            ? ` (Interface: ${network.interface_name})` 
            : '';
        suggestionTextElement.textContent = `New network detected: ${network.cidr}${interfaceText}. Would you like to add it?`;
        suggestionElement.style.display = 'block';
        
        // Theme will be applied automatically via CSS
    }
}

function hideNetworkSuggestion() {
    const suggestionElement = document.getElementById('network-suggestions');
    if (suggestionElement) {
        suggestionElement.style.display = 'none';
    }
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
            showNetworkSuggestion(Math.min(window.currentSuggestionIndex, window.detectedNetworks.length - 1));
        } else {
            hideNetworkSuggestion();
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
    console.log('Dismissing network suggestion');
    window.detectedNetworks.splice(window.currentSuggestionIndex, 1);
    if (window.detectedNetworks.length > 0) {
        showNetworkSuggestion(Math.min(window.currentSuggestionIndex, window.detectedNetworks.length - 1));
    } else {
        hideNetworkSuggestion();
    }
}

// Initialize network suggestion functionality
function initNetworkSuggestions() {
    console.log('Initializing network suggestions...');
    
    // Add event listeners to buttons
    const createBtn = document.getElementById('create-network-btn');
    const dismissBtn = document.getElementById('dismiss-suggestion-btn');
    
    if (createBtn) {
        createBtn.addEventListener('click', createNetworkFromSuggestion);
    }
    if (dismissBtn) {
        dismissBtn.addEventListener('click', dismissNetworkSuggestion);
    }
    // Check for network suggestions immediately and periodically
    checkForNetworkSuggestions();
    // Clear any existing interval
    if (window.networkSuggestionInterval) {
        clearInterval(window.networkSuggestionInterval);
    }
    window.networkSuggestionInterval = setInterval(checkForNetworkSuggestions, 15000);
}