# Directory Structure

Only git-tracked paths are listed. `data/`, `logs/`, `reconya/`, `oui/`, `src/`, and the `*.db*` files at the repo root are local runtime artifacts (gitignored) left behind by `make start` / manual `go run`, they are not part of the source tree and can be ignored or deleted freely.

```
reconya/
├── backend/                      # Everything that ships in the binary
│   ├── cmd/main.go               # Entry point: wiring, background workers, HTTP server
│   ├── assets/
│   │   ├── assets.go             # go:embed directives (templates, static, version)
│   │   ├── templates/            # html/template source (index.html + standalone/login.html)
│   │   └── static/{css,js}/      # Console CSS/JS, embedded and served at /static/
│   ├── db/                       # SQLite connection, schema, repository implementations
│   │   ├── sqlite.go             # ConnectToSQLite, InitializeSchema (CREATE TABLE IF NOT EXISTS)
│   │   ├── repository.go         # Repository interfaces + RepositoryFactory
│   │   ├── sqlite_repositories.go
│   │   ├── alert_repository.go
│   │   ├── geolocation_repository.go
│   │   └── db_manager.go
│   ├── internal/                 # Domain packages, one per concern (see backend-patterns.md)
│   │   ├── alert/                # Alert rules: new device, risky port, unreachable, duplicate MAC...
│   │   ├── config/                # Env-var driven Config, network-isolation flags
│   │   ├── device/                # Device CRUD, status aging (online/idle/offline)
│   │   ├── eventlog/               # Append-only activity log
│   │   ├── fingerprint/            # OS/device-type fingerprinting from scan signals
│   │   ├── ipv6monitor/            # Passive IPv6 neighbor discovery
│   │   ├── network/                # Network (CIDR) CRUD and selection
│   │   ├── nicidentifier/          # Primary NIC detection, new-network suggestions
│   │   ├── oui/                     # MAC vendor lookup (bundled OUI DB, optional online refresh)
│   │   ├── pingsweep/               # ICMP/TCP/ARP sweep implementation
│   │   ├── portscan/                # Background port scanning + service/banner detection
│   │   ├── scan/                    # ScanManager: orchestrates sweeps, IPv6 monitor, alert eval
│   │   ├── scanner/                 # Low-level scan primitives shared by pingsweep/portscan
│   │   ├── settings/                # User-configurable settings (single row)
│   │   ├── systemstatus/            # Single-row status: detected NIC, selected network, geo cache
│   │   ├── util/                     # Shared helpers
│   │   ├── web/                      # WebHandler: routes, page rendering, /api/* handlers
│   │   └── webservice/                # Web-service (HTTP port) screenshot capture
│   ├── middleware/                 # LoggingMiddleware, CORS
│   ├── models/                     # Plain structs shared across layers (Device, Network, Alert, ...)
│   └── tests/
│       ├── integration/            # DB-backed handler/service/repository tests
│       ├── testutils/               # SetupTestDatabase, fixtures, HTTP test helpers
│       └── config/test.env
├── scripts/                        # Shell helpers invoked by the Makefile (start/stop/status/seed/bump-version)
├── .github/workflows/               # test.yml (go test ./... on PRs), release.yml (tag-triggered builds)
├── Makefile                         # make start/start-dev/stop/status/logs/build/release/help
├── install.sh                       # curl-pipe installer for pre-built binaries
├── VERSION                          # Plain-text version, injected via -ldflags at build time
├── index.html / CNAME               # GitHub Pages marketing landing page (reconya.com), not the app
└── screenshots/                     # README assets
```

## Critical Paths Quick Reference

| Task | Location |
|---|---|
| Add a new domain feature | `backend/internal/<feature>/` |
| Add/modify an API or page route | `backend/internal/web/router.go` + `handlers.go` |
| Change the DB schema | `backend/db/sqlite.go` (`InitializeSchema`) |
| Add/modify a repository method | `backend/db/repository.go` (interface) + `backend/db/sqlite_repositories.go` (impl) |
| Add a shared data type | `backend/models/` |
| Change background worker behavior | `backend/cmd/main.go` |
| Console frontend (HTML/CSS/JS) | `backend/assets/templates/`, `backend/assets/static/{css,js}/` |
| Add an env-configurable setting | `backend/internal/config/config.go` |
| Add a test | `backend/tests/integration/` or a `_test.go` next to the code it covers |
