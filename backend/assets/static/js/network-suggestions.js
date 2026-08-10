/* Auto-discovery of local networks.
 *
 * reconYa is meant to be useful the moment it starts, so any interface network
 * it detects and does not already know about is added automatically. The scan
 * control picks it up from there and starts sweeping.
 */
(function () {
    'use strict';

    var detected = [];
    var timer = null;

    function checkForNetworkSuggestions() {
        return Promise.all([
            RC.getJSON('/api/detected-networks').catch(function () { return []; }),
            RC.getJSON('/api/networks').catch(function () { return { networks: [] }; })
        ]).then(function (results) {
            detected = results[0] || [];
            var existing = (results[1] && results[1].networks) || [];

            var missing = detected.filter(function (n) {
                return !existing.some(function (e) { return e.cidr === n.cidr; });
            });

            if (missing.length) return addDetected(missing);
        }).catch(function (err) {
            console.error('Failed to check for detected networks:', err);
        });
    }

    // Added one at a time rather than in parallel: every write funnels through
    // the DB manager's single worker anyway, and serialising here keeps the
    // event log readable.
    function addDetected(networks) {
        return networks.reduce(function (chain, network) {
            return chain.then(function () {
                return RC.sendForm('/api/network-suggestion', {
                    cidr: network.cidr,
                    interface_name: network.interface_name || network['interface'] || '',
                    gateway: network.gateway || '',
                    name: network.name || ''
                }).catch(function (err) {
                    console.error('Failed to auto-add network', network.cidr, err);
                });
            });
        }, Promise.resolve()).then(function () {
            if (window.RCScan) return window.RCScan.load();
        });
    }

    function initNetworkSuggestions() {
        checkForNetworkSuggestions();
        if (timer) clearInterval(timer);
        timer = setInterval(checkForNetworkSuggestions, 15000);
    }

    window.RCSuggestions = {
        init: initNetworkSuggestions,
        detected: function () { return detected; }
    };
})();
