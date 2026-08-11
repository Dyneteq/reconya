# Backend Patterns

## Module Architecture

Each concern lives in its own package under `backend/internal/<name>/`. There is no shared "controllers" or "services" top-level folder, each module owns its full vertical slice (service + optional handlers), and `backend/internal/web` is the only package that talks HTTP.

A typical module (`backend/internal/device/`):

```
device_service.go    # DeviceService: business logic, talks to db.DeviceRepository
device_handlers.go   # thin handler helpers used by web.WebHandler, if any
device_updater.go    # the periodic status-aging logic invoked from cmd/main.go
```

Modules depend on each other by taking constructor arguments, not by reaching into globals, e.g. `device.NewDeviceService` takes a `*network.NetworkService`, `alert.NewAlertService` takes `device.DeviceReader`/`network.NetworkReader` interfaces it defines itself. `backend/cmd/main.go` is the single place that constructs every service in dependency order and wires them together; nothing else does this. When adding a module, follow that pattern: define what you need from other packages as a small interface in your own package rather than importing their concrete service type where a narrower interface will do (see `alert.DeviceReader` in `alert_service.go`).

## Adding a New Module

1. Create `backend/internal/<name>/<name>_service.go` with a `<Name>Service` struct and a `New<Name>Service(...)` constructor that takes its dependencies (a repository, other services it needs, `*config.Config`) as arguments.
2. If it needs persistence, add the repository interface to `backend/db/repository.go` and the SQLite implementation to `backend/db/sqlite_repositories.go` (see [database.md](database.md)).
3. Wire it up in `backend/cmd/main.go`: construct the repository via `repoFactory`, construct the service, pass it to anything downstream that needs it (usually `web.NewWebHandler`).
4. If it needs HTTP endpoints, add handler methods on `WebHandler` in `backend/internal/web/handlers.go` (or a new `<name>_handlers.go` in that package, following `alert_handlers.go`) and register routes in `backend/internal/web/router.go`.
5. If it needs a periodic background pass, add a `run<Name>` goroutine function in `main.go` following the existing ones (own panic recovery, `ticker`, `done` channel for shutdown).

## Repository Pattern

`backend/db/repository.go` defines one Go interface per aggregate (`DeviceRepository`, `NetworkRepository`, `EventLogRepository`, `AlertRepository`, ...), each embedding the base `Repository` interface (`Close() error`). Concrete SQLite implementations live in `backend/db/sqlite_repositories.go`, `alert_repository.go`, and `geolocation_repository.go`. `db.RepositoryFactory` (constructed once in `main.go` from the open `*sql.DB`) hands out repository instances, services take a repository *interface*, never the factory or the raw `*sql.DB`, so they stay testable against `testutils.SetupTestDatabase`.

Repositories take a `context.Context` as their first argument; most service methods above them do not thread a context through and instead call `context.Background()` or similar at the repository call site, match whatever the surrounding function already does rather than introducing a mix.

## Alert Rules Pattern

`backend/internal/alert/` is the clearest example of separating pure logic from I/O:

- `rules.go`, pure functions (`evalUnidentifiedHosts`, `evalDuplicateMACs`, `evalRiskyPorts`, `evalHostUnreachable`) that take an `EvalInput` (in-memory snapshot of devices/networks) and return `[]Finding`. No database access, fully unit-testable (see `rules_test.go`).
- `alert_service.go`, `AlertService.Evaluate` loads state through `DeviceReader`/`NetworkReader`, calls `EvaluateStateful`, and persists resulting `Finding`s as alert rows, deduplicating and expiring one-shot alerts.

New alert rules should be added as another pure `eval*` function plus a case in `EvaluateStateful`, not folded into the service's I/O code.

## Web Handler Pattern

`WebHandler` (backend/internal/web/handlers.go) holds references to every service and is the only place that constructs HTTP responses:

- Page routes call `h.ServePage("name")`, which renders `backend/assets/templates/index.html` (a single template selecting content by page name) against the embedded template FS.
- `/api/*` routes are one method per endpoint, e.g. `APIUpdateDevice`, `APIScanStart`. They: pull the session via `h.sessionStore.Get(r, "reconya-session")`, resolve the user with `h.getUserFromSession(session)` and return 401 if nil, do minimal request parsing, call into a service, and write JSON with `json.NewEncoder(w).Encode(...)`.
- Anything slow (a port scan, a rescan) is kicked off with `go h.someService.Run(...)` and the handler returns immediately with a "queued" response; the frontend polls for the result rather than the request blocking on it. Follow this pattern for any new handler that triggers scanning work.

## Frontend (HTMX) Pattern

There is no JS framework and no build step. `backend/assets/static/js/*.js` are small, page-scoped vanilla JS files (`devices.js`, `alerts.js`, `network-viz.js`, `scan-control.js`, ...) that call the `/api/*` endpoints (`fetch`) and patch the DOM, plus HTMX attributes in the templates for simple swap-on-response cases. When adding a UI feature, prefer an HTMX attribute on the element for simple fetch-and-swap; reach for a new/extended JS file only when you need client-side state (e.g. the topology map, scan control polling).
