# Web Console

The server-rendered frontend: how the single-template page mechanism works, and the behavior of each console surface (dashboard/topology, devices, networks, logs, alerts, settings). See [code-conventions.md](code-conventions.md#frontend) for the file layout and [backend-patterns.md](backend-patterns.md#web-handler-pattern) for the handler pattern these all sit on top of.

## One Template, Client-Rendered Content

All six pages (`/`, `/devices`, `/networks`, `/logs`, `/alerts`, `/settings`) are served by the same handler factory, `h.ServePage(pageName)`, which renders a single parsed template, `backend/assets/templates/index.html`, passing `{Page: pageName, User: ...}`. `/login` is the only page with its own separate template (`standalone/login.html`), rendered outside this mechanism.

The mechanism is a hybrid: the Go template picks which HTML *skeleton* to emit via `{{if eq .Page "devices"}}...{{else if ...}}` branches on `.Page`, but each branch just emits an empty placeholder container (e.g. a loading div). There are no HTMX attributes anywhere in the current template, an earlier htmx/radial-map/device-modal-based implementation was fully replaced; `templateFuncMap()` is deliberately empty. All actual content population is client-side JS reading `data-page` off the shell element and fetching JSON from `/api/*`. Keep this in mind if you're tempted to add HTMX back for a new page fragment, it would be reintroducing a pattern the rest of the console has moved away from; follow the JS-fetch-and-render convention instead.

## Polling Cadence

`console.js` defines the poll intervals used across the console (`POLL`): devices every 15s, dashboard metrics every 15s, scan status every 5s, event log every 20s, alerts every 15s, and a 3s internet-reachability ping. The scan-control widget's own runtime clock (elapsed sweep time) ticks independently every 1s on its own timer so it doesn't need a full state poll to advance visibly.

## Dashboard / Topology Map (`/`)

`APIDashboardMetrics` scopes to the currently selected/active network if one is set (else all devices), and returns device counts (found/online/offline), public IP + geolocation (fetched on demand if not cached), and a saturation percentage (`devices in range / total host addresses in the CIDR × 100`). Alert counts and scan status are deliberately not part of this payload, they come from their own endpoints and are polled separately.

The topology map itself (`network-viz.js`) is a `<canvas>` (not SVG), fed by whatever the devices poll already fetched, there's no dedicated topology endpoint. Two layouts: TREE (gateway → core switch → one band per network sized by host count → hosts in rows) and RADIAL (one ring per network around a center gateway). Node shape encodes role (square = infrastructure, triangle = unidentified, circle = normal host) and fill color encodes status (green = online, dim green = idle, dark = offline, amber = unidentified). New nodes fly out from the gateway point over ~900ms when they first appear; the canvas runs its own animation loop independent of the poll cadence, so motion stays smooth between data refreshes.

**Clicking a device node opens the side drawer, not a modal or dropdown.** The drawer (`#rc-drawer`, OVERVIEW/PORTS/HISTORY/NOTES tabs) is a persistent panel slid into view via a CSS class toggle on the shell, populated by fetching `/api/devices/{id}/modal` (named for a historical modal implementation; it now backs the drawer). `dialog.js` (the shared `#rc-dialog` popup) is reserved for things that genuinely interrupt: adding/editing/deleting a network and the About panel, it is explicitly not used for device inspection anymore.

**The drawer head has two flag toggles** (`#rc-d-favorite` a star, `#rc-d-ignore` a stop icon) next to the close button, each a direct `PUT /api/devices/{id}` on click (`{is_favorite: ...}` / `{ignored: ...}`). The OVERVIEW tab's ADDRESSING row is click-to-cycle (`unknown → static → dhcp → unknown`), same endpoint. See [01-device-identification.md](01-device-identification.md#curation-flags-favorite--ignore--addressing) for what these flags actually do.

**Two dock-head toggles control what the dock list and topology map show**, both mirroring the same pattern: `#rc-offline-toggle` (SHOW OFFLINE, existing) and `#rc-ignored-toggle` (SHOW IGNORED, new). Both persist to `localStorage` (`reconya.showOffline` / `reconya.showIgnored`) via `RC.store`, and both are read by `network-viz.js` (`RCMap.showOffline()`/`showIgnored()`) and `console.js`'s `visibleDevices()`. A favorited device (`is_favorite`) is always shown regardless of the offline toggle; an ignored device is hidden unless the ignored toggle is on, regardless of its status.

**The SCAN PLAN panel (`#rc-subnets`, `console.js`'s `renderSubnetsFromState`) is per-range, not per-network.** A network with several CIDR ranges gets one row per range, each showing known/online hosts against the range's real address capacity (`RC.cidrSize`, computed client-side from the CIDR prefix rather than a server round-trip) and a running total across all active ranges. Clicking a row toggles that range in or out of future scans (`POST /api/networks/{id}/ranges/{rangeId}/toggle`), a persisted change, distinct from the segment tiles above it. `segments.js` renders those tiles (`#rc-segments`, one per range, fed by `/api/networks`) as a separate, purely client-side, ephemeral filter: clicking one scopes the device dock to that range's CIDR via `RC.ipInCidr` until clicked again or the page reloads, it does not touch the range's active state.

## Devices (`/devices`)

`APIDeviceList` scopes results to the active network server-side (filters by `NetworkID` when one is selected), specifically so a response never leaks another network's device inventory to the client. Response includes the device list, the screenshots-enabled flag, and the network list for the network filter.

`name` and `comment` are editable via `PUT /api/devices/{id}` (`APIUpdateDevice`); an empty string in the request means "leave unchanged," not "clear the field," there is currently no way to explicitly blank out a name/comment via this endpoint. `is_favorite`, `ignored`, and `addressing` are also editable through the same endpoint, but as real pointers, not the empty-string convention: a field simply absent from the JSON body is left untouched, an explicit `false`/`""` is applied. `addressing` must be `""`, `"static"`, or `"dhcp"`, anything else is a `400`. Device deletion (`DELETE /api/devices/{id}`) logs a `DeviceDeleted` event before removing the row.

## Networks (`/networks`)

Table of networks (ranges, name, description, host count, last-scanned, scanning state) with per-row edit/delete/scan actions and an "add network" action, all routed through the shared dialog for the add/edit form. The CIDR column shows every active range's CIDR (comma-joined, collapsing to `"<first> +N more"` past two) rather than a single value.

**The add/edit dialog (`network-list.js`) has a repeatable range list, not a single CIDR input.** Each row is a CIDR + optional label pair with a remove button; "+ ADD RANGE" appends another empty row. On submit, non-empty rows are collected into parallel `cidr[]`/`cidr_label[]` form fields (`application/x-www-form-urlencoded`, repeated keys), which `parseNetworkRangeForm` on the backend zips back into ranges. At least one CIDR is required; editing an existing network seeds one row per current range.

**The same dialog has a separate "Static ranges" textarea**, one CIDR per line, unrelated to the scan ranges above: these don't affect what gets scanned, they only feed `Network.AddressingForIP` to annotate devices STATIC/DHCP (see [01-device-identification.md](01-device-identification.md#curation-flags-favorite--ignore--addressing)). Submitted as repeated `static_range` form fields, parsed by `parseStaticRangesForm`; empty is valid (no static ranges configured).

**Detected-network suggestions are applied automatically, with no confirmation prompt.** Every 15 seconds, the console diffs `GET /api/detected-networks` against the existing network list and silently `POST`s any CIDR that's missing to `/api/network-suggestion`, auto-named `"Network <cidr>"`. "Already known" is checked against every range of every existing network (`networkOwnsCIDR` in `network-suggestions.js`), not just a network's legacy single-CIDR mirror field, otherwise a multi-range network could get a redundant duplicate suggested for a range that isn't its first one. This auto-add path always creates a new one-range network (mirrors `NetworkService.FindOrCreate`), it never appends a detected CIDR as an additional range onto an existing network, if you're asked to change that, it's new behavior, not a bug fix.

## Logs (`/logs`)

Full activity table via `GET /api/event-logs-table` (last 100 rows), polled every 20s. The dashboard's live event feed uses the smaller `GET /api/event-logs` (last 20), and a device's HISTORY drawer tab uses `GET /api/devices/{id}/events` (last 50 for that device). Event types are human-readable strings (`"Ping sweep"`, `"Device is now offline"`, `"New network detected"`, etc., see `models.EEventLogType`), not slugs, both the map's feed and the logs table switch on the literal string to choose an icon/color. The live feed collapses consecutive identical-type rows into a single "type ×N" entry client-side, so a burst of per-device events during a sweep doesn't flood the feed with duplicates.

## Alerts (`/alerts` + dashboard tile)

One `GET /api/alerts` fetch drives three surfaces: the top-bar summary tile, the feed panel (used both in the dashboard's dock and on the full `/alerts` page), and a critical-alert toast that surfaces the first not-yet-dismissed open critical alert. Toast dismissal is session-only client state, it is not an acknowledgement and does not touch the server; dismissing the toast just stops it from popping up again this session, the alert is still open server-side.

Clicking an alert row with an attached device opens that device's drawer instead of acknowledging it; clicking a row with no device acknowledges it directly. The page-level "acknowledge all" action (present only on the full `/alerts` page) hits `POST /api/alerts/ack-all`. See [03-alerting.md](03-alerting.md) for what acknowledgement actually changes server-side.

## Settings

Only one setting is actually wired up: `screenshots_enabled`, via `POST /api/settings/screenshots`. The settings page also displays a read-only "fixed values" block (sweep interval, idle/offline thresholds, alert threshold, port-rescan cooldown) as hardcoded display strings, not fetched from the server, and two maintenance actions (`cleanup-names`, `cleanup-network-broadcast`) that POST directly to their respective endpoints. An earlier version of this page had `scan_interval`/`concurrent_scans` fields that no backend handler ever read, they were removed rather than left as dead controls, on the reasoning that a control that silently does nothing is worse than no control. Don't reintroduce settings fields without wiring a handler to actually consume them.

## Authentication

Single static credential pair from `.env` (no user table, no password hashing) checked against a `reconya-session` cookie (`gorilla/sessions`, 30-day expiry, HttpOnly). Page routes re-check the session on every full navigation/reload and redirect to `/login` on failure; `/api/*` handlers independently return a bare `401` with no redirect.

**There is no client-side 401 interceptor.** If a session expires while the console is already open, the background polls (devices/metrics/scan/alerts/events) start failing silently (logged only to the browser devtools console); nothing redirects the user to `/login` until they trigger a full page navigation, at which point the server-side page-route check catches it. If you're asked to "fix" the console going stale after logout/session-expiry, this is the mechanism to add: a shared fetch wrapper that redirects on 401, there currently isn't one.
