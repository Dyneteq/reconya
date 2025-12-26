function showModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.style.display = 'flex';
        setTimeout(() => {
            modal.classList.add('show');
            const modalContent = modal.querySelector('.modal-content, .bg-zinc-800');
            if (modalContent) {
                modalContent.classList.add('show');
            }
        }, 10);

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
        modal.classList.remove('show');
        const modalContent = modal.querySelector('.modal-content, .bg-zinc-800');
        if (modalContent) {
            modalContent.classList.remove('show');
        }

        setTimeout(() => {
            modal.style.display = 'none';
        }, 200);
    }
}

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

window.showModal = showModal;
window.closeModal = closeModal;
window.confirmNetworkDelete = confirmNetworkDelete;