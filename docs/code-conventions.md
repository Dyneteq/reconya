# Code Conventions

## Language

Backend is 100% Go. Frontend is server-rendered `html/template` plus small vanilla JS files and HTMX attributes, no TypeScript, no JS framework, no bundler.

## Go Style

- Standard `gofmt`/`go vet` formatting; run both before committing.
- Errors are wrapped with `fmt.Errorf("...: %w", err)` at each layer boundary (repository → service → handler) so context accumulates without losing the original error for `errors.Is`/`errors.As`.
- Sentinel errors are package-level vars, e.g. `db.ErrNotFound` in `backend/db/repository.go`.
- Constructors are `New<Type>(...)` returning `*Type`, taking dependencies as explicit arguments (see [backend-patterns.md](backend-patterns.md)), no DI framework, no globals for services.
- Enum-like values are typed string constants (e.g. `models.DeviceStatus`, `models.DeviceType` in `backend/models/device.go`), not raw strings or `iota` ints, keeps JSON output and SQL storage human-readable.
- Background loops (goroutines started in `cmd/main.go`) each wrap their body in a `defer recover()` and log rather than propagate, a bug in one periodic task must not take down the process. Follow this pattern for any new long-running goroutine.

## Comments

Comments in this codebase explain *why*, not *what*, see the header comments on `runDeviceUpdater`, `runAlertEvaluator`, and the `alerts` table definition in `sqlite.go` for the expected tone: they call out a non-obvious constraint or a past bug, not a restatement of the code. Prefer no comment over one that just narrates the following line.

## Models

`backend/models/*.go` are plain structs shared across the repository, service, and handler layers, no separate DTOs. Struct tags carry `json:"..."` for API responses; many structs also still carry `bson:"..."` tags left over from a prior MongoDB-backed version. The `bson` tags are vestigial, the database is SQLite-only now (see [database.md](database.md)), don't treat their presence as a sign MongoDB is in use, and no need to keep adding them to new fields.

## Naming

- Packages: short, lowercase, no underscores (`pingsweep`, `nicidentifier`, `eventlog`).
- Files within a package: `<name>_service.go`, `<name>_handlers.go`, `<name>_repository.go`, `<name>_test.go`.
- IDs are UUIDs (`github.com/google/uuid`), stored as `TEXT PRIMARY KEY`, routes constrain the `{id}` path variable with the UUID regex (see `router.go`), so don't switch any resource to an integer/auto-increment ID without updating those route patterns.

## API Conventions

- All `/api/*` routes require an authenticated session (`h.getUserFromSession`), returning `401` if absent, mutating handlers check this explicitly at the top of the function rather than through middleware.
- JSON responses are written by hand with `json.NewEncoder(w).Encode(...)`, not a shared response-envelope helper; match the shape already used by sibling endpoints in the same handler file rather than inventing a new envelope.
- Long-running actions (scans) return immediately with a "queued" acknowledgment and do the work in a `go` goroutine, see [backend-patterns.md](backend-patterns.md#web-handler-pattern).

## Frontend

- Templates: single `index.html` selecting content by page name, plus `standalone/login.html`. Keep new pages inside this pattern rather than introducing a second template entrypoint.
- Static JS is page-scoped, one file per concern (`devices.js`, `alerts.js`, `scan-control.js`, `network-viz.js`, ...) under `backend/assets/static/js/`. No shared JS framework/state store, DOM manipulation and `fetch` calls are direct.
- CSS lives in one file, `backend/assets/static/css/console.css`.
