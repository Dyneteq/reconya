/* Devices: the shared table renderer and the inspection drawer.
 *
 * One component serves three surfaces — the dock strip under the map, the
 * full-height HOSTS page, and the drawer that both of them (and the map, and
 * the alert feed) open. They render the same columns because they are the same
 * table.
 */
(function () {
    'use strict';

    // Shared cache so the map, dock, drawer and metrics all read one snapshot
    // instead of each fetching their own and disagreeing.
    var state = {
        devices: [],
        networks: {},          // network id -> CIDR
        activeNetworkId: null,
        selectedId: null,
        tab: 'overview',
        detail: null,          // full device record for the open drawer
        history: null,
        listeners: []
    };

    var RISKY_PORTS = ['21', '23', '445', '3389', '5900', '9100'];
    var ADDRESSING_LABELS = { '': 'UNKNOWN', 'static': 'STATIC', 'dhcp': 'DHCP' };
    var ADDRESSING_CYCLE = { '': 'static', 'static': 'dhcp', 'dhcp': '' };

    function onDevices(fn) {
        state.listeners.push(fn);
    }

    function notify() {
        state.listeners.forEach(function (fn) {
            try { fn(state); } catch (e) { console.error('device listener failed', e); }
        });
    }

    function loadDevices() {
        return RC.getJSON('/api/device-list')
            .then(function (data) {
                state.devices = data.devices || [];
                state.networks = data.networks || {};
                state.activeNetworkId = data.activeNetworkId || null;
                notify();
                return state;
            })
            .catch(function (err) {
                console.error('Failed to load devices:', err);
                return state;
            });
    }

    function getState() {
        return state;
    }

    /* ── Table ──────────────────────────────────────────────────────── */

    function roleLabel(device) {
        var t = (device.device_type || 'unknown').replace(/_/g, ' ');
        return t.toUpperCase();
    }

    function addressingLabel(addressing) {
        return ADDRESSING_LABELS[addressing || ''] || 'UNKNOWN';
    }

    function portsLabel(device) {
        var open = RC.openPorts(device);
        if (!open.length) return '—';

        var shown = open.slice(0, 3).map(function (p) { return p.number; }).join(', ');
        return open.length > 3 ? shown + ' +' + (open.length - 3) : shown;
    }

    function deviceRow(device) {
        var color = RC.isUnidentified(device) ? 'var(--rc-amber)' : RC.statusColor(device.status);
        var selected = device.id === state.selectedId ? ' is-selected' : '';

        return '<div class="rc-devices__row' + selected + '" data-device-id="' + RC.esc(device.id) + '">' +
            '<span class="rc-cell--addr" style="color:' + color + '">' +
                '<span class="rc-dot" style="background:' + color + '"></span>' +
                (device.is_favorite ? '<span title="Favorite" style="color:var(--rc-amber)">★ </span>' : '') +
                RC.esc(device.ipv4 || '—') +
            '</span>' +
            '<span class="rc-cell--host">' + RC.esc(RC.deviceName(device) || '—') + '</span>' +
            '<span class="rc-cell--vendor">' + RC.esc(device.vendor || '—') + '</span>' +
            '<span class="rc-cell--role">' + RC.esc(roleLabel(device)) + '</span>' +
            '<span class="rc-cell--ports">' + RC.esc(portsLabel(device)) + '</span>' +
            '<span class="rc-cell--seen">' + RC.esc(RC.timeAgo(device.last_seen_online_at || device.updated_at)) + '</span>' +
            '</div>';
    }

    function renderDeviceRows(devices) {
        if (!devices.length) return RC.empty('No devices match');
        return devices.map(deviceRow).join('');
    }

    /* ── Drawer ─────────────────────────────────────────────────────── */

    function openDrawer(deviceId) {
        state.selectedId = deviceId;
        state.tab = 'overview';
        state.history = null;

        // Keep the map's selection marker in step however the drawer was
        // opened — dock row, host table, alert feed or the canvas itself.
        if (window.RCMap) window.RCMap.setSelected(deviceId);

        var shell = RC.el('rc-shell');
        if (shell) shell.classList.add('is-drawer-open');

        syncTabs();
        notify();
        loadDetail(deviceId);
    }

    function closeDrawer() {
        state.selectedId = null;
        state.detail = null;
        if (window.RCMap) window.RCMap.setSelected(null);

        var shell = RC.el('rc-shell');
        if (shell) shell.classList.remove('is-drawer-open');

        notify();
    }

    function loadDetail(deviceId) {
        RC.render(RC.el('rc-d-body'), '<div class="rc-loading">LOADING HOST</div>');

        RC.getJSON('/api/devices/' + encodeURIComponent(deviceId) + '/modal')
            .then(function (data) {
                // Guard against a slow response landing after the user moved on.
                if (state.selectedId !== deviceId) return;
                state.detail = data.device || null;
                renderDrawer();
            })
            .catch(function (err) {
                console.error('Failed to load device:', err);
                if (state.selectedId !== deviceId) return;
                RC.render(RC.el('rc-d-body'), RC.empty('Could not load this host'));
            });
    }

    function syncTabs() {
        var tabs = RC.el('rc-d-tabs');
        if (!tabs) return;

        Array.prototype.forEach.call(tabs.children, function (tab) {
            tab.classList.toggle('is-active', tab.getAttribute('data-tab') === state.tab);
        });
    }

    function renderDrawer() {
        var device = state.detail;
        if (!device) return;

        var color = RC.isUnidentified(device) ? 'var(--rc-amber)' : RC.statusColor(device.status);
        var status = (device.status || 'unknown').toUpperCase();
        var seen = RC.timeAgo(device.last_seen_online_at);

        var dot = RC.el('rc-d-dot');
        if (dot) dot.style.background = color;
        if (dot) dot.style.boxShadow = '0 0 8px ' + color;

        var statusEl = RC.el('rc-d-status');
        if (statusEl) {
            statusEl.textContent = status + (seen !== '—' ? ' · ' + seen : '');
            statusEl.style.color = color;
        }

        RC.text('rc-d-ip', device.ipv4 || '—');
        RC.text('rc-d-sub', [RC.deviceName(device) || 'no hostname', device.vendor || 'unknown vendor'].join(' · '));

        syncFlagButtons(device);
        syncTabs();

        var body = RC.el('rc-d-body');
        if (!body) return;

        if (state.tab === 'overview') {
            RC.render(body, overviewHTML(device));
            bindAddressingCycle(device);
        }
        else if (state.tab === 'ports') RC.render(body, portsHTML(device));
        else if (state.tab === 'history') renderHistory(device);
        else if (state.tab === 'notes') renderNotes(device);
    }

    // Reflects is_favorite/ignored on the two drawer-head flag buttons —
    // same text/color-swap pattern console.js uses for the SHOW OFFLINE toggle.
    function syncFlagButtons(device) {
        var favorite = RC.el('rc-d-favorite');
        if (favorite) {
            favorite.textContent = device.is_favorite ? '★' : '☆';
            favorite.style.color = device.is_favorite ? 'var(--rc-amber)' : '';
        }
        var ignore = RC.el('rc-d-ignore');
        if (ignore) {
            ignore.style.color = device.ignored ? 'var(--rc-red)' : '';
            ignore.style.opacity = device.ignored ? '1' : '.5';
        }
    }

    function toggleFavorite() {
        if (!state.detail) return;
        var device = state.detail;
        RC.sendJSON('PUT', '/api/devices/' + encodeURIComponent(device.id), {
            is_favorite: !device.is_favorite
        }).then(function () {
            device.is_favorite = !device.is_favorite;
            syncFlagButtons(device);
            loadDevices();
        }).catch(function (err) {
            console.error('Failed to toggle favorite:', err);
        });
    }

    function toggleIgnore() {
        if (!state.detail) return;
        var device = state.detail;
        RC.sendJSON('PUT', '/api/devices/' + encodeURIComponent(device.id), {
            ignored: !device.ignored
        }).then(function () {
            device.ignored = !device.ignored;
            syncFlagButtons(device);
            loadDevices();
        }).catch(function (err) {
            console.error('Failed to toggle ignore:', err);
        });
    }

    function bindAddressingCycle(device) {
        var el = RC.el('rc-d-addressing');
        if (!el) return;

        el.addEventListener('click', function () {
            var next = ADDRESSING_CYCLE[device.addressing || ''];
            RC.sendJSON('PUT', '/api/devices/' + encodeURIComponent(device.id), {
                addressing: next
            }).then(function () {
                device.addressing = next;
                el.textContent = addressingLabel(next);
                loadDevices();
            }).catch(function (err) {
                console.error('Failed to update addressing:', err);
            });
        });
    }

    function overviewHTML(device) {
        var os = device.os ? [device.os.name, device.os.version].filter(Boolean).join(' ') : '';
        var open = RC.openPorts(device);

        var rows = [
            ['MAC', device.mac || '—'],
            ['VENDOR', device.vendor || '—'],
            ['SUBNET', state.networks[device.network_id] || '—'],
            ['ROLE', roleLabel(device)],
            ['OS GUESS', os || 'no fingerprint'],
            ['FIRST SEEN', RC.dateTime(device.created_at)],
            ['LAST SEEN', RC.timeAgo(device.last_seen_online_at) + ' ago']
        ];

        var html = '<dl class="rc-kv">' + rows.map(function (r) {
            return '<dt>' + RC.esc(r[0]) + '</dt><dd>' + RC.esc(r[1]) + '</dd>';
        }).join('') +
            '<dt>ADDRESSING</dt><dd><span id="rc-d-addressing" style="cursor:pointer;border-bottom:1px dotted var(--rc-text-4)" title="Click to change">' +
                RC.esc(addressingLabel(device.addressing)) + '</span></dd>' +
        '</dl>';

        html += '<div class="rc-section"><span>OPEN PORTS</span><span style="color:var(--rc-text-5)">' +
            open.length + ' FOUND</span></div>';
        html += open.length ? portRows(open) : RC.empty('No open ports recorded');

        var services = device.web_services || [];
        if (services.length) {
            html += '<div class="rc-section"><span>WEB SERVICES</span></div>';
            html += '<div class="rc-ports">' + services.map(function (s) {
                return '<div class="rc-port">' +
                    '<span class="rc-port__num">' + RC.esc(String(s.port || '')) + '</span>' +
                    '<span class="rc-port__svc">' + RC.esc(s.title || s.url || '') + '</span>' +
                    '<a class="rc-port__detail" href="' + RC.esc(s.url || '#') + '" target="_blank" rel="noreferrer noopener">↗</a>' +
                    '</div>';
            }).join('') + '</div>';
        }

        return html;
    }

    function portRows(ports) {
        return '<div class="rc-ports">' + ports.map(function (p) {
            var risky = RISKY_PORTS.indexOf(String(p.number)) !== -1;
            return '<div class="rc-port">' +
                '<span class="rc-port__num' + (risky ? ' is-risky' : '') + '">' + RC.esc(p.number) + '</span>' +
                '<span class="rc-port__svc">' + RC.esc(p.service || 'unknown') + '</span>' +
                '<span class="rc-port__detail">' + RC.esc(p.protocol || 'tcp') + '</span>' +
                '</div>';
        }).join('') + '</div>';
    }

    function portsHTML(device) {
        var open = RC.openPorts(device);
        if (!open.length) return RC.empty('No open ports recorded');

        return '<div style="padding:14px 16px">' +
            '<div class="rc-section" style="padding:0 0 8px">' +
                '<span>' + open.length + ' OPEN</span>' +
                '<span style="color:var(--rc-text-5)">' + RC.esc(scanStamp(device)) + '</span>' +
            '</div>' + portRows(open) + '</div>';
    }

    function scanStamp(device) {
        if (!device.port_scan_ended_at) return 'never scanned';
        return 'scanned ' + RC.timeAgo(device.port_scan_ended_at) + ' ago';
    }

    function renderHistory(device) {
        var body = RC.el('rc-d-body');

        if (state.history) {
            RC.render(body, historyHTML(state.history));
            return;
        }

        RC.render(body, '<div class="rc-loading">LOADING HISTORY</div>');
        RC.getJSON('/api/devices/' + encodeURIComponent(device.id) + '/events')
            .then(function (data) {
                if (state.selectedId !== device.id || state.tab !== 'history') return;
                state.history = data.logs || [];
                RC.render(RC.el('rc-d-body'), historyHTML(state.history));
            })
            .catch(function (err) {
                console.error('Failed to load device history:', err);
                if (state.selectedId !== device.id) return;
                RC.render(RC.el('rc-d-body'), RC.empty('Could not load history'));
            });
    }

    function historyHTML(logs) {
        if (!logs.length) return RC.empty('Nothing recorded for this host yet');

        return '<div class="rc-history">' + logs.map(function (log) {
            var mark = eventMark(log.type);
            return '<div class="rc-history__row">' +
                '<span class="rc-history__when">' + RC.esc(RC.dateTime(log.created_at)) + '</span>' +
                '<span style="flex:none;width:8px;color:' + mark.color + '">' + mark.glyph + '</span>' +
                '<span class="rc-history__text">' + RC.esc(log.description || log.type || '') + '</span>' +
                '</div>';
        }).join('') + '</div>';
    }

    // EEventLogType values are human strings ("Ping sweep", "Device online"),
    // not slugs — match on those, not on snake_case that never appears.
    function eventMark(type) {
        switch (type) {
            case 'Device online':
                return { glyph: '+', color: 'var(--rc-accent)' };
            case 'Device became idle':
                return { glyph: '~', color: 'var(--rc-amber)' };
            case 'Device is now offline':
                return { glyph: '−', color: 'var(--rc-text-4)' };
            case 'Device deleted':
                return { glyph: '✕', color: 'var(--rc-red)' };
            case 'Port scan started':
                return { glyph: '›', color: 'var(--rc-text-4)' };
            case 'Port scan completed':
            case 'Ping sweep':
                return { glyph: '✓', color: 'var(--rc-text-4)' };
            case 'Warning':
            case 'Alert':
                return { glyph: '▲', color: 'var(--rc-amber)' };
            default:
                return { glyph: '●', color: 'var(--rc-text-4)' };
        }
    }

    function renderNotes(device) {
        var note = device.comment || '';

        RC.render(RC.el('rc-d-body'),
            '<div class="rc-notes">' +
                '<div class="rc-field">' +
                    '<label for="rc-d-note">Operator notes</label>' +
                    '<textarea id="rc-d-note" placeholder="What is this host, and what should the next person know about it?">' +
                        RC.esc(note) +
                    '</textarea>' +
                '</div>' +
                '<div style="display:flex;gap:8px">' +
                    '<button class="rc-btn rc-btn--solid" id="rc-d-note-save">SAVE NOTE</button>' +
                    '<span id="rc-d-note-status" style="align-self:center;font-size:10px;letter-spacing:.12em;color:var(--rc-text-5)"></span>' +
                '</div>' +
            '</div>');

        var save = RC.el('rc-d-note-save');
        if (!save) return;

        save.addEventListener('click', function () {
            var field = RC.el('rc-d-note');
            if (!field) return;

            RC.text('rc-d-note-status', 'SAVING…');
            RC.sendJSON('PUT', '/api/devices/' + encodeURIComponent(device.id), {
                name: device.name || '',
                comment: field.value
            }).then(function () {
                device.comment = field.value;
                RC.text('rc-d-note-status', 'SAVED');
                loadDevices();
            }).catch(function (err) {
                console.error('Failed to save note:', err);
                RC.text('rc-d-note-status', 'FAILED');
            });
        });
    }

    /* ── Drawer footer actions ──────────────────────────────────────── */

    function rescanHost() {
        if (!state.detail) return;
        var device = state.detail;
        var btn = RC.el('rc-d-rescan');

        if (btn) {
            btn.textContent = 'QUEUEING…';
            btn.disabled = true;
        }

        RC.sendJSON('POST', '/api/devices/' + encodeURIComponent(device.id) + '/rescan')
            .then(function () {
                if (btn) btn.textContent = 'SCAN QUEUED';
            })
            .catch(function (err) {
                console.error('Rescan failed:', err);
                if (btn) btn.textContent = 'RESCAN FAILED';
            })
            .then(function () {
                setTimeout(function () {
                    if (btn) {
                        btn.textContent = 'RESCAN HOST';
                        btn.disabled = false;
                    }
                }, 2500);
            });
    }

    function renameHost() {
        if (!state.detail) return;
        var device = state.detail;

        openDialog({
            title: 'RENAME HOST',
            body: '<div class="rc-field">' +
                    '<label for="rc-rename-input">Display name for ' + RC.esc(device.ipv4) + '</label>' +
                    '<input id="rc-rename-input" value="' + RC.esc(device.name || '') + '" placeholder="' +
                        RC.esc(device.hostname || 'e.g. nas-vault') + '">' +
                  '</div>',
            confirm: 'SAVE',
            onConfirm: function () {
                var field = RC.el('rc-rename-input');
                if (!field) return;

                RC.sendJSON('PUT', '/api/devices/' + encodeURIComponent(device.id), {
                    name: field.value,
                    comment: device.comment || ''
                }).then(function () {
                    device.name = field.value;
                    closeDialog();
                    renderDrawer();
                    loadDevices();
                }).catch(function (err) {
                    console.error('Rename failed:', err);
                    failDialog('RENAME FAILED', 'The host could not be renamed. ' + err.message);
                });
            }
        });
    }

    function deleteHost() {
        if (!state.detail) return;
        var device = state.detail;

        openDialog({
            title: 'DELETE HOST',
            body: '<div style="font-size:12px;color:var(--rc-text-3);line-height:1.6">' +
                'Remove <strong style="color:var(--rc-text)">' + RC.esc(device.ipv4) + '</strong> and its ' +
                'scan history. If the host is still on the network the next sweep will find it again.' +
                '</div>',
            confirm: 'DELETE',
            danger: true,
            onConfirm: function () {
                fetch('/api/devices/' + encodeURIComponent(device.id), {
                    method: 'DELETE',
                    credentials: 'include'
                }).then(function (r) {
                    if (!r.ok) throw new Error(r.status + ' ' + r.statusText);
                    closeDialog();
                    closeDrawer();
                    loadDevices();
                }).catch(function (err) {
                    console.error('Delete failed:', err);
                    failDialog('DELETE FAILED', 'The host could not be deleted. ' + err.message);
                });
            }
        });
    }

    function bindDrawer() {
        var close = RC.el('rc-d-close');
        if (close) close.addEventListener('click', closeDrawer);

        var favorite = RC.el('rc-d-favorite');
        if (favorite) favorite.addEventListener('click', toggleFavorite);

        var ignore = RC.el('rc-d-ignore');
        if (ignore) ignore.addEventListener('click', toggleIgnore);

        var tabs = RC.el('rc-d-tabs');
        if (tabs) {
            tabs.addEventListener('click', function (e) {
                var tab = e.target.closest('[data-tab]');
                if (!tab) return;
                state.tab = tab.getAttribute('data-tab');
                renderDrawer();
            });
        }

        var rescan = RC.el('rc-d-rescan');
        if (rescan) rescan.addEventListener('click', rescanHost);

        var rename = RC.el('rc-d-rename');
        if (rename) rename.addEventListener('click', renameHost);

        var del = RC.el('rc-d-delete');
        if (del) del.addEventListener('click', deleteHost);

        document.addEventListener('keydown', function (e) {
            if (e.key !== 'Escape') return;
            var dialog = RC.el('rc-dialog');
            if (dialog && dialog.classList.contains('is-open')) return;
            if (state.selectedId) closeDrawer();
        });
    }

    document.addEventListener('DOMContentLoaded', bindDrawer);

    window.RCDevices = {
        load: loadDevices,
        state: getState,
        onDevices: onDevices,
        renderRows: renderDeviceRows,
        roleLabel: roleLabel,
        openDrawer: openDrawer,
        closeDrawer: closeDrawer
    };
})();
