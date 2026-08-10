package alert

import (
	"testing"
	"time"

	"reconya/models"

	"github.com/stretchr/testify/assert"
)

func ptr(s string) *string { return &s }

func at(t time.Time) *time.Time { return &t }

// baseNow is a fixed clock so duration-sensitive assertions are deterministic.
var baseNow = time.Date(2026, 8, 10, 14, 0, 0, 0, time.UTC)

func input(devices ...*models.Device) EvalInput {
	return EvalInput{
		Devices:          devices,
		NetworkCIDR:      map[string]string{"net-1": "10.20.2.0/24"},
		Now:              baseNow,
		OfflineThreshold: 6 * time.Hour,
		OfflineWindow:    7 * 24 * time.Hour,
	}
}

func ruleIDs(findings []Finding) []string {
	ids := make([]string, 0, len(findings))
	for _, f := range findings {
		ids = append(ids, f.RuleID)
	}
	return ids
}

func findByRule(findings []Finding, rule string) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.RuleID == rule {
			out = append(out, f)
		}
	}
	return out
}

func TestUnidentifiedHost(t *testing.T) {
	// A locally-administered MAC with nothing else known about it.
	anonymous := &models.Device{
		ID: "d1", IPv4: "10.20.2.83", NetworkID: "net-1",
		Status: models.DeviceStatusOnline,
		MAC:    ptr("9A:3F:C1:07:B4:5E"),
	}

	t.Run("flags a fully anonymous host", func(t *testing.T) {
		findings := findByRule(EvaluateStateful(input(anonymous)), RuleUnidentifiedHost)
		assert.Len(t, findings, 1)
		assert.Equal(t, models.AlertCritical, findings[0].Severity)
		assert.Equal(t, "unidentified_host:d1", findings[0].DedupeKey)
		assert.Contains(t, findings[0].Title, "10.20.2.0/24")
	})

	t.Run("a burned-in MAC is not anonymous", func(t *testing.T) {
		d := *anonymous
		d.MAC = ptr("78:8A:20:41:0C:B2") // universally administered
		assert.Empty(t, findByRule(EvaluateStateful(input(&d)), RuleUnidentifiedHost))
	})

	t.Run("a known vendor clears it", func(t *testing.T) {
		d := *anonymous
		d.Vendor = ptr("Apple, Inc.")
		assert.Empty(t, findByRule(EvaluateStateful(input(&d)), RuleUnidentifiedHost))
	})

	t.Run("a hostname clears it", func(t *testing.T) {
		d := *anonymous
		d.Hostname = ptr("ipad-front")
		assert.Empty(t, findByRule(EvaluateStateful(input(&d)), RuleUnidentifiedHost))
	})

	t.Run("an open port clears it", func(t *testing.T) {
		d := *anonymous
		d.Ports = []models.Port{{Number: "22", Protocol: "tcp", State: "open", Service: "ssh"}}
		assert.Empty(t, findByRule(EvaluateStateful(input(&d)), RuleUnidentifiedHost))
	})

	t.Run("a closed port does not clear it", func(t *testing.T) {
		d := *anonymous
		d.Ports = []models.Port{{Number: "22", Protocol: "tcp", State: "closed"}}
		assert.Len(t, findByRule(EvaluateStateful(input(&d)), RuleUnidentifiedHost), 1)
	})

	t.Run("offline hosts are not flagged", func(t *testing.T) {
		d := *anonymous
		d.Status = models.DeviceStatusOffline
		assert.Empty(t, findByRule(EvaluateStateful(input(&d)), RuleUnidentifiedHost))
	})

	t.Run("a device with no MAC at all is not flagged", func(t *testing.T) {
		d := *anonymous
		d.MAC = nil
		assert.Empty(t, findByRule(EvaluateStateful(input(&d)), RuleUnidentifiedHost))
	})
}

func TestDuplicateMAC(t *testing.T) {
	t.Run("flags two IPs sharing one MAC", func(t *testing.T) {
		findings := findByRule(EvaluateStateful(input(
			&models.Device{ID: "d1", IPv4: "10.20.2.140", NetworkID: "net-1", MAC: ptr("9A:3F:C1:07:B4:5E"), Vendor: ptr("x"), Status: models.DeviceStatusOnline},
			&models.Device{ID: "d2", IPv4: "10.20.2.19", NetworkID: "net-1", MAC: ptr("9a-3f-c1-07-b4-5e"), Vendor: ptr("x"), Status: models.DeviceStatusOnline},
		)), RuleDuplicateMAC)

		assert.Len(t, findings, 1)
		assert.Equal(t, models.AlertCritical, findings[0].Severity)
		// Separator style must not create two distinct keys.
		assert.Equal(t, "duplicate_mac:9a3fc107b45e", findings[0].DedupeKey)
		// Attached to the numerically lowest IP — .19 before .140, not the
		// lexical order a plain string sort would give.
		assert.Equal(t, "d2", *findings[0].DeviceID)
		assert.Equal(t, "10.20.2.19 and 10.20.2.140 share 9A3FC107B45E", findings[0].Detail)
	})

	t.Run("distinct MACs are fine", func(t *testing.T) {
		findings := EvaluateStateful(input(
			&models.Device{ID: "d1", IPv4: "10.20.2.1", MAC: ptr("00:11:22:33:44:55"), Vendor: ptr("x"), Status: models.DeviceStatusOnline},
			&models.Device{ID: "d2", IPv4: "10.20.2.2", MAC: ptr("00:11:22:33:44:66"), Vendor: ptr("x"), Status: models.DeviceStatusOnline},
		))
		assert.Empty(t, findByRule(findings, RuleDuplicateMAC))
	})

	t.Run("malformed MACs are ignored rather than grouped together", func(t *testing.T) {
		findings := EvaluateStateful(input(
			&models.Device{ID: "d1", IPv4: "10.20.2.1", MAC: ptr(""), Vendor: ptr("x"), Status: models.DeviceStatusOnline},
			&models.Device{ID: "d2", IPv4: "10.20.2.2", MAC: ptr("nonsense"), Vendor: ptr("x"), Status: models.DeviceStatusOnline},
		))
		assert.Empty(t, findByRule(findings, RuleDuplicateMAC))
	})

	t.Run("stale offline rows are lease churn, not a conflict", func(t *testing.T) {
		// A host that got a new DHCP lease leaves its old address behind. Both
		// rows carry the same MAC, but only one of them is a live host.
		findings := findByRule(EvaluateStateful(input(
			&models.Device{ID: "d1", IPv4: "10.20.2.140", NetworkID: "net-1", MAC: ptr("9A:3F:C1:07:B4:5E"),
				Vendor: ptr("x"), Status: models.DeviceStatusOffline},
			&models.Device{ID: "d2", IPv4: "10.20.2.19", NetworkID: "net-1", MAC: ptr("9A:3F:C1:07:B4:5E"),
				Vendor: ptr("x"), Status: models.DeviceStatusOnline},
		)), RuleDuplicateMAC)

		assert.Empty(t, findings)
	})
}

func TestRiskyPorts(t *testing.T) {
	printer := &models.Device{
		ID: "d1", IPv4: "10.20.3.19", NetworkID: "net-1", Name: "printer-ops",
		Status: models.DeviceStatusOnline,
		Vendor: ptr("Brother Industries"),
		Ports: []models.Port{
			{Number: "80", Protocol: "tcp", State: "open", Service: "http"},
			{Number: "9100", Protocol: "tcp", State: "open", Service: "jetdirect"},
			{Number: "23", Protocol: "tcp", State: "closed", Service: "telnet"},
		},
	}

	findings := findByRule(EvaluateStateful(input(printer)), RuleRiskyPort)

	assert.Len(t, findings, 1, "only the open risky port should fire")
	assert.Equal(t, "risky_port:d1:9100", findings[0].DedupeKey)
	assert.Equal(t, models.AlertWarning, findings[0].Severity)
	assert.Contains(t, findings[0].Title, "printer-ops")
}

func TestHostUnreachable(t *testing.T) {
	t.Run("fires past the threshold", func(t *testing.T) {
		findings := findByRule(EvaluateStateful(input(&models.Device{
			ID: "d1", IPv4: "10.20.3.41", NetworkID: "net-1", Name: "hue-bridge",
			Status:           models.DeviceStatusOffline,
			Vendor:           ptr("Signify N.V."),
			LastSeenOnlineAt: at(baseNow.Add(-8 * time.Hour)),
		})), RuleHostUnreachable)

		assert.Len(t, findings, 1)
		assert.Equal(t, models.AlertWarning, findings[0].Severity)
		assert.Contains(t, findings[0].Title, "8h")
		assert.Contains(t, findings[0].Detail, "hue-bridge")
	})

	t.Run("stays quiet inside the threshold", func(t *testing.T) {
		findings := findByRule(EvaluateStateful(input(&models.Device{
			ID: "d1", IPv4: "10.20.3.41", Status: models.DeviceStatusOffline,
			Vendor:           ptr("x"),
			LastSeenOnlineAt: at(baseNow.Add(-2 * time.Hour)),
		})), RuleHostUnreachable)

		assert.Empty(t, findings)
	})

	t.Run("goes quiet again once the host is simply gone", func(t *testing.T) {
		// Without an upper bound this rule matches every device ever
		// discovered, forever — a fortnight-old absence is not an incident.
		findings := findByRule(EvaluateStateful(input(&models.Device{
			ID: "d1", IPv4: "10.55.0.180", Status: models.DeviceStatusOffline,
			Vendor:           ptr("x"),
			LastSeenOnlineAt: at(baseNow.Add(-14 * 24 * time.Hour)),
		})), RuleHostUnreachable)

		assert.Empty(t, findings)
	})

	t.Run("a host that was never online is not a regression", func(t *testing.T) {
		findings := findByRule(EvaluateStateful(input(&models.Device{
			ID: "d1", IPv4: "10.20.3.41", Status: models.DeviceStatusOffline, Vendor: ptr("x"),
		})), RuleHostUnreachable)

		assert.Empty(t, findings)
	})

	t.Run("online hosts are never unreachable", func(t *testing.T) {
		findings := findByRule(EvaluateStateful(input(&models.Device{
			ID: "d1", IPv4: "10.20.3.41", Status: models.DeviceStatusOnline, Vendor: ptr("x"),
			LastSeenOnlineAt: at(baseNow.Add(-8 * time.Hour)),
		})), RuleHostUnreachable)

		assert.Empty(t, findings)
	})
}

func TestEvaluateStatefulIsStableAndSkipsNils(t *testing.T) {
	in := input(
		nil,
		&models.Device{ID: "d1", IPv4: "10.20.2.83", NetworkID: "net-1", Status: models.DeviceStatusOnline, MAC: ptr("9A:3F:C1:07:B4:5E")},
		&models.Device{ID: "d2", IPv4: "10.20.2.1", NetworkID: "net-1", Status: models.DeviceStatusOnline, Vendor: ptr("x"),
			Ports: []models.Port{{Number: "23", Protocol: "tcp", State: "open", Service: "telnet"}}},
	)

	first := EvaluateStateful(in)
	second := EvaluateStateful(in)

	assert.Equal(t, ruleIDs(first), ruleIDs(second), "ordering must be deterministic")
	assert.Equal(t, []string{RuleRiskyPort, RuleUnidentifiedHost}, ruleIDs(first))
}

func TestNewPortFinding(t *testing.T) {
	d := &models.Device{ID: "d1", IPv4: "10.20.1.14", NetworkID: "net-1", Name: "nas-vault"}
	f := NewPortFinding(d, models.Port{Number: "5001", Protocol: "tcp", State: "open", Service: "dsm-tls"})

	assert.Equal(t, RuleNewPort, f.RuleID)
	assert.Equal(t, "new_port:d1:5001", f.DedupeKey)
	assert.Equal(t, models.AlertWarning, f.Severity)
	assert.Contains(t, f.Title, "nas-vault")
	assert.Contains(t, f.Detail, "5001/tcp")
}

func TestNewDeviceFinding(t *testing.T) {
	d := &models.Device{ID: "d1", IPv4: "10.20.2.77", NetworkID: "net-1", Vendor: ptr("Apple, Inc.")}
	f := NewDeviceFinding(d, "10.20.2.0/24")

	assert.Equal(t, RuleNewDevice, f.RuleID)
	assert.Equal(t, "new_device:d1", f.DedupeKey)
	assert.Equal(t, models.AlertNotice, f.Severity)
	assert.Contains(t, f.Detail, "Apple, Inc.")
	assert.Contains(t, f.Detail, "10.20.2.0/24")
}

func TestIsLocallyAdministeredMAC(t *testing.T) {
	cases := map[string]bool{
		"9A:3F:C1:07:B4:5E": true,  // 0x9A -> bit 1 set
		"02:00:00:00:00:01": true,  // canonical locally administered
		"06:11:22:33:44:55": true,  // 0x06
		"0E:11:22:33:44:55": true,  // 0x0E
		"78:8A:20:41:0C:B2": false, // Ubiquiti OUI
		"00:11:32:8A:4C:E1": false, // Synology OUI
		"not-a-mac":         false,
		"":                  false,
	}

	for mac, want := range cases {
		assert.Equalf(t, want, isLocallyAdministeredMAC(mac), "mac %q", mac)
	}
}

func TestHumaniseDuration(t *testing.T) {
	assert.Equal(t, "45m", humaniseDuration(45*time.Minute))
	assert.Equal(t, "6h", humaniseDuration(6*time.Hour))
	assert.Equal(t, "2d", humaniseDuration(50*time.Hour))
}
