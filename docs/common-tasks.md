# Common Tasks

Concrete recipes. Background/rationale for each is in the linked doc.

## Add a Column to an Existing Table

1. Add `ALTER TABLE <table> ADD COLUMN <name> <type>` to the end of `db.InitializeSchema` (`backend/db/sqlite.go`), following the existing `log.Printf("Note: ... might already exist: %v", err)` pattern, do not touch the original `CREATE TABLE` block.
2. Add the field to the matching struct in `backend/models/`.
3. Update the repository's scan/insert SQL in `backend/db/sqlite_repositories.go` (or wherever that table's queries live) to read/write the new column.
4. See [database.md](database.md).

## Add a New Table

1. Add a `CREATE TABLE IF NOT EXISTS` block (plus its indexes) to `db.InitializeSchema`.
2. Add the repository interface to `backend/db/repository.go` and its SQLite implementation (new file `backend/db/<name>_repository.go`, following `alert_repository.go`).
3. Add the corresponding struct to `backend/models/`.
4. Wire a repository instance through `db.RepositoryFactory` in `cmd/main.go` if a service needs it.

## Add a New Domain Module

See [backend-patterns.md](backend-patterns.md#adding-a-new-module) for the full checklist: service file → repository (if needed) → wire into `cmd/main.go` → handlers/routes (if user-facing) → background goroutine (if periodic).

## Add a New API Endpoint

1. Add a handler method on `WebHandler` in `backend/internal/web/handlers.go` (or a new `<name>_handlers.go`, see `alert_handlers.go`).
2. Check auth at the top: `session, _ := h.sessionStore.Get(r, "reconya-session"); user := h.getUserFromSession(session)`, `401` if nil (skip for read-only endpoints only if every sibling read endpoint in that file also skips it, check first).
3. Register the route in `backend/internal/web/router.go`, under the `/api` subrouter. If the route takes a resource ID, use the existing UUID regex pattern from a neighboring route.
4. If the work is slow (a scan), kick it off with `go` and return a "queued" response immediately.

## Add a New Alert Rule

1. Add a pure `eval<Name>(in EvalInput) []Finding` function to `backend/internal/alert/rules.go`.
2. Add a case for it in `EvaluateStateful`.
3. Give each `Finding` a `DedupeKey` that's stable across repeated evaluations of the same condition (rule id + device/network id + whatever varies), this is the upsert key in the `alerts` table, see [database.md](database.md#alerts-upsert-pattern).
4. Add test cases to `rules_test.go`, no database needed, `eval*` functions are pure.

## Add a New Page

1. Add a route in `router.go`: `r.HandleFunc("/newpage", h.ServePage("newpage")).Methods("GET")`.
2. Add the page's content block to `backend/assets/templates/index.html` following the existing page-name-selected pattern.
3. Add a page-scoped JS file under `backend/assets/static/js/` if it needs client-side behavior, and/or HTMX attributes for simple fetch-and-swap interactions.

## Add a New Config/Env Var

1. Add the field to `config.Config` in `backend/internal/config/config.go`.
2. Read it in `LoadConfig()` using `envBool`/`envInt`/`os.Getenv` per the existing helpers, with a safe default (see [architecture.md](architecture.md#network-isolation) if it enables any outbound network call, those must default to `false`).
3. Document it in the README's Configuration section.

## Fix a Bug

1. Reproduce with a test first where practical (unit test in `rules_test.go`/model tests for pure logic, integration test in `backend/tests/integration/` if it needs the DB or a handler).
2. Fix.
3. `cd backend && go test ./...` and `go vet ./...`.

## Cut a Release

See [development-workflow.md](development-workflow.md#building-a-release-binary): `make release V=x.y.z`, then push the branch and the tag, `.github/workflows/release.yml` builds and publishes binaries for the tag.
