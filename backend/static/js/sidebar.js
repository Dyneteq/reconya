// Sidebar functionality
function initSidebar() {
    console.log('Initializing sidebar...');
    const sidebar = document.getElementById('sidebar');
    const sidebarToggle = document.getElementById('sidebarToggle');
    const sidebarToggleIcon = sidebarToggle ? sidebarToggle.querySelector('i') : null;
    const mainContent = document.getElementById('main-content');
    const navItems = document.querySelectorAll('.nav-item');
    
    console.log('Sidebar elements found:', { 
        sidebar: !!sidebar, 
        sidebarToggle: !!sidebarToggle, 
        mainContent: !!mainContent, 
        navItemsCount: navItems.length 
    });
        
    // Start with sidebar collapsed
    if (sidebar) {
        sidebar.classList.add('collapsed');
    }
    if (mainContent) {
        mainContent.style.marginLeft = '0';
    }
    
    // Single sidebar toggle functionality
    if (sidebarToggle) {
        console.log('Adding sidebar toggle click handler');
        sidebarToggle.addEventListener('click', function(e) {
            console.log('Sidebar toggle clicked!');
            e.stopPropagation(); // Prevent click from bubbling
            
            if (sidebar.classList.contains('collapsed')) {
                console.log('Opening sidebar');
                sidebar.classList.remove('collapsed');
                if (mainContent) mainContent.style.marginLeft = '16rem'; // 256px
            } else {
                console.log('Closing sidebar');
                sidebar.classList.add('collapsed');
                if (mainContent) mainContent.style.marginLeft = '0';
            }
        });
    } else {
        console.log('Sidebar toggle button not found!');
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
            } else {
                console.log('No data-page found on clicked item');
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