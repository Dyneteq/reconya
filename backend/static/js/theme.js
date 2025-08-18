// Theme functionality
function initTheme() {
    console.log('Initializing theme...');
    const themeToggle = document.getElementById('themeToggle');
    const sunIcon = document.getElementById('sunIcon');
    const moonIcon = document.getElementById('moonIcon');
    
    console.log('Theme elements found:', { themeToggle, sunIcon, moonIcon });
    
    // Get saved theme or default to dark
    const savedTheme = localStorage.getItem('theme') || 'dark';
    
    // Apply saved theme
    if (savedTheme === 'light') {
        document.documentElement.setAttribute('data-theme', 'light');
        if (sunIcon) sunIcon.classList.add('hidden');
        if (moonIcon) moonIcon.classList.remove('hidden');
    } else {
        document.documentElement.setAttribute('data-theme', 'dark');
        if (sunIcon) sunIcon.classList.remove('hidden');
        if (moonIcon) moonIcon.classList.add('hidden');
    }
    
    // Theme toggle functionality
    if (themeToggle) {
        console.log('Adding theme toggle click handler');
        themeToggle.addEventListener('click', function() {
            console.log('Theme toggle clicked!');
            const currentTheme = document.documentElement.getAttribute('data-theme');
            console.log('Current theme:', currentTheme);
            
            if (currentTheme === 'light') {
                // Switch to dark
                console.log('Switching to dark theme');
                document.documentElement.setAttribute('data-theme', 'dark');
                if (sunIcon) sunIcon.classList.remove('hidden');
                if (moonIcon) moonIcon.classList.add('hidden');
                localStorage.setItem('theme', 'dark');
            } else {
                // Switch to light
                console.log('Switching to light theme');
                document.documentElement.setAttribute('data-theme', 'light');
                if (sunIcon) sunIcon.classList.add('hidden');
                if (moonIcon) moonIcon.classList.remove('hidden');
                localStorage.setItem('theme', 'light');
            }
            
            // Theme will be applied automatically via CSS
        });
    } else {
        console.log('Theme toggle button not found!');
    }
}