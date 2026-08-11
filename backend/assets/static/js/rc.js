/* Shared helpers for the console. Loaded first; everything else hangs off RC.
 *
 * There is no bundler and no modules here — plain scripts on window, in the
 * order index.html lists them.
 */
window.RC = (function () {
    'use strict';

    // Severity metadata in one place. Rank is the feed's sort order and matches
    // models.SeverityRank on the server.
    var SEVERITIES = {
        critical: { rank: 0, color: 'var(--rc-red)', short: 'C' },
        warning:  { rank: 1, color: 'var(--rc-amber)', short: 'W' },
        notice:   { rank: 2, color: 'var(--rc-accent)', short: 'N' },
        info:     { rank: 3, color: 'var(--rc-text-4)', short: 'I' }
    };

    var STATUS_COLORS = {
        online:  '#4ade80',
        idle:    'rgba(74,222,128,0.45)',
        offline: '#3a4a42'
    };

    // Roles that read as infrastructure. The map draws these as squares and the
    // tables show them first-class.
    var INFRA_TYPES = ['router', 'switch', 'firewall', 'access_point', 'nas', 'server'];

    function el(id) {
        return document.getElementById(id);
    }

    function esc(value) {
        if (value === null || value === undefined) return '';
        return String(value)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');
    }

    function text(id, value) {
        var node = el(id);
        if (node) node.textContent = value;
    }

    // getJSON / sendJSON both throw on non-2xx so callers can use one catch for
    // transport errors and HTTP errors alike.
    function getJSON(url) {
        return fetch(url, { credentials: 'include' }).then(function (r) {
            if (!r.ok) throw new Error(r.status + ' ' + r.statusText);
            return r.json();
        });
    }

    function sendJSON(method, url, body) {
        var opts = { method: method, credentials: 'include' };
        if (body !== undefined) {
            opts.headers = { 'Content-Type': 'application/json' };
            opts.body = JSON.stringify(body);
        }
        return fetch(url, opts).then(function (r) {
            if (!r.ok) throw new Error(r.status + ' ' + r.statusText);
            return r.status === 204 ? null : r.json().catch(function () { return null; });
        });
    }

    function sendForm(url, fields) {
        var body = new URLSearchParams();
        Object.keys(fields).forEach(function (k) { body.append(k, fields[k]); });
        return fetch(url, {
            method: 'POST',
            credentials: 'include',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: body.toString()
        }).then(function (r) {
            return r.json().catch(function () { return { success: r.ok }; });
        });
    }

    function statusColor(status) {
        return STATUS_COLORS[(status || '').toLowerCase()] || '#4b5563';
    }

    function severity(name) {
        return SEVERITIES[(name || '').toLowerCase()] || SEVERITIES.info;
    }

    function severityNames() {
        return ['critical', 'warning', 'notice', 'info'];
    }

    function isInfra(deviceType) {
        return INFRA_TYPES.indexOf((deviceType || '').toLowerCase()) !== -1;
    }

    // timeAgo renders the compact "4s / 12m / 6h / 3d" spans the console uses
    // everywhere. Returns '—' for anything it cannot parse.
    function timeAgo(value) {
        if (!value) return '—';
        var then = new Date(value).getTime();
        if (isNaN(then)) return '—';

        var secs = Math.max(0, Math.floor((Date.now() - then) / 1000));
        if (secs < 60) return secs + 's';
        if (secs < 3600) return Math.floor(secs / 60) + 'm';
        if (secs < 86400) return Math.floor(secs / 3600) + 'h';
        return Math.floor(secs / 86400) + 'd';
    }

    function clockTime(value) {
        if (!value) return '';
        var d = new Date(value);
        if (isNaN(d.getTime())) return '';
        return String(d.getHours()).padStart(2, '0') + ':' + String(d.getMinutes()).padStart(2, '0');
    }

    function dateTime(value) {
        if (!value) return '—';
        var d = new Date(value);
        if (isNaN(d.getTime())) return '—';
        return d.getFullYear() + '-' +
            String(d.getMonth() + 1).padStart(2, '0') + '-' +
            String(d.getDate()).padStart(2, '0') + ' ' +
            String(d.getHours()).padStart(2, '0') + ':' +
            String(d.getMinutes()).padStart(2, '0');
    }

    // elapsed renders mm:ss / h:mm:ss for the sweep runtime ticker.
    function elapsed(since) {
        var start = new Date(since).getTime();
        if (isNaN(start)) return '—';

        var s = Math.max(0, Math.floor((Date.now() - start) / 1000));
        var h = Math.floor(s / 3600);
        var m = Math.floor((s % 3600) / 60);
        var sec = s % 60;

        if (h > 0) return h + ':' + String(m).padStart(2, '0') + ':' + String(sec).padStart(2, '0');
        return String(m).padStart(2, '0') + ':' + String(sec).padStart(2, '0');
    }

    function deviceName(device) {
        return device.name || device.hostname || '';
    }

    function openPorts(device) {
        return (device.ports || []).filter(function (p) {
            return String(p.state || '').toLowerCase() === 'open';
        });
    }

    // A device the scanner could not characterise at all. Mirrors the
    // unidentified_host rule in internal/alert/rules.go so the map's amber nodes
    // and the alert feed agree with each other.
    function isUnidentified(device) {
        if (!device || (device.status || '').toLowerCase() === 'offline') return false;
        if (device.vendor || device.hostname || device.name) return false;
        if (openPorts(device).length > 0) return false;
        if (!device.mac) return false;

        var hex = String(device.mac).toLowerCase().replace(/[^0-9a-f]/g, '');
        if (hex.length !== 12) return false;
        return (parseInt(hex.slice(0, 2), 16) & 0x02) !== 0;
    }

    // ipToInt/ipInCidr/cidrSize are IPv4-only, matching network_ranges (IPv6
    // networks stay single-prefix and don't use the ranges table).
    function ipToInt(ip) {
        var parts = String(ip || '').split('.');
        if (parts.length !== 4) return null;
        var n = 0;
        for (var i = 0; i < 4; i++) {
            var octet = Number(parts[i]);
            if (isNaN(octet) || octet < 0 || octet > 255) return null;
            n = (n << 8) | octet;
        }
        return n >>> 0;
    }

    function ipInCidr(ip, cidr) {
        var parts = String(cidr || '').split('/');
        if (parts.length !== 2) return false;

        var base = ipToInt(parts[0]);
        var prefix = Number(parts[1]);
        var target = ipToInt(ip);
        if (base === null || target === null || isNaN(prefix) || prefix < 0 || prefix > 32) return false;

        if (prefix === 0) return true;
        var mask = (0xFFFFFFFF << (32 - prefix)) >>> 0;
        return (base & mask) === (target & mask);
    }

    // Usable host count (excludes network/broadcast addresses for /0-/30, as
    // real allocatable capacity is what the Scan Plan panel wants to show).
    function cidrSize(cidr) {
        var parts = String(cidr || '').split('/');
        if (parts.length !== 2) return 0;
        var prefix = Number(parts[1]);
        if (isNaN(prefix) || prefix < 0 || prefix > 32) return 0;

        var hostBits = 32 - prefix;
        if (hostBits === 0) return 1;
        if (hostBits === 1) return 2;
        return Math.pow(2, hostBits) - 2;
    }

    // Small localStorage wrapper — private-mode browsers throw on write.
    var store = {
        get: function (key, fallback) {
            try {
                var v = localStorage.getItem(key);
                return v === null ? fallback : v;
            } catch (e) {
                return fallback;
            }
        },
        set: function (key, value) {
            try {
                localStorage.setItem(key, value);
            } catch (e) {
                /* nothing sensible to do; the preference just won't persist */
            }
        }
    };

    // Replaces a container's children from an HTML string.
    function render(node, html) {
        if (!node) return;
        node.innerHTML = html;
    }

    function empty(message) {
        return '<div class="rc-empty">' + esc(message) + '</div>';
    }

    // on delegates a listener from a container down to matching descendants, so
    // re-rendered lists don't need their handlers rebound.
    function on(container, event, selector, handler) {
        if (!container) return;
        container.addEventListener(event, function (e) {
            var match = e.target.closest(selector);
            if (match && container.contains(match)) handler(e, match);
        });
    }

    return {
        SEVERITIES: SEVERITIES,
        el: el,
        esc: esc,
        text: text,
        getJSON: getJSON,
        sendJSON: sendJSON,
        sendForm: sendForm,
        statusColor: statusColor,
        severity: severity,
        severityNames: severityNames,
        isInfra: isInfra,
        isUnidentified: isUnidentified,
        ipInCidr: ipInCidr,
        cidrSize: cidrSize,
        timeAgo: timeAgo,
        clockTime: clockTime,
        dateTime: dateTime,
        elapsed: elapsed,
        deviceName: deviceName,
        openPorts: openPorts,
        store: store,
        render: render,
        empty: empty,
        on: on
    };
})();
