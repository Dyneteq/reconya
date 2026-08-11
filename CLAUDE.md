# reconYa - Developer Guide

This documentation is designed for AI assistants and developers working with the reconYa codebase. For user-facing documentation, see [README.md](README.md). For contribution guidelines, see [CONTRIBUTING.md](CONTRIBUTING.md).

---

## Quick Start

reconYa is a self-hosted network reconnaissance and asset discovery tool: it sweeps a local subnet, fingerprints what answers, and keeps a live console (topology map, device list, alerts, event log) of the result.

**Tech Stack:** Go 1.25 backend (`net/http` + gorilla/mux), server-rendered HTML + HTMX + vanilla JS frontend, SQLite (embedded, no separate DB server)

**Get Started:**
```bash
git clone https://github.com/Dyneteq/reconya.git
cd reconya
make install
make start-dev   # http://localhost:3008, default login admin / password
```

---

## Documentation Index

1. **[Architecture Overview](docs/architecture.md)**
   - Tech stack, request flow, background workers
   - Data model hierarchy
   - Authentication
   - Network isolation (outbound calls are opt-in only)

2. **[Directory Structure](docs/directory-structure.md)**
   - Full tree of git-tracked paths
   - What `src/`, `reconya/`, `oui/`, `data/` are (gitignored local runtime artifacts, not source)
   - Critical paths quick reference

3. **[Backend Patterns](docs/backend-patterns.md)**
   - Module architecture (`backend/internal/<name>/`)
   - How to add a new module
   - Repository pattern
   - Alert rules pattern (pure functions vs. I/O)
   - Web handler and HTMX/frontend patterns

4. **[Database](docs/database.md)**
   - SQLite only, no migration files, schema evolves via idempotent `CREATE TABLE IF NOT EXISTS` / `ALTER TABLE ADD COLUMN`
   - Table reference
   - Alerts upsert/dedupe pattern

5. **[Development Workflow](docs/development-workflow.md)**
   - Setup, `make start-dev`, environment variables
   - Adding a feature end-to-end
   - Building and releasing (`make release V=x.y.z`)

6. **[Code Conventions](docs/code-conventions.md)**
   - Go style, error wrapping, naming
   - Why some models still carry `bson` tags
   - API and frontend conventions

7. **[Testing](docs/testing.md)**
   - Colocated unit tests vs. `backend/tests/integration/`
   - Real-SQLite test database via `testutils.SetupTestDatabase`
   - Running `go test ./...`

8. **[Common Tasks](docs/common-tasks.md)**
   - Add a column / table, new module, new API endpoint, new alert rule, new page, new config var
   - Fix a bug, cut a release

9. **[Claude Memory & Preferences](docs/MEMORY.md)**
   - Commit message conventions (no `Co-authored-by`, no em dashes, no emoji)
   - Scope boundaries (no cloud/SaaS, outbound calls opt-in by default)
   - Patterns to remember (no migrations, vestigial `bson` tags, pure alert rules)

### Feature Behavior

10. **[Network Scanning](docs/00-network-scanning.md)**
    - Ping sweep algorithm (ICMP/ARP/TCP corroboration rule), continuous 30s scan loop
    - Primary NIC detection and network auto-suggestion (never auto-creates)
    - IPv6 passive monitoring, and where its documented env vars don't actually wire up

11. **[Device Identification & Fingerprinting](docs/01-device-identification.md)**
    - Vendor lookup (two separate paths; the bundled IEEE OUI database is dead code)
    - Hostname resolution methods, device-type classification heuristics
    - New-vs-update matching (`CreateOrUpdateWithDelta`), why the delta exists

12. **[Port Scanning & Web Services](docs/02-port-scanning.md)**
    - Fixed 89-port list, replace-all port storage, 30s cooldown
    - Web service detection and the screenshot fallback chain
    - The screenshots-enabled setting doesn't actually control scan behavior (yet)

13. **[Alerting](docs/03-alerting.md)**
    - Each rule's exact trigger condition (unidentified host, duplicate MAC, risky port, host unreachable)
    - Dedupe/upsert mechanics, ack semantics, one-shot expiry
    - When evaluation runs (1-minute ticker + after every sweep)

14. **[Web Console](docs/04-web-console.md)**
    - Single-template-many-pages rendering mechanism
    - Per-page behavior: dashboard/topology, devices, networks, logs, alerts, settings
    - Polling cadence, and the missing client-side 401/session-expiry handling

---

## Project Overview

**Core Capabilities:**
- IPv4 network scanning (native Go ICMP/TCP/ARP), IPv6 passive monitoring (no traffic generated)
- Device identification: MAC/vendor, hostname, OS fingerprint, device type
- Background port scanning with service/banner detection
- Alerting: new devices, new/risky open ports, unreachable hosts, duplicate MACs, unidentified hosts
- Server-rendered web console: topology map, host list, live event log
- Multi-network support (multiple CIDRs)
- Zero outbound calls by default, every third-party lookup is an explicit opt-in flag

**Target Users:** Network administrators, security professionals, home users monitoring their own LAN.

---

## Technology Stack

**Backend:**
- Go 1.25, `net/http` + `gorilla/mux`
- SQLite via `mattn/go-sqlite3` (cgo, WAL mode)
- `gorilla/sessions` (cookie sessions), `joho/godotenv` (.env loading)
- Optional `chromedp` build tag for in-process screenshots (off by default; falls back to a system browser/tool)

**Frontend:**
- `html/template` (embedded via `//go:embed`), HTMX, vanilla JS (no bundler, no framework)

**Testing:**
- `testing` + `testify`, real-SQLite integration tests (no mocking framework)

---

## Critical Paths Quick Reference

| Task | Location |
|---|---|
| Add a domain feature | `backend/internal/<feature>/` |
| Add/modify a route | `backend/internal/web/router.go` + `handlers.go` |
| Change the DB schema | `backend/db/sqlite.go` (`InitializeSchema`) |
| Add a repository method | `backend/db/repository.go` (interface) + `sqlite_repositories.go` (impl) |
| Wire a new service | `backend/cmd/main.go` |
| Console frontend | `backend/assets/templates/`, `backend/assets/static/{css,js}/` |
| Add an env-configurable setting | `backend/internal/config/config.go` |
| Tests | `backend/tests/integration/` or colocated `_test.go` |

---

## Related Documentation

| Document | Audience | Purpose |
|---|---|---|
| [README.md](README.md) | Users | Features, installation, configuration, usage |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Contributors | PR workflow, coding standards |
| **CLAUDE.md** | Developers, AI | Codebase architecture, patterns (this file) |

---

**Document Version:** 1.0.0
**Last Updated:** 2026-08-11
**Maintainer:** Update when architecture changes or patterns evolve
