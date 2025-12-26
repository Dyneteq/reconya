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

        window.detectedNetworks = (detectedData || []).filter(network => !isNetworkDismissed(network.cidr));
        const existingNetworks = networksData.networks || [];

        if (window.detectedNetworks.length > 0) {
            showNetworkSuggestion(0);
        } else {
            // No new networks, but check if we have existing networks that were detected
            // by checking the server logs pattern (192.168.2.12/24 -> 192.168.2.0/24)
            const detectedButExisting = findDetectedExistingNetwork(existingNetworks);
            if (detectedButExisting) {
                showDetectedExistingNetwork(detectedButExisting);
            } else {
                // Only show "no networks detected" if there are no dismissed networks
                const dismissedNetworks = getDismissedNetworks();
                if (dismissedNetworks.length === 0) {
                    showNoNetworksDetected();
                } else {
                    hideNetworkSuggestion();
                }
            }
        }
    }).catch(error => {
        console.error('Failed to fetch network data:', error);
        showNetworkError();
    });
}

// Helper function to find a network that matches the pattern from server logs
function findDetectedExistingNetwork(existingNetworks) {
    // First try to get actual detected networks from server logs
    // Check if we have any existing networks that match typical auto-detected patterns
    // Priority order: match actual detected network first (192.168.2.0/24 based on ifconfig)
    const detectionPatterns = [
        '192.168.2.0/24', '192.168.1.0/24', '192.168.0.0/24', 
        '10.0.0.0/24', '172.16.0.0/24', '192.168.0.0/16'
    ];
    
    // Look for networks that exist and match detection patterns, but skip dismissed ones
    for (let pattern of detectionPatterns) {
        if (isNetworkDismissed(pattern)) {
            continue;
        }
        const found = existingNetworks.find(net => net.cidr === pattern);
        if (found) return found;
    }
    return null;
}

function showDetectedExistingNetwork(network) {
    const suggestionElement = document.getElementById('network-suggestions');
    const suggestionTextElement = document.getElementById('suggestion-text');
    const actionButtons = document.getElementById('network-action-buttons');
    const createBtn = document.getElementById('create-network-btn');
    const dismissBtn = document.getElementById('dismiss-suggestion-btn');
    
    if (suggestionElement && suggestionTextElement) {
        suggestionTextElement.textContent = `Network ${network.cidr} detected and already configured. ✓`;
        showSuggestionElement(); // Use helper function to restore display
        
        // Show action buttons inline with text for existing network
        if (actionButtons) actionButtons.style.display = 'flex';
        if (createBtn) createBtn.style.display = 'none';
        if (dismissBtn) {
            dismissBtn.textContent = 'OK';
            dismissBtn.style.display = 'block';
        }
    }
}

// Helper function to show the suggestion element properly
function showSuggestionElement() {
    const suggestionElement = document.getElementById('network-suggestions');
    if (suggestionElement) {
        suggestionElement.style.display = 'block';
        // Restore original styling
        suggestionElement.style.margin = '';
        suggestionElement.style.padding = '';
        suggestionElement.style.height = '';
        suggestionElement.style.overflow = '';
    }
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
    const createBtn = document.getElementById('create-network-btn');
    const dismissBtn = document.getElementById('dismiss-suggestion-btn');
    
    if (suggestionElement && suggestionTextElement) {
        const interfaceText = network.interface_name && network.interface_name !== 'undefined' 
            ? ` (Interface: ${network.interface_name})` 
            : '';
        
        // Check if network already exists
        checkIfNetworkExists(network.cidr).then(exists => {
            const actionButtons = document.getElementById('network-action-buttons');
            
            if (exists) {
                suggestionTextElement.textContent = `Network ${network.cidr}${interfaceText} detected and already configured.`;
                if (actionButtons) actionButtons.style.display = 'flex';
                if (createBtn) createBtn.style.display = 'none';
                if (dismissBtn) {
                    dismissBtn.textContent = 'OK';
                    dismissBtn.style.display = 'block';
                }
            } else {
                suggestionTextElement.textContent = `New network detected: ${network.cidr}${interfaceText}. Would you like to add it?`;
                if (actionButtons) actionButtons.style.display = 'flex';
                if (createBtn) {
                    createBtn.style.display = 'block';
                    createBtn.textContent = 'Add Network';
                }
                if (dismissBtn) {
                    dismissBtn.textContent = 'Dismiss';
                    dismissBtn.style.display = 'block';
                }
            }
        });
        
        showSuggestionElement(); // Use helper function to restore display
    }
}

function hideNetworkSuggestion() {
    const suggestionElement = document.getElementById('network-suggestions');
    if (suggestionElement) {
        suggestionElement.style.display = 'none';
        // Remove any margin/padding that might leave space
        suggestionElement.style.margin = '0';
        suggestionElement.style.padding = '0';
        suggestionElement.style.height = '0';
        suggestionElement.style.overflow = 'hidden';
    }
}

function showNoNetworksDetected() {
    const suggestionElement = document.getElementById('network-suggestions');
    const suggestionTextElement = document.getElementById('suggestion-text');
    const actionButtons = document.getElementById('network-action-buttons');
    
    if (suggestionElement && suggestionTextElement) {
        suggestionTextElement.textContent = 'No active networks detected. You may need to manually configure your networks.';
        showSuggestionElement(); // Use helper function to restore display
        
        // Hide all action buttons since there's nothing to add
        if (actionButtons) actionButtons.style.display = 'none';
    }
}

function showNetworkError() {
    const suggestionElement = document.getElementById('network-suggestions');
    const suggestionTextElement = document.getElementById('suggestion-text');
    const actionButtons = document.getElementById('network-action-buttons');
    
    if (suggestionElement && suggestionTextElement) {
        suggestionTextElement.textContent = 'Failed to detect networks. Please check your network configuration.';
        showSuggestionElement(); // Use helper function to restore display
        
        // Hide action buttons on error
        if (actionButtons) actionButtons.style.display = 'none';
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
    
    // Get the current network CIDR to dismiss
    let networkCidr = null;
    
    if (window.detectedNetworks && window.detectedNetworks.length > 0 && window.currentSuggestionIndex < window.detectedNetworks.length) {
        // For new detected networks
        networkCidr = window.detectedNetworks[window.currentSuggestionIndex].cidr;
        window.detectedNetworks.splice(window.currentSuggestionIndex, 1);
    } else {
        // For existing network suggestions, try to extract CIDR from the text
        const suggestionText = document.getElementById('suggestion-text');
        if (suggestionText) {
            const text = suggestionText.textContent;
            const cidrMatch = text.match(/Network (\S+)/);
            if (cidrMatch) {
                networkCidr = cidrMatch[1];
            }
        }
    }
    
    // Add to dismissed list if we found a CIDR
    if (networkCidr) {
        addDismissedNetwork(networkCidr);
    }
    
    // Continue with next suggestion or hide
    if (window.detectedNetworks && window.detectedNetworks.length > 0) {
        showNetworkSuggestion(Math.min(window.currentSuggestionIndex, window.detectedNetworks.length - 1));
    } else {
        hideNetworkSuggestion();
    }
}

// Initialize network suggestion functionality
function initNetworkSuggestions() {
    // Check if elements exist
    const suggestionElement = document.getElementById('network-suggestions');
    const createBtn = document.getElementById('create-network-btn');
    const dismissBtn = document.getElementById('dismiss-suggestion-btn');
    const closeBtn = document.getElementById('close-suggestions-btn');
    const suggestionText = document.getElementById('suggestion-text');
    
    if (!suggestionElement) {
        return;
    }
    
    // Add event listeners to buttons
    if (createBtn) {
        createBtn.addEventListener('click', createNetworkFromSuggestion);
    }
    if (dismissBtn) {
        dismissBtn.addEventListener('click', dismissNetworkSuggestion);
    }
    if (closeBtn) {
        closeBtn.addEventListener('click', function() {
            
            // Get the current network CIDR to dismiss
            let networkCidr = null;
            
            if (window.detectedNetworks && window.detectedNetworks.length > 0 && window.currentSuggestionIndex < window.detectedNetworks.length) {
                // For new detected networks
                networkCidr = window.detectedNetworks[window.currentSuggestionIndex].cidr;
            } else {
                // For existing network suggestions, try to extract CIDR from the text
                const suggestionText = document.getElementById('suggestion-text');
                if (suggestionText) {
                    const text = suggestionText.textContent;
                    const cidrMatch = text.match(/Network (\S+)/);
                    if (cidrMatch) {
                        networkCidr = cidrMatch[1];
                    }
                }
            }
            
            // Add to dismissed list if we found a CIDR
            if (networkCidr) {
                addDismissedNetwork(networkCidr);
            }
            
            hideNetworkSuggestion();
            // Stop checking for suggestions until page refresh
            if (window.networkSuggestionInterval) {
                clearInterval(window.networkSuggestionInterval);
                window.networkSuggestionInterval = null;
            }
        });
    }
    
    // Check for network suggestions immediately and periodically  
    checkForNetworkSuggestions();
    
    // Clear any existing interval
    if (window.networkSuggestionInterval) {
        clearInterval(window.networkSuggestionInterval);
    }
    window.networkSuggestionInterval = setInterval(checkForNetworkSuggestions, 15000);
}

// Test function to force show the network suggestions box
window.testNetworkSuggestions = function() {
    const suggestionElement = document.getElementById('network-suggestions');
    const suggestionText = document.getElementById('suggestion-text');
    
    if (suggestionElement && suggestionText) {
        suggestionText.textContent = 'TEST: Network suggestions are working!';
        suggestionElement.style.display = 'block';
    }
};