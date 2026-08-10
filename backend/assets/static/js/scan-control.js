/* Scan control: the top bar's sweep cell and its START/PAUSE button.
 *
 * The design's sweep meter reads "62% · ETA 01:48 · 240 PPS". The scanner
 * reports none of that — ScanState carries a target network, a running flag, a
 * start time and a sweep counter, and nothing else. Rather than invent a
 * percentage this renders what is actually known, and the bar animates while a
 * sweep is running instead of pretending to fill.
 */
(function () {
    'use strict';

    var state = {
        scan: null,
        networks: [],
        listeners: []
    };

    var runtimeTimer = null;
    var autoStartTried = false;

    function onScan(fn) {
        state.listeners.push(fn);
    }

    function notify() {
        state.listeners.forEach(function (fn) {
            try { fn(state); } catch (e) { console.error('scan listener failed', e); }
        });
    }

    function loadScanControl() {
        return RC.getJSON('/api/scan/control')
            .then(function (data) {
                state.networks = data.networks || [];
                state.scan = data.scanState || null;
                render();
                notify();
                maybeAutoStart();
                return state;
            })
            .catch(function (err) {
                console.error('Failed to load scan control:', err);
                return state;
            });
    }

    // reconYa is meant to be sweeping the moment the console opens. The server
    // preselects the network the machine is actually on; until that async
    // identification lands there may be nothing to target yet, so keep trying
    // on each control load rather than only the first.
    function maybeAutoStart() {
        if (autoStartTried) return;
        if (state.scan && (state.scan.is_running || state.scan.is_stopping)) {
            autoStartTried = true;
            return;
        }

        var target = defaultNetworkId();
        if (!target) return;

        autoStartTried = true;
        startScan(target);
    }

    // The network to sweep when the user hasn't picked one: whatever the
    // server selected (the LAN this machine is on), else the first network
    // that describes an actual subnet — a /32 is a tunnel address, not a LAN.
    function defaultNetworkId() {
        var active = activeNetwork();
        if (active && active.id) return active.id;

        var subnet = state.networks.filter(function (n) {
            return n.cidr && n.cidr.indexOf('/32') === -1;
        })[0];
        return subnet ? subnet.id : null;
    }

    function loadScanStatus() {
        return RC.getJSON('/api/scan/status')
            .then(function (scan) {
                state.scan = scan;
                render();
                notify();
                maybeAutoStart();
                return state;
            })
            .catch(function (err) {
                console.error('Failed to load scan status:', err);
                return state;
            });
    }

    function activeNetwork() {
        if (!state.scan) return null;
        return state.scan.current_network || state.scan.selected_network || null;
    }

    function render() {
        var scan = state.scan;
        var cell = RC.el('rc-sweep');
        if (!cell) return;

        var running = !!(scan && scan.is_running);
        var stopping = !!(scan && scan.is_stopping);
        var network = activeNetwork();

        cell.classList.toggle('is-running', running && !stopping);

        RC.text('rc-sweep-target', network
            ? 'SWEEP ' + (network.cidr || network.name || '')
            : 'NO NETWORK SELECTED');

        var stateLabel = stopping ? 'STOPPING' : (running ? 'RUNNING' : 'IDLE');
        var stateEl = RC.el('rc-sweep-state');
        if (stateEl) {
            stateEl.textContent = stateLabel;
            stateEl.style.color = running ? 'var(--rc-accent)' : 'var(--rc-text-5)';
        }

        var bar = RC.el('rc-sweep-bar');
        if (bar) bar.classList.toggle('rc-sweep__bar--idle', !running);

        renderFacts();

        var toggle = RC.el('rc-sweep-toggle');
        if (toggle) {
            toggle.textContent = stopping ? 'STOPPING…' : (running ? 'PAUSE SWEEP' : 'START SWEEP');
            toggle.disabled = stopping || (!running && !network && state.networks.length === 0);
            toggle.classList.toggle('rc-actions__btn--solid', !running);
        }

        manageRuntimeTicker(running);
    }

    function renderFacts() {
        var scan = state.scan;
        var facts = [];

        if (scan && scan.is_running && scan.start_time) {
            facts.push('RUNTIME ' + RC.elapsed(scan.start_time));
        }
        if (scan && typeof scan.scan_count === 'number') {
            facts.push('SWEEP #' + scan.scan_count);
        }
        if (scan && scan.last_scan_time) {
            facts.push('LAST ' + RC.timeAgo(scan.last_scan_time) + ' AGO');
        }
        if (scan && scan.ipv6_monitoring) {
            facts.push('IPv6 MONITOR');
        }
        if (!facts.length) facts.push('NO SWEEPS YET');

        RC.render(RC.el('rc-sweep-facts'), facts.map(function (f) {
            return '<span>' + RC.esc(f) + '</span>';
        }).join('<span>·</span>'));
    }

    // The runtime is the one value that changes without a poll, so it gets its
    // own one-second ticker rather than pulling the whole scan state every
    // second just to move a clock.
    function manageRuntimeTicker(running) {
        if (running && !runtimeTimer) {
            runtimeTimer = setInterval(renderFacts, 1000);
        } else if (!running && runtimeTimer) {
            clearInterval(runtimeTimer);
            runtimeTimer = null;
        }
    }

    function startScan(networkId) {
        var target = networkId || defaultNetworkId();

        if (!target) {
            failDialog('NO NETWORK', 'Add a network before starting a sweep.');
            return Promise.resolve();
        }

        return RC.sendForm('/api/scan/start', { 'network-selector': target })
            .then(function (res) {
                if (res && res.error) failDialog('SWEEP DID NOT START', res.error);
                return loadScanControl();
            })
            .catch(function (err) {
                console.error('Failed to start scan:', err);
                return loadScanControl();
            });
    }

    function stopScan() {
        return RC.sendForm('/api/scan/stop', {})
            .then(function (res) {
                if (res && res.error) failDialog('SWEEP DID NOT STOP', res.error);
                return loadScanStatus();
            })
            .catch(function (err) {
                console.error('Failed to stop scan:', err);
                return loadScanStatus();
            });
    }

    function toggleScan() {
        if (state.scan && state.scan.is_running) return stopScan();
        return startScan();
    }

    function selectNetwork(networkId) {
        return RC.sendForm('/api/scan/select-network', { 'network-id': networkId })
            .then(loadScanControl);
    }

    function bind() {
        var toggle = RC.el('rc-sweep-toggle');
        if (toggle) toggle.addEventListener('click', toggleScan);
    }

    document.addEventListener('DOMContentLoaded', bind);

    window.RCScan = {
        load: loadScanControl,
        poll: loadScanStatus,
        state: function () { return state; },
        onScan: onScan,
        start: startScan,
        stop: stopScan,
        selectNetwork: selectNetwork
    };
})();
