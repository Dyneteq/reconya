function showModal(modalId) {
    var modal = document.getElementById(modalId);
    if (modal) {
        modal.style.display = 'flex';
        setTimeout(function() {
            modal.classList.add('show');
        }, 10);

        var backdrop = modal.querySelector('.modal-backdrop');
        if (backdrop) {
            backdrop.onclick = function() { closeModal(modalId); };
        }
    }
}

function closeModal(modalId) {
    var modal = document.getElementById(modalId);
    if (modal) {
        modal.classList.remove('show');
        setTimeout(function() {
            modal.style.display = 'none';
        }, 200);
    }
}

window.showModal = showModal;
window.closeModal = closeModal;
