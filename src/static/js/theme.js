function initTheme() {
    const savedTheme = localStorage.getItem('theme') || 'dark';

    applyTheme(savedTheme);

    setupThemeToggle();
}

function applyTheme(theme) {
    const sunIcon = document.getElementById('sunIcon');
    const moonIcon = document.getElementById('moonIcon');
    
    document.documentElement.setAttribute('data-theme', theme);
    
    if (theme === 'light') {
        if (sunIcon) sunIcon.classList.add('hidden');
        if (moonIcon) moonIcon.classList.remove('hidden');
    } else {
        if (sunIcon) sunIcon.classList.remove('hidden');
        if (moonIcon) moonIcon.classList.add('hidden');
    }
    
    localStorage.setItem('theme', theme);
}

function toggleTheme() {
    const currentTheme = document.documentElement.getAttribute('data-theme');
    const newTheme = currentTheme === 'light' ? 'dark' : 'light';
    applyTheme(newTheme);
}

function setupThemeToggle() {
    const themeToggle = document.getElementById('themeToggle');

    if (themeToggle) {
        themeToggle.removeEventListener('click', toggleTheme);
        themeToggle.addEventListener('click', toggleTheme);
    } else {
        let retryCount = 0;
        const maxRetries = 3;
        const retrySetup = () => {
            if (retryCount < maxRetries) {
                retryCount++;
                console.log(`Retrying theme toggle setup... (${retryCount}/${maxRetries})`);
                setTimeout(() => {
                    const toggle = document.getElementById('themeToggle');
                    if (toggle) {
                        toggle.removeEventListener('click', toggleTheme);
                        toggle.addEventListener('click', toggleTheme);
                    } else {
                        retrySetup();
                    }
                }, 50 * retryCount);
            } else {
                console.warn('Failed to find theme toggle button after maximum retries');
            }
        };
        retrySetup();
    }
}

function forceThemeSwitch() {
    const html = document.documentElement;
    const current = html.getAttribute('data-theme');
    
    if (current === 'light') {
        html.setAttribute('data-theme', 'dark');
    } else {
        html.setAttribute('data-theme', 'light');
    }
}

window.initTheme = initTheme;
window.applyTheme = applyTheme;
window.toggleTheme = toggleTheme;
window.forceThemeSwitch = forceThemeSwitch;