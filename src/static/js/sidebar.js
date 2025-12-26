function initSidebar() {
    const sidebar = document.getElementById('sidebar');
    const sidebarToggle = document.getElementById('sidebarToggle');
    const sidebarToggleIcon = sidebarToggle ? sidebarToggle.querySelector('i') : null;
    const mainContent = document.getElementById('main-content');
    const navItems = document.querySelectorAll('.nav-item');

    if (sidebar) {
        sidebar.classList.remove('collapsed');
    }
    if (mainContent) {
        mainContent.style.marginLeft = '16rem';
    }

    if (sidebarToggle) {
        sidebarToggle.addEventListener('click', function(e) {
            e.stopPropagation();

            if (sidebar.classList.contains('collapsed')) {
                sidebar.classList.remove('collapsed');
                if (mainContent) mainContent.style.marginLeft = '16rem';
            } else {
                sidebar.classList.add('collapsed');
                if (mainContent) mainContent.style.marginLeft = '0';
            }
        });
    }

    navItems.forEach((item) => {
        item.addEventListener('click', function(e) {
            const page = this.getAttribute('data-page');

            if (page) {
                e.preventDefault();
                e.stopPropagation();

                navItems.forEach(nav => nav.classList.remove('active'));

                this.classList.add('active');

                setTimeout(() => {
                    if (page === 'home') {
                        window.location.href = '/';
                    } else {
                        window.location.href = `/${page}`;
                    }
                }, 100);
            }
        });
    });

    const currentPath = window.location.pathname;
    const currentPage = currentPath === '/' ? 'home' : currentPath.substring(1);
    const activeItem = document.querySelector(`[data-page="${currentPage}"]`);
    if (activeItem) {
        activeItem.classList.add('active');
    }

    function handleResize() {
    }

    window.addEventListener('resize', handleResize);

    handleResize();
}