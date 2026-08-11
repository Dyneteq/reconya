# Architecture Overview

## Tech Stack

- **Backend:** Go 1.25, standard `net/http` + [gorilla/mux](https://github.com/gorilla/mux) for routing
- **Frontend:** Server-rendered HTML templates (`html/template`) + HTMX for partial updates, vanilla JS for the topology map and interactive widgets
- **Database:** SQLite (via `mattn/go-sqlite3`, cgo required), WAL mode
- **Sessions:** `gorilla/sessions`, cookie-based
- **Embedding:** Templates, static assets, and the OUI vendor database are compiled into the binary with `//go:embed`, there is no separate frontend build step

There is no API/frontend split in the SPA sense. `backend/internal/web` renders full pages and HTMX fragments from the same handler layer that owns the domain services.

## Request Flow

```
Browser
  │  GET/POST (page load, HTMX fragment, or /api/* call)
  ▼
middleware.LoggingMiddleware
  ▼
gorilla/mux Router (backend/internal/web/router.go)
  │
  ├─ Page routes ("/", "/devices", "/networks", ...) → h.ServePage(name)
  │     renders backend/assets/templates/index.html against html/template
  │
  ├─ Auth routes ("/login", "/logout") → session cookie via gorilla/sessions
  │
  └─ /api/* routes → WebHandler methods (handlers.go, alert_handlers.go)
        │
        ▼
     internal/<domain> services (device, network, alert, scan, portscan, ...)
        │
        ▼
     db.RepositoryFactory → SQLite repositories (backend/db)
        │
        ▼
     reconya.db (SQLite, WAL)
```

`WebHandler` (backend/internal/web/handlers.go) is the composition root for the web layer: `main.go` constructs every service, then wires them all into one `WebHandler` via `web.NewWebHandler(...)`, and `SetupRoutes()` attaches its methods to routes.

## Background Workers

`backend/cmd/main.go` starts several long-running goroutines alongside the HTTP server. They are the actual engine of the app, the web layer is mostly a view onto state these loops maintain:

| Goroutine | Interval | Responsibility |
|---|---|---|
| `runDeviceUpdater` | 5s | Ages devices from online → idle → offline, writes event log transitions |
| `runNetworkDetection` (`nicidentifier`) | 30s | Detects the primary NIC and suggests/creates networks for new interfaces |
| `runGeolocationCacheCleanup` | 6h (+ once at startup) | Expires stale geolocation cache rows |
| `runAlertEvaluator` (`internal/alert`) | 1m | Re-runs stateful alert rules that fire on elapsed time, not scan events |
| `scan.ScanManager` (`internal/scan`) | on demand / selected network | Drives ping sweeps (`internal/pingsweep`), IPv6 monitoring (`internal/ipv6monitor`), and triggers alert evaluation after each sweep |
| `portscan.PortScanService` | per-device, backgrounded from handlers | Runs port scans off the request path; results land via the device/event-log services |

All of them recover from panics individually and log rather than crash the process; `main()` itself also recovers from a top-level panic and restarts.

## Data Model Hierarchy

```
Network (CIDR)
  └─ Device (IPv4/MAC/vendor/status/hostname)
       ├─ Port (open ports, service, banner)
       ├─ WebService (screenshot metadata for HTTP(S) ports)
       └─ EventLog (status transitions, scan events, alerts)

Alert (device- or network-scoped, acknowledgeable)
SystemStatus (single-row: detected NIC, selected network, public IP/geo cache)
Settings (single-row: user-configurable toggles)
User (login credentials, currently a single admin account from env vars)
```

See [database.md](database.md) for the actual table definitions.

## Authentication

Single-user, credential-based login (`LOGIN_USERNAME` / `LOGIN_PASSWORD` from `.env`, no self-registration). A successful `/login` POST sets a `reconya-session` cookie via `gorilla/sessions`; `WebHandler.getUserFromSession` gates every mutating `/api/*` handler and the page routes redirect to `/login` when the session is missing.

## Network Isolation

By design, reconYa makes zero outbound network calls unless explicitly enabled. Every optional integration is a separate `bool` in `internal/config.Config`, defaulting to `false`:

| Config field | Env var | Calls out to |
|---|---|---|
| `PublicIPLookupEnabled` | `PUBLIC_IP_LOOKUP_ENABLED` | api.ipify.org |
| `GeolocationEnabled` | `GEOLOCATION_ENABLED` | ip-api.com |
| `VendorLookupEnabled` | `VENDOR_LOOKUP_ONLINE_ENABLED` | api.macvendors.com |
| `OUIDownloadEnabled` | `OUI_DOWNLOAD_ENABLED` | standards-oui.ieee.org |

Keep this property intact when touching `internal/config`, `internal/oui`, `internal/nicidentifier`, or `internal/systemstatus`, a new outbound call needs its own opt-in flag, not an always-on default.
