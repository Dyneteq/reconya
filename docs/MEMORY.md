# Claude Memory & Preferences

Preferences and patterns specific to working with Claude Code on reconYa. This file is git-tracked and applies to anyone (human or AI) working from this clone; personal cross-project preferences live in Claude Code's own memory, not here.

---

## Commit Message Preferences

- **Do NOT add** `Co-authored-by` trailers.
- **No em dashes** in commit messages, code, or any generated text.
- No emojis in commit messages.
- Keep messages short and imperative (see `git log` for style, e.g. `Fix phantom devices, cumulative sweep duration, and scan counter`, `Console rewrite, alert system, and correct default scan target`). No `feat:`/`fix:` conventional-commit prefixes in use.
- There is no PR or issue template in `.github/`, write plain, direct descriptions.

---

## Scope

- **Cloud/SaaS hosting is out of scope.** reconYa's core function (ICMP/ARP/port scanning) requires direct access to a local network, so a hosted multi-tenant version isn't a meaningful product direction. Don't propose cloud deployment or multi-tenant architecture as a monetization or scaling path.
- **Outbound network calls default to off.** Any new integration that calls out to a third party (vendor lookup, geolocation, etc.) must be gated behind its own `false`-by-default config flag, see [architecture.md](architecture.md#network-isolation). This is a stated product property ("zero outbound calls by default"), not an oversight to "fix".

---

## Codebase Patterns to Remember

- **No migration files.** Schema changes are appended to `db.InitializeSchema` in `backend/db/sqlite.go` as `CREATE TABLE IF NOT EXISTS` / `ALTER TABLE ADD COLUMN`, guarded so re-running is a no-op. See [database.md](database.md).
- **`bson` struct tags in `backend/models/` are vestigial** (left over from a prior MongoDB backend). The database is SQLite-only now. Don't read their presence as evidence of a Mongo dependency, and don't feel obligated to keep adding them to new fields.
- **Background goroutines recover from their own panics** and log rather than propagate, a bug in one periodic task (device updater, alert evaluator, ...) must never take down the process. Follow this shape for any new one; see [architecture.md](architecture.md#background-workers).
- **Alert rules are pure functions** (`backend/internal/alert/rules.go`) separated from the I/O that persists them (`alert_service.go`). Keep new rules in that shape so they stay unit-testable without a database.
- **No mocking framework.** Tests that need a database use a real, throwaway SQLite file via `testutils.SetupTestDatabase`. See [testing.md](testing.md).

---

## Release Workflow

- `make release V=x.y.z` bumps `VERSION`, commits, and tags via `scripts/bump-version.sh`.
- Push both the branch and the `vx.y.z` tag; `.github/workflows/release.yml` builds Linux (Zig cross-compile) and macOS (native, amd64+arm64) binaries and publishes them.

---

**Last Updated:** 2026-08-11
**Maintained by:** Update as new patterns emerge or existing ones change.
