# Alerting

Alert rules run in `backend/internal/alert/`. Pure detection logic (`rules.go`) is separated from persistence (`alert_service.go`), see [backend-patterns.md](backend-patterns.md#alert-rules-pattern). This doc covers what actually triggers an alert, how dedupe/state work, and the API surface.

## Alert Model

`models.Alert` (`backend/models/alert.go`): `ID`, `RuleID`, `DedupeKey` (never serialized, `json:"-"`), `Severity`, `Title`, `Detail`, `DeviceID *string`, `NetworkID *string`, `State`, `Occurrences int`, `FirstSeenAt`, `LastSeenAt`, `AckedAt *time.Time`, `ResolvedAt *time.Time`, `CreatedAt`, `UpdatedAt`.

**State** (`AlertState`): `open`, `acked`, `resolved`. States are operator-controlled, not just a mirror of the underlying condition: rules re-emit the same finding on every evaluation pass, so state only changes when either the operator acts (ack) or the condition itself stops/resumes holding (resolve/reopen). See "Upsert and dedupe" below.

**Severity** (`AlertSeverity`): `critical`, `warning`, `notice`, `info`, ranked in that order (unranked values sort last).

## Rules

Six rule IDs: `new_device`, `unidentified_host`, `duplicate_mac`, `new_port`, `risky_port`, `host_unreachable`. Four of them (`unidentified_host`, `duplicate_mac`, `risky_port`, `host_unreachable`) are "stateful" and re-evaluated from scratch on every pass; `new_device` and `new_port` are event-triggered one-shots. See [database.md](database.md#alerts-upsert-pattern) for why this split matters for the dedupe key.

**A device with `Ignored == true` never fires any of the six rules** (see [01-device-identification.md](01-device-identification.md#curation-flags-favorite--ignore--addressing)): all four stateful `eval*` functions skip it in their per-device loop, and `scan.ScanManager` skips both `RecordNewDevice` and `RecordNewPorts` for it. The device is still swept and its status still updates, it just never generates a finding.

### unidentified_host

Fires when a *non-offline* device has **all** of: no vendor string, no hostname, no user-assigned name, zero open ports, **and** a locally-administered (randomized) MAC bit set. A randomized MAC alone is normal (many devices randomize for privacy) and is not treated as suspicious by itself; it only counts alongside a total absence of any other identifying signal. Severity `critical`. Dedupe key: `unidentified_host:<device_id>`.

### duplicate_mac

Groups all *online* devices by normalized MAC (lowercased, separators stripped, must resolve to exactly 12 hex characters). Any MAC shared by two or more currently-online devices fires once, attached to the device with the lowest IP in the group. Offline devices are excluded from the comparison on purpose: a device that got reassigned a new DHCP lease leaves a stale offline row behind, and without this exclusion that row would collide with its own live replacement. Severity `critical`. Dedupe key: `duplicate_mac:<normalized_mac>`.

### risky_port

Fires per open port on an online device that matches a hardcoded list:

| Port | Why it's flagged |
|---|---|
| 21 | FTP, credentials travel in clear text |
| 23 | Telnet, credentials travel in clear text |
| 445 | SMB, file sharing exposed to the whole segment |
| 3389 | RDP, remote desktop exposed to the whole segment |
| 5900 | VNC, remote desktop, frequently unauthenticated |
| 9100 | JetDirect, raw printing, unauthenticated by design |

Only ports currently in `open` state count; the same risky port sitting closed on a device does not fire. Severity `warning`. Dedupe key: `risky_port:<device_id>:<port>`.

### host_unreachable

Fires only for a device that is `offline` **and** has a recorded `LastSeenOnlineAt` (a device never seen online never fires this). The time since last-seen must be at least `ALERT_OFFLINE_THRESHOLD_HOURS` (env-configurable, default 6h) and, if set, no more than a fixed 7-day window, so a device gone for months doesn't re-trigger the same alert indefinitely on top of already being resolved. Severity `warning`. Dedupe key: `host_unreachable:<device_id>`.

### new_device / new_port

Not part of the stateful re-evaluation loop, they describe a moment, not an ongoing condition. Fired directly from `scan.ScanManager` right after `DeviceService.CreateOrUpdateWithDelta` reports `delta.IsNew` or a non-empty `delta.NewPorts`: `AlertService.RecordNewDevice` / `RecordNewPorts`. This is why the device-write path threads a `DeviceDelta` back to the caller instead of just returning the saved row: once the SQLite row is overwritten in place, "this device is new" and "this port just opened" cannot be recovered from the database afterward. Severity `critical` (new device) / dedupe key `new_device:<device_id>` and `new_port:<device_id>:<port>` respectively. A brand-new device never gets `new_port` alerts for its initial port set, only for ports that open on a later scan, since a first sighting's entire port list would otherwise be noise.

## Evaluation Flow

`AlertService.Evaluate` (mutex-guarded, so the ticker and a scan sweep can't run it concurrently):

1. Load all devices and all networks (network lookup is best-effort, used only for CIDR labels).
2. Run the four stateful `eval*` functions against that snapshot.
3. Upsert every returned finding (see below), tracking which dedupe keys were live in this pass, per rule.
4. For each stateful rule, auto-resolve any open/acked alert of that rule whose dedupe key was **not** in this pass's live set, the condition stopped holding.
5. Expire one-shots: any `new_device`/`new_port` alert whose `last_seen_at` is more than 24 hours old gets marked `resolved`, purely on age, no explicit "un-new" condition exists.

**Two triggers run `Evaluate`:**
- A 1-minute ticker (`runAlertEvaluator` in `cmd/main.go`), for conditions that cross a threshold purely from elapsed time (`host_unreachable`, one-shot expiry) with no scan activity involved.
- Once after every completed sweep in `scan.ScanManager`, evaluating **all** devices (not just the swept network), because auto-resolve is scoped per rule: evaluating a subset would incorrectly resolve valid findings belonging to networks that weren't part of this sweep.

## Upsert and Dedupe

Alerts are written via `INSERT ... ON CONFLICT(dedupe_key) DO UPDATE`, since rules re-emit their full finding set every pass rather than diffing against history. On conflict:

- `severity`, `title`, `detail`, `device_id`, `network_id`, `last_seen_at`, `updated_at` always refresh to the latest values.
- `state` is left alone, **unless** the existing row was `resolved`, in which case it flips back to `open`. This is deliberate: since rules re-emit their whole finding set after every pass, bumping an acked alert back to open on every re-fire would make acknowledgement pointless. A resolved alert whose condition returns is the one case that should reopen.
- `occurrences` increments only on that resolved-to-open transition, it counts recurrences of a condition, not raw sightings.

Net effect: acking an alert suppresses it indefinitely while the condition persists. If the condition later stops holding, auto-resolve (step 4 above) moves it to `resolved` regardless of prior ack state (its `WHERE` clause is simply "not already resolved"). If the condition then returns, the next upsert reopens it from `resolved` and bumps `occurrences`.

## API

All endpoints require an authenticated session.

- `GET /api/alerts`, query params `state` (repeatable; defaults to `open,acked` if omitted), `severity` (repeatable, no default filter), `limit` (default 100). Response: `{alerts: [...], counts: AlertCounts, open_count: int}`. `counts` is always computed over open alerts only, regardless of the `state` filter applied to `alerts`. There is no `device_id` filter on this endpoint even though the underlying query supports one.
- `POST /api/alerts/{id}/ack`, acknowledges one alert, `404` if the ID doesn't exist.
- `POST /api/alerts/ack-all`, acknowledges every currently-`open` alert (already-acked or resolved rows are untouched); returns the count acknowledged.
