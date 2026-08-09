package scanner

import (
	"net"
	"testing"
)

func mustCIDR(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("ParseCIDR(%q): %v", cidr, err)
	}
	return ipNet
}

func TestIsOnLinkWithin(t *testing.T) {
	subnets := []*net.IPNet{
		mustCIDR(t, "192.168.1.0/24"),
		mustCIDR(t, "10.0.4.0/23"), // spans 10.0.4.0 - 10.0.5.255
	}

	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"inside first subnet", "192.168.1.42", true},
		{"first subnet network address", "192.168.1.0", true},
		{"first subnet broadcast", "192.168.1.255", true},
		{"adjacent subnet is not on link", "192.168.2.1", false},
		{"inside wider second subnet", "10.0.5.200", true},
		{"just past second subnet", "10.0.6.1", false},
		// A VPN range the host is not attached to: this is the case that made
		// every address in 10.55.0.0/24 look alive, because TCP alone decided.
		{"unattached vpn range", "10.55.0.17", false},
		{"malformed address", "not-an-ip", false},
		{"ipv6 address", "fe80::1", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOnLinkWithin(tt.ip, subnets); got != tt.want {
				t.Errorf("isOnLinkWithin(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestIsOnLinkWithinNoSubnets(t *testing.T) {
	// With no attached subnets nothing is on link, so TCP-only detection stays
	// authoritative rather than every host being rejected.
	if isOnLinkWithin("192.168.1.1", nil) {
		t.Error("isOnLinkWithin with no subnets = true, want false")
	}
}

func TestNormalizeMAC(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"lower case is upper cased", "aa:bb:cc:dd:ee:ff", "AA:BB:CC:DD:EE:FF"},
		{"already upper case", "AA:BB:CC:DD:EE:FF", "AA:BB:CC:DD:EE:FF"},
		{"surrounding whitespace", "  aa:bb:cc:dd:ee:ff ", "AA:BB:CC:DD:EE:FF"},
		{"incomplete entry", "(incomplete)", ""},
		{"all zero", "00:00:00:00:00:00", ""},
		{"broadcast", "ff:ff:ff:ff:ff:ff", ""},
		{"too short", "aa:bb:cc:dd:ee", ""},
		{"wrong separator count", "aabb:ccdd:eeff:0011", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeMAC(tt.in); got != tt.want {
				t.Errorf("normalizeMAC(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSameHost(t *testing.T) {
	want := net.ParseIP("192.168.1.10")

	tests := []struct {
		name string
		peer net.Addr
		want bool
	}{
		{"matching raw icmp peer", &net.IPAddr{IP: net.ParseIP("192.168.1.10")}, true},
		{"matching datagram icmp peer", &net.UDPAddr{IP: net.ParseIP("192.168.1.10")}, true},
		// This is the promiscuity guard: a raw ICMP socket sees replies meant
		// for other probes, and without this check any one reply would mark all
		// 50 concurrent probes online.
		{"different raw peer", &net.IPAddr{IP: net.ParseIP("192.168.1.11")}, false},
		{"different datagram peer", &net.UDPAddr{IP: net.ParseIP("192.168.1.11")}, false},
		{"unexpected address type", &net.TCPAddr{IP: net.ParseIP("192.168.1.10")}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameHost(tt.peer, want); got != tt.want {
				t.Errorf("sameHost(%v, %v) = %v, want %v", tt.peer, want, got, tt.want)
			}
		})
	}
}

func TestReadARPTableBSDParsing(t *testing.T) {
	// Representative `arp -an` output, including the incomplete entries that
	// appear for addresses probed but never answered.
	output := `? (192.168.1.1) at 3c:37:e6:aa:bb:cc on en0 ifscope [ethernet]
? (192.168.1.15) at (incomplete) on en0 ifscope [ethernet]
? (192.168.1.42) at a4:c3:61:11:22:33 on en0 ifscope [ethernet]
? (224.0.0.251) at 1:0:5e:0:0:fb on en0 ifscope permanent [ethernet]`

	got := map[string]string{}
	for _, match := range bsdARPLine.FindAllStringSubmatch(output, -1) {
		if mac := normalizeMAC(match[2]); mac != "" {
			got[match[1]] = mac
		}
	}

	want := map[string]string{
		"192.168.1.1":  "3C:37:E6:AA:BB:CC",
		"192.168.1.42": "A4:C3:61:11:22:33",
	}

	if len(got) != len(want) {
		t.Fatalf("parsed %d entries (%v), want %d", len(got), got, len(want))
	}
	for ip, mac := range want {
		if got[ip] != mac {
			t.Errorf("entry %s = %q, want %q", ip, got[ip], mac)
		}
	}
}
