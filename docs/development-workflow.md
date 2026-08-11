# Development Workflow

## Initial Setup

```bash
git clone https://github.com/Dyneteq/reconya.git
cd reconya
make install     # creates backend/.env (admin/password default) and runs `go mod download && go mod tidy`
```

Requires Go 1.25+, a C compiler (cgo, for `mattn/go-sqlite3`), and `make`. On Windows, run with `CGO_ENABLED=1` and have TDM-GCC or MSVC Build Tools installed.

## Running Locally

```bash
make start-dev   # foreground, logs to stdout, Ctrl+C to stop, use this while developing
make start       # background daemon, logs to logs/reconya.log
make stop        # stop the daemon
make status      # check whether the daemon is running
make logs        # dump daemon log
make logs-follow # tail -f the daemon log
make logs-errors # dump the error log
```

Both `start` and `start-dev` run `go run` directly (no separate build step needed for day-to-day work) and inject the version via `-ldflags -X reconya/assets.Version=$(VERSION)`, where `VERSION` comes from the root `VERSION` file.

Equivalent manual invocation, if you want to skip the Makefile:

```bash
cd backend
go run -ldflags "-X reconya/assets.Version=dev" ./cmd
```

The server listens on `PORT` (default `3008`, i.e. http://localhost:3008), and logs in with `LOGIN_USERNAME` / `LOGIN_PASSWORD` from `backend/.env`.

Because there's no separate frontend build, editing anything under `backend/assets/templates/` or `backend/assets/static/` takes effect on the next request after a process restart (templates/static are embedded at compile time via `//go:embed`, not hot-reloaded), `go run` under `start-dev` recompiles on every restart, so just Ctrl+C and rerun after a template/JS change.

## Environment Variables

Set in `backend/.env` (see README for the full list). The ones that matter for development:

| Var | Default | Notes |
|---|---|---|
| `LOGIN_USERNAME` / `LOGIN_PASSWORD` | required, no default | single-user login |
| `DATABASE_NAME` | required, no default | also used to derive the default SQLite filename |
| `SQLITE_PATH` | `data/<DATABASE_NAME>.db` | |
| `PORT` | `3008` | |
| `PUBLIC_IP_LOOKUP_ENABLED`, `GEOLOCATION_ENABLED`, `VENDOR_LOOKUP_ONLINE_ENABLED`, `OUI_DOWNLOAD_ENABLED` | all `false` | opt-in outbound calls, see [architecture.md](architecture.md#network-isolation) |
| `ALERT_OFFLINE_THRESHOLD_HOURS` | `6` | how long a device must be unreachable before `host_unreachable` fires |
| `IPV6_MONITORING_ENABLED` and related `IPV6_*` vars | see README | passive IPv6 discovery tuning |

Network scanning (ICMP/ARP) generally needs elevated privileges, the built binary is typically run with `sudo`; `go run`/`make start-dev` may need it too depending on your OS.

## Adding a Feature: Typical Walkthrough

1. Decide whether it's a new module (`backend/internal/<name>/`) or belongs in an existing one, see [backend-patterns.md](backend-patterns.md).
2. Add/modify the schema in `backend/db/sqlite.go` if it needs persistence, see [database.md](database.md).
3. Add repository interface + implementation if needed.
4. Implement the service.
5. Wire the service into `backend/cmd/main.go`.
6. Add handler methods + routes in `backend/internal/web/` if it's user-facing.
7. Add/update templates and static JS if it needs UI.
8. Write tests, see [testing.md](testing.md).
9. `cd backend && go build ./...` and `go vet ./...` before committing.

## Building a Release Binary

```bash
make build       # backend/reconya, CGO_ENABLED=1, version-stamped
```

Actual release binaries for Linux/macOS (amd64+arm64) are produced by `.github/workflows/release.yml`, triggered by pushing a `vX.Y.Z` tag:

```bash
make release V=x.y.z    # runs scripts/bump-version.sh: bumps VERSION, commits, tags
git push origin <branch> vx.y.z
```

## Other Useful Commands

```bash
make deps             # go mod download && go mod tidy
make clean            # go clean + remove built binary and pid file
make prune-phantoms   # delete stale offline-device rows with no corroborating data (dry-run by default, CONFIRM=1 to apply)
make help             # full command list
```
