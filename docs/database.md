# Database

SQLite only (`mattn/go-sqlite3`, cgo). No other database backend is supported: `internal/config.DatabaseType` has a single value, `SQLite`.

## Connection

`db.ConnectToSQLite` (backend/db/sqlite.go) opens the DB with WAL journaling and a set of concurrency-tuned pragmas (`busy_timeout=30000`, `synchronous=NORMAL`, `foreign_keys=ON`, `mmap_size=256MB`). Path comes from `SQLITE_PATH` (default `data/<DATABASE_NAME>.db`).

## Schema: no migration files

There is no `migrations/` directory and no migration runner. `db.InitializeSchema(db *sql.DB)` is called once at startup (`cmd/main.go`) and is the entire schema definition:

1. Each table is created with `CREATE TABLE IF NOT EXISTS ...`, safe to re-run, no-op if the table exists.
2. Columns added after a table's initial release are appended with `ALTER TABLE ... ADD COLUMN ...`, each wrapped so a "duplicate column" error (the column already exists on upgrade) is logged and ignored rather than treated as fatal.
3. Indexes are `CREATE INDEX IF NOT EXISTS`.

**To add a column:** append an `ALTER TABLE <table> ADD COLUMN <name> <type>` call at the end of `InitializeSchema`, following the existing `log.Printf("Note: ... might already exist: %v", err)` pattern, do not edit the original `CREATE TABLE` block, since that only runs on a brand-new database and existing installs would never get the column.

**To add a table:** add a new `CREATE TABLE IF NOT EXISTS` block (see `alerts` or `web_services` for a fully-worked example including its indexes), then add the corresponding repository interface method(s) in `backend/db/repository.go` and implementation in `backend/db/sqlite_repositories.go` (or a new `<name>_repository.go` file, following `alert_repository.go`).

There is no down-migration or rollback path: this is an intentionally append-only, backward-compatible schema evolution strategy suited to a single-file embedded database with no deploy pipeline.

## Tables

| Table | Purpose |
|---|---|
| `networks` | CIDR ranges the app knows about; `status`, `device_count`, `last_scanned_at` are denormalized for the networks page |
| `devices` | One row per discovered host: identity (IPv4/MAC/vendor/hostname), status, OS fingerprint fields, IPv6 addresses, scan timestamps. `ipv4` is uniquely indexed |
| `ports` | Open ports found on a device (`device_id` FK), one row per port |
| `event_logs` | Append-only activity feed (status transitions, scan events); indexed on `created_at DESC` since the UI always reads "latest N" |
| `system_status` | Single-row-ish table: detected NIC, selected/preselected network, cached public IP |
| `local_devices` | The local machine's own interface info, one row keyed to `system_status` |
| `web_services` | HTTP(S) services found on scanned ports, with optional screenshot path |
| `geolocation_cache` | Cached IP → geo lookups, TTL'd via `expires_at` and swept by the cleanup goroutine |
| `settings` | Single-row-per-user app settings (currently just `screenshots_enabled`) |
| `alerts` | Alert/finding rows. `dedupe_key` is `UNIQUE` and is the upsert target, see below |

## Alerts upsert pattern

Alert rules re-evaluate the *entire* current state on every pass (see [backend-patterns.md](backend-patterns.md#alert-rules-pattern)) and re-emit the full finding set rather than diffing against what's already stored. Writes go through `INSERT ... ON CONFLICT(dedupe_key) DO UPDATE` so a still-active finding bumps `occurrences`/`last_seen_at` in place instead of creating duplicate rows. If you add a new alert rule, its `Finding.DedupeKey` must be stable across re-evaluations of the same underlying condition (e.g. derived from rule id + device id + port), or you'll get one row per evaluation tick instead of one row per finding.

## Repository Layer

See [backend-patterns.md](backend-patterns.md#repository-pattern) for how services consume repositories. In short: interface in `repository.go`, SQLite implementation in `sqlite_repositories.go` / `alert_repository.go` / `geolocation_repository.go`, all handed out by `db.RepositoryFactory`.

## Testing against a real database

Tests do not mock the database. `backend/tests/testutils/database.go`'s `SetupTestDatabase(t)` opens a throwaway SQLite file in `t.TempDir()`, runs the real `db.InitializeSchema`, and returns a cleanup func, so integration tests exercise actual SQL, not a fake. See [testing.md](testing.md).
