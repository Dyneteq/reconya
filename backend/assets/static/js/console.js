/* Console shell.
 *
 * Owns the top bar, the map overlays, the device dock, the feed tabs and the
 * polling schedule. Every page renders inside the same chrome, so this runs on
 * all of them and only the view-specific wiring is conditional.
 *
 * Poll cadences are the ones the previous UI had settled on — devices and
 * metrics every 15s, scan status every 5s, events every 20s, internet ping
 * every 3s — plus alerts on the device cadence.
 */
(function () {
    'use strict';

    var POLL = {
        devices: 15000,
        metrics: 15000,
        scan: 5000,
        events: 20000,
        alerts: 15000,
        ping: 3000
    };

    var page = 'dashboard';
    var isMap = false;
    var feed = 'alerts';
    var events = [];
    var metrics = null;
    var internet = { up: null, ip: '' };

    function start() {
        var shell = RC.el('rc-shell');
        page = (shell && shell.getAttribute('data-page')) || 'dashboard';
        isMap = !RC.el('rc-hosts-page') && !RC.el('rc-networks-page') &&
                !RC.el('rc-logs-page') && !RC.el('rc-settings-page') && !RC.el('rc-alerts-page');

        bindUserMenu();
        bindNewNetwork();
        bindAlertTile();

        // Shared across every page: the top bar is always present.
        window.RCDevices.onDevices(onDevices);
        window.RCAlerts.onAlerts(renderUnidentifiedMetric);

        window.RCDevices.load();
        window.RCScan.load();
        window.RCAlerts.load();
        loadMetrics();
        pingInternet();
        window.RCSuggestions.init();

        if (isMap) {
            window.RCSegments.load().then(renderSubnetsFromState);
            window.RCSegments.onScopeChange(function () {
                renderDock(window.RCDevices.state());
            });
        }

        every(POLL.devices, function () { window.RCDevices.load(); });
        every(POLL.scan, function () { window.RCScan.poll(); });
        every(POLL.alerts, function () { window.RCAlerts.load(); });
        every(POLL.metrics, loadMetrics);
        every(POLL.metrics, function () {
            if (isMap) window.RCSegments.load().then(renderSubnetsFromState);
        });
        every(POLL.ping, pingInternet);

        if (isMap) startMapView();
        if (page === 'devices') startHostsPage();
        if (page === 'networks') window.RCNetworks.load();
        if (page === 'logs') startLogsPage();
        if (page === 'settings') window.RCSettings.load();
    }

    function every(ms, fn) {
        setInterval(function () {
            try { fn(); } catch (e) { console.error('poll failed', e); }
        }, ms);
    }

    /* ── Top bar ────────────────────────────────────────────────────── */

    function loadMetrics() {
        return RC.getJSON('/api/dashboard-metrics')
            .then(function (data) {
                metrics = data;
                renderMetrics();
                renderNetSummary();
            })
            .catch(function (err) {
                console.error('Failed to load metrics:', err);
            });
    }

    function renderMetrics() {
        if (!metrics) return;

        RC.text('rc-m-online', metrics.devicesOnline != null ? metrics.devicesOnline : 0);
        RC.text('rc-m-total', '/ ' + (metrics.devicesFound != null ? metrics.devicesFound : 0));
        // saturation is a bare number from the API, so the unit belongs here.
        RC.text('rc-m-saturation', metrics.saturation != null
            ? metrics.saturation + '% of range in use'
            : (metrics.devicesOffline || 0) + ' offline');
    }

    function renderNetSummary() {
        var networkCount = Object.keys(window.RCDevices.state().networks || {}).length;
        var parts = [];

        if (metrics && metrics.networkRange) parts.push(metrics.networkRange);
        if (networkCount) parts.push(networkCount + (networkCount === 1 ? ' network' : ' networks'));

        // The public IP is only known when PUBLIC_IP_LOOKUP_ENABLED is on — that
        // lookup is an outbound call and off by default. Reachability is always
        // known, so say that rather than showing an empty address.
        var ip = (metrics && metrics.publicIP) || internet.ip;
        var wan;
        if (internet.up === false) {
            wan = '<span style="color:var(--rc-red)">▼ WAN DOWN</span>';
        } else if (ip) {
            wan = 'WAN ' + RC.esc(ip) + ' <span style="color:var(--rc-accent)">▲</span>';
        } else if (internet.up) {
            wan = '<span style="color:var(--rc-accent)">▲ WAN UP</span>';
        } else {
            wan = 'WAN <span style="color:var(--rc-text-5)">—</span>';
        }

        RC.render(RC.el('rc-net-summary'),
            (parts.length ? RC.esc(parts.join(' · ')) + ' · ' : '') + wan);
    }

    function pingInternet() {
        return RC.getJSON('/api/ping-internet')
            .then(function (data) {
                internet.up = !!data.up;
                renderNetSummary();
            })
            .catch(function () {
                internet.up = false;
                renderNetSummary();
            });
    }

    // "New in 24h" is computed here rather than served: the device list already
    // carries created_at, so an extra endpoint would only restate it.
    function renderNewMetric(state) {
        var cutoff = Date.now() - 24 * 60 * 60 * 1000;
        var fresh = state.devices.filter(function (d) {
            var created = new Date(d.created_at).getTime();
            return !isNaN(created) && created >= cutoff;
        });

        RC.text('rc-m-new', fresh.length);
        RC.text('rc-m-new-sub', fresh.length
            ? 'newest ' + RC.timeAgo(fresh.map(function (d) { return d.created_at; }).sort().pop()) + ' ago'
            : 'nothing new today');
    }

    function renderUnidentifiedMetric() {
        var alerts = window.RCAlerts.state().alerts || [];
        var open = alerts.filter(function (a) {
            return a.rule_id === 'unidentified_host' && a.state === 'open';
        });

        RC.text('rc-m-unknown', open.length);
        RC.text('rc-m-unknown-sub', open.length
            ? 'oldest ' + RC.timeAgo(open.map(function (a) { return a.first_seen_at; }).sort()[0]) + ' ago'
            : 'every host accounted for');
    }

    /* ── Device data fan-out ────────────────────────────────────────── */

    function onDevices(state) {
        renderNewMetric(state);
        renderNetSummary();

        if (isMap) {
            window.RCMap.setData(state.devices, state.networks);
            renderDock(state);
            window.RCSegments.render();
            renderSubnetsFromState(state);
        }
        if (page === 'devices') renderHostsPage(state);
    }

    /* ── Map view ───────────────────────────────────────────────────── */

    function startMapView() {
        window.RCMap.init({
            onSelect: function (deviceId) {
                window.RCDevices.openDrawer(deviceId);
            }
        });

        renderModeMenu();
        bindMapControls();
        bindDock();
        bindFeedTabs();
        bindScanPlan();

        loadEvents();
        every(POLL.events, loadEvents);

        syncOfflineToggle();
        syncIgnoredToggle();
    }

    function renderModeMenu() {
        var menu = RC.el('rc-mode-menu');
        if (!menu) return;

        var current = window.RCMap.mode();

        RC.render(menu, window.RCMap.modes().map(function (m) {
            return '<div class="rc-mode__opt' + (m.id === current ? ' is-active' : '') +
                '" data-mode="' + RC.esc(m.id) + '">' +
                '<div class="rc-mode__opt-row">' +
                    '<span class="rc-mode__opt-label">' + RC.esc(m.id) + '</span>' +
                    '<span style="font-size:10px;color:var(--rc-accent)">' + (m.id === current ? '●' : '') + '</span>' +
                '</div>' +
                '<div class="rc-mode__opt-hint">' + RC.esc(m.hint) + '</div>' +
                '</div>';
        }).join(''));

        RC.text('rc-mode-label', 'MAP · ' + current);

        var hint = window.RCMap.modes().filter(function (m) { return m.id === current; })[0];
        RC.text('rc-mode-hint', hint ? hint.hint : '');
    }

    function bindMapControls() {
        var btn = RC.el('rc-mode-btn');
        var menu = RC.el('rc-mode-menu');

        if (btn && menu) {
            btn.addEventListener('click', function (e) {
                e.stopPropagation();
                var open = menu.style.display !== 'none';
                menu.style.display = open ? 'none' : 'block';
                RC.text('rc-mode-caret', open ? '▼' : '▲');
            });

            document.addEventListener('click', function () {
                menu.style.display = 'none';
                RC.text('rc-mode-caret', '▼');
            });

            RC.on(menu, 'click', '[data-mode]', function (e, opt) {
                window.RCMap.setMode(opt.getAttribute('data-mode'));
                renderModeMenu();
                menu.style.display = 'none';
                RC.text('rc-mode-caret', '▼');
            });
        }

        var filter = RC.el('rc-map-filter');
        if (filter) {
            filter.addEventListener('input', function () {
                window.RCMap.setFilter(filter.value);
                var state = window.RCDevices.state();
                window.RCMap.setData(state.devices, state.networks);
            });
        }

        var zoomIn = RC.el('rc-zoom-in');
        if (zoomIn) zoomIn.addEventListener('click', function () { window.RCMap.zoomIn(); });

        var zoomOut = RC.el('rc-zoom-out');
        if (zoomOut) zoomOut.addEventListener('click', function () { window.RCMap.zoomOut(); });

        var zoomFit = RC.el('rc-zoom-fit');
        if (zoomFit) zoomFit.addEventListener('click', function () { window.RCMap.fit(); });
    }

    // The SCAN PLAN list: one row per range (not per network — a network can
    // own several), showing known/online hosts against the range's real
    // address capacity and a running total across all active ranges. Each row
    // toggles the range in or out of future scans.
    function renderSubnetsFromState(state) {
        var container = RC.el('rc-subnets');
        if (!container) return;

        var byIP = {};
        (state.devices || []).forEach(function (d) { byIP[d.ipv4] = d; });

        var runningTotal = 0;
        var rows = [];

        window.RCSegments.networks().forEach(function (n) {
            (n.ranges || []).forEach(function (r) {
                var known = 0, online = 0;
                Object.keys(byIP).forEach(function (ip) {
                    if (!RC.ipInCidr(ip, r.cidr)) return;
                    known++;
                    if ((byIP[ip].status || '').toLowerCase() === 'online') online++;
                });

                var capacity = RC.cidrSize(r.cidr);
                if (r.active) runningTotal += capacity;

                var pct = capacity ? Math.round((known / capacity) * 100) : 0;
                var color = !r.active ? 'var(--rc-text-5)'
                    : (pct >= 60 ? 'var(--rc-accent)' : (pct >= 20 ? 'rgba(74,222,128,.6)' : 'var(--rc-amber)'));

                rows.push('<div data-network-id="' + RC.esc(n.id) + '" data-range-id="' + RC.esc(r.id) +
                        '" style="cursor:pointer' + (r.active ? '' : ';opacity:.45') + '" title="click to ' +
                        (r.active ? 'exclude from' : 'include in') + ' scans">' +
                    '<div class="rc-subnet__row">' +
                        '<span class="rc-subnet__name">' + RC.esc(r.label || r.cidr) + '</span>' +
                        '<span class="rc-subnet__count">' + known + ' known / ' + capacity + '</span>' +
                    '</div>' +
                    '<div class="rc-subnet__track"><div class="rc-subnet__fill" style="width:' +
                        pct + '%;background:' + color + '"></div></div>' +
                    '</div>');
            });
        });

        RC.render(container, rows.length
            ? rows.join('') + '<div style="font-size:10px;color:var(--rc-text-5);padding-top:4px">' +
                runningTotal + ' addresses in active scan plan</div>'
            : '<div style="font-size:11px;color:var(--rc-text-5)">No ranges configured yet</div>');
    }

    function bindScanPlan() {
        RC.on(RC.el('rc-subnets'), 'click', '[data-range-id]', function (e, row) {
            var networkId = row.getAttribute('data-network-id');
            var rangeId = row.getAttribute('data-range-id');
            RC.sendJSON('POST', '/api/networks/' + encodeURIComponent(networkId) +
                '/ranges/' + encodeURIComponent(rangeId) + '/toggle')
                .then(function () { return window.RCSegments.load(); })
                .then(function () { renderSubnetsFromState(window.RCDevices.state()); })
                .catch(function (err) { console.error('Failed to toggle range:', err); });
        });
    }

    /* ── Dock ───────────────────────────────────────────────────────── */

    function visibleDevices(state) {
        var devices = window.RCSegments.filterDevices(state.devices);
        var showOffline = window.RCMap.showOffline();
        var showIgnored = window.RCMap.showIgnored();

        return devices.filter(function (d) {
            // Favorite devices stay visible even when offline and the global
            // toggle is off.
            if (!showOffline && !d.is_favorite && (d.status || '').toLowerCase() === 'offline') return false;
            if (!showIgnored && d.ignored) return false;
            return true;
        });
    }

    function renderDock(state) {
        var rows = visibleDevices(state).slice().sort(bySeen);
        RC.render(RC.el('rc-dock-devices'), window.RCDevices.renderRows(rows));

        var online = state.devices.filter(function (d) {
            return (d.status || '').toLowerCase() === 'online';
        }).length;
        RC.text('rc-dock-meta', state.devices.length + ' KNOWN · ' + online + ' ONLINE');
    }

    function bySeen(a, b) {
        var ta = new Date(a.last_seen_online_at || a.updated_at).getTime() || 0;
        var tb = new Date(b.last_seen_online_at || b.updated_at).getTime() || 0;
        return tb - ta;
    }

    function bindDock() {
        RC.on(RC.el('rc-dock-devices'), 'click', '[data-device-id]', function (e, row) {
            var id = row.getAttribute('data-device-id');
            window.RCMap.setSelected(id);
            window.RCDevices.openDrawer(id);
        });

        var toggle = RC.el('rc-offline-toggle');
        if (toggle) {
            toggle.addEventListener('click', function () {
                window.RCMap.setShowOffline(!window.RCMap.showOffline());
                syncOfflineToggle();

                var state = window.RCDevices.state();
                window.RCMap.setData(state.devices, state.networks);
                renderDock(state);
            });
        }

        var ignoredToggle = RC.el('rc-ignored-toggle');
        if (ignoredToggle) {
            ignoredToggle.addEventListener('click', function () {
                window.RCMap.setShowIgnored(!window.RCMap.showIgnored());
                syncIgnoredToggle();

                var state = window.RCDevices.state();
                window.RCMap.setData(state.devices, state.networks);
                renderDock(state);
            });
        }
    }

    function syncOfflineToggle() {
        var on = window.RCMap.showOffline();
        var toggle = RC.el('rc-offline-toggle');
        if (!toggle) return;

        toggle.textContent = 'SHOW OFFLINE: ' + (on ? 'ON' : 'OFF');
        toggle.style.color = on ? 'var(--rc-accent)' : 'var(--rc-text-4)';
    }

    function syncIgnoredToggle() {
        var on = window.RCMap.showIgnored();
        var toggle = RC.el('rc-ignored-toggle');
        if (!toggle) return;

        toggle.textContent = 'SHOW IGNORED: ' + (on ? 'ON' : 'OFF');
        toggle.style.color = on ? 'var(--rc-accent)' : 'var(--rc-text-4)';
    }

    /* ── Feed ───────────────────────────────────────────────────────── */

    function bindFeedTabs() {
        ['rc-tab-alerts', 'rc-tab-events'].forEach(function (id) {
            var tab = RC.el(id);
            if (!tab) return;
            tab.addEventListener('click', function () {
                setFeed(tab.getAttribute('data-feed'));
            });
        });

    }

    // Bound on every page, not just the map: the tile is part of the top bar,
    // so it has to do something sensible from HOSTS or CONF too.
    function bindAlertTile() {
        var tile = RC.el('rc-alert-tile');
        if (!tile) return;

        tile.addEventListener('click', function () {
            if (isMap) {
                setFeed('alerts');
                window.RCAlerts.setFilter('ALL');
            } else if (page !== 'alerts') {
                window.location.href = '/alerts';
            }
        });
    }

    function setFeed(next) {
        feed = next;

        var alertsTab = RC.el('rc-tab-alerts');
        var eventsTab = RC.el('rc-tab-events');
        if (alertsTab) alertsTab.classList.toggle('is-active', feed === 'alerts');
        if (eventsTab) eventsTab.classList.toggle('is-active', feed === 'events');

        var filters = RC.el('rc-alert-filters');
        if (filters) filters.style.display = feed === 'alerts' ? 'flex' : 'none';

        var alertsPane = RC.el('rc-feed-alerts');
        if (alertsPane) alertsPane.style.display = feed === 'alerts' ? 'block' : 'none';

        var eventsPane = RC.el('rc-feed-events');
        if (eventsPane) eventsPane.style.display = feed === 'events' ? 'block' : 'none';
    }

    function loadEvents() {
        return RC.getJSON('/api/event-logs')
            .then(function (data) {
                events = collapse(data.logs || []);
                renderEvents();
            })
            .catch(function (err) {
                console.error('Failed to load events:', err);
            });
    }

    // A running sweep emits the same event type over and over; collapsing runs
    // of identical types into "type ×N" keeps a 20-row feed informative instead
    // of showing twenty identical port-scan lines.
    function collapse(logs) {
        var out = [];

        logs.forEach(function (log) {
            var last = out[out.length - 1];
            if (last && last.type === log.type && last.description === log.description) {
                last.count++;
                return;
            }
            out.push({
                type: log.type,
                description: log.description,
                created_at: log.created_at,
                count: 1
            });
        });

        return out;
    }

    function renderEvents() {
        var container = RC.el('rc-feed-events');
        if (!container) return;

        if (!events.length) {
            RC.render(container, RC.empty('No activity recorded yet'));
            return;
        }

        RC.render(container, '<div class="rc-events">' + events.map(function (e) {
            var mark = eventMark(e.type);
            return '<div class="rc-event">' +
                '<span class="rc-event__time">' + RC.esc(RC.clockTime(e.created_at)) + '</span>' +
                '<span class="rc-event__mark" style="color:' + mark.color + '">' + mark.glyph + '</span>' +
                '<span class="rc-event__text">' + RC.esc(e.description || e.type) +
                    (e.count > 1 ? ' ×' + e.count : '') + '</span>' +
                '</div>';
        }).join('') + '</div>');
    }

    function eventMark(type) {
        switch (type) {
            case 'Device online': return { glyph: '+', color: 'var(--rc-accent)' };
            case 'Device became idle': return { glyph: '~', color: 'var(--rc-amber)' };
            case 'Device is now offline': return { glyph: '−', color: 'var(--rc-text-4)' };
            case 'Device deleted': return { glyph: '✕', color: 'var(--rc-red)' };
            case 'Ping sweep':
            case 'Port scan completed': return { glyph: '✓', color: 'var(--rc-text-4)' };
            case 'Warning':
            case 'Alert': return { glyph: '▲', color: 'var(--rc-amber)' };
            default: return { glyph: '●', color: 'var(--rc-text-4)' };
        }
    }

    /* ── HOSTS page ─────────────────────────────────────────────────── */

    var hostFilter = '';

    function startHostsPage() {
        var search = RC.el('rc-host-search');
        if (search) {
            search.addEventListener('input', function () {
                hostFilter = search.value.toLowerCase().trim();
                renderHostsPage(window.RCDevices.state());
            });
        }

        RC.on(RC.el('rc-hosts-page'), 'click', '[data-device-id]', function (e, row) {
            window.RCDevices.openDrawer(row.getAttribute('data-device-id'));
        });
    }

    function renderHostsPage(state) {
        var container = RC.el('rc-hosts-page');
        if (!container) return;

        var rows = state.devices.filter(function (d) {
            if (!hostFilter) return true;
            return [d.ipv4, d.name, d.hostname, d.mac, d.vendor, state.networks[d.network_id]]
                .join(' ').toLowerCase().indexOf(hostFilter) !== -1;
        }).sort(bySeen);

        var online = state.devices.filter(function (d) {
            return (d.status || '').toLowerCase() === 'online';
        }).length;
        RC.text('rc-hosts-count', state.devices.length + ' KNOWN · ' + online + ' ONLINE');

        RC.render(container,
            '<div class="rc-devices" style="border:1px solid var(--rc-line-10)">' +
                '<div class="rc-devices__head">' +
                    '<span>ADDRESS</span><span>HOSTNAME</span><span>VENDOR</span>' +
                    '<span>ROLE</span><span>PORTS</span><span class="rc-cell--right">SEEN</span>' +
                '</div>' +
                window.RCDevices.renderRows(rows) +
            '</div>');
    }

    /* ── LOG page ───────────────────────────────────────────────────── */

    function startLogsPage() {
        loadLogsPage();
        every(POLL.events, loadLogsPage);
    }

    function loadLogsPage() {
        return RC.getJSON('/api/event-logs-table')
            .then(function (data) {
                renderLogsPage(data.eventLogs || data.logs || []);
            })
            .catch(function (err) {
                console.error('Failed to load activity:', err);
                RC.render(RC.el('rc-logs-page'), RC.empty('Could not load activity'));
            });
    }

    function renderLogsPage(logs) {
        var container = RC.el('rc-logs-page');
        if (!container) return;

        if (!logs.length) {
            RC.render(container, RC.empty('No activity recorded yet'));
            return;
        }

        var rows = logs.map(function (log) {
            var mark = eventMark(log.type);
            return '<tr>' +
                '<td style="white-space:nowrap;color:var(--rc-text-5)">' + RC.esc(RC.dateTime(log.created_at)) + '</td>' +
                '<td style="color:' + mark.color + ';white-space:nowrap">' + mark.glyph + ' ' + RC.esc(log.type || '') + '</td>' +
                '<td style="color:var(--rc-text-2)">' + RC.esc(log.description || '') + '</td>' +
                '<td style="text-align:right;color:var(--rc-text-4)">' +
                    RC.esc(formatDuration(log.duration_seconds)) + '</td>' +
                '</tr>';
        }).join('');

        RC.render(container,
            '<div class="rc-table__scroll"><table class="rc-table">' +
            '<thead><tr><th>TIME</th><th>TYPE</th><th>EVENT</th>' +
            '<th style="text-align:right">DURATION</th></tr></thead>' +
            '<tbody>' + rows + '</tbody></table></div>');
    }

    function formatDuration(seconds) {
        if (seconds == null) return '—';
        if (seconds < 60) return seconds.toFixed(1) + 's';
        return Math.floor(seconds / 60) + 'm ' + Math.round(seconds % 60) + 's';
    }

    /* ── Rail user menu & top bar actions ───────────────────────────── */

    function bindUserMenu() {
        var btn = RC.el('rc-user-btn');
        var menu = RC.el('rc-usermenu');
        if (!btn || !menu) return;

        btn.addEventListener('click', function (e) {
            e.stopPropagation();
            menu.classList.toggle('is-open');
        });

        document.addEventListener('click', function (e) {
            if (!e.target.closest('#rc-usermenu')) menu.classList.remove('is-open');
        });

        var about = RC.el('rc-about-btn');
        if (about) about.addEventListener('click', showAbout);
    }

    function showAbout() {
        RC.getJSON('/api/about').catch(function () { return {}; }).then(function (data) {
            openDialog({
                title: 'ABOUT RECONYA',
                cancel: 'CLOSE',
                body: '<div style="font-size:12px;line-height:1.7;color:var(--rc-text-3)">' +
                    '<div style="font-family:var(--rc-display);font-size:16px;font-weight:800;color:var(--rc-accent);margin-bottom:4px">reconYa</div>' +
                    '<div style="margin-bottom:14px">Version ' + RC.esc(data.version || 'dev') + '</div>' +
                    '<p style="margin:0 0 12px">Network reconnaissance for your own network. Discovers hosts, ' +
                    'fingerprints what they run, and raises alerts when something changes.</p>' +
                    '<p style="margin:0;color:var(--rc-text-4)">Scan data stays on this machine. Outbound lookups ' +
                    '(public IP, geolocation, vendor, OUI) are each opt-in via the environment.</p>' +
                    '</div>'
            });
        });
    }

    function bindNewNetwork() {
        var btn = RC.el('rc-new-network');
        if (btn) btn.addEventListener('click', function () { openNetworkDialog(); });
    }

    document.addEventListener('DOMContentLoaded', start);
})();
