# Port Scanning and Web Services

Per-device port scanning and the web-service/screenshot capture that follows it. See [00-network-scanning.md](00-network-scanning.md) for the ping sweep that discovers devices in the first place.

## Port Scan

"Top-ports" (per the README) means a fixed, hardcoded list of 89 well-known and common application ports (`scanner.GetDefaultPorts()`), not an nmap-style `--top-ports N` computation and not a CIDR/range concept. Every port scan checks the same 89 ports regardless of device type.

**Two trigger paths, with different guarantees:**

- **Automatic**, after a ping sweep: for each newly-processed device, if `DeviceService.EligibleForPortScan` returns true, the scan runs as a background goroutine.
- **Manual**, via the console's RESCAN HOST button (`POST /api/devices/{id}/rescan`): runs the scan unconditionally, **bypassing the eligibility/cooldown check entirely**. A manual rescan always fires regardless of how recently the device was last scanned.

Scanning itself is a pure-Go TCP connect scan (`backend/internal/scanner`, no `nmap` dependency), up to 100 concurrent workers, 500ms dial timeout per port. Every open port gets a service label from a static ~130-entry port-to-name table, applied unconditionally, it's a lookup table, not a live probe. A smaller hardcoded subset of ports (21, 22, 23, 25, 80, 110, 143, 443, 587, 993, 995, 3306, 5432, 6379, 8080) additionally gets a banner grab (HTTP HEAD for the HTTP-ish ports, raw read otherwise; TLS ports like 443/8443 are explicitly skipped, grabbing a TLS banner would need a handshake this scanner doesn't implement).

**The banner is captured but never persisted**: `models.Port` has no banner field, so `PortScanService.ExecutePortScan` reads it and then discards it when building the `Port` rows it saves. If a future task needs banners visible in the UI or stored for alerting, that's new plumbing (add the column, thread it through), not a bug to "fix" in the grab logic itself.

## Port Storage: Replace-All, Not Diff

A device's port rows are only touched if the new scan found at least one open port: if it did, all existing rows for that device are deleted and the new set is bulk-inserted; if the scan found zero open ports, the old rows are left as-is. A device that briefly stops answering any port during one scan pass will not have its port list cleared, stale port rows can persist until a scan that finds at least one open port runs again. The alert-facing "newly opened ports" diff (`DeviceDelta.NewPorts`, see [03-alerting.md](03-alerting.md)) is computed separately, in memory, before this replace-all write happens, it is not derived from comparing database rows.

## Cooldown

`EligibleForPortScan` gates the automatic trigger only: a device is ineligible if `PortScanEndedAt + 30s` is still in the future. There is no equivalent gate on the manual rescan endpoint.

**Quirk worth knowing:** `ResetPortScanCooldowns` (clears `port_scan_started_at`/`port_scan_ended_at`/`web_scan_ended_at` on every device) runs unconditionally on every backend startup, in `cmd/main.go`, right after schema init. The log line and the function's own doc comment both say "for development," but there is no environment check gating it, it runs the same way in a production build. Given the cooldown itself is only 30 seconds, the practical impact is small, but don't assume this call is dev-only if you're touching startup sequencing.

**Also dead**: `Device.PortScanStartedAt` exists on the model and in the SQL read/write/reset paths, but nothing in the service layer ever assigns it. It is always `NULL` in practice, only `PortScanEndedAt` actually drives behavior.

## Web Service Detection

Runs after a port scan, over that device's now-populated `Ports`. A port is a web-service candidate if it's open and either its number is in a small hardcoded list (80, 443, 8080, 8443, 8000, 8008, 8081, 9000, 3000, 5000) or its service-name label contains "http" or "web". For each candidate, an HTTP GET (10s timeout, self-signed TLS certs accepted, response capped at 1MB) is attempted; only 2xx/3xx responses are kept, and the page `<title>` is extracted for HTML responses.

Web service rows follow the same replace-all-if-nonempty persistence pattern as ports. The stored `port`/`protocol` for a web service are re-derived from the response URL string via crude substring matching (recognizes `:80`, `:443`, `:8080`, `:8443` only, defaulting to port 80 otherwise), not carried through directly from the scan.

## Screenshots

Backend priority order, first success wins: **chromedp (only in `-tags chromedp` builds) → Firefox headless → Chrome/Chromium headless → wkhtmltoimage → webkit2png (macOS)**. This differs from the order the README lists (chrome/chromium, firefox, wkhtmltoimage, webkit2png), Firefox is actually tried before Chrome in the real fallback chain. Chrome discovery tries several binary names (`google-chrome`, `chromium`, `chromium-browser`, `chrome`) plus known absolute install paths before giving up. Each exec-based backend gets a hard 15-second kill timeout; window size is 1280x1024 across all backends.

A captured screenshot is written to a temp file under `/tmp/reconya-screenshots/`, immediately base64-encoded, and the temp file is deleted, so the `screenshot` column in `web_services` stores a **base64 PNG string**, nothing persists on disk after capture completes.

**The screenshots-enabled setting does not actually control screenshot capture during scans.** `Settings.ScreenshotsEnabled` (default `true`, toggle at `/settings`) only affects what the frontend is told via API responses (whether to show a screenshot placeholder in the UI); `PortScanService`'s own internal `ScreenshotsEnabled` field is hardcoded `false` at construction ("disabled for automated scans to improve performance") and nothing in the codebase ever flips it to `true`. In the current build, scans always take the no-screenshot code path regardless of the settings toggle. If asked to make the setting actually control capture, that means wiring `PortScanService.ScreenshotsEnabled` to read the per-user setting, it is a missing connection, not a regression.
