/* Networks: the NETS page table and the add/edit dialog. */
(function () {
    'use strict';

    var networks = [];
    var scanState = null;

    function loadNetworks() {
        return RC.getJSON('/api/networks')
            .then(function (data) {
                networks = data.networks || [];
                scanState = data.scanState || null;
                renderPage();
                return networks;
            })
            .catch(function (err) {
                console.error('Failed to load networks:', err);
                RC.render(RC.el('rc-networks-page'), RC.empty('Could not load networks'));
                return [];
            });
    }

    function isScanning(network) {
        if (!scanState || !scanState.is_running) return false;
        var current = scanState.current_network || scanState.selected_network;
        return !!(current && current.id === network.id);
    }

    function renderPage() {
        var container = RC.el('rc-networks-page');
        if (!container) return;

        if (!networks.length) {
            RC.render(container, RC.empty('No networks configured. Add one to start scanning.'));
            return;
        }

        var rows = networks.map(function (n) {
            var scanning = isScanning(n);
            return '<tr data-network-id="' + RC.esc(n.id) + '">' +
                '<td class="rc-code">' + RC.esc(n.cidr || '—') + '</td>' +
                '<td style="color:var(--rc-text)">' + RC.esc(n.name || 'unnamed') + '</td>' +
                '<td>' + RC.esc(n.description || '—') + '</td>' +
                '<td style="text-align:right">' + RC.esc(String(n.device_count || 0)) + '</td>' +
                '<td>' + RC.esc(n.last_scanned_at ? RC.timeAgo(n.last_scanned_at) + ' ago' : 'never') + '</td>' +
                '<td style="color:' + (scanning ? 'var(--rc-accent)' : 'var(--rc-text-5)') + '">' +
                    (scanning ? '● SCANNING' : 'IDLE') + '</td>' +
                '<td class="rc-cell--right">' +
                    '<button class="rc-btn" data-action="scan" style="padding:3px 8px">SCAN</button> ' +
                    '<button class="rc-btn" data-action="edit" style="padding:3px 8px">EDIT</button> ' +
                    '<button class="rc-btn rc-btn--danger" data-action="delete" style="padding:3px 8px">⌫</button>' +
                '</td>' +
                '</tr>';
        }).join('');

        RC.render(container,
            '<div class="rc-table__scroll"><table class="rc-table">' +
            '<thead><tr>' +
                '<th>CIDR</th><th>NAME</th><th>DESCRIPTION</th>' +
                '<th style="text-align:right">HOSTS</th><th>LAST SWEEP</th><th>STATE</th><th></th>' +
            '</tr></thead>' +
            '<tbody>' + rows + '</tbody></table></div>');
    }

    /* ── Dialog ─────────────────────────────────────────────────────── */

    function networkForm(network) {
        network = network || {};
        return '<div class="rc-form">' +
            '<div class="rc-field">' +
                '<label for="rc-net-cidr">CIDR</label>' +
                '<input id="rc-net-cidr" value="' + RC.esc(network.cidr || '') + '" placeholder="192.168.1.0/24">' +
            '</div>' +
            '<div class="rc-field">' +
                '<label for="rc-net-name">Name</label>' +
                '<input id="rc-net-name" value="' + RC.esc(network.name || '') + '" placeholder="Office LAN">' +
            '</div>' +
            '<div class="rc-field">' +
                '<label for="rc-net-desc">Description</label>' +
                '<input id="rc-net-desc" value="' + RC.esc(network.description || '') + '" placeholder="optional">' +
            '</div>' +
            '<div id="rc-net-error" style="color:var(--rc-red);font-size:11px"></div>' +
        '</div>';
    }

    function openNetworkDialog(networkId) {
        var network = networks.filter(function (n) { return n.id === networkId; })[0];

        openDialog({
            title: network ? 'EDIT NETWORK' : 'ADD NETWORK',
            body: networkForm(network),
            confirm: 'SAVE',
            onConfirm: function () {
                var fields = {
                    cidr: (RC.el('rc-net-cidr') || {}).value || '',
                    name: (RC.el('rc-net-name') || {}).value || '',
                    description: (RC.el('rc-net-desc') || {}).value || ''
                };

                if (!fields.cidr.trim()) {
                    RC.text('rc-net-error', 'A CIDR is required.');
                    return;
                }

                var url = network ? '/api/networks/' + encodeURIComponent(network.id) : '/api/networks';
                var body = new URLSearchParams();
                Object.keys(fields).forEach(function (k) { body.append(k, fields[k]); });

                fetch(url, {
                    method: network ? 'PUT' : 'POST',
                    credentials: 'include',
                    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                    body: body.toString()
                })
                    .then(function (r) { return r.json(); })
                    .then(function (res) {
                        if (!res.success) {
                            RC.text('rc-net-error', res.message || 'Could not save this network.');
                            return;
                        }
                        closeDialog();
                        loadNetworks();
                    })
                    .catch(function (err) {
                        console.error('Failed to save network:', err);
                        RC.text('rc-net-error', 'Could not save this network.');
                    });
            }
        });
    }

    function deleteNetwork(networkId) {
        var network = networks.filter(function (n) { return n.id === networkId; })[0];
        if (!network) return;

        var count = network.device_count || 0;
        var body = '<div style="font-size:12px;color:var(--rc-text-3);line-height:1.6">' +
            'Remove <strong style="color:var(--rc-text)">' + RC.esc(network.cidr) + '</strong>' +
            (count ? ' and detach its ' + count + ' known host' + (count === 1 ? '' : 's') : '') + '?' +
            '</div>';

        openDialog({
            title: 'DELETE NETWORK',
            body: body,
            confirm: 'DELETE',
            danger: true,
            onConfirm: function () {
                fetch('/api/networks/' + encodeURIComponent(networkId), {
                    method: 'DELETE',
                    credentials: 'include'
                })
                    .then(function (r) { return r.json(); })
                    .then(function (res) {
                        closeDialog();
                        if (res && res.error) {
                            failDialog('DELETE REFUSED', res.error);
                            return;
                        }
                        loadNetworks();
                    })
                    .catch(function (err) {
                        console.error('Failed to delete network:', err);
                        closeDialog();
                        failDialog('DELETE FAILED', String(err.message));
                    });
            }
        });
    }

    function bind() {
        var page = RC.el('rc-networks-page');
        RC.on(page, 'click', 'button[data-action]', function (e, btn) {
            e.stopPropagation();

            var row = btn.closest('[data-network-id]');
            if (!row) return;
            var id = row.getAttribute('data-network-id');

            var action = btn.getAttribute('data-action');
            if (action === 'edit') openNetworkDialog(id);
            else if (action === 'delete') deleteNetwork(id);
            else if (action === 'scan') {
                window.RCScan.selectNetwork(id).then(function () {
                    return window.RCScan.start(id);
                }).then(loadNetworks);
            }
        });
    }

    document.addEventListener('DOMContentLoaded', bind);

    window.RCNetworks = {
        load: loadNetworks,
        list: function () { return networks; }
    };
    window.openNetworkDialog = openNetworkDialog;
})();
