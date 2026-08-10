/* Alerts: the feed panel, the header tile, and the critical-alert toast.
 *
 * Everything here is backed by /api/alerts. Acknowledgement is a server-side
 * state change, so it survives a reload and is shared between browsers rather
 * than living in one machine's localStorage.
 */
(function () {
    'use strict';

    var state = {
        alerts: [],
        counts: { critical: 0, warning: 0, notice: 0, info: 0 },
        openCount: 0,
        filter: 'ALL',
        // Toasts the operator dismissed this session. Dismissing is not the same
        // as acknowledging: the alert stays open in the feed, it just stops
        // interrupting.
        dismissed: {},
        listeners: []
    };

    function onAlerts(fn) {
        state.listeners.push(fn);
    }

    function notify() {
        state.listeners.forEach(function (fn) {
            try { fn(state); } catch (e) { console.error('alert listener failed', e); }
        });
    }

    function loadAlerts() {
        return RC.getJSON('/api/alerts')
            .then(function (data) {
                state.alerts = data.alerts || [];
                state.counts = data.counts || state.counts;
                state.openCount = data.open_count || 0;
                renderAll();
                notify();
                return state;
            })
            .catch(function (err) {
                console.error('Failed to load alerts:', err);
                return state;
            });
    }

    /* ── Header tile ────────────────────────────────────────────────── */

    function renderTile() {
        var tile = RC.el('rc-alert-tile');
        if (!tile) return;

        var critical = state.counts.critical > 0;
        tile.classList.toggle('is-critical', critical);

        var accent = critical ? 'var(--rc-red)'
            : (state.counts.warning > 0 ? 'var(--rc-amber)' : 'var(--rc-accent)');

        var count = RC.el('rc-alert-count');
        if (count) {
            count.textContent = state.openCount;
            count.style.color = accent;
        }

        var icon = RC.el('rc-alert-icon');
        if (icon) icon.style.color = accent;

        var shorts = RC.el('rc-alert-shorts');
        if (shorts) {
            RC.render(shorts, ['critical', 'warning', 'notice'].map(function (name) {
                var meta = RC.severity(name);
                return '<span style="color:' + meta.color + '">' +
                    state.counts[name] + meta.short + '</span>';
            }).join(''));
        }

        var badge = RC.el('rc-feed-badge');
        if (badge) {
            badge.textContent = state.openCount;
            badge.style.background = accent;
        }
    }

    /* ── Filters ────────────────────────────────────────────────────── */

    function renderFilters(container) {
        if (!container) return;

        var names = ['ALL'].concat(RC.severityNames().map(function (n) { return n.toUpperCase(); }));

        RC.render(container, names.map(function (label) {
            var active = state.filter === label;
            var color = label === 'ALL' ? 'var(--rc-accent)' : RC.severity(label).color;
            var style = active
                ? 'background:' + color + ';border-color:' + color + ';color:var(--rc-on-accent)'
                : 'color:' + color;

            return '<button class="rc-chip' + (active ? ' is-active' : '') +
                '" data-filter="' + RC.esc(label) + '" style="' + style + '">' +
                RC.esc(label) + '</button>';
        }).join(''));
    }

    function visibleAlerts() {
        return state.alerts.filter(function (a) {
            return state.filter === 'ALL' || (a.severity || '').toUpperCase() === state.filter;
        });
    }

    /* ── Feed ───────────────────────────────────────────────────────── */

    function alertRow(alert) {
        var meta = RC.severity(alert.severity);
        var acked = alert.state === 'acked';

        return '<div class="rc-alert' + (acked ? ' is-acked' : '') +
            '" data-alert-id="' + RC.esc(alert.id) + '"' +
            (alert.device_id ? ' data-device-id="' + RC.esc(alert.device_id) + '"' : '') + '>' +
            '<span class="rc-alert__bar" style="background:' + meta.color + '"></span>' +
            '<div class="rc-alert__body">' +
                '<div class="rc-alert__meta">' +
                    '<span class="rc-alert__sev" style="color:' + meta.color + '">' +
                        RC.esc((alert.severity || '').toUpperCase()) + '</span>' +
                    '<span class="rc-alert__time">' + RC.esc(RC.clockTime(alert.last_seen_at)) + '</span>' +
                    (alert.occurrences > 1
                        ? '<span class="rc-alert__time">×' + RC.esc(String(alert.occurrences)) + '</span>'
                        : '') +
                    '<span class="rc-alert__state" style="color:' +
                        (acked ? 'var(--rc-text-5)' : meta.color) + '">' +
                        (acked ? 'ACK' : 'OPEN') + '</span>' +
                '</div>' +
                '<div class="rc-alert__title">' + RC.esc(alert.title) + '</div>' +
                '<div class="rc-alert__detail">' + RC.esc(alert.detail) + '</div>' +
            '</div>' +
            '</div>';
    }

    function renderFeed(container) {
        if (!container) return;

        var rows = visibleAlerts();
        if (!rows.length) {
            RC.render(container, RC.empty(
                state.filter === 'ALL'
                    ? 'No alerts. Routine scan activity is on the EVENTS tab.'
                    : 'No ' + state.filter.toLowerCase() + ' alerts'
            ));
            return;
        }

        RC.render(container, rows.map(alertRow).join(''));
    }

    /* ── Toast ──────────────────────────────────────────────────────── */

    function toastCandidate() {
        for (var i = 0; i < state.alerts.length; i++) {
            var a = state.alerts[i];
            if (a.severity === 'critical' && a.state === 'open' && !state.dismissed[a.id]) return a;
        }
        return null;
    }

    function renderToast() {
        var mount = RC.el('rc-toast-mount');
        if (!mount) return;

        var alert = toastCandidate();
        if (!alert) {
            RC.render(mount, '');
            return;
        }

        RC.render(mount,
            '<div class="rc-toast" data-toast-id="' + RC.esc(alert.id) + '">' +
                '<div class="rc-toast__rule"></div>' +
                '<div class="rc-toast__body">' +
                    '<div class="rc-toast__meta">' +
                        '<span class="rc-toast__pulse"></span>' +
                        '<span class="rc-alert__sev" style="color:var(--rc-red)">CRITICAL</span>' +
                        '<span class="rc-alert__time">' + RC.esc(RC.clockTime(alert.last_seen_at)) + '</span>' +
                        '<span class="rc-close" data-toast-dismiss>✕</span>' +
                    '</div>' +
                    '<div class="rc-toast__title">' + RC.esc(alert.title) + '</div>' +
                    '<div class="rc-toast__detail">' + RC.esc(alert.detail) + '</div>' +
                    '<div class="rc-toast__actions">' +
                        (alert.device_id
                            ? '<button class="rc-btn rc-btn--solid rc-btn--grow" data-toast-inspect>INSPECT HOST</button>'
                            : '') +
                        '<button class="rc-btn rc-btn--grow" data-toast-ack>ACKNOWLEDGE</button>' +
                    '</div>' +
                '</div>' +
            '</div>');
    }

    /* ── Actions ────────────────────────────────────────────────────── */

    function ackAlert(id) {
        // Update in place first so the feed reacts immediately; the next poll
        // reconciles with the server either way.
        state.alerts.forEach(function (a) {
            if (a.id === id && a.state === 'open') {
                a.state = 'acked';
                var sev = (a.severity || 'info').toLowerCase();
                if (state.counts[sev] > 0) state.counts[sev]--;
                state.openCount = Math.max(0, state.openCount - 1);
            }
        });
        renderAll();

        return RC.sendJSON('POST', '/api/alerts/' + encodeURIComponent(id) + '/ack')
            .then(loadAlerts)
            .catch(function (err) {
                console.error('Failed to acknowledge alert:', err);
                return loadAlerts();
            });
    }

    function ackAllAlerts() {
        return RC.sendJSON('POST', '/api/alerts/ack-all')
            .then(loadAlerts)
            .catch(function (err) {
                console.error('Failed to acknowledge alerts:', err);
            });
    }

    function setFilter(label) {
        state.filter = label;
        renderAll();
    }

    function renderAll() {
        renderTile();
        renderFilters(RC.el('rc-alert-filters'));
        renderFilters(RC.el('rc-alert-filters-page'));
        renderFeed(RC.el('rc-feed-alerts'));
        renderFeed(RC.el('rc-alerts-page'));
        renderToast();
    }

    function bind() {
        [RC.el('rc-alert-filters'), RC.el('rc-alert-filters-page')].forEach(function (node) {
            RC.on(node, 'click', '[data-filter]', function (e, chip) {
                setFilter(chip.getAttribute('data-filter'));
            });
        });

        [RC.el('rc-feed-alerts'), RC.el('rc-alerts-page')].forEach(function (node) {
            RC.on(node, 'click', '[data-alert-id]', function (e, row) {
                var deviceId = row.getAttribute('data-device-id');
                if (deviceId) {
                    window.RCDevices.openDrawer(deviceId);
                    return;
                }
                // Nothing to inspect, so the click acknowledges instead.
                ackAlert(row.getAttribute('data-alert-id'));
            });
        });

        var mount = RC.el('rc-toast-mount');
        RC.on(mount, 'click', '[data-toast-dismiss]', function (e) {
            var toast = e.target.closest('[data-toast-id]');
            if (!toast) return;
            state.dismissed[toast.getAttribute('data-toast-id')] = true;
            renderToast();
        });
        RC.on(mount, 'click', '[data-toast-ack]', function (e) {
            var toast = e.target.closest('[data-toast-id]');
            if (!toast) return;
            var id = toast.getAttribute('data-toast-id');
            state.dismissed[id] = true;
            ackAlert(id);
        });
        RC.on(mount, 'click', '[data-toast-inspect]', function (e) {
            var toast = e.target.closest('[data-toast-id]');
            if (!toast) return;
            var alert = state.alerts.filter(function (a) {
                return a.id === toast.getAttribute('data-toast-id');
            })[0];
            if (alert && alert.device_id) window.RCDevices.openDrawer(alert.device_id);
        });
    }

    document.addEventListener('DOMContentLoaded', bind);

    window.RCAlerts = {
        load: loadAlerts,
        state: function () { return state; },
        onAlerts: onAlerts,
        setFilter: setFilter
    };
    window.ackAllAlerts = ackAllAlerts;
})();
