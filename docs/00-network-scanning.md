# Network Scanning

Device discovery: how a "scan" actually runs, how the sweep decides a host is alive, and how IPv6 passive monitoring works. Port scanning (a separate, per-device pass) is covered in [02-port-scanning.md](02-port-scanning.md).

## Network Model and Selection

`models.Network` (`backend/models/network.go`) is mostly just `CIDR` plus display metadata (`Name`, `Description`, `Status`, `LastScannedAt`, `DeviceCount`). It also has `IPv6Prefix`/`AddressFamily` fields and dual-stack helper methods, but nothing else in the codebase actually uses them for scan targeting: IPv6 discovery works off live neighbor-discovery data, not this field, and the "network to scan" is always the network's `CIDR`.

`NetworkService.Create` is an unconditional upsert; `FindOrCreate` looks up by CIDR first and only creates on a miss. `Delete` (the HTTP handler, not the service) refuses to delete a network with a scan currently running against it or with any devices still attached, and translates the resulting SQLite foreign-key error into a friendlier message.

**"Selected network" is scan-manager state, not a network property.** `scan.ScanManager` tracks `SelectedNetwork` (what the UI picked) separately from `CurrentNetwork` (what's actively being scanned right now); `GetSelectedOrCurrentNetwork()` returns whichever applies. At startup, the primary-NIC detection goroutine (below) preselects the network matching the machine's own LAN, so a scan started with no explicit selection targets the network the machine is actually on rather than whichever network row happens to sort first.

## Starting and Stopping a Scan

Scanning is **continuous, not one-shot per request**. `POST /api/scan/start` calls `ScanManager.StartScan(networkID)`, which sets state to running and launches a loop: it runs one sweep immediately, then repeats every 30 seconds on a ticker, until `POST /api/scan/stop` closes the stop channel. `GET /api/scan/status` just returns the manager's in-memory state; `GET /api/scan/control` returns that state plus the network list, for the console's scan-control widget. `POST /api/scan/select-network` only updates `SelectedNetwork` and does not start anything.

Each sweep iteration (`runSingleScan`):

1. Checks the stop signal (cooperative cancellation between steps, not context cancellation).
2. Logs a `PingSweep` start event.
3. Runs the actual sweep (`PingSweepService.ExecuteSweepScanCommand`, see below) against the network's CIDR.
4. For every discovered device, calls `DeviceService.CreateOrUpdateWithDelta`. If the delta says the device is new or has newly-opened ports, fires the corresponding alert immediately (`RecordNewDevice`/`RecordNewPorts`), because this is the last point at which "first sighting" is knowable, the row is about to be overwritten in place. See [03-alerting.md](03-alerting.md).
5. Logs a `DeviceOnline` event per discovered device, and enqueues devices eligible for a port scan (`DeviceService.EligibleForPortScan`) as background goroutines.
6. Logs a second `PingSweep` event carrying the actual elapsed duration for this iteration (measured fresh each pass, not against the session's overall start time, which would otherwise grow by 30 seconds on every iteration).
7. Runs alert evaluation against **all** devices, not just this network's, since auto-resolve is scoped per rule; evaluating a subset would incorrectly resolve valid findings that belong to networks not part of this sweep.

The reported "scan count" (`/api/scan/status`) is not the in-memory loop counter, it's a live count of completed `PingSweep` events in the database, specifically so the number doesn't jump around whenever a scan starts or stops.

## The Ping Sweep

Device discovery is a pure-Go implementation (`backend/internal/scanner`, `golang.org/x/net/icmp`), no `ping`, `nmap`, or `arp-scan` binaries are shelled out to for the sweep itself. `ScanNetwork(cidr)` enumerates every host address in the CIDR (excluding the network and broadcast addresses), invalidates the cached ARP table so a sweep never opens against entries left from a previous network, and fans the address list out to 50 concurrent worker goroutines.

**A host counts as online only if corroborated**, per-IP:

```
online := icmpEchoReplyMatched || macResolvedViaARP || (tcpConnectSucceeded && !isOnLink(ip))
```

ICMP (tried unprivileged first via a UDP-datagram ICMP socket, falling back to a raw socket if that's unavailable) or a resolved ARP entry are trusted directly. A bare TCP connect success is trusted only when the IP is *off-link* (routed, not directly on the local subnet), because an on-link host that's actually alive must answer ARP, if only TCP succeeds for an on-link address, that's attributed to a middlebox (VPN, proxy, captive portal) intercepting the connection rather than a genuine live host, and is deliberately not counted, or an entire /24 behind such a device would look fully occupied.

TCP probing tries 12 common ports (80, 443, 22, 21, 23, 25, 53, 135, 139, 445, 3389, 8080) in parallel with a short per-dial timeout, returning on the first success. MAC resolution reads the OS ARP table, and if that misses, sends a throwaway UDP packet to provoke an ARP resolution before re-checking. Hostname resolution tries, in order: reverse DNS, NetBIOS (`nmblookup`/`nbtstat`), mDNS (`<name>.local`), and HTTP banner headers (`Server`/`Location`), SNMP is present as an unimplemented stub.

Vendor lookup uses a small built-in OUI map by default; if `VENDOR_LOOKUP_ONLINE_ENABLED=true`, unresolved OUIs fall back to a call to `api.macvendors.com` per discovered device, this is an outbound call and stays off by default, see [architecture.md](architecture.md#network-isolation).

## Primary NIC Detection and Network Suggestions

Runs on a 30-second ticker (`nicidentifier.NicIdentifierService.CheckForNewNetworks`), independently of whether a scan is active.

**Primary NIC selection** skips down/loopback interfaces, classifies each remaining interface as Docker/VM-related (by interface name prefix, or by IP falling in a known container-network range like `172.16.0.0/12`) or a genuine host candidate, and prefers a candidate whose IP is in a common private range (`192.168.0.0/16`, `10.0.0.0/8`); a Docker interface is only used as a last resort if nothing else exists.

**New-network detection**: for every up, non-loopback, non-point-to-point interface (VPN/tunnel interfaces like `utun*` are skipped, they carry a single host address, not a scannable LAN), for each IPv4 address not already a Docker/VM network and not a /31-or-smaller host-only network, the service derives the base CIDR and checks whether a matching `Network` row already exists.

**Detection never creates a network row by itself**, it only writes a `NewNetworkDetected` event log ("Consider creating it for scanning") and makes the CIDR available via `GET /api/detected-networks`. Turning a suggestion into an actual scannable network requires an explicit `POST /api/network-suggestion` call (the console's network-suggestions UI), which calls `NetworkService.Create` with an auto-generated name and description.

## IPv6 Passive Monitoring

Generates no traffic by design: it reads the OS's existing neighbor-discovery cache (`ip -6 neigh show` on Linux, `ndp -an` on macOS) rather than sending any probes.

**Lifecycle is coupled to the IPv4 scan session**, not independent: `IPv6MonitorService.Start()`/`Stop()` are called from `ScanManager.StartScan`/`StopScan`, so IPv6 monitoring only runs while an IPv4 scan session is active. On start, it auto-detects monitorable interfaces and guesses `/64` host prefixes from each interface's existing global addresses, then runs a small set of goroutines: one polling the NDP table every 30 seconds, one re-enumerating the local host's own interface addresses every 60 seconds, and a multicast-traffic monitor.

A discovered IPv6 address is matched to an existing device by MAC first, then by IPv6 address; if a match is found, the address is attached and the device is marked online. **If no matching IPv4 device exists, the IPv6-only sighting is discarded, not persisted**, the code explicitly skips creating IPv6-only device rows to avoid malformed entries, pending proper dual-stack support.

**Known documentation/code mismatch**: the README and `.env.example` describe `IPV6_MONITORING_ENABLED`, `IPV6_MONITOR_INTERFACES`, `IPV6_MONITOR_INTERVAL`, `IPV6_LINK_LOCAL_MONITORING`, and `IPV6_MULTICAST_MONITORING` as configurable. None of them are actually read anywhere in the Go source, `config.Config` has no IPv6 fields at all. In the running code, link-local and multicast monitoring are hardcoded on, interfaces/prefixes are always auto-detected, and the NDP poll interval is hardcoded to 30 seconds. Also, the multicast-traffic monitor is a placeholder that just blocks until the service stops, it logs that it "would require raw socket implementation" and does nothing. Don't treat those env vars as functional until this is reconciled; if you're asked to make IPv6 monitoring configurable, that wiring needs to be added from scratch, not fixed.

## Broadcast/Network-Address Device Cleanup

The sweep's IP list already excludes network and broadcast addresses, and `CreateOrUpdateWithDelta` refuses to persist a device at one going forward. `POST /api/devices/cleanup-network-broadcast` (`DeviceService.CleanupNetworkBroadcastDevices`) is a retroactive, manually-triggered fix-up for rows that predate that guard (e.g. from older code paths or the IPv6 monitor), nothing schedules it automatically, unlike the geolocation cache cleanup which runs on its own 6-hour ticker.
