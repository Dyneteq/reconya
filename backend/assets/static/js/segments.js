/* Segments: the per-range status tiles at the top of the SCAN PLAN panel.
 *
 * One tile per range across every network. Clicking a tile sets an in-memory
 * scope that filters the device dock/hosts list by IP-in-CIDR membership —
 * this is separate from a range's persisted active/inactive scan state,
 * which the SCAN PLAN list below toggles instead.
 */
(function () {
    'use strict';

    var networks = [];
    var scopedRangeId = null;
    var scopeListeners = [];

    function onScopeChange(fn) {
        scopeListeners.push(fn);
    }

    function notifyScopeChange() {
        scopeListeners.forEach(function (fn) {
            try { fn(scopedCidr()); } catch (e) { console.error('segment scope listener failed', e); }
        });
    }

    function load() {
        return RC.getJSON('/api/networks')
            .then(function (data) {
                networks = data.networks || [];
                render();
                return networks;
            })
            .catch(function (err) {
                console.error('Failed to load segments:', err);
                return networks;
            });
    }

    function allRanges() {
        var out = [];
        networks.forEach(function (n) {
            (n.ranges || []).forEach(function (r) {
                out.push({ networkId: n.id, range: r });
            });
        });
        return out;
    }

    function scopedCidr() {
        if (!scopedRangeId) return null;
        var match = allRanges().filter(function (entry) { return entry.range.id === scopedRangeId; })[0];
        return match ? match.range.cidr : null;
    }

    // Used by devices.js-driven views to scope the visible device set.
    function filterDevices(devices) {
        var cidr = scopedCidr();
        if (!cidr) return devices;
        return devices.filter(function (d) { return RC.ipInCidr(d.ipv4, cidr); });
    }

    function render() {
        var container = RC.el('rc-segments');
        if (!container) return;

        var entries = allRanges();
        if (!entries.length) {
            RC.render(container, '');
            return;
        }

        var devices = (window.RCDevices ? window.RCDevices.state().devices : []) || [];

        RC.render(container, entries.map(function (entry) {
            var r = entry.range;
            var count = devices.filter(function (d) { return RC.ipInCidr(d.ipv4, r.cidr); }).length;
            var classes = ['rc-segment'];
            if (r.id === scopedRangeId) classes.push('is-active');
            if (!r.active) classes.push('is-inactive');

            return '<div class="' + classes.join(' ') + '" data-range-id="' + RC.esc(r.id) + '" title="' + RC.esc(r.cidr) + '">' +
                '<span class="rc-segment__cidr">' + RC.esc(r.label || r.cidr) + '</span>' +
                '<span class="rc-segment__count">' + count + ' known</span>' +
                '</div>';
        }).join(''));
    }

    function bind() {
        RC.on(RC.el('rc-segments'), 'click', '[data-range-id]', function (e, tile) {
            var id = tile.getAttribute('data-range-id');
            scopedRangeId = (scopedRangeId === id) ? null : id;
            render();
            notifyScopeChange();
        });
    }

    document.addEventListener('DOMContentLoaded', bind);

    window.RCSegments = {
        load: load,
        render: render,
        filterDevices: filterDevices,
        scopedCidr: scopedCidr,
        onScopeChange: onScopeChange,
        networks: function () { return networks; }
    };
})();
