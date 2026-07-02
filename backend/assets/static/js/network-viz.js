(function () {
    'use strict';

    var canvas, ctx;
    var lw = 0, lh = 600;
    var nodes = [];
    var gateway = { x: 0, y: 0, pulse: 0, sweep: 0, rot: 0 };
    var packets = [];
    var hitIdx = null;
    var rafId = null;
    var prevTs = 0;
    var MAX_NODES = 50;
    var ENTRY_MS  = 1200;

    // Zoom / pan
    var zoom = 1, panX = 0, panY = 0;
    var ZOOM_MIN = 0.4, ZOOM_MAX = 4;
    var pinchDist0 = null, pinchZoom0 = 1;

    var currentTs = 0;
    var ipackets = [];
    // y=118: node circle (r=20) starts at y=98, comfortably below the 73px glass navbar
    var internet = { x: 0, y: 118, connected: null, ip: '', prevConnected: null, lostAt: 0 };

    var activityEvents = [];

    // Theme-aware color palette — updated each render()
    var C = {};

    function updateColors() {
        var light = document.documentElement.getAttribute('data-theme') === 'light';
        if (light) {
            C.bg         = '#f2faf6';
            C.green      = '#059669';
            C.gridDot    = 'rgba(5,150,105,0.12)';
            C.vigOuter   = 'rgba(180,210,195,0.40)';
            C.orbitInner = 'rgba(5,150,105,0.20)';
            C.orbitOuter = 'rgba(5,150,105,0.11)';
            C.sweepAlpha = 0.10;
            C.sweepLine  = 'rgba(5,150,105,0.42)';
            C.edgeActive = 'rgba(5,150,105,0.68)';
            C.edgeInact  = 'rgba(100,116,139,0.38)';
            C.packetBase = 'rgba(5,150,105,';
            C.pulseBase  = 'rgba(5,150,105,';
            C.glowInner  = 'rgba(5,150,105,0.35)';
            C.glowMid    = 'rgba(5,150,105,0.10)';
            C.hexOuter   = 'rgba(5,150,105,0.55)';
            C.gwLabel    = 'rgba(5,150,105,0.70)';
            C.nodeGlow   = 'rgba(5,150,105,0.18)';
            C.ipOnline   = 'rgba(5,150,105,0.95)';
            C.ipOffline  = 'rgba(100,116,139,0.75)';
            C.bracket    = 'rgba(5,150,105,0.22)';
            C.statTotal  = 'rgba(5,150,105,0.65)';
            C.statOff    = 'rgba(100,116,139,0.80)';
            C.emptyText  = 'rgba(100,116,139,0.50)';
            C.warnRedFill = 'rgba(220,38,38,';
            C.warnRedEdge = 'rgba(220,38,38,0.55)';
            C.warnRedSolid = '#dc2626';
        } else {
            C.bg         = '#070d17';
            C.green      = '#10b981';
            C.gridDot    = 'rgba(16,185,129,0.09)';
            C.vigOuter   = 'rgba(0,0,0,0.55)';
            C.orbitInner = 'rgba(16,185,129,0.22)';
            C.orbitOuter = 'rgba(16,185,129,0.13)';
            C.sweepAlpha = 0.18;
            C.sweepLine  = 'rgba(16,185,129,0.55)';
            C.edgeActive = 'rgba(16,185,129,0.75)';
            C.edgeInact  = 'rgba(75,85,99,0.45)';
            C.packetBase = 'rgba(110,231,183,';
            C.pulseBase  = 'rgba(16,185,129,';
            C.glowInner  = 'rgba(16,185,129,0.65)';
            C.glowMid    = 'rgba(16,185,129,0.18)';
            C.hexOuter   = 'rgba(16,185,129,0.65)';
            C.gwLabel    = 'rgba(16,185,129,0.65)';
            C.nodeGlow   = 'rgba(16,185,129,0.28)';
            C.ipOnline   = 'rgba(16,185,129,0.95)';
            C.ipOffline  = 'rgba(107,114,128,0.70)';
            C.bracket    = 'rgba(16,185,129,0.22)';
            C.statTotal  = 'rgba(16,185,129,0.55)';
            C.statOff    = 'rgba(107,114,128,0.75)';
            C.emptyText  = 'rgba(107,114,128,0.40)';
            C.warnRedFill = 'rgba(239,68,68,';
            C.warnRedEdge = 'rgba(239,68,68,0.60)';
            C.warnRedSolid = '#ef4444';
        }
    }

    function sc(s) {
        var light = document.documentElement.getAttribute('data-theme') === 'light';
        switch ((s || '').toLowerCase()) {
            case 'online':  return light ? '#059669' : '#10b981';
            case 'idle':    return light ? '#d97706' : '#eab308';
            case 'offline': return light ? '#9ca3af' : '#374151';
            default:        return light ? '#6b7280' : '#4b5563';
        }
    }

    function easeOutCubic(t) { return 1 - Math.pow(1 - t, 3); }

    function hexPath(cx, cy, r, rot) {
        ctx.beginPath();
        for (var i = 0; i < 6; i++) {
            var a = i / 6 * Math.PI * 2 + (rot || 0);
            if (i === 0) ctx.moveTo(cx + Math.cos(a) * r, cy + Math.sin(a) * r);
            else         ctx.lineTo(cx + Math.cos(a) * r, cy + Math.sin(a) * r);
        }
        ctx.closePath();
    }

    function initNetworkViz() {
        canvas = document.getElementById('network-viz-canvas');
        if (!canvas) return;
        ctx = canvas.getContext('2d');
        var wrap = document.getElementById('network-viz-wrap');
        if (!wrap) return;

        function resize() {
            var dpr = window.devicePixelRatio || 1;
            lw = wrap.clientWidth  || window.innerWidth  || 800;
            lh = wrap.clientHeight || window.innerHeight || 600;
            canvas.style.width  = lw + 'px';
            canvas.style.height = lh + 'px';
            canvas.width  = Math.round(lw * dpr);
            canvas.height = Math.round(lh * dpr);
            ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
            gateway.x = lw / 2;
            gateway.y = lh / 2;
            zoom = 1; panX = 0; panY = 0;
            positionNodes();
        }
        resize();

        if (window.ResizeObserver) {
            new ResizeObserver(resize).observe(wrap);
        } else {
            window.addEventListener('resize', resize);
        }

        canvas.addEventListener('mousemove', onMove);
        canvas.addEventListener('mouseleave', onLeave);
        canvas.addEventListener('click', onClick);
        canvas.addEventListener('wheel', onWheel, { passive: false });
        canvas.addEventListener('touchstart',  onTouchStart, { passive: true });
        canvas.addEventListener('touchmove',   onTouchMove,  { passive: false });
        canvas.addEventListener('touchend',    function () { pinchDist0 = null; }, { passive: true });
        canvas.addEventListener('dblclick',    function () { zoom = 1; panX = 0; panY = 0; });

        if (rafId) cancelAnimationFrame(rafId);
        rafId = requestAnimationFrame(tick);

        fetchData();
        setInterval(fetchData, 15000);
        fetchInternetIP();
        setInterval(fetchInternetIP, 30000);
        pingInternet();
        setInterval(pingInternet, 3000);
        fetchActivity();
        setInterval(fetchActivity, 20000);
    }

    function pingInternet() {
        fetch('/api/ping-internet', { credentials: 'include' })
            .then(function (r) { return r.json(); })
            .then(function (d) {
                var wasConn = internet.connected;
                internet.connected = !!d.up;
                if (wasConn === true && !d.up) internet.lostAt = currentTs;
            })
            .catch(function () {
                var wasConn = internet.connected;
                internet.connected = false;
                if (wasConn === true) internet.lostAt = currentTs;
            });
    }

    function fetchInternetIP() {
        fetch('/api/dashboard-metrics', { credentials: 'include' })
            .then(function (r) { return r.json(); })
            .then(function (d) {
                var conn = !!(d.publicIP && d.publicIP !== 'N/A' && d.publicIP !== '');
                internet.ip = conn ? d.publicIP : '';
            })
            .catch(function () {});
    }

    function fetchActivity() {
        fetch('/api/event-logs', { credentials: 'include' })
            .then(function (r) { return r.json(); })
            .then(function (d) { activityEvents = (d.logs || []).slice(0, 8); })
            .catch(function () {});
    }

    function timeAgo(ts) {
        if (!ts) return '';
        var diff = Date.now() - new Date(ts).getTime();
        var m = Math.floor(diff / 60000);
        if (m < 1)  return 'now';
        if (m < 60) return m + 'm';
        var h = Math.floor(m / 60);
        if (h < 24) return h + 'h';
        return Math.floor(h / 24) + 'd';
    }

    function drawActivityFeed() {
        if (activityEvents.length === 0) return;
        var lineH   = 14;
        var maxDesc = 44;
        var x       = 30;
        var yBase   = lh - 36; // bottom bracket clearance

        ctx.textBaseline = 'bottom';
        ctx.textAlign    = 'left';
        ctx.font = '8px "Orbitron", monospace';
        ctx.fillStyle = C.bracket;
        ctx.fillText('ACTIVITY', x, yBase - activityEvents.length * lineH - 6);

        for (var i = activityEvents.length - 1; i >= 0; i--) {
            var ev   = activityEvents[i];
            var desc = ev.description || ev.Description || '';
            var ts   = ev.created_at  || ev.CreatedAt   || '';
            if (desc.length > maxDesc) desc = desc.slice(0, maxDesc) + '…';

            var row = yBase - (activityEvents.length - 1 - i) * lineH;
            var fresh = i === 0;

            ctx.font      = '9px "Roboto Mono", monospace';
            ctx.fillStyle = fresh ? C.ipOnline : C.ipOffline;
            ctx.fillText('· ' + desc, x, row);

            var tag = timeAgo(ts);
            if (tag) {
                ctx.fillStyle  = C.statOff;
                ctx.textAlign  = 'right';
                ctx.fillText(tag, x + 320, row);
                ctx.textAlign  = 'left';
            }
        }
    }

    function fetchData() {
        fetch('/api/device-list', { credentials: 'include' })
            .then(function (r) { return r.json(); })
            .then(function (d) { setDevices(d.devices || []); })
            .catch(function () {});
    }

    function positionNodes() {
        if (!lw) return;
        var cx = lw / 2, cy = lh / 2;
        var maxR = Math.min(cx * 0.82, cy * 0.78);
        // Tighter rings leave clear vertical space between internet node and device cloud
        var inner = maxR * 0.40, outer = maxR * 0.68;
        internet.x = cx;
        internet.y = 118;

        var active   = nodes.filter(function (n) { return n.status === 'online' || n.status === 'idle'; });
        var inactive = nodes.filter(function (n) { return n.status !== 'online' && n.status !== 'idle'; });

        function place(arr, r) {
            arr.forEach(function (n, i) {
                var a = (arr.length > 1 ? i / arr.length : 0.25) * Math.PI * 2 - Math.PI / 2;
                n.tx = cx + Math.cos(a) * r;
                n.ty = cy + Math.sin(a) * r;
            });
        }
        place(active, inner);
        place(inactive, outer);
    }

    function setDevices(devices) {
        var cap = devices.slice(0, MAX_NODES);
        var byId = {};
        nodes.forEach(function (n) { byId[n.id] = n; });

        var cx = gateway.x || lw / 2 || 300;
        var cy = gateway.y || lh / 2 || 240;

        nodes = cap.map(function (d) {
            var status = d.status || 'unknown';
            var open   = (d.ports || []).filter(function (p) { return p.state === 'open'; }).length;
            var ex = byId[d.id];
            if (ex) {
                ex.status = status;
                ex.ip     = d.ipv4  || '';
                ex.label  = d.name  || d.hostname || '';
                ex.mac    = d.mac   || '';
                ex.open   = open;
                ex.color  = sc(status);
                return ex;
            }
            return {
                id: d.id, ip: d.ipv4 || '',
                label: d.name || d.hostname || '',
                mac: d.mac || '', status: status,
                open: open, color: sc(status),
                x: cx, y: cy, tx: cx, ty: cy,
                initTs: null, isNew: true
            };
        });
        positionNodes();
    }

    function tick(ts) {
        var dt = Math.min(ts - (prevTs || ts), 50);
        prevTs = ts;
        currentTs = ts;

        gateway.pulse += dt * 0.003;
        gateway.sweep += dt * 0.00055;
        gateway.rot   += dt * 0.00035;

        // Internet state change detection
        if (internet.prevConnected === true && internet.connected === false) {
            internet.lostAt = ts;
        }
        internet.prevConnected = internet.connected;

        nodes.forEach(function (n) {
            // Refresh color each tick in case theme changed
            n.color = sc(n.status);

            if (n.isNew) {
                if (!n.initTs) n.initTs = ts;
                var ep = easeOutCubic(Math.min(1, (ts - n.initTs) / ENTRY_MS));
                n.x = gateway.x + (n.tx - gateway.x) * ep;
                n.y = gateway.y + (n.ty - gateway.y) * ep;
                if (ep >= 1) { n.isNew = false; n.x = n.tx; n.y = n.ty; }
            } else {
                n.x += (n.tx - n.x) * 0.06;
                n.y += (n.ty - n.y) * 0.06;
            }
        });

        if (Math.random() < 0.22) {
            var live = nodes.filter(function (n) { return n.status === 'online' && !n.isNew; });
            if (live.length) {
                var t = live[Math.floor(Math.random() * live.length)];
                packets.push({ id: t.id, t: 0, spd: 0.006 + Math.random() * 0.006 });
            }
        }
        packets = packets.filter(function (p) { p.t += p.spd; return p.t < 1; });

        if (internet.connected && Math.random() < 0.15) {
            ipackets.push({ t: 0, spd: 0.006 + Math.random() * 0.006 });
        }
        ipackets = ipackets.filter(function (p) { p.t += p.spd; return p.t < 1; });

        render();
        rafId = requestAnimationFrame(tick);
    }

    function render() {
        updateColors();
        ctx.clearRect(0, 0, lw, lh);

        // Background and fixed HUD are drawn in screen space
        drawBg();

        // Everything else is in zoomed/panned world space
        ctx.save();
        ctx.translate(panX, panY);
        ctx.scale(zoom, zoom);

        drawOrbitRings();
        drawSweep();
        drawInternetEdge();
        if (nodes.length > 0) { drawEdges(); drawPackets(); }
        if (internet.connected) drawInternetPackets();
        drawGatewayNode();
        if (nodes.length > 0) drawDevices();
        drawInternetNode();

        ctx.restore();

        // HUD overlays stay in screen space
        drawBrackets();
        drawStats();
        drawActivityFeed();
        drawZoomHint();
        drawInternetWarning();
    }

    function drawInternetEdge() {
        if (internet.connected === null) return;
        var conn = internet.connected;
        ctx.beginPath();
        ctx.moveTo(gateway.x, gateway.y);
        ctx.lineTo(internet.x, internet.y);
        ctx.strokeStyle = conn ? C.edgeActive : C.warnRedEdge;
        ctx.lineWidth = conn ? 2.0 : 1.5;
        ctx.setLineDash(conn ? [] : [5, 9]);
        ctx.stroke();
        ctx.setLineDash([]);
    }

    function drawInternetPackets() {
        var gx = gateway.x, gy = gateway.y;
        var ix = internet.x, iy = internet.y;
        ipackets.forEach(function (p) {
            var fade = Math.sin(p.t * Math.PI);
            for (var j = 0; j < 5; j++) {
                var pt = Math.max(0, p.t - j * 0.03);
                ctx.beginPath();
                ctx.arc(
                    gx + (ix - gx) * pt,
                    gy + (iy - gy) * pt,
                    3.5 - j * 0.5, 0, Math.PI * 2
                );
                ctx.fillStyle = C.packetBase + (fade * (1 - j / 5)) + ')';
                ctx.fill();
            }
        });
    }

    function drawInternetNode() {
        if (internet.connected === null) return;
        var cx = internet.x, cy = internet.y;
        var conn = internet.connected;
        var r = 20;
        var color = conn ? C.green : C.warnRedSolid;

        // Glow halo
        var g = ctx.createRadialGradient(cx, cy, 0, cx, cy, r * 2.8);
        g.addColorStop(0, conn ? C.glowInner : C.warnRedFill + '0.30)');
        g.addColorStop(1, 'rgba(0,0,0,0)');
        ctx.beginPath(); ctx.arc(cx, cy, r * 2.8, 0, Math.PI * 2);
        ctx.fillStyle = g; ctx.fill();

        // Globe circle
        ctx.beginPath(); ctx.arc(cx, cy, r, 0, Math.PI * 2);
        ctx.fillStyle = C.bg; ctx.fill();
        ctx.strokeStyle = color; ctx.lineWidth = 2; ctx.stroke();

        // Globe interior lines (clipped)
        ctx.save();
        ctx.beginPath(); ctx.arc(cx, cy, r - 1, 0, Math.PI * 2); ctx.clip();
        ctx.strokeStyle = color; ctx.lineWidth = 0.8; ctx.globalAlpha = 0.45;
        [-0.55, 0, 0.55].forEach(function (f) {
            var yl = cy + f * r;
            var hw = Math.sqrt(Math.max(0, r * r - (f * r) * (f * r)));
            ctx.beginPath(); ctx.moveTo(cx - hw, yl); ctx.lineTo(cx + hw, yl); ctx.stroke();
        });
        ctx.beginPath(); ctx.ellipse(cx, cy, r * 0.42, r - 1, 0, 0, Math.PI * 2); ctx.stroke();
        ctx.globalAlpha = 1;
        ctx.restore();

        ctx.fillStyle = color;
        ctx.font = '8px "Orbitron", monospace';
        ctx.textAlign = 'center'; ctx.textBaseline = 'top';
        ctx.fillText('INTERNET', cx, cy + r + 5);
        if (internet.ip) {
            ctx.font = '7px "Roboto Mono", monospace';
            ctx.fillStyle = conn ? C.ipOnline : C.warnRedFill + '0.85)';
            ctx.fillText(internet.ip, cx, cy + r + 16);
        }
    }

    var _warnDomState = null;
    function drawInternetWarning() {
        var connected = internet.connected;
        if (connected === _warnDomState) return;
        _warnDomState = connected;
        var el = document.getElementById('internet-down-warning');
        if (!el) return;
        if (connected === false) {
            el.classList.remove('hidden');
        } else {
            el.classList.add('hidden');
        }
    }

    function drawBg() {
        ctx.fillStyle = C.bg;
        ctx.fillRect(0, 0, lw, lh);

        var s = 28;
        ctx.fillStyle = C.gridDot;
        for (var x = s; x < lw; x += s)
            for (var y = s; y < lh; y += s) {
                ctx.beginPath(); ctx.arc(x, y, 0.6, 0, Math.PI * 2); ctx.fill();
            }

        var vg = ctx.createRadialGradient(lw / 2, lh / 2, lh * 0.15, lw / 2, lh / 2, Math.max(lw, lh) * 0.72);
        vg.addColorStop(0, 'rgba(0,0,0,0)');
        vg.addColorStop(1, C.vigOuter);
        ctx.fillStyle = vg; ctx.fillRect(0, 0, lw, lh);
    }

    function drawOrbitRings() {
        var cx = gateway.x, cy = gateway.y;
        var mr = Math.min(cx * 0.82, cy * 0.78);
        [mr * 0.40, mr * 0.68].forEach(function (r, i) {
            ctx.setLineDash([4, 8]);
            ctx.beginPath();
            ctx.arc(cx, cy, r, 0, Math.PI * 2);
            ctx.strokeStyle = i === 0 ? C.orbitInner : C.orbitOuter;
            ctx.lineWidth = 1;
            ctx.stroke();
            ctx.setLineDash([]);
        });
    }

    function drawSweep() {
        var cx = gateway.x, cy = gateway.y;
        var maxR = Math.max(lw, lh);
        var sweepW = 0.70, steps = 28;
        for (var i = 0; i < steps; i++) {
            var a0 = gateway.sweep - sweepW * (i / steps);
            var a1 = gateway.sweep - sweepW * ((i + 1) / steps);
            ctx.beginPath();
            ctx.moveTo(cx, cy);
            ctx.arc(cx, cy, maxR, a0, a1, true);
            ctx.closePath();
            ctx.fillStyle = C.pulseBase + ((1 - i / steps) * C.sweepAlpha) + ')';
            ctx.fill();
        }
        ctx.beginPath();
        ctx.moveTo(cx, cy);
        ctx.lineTo(cx + Math.cos(gateway.sweep) * maxR, cy + Math.sin(gateway.sweep) * maxR);
        ctx.strokeStyle = C.sweepLine;
        ctx.lineWidth = 1.5; ctx.stroke();
    }

    function drawEdges() {
        nodes.forEach(function (n) {
            var active = n.status === 'online';
            ctx.beginPath();
            ctx.moveTo(gateway.x, gateway.y);
            ctx.lineTo(n.x, n.y);
            ctx.strokeStyle = active ? C.edgeActive : C.edgeInact;
            ctx.lineWidth = active ? 2.0 : 1.2;
            ctx.setLineDash(active ? [] : [4, 8]);
            ctx.stroke();
        });
        ctx.setLineDash([]);
    }

    function drawPackets() {
        packets.forEach(function (p) {
            var node = null;
            for (var i = 0; i < nodes.length; i++) { if (nodes[i].id === p.id) { node = nodes[i]; break; } }
            if (!node) return;
            var fade = Math.sin(p.t * Math.PI);
            for (var j = 0; j < 5; j++) {
                var pt = Math.max(0, p.t - j * 0.03);
                ctx.beginPath();
                ctx.arc(
                    gateway.x + (node.x - gateway.x) * pt,
                    gateway.y + (node.y - gateway.y) * pt,
                    3.5 - j * 0.5, 0, Math.PI * 2
                );
                ctx.fillStyle = C.packetBase + (fade * (1 - j / 5)) + ')';
                ctx.fill();
            }
        });
    }

    function drawGatewayNode() {
        var cx = gateway.x, cy = gateway.y;

        for (var i = 0; i < 3; i++) {
            var rp = (gateway.pulse * 0.38 + i / 3) % 1;
            ctx.beginPath();
            ctx.arc(cx, cy, 18 + rp * 46, 0, Math.PI * 2);
            ctx.strokeStyle = C.pulseBase + ((1 - rp) * 0.55) + ')';
            ctx.lineWidth = 1.5; ctx.stroke();
        }

        var glow = ctx.createRadialGradient(cx, cy, 0, cx, cy, 42);
        glow.addColorStop(0,   C.glowInner);
        glow.addColorStop(0.4, C.glowMid);
        glow.addColorStop(1,   'rgba(0,0,0,0)');
        ctx.beginPath(); ctx.arc(cx, cy, 42, 0, Math.PI * 2);
        ctx.fillStyle = glow; ctx.fill();

        hexPath(cx, cy, 27, gateway.rot);
        ctx.strokeStyle = C.hexOuter; ctx.lineWidth = 2; ctx.stroke();

        hexPath(cx, cy, 18, gateway.rot + Math.PI / 6);
        ctx.fillStyle = C.bg; ctx.fill();
        ctx.strokeStyle = C.green; ctx.lineWidth = 2.5; ctx.stroke();

        ctx.fillStyle = C.green;
        ctx.font = 'bold 10px "Roboto Mono", monospace';
        ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
        ctx.fillText('GW', cx, cy);

        ctx.fillStyle = C.gwLabel;
        ctx.font = '8px "Orbitron", monospace';
        ctx.fillText('GATEWAY', cx, cy + 40);
    }

    function drawDevices() {
        nodes.forEach(function (n, idx) {
            var hov = (hitIdx === idx);
            var active = n.status === 'online' || n.status === 'idle';
            var r = hov ? (active ? 18 : 10) : (active ? 12 : 5);

            if (n.status === 'online') {
                var g = ctx.createRadialGradient(n.x, n.y, 0, n.x, n.y, r * 3.2);
                g.addColorStop(0, C.nodeGlow);
                g.addColorStop(1, 'rgba(0,0,0,0)');
                ctx.beginPath(); ctx.arc(n.x, n.y, r * 3.2, 0, Math.PI * 2);
                ctx.fillStyle = g; ctx.fill();
            }

            hexPath(n.x, n.y, r, Math.PI / 6);
            ctx.fillStyle = C.bg; ctx.fill();
            ctx.strokeStyle = n.color; ctx.lineWidth = hov ? 2.5 : (active ? 2 : 1); ctx.stroke();

            if (n.status === 'online') {
                ctx.beginPath(); ctx.arc(n.x, n.y, Math.min(3.5, r * 0.4), 0, Math.PI * 2);
                ctx.fillStyle = n.color; ctx.fill();
            } else {
                var xs = r * 0.55;
                ctx.strokeStyle = n.color; ctx.lineWidth = active ? 1.5 : 1;
                ctx.beginPath();
                ctx.moveTo(n.x - xs, n.y - xs); ctx.lineTo(n.x + xs, n.y + xs);
                ctx.moveTo(n.x + xs, n.y - xs); ctx.lineTo(n.x - xs, n.y + xs);
                ctx.stroke();
            }

            // Only label active (online/idle) nodes — offline labels flood the canvas
            if (active) {
                ctx.fillStyle = n.status === 'online' ? C.ipOnline : C.ipOffline;
                ctx.font = '10px "Roboto Mono", monospace';
                ctx.textAlign = 'center'; ctx.textBaseline = 'top';
                ctx.fillText(n.ip, n.x, n.y + r + 5);
            }
        });
    }

    function drawBrackets() {
        var px = 18, s = 22;
        var yTop = 84;          // below the 73px glass navbar
        var yBot = lh - 18;
        ctx.strokeStyle = C.bracket; ctx.lineWidth = 1.5;
        [
            [px,      yTop,  1,  1],
            [lw - px, yTop, -1,  1],
            [px,      yBot,  1, -1],
            [lw - px, yBot, -1, -1]
        ].forEach(function (b) {
            ctx.beginPath();
            ctx.moveTo(b[0] + b[2] * s, b[1]);
            ctx.lineTo(b[0], b[1]);
            ctx.lineTo(b[0], b[1] + b[3] * s);
            ctx.stroke();
        });
    }

    function drawStats() {
        var on  = nodes.filter(function (n) { return n.status === 'online'; }).length;
        var off = nodes.filter(function (n) { return n.status === 'offline'; }).length;
        var tot = nodes.length;

        if (tot === 0) {
            ctx.fillStyle = C.emptyText;
            ctx.font = '9px "Roboto Mono", monospace';
            ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
            ctx.fillText('RUN A SCAN TO POPULATE TOPOLOGY', lw / 2, lh - 32);
            return;
        }

        // x/y positioned below the glass navbar (73px) and inside the bracket inset
        var x = 30, y = 90;
        ctx.font = '9px "Orbitron", monospace';
        ctx.textAlign = 'left'; ctx.textBaseline = 'top';
        ctx.fillStyle = C.statTotal; ctx.fillText('TOTAL:   ' + tot, x, y);
        ctx.fillStyle = C.green;     ctx.fillText('ONLINE:  ' + on,  x, y + 15);
        ctx.fillStyle = C.statOff;   ctx.fillText('OFFLINE: ' + off, x, y + 30);
    }

    // Convert screen coords → world coords
    function toWorld(sx, sy) {
        return { x: (sx - panX) / zoom, y: (sy - panY) / zoom };
    }

    function hitTest(mx, my) {
        var w = toWorld(mx, my);
        var r = 24 / zoom; // hit radius stays ~24 screen-px regardless of zoom
        for (var i = nodes.length - 1; i >= 0; i--) {
            var n = nodes[i], dx = n.x - w.x, dy = n.y - w.y;
            if (dx * dx + dy * dy < r * r) return i;
        }
        return -1;
    }

    function onWheel(e) {
        e.preventDefault();
        var rect = canvas.getBoundingClientRect();
        var mx = e.clientX - rect.left;
        var my = e.clientY - rect.top;

        var delta = e.deltaY < 0 ? 1.12 : 1 / 1.12;
        var newZoom = Math.max(ZOOM_MIN, Math.min(ZOOM_MAX, zoom * delta));

        // Zoom toward cursor
        panX = mx - (mx - panX) * (newZoom / zoom);
        panY = my - (my - panY) * (newZoom / zoom);
        zoom = newZoom;
    }

    function onTouchStart(e) {
        if (e.touches.length === 2) {
            var dx = e.touches[0].clientX - e.touches[1].clientX;
            var dy = e.touches[0].clientY - e.touches[1].clientY;
            pinchDist0 = Math.sqrt(dx * dx + dy * dy);
            pinchZoom0 = zoom;
        }
    }

    function onTouchMove(e) {
        if (e.touches.length === 2 && pinchDist0 !== null) {
            e.preventDefault();
            var dx = e.touches[0].clientX - e.touches[1].clientX;
            var dy = e.touches[0].clientY - e.touches[1].clientY;
            var dist = Math.sqrt(dx * dx + dy * dy);
            var rect = canvas.getBoundingClientRect();
            var mx = (e.touches[0].clientX + e.touches[1].clientX) / 2 - rect.left;
            var my = (e.touches[0].clientY + e.touches[1].clientY) / 2 - rect.top;
            var newZoom = Math.max(ZOOM_MIN, Math.min(ZOOM_MAX, pinchZoom0 * dist / pinchDist0));
            panX = mx - (mx - panX) * (newZoom / zoom);
            panY = my - (my - panY) * (newZoom / zoom);
            zoom = newZoom;
        }
    }

    function drawZoomHint() {
        if (zoom === 1) return;
        ctx.fillStyle = C.statTotal;
        ctx.font = '8px "Orbitron", monospace';
        ctx.textAlign = 'right'; ctx.textBaseline = 'top';
        ctx.fillText(Math.round(zoom * 100) + '%  ·  dbl-click to reset', lw - 14, 22);
    }

    function makeEl(tag, styles, text) {
        var el = document.createElement(tag);
        Object.keys(styles).forEach(function (k) { el.style[k] = styles[k]; });
        if (text !== undefined) el.textContent = text;
        return el;
    }

    function onMove(e) {
        var rect = canvas.getBoundingClientRect();
        var mx = e.clientX - rect.left, my = e.clientY - rect.top;
        var i = hitTest(mx, my);
        hitIdx = i >= 0 ? i : null;
        canvas.style.cursor = i >= 0 ? 'pointer' : 'default';

        var tip = document.getElementById('network-viz-tip');
        if (!tip) return;
        if (i < 0) { tip.style.display = 'none'; return; }

        var light = document.documentElement.getAttribute('data-theme') === 'light';
        var n = nodes[i];
        var pl = (mx + 14 + 190 > lw) ? mx - 200 : mx + 14;
        tip.style.cssText = 'display:block;position:absolute;pointer-events:none;' +
            'left:' + pl + 'px;top:' + (my - 12) + 'px;' +
            'background:' + (light ? 'rgba(242,250,246,0.98)' : 'rgba(7,13,23,0.97)') + ';' +
            'border:1px solid ' + (light ? 'rgba(5,150,105,0.35)' : 'rgba(16,185,129,0.30)') + ';' +
            'border-radius:4px;padding:8px 10px;z-index:100;max-width:190px;line-height:1.5;';

        tip.textContent = '';

        var ipColor = light ? '#059669' : '#10b981';
        var nameColor = light ? '#111827' : '#d1d5db';
        var macColor  = light ? '#6b7280' : '#6b7280';

        tip.appendChild(makeEl('div', { font: 'bold 10px Orbitron,monospace', color: ipColor }, n.ip));
        if (n.label) tip.appendChild(makeEl('div', { font: "11px 'Roboto Condensed',sans-serif", color: nameColor, marginTop: '2px' }, n.label));
        if (n.mac)   tip.appendChild(makeEl('div', { font: "9px 'Roboto Mono',monospace",        color: macColor,  marginTop: '1px' }, n.mac));

        var meta = makeEl('div', { font: "9px 'Roboto Mono',monospace", marginTop: '5px' });
        var statusSpan = makeEl('span', { color: n.color }, n.status || 'unknown');
        var portsText = n.open > 0 ? n.open + ' open port' + (n.open > 1 ? 's' : '') : 'no open ports';
        meta.appendChild(statusSpan);
        meta.appendChild(document.createTextNode(' · ' + portsText));
        tip.appendChild(meta);
    }

    function onLeave() {
        hitIdx = null;
        canvas.style.cursor = 'default';
        var tip = document.getElementById('network-viz-tip');
        if (tip) tip.style.display = 'none';
    }

    function onClick(e) {
        var rect = canvas.getBoundingClientRect();
        var i = hitTest(e.clientX - rect.left, e.clientY - rect.top);
        if (i >= 0 && typeof loadDeviceModal === 'function') loadDeviceModal(nodes[i].id);
    }

    window.initNetworkViz   = initNetworkViz;
    window.updateNetworkViz = function (devices) { setDevices(devices); };
})();
