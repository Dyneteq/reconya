# Device Identification and Fingerprinting

How a discovered host becomes a `Device` row: vendor lookup, hostname resolution, type classification, and the new-vs-update matching logic. See [00-network-scanning.md](00-network-scanning.md) for how a host gets discovered in the first place.

## Device Model

`models.Device` (`backend/models/device.go`) carries identity (`IPv4`, three IPv6 address fields plus `IPv6Addresses []string`, `MAC`, `Vendor`, `Hostname`), `Status` (`unknown`/`online`/`idle`/`offline`), `DeviceType`, an embedded `OS` struct (`Name`/`Version`/`Family`/`Confidence`), scan-state timestamps (`LastSeenOnlineAt`, `PortScanStartedAt`, `PortScanEndedAt`, `WebScanEndedAt`), three user-curation flags: `IsFavorite`, `Ignored`, `Addressing` (see below), and two long-term tracking fields: `FirstSeenAt`, `DiscoveryMethod` (see below).

## Curation Flags (Favorite / Ignore / Addressing)

Three per-device fields, all editable via `PUT /api/devices/{id}` (`is_favorite`, `ignored`, `addressing` in the JSON body), each a genuine pointer server-side so an absent field is distinguishable from an explicit `false`/`""`:

- **`IsFavorite`** (bool): purely a display hint. The console's "show offline" filter (dock list and topology map) keeps a favorited device visible even when it's offline and the global toggle is off. Nothing in the scan or alert path reads it.
- **`Ignored`** (bool): the device still gets swept and its `Status`/`LastSeenOnlineAt` still update every cycle (see [00-network-scanning.md](00-network-scanning.md)), so it never looks "gone". What's skipped: `EligibleForPortScan` returns `false` for an ignored device (no port scan is ever queued for it), and `scan_manager.runSingleScan` doesn't call `RecordNewDevice`/`RecordNewPorts` for it. The stateful alert rules (`evalUnidentifiedHosts`, `evalDuplicateMACs`, `evalRiskyPorts`, `evalHostUnreachable`, see [03-alerting.md](03-alerting.md)) also skip any device with `Ignored == true`. The console hides ignored devices from the dock and map by default, behind a "SHOW IGNORED" toggle mirroring the existing "SHOW OFFLINE" one.
- **`Addressing`** (`""`/`static`/`dhcp`): annotates how a device's address is assigned. It's derived automatically from the owning `Network.StaticRanges` (a list of CIDRs, edited per-network alongside the scan ranges): a device whose IP falls inside one of them gets `static`, otherwise `dhcp` once at least one static range is configured, or `""` if the network has none configured. A manual override (set via the drawer's ADDRESSING control) always wins and is preserved across future sweeps, exactly like a user-set `Name`/`Comment`, see the preserve logic below.

Because a ping-sweep-built `models.Device{}` always carries the zero value for these three fields, `CreateOrUpdateWithDelta` (below) always carries `IsFavorite`/`Ignored` forward from the existing record, and re-derives `Addressing` from the network's `StaticRanges` only when the existing value is still `""` (unset) — a manual override is never clobbered by a later sweep.

## Long-Term Tracking (First Seen / Discovery Method)

Two per-device fields, both read-only (not part of `PUT /api/devices/{id}`, unlike the curation flags above) and surfaced in the drawer's OVERVIEW tab:

- **`FirstSeenAt`** (timestamp): set once, the first time `CreateOrUpdateWithDelta` sees a device it has no existing record for (by IP, or by MAC on an IP change), and never touched again — `setTimestamps` and the repository's update-path preserve both enforce this, the same "preserve unless still zero" shape `CreatedAt` uses. Distinct from `CreatedAt`, which is documented as upsert-preserved but carries no explicit immutability contract; `FirstSeenAt` is that contract made concrete. Existing rows from before this column existed are backfilled once from `created_at` (see [database.md](database.md)).
- **`DiscoveryMethod`** (`""`/`icmp`/`arp`/`tcp`): which signal corroborated the *most recent* sighting, per the same trust ordering the ping sweep already uses (see [00-network-scanning.md](00-network-scanning.md)) — ICMP and ARP are trusted directly, TCP only when off-link. Set in `NativeScanner.scanIP`, threaded through `ScanResult.Method` into the freshly-built `models.Device`, and persisted unconditionally on every sweep (no preserve logic, unlike the curation flags): it reflects "how was this device last found," not "how was it first found."

`DeviceType` declares 14 values (`router`, `switch`, `nas`, `printer`, `camera`, `server`, `workstation`, `laptop`, `mobile`, `iot`, `access_point`, `firewall`, `voip`, `unknown`), but the classification logic (below) can only ever produce 8 of them: `router`, `nas`, `printer`, `camera`, `mobile`, `server`, `workstation`, `voip`. `switch`, `laptop`, `iot`, `access_point`, and `firewall` exist in the enum with no code path that assigns them, they're either reserved for future heuristics or manual/API-only use, not something you'll see appear from a scan today.

## MAC Vendor Lookup

Two separate lookup paths exist, and they don't share data:

**The one actually used during scanning** is a small hardcoded Go map (~40 entries: Apple, Cisco, Samsung, Intel, Netgear, etc.) inside `scanner.NativeScanner.lookupVendor`. If the MAC's OUI isn't in that map and `VENDOR_LOOKUP_ONLINE_ENABLED=true`, it falls back to `api.macvendors.com` (2s timeout) as a per-miss lookup, this is the outbound call gated by that env var, see [architecture.md](architecture.md#network-isolation).

**`internal/oui.OUIService`** is a separate, much larger subsystem, it loads the full IEEE OUI registry from a local `oui.txt` file (optionally refreshed from `standards-oui.ieee.org` over plain HTTP when `OUI_DOWNLOAD_ENABLED=true`, at most every 30 days), and is constructed and initialized at startup. **It is effectively dead code at runtime**: the service is stored on `DeviceService` but nothing ever calls `LookupVendor` on it. If you're asked to improve vendor coverage, wiring this service's `LookupVendor` as a second-tier lookup (before or instead of the online API call) is the natural fix, don't assume it's already contributing just because it's initialized and populated.

Also note: `oui.txt` itself is not committed to the repo (it's a runtime download target), so on a fresh checkout the bundled-database story only applies after a successful download, which requires `OUI_DOWNLOAD_ENABLED=true` at least once.

## Hostname Resolution

Runs per-IP during the ping sweep, only for hosts already determined online. Five methods are tried in order, first non-empty result wins:

1. Reverse DNS (`net.DefaultResolver.LookupAddr`, 2s timeout)
2. NetBIOS name query (shells out to `nmblookup`/`nbtstat`)
3. mDNS (`<ip>.local`, 500ms timeout, resolved address must match the probed IP)
4. SNMP system name, **not actually implemented**, this step always returns empty (it only checks port 161 reachability, never speaks SNMP)
5. HTTP banner (HEAD to ports 80/8080/443/8443, hostname parsed out of a `Server` header or a redirect `Location`)

Hostname only comes from this ping-sweep phase; a port scan pass does not re-resolve or overwrite it. The IPv6 monitor does its own separate, simpler reverse-DNS lookup for IPv6 global addresses.

## OS Fingerprinting

There is no active OS fingerprinting in this codebase, no TTL analysis, no banner-based OS detection, nothing resembling `nmap -O`. `FingerprintService.AnalyzeDevice` only does device-type classification (below); its one OS-aware step just *reads* `device.OS.Name` if it happens to already be set, but nothing anywhere in the codebase ever writes `os_name`/`os_version`/`os_family`/`os_confidence`. Those are real schema columns with a real struct field, but no writer, they're vestigial. Don't build logic that assumes `device.OS` is populated; treat any current appearance of OS data in the UI or API response as always empty in practice.

## Device Type Classification

`FingerprintService.AnalyzeDevice` runs five signal sources in a fixed priority order, each one only fills in the type if it's still `unknown` (vendor is the exception; it always sets first and can be overwritten by later steps that check `unknown`... in practice vendor runs first so it wins unless it found nothing):

1. **Vendor string** (substring match, case-insensitive): cisco/juniper/netgear/linksys/d-link/tp-link → router; synology/qnap/drobo/netapp/seagate → nas; hp/canon/epson/brother/lexmark/xerox → printer; hikvision/dahua/axis/vivotek/foscam → camera; apple/samsung/lg/sony/huawei/xiaomi → mobile; dell/ibm/supermicro/intel → server.
2. **Open ports** (open-state only): 161+23 both open → router; (139 or 445) and (548 or 2049) → nas; (80 or 443) and (21 or 22) → server; 515/631/9100 → printer; 554/8080 or an `rtsp` service label → camera; 5060/5061 or a `sip` service label → voip; port 22 open with 3 or fewer total open ports → server (an "SSH-only device" fallback).
3. **Hostname substrings**: nas/synology/diskstation → nas; router/gateway/ap-/access-point → **router** (note: not `access_point`, despite that enum value existing); printer/print/hp-/canon- → printer; camera/cam/ipcam → camera; server/srv/web/db → server.
4. **Web service signatures** (title/server header of any discovered web service, first match wins): synology/diskstation/qnap/"nas" → nas; router/"access point"/wireless/`lighttpd` server header → router; printer/"print server"/cups → printer; "ip camera"/webcam/surveillance → camera.
5. **OS name** (only runs if `device.OS` is non-nil, which per above is effectively never true in current builds): windows/ubuntu/centos/rhel server strings → server; linux + embedded/openwrt/dd-wrt → router; ios/android → mobile; non-server windows or "mac os" or ubuntu → workstation.

If nothing matched, the device defaults to **`workstation`**.

## New vs Update: CreateOrUpdateWithDelta

`DeviceService.CreateOrUpdateWithDelta` decides whether an incoming scan result is a brand-new device or an update to one already on record:

1. Match by `IPv4` first.
2. If no match **and** the incoming record has a MAC, fall back to matching by MAC, this is the DHCP-reassignment case: a device that kept its MAC but got a new IP is recognized as the same device rather than created as a new one, and the existing row's ID/IPv4/hostname/vendor/network/status are updated in place onto that existing record.
3. If the user had set a `Name` or `Comment` on the existing record and the incoming scan data doesn't specify one, the existing value is preserved rather than cleared, a scan result never blanks out user-entered data. `IsFavorite`/`Ignored` are always carried forward the same way, and `Addressing` is preserved if a manual value is already set, or re-derived from the network's static ranges otherwise (see Curation Flags above).
4. `CreatedAt` is preserved from the existing record; `UpdatedAt` always advances.

**Why this matters for alerting**: the delta (`DeviceDelta{IsNew, NewPorts}`) is computed *before* the write, while the previous state is still known, because once the row is overwritten in place, "this device is new" and "this port just opened" are no longer recoverable from the database. See [03-alerting.md](03-alerting.md) for how `RecordNewDevice`/`RecordNewPorts` consume this. `NewPorts` is deliberately `nil` (not "everything closed") whenever either the previous or current port list is empty, a sweep that didn't run a port scan for this device carries no port information at all, and treating that as "all ports newly closed" would make every subsequent scan re-report the same ports as newly opened.

`EligibleForPortScan` (used both to decide whether to schedule a port scan and referenced above in the port-scanning doc) becomes true again 30 seconds after the last completed port scan, or immediately if the device has never been port-scanned.

## Fingerprinting Timing

`PerformDeviceFingerprinting` runs on **every port scan pass**, not just for newly-discovered devices. Since a port scan runs on essentially every scan cycle for any device outside its 30-second cooldown, device-type classification effectively re-evaluates continuously for every known-online device, not once at discovery.

## Device Name Cleanup

`POST /api/devices/cleanup-names` blanks the `name` column on **every** device row in the database (leaves `comment`, `hostname`, and `vendor` untouched). This is a bulk reset, not a per-device targeted cleanup, useful after a period where auto-generated or incorrect names got written, but destructive to any manually-set names too. A second, near-identical `CleanupDeviceNames` handler exists in `internal/device/device_handlers.go` but is never mounted on a route, it's dead code left over from an earlier handler layout; don't be misled into thinking there are two different cleanup behaviors reachable from the API.

Related, separate cleanup functions: `CleanupDuplicateDevices` (groups by MAC, keeps the most recently updated row, migrates any name/comment onto the keeper before deleting the rest) and `CleanupNetworkBroadcastDevices` (see [00-network-scanning.md](00-network-scanning.md#broadcastnetwork-address-device-cleanup)).
