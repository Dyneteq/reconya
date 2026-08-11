package alert

import (
	"bytes"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"reconya/models"
)

// Rule identifiers. These are stable strings: they are the auto-resolve scope
// (see AlertRepository.ResolveMissing) and the console filters on them.
const (
	RuleNewDevice        = "new_device"
	RuleUnidentifiedHost = "unidentified_host"
	RuleDuplicateMAC     = "duplicate_mac"
	RuleNewPort          = "new_port"
	RuleRiskyPort        = "risky_port"
	RuleHostUnreachable  = "host_unreachable"
)

// StatefulRules are recomputed in full from the current device set on every
// evaluation, so an alert of one of these rules that is not re-emitted means
// its condition stopped holding and it can be auto-resolved.
//
// RuleNewDevice and RuleNewPort are deliberately absent: they describe an event,
// not a state, so there is nothing to recompute. They expire on age instead
// (see AlertService.expireOneShots).
var StatefulRules = []string{
	RuleUnidentifiedHost,
	RuleDuplicateMAC,
	RuleRiskyPort,
	RuleHostUnreachable,
}

// riskyPorts are services that should not normally be reachable across a flat
// LAN. The value is what the alert detail calls them.
var riskyPorts = map[string]string{
	"21":   "FTP — credentials travel in clear text",
	"23":   "Telnet — credentials travel in clear text",
	"445":  "SMB — file sharing exposed to the whole segment",
	"3389": "RDP — remote desktop exposed to the whole segment",
	"5900": "VNC — remote desktop, frequently unauthenticated",
	"9100": "JetDirect — raw printing, unauthenticated by design",
}

// Finding is a candidate alert produced by a rule, before it is reconciled with
// what is already stored.
type Finding struct {
	RuleID    string
	DedupeKey string
	Severity  models.AlertSeverity
	Title     string
	Detail    string
	DeviceID  *string
	NetworkID *string
}

// EvalInput is everything the stateful rules look at. Keeping it a plain value
// is what makes the rules testable without a database.
type EvalInput struct {
	Devices []*models.Device
	// NetworkCIDR maps network id -> CIDR, used to make alert details readable.
	NetworkCIDR map[string]string
	// Now is the evaluation clock, injected so tests are deterministic.
	Now time.Time
	// OfflineThreshold is how long a device must have been unreachable before
	// it is worth alerting on.
	OfflineThreshold time.Duration
	// OfflineWindow is how long after that the alert stops being news. A host
	// that has been gone for a fortnight is not an incident, it is just not on
	// the network any more; without this bound every historical device in the
	// table alerts forever.
	OfflineWindow time.Duration
}

// EvaluateStateful runs every stateful rule over the full device set and returns
// findings sorted by rule then dedupe key, so callers and tests see a stable
// order.
func EvaluateStateful(in EvalInput) []Finding {
	var findings []Finding

	findings = append(findings, evalUnidentifiedHosts(in)...)
	findings = append(findings, evalDuplicateMACs(in)...)
	findings = append(findings, evalRiskyPorts(in)...)
	findings = append(findings, evalHostUnreachable(in)...)

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].RuleID != findings[j].RuleID {
			return findings[i].RuleID < findings[j].RuleID
		}
		return findings[i].DedupeKey < findings[j].DedupeKey
	})

	return findings
}

// evalUnidentifiedHosts flags hosts that answer pings but reveal nothing about
// themselves: no vendor, no hostname, no open ports, and a locally-administered
// MAC (i.e. randomised rather than burned in).
//
// All four have to hold. Any one of them alone is ordinary — plenty of legitimate
// devices randomise their MAC, and a host that simply hasn't been port-scanned
// yet has no ports either.
func evalUnidentifiedHosts(in EvalInput) []Finding {
	var findings []Finding

	for _, d := range in.Devices {
		if d == nil || d.Status == models.DeviceStatusOffline || d.Ignored {
			continue
		}
		if hasText(d.Vendor) || hasText(d.Hostname) || d.Name != "" {
			continue
		}
		if len(openPorts(d.Ports)) > 0 {
			continue
		}
		if d.MAC == nil || !isLocallyAdministeredMAC(*d.MAC) {
			continue
		}

		id := d.ID
		netID := d.NetworkID
		findings = append(findings, Finding{
			RuleID:    RuleUnidentifiedHost,
			DedupeKey: RuleUnidentifiedHost + ":" + d.ID,
			Severity:  models.AlertCritical,
			Title:     "Unidentified host on " + networkLabel(in, d.NetworkID),
			Detail: fmt.Sprintf("%s · randomised MAC %s, no vendor, no hostname, no open ports",
				d.IPv4, *d.MAC),
			DeviceID:  &id,
			NetworkID: nullIfEmpty(&netID),
		})
	}

	return findings
}

// evalDuplicateMACs flags a MAC address claimed by more than one live IP. That
// is either a spoof or a misconfigured bridge; both are worth a look.
//
// Offline rows are excluded, and that exclusion is what makes the rule mean
// anything: a host that gets a new DHCP lease leaves its old address behind as
// a stale record, so matching those would report ordinary lease churn as an
// address conflict. Two hosts answering right now on one MAC is the anomaly.
func evalDuplicateMACs(in EvalInput) []Finding {
	byMAC := map[string][]*models.Device{}

	for _, d := range in.Devices {
		if d == nil || d.MAC == nil || d.Status == models.DeviceStatusOffline || d.Ignored {
			continue
		}
		mac := normaliseMAC(*d.MAC)
		if mac == "" {
			continue
		}
		byMAC[mac] = append(byMAC[mac], d)
	}

	var findings []Finding
	for mac, devices := range byMAC {
		if len(devices) < 2 {
			continue
		}

		// Numeric order, not lexical — .19 belongs before .140.
		sort.Slice(devices, func(i, j int) bool { return ipLess(devices[i].IPv4, devices[j].IPv4) })

		ips := make([]string, 0, len(devices))
		for _, d := range devices {
			ips = append(ips, d.IPv4)
		}

		// Attach to the lowest device so the console has somewhere to jump to;
		// the detail names every address involved.
		id := devices[0].ID
		netID := devices[0].NetworkID

		findings = append(findings, Finding{
			RuleID:    RuleDuplicateMAC,
			DedupeKey: RuleDuplicateMAC + ":" + mac,
			Severity:  models.AlertCritical,
			Title:     "Duplicate MAC address observed",
			Detail:    strings.Join(ips, " and ") + " share " + strings.ToUpper(mac),
			DeviceID:  &id,
			NetworkID: nullIfEmpty(&netID),
		})
	}

	return findings
}

// evalRiskyPorts flags plaintext or remote-control services that are open to the
// segment.
func evalRiskyPorts(in EvalInput) []Finding {
	var findings []Finding

	for _, d := range in.Devices {
		if d == nil || d.Status == models.DeviceStatusOffline || d.Ignored {
			continue
		}

		for _, p := range openPorts(d.Ports) {
			why, risky := riskyPorts[p.Number]
			if !risky {
				continue
			}

			id := d.ID
			netID := d.NetworkID
			findings = append(findings, Finding{
				RuleID:    RuleRiskyPort,
				DedupeKey: fmt.Sprintf("%s:%s:%s", RuleRiskyPort, d.ID, p.Number),
				Severity:  models.AlertWarning,
				Title:     fmt.Sprintf("%s open on %s", serviceLabel(p), deviceLabel(d)),
				Detail:    fmt.Sprintf("%s · %s/%s · %s", d.IPv4, p.Number, protocolOrTCP(p), why),
				DeviceID:  &id,
				NetworkID: nullIfEmpty(&netID),
			})
		}
	}

	return findings
}

// evalHostUnreachable flags devices that have recently stopped answering.
//
// The signal is the transition — something that was working has gone quiet —
// so it is bounded on both sides. Below OfflineThreshold a host is merely
// asleep; past OfflineWindow it has simply left, and continuing to alert would
// mean every device ever discovered stays permanently in the feed. A device
// that was never seen online is skipped for the same reason: there is no
// "was working, now isn't" to report.
func evalHostUnreachable(in EvalInput) []Finding {
	var findings []Finding

	for _, d := range in.Devices {
		if d == nil || d.Status != models.DeviceStatusOffline || d.LastSeenOnlineAt == nil || d.Ignored {
			continue
		}

		away := in.Now.Sub(*d.LastSeenOnlineAt)
		if away < in.OfflineThreshold {
			continue
		}
		if in.OfflineWindow > 0 && away > in.OfflineWindow {
			continue
		}

		id := d.ID
		netID := d.NetworkID
		findings = append(findings, Finding{
			RuleID:    RuleHostUnreachable,
			DedupeKey: RuleHostUnreachable + ":" + d.ID,
			Severity:  models.AlertWarning,
			Title:     "Host unreachable for " + humaniseDuration(away),
			Detail: fmt.Sprintf("%s · last reply %s",
				deviceLabel(d), d.LastSeenOnlineAt.Format("2006-01-02 15:04")),
			DeviceID:  &id,
			NetworkID: nullIfEmpty(&netID),
		})
	}

	return findings
}

// NewDeviceFinding reports a host seen for the first time. This is event-shaped:
// it is emitted from the upsert path, because once the row is written there is
// no longer any way to tell it apart from a host that has been there for months.
func NewDeviceFinding(d *models.Device, networkCIDR string) Finding {
	id := d.ID
	netID := d.NetworkID

	detail := d.IPv4
	if hasText(d.Vendor) {
		detail += " · " + *d.Vendor
	}
	if networkCIDR != "" {
		detail += " · " + networkCIDR
	}

	return Finding{
		RuleID:    RuleNewDevice,
		DedupeKey: RuleNewDevice + ":" + d.ID,
		Severity:  models.AlertNotice,
		Title:     "New device joined the network",
		Detail:    detail,
		DeviceID:  &id,
		NetworkID: nullIfEmpty(&netID),
	}
}

// NewPortFinding reports a port that was not open the last time this device was
// scanned. Also event-shaped, for the same reason as NewDeviceFinding.
func NewPortFinding(d *models.Device, p models.Port) Finding {
	id := d.ID
	netID := d.NetworkID

	return Finding{
		RuleID:    RuleNewPort,
		DedupeKey: fmt.Sprintf("%s:%s:%s", RuleNewPort, d.ID, p.Number),
		Severity:  models.AlertWarning,
		Title:     "New service exposed on " + deviceLabel(d),
		Detail: fmt.Sprintf("%s opened %s/%s%s",
			d.IPv4, p.Number, protocolOrTCP(p), serviceSuffix(p)),
		DeviceID:  &id,
		NetworkID: nullIfEmpty(&netID),
	}
}

// --- helpers ---------------------------------------------------------------

func openPorts(ports []models.Port) []models.Port {
	var open []models.Port
	for _, p := range ports {
		if strings.EqualFold(p.State, "open") {
			open = append(open, p)
		}
	}
	return open
}

// isLocallyAdministeredMAC reports whether the U/L bit of the first octet is
// set, which marks an address that was assigned by software rather than burned
// in by the manufacturer — i.e. a randomised MAC.
func isLocallyAdministeredMAC(mac string) bool {
	clean := normaliseMAC(mac)
	if len(clean) < 2 {
		return false
	}

	first, err := hexNibblePair(clean[:2])
	if err != nil {
		return false
	}

	return first&0x02 != 0
}

func hexNibblePair(s string) (byte, error) {
	var v byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		var n byte
		switch {
		case c >= '0' && c <= '9':
			n = c - '0'
		case c >= 'a' && c <= 'f':
			n = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			n = c - 'A' + 10
		default:
			return 0, fmt.Errorf("not hex: %q", s)
		}
		v = v<<4 | n
	}
	return v, nil
}

// normaliseMAC strips separators and lowercases, so 00:1A:2B, 00-1a-2b and
// 001a2b all dedupe to the same key.
func normaliseMAC(mac string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(mac) {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			b.WriteRune(r)
		}
	}
	if b.Len() != 12 {
		return ""
	}
	return b.String()
}

// ipLess orders addresses numerically, so 10.20.2.19 sorts before 10.20.2.140.
// Anything unparseable falls back to a string compare and sorts after real
// addresses.
func ipLess(a, b string) bool {
	ipA, ipB := net.ParseIP(a), net.ParseIP(b)
	if ipA == nil || ipB == nil {
		return a < b
	}
	return bytes.Compare(ipA.To16(), ipB.To16()) < 0
}

func hasText(s *string) bool {
	return s != nil && strings.TrimSpace(*s) != ""
}

func nullIfEmpty(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

// deviceLabel is the friendliest name available for a device.
func deviceLabel(d *models.Device) string {
	if d.Name != "" {
		return d.Name
	}
	if hasText(d.Hostname) {
		return *d.Hostname
	}
	return d.IPv4
}

func networkLabel(in EvalInput, networkID string) string {
	if cidr, ok := in.NetworkCIDR[networkID]; ok && cidr != "" {
		return cidr
	}
	return "the network"
}

func serviceLabel(p models.Port) string {
	if p.Service != "" {
		return strings.ToUpper(p.Service)
	}
	return "Port " + p.Number
}

func serviceSuffix(p models.Port) string {
	if p.Service == "" {
		return ""
	}
	return " · " + p.Service
}

func protocolOrTCP(p models.Port) string {
	if p.Protocol == "" {
		return "tcp"
	}
	return p.Protocol
}

// humaniseDuration renders a coarse "6h" / "2d" style span for alert titles.
func humaniseDuration(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours())/24)
	}
}
