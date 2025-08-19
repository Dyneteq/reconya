// Settings functionality
function loadSettings() {
    const container = document.getElementById('settings-container');
    if (!container) return;
    
    container.innerHTML = '<div class="flex items-center justify-center py-8"><div class="animate-spin rounded-full h-8 w-8 border-2 border-green-500 border-t-transparent"></div><span class="ml-3 text-gray-400">Loading settings...</span></div>';
    
    fetch('/api/settings', { credentials: 'include' })
        .then(response => {
            console.log('Settings response status:', response.status);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            return response.json();
        })
        .then(data => {
            console.log('Settings data:', data);
            container.innerHTML = renderSettings(data.settings || {});
            setupSettingsEventListeners();
        })
        .catch(error => {
            console.error('Failed to load settings:', error);
            // Show a basic settings form even if API fails
            container.innerHTML = renderSettings({
                screenshots_enabled: false,
                scan_timeout: 30,
                concurrent_scans: 50
            });
            setupSettingsEventListeners();
            
            // Show error at top of form
            const form = document.getElementById('settings-form');
            if (form) {
                form.insertAdjacentHTML('afterbegin', `
                    <div class="bg-red-600/10 border border-red-500 rounded p-3 mb-4">
                        <div class="flex items-center text-red-400">
                            <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.728-.833-2.498 0L4.268 16.5c-.77.833.192 2.5 1.732 2.5z"></path>
                            </svg>
                            Could not load current settings: ${error.message}
                        </div>
                    </div>
                `);
            }
        });
}

function renderSettings(settings) {
    return `
        <div class="p-6">
            <h2 class="text-xl font-bold mb-6 text-green-500">Settings</h2>
            
            <form id="settings-form" class="space-y-6">
                <div class="bg-gray-900 rounded-lg p-6 max-w-2xl">
                    <div class="space-y-6">
                        <!-- Screenshots Toggle -->
                        <div class="flex items-center justify-between py-4 border-b border-gray-700">
                            <div>
                                <h3 class="text-lg font-medium text-gray-200">Screenshots</h3>
                                <p class="text-sm text-gray-400">Enable automatic screenshots of web services</p>
                            </div>
                            <label class="relative inline-flex items-center cursor-pointer">
                                <input type="checkbox" 
                                       name="screenshots_enabled" 
                                       class="sr-only peer" 
                                       id="screenshotsToggle"
                                       ${settings.screenshots_enabled ? 'checked' : ''}>
                                <div class="w-11 h-6 bg-gray-600 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-green-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-green-600"></div>
                            </label>
                        </div>
                        
                        <!-- Scan Timeout Setting -->
                        <div class="py-4 border-b border-gray-700">
                            <div class="mb-3">
                                <h3 class="text-lg font-medium text-gray-200">Scan Timeout</h3>
                                <p class="text-sm text-gray-400">Maximum time to wait for port scan responses (seconds)</p>
                            </div>
                            <input type="number" 
                                   name="scan_timeout" 
                                   class="w-32 px-3 py-2 bg-gray-700 border border-gray-600 text-gray-200 rounded text-sm focus:outline-none focus:ring-2 focus:ring-green-500" 
                                   value="${settings.scan_timeout || 30}"
                                   min="1"
                                   max="300">
                        </div>
                        
                        <!-- Concurrent Scans Setting -->
                        <div class="py-4 border-b border-gray-700">
                            <div class="mb-3">
                                <h3 class="text-lg font-medium text-gray-200">Concurrent Scans</h3>
                                <p class="text-sm text-gray-400">Number of simultaneous port scans to run</p>
                            </div>
                            <input type="number" 
                                   name="concurrent_scans" 
                                   class="w-32 px-3 py-2 bg-gray-700 border border-gray-600 text-gray-200 rounded text-sm focus:outline-none focus:ring-2 focus:ring-green-500" 
                                   value="${settings.concurrent_scans || 50}"
                                   min="1"
                                   max="1000">
                        </div>
                        
                        <!-- Settings Info -->
                        <div class="text-sm text-gray-500 space-y-2 pt-4">
                            ${settings.id ? `<div><span class="font-medium">Settings ID:</span> ${settings.id}</div>` : ''}
                            ${settings.created_at ? `<div><span class="font-medium">Created:</span> ${new Date(settings.created_at).toLocaleString()}</div>` : ''}
                            ${settings.updated_at ? `<div><span class="font-medium">Last Updated:</span> ${new Date(settings.updated_at).toLocaleString()}</div>` : ''}
                        </div>
                    </div>
                </div>
                
                <!-- Save Button -->
                <div class="flex justify-end max-w-2xl">
                    <button type="submit" class="px-6 py-2 bg-green-600 hover:bg-green-700 text-white rounded transition-colors">
                        Save Settings
                    </button>
                </div>
            </form>
        </div>
    `;
}

function setupSettingsEventListeners() {
    const form = document.getElementById('settings-form');
    if (form) {
        form.addEventListener('submit', handleSettingsSubmit);
    }
}

function handleSettingsSubmit(event) {
    event.preventDefault();
    
    const formData = new FormData(event.target);
    const submitBtn = event.target.querySelector('button[type="submit"]');
    
    // Handle screenshots setting
    const screenshotsEnabled = formData.has('screenshots_enabled');
    const screenshotsFormData = new FormData();
    screenshotsFormData.append('screenshots_enabled', screenshotsEnabled ? 'true' : 'false');
    
    // Save screenshots setting
    fetch('/api/settings/screenshots', {
        method: 'POST',
        body: screenshotsFormData,
        credentials: 'include'
    })
    .then(response => response.json())
    .then(data => {
        console.log('Screenshots setting response:', data);
        if (data.success) {
            // Show success feedback
            if (submitBtn) {
                const originalText = submitBtn.textContent;
                submitBtn.textContent = 'Saved!';
                submitBtn.disabled = true;
                setTimeout(() => {
                    submitBtn.textContent = originalText;
                    submitBtn.disabled = false;
                }, 2000);
            }
        } else {
            alert('Failed to save settings: ' + (data.error || 'Unknown error'));
        }
    })
    .catch(error => {
        console.error('Failed to save settings:', error);
        alert('Failed to save settings: ' + error.message);
    });
}

// Make functions available globally
window.loadSettings = loadSettings;
window.renderSettings = renderSettings;
window.setupSettingsEventListeners = setupSettingsEventListeners;
window.handleSettingsSubmit = handleSettingsSubmit;