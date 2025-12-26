// Network list functionality
function loadNetworkList() {
    const targetEl = document.getElementById('networks-container');
    if (targetEl) {
        targetEl.innerHTML = '<div class="flex items-center justify-center py-8"><div class="animate-spin rounded-full h-8 w-8 border-2 border-green-500 border-t-transparent"></div><span class="ml-3 text-gray-400">Loading networks...</span></div>';
        
        fetch('/api/networks')
            .then(response => response.json())
            .then(data => {
                targetEl.innerHTML = renderNetworkTable(data.networks || [], data.scanState);
            })
            .catch(error => {
                console.error('Error loading network list:', error);
                targetEl.innerHTML = '<div class="text-red-400">Failed to load networks</div>';
            });
    }
}

function renderNetworkTable(networks, scanState) {
    const header = `
        <div class="mb-4 flex justify-between items-center">
            <p class="text-gray-400">Manage network ranges for reconnaissance scanning</p>
        </div>
    `;
    
    if (!networks || networks.length === 0) {
        return header + '<div class="text-center text-gray-400 py-8">No networks configured. Add your first network to start scanning.</div>';
    }
    
    return header + `
        <div class="rounded-lg overflow-hidden" style="background: var(--bg-secondary);">
            <table class="w-full">
                <thead style="background: var(--bg-primary);">
                    <tr>
                        <th class="px-4 py-3 text-left text-green-500">Network</th>
                        <th class="px-4 py-3 text-left text-green-500">CIDR</th>
                        <th class="px-4 py-3 text-left text-green-500">Status</th>
                        <th class="px-4 py-3 text-left text-green-500">Devices</th>
                        <th class="px-4 py-3 text-left text-green-500">Actions</th>
                    </tr>
                </thead>
                <tbody>
                    ${networks.map(network => `
                        <tr class="cursor-pointer ${isSelectedNetwork(network, scanState) ? '' : ''}" style="transition: background 0.2s;${isSelectedNetwork(network, scanState) ? 'background: rgba(16,185,129,0.08); border-left: 4px solid var(--border-accent);' : ''}" onmouseover="this.style.background='var(--bg-tertiary)'" onmouseout="this.style.background='';" onclick="selectNetwork('${network.id}')">
                            <td class="px-4 py-3">
                                <div class="text-gray-200">${network.name || 'Unnamed Network'}</div>
                            </td>
                            <td class="px-4 py-3">
                                <code class="text-green-400 px-2 py-1 rounded text-sm" style="background: var(--bg-primary);">${network.cidr}</code>
                            </td>
                            <td class="px-4 py-3">
                                <span class="px-2 py-1 rounded text-xs ${getNetworkStatusBadgeColor(network.status)}">${getNetworkStatusText(network.status)}</span>
                            </td>
                            <td class="px-4 py-3">
                                <span class="text-gray-400">${network.DeviceCount || 0} devices</span>
                            </td>
                            <td class="px-4 py-3">
                                <div class="flex gap-2">
                                    <button class="px-2 py-1 text-green-400 border border-green-400 rounded text-sm hover:bg-green-400 hover:text-white transition-colors" 
                                            onclick="event.stopPropagation(); editNetwork('${network.id}')"
                                            title="Edit">
                                        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"></path>
                                        </svg>
                                    </button>
                                    <button class="px-2 py-1 text-red-400 rounded text-sm hover:bg-red-400 hover:text-white transition-colors" 
                                            onclick="event.stopPropagation(); deleteNetwork('${network.id}')"
                                            title="Delete">
                                        <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                                            <path d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"/>
                                        </svg>
                                    </button>
                                </div>
                            </td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        </div>
    `;
}

function isSelectedNetwork(network, scanState) {
    return scanState && scanState.selected_network && scanState.selected_network.id === network.id;
}

function getNetworkStatusBadgeColor(status) {
    switch(status) {
        case 'active': return 'bg-green-600 text-white';
        case 'scanning': return 'bg-yellow-600 text-white';
        default: return 'bg-gray-600 text-gray-200';
    }
}

function getNetworkStatusText(status) {
    switch(status) {
        case 'active': return 'Active';
        case 'scanning': return 'Scanning';
        default: return 'Inactive';
    }
}

function editNetwork(networkId) {
    fetch(`/api/network-modal/${networkId}`)
        .then(response => response.json())
        .then(data => {
            const modalContent = document.getElementById('network-modal-content');
            if (modalContent && typeof renderNetworkModal === 'function') {
                modalContent.innerHTML = renderNetworkModal(data);
                showModal('networkModal');
            }
        })
        .catch(error => {
            console.error('Failed to load network edit modal:', error);
        });
}

function deleteNetwork(networkId) {
    if (confirm('Are you sure you want to delete this network? This action cannot be undone.')) {
        fetch(`/api/networks/${networkId}`, {
            method: 'DELETE',
            credentials: 'include'
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                // Try to refresh using both methods to ensure the table updates
                if (typeof window.loadNetworksPage === 'function') {
                    window.loadNetworksPage(); // Refresh main page table
                }
                loadNetworkList(); // Also refresh any other network lists
                alert(data.message || 'Network deleted successfully');
            } else {
                alert(data.error || 'Failed to delete network');
            }
        })
        .catch(error => {
            console.error('Failed to delete network:', error);
            alert('Failed to delete network: ' + error.message);
        });
    }
}

function selectNetwork(networkId) {
    fetch('/api/scan/select-network', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: `network-id=${networkId}`
    })
    .then(response => {
        if (response.ok) {
            loadNetworkList(); // Refresh to show selected state
        }
    })
    .catch(error => {
        console.error('Error selecting network:', error);
    });
}

function loadNetworkModal() {
    fetch('/api/network-modal')
        .then(response => response.json())
        .then(data => {
            const modalContent = document.getElementById('network-modal-content');
            if (modalContent) {
                modalContent.innerHTML = renderNetworkModal(data);
                showModal('networkModal');
            }
        })
        .catch(error => {
            console.error('Failed to load network modal:', error);
        });
}

function renderNetworkModal(data) {
    const network = data.network || {};
    const error = data.error || '';
    const isEdit = network && network.id;
    
    return `
        <div class="p-6">
            <!-- Header -->
            <div class="flex justify-between items-center mb-4 pb-3 border-b border-green-600">
                <div class="flex items-center">
                    <svg class="w-6 h-6 text-green-500 mr-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"></path>
                    </svg>
                    <span class="text-lg font-bold text-green-500">
                        ${isEdit ? 'Edit Network' : 'Add New Network'}
                    </span>
                </div>
                <button type="button" class="text-gray-400 hover:text-white text-xl" onclick="closeModal('networkModal')">
                    <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                    </svg>
                </button>
            </div>
            
            <!-- Form -->
            <form onsubmit="return submitNetworkForm(event, '${isEdit ? 'PUT' : 'POST'}', '${isEdit ? `/api/networks/${network.id}` : '/api/networks'}')">
                ${error ? `
                    <div class="bg-red-600/10 border border-red-500 rounded p-3 mb-4">
                        <div class="flex items-center text-red-400">
                            <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.728-.833-2.498 0L4.268 16.5c-.77.833.192 2.5 1.732 2.5z"></path>
                            </svg>
                            ${error}
                        </div>
                    </div>
                ` : ''}
                
                <div class="mb-4">
                    <label for="networkName" class="block text-green-500 text-sm font-semibold mb-2">Network Name</label>
                    <input type="text" 
                           class="w-full px-3 py-2 rounded text-sm focus:outline-none focus:ring-2 focus:ring-green-500"
                           style="background: var(--bg-tertiary); border: 1px solid var(--border-accent); color: var(--text-primary);"
                           id="networkName" 
                           name="name"
                           value="${network?.name || ''}"
                           placeholder="e.g., Home Network, Office LAN">
                    <div class="text-gray-400 text-xs mt-1">Optional friendly name</div>
                </div>
                
                <div class="mb-4">
                    <label for="networkCIDR" class="block text-green-500 text-sm font-semibold mb-2">CIDR Address <span class="text-red-400">*</span></label>
                    <input type="text" 
                           class="w-full px-3 py-2 rounded text-sm focus:outline-none focus:ring-2 focus:ring-green-500"
                           style="background: var(--bg-tertiary); border: 1px solid var(--border-accent); color: var(--text-primary);"
                           id="networkCIDR" 
                           name="cidr"
                           value="${network?.cidr || ''}"
                           placeholder="e.g., 192.168.1.0/24"
                           required>
                    <div class="text-gray-400 text-xs mt-1">Network range in CIDR notation</div>
                </div>
                
                <div class="mb-4">
                    <label for="networkDescription" class="block text-green-500 text-sm font-semibold mb-2">Description</label>
                    <textarea class="w-full px-3 py-2 rounded text-sm focus:outline-none focus:ring-2 focus:ring-green-500 resize-y" 
                              style="background: var(--bg-tertiary); border: 1px solid var(--border-accent); color: var(--text-primary);"
                              id="networkDescription" 
                              name="description"
                              rows="2"
                              placeholder="Optional description...">${network?.Description || ''}</textarea>
                </div>
                
                ${!isEdit ? `
                    <div class="bg-blue-600/10 border border-blue-500 rounded p-3 mb-4">
                        <div class="flex items-center text-blue-400">
                            <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                            </svg>
                            <span class="text-sm">After adding, you can scan this network to discover devices.</span>
                        </div>
                    </div>
                ` : ''}
                
                <div class="flex justify-end gap-3 pt-3" style="border-top: 1px solid var(--border-accent);">
                    <button type="button" class="px-4 py-2 rounded transition-colors" style="color: var(--text-muted); border: 1px solid var(--border-color); background: none;" onclick="closeModal('networkModal')">
                        Cancel
                    </button>
                    <button type="submit" class="px-4 py-2 rounded transition-colors" style="background: var(--bs-success); color: #fff;" >
                        ${isEdit ? 'Update' : 'Add'} Network
                    </button>
                </div>
            </form>
        </div>
    `;
}

function submitNetworkForm(event, method, url) {
    event.preventDefault();

    const form = event.target;
    const formData = new FormData(form);

    fetch(url, {
        method: method,
        body: formData
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            closeModal('networkModal');
            // Try to refresh using both methods to ensure the table updates
            if (typeof window.loadNetworksPage === 'function') {
                window.loadNetworksPage(); // Refresh main page table
            }
            loadNetworkList(); // Also refresh any other network lists
            // Also refresh scan control if it exists
            if (typeof window.loadScanControl === 'function') {
                window.loadScanControl(false);
            }
        } else {
            // Show error in modal
            const modalContent = document.getElementById('network-modal-content');
            if (modalContent) {
                modalContent.innerHTML = renderNetworkModal({
                    network: Object.fromEntries(formData.entries()),
                    error: data.error || 'Failed to save network'
                });
            }
            // Also show an alert for user feedback
            alert(data.error || 'Failed to save network');
        }
    })
    .catch(error => {
        console.error('Failed to submit network form:', error);
        alert('Failed to save network');
    });

    return false; // Prevent default form submission
}

// Make functions available globally
window.loadNetworkList = loadNetworkList;
window.renderNetworkTable = renderNetworkTable;
window.isSelectedNetwork = isSelectedNetwork;
window.getNetworkStatusBadgeColor = getNetworkStatusBadgeColor;
window.getNetworkStatusText = getNetworkStatusText;
window.editNetwork = editNetwork;
window.deleteNetwork = deleteNetwork;
window.selectNetwork = selectNetwork;
window.loadNetworkModal = loadNetworkModal;
window.renderNetworkModal = renderNetworkModal;
window.submitNetworkForm = submitNetworkForm;