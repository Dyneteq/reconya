/* Settings (CONF view).
 *
 * Only the settings the backend actually persists are rendered. The old form
 * offered scan_interval and concurrent_scans fields that no handler ever read
 * or stored — showing controls that silently do nothing is worse than not
 * showing them, so they are gone. What the app does hold that is not editable
 * (sweep cadence, offline threshold) is shown read-only with where to change it.
 */
(function () {
    'use strict';

    function loadSettings() {
        return RC.getJSON('/api/settings')
            .then(function (data) {
                render(data.settings || {});
                return data.settings;
            })
            .catch(function (err) {
                console.error('Failed to load settings:', err);
                RC.render(RC.el('rc-settings-page'), RC.empty('Could not load settings'));
            });
    }

    function render(settings) {
        var container = RC.el('rc-settings-page');
        if (!container) return;

        RC.render(container,
            '<div class="rc-form">' +
                '<div>' +
                    '<div class="rc-section" style="padding:0 0 10px">SCANNING</div>' +
                    '<label class="rc-check">' +
                        '<input type="checkbox" id="rc-set-screenshots"' +
                            (settings.screenshots_enabled ? ' checked' : '') + '>' +
                        '<span>Capture screenshots of web services found on discovered hosts. ' +
                        'This runs a headless browser against every open HTTP port, so it makes ' +
                        'scans noticeably slower.</span>' +
                    '</label>' +
                '</div>' +

                '<div style="display:flex;gap:10px;align-items:center">' +
                    '<button class="rc-btn rc-btn--solid" id="rc-set-save">SAVE</button>' +
                    '<span id="rc-set-status" style="font-size:10px;letter-spacing:.12em;color:var(--rc-text-5)"></span>' +
                '</div>' +

                '<div>' +
                    '<div class="rc-section" style="padding:0 0 10px">FIXED VALUES</div>' +
                    '<dl class="rc-kv" style="border:1px solid var(--rc-line-10);padding:14px 16px">' +
                        '<dt>SWEEP EVERY</dt><dd>30 seconds</dd>' +
                        '<dt>IDLE AFTER</dt><dd>1 minute without a reply</dd>' +
                        '<dt>OFFLINE AFTER</dt><dd>3 minutes without a reply</dd>' +
                        '<dt>ALERT AFTER</dt><dd>ALERT_OFFLINE_THRESHOLD_HOURS (default 6h)</dd>' +
                        '<dt>PORT RESCAN</dt><dd>30 second cooldown per host</dd>' +
                    '</dl>' +
                    '<div class="rc-field__hint" style="margin-top:10px">' +
                        'These come from the environment and the scan manager rather than the ' +
                        'database. Change them in <span class="rc-code">.env</span> and restart.' +
                    '</div>' +
                '</div>' +

                '<div>' +
                    '<div class="rc-section" style="padding:0 0 10px">MAINTENANCE</div>' +
                    '<div style="display:flex;gap:8px;flex-wrap:wrap">' +
                        '<button class="rc-btn" id="rc-set-cleanup-names">CLEAN UP HOST NAMES</button>' +
                        '<button class="rc-btn" id="rc-set-cleanup-broadcast">REMOVE BROADCAST ENTRIES</button>' +
                    '</div>' +
                    '<div class="rc-field__hint" style="margin-top:10px" id="rc-set-maint-status">' +
                        'Removes placeholder names and network/broadcast addresses that older ' +
                        'sweeps recorded as hosts.' +
                    '</div>' +
                '</div>' +
            '</div>');

        bindForm();
    }

    function bindForm() {
        var save = RC.el('rc-set-save');
        if (save) {
            save.addEventListener('click', function () {
                var box = RC.el('rc-set-screenshots');
                RC.text('rc-set-status', 'SAVING…');

                RC.sendForm('/api/settings/screenshots', {
                    screenshots_enabled: box && box.checked ? 'true' : 'false'
                }).then(function (res) {
                    RC.text('rc-set-status', res && res.success ? 'SAVED' : 'FAILED');
                }).catch(function (err) {
                    console.error('Failed to save settings:', err);
                    RC.text('rc-set-status', 'FAILED');
                });
            });
        }

        maintenance('rc-set-cleanup-names', '/api/devices/cleanup-names');
        maintenance('rc-set-cleanup-broadcast', '/api/devices/cleanup-network-broadcast');
    }

    function maintenance(buttonId, url) {
        var btn = RC.el(buttonId);
        if (!btn) return;

        btn.addEventListener('click', function () {
            btn.disabled = true;
            RC.text('rc-set-maint-status', 'Running…');

            fetch(url, { method: 'POST', credentials: 'include' })
                .then(function (r) { return r.text(); })
                .then(function (body) {
                    RC.text('rc-set-maint-status', body || 'Done.');
                })
                .catch(function (err) {
                    console.error('Maintenance task failed:', err);
                    RC.text('rc-set-maint-status', 'Failed: ' + err.message);
                })
                .then(function () { btn.disabled = false; });
        });
    }

    window.RCSettings = { load: loadSettings };
})();
