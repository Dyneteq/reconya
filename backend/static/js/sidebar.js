// Sidebar functionality
function initSidebar() {
    const sidebar = document.getElementById('sidebar');
    const sidebarToggle = document.getElementById('sidebarToggle');
    const sidebarToggleIcon = sidebarToggle ? sidebarToggle.querySelector('i') : null;
    const mainContent = document.getElementById('main-content');
    const navItems = document.querySelectorAll('.nav-item');
        
    // Start with sidebar collapsed
    if (sidebar) {
        sidebar.classList.add('collapsed');
    }
    if (mainContent) {
        mainContent.style.marginLeft = '0';
    }
    
    // Single sidebar toggle functionality
    if (sidebarToggle) {
        sidebarToggle.addEventListener('click', function(e) {
            e.stopPropagation(); // Prevent click from bubbling
            
            if (sidebar.classList.contains('collapsed')) {
                sidebar.classList.remove('collapsed');
                if (mainContent) mainContent.style.marginLeft = '16rem'; // 256px
            } else {
                sidebar.classList.add('collapsed');
                if (mainContent) mainContent.style.marginLeft = '0';
            }
        });
    }
    
    // Navigation functionality
    navItems.forEach((item) => {
        item.addEventListener('click', function(e) {
            const page = this.getAttribute('data-page');
            
            if (page) {
                e.preventDefault();
                e.stopPropagation();
                                
                // Remove active class from all items
                navItems.forEach(nav => nav.classList.remove('active'));
                
                // Add active class to clicked item
                this.classList.add('active');
                
                // Close sidebar only on mobile after navigation (keep open on desktop)
                if (window.innerWidth <= 1024) {
                    sidebar.classList.add('collapsed');
                    mainContent.style.marginLeft = '0';
                }
                
                // Navigate to page
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
    
    // Set active nav item based on current page
    const currentPath = window.location.pathname;
    const currentPage = currentPath === '/' ? 'home' : currentPath.substring(1);
    const activeItem = document.querySelector(`[data-page="${currentPage}"]`);
    if (activeItem) {
        activeItem.classList.add('active');
    }
    
    // Mobile responsive behavior
    function handleResize() {
        if (window.innerWidth <= 1024) {
            // Mobile view - collapse sidebar for overlay behavior
            if (!sidebar.classList.contains('collapsed')) {
                sidebar.classList.add('collapsed');
            }
            mainContent.style.marginLeft = '0';
        } else {
            // Desktop view - sidebar behavior is unchanged
        }
    }
    
    // Listen for window resize
    window.addEventListener('resize', handleResize);
    
    // Initial check
    handleResize();
}

// Sidebar initialization is now handled by main.js DOMContentLoaded