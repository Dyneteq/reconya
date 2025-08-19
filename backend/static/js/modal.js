// Modal functionality
function showModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.style.display = 'flex';
        // Add show animation
        setTimeout(() => {
            modal.classList.add('show');
            const modalContent = modal.querySelector('.modal-content, .bg-zinc-800');
            if (modalContent) {
                modalContent.classList.add('show');
            }
        }, 10);
        
        // Close modal when clicking backdrop
        const backdrop = modal.querySelector('.modal-backdrop');
        if (backdrop) {
            backdrop.addEventListener('click', function(event) {
                closeModal(modalId);
            });
        }
    }
}

function closeModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        // Remove show animation
        modal.classList.remove('show');
        const modalContent = modal.querySelector('.modal-content, .bg-zinc-800');
        if (modalContent) {
            modalContent.classList.remove('show');
        }
        
        // Hide modal after animation
        setTimeout(() => {
            modal.style.display = 'none';
        }, 200);
    }
}

// Global modal functions for network operations
function confirmNetworkDelete(networkId) {
    
    fetch(`/api/network-delete-modal/${networkId}`)
        .then(response => {
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            return response.text();
        })
        .then(html => {
            console.log('Network delete modal HTML received');
            const modalContent = document.getElementById('network-delete-modal-content');
            if (modalContent) {
                modalContent.innerHTML = html;
                showModal('networkDeleteModal');
            } else {
                console.error('Modal content element not found');
            }
        })
        .catch(error => {
            console.error('Failed to load network delete modal:', error);
            alert('Failed to load delete confirmation. Please try again.');
        });
}

// Make functions available globally
window.showModal = showModal;
window.closeModal = closeModal;
window.confirmNetworkDelete = confirmNetworkDelete;