/* Topology map.
 *
 * Two layouts, both drawn from live device data:
 *
 *   TREE    gateway → core switch → one band per network → host swarm
 *   RADIAL  one dashed ring per network, gateway at the centre
 *
 * The design also sketches CLUSTERS and GRID. GRID enumerates a /24's whole
 * address space, which this app does not model — it stores discovered hosts,
 * not allocations — so drawing it would mean inventing 200 empty cells per
 * network. Both are left out rather than faked.
 *
 * Node shape encodes role, fill encodes status. The internet node and packet
 * animation the old map drew are gone: the design has no internet node, and
 * that state now lives in the top bar.
 *
 * Each device earns a persistent slot within its network on first sight (see
 * assignSlots) and keeps it across polls, and a network's band/ring capacity
 * eases toward its live device count instead of snapping. A host flapping
 * on or offline therefore moves (or removes) only itself, rather than
 * reshuffling every other node on the map.
 */
(function () {
    'use strict';

    var COLORS = {
        bg: '#050806',
        grid: 'rgba(74,222,128,0.045)',
        line: 'rgba(74,222,128,0.16)',
        lineSoft: 'rgba(74,222,128,0.08)',
        lineMid: 'rgba(74,222,128,0.30)',
        accent: '#4ade80',
        accentDim: 'rgba(74,222,128,0.55)',
        amber: '#f5b544',
        idle: 'rgba(74,222,128,0.45)',
        offline: '#3a4a42',
        text: '#dbe5de',
        textDim: '#6f8378',
        textFaint: '#4c5f55'
    };

    var MODES = [
        { id: 'TREE', hint: 'gateway → switch → subnet → hosts' },
        { id: 'RADIAL', hint: 'rings = subnet · shape = role' }
    ];

    var GRID = 44;
    var ENTRY_MS = 900;
    var ZOOM_MIN = 0.4;
    var ZOOM_MAX = 6;

    var canvas, ctx, wrap;
    var lw = 0, lh = 0;
    var rafId = null;

    var view = { zoom: 1, panX: 0, panY: 0 };
    var drag = null;
    var pinch = null;
    var suppressClick = false;

    var nodes = [];            // laid-out, animated node records
    var groups = [];           // one per network, in display order
    var devices = [];          // filtered device set currently drawn
    var slotState = {};        // per-network device -> slot assignments, see assignSlots()
    var hovered = null;
    var selectedId = null;
    var showOffline = false;
    var showIgnored = false;
    var filterText = '';
    var mode = 'TREE';
    var onSelect = null;

    /* ── Setup ──────────────────────────────────────────────────────── */

    function init(opts) {
        canvas = document.getElementById('rc-map-canvas');
        wrap = document.getElementById('rc-map-view');
        if (!canvas || !wrap) return;

        ctx = canvas.getContext('2d');
        onSelect = (opts && opts.onSelect) || null;
        mode = RC.store.get('reconya.mapMode', 'TREE');
        if (!MODES.some(function (m) { return m.id === mode; })) mode = 'TREE';
        showOffline = RC.store.get('reconya.showOffline', 'false') === 'true';
        showIgnored = RC.store.get('reconya.showIgnored', 'false') === 'true';

        resize();
        if (window.ResizeObserver) new ResizeObserver(resize).observe(wrap);
        else window.addEventListener('resize', resize);

        canvas.addEventListener('mousedown', onDown);
        canvas.addEventListener('mousemove', onMove);
        canvas.addEventListener('mouseup', onUp);
        canvas.addEventListener('mouseleave', onLeave);
        canvas.addEventListener('click', onClick);
        canvas.addEventListener('wheel', onWheel, { passive: false });
        canvas.addEventListener('touchstart', onTouchStart, { passive: true });
        canvas.addEventListener('touchmove', onTouchMove, { passive: false });
        canvas.addEventListener('touchend', function () { pinch = null; }, { passive: true });
        canvas.addEventListener('dblclick', fit);

        if (rafId) cancelAnimationFrame(rafId);
        rafId = requestAnimationFrame(tick);
    }

    function resize() {
        if (!canvas || !wrap) return;

        var dpr = window.devicePixelRatio || 1;
        lw = wrap.clientWidth || 800;
        lh = wrap.clientHeight || 600;

        canvas.width = Math.round(lw * dpr);
        canvas.height = Math.round(lh * dpr);
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

        layout();
    }

    /* ── Data ───────────────────────────────────────────────────────── */

    function setData(allDevices, networkCIDRs) {
        var filtered = (allDevices || []).filter(function (d) {
            // Favorite devices stay on the map even when offline and the
            // global toggle is off.
            if (!showOffline && !d.is_favorite && (d.status || '').toLowerCase() === 'offline') return false;
            if (!showIgnored && d.ignored) return false;
            return matchesFilter(d);
        });

        devices = filtered;
        groups = groupByNetwork(filtered, networkCIDRs || {});
        layout();
    }

    function matchesFilter(device) {
        if (!filterText) return true;
        var haystack = [
            device.ipv4, device.name, device.hostname, device.vendor, device.device_type,
            (device.ports || []).map(function (p) { return p.number; }).join(' ')
        ].join(' ').toLowerCase();
        return haystack.indexOf(filterText) !== -1;
    }

    function groupByNetwork(list, cidrs) {
        var byId = {};

        list.forEach(function (d) {
            var key = d.network_id || 'unassigned';
            if (!byId[key]) {
                byId[key] = { id: key, label: cidrs[key] || 'unassigned', devices: [] };
            }
            byId[key].devices.push(d);
        });

        return Object.keys(byId).map(function (k) { return byId[k]; })
            .sort(function (a, b) { return a.label.localeCompare(b.label); });
    }

    /* ── Layout ─────────────────────────────────────────────────────── */

    // Devices earn a slot within their network the first time they are seen,
    // and keep it for as long as they keep appearing in the device set. A
    // device dropping out (offline, filtered, deleted) frees its slot for
    // reuse but does not shift its neighbours into the gap, and a group's
    // capacity (its slot count) eases toward the live device count rather
    // than snapping to it. Together these stop one host flapping from
    // reshuffling every other host's position, on this network or any other,
    // which is what read as the whole map "flickering".
    function assignSlots(group) {
        var state = slotState[group.id];
        if (!state) state = slotState[group.id] = { indexById: {}, maxIndex: -1, capacity: 0 };

        var presentIds = {};
        var used = {};
        group.devices.forEach(function (d) {
            presentIds[d.id] = true;
            var idx = state.indexById[d.id];
            if (idx != null) used[idx] = true;
        });

        group.devices.forEach(function (d) {
            if (state.indexById[d.id] != null) return;
            var idx = 0;
            while (used[idx]) idx++;
            state.indexById[d.id] = idx;
            used[idx] = true;
            if (idx > state.maxIndex) state.maxIndex = idx;
        });

        // Forget devices no longer present so their slot can be reused and
        // the map does not grow without bound.
        Object.keys(state.indexById).forEach(function (id) {
            if (!presentIds[id]) delete state.indexById[id];
        });

        var target = Math.max(1, state.maxIndex + 1);
        state.capacity = state.capacity ? state.capacity + (target - state.capacity) * 0.25 : target;

        group.slotOf = state.indexById;
        group.capacity = state.capacity;
    }

    function pruneSlotState() {
        var liveIds = {};
        groups.forEach(function (g) { liveIds[g.id] = true; });
        Object.keys(slotState).forEach(function (id) {
            if (!liveIds[id]) delete slotState[id];
        });
    }

    // Nodes keep their animated position across relayouts so devices glide to a
    // new spot rather than snapping when the mode or the device set changes.
    function layout() {
        if (!lw || !lh) return;

        pruneSlotState();
        groups.forEach(assignSlots);

        var previous = {};
        nodes.forEach(function (n) { previous[n.id] = n; });

        var placed = mode === 'RADIAL' ? layoutRadial() : layoutTree();

        nodes = placed.map(function (p) {
            var old = previous[p.id];
            if (old) {
                p.x = old.x;
                p.y = old.y;
                p.born = old.born;
            } else {
                // New nodes fly out from the gateway.
                p.x = gatewayPoint().x;
                p.y = gatewayPoint().y;
                p.born = performance.now();
            }
            return p;
        });
    }

    function gatewayPoint() {
        if (mode === 'RADIAL') return { x: lw / 2, y: lh / 2 };
        return { x: 90, y: lh / 2 };
    }

    function nodeRecord(device, tx, ty) {
        return {
            id: device.id,
            device: device,
            tx: tx,
            ty: ty,
            x: tx,
            y: ty,
            born: 0,
            shape: shapeFor(device),
            color: colorFor(device)
        };
    }

    function shapeFor(device) {
        if (RC.isUnidentified(device)) return 'triangle';
        if (RC.isInfra(device.device_type)) return 'square';
        if (!device.device_type || device.device_type === 'unknown') return 'triangle';
        return 'circle';
    }

    function colorFor(device) {
        if (RC.isUnidentified(device)) return COLORS.amber;

        switch ((device.status || '').toLowerCase()) {
            case 'online': return COLORS.accent;
            case 'idle': return COLORS.idle;
            case 'offline': return COLORS.offline;
            default: return COLORS.textDim;
        }
    }

    // TREE — gateway, a core switch, one band per network sized by (eased)
    // slot capacity, and each band's hosts laid out as rows by their
    // persistent slot so a device keeps its row/column across polls.
    function layoutTree() {
        var placed = [];
        var totalCapacity = groups.reduce(function (sum, g) { return sum + g.capacity; }, 0) || 1;
        var usable = lh - 120;
        var top = 60;

        var swarmX = 620;
        var available = Math.max(160, lw - swarmX - 40);
        var perRow = Math.max(6, Math.floor(available / 15));

        var cursor = top;
        groups.forEach(function (group) {
            var bandH = Math.max(48, usable * (group.capacity / totalCapacity));
            var mid = cursor + bandH / 2;

            group.bandMid = mid;
            group.bandTop = cursor;
            group.bandH = bandH;

            var rows = Math.ceil(group.capacity / perRow) || 1;
            var rowH = Math.min(15, Math.max(9, (bandH - 14) / rows));

            group.devices.forEach(function (d) {
                var slot = group.slotOf[d.id];
                var row = Math.floor(slot / perRow);
                var col = slot % perRow;
                placed.push(nodeRecord(d,
                    swarmX + col * 15 + 6,
                    mid - (rows * rowH) / 2 + row * rowH + rowH / 2));
            });

            cursor += bandH;
        });

        return placed;
    }

    // RADIAL — one ring per network, radius growing outward, hosts spread
    // evenly around each ring by their persistent slot (not their live index)
    // so removing one host does not rotate the rest of the ring.
    function layoutRadial() {
        var placed = [];
        var cx = lw / 2;
        var cy = lh / 2;
        var maxR = Math.min(lw, lh) / 2 - 60;

        groups.forEach(function (group, gi) {
            var step = groups.length === 1 ? 0.62 : 0.3 + (gi / Math.max(1, groups.length - 1)) * 0.64;
            var R = maxR * step;
            group.ringR = R;
            group.ringAngle = -Math.PI / 2 + gi * 0.9;

            var capacity = group.capacity || 1;
            group.devices.forEach(function (d) {
                var slot = group.slotOf[d.id];
                var a = (slot / capacity) * Math.PI * 2 + gi * 0.4;
                // Deterministic wobble keyed off the slot — no Math.random, so
                // the layout is stable across redraws.
                var wobble = ((slot * 37) % 11 - 5) / 5 * (maxR * 0.02);
                placed.push(nodeRecord(d,
                    cx + Math.cos(a) * (R + wobble),
                    cy + Math.sin(a) * (R + wobble)));
            });
        });

        return placed;
    }

    /* ── Render loop ────────────────────────────────────────────────── */

    function tick(ts) {
        rafId = requestAnimationFrame(tick);
        if (!ctx || !lw) return;

        nodes.forEach(function (n) {
            var age = ts - n.born;
            if (age < ENTRY_MS) {
                var t = easeOutCubic(Math.min(1, age / ENTRY_MS));
                var origin = gatewayPoint();
                n.x = origin.x + (n.tx - origin.x) * t;
                n.y = origin.y + (n.ty - origin.y) * t;
            } else {
                n.x += (n.tx - n.x) * 0.12;
                n.y += (n.ty - n.y) * 0.12;
            }
        });

        render();
    }

    function easeOutCubic(t) {
        return 1 - Math.pow(1 - t, 3);
    }

    function render() {
        ctx.fillStyle = COLORS.bg;
        ctx.fillRect(0, 0, lw, lh);

        ctx.save();
        ctx.translate(view.panX, view.panY);
        ctx.scale(view.zoom, view.zoom);

        drawGrid();
        if (mode === 'RADIAL') drawRadialChrome();
        else drawTreeChrome();

        nodes.forEach(drawNode);
        drawSelection();

        ctx.restore();

        if (!nodes.length) drawEmpty();
    }

    function drawGrid() {
        var x0 = -view.panX / view.zoom;
        var y0 = -view.panY / view.zoom;
        var x1 = x0 + lw / view.zoom;
        var y1 = y0 + lh / view.zoom;

        ctx.strokeStyle = COLORS.grid;
        ctx.lineWidth = 1 / view.zoom;

        for (var x = Math.floor(x0 / GRID) * GRID; x < x1; x += GRID) {
            ctx.beginPath(); ctx.moveTo(x, y0); ctx.lineTo(x, y1); ctx.stroke();
        }
        for (var y = Math.floor(y0 / GRID) * GRID; y < y1; y += GRID) {
            ctx.beginPath(); ctx.moveTo(x0, y); ctx.lineTo(x1, y); ctx.stroke();
        }
    }

    function drawTreeChrome() {
        var gwX = 90;
        var gwY = lh / 2;
        var swX = 250;
        var subX = 430;

        // Gateway
        ctx.strokeStyle = COLORS.accentDim;
        ctx.lineWidth = 1;
        ctx.strokeRect(gwX - 26, gwY - 16, 52, 32);
        ctx.fillStyle = COLORS.accent;
        ctx.fillRect(gwX - 5, gwY - 5, 10, 10);

        ctx.textAlign = 'center';
        ctx.fillStyle = COLORS.accentDim;
        ctx.font = '10px "IBM Plex Mono", monospace';
        ctx.fillText('GATEWAY', gwX, gwY - 26);

        // Core switch
        ctx.strokeStyle = COLORS.lineMid;
        ctx.beginPath(); ctx.moveTo(gwX + 26, gwY); ctx.lineTo(swX - 20, gwY); ctx.stroke();
        ctx.strokeRect(swX - 20, gwY - 13, 40, 26);
        ctx.fillStyle = 'rgba(74,222,128,0.75)';
        ctx.font = '9px "IBM Plex Mono", monospace';
        ctx.fillText('SWITCH', swX, gwY - 22);

        groups.forEach(function (group) {
            var mid = group.bandMid;

            ctx.strokeStyle = COLORS.lineMid;
            ctx.lineWidth = 1;
            ctx.beginPath();
            ctx.moveTo(swX + 20, gwY);
            ctx.bezierCurveTo(swX + 80, gwY, subX - 90, mid, subX - 26, mid);
            ctx.stroke();

            ctx.strokeStyle = 'rgba(74,222,128,0.28)';
            ctx.strokeRect(subX - 26, mid - 13, 26, 26);

            ctx.textAlign = 'left';
            ctx.fillStyle = COLORS.text;
            ctx.font = '10px "IBM Plex Mono", monospace';
            ctx.fillText(group.label, subX + 6, mid - 2);

            ctx.fillStyle = COLORS.textFaint;
            ctx.font = '9px "IBM Plex Mono", monospace';
            ctx.fillText(group.devices.length + ' HOSTS', subX + 6, mid + 11);

            // A single leader from the subnet box to the first host of each row.
            var first = nodes.filter(function (n) {
                return (n.device.network_id || 'unassigned') === group.id;
            })[0];
            if (first) {
                ctx.strokeStyle = COLORS.lineSoft;
                ctx.beginPath();
                ctx.moveTo(subX + 4, mid);
                ctx.lineTo(first.x - 8, first.y);
                ctx.stroke();
            }
        });
    }

    function drawRadialChrome() {
        var cx = lw / 2;
        var cy = lh / 2;

        var glow = ctx.createRadialGradient(cx, cy, 0, cx, cy, Math.min(lw, lh) * 0.28);
        glow.addColorStop(0, 'rgba(74,222,128,0.14)');
        glow.addColorStop(1, 'rgba(74,222,128,0)');
        ctx.fillStyle = glow;
        ctx.beginPath();
        ctx.arc(cx, cy, Math.min(lw, lh) * 0.28, 0, Math.PI * 2);
        ctx.fill();

        groups.forEach(function (group, gi) {
            ctx.strokeStyle = 'rgba(74,222,128,' + Math.max(0.06, 0.18 - gi * 0.03) + ')';
            ctx.lineWidth = 1;
            ctx.setLineDash([3, 6]);
            ctx.beginPath();
            ctx.arc(cx, cy, group.ringR, 0, Math.PI * 2);
            ctx.stroke();
            ctx.setLineDash([]);

            var a = group.ringAngle;
            var lx = cx + Math.cos(a) * (group.ringR + 12);
            var ly = cy + Math.sin(a) * (group.ringR + 12);

            ctx.fillStyle = COLORS.accentDim;
            ctx.font = '9px "IBM Plex Mono", monospace';
            ctx.textAlign = Math.cos(a) < -0.2 ? 'right' : (Math.cos(a) > 0.2 ? 'left' : 'center');
            ctx.fillText(group.label + ' · ' + group.devices.length,
                lx, ly + (Math.sin(a) < 0 ? -4 : 10));
        });

        ctx.strokeStyle = COLORS.accentDim;
        ctx.lineWidth = 1;
        ctx.beginPath(); ctx.arc(cx, cy, 22, 0, Math.PI * 2); ctx.stroke();
        ctx.fillStyle = COLORS.accent;
        ctx.beginPath(); ctx.arc(cx, cy, 6, 0, Math.PI * 2); ctx.fill();

        ctx.fillStyle = 'rgba(74,222,128,0.85)';
        ctx.font = '10px "IBM Plex Mono", monospace';
        ctx.textAlign = 'center';
        ctx.fillText('GATEWAY', cx, cy + 40);
    }

    function drawNode(n) {
        var s = n.shape === 'square' ? 5 : 4.4;

        ctx.fillStyle = n.color;
        ctx.strokeStyle = n.color;
        ctx.lineWidth = 1;

        if (n.shape === 'square') {
            ctx.fillRect(n.x - s / 2, n.y - s / 2, s, s);
        } else if (n.shape === 'triangle') {
            ctx.beginPath();
            ctx.moveTo(n.x, n.y - s);
            ctx.lineTo(n.x + s, n.y + s * 0.7);
            ctx.lineTo(n.x - s, n.y + s * 0.7);
            ctx.closePath();
            ctx.fill();
        } else {
            ctx.beginPath();
            ctx.arc(n.x, n.y, s / 1.7, 0, Math.PI * 2);
            ctx.fill();
        }

        // A soft halo marks hosts with open ports — the ones worth looking at.
        if (n.color === COLORS.accent && RC.openPorts(n.device).length > 0) {
            ctx.globalAlpha = 0.25;
            ctx.beginPath();
            ctx.arc(n.x, n.y, s * 2.1, 0, Math.PI * 2);
            ctx.stroke();
            ctx.globalAlpha = 1;
        }

        if (hovered && hovered.id === n.id) {
            ctx.globalAlpha = 0.5;
            ctx.beginPath();
            ctx.arc(n.x, n.y, 12, 0, Math.PI * 2);
            ctx.stroke();
            ctx.globalAlpha = 1;
        }
    }

    function drawSelection() {
        if (!selectedId) return;

        var target = nodes.filter(function (n) { return n.id === selectedId; })[0];
        if (!target) return;

        ctx.strokeStyle = COLORS.accent;
        ctx.lineWidth = 1.4;

        var b = 11;
        [[-1, -1], [1, -1], [-1, 1], [1, 1]].forEach(function (corner) {
            var sx = corner[0], sy = corner[1];
            ctx.beginPath();
            ctx.moveTo(target.x + sx * b, target.y + sy * b - sy * 5);
            ctx.lineTo(target.x + sx * b, target.y + sy * b);
            ctx.lineTo(target.x + sx * b - sx * 5, target.y + sy * b);
            ctx.stroke();
        });

        ctx.globalAlpha = 0.5;
        ctx.beginPath();
        ctx.arc(target.x, target.y, 20, 0, Math.PI * 2);
        ctx.stroke();
        ctx.globalAlpha = 1;

        // Leader line, flipped so the label never runs off the near edge.
        var side = target.x > lw / 2 ? -1 : 1;
        ctx.beginPath();
        ctx.moveTo(target.x + side * 13, target.y);
        ctx.lineTo(target.x + side * 46, target.y - 14);
        ctx.lineTo(target.x + side * 78, target.y - 14);
        ctx.stroke();

        ctx.fillStyle = COLORS.text;
        ctx.font = '10px "IBM Plex Mono", monospace';
        ctx.textAlign = side > 0 ? 'left' : 'right';
        ctx.fillText(
            target.device.ipv4 + '  ' + (RC.deviceName(target.device) || ''),
            target.x + side * 84, target.y - 11);
    }

    function drawEmpty() {
        ctx.fillStyle = COLORS.textFaint;
        ctx.font = '11px "IBM Plex Mono", monospace';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(
            filterText ? 'NO HOSTS MATCH THE FILTER' : 'NO HOSTS DISCOVERED YET',
            lw / 2, lh / 2);
        ctx.textBaseline = 'alphabetic';
    }

    /* ── Interaction ────────────────────────────────────────────────── */

    function toWorld(sx, sy) {
        return { x: (sx - view.panX) / view.zoom, y: (sy - view.panY) / view.zoom };
    }

    function hitTest(mx, my) {
        var w = toWorld(mx, my);
        var r = 14 / view.zoom;   // stays ~14 screen px regardless of zoom

        for (var i = nodes.length - 1; i >= 0; i--) {
            var dx = nodes[i].x - w.x;
            var dy = nodes[i].y - w.y;
            if (dx * dx + dy * dy < r * r) return nodes[i];
        }
        return null;
    }

    function onDown(e) {
        drag = { x: e.clientX, y: e.clientY, panX: view.panX, panY: view.panY, moved: false };
        canvas.style.cursor = 'grabbing';
    }

    function onMove(e) {
        var rect = canvas.getBoundingClientRect();
        var mx = e.clientX - rect.left;
        var my = e.clientY - rect.top;

        if (drag) {
            var dx = e.clientX - drag.x;
            var dy = e.clientY - drag.y;
            if (Math.abs(dx) + Math.abs(dy) > 3) drag.moved = true;
            view.panX = drag.panX + dx;
            view.panY = drag.panY + dy;
            hideTip();
            return;
        }

        hovered = hitTest(mx, my);
        canvas.style.cursor = hovered ? 'pointer' : 'crosshair';

        if (hovered) showTip(hovered, mx, my);
        else hideTip();
    }

    function onUp() {
        if (!drag) return;
        // A drag that moved must not also register as a click on whatever node
        // happened to be under the cursor when the button came up.
        suppressClick = drag.moved;
        drag = null;
        canvas.style.cursor = hovered ? 'pointer' : 'crosshair';
    }

    function onLeave() {
        drag = null;
        hovered = null;
        hideTip();
        canvas.style.cursor = 'crosshair';
    }

    function onClick(e) {
        if (suppressClick) {
            suppressClick = false;
            return;
        }

        var rect = canvas.getBoundingClientRect();
        var node = hitTest(e.clientX - rect.left, e.clientY - rect.top);
        if (!node) return;

        selectedId = node.id;
        if (onSelect) onSelect(node.id);
    }

    function onWheel(e) {
        e.preventDefault();

        var rect = canvas.getBoundingClientRect();
        zoomAt(e.clientX - rect.left, e.clientY - rect.top,
            Math.exp(-e.deltaY * 0.0016));
    }

    function onTouchStart(e) {
        if (e.touches.length !== 2) return;
        var dx = e.touches[0].clientX - e.touches[1].clientX;
        var dy = e.touches[0].clientY - e.touches[1].clientY;
        pinch = { dist: Math.sqrt(dx * dx + dy * dy), zoom: view.zoom };
    }

    function onTouchMove(e) {
        if (e.touches.length !== 2 || !pinch) return;
        e.preventDefault();

        var dx = e.touches[0].clientX - e.touches[1].clientX;
        var dy = e.touches[0].clientY - e.touches[1].clientY;
        var dist = Math.sqrt(dx * dx + dy * dy);

        var rect = canvas.getBoundingClientRect();
        var mx = (e.touches[0].clientX + e.touches[1].clientX) / 2 - rect.left;
        var my = (e.touches[0].clientY + e.touches[1].clientY) / 2 - rect.top;

        var target = pinch.zoom * dist / pinch.dist;
        zoomAt(mx, my, target / view.zoom);
    }

    // zoomAt keeps the point under the cursor fixed while scaling around it.
    function zoomAt(px, py, factor) {
        var next = Math.max(ZOOM_MIN, Math.min(ZOOM_MAX, view.zoom * factor));
        var scale = next / view.zoom;

        view.panX = px - (px - view.panX) * scale;
        view.panY = py - (py - view.panY) * scale;
        view.zoom = next;

        reportZoom();
    }

    function zoomBy(factor) {
        zoomAt(lw / 2, lh / 2, factor);
    }

    function fit() {
        view.zoom = 1;
        view.panX = 0;
        view.panY = 0;
        reportZoom();
    }

    function reportZoom() {
        var label = document.getElementById('rc-zoom-level');
        if (label) label.textContent = Math.round(view.zoom * 100) + '%';
    }

    function showTip(node, mx, my) {
        var tip = document.getElementById('rc-map-tip');
        if (!tip) return;

        var d = node.device;
        var open = RC.openPorts(d);
        var name = RC.deviceName(d);

        var lines = ['<div style="color:var(--rc-accent);font-weight:600">' + RC.esc(d.ipv4) + '</div>'];
        if (name) lines.push('<div style="color:var(--rc-text);margin-top:2px">' + RC.esc(name) + '</div>');
        if (d.vendor) lines.push('<div style="color:var(--rc-text-3);font-size:10px;margin-top:1px">' + RC.esc(d.vendor) + '</div>');
        lines.push('<div style="font-size:10px;margin-top:5px;color:var(--rc-text-4)">' +
            '<span style="color:' + node.color + '">' + RC.esc((d.status || 'unknown').toUpperCase()) + '</span>' +
            ' · ' + (open.length ? open.length + ' open' : 'no open ports') + '</div>');

        tip.innerHTML = lines.join('');
        tip.style.display = 'block';
        tip.style.left = (mx + 14 + 200 > lw ? mx - 210 : mx + 14) + 'px';
        tip.style.top = (my - 12) + 'px';
    }

    function hideTip() {
        var tip = document.getElementById('rc-map-tip');
        if (tip) tip.style.display = 'none';
    }

    /* ── External control ───────────────────────────────────────────── */

    function setMode(next) {
        if (!MODES.some(function (m) { return m.id === next; })) return;
        mode = next;
        RC.store.set('reconya.mapMode', next);
        layout();
    }

    function setFilter(value) {
        filterText = (value || '').toLowerCase().trim();
    }

    function setShowOffline(next) {
        showOffline = !!next;
        RC.store.set('reconya.showOffline', showOffline ? 'true' : 'false');
    }

    function setShowIgnored(next) {
        showIgnored = !!next;
        RC.store.set('reconya.showIgnored', showIgnored ? 'true' : 'false');
    }

    function setSelected(id) {
        selectedId = id;
    }

    return (window.RCMap = {
        init: init,
        setData: setData,
        setMode: setMode,
        mode: function () { return mode; },
        modes: function () { return MODES; },
        setFilter: setFilter,
        setShowOffline: setShowOffline,
        showOffline: function () { return showOffline; },
        setShowIgnored: setShowIgnored,
        showIgnored: function () { return showIgnored; },
        setSelected: setSelected,
        zoomIn: function () { zoomBy(1.3); },
        zoomOut: function () { zoomBy(1 / 1.3); },
        fit: fit
    });
})();
