# Testing

## Framework

Standard `testing` package + `testify` (`assert`/`require`). No mocking framework, tests that need a database use a real, throwaway SQLite instance.

## Layout

Two patterns coexist:

- **Colocated unit tests**, `<file>_test.go` next to the code it covers, `package <same as prod code>`. Used for pure-logic packages: `backend/internal/alert/rules_test.go`, `backend/internal/scanner/onlink_test.go`, `backend/internal/webservice/screenshot_fallback_test.go`, `backend/internal/web/template_funcs_test.go`, `backend/models/device_test.go`.
- **Integration tests**, `backend/tests/integration/`, `package integration`, exercising repositories/services/handlers against a real SQLite database: `device_service_test.go`, `device_handlers_test.go`, `alert_repository_test.go`, `shared_test.go` (shared fixtures/helpers for that package).

When adding a test for pure logic (no DB, no HTTP), colocate it. When it needs the database or a handler round-trip, put it in `backend/tests/integration/`.

## Test Database

`backend/tests/testutils/database.go`:

```go
testDB, cleanup := testutils.SetupTestDatabase(t)        // real SQLite file in t.TempDir(), schema via db.InitializeSchema
defer cleanup()

factory, cleanup := testutils.SetupTestRepositoryFactory(t)  // same, wrapped in a RepositoryFactory
defer cleanup()
```

`testutils.GetTestConfig()` returns a `*config.Config` stub for services that need one. `backend/tests/testutils/fixtures.go` has builders like `CreateTestDevice()`, `CreateTestDeviceWithIP(ip)`, `CreateTestNetwork()`, prefer these over hand-rolling structs so tests stay consistent as models grow fields. `backend/tests/testutils/http.go` has HTTP test helpers for handler tests.

## Pattern

Arrange-Act-Assert, using `t.Run` subtests to group related cases under one setup (see `device_service_test.go`, `alert_repository_test.go`). Prefer `require` for setup/precondition checks that should abort the test immediately on failure, `assert` for the actual expectations so multiple assertions in one subtest all get reported.

## Running Tests

```bash
cd backend
go test ./...                    # everything
go test ./internal/alert/...     # one package
go test -run TestDeviceRepository_Integration ./tests/integration/...
go test -v ./...                 # verbose
```

CI (`.github/workflows/test.yml`) runs `go test ./...` with `CGO_ENABLED=1` on every PR to `master`, the cgo SQLite driver requires this locally too.

## What to Test

- New alert rules: add cases to `rules_test.go` covering the pure `eval*` function directly, no database needed.
- New repository methods: add an integration test in `backend/tests/integration/` using `SetupTestRepositoryFactory`.
- New handlers: follow `device_handlers_test.go` for the request/response round-trip pattern.
- Bug fixes: reproduce with a failing test first where practical, then fix.
