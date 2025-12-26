document.addEventListener('DOMContentLoaded', function() {
    if (typeof initTheme === 'function') {
        initTheme();
    }

    if (typeof initSidebar === 'function') {
        initSidebar();
    }

    if (typeof initNetworkSuggestions === 'function') {
        initNetworkSuggestions();
    }

    initDropdownMenu();

    initPageContent();
});

function initPageContent() {
    const currentPath = window.location.pathname;

    switch (currentPath) {
        case '/':
        case '/home':
            if (typeof loadDevices === 'function') {
                loadDevices();
            }
            if (typeof loadScanControl === 'function') {
                loadScanControl();
            }
            if (typeof startNetworkMapUpdates === 'function') {
                startNetworkMapUpdates();
            }
            if (typeof loadRecentActivity === 'function') {
                loadRecentActivity();
            }
            if (typeof loadDashboardMetrics === 'function') {
                loadDashboardMetrics();
                setInterval(loadDashboardMetrics, 30000);
            }
            break;
        case '/devices':
            if (typeof loadDeviceList === 'function') {
                loadDeviceList();
            } else if (typeof loadDevices === 'function') {
                loadDevices();
            }
            if (typeof startNetworkMapUpdates === 'function') {
                startNetworkMapUpdates();
            }
            break;
        case '/networks':
            if (typeof loadNetworkList === 'function') {
                loadNetworkList();
            }
            break;
        case '/settings':
            if (typeof loadSettings === 'function') {
                loadSettings();
            }
            break;
        default:
            break;
    }
}

function initDropdownMenu() {
    const dropdownButton = document.getElementById('dropdownMenuButton');
    const dropdownMenu = document.getElementById('dropdownMenu');

    if (dropdownButton && dropdownMenu) {
        dropdownButton.addEventListener('click', function(e) {
            e.preventDefault();
            e.stopPropagation();
            dropdownMenu.classList.toggle('hidden');
        });

        document.addEventListener('click', function(e) {
            if (!dropdownButton.contains(e.target) && !dropdownMenu.contains(e.target)) {
                dropdownMenu.classList.add('hidden');
            }
        });

        document.addEventListener('keydown', function(e) {
            if (e.key === 'Escape') {
                dropdownMenu.classList.add('hidden');
            }
        });

        window.testDropdownToggle = function() {
            dropdownMenu.classList.toggle('hidden');
        };
    }
}

function loadAboutPage() {
    showAboutModal();
}

function showAboutModal() {
    fetch('/api/about', { credentials: 'include' })
        .then(response => response.json())
        .then(data => {
            const version = data.version || 'unknown';
            displayAboutModal(version);
        })
        .catch(error => {
            console.error('Error fetching version:', error);
            displayAboutModal('unknown');
        });
}

function displayAboutModal(version) {
    const aboutContent = `
        <div class="p-6 max-w-2xl mx-auto">
            <div class="text-center mb-6">
                <h1 class="text-3xl font-bold text-green-500 mb-3" style="font-family: 'Orbitron', monospace; font-weight: 800; letter-spacing: 0.5px;">
                    <i class="ti ti-network mr-2"></i>reconYa
                </h1>
                <p class="text-lg text-gray-300 mb-3">Network Reconnaissance and Asset Discovery Tool</p>
                <div class="inline-block bg-green-500 text-black px-3 py-1 rounded text-sm font-semibold">
                    v${version}
                </div>
            </div>

            <div class="bg-gray-800 rounded-lg p-4 mb-4 border border-green-500/30">
                <h3 class="text-lg font-semibold text-green-500 mb-3">
                    <i class="ti ti-info-circle mr-2"></i>About reconYa
                </h3>
                <p class="text-gray-300 mb-3 text-sm">
                    reconYa is a comprehensive network reconnaissance and asset discovery tool built with Go and modern web technologies. 
                    It provides real-time network scanning, device identification, and monitoring capabilities for 
                    network administrators, security professionals, and enthusiasts.
                </p>
                <p class="text-gray-300 mb-2 text-sm">
                    <strong class="text-white">Key Features:</strong>
                </p>
                <ul class="space-y-1 text-sm">
                    <li class="flex items-center text-gray-300"><i class="ti ti-circle-check text-green-500 mr-2"></i>Real-time network scanning with nmap integration</li>
                    <li class="flex items-center text-gray-300"><i class="ti ti-circle-check text-green-500 mr-2"></i>Device identification and vendor detection</li>
                    <li class="flex items-center text-gray-300"><i class="ti ti-circle-check text-green-500 mr-2"></i>Port scanning and service detection</li>
                    <li class="flex items-center text-gray-300"><i class="ti ti-circle-check text-green-500 mr-2"></i>Web-based dashboard with live updates</li>
                    <li class="flex items-center text-gray-300"><i class="ti ti-circle-check text-green-500 mr-2"></i>Event logging and monitoring</li>
                </ul>
            </div>

            <div class="bg-gray-800 rounded-lg p-4 mb-4 border border-green-500/30">
                <h3 class="text-lg font-semibold text-green-500 mb-3">
                    <i class="ti ti-users mr-2"></i>Community & Support
                </h3>
                <div class="flex flex-wrap gap-2 text-sm">
                    <a href="https://discord.gg/JW7VtBnNXp" target="_blank" class="bg-blue-600 hover:bg-blue-700 text-white px-3 py-1 rounded transition-colors">
                        <i class="ti ti-brand-discord mr-1"></i>Discord
                    </a>
                    <a href="https://github.com/Dyneteq/reconya" target="_blank" class="bg-gray-700 hover:bg-gray-600 text-white px-3 py-1 rounded border border-gray-600 transition-colors">
                        <i class="ti ti-brand-github mr-1"></i>GitHub
                    </a>
                    <a href="https://www.reddit.com/r/reconya/" target="_blank" class="bg-orange-600 hover:bg-orange-700 text-white px-3 py-1 rounded transition-colors">
                        <i class="ti ti-brand-reddit mr-1"></i>Reddit
                    </a>
                </div>
            </div>

            <div class="bg-gray-800 rounded-lg p-4 border border-green-500/30">
                <h3 class="text-lg font-semibold text-green-500 mb-3">
                    <i class="ti ti-file-text mr-2"></i>License
                </h3>
                <p class="text-gray-300 mb-2 text-sm">
                    reconYa is released under the <strong class="text-white">Creative Commons Attribution-NonCommercial 4.0 International License</strong>. 
                    Commercial use requires permission.
                </p>
                <p class="text-gray-500 text-xs">
                    Built with <i class="ti ti-heart text-red-500"></i> using Go, Tailwind CSS, and modern web technologies.
                </p>
            </div>
        </div>
    `;

    // Create and show modal
    const modalEl = document.getElementById('aboutModal');
    if (modalEl) {
        const contentEl = document.getElementById('about-modal-content');
        if (contentEl) {
            contentEl.innerHTML = aboutContent;
            showModal('aboutModal');
        }
    }
}

// Make functions globally available
window.loadAboutPage = loadAboutPage;
window.showAboutModal = showAboutModal;