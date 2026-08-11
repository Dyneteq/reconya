package models

import (
	"fmt"
	"net"
	"time"
)

// AddressFamily represents the IP address family of a network
type AddressFamily string

const (
	AddressFamilyIPv4 AddressFamily = "ipv4"
	AddressFamilyIPv6 AddressFamily = "ipv6"
	AddressFamilyDual AddressFamily = "dual"
)

type Network struct {
	ID            string         `bson:"_id,omitempty" json:"id"`
	Name          string         `bson:"name" json:"name"`
	CIDR          string         `bson:"cidr" json:"cidr"`
	IPv6Prefix    *string        `bson:"ipv6_prefix,omitempty" json:"ipv6_prefix,omitempty"`
	AddressFamily AddressFamily  `bson:"address_family" json:"address_family"`
	Description   string         `bson:"description" json:"description"`
	Status        string         `bson:"status" json:"status"`
	LastScannedAt *time.Time     `bson:"last_scanned_at" json:"last_scanned_at"`
	DeviceCount   int            `bson:"device_count" json:"device_count"`
	Ranges        []NetworkRange `bson:"ranges,omitempty" json:"ranges"`
	StaticRanges  []string       `bson:"static_ranges,omitempty" json:"static_ranges,omitempty"`
	CreatedAt     time.Time      `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time      `bson:"updated_at" json:"updated_at"`
}

// NetworkRange is one IPv4 CIDR range scoped under a Network. A Network can
// own several ranges so scans target the real subnets in use instead of a
// covering supernet.
type NetworkRange struct {
	ID            string     `json:"id"`
	NetworkID     string     `json:"network_id"`
	CIDR          string     `json:"cidr"`
	Label         string     `json:"label"`
	Active        bool       `json:"active"`
	LastScannedAt *time.Time `json:"last_scanned_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// ActiveRanges returns the ranges eligible for scanning.
func (n *Network) ActiveRanges() []NetworkRange {
	active := make([]NetworkRange, 0, len(n.Ranges))
	for _, r := range n.Ranges {
		if r.Active {
			active = append(active, r)
		}
	}
	return active
}

// ValidateNetworkRanges validates a set of CIDR strings intended for a
// network's ranges.
func ValidateNetworkRanges(cidrs []string) error {
	if len(cidrs) == 0 {
		return fmt.Errorf("at least one CIDR range is required")
	}
	for _, cidr := range cidrs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("invalid CIDR %q: %w", cidr, err)
		}
	}
	return nil
}

// ValidateStaticRanges validates a set of CIDR strings intended for a
// network's static-address ranges. Unlike ValidateNetworkRanges, an empty
// set is valid (it just means no static ranges are defined).
func ValidateStaticRanges(cidrs []string) error {
	for _, cidr := range cidrs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("invalid static range %q: %w", cidr, err)
		}
	}
	return nil
}

// AddressingForIP derives a Device's Addressing from this network's
// StaticRanges: static if the IP falls inside one of them, dhcp if ranges
// are configured but the IP falls outside all of them, and unknown if no
// static ranges are configured at all.
func (n *Network) AddressingForIP(ip string) Addressing {
	if len(n.StaticRanges) == 0 {
		return AddressingUnknown
	}

	testIP := net.ParseIP(ip)
	if testIP == nil {
		return AddressingUnknown
	}

	for _, cidr := range n.StaticRanges {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(testIP) {
			return AddressingStatic
		}
	}

	return AddressingDHCP
}

func (n *Network) IsIPv4Enabled() bool {
	return n.AddressFamily == AddressFamilyIPv4 || n.AddressFamily == AddressFamilyDual
}

func (n *Network) IsIPv6Enabled() bool {
	return n.AddressFamily == AddressFamilyIPv6 || n.AddressFamily == AddressFamilyDual
}

func (n *Network) IsDualStack() bool {
	return n.AddressFamily == AddressFamilyDual
}

func (n *Network) GetIPv4CIDR() string {
	if n.IsIPv4Enabled() {
		return n.CIDR
	}
	return ""
}

func (n *Network) GetIPv6Prefix() string {
	if n.IsIPv6Enabled() && n.IPv6Prefix != nil {
		return *n.IPv6Prefix
	}
	return ""
}

func (n *Network) ValidateNetworkAddresses() error {
	if n.IsIPv4Enabled() {
		if n.CIDR == "" {
			return fmt.Errorf("IPv4 CIDR is required for IPv4 or dual-stack networks")
		}
		if _, _, err := net.ParseCIDR(n.CIDR); err != nil {
			return fmt.Errorf("invalid IPv4 CIDR: %w", err)
		}
	}

	if n.IsIPv6Enabled() {
		if n.IPv6Prefix == nil || *n.IPv6Prefix == "" {
			return fmt.Errorf("IPv6 prefix is required for IPv6 or dual-stack networks")
		}
		if _, _, err := net.ParseCIDR(*n.IPv6Prefix); err != nil {
			return fmt.Errorf("invalid IPv6 prefix: %w", err)
		}
	}

	return nil
}

func (n *Network) ContainsIPv4(ip string) bool {
	if !n.IsIPv4Enabled() {
		return false
	}

	testIP := net.ParseIP(ip)
	if testIP == nil {
		return false
	}

	cidrs := n.rangeCIDRs()
	if len(cidrs) == 0 {
		cidrs = []string{n.CIDR}
	}

	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(testIP) {
			return true
		}
	}

	return false
}

func (n *Network) rangeCIDRs() []string {
	cidrs := make([]string, 0, len(n.Ranges))
	for _, r := range n.Ranges {
		cidrs = append(cidrs, r.CIDR)
	}
	return cidrs
}

func (n *Network) ContainsIPv6(ip string) bool {
	if !n.IsIPv6Enabled() || n.IPv6Prefix == nil {
		return false
	}

	_, ipNet, err := net.ParseCIDR(*n.IPv6Prefix)
	if err != nil {
		return false
	}

	testIP := net.ParseIP(ip)
	if testIP == nil {
		return false
	}

	return ipNet.Contains(testIP)
}

func (n *Network) GetDisplayName() string {
	if n.Name != "" {
		return n.Name
	}

	if n.IsDualStack() {
		return fmt.Sprintf("%s + %s", n.CIDR, n.GetIPv6Prefix())
	}

	if n.IsIPv6Enabled() {
		return n.GetIPv6Prefix()
	}

	active := n.ActiveRanges()
	if len(active) > 1 {
		if len(active) == 2 {
			return fmt.Sprintf("%s + %s", active[0].CIDR, active[1].CIDR)
		}
		return fmt.Sprintf("%s +%d more", active[0].CIDR, len(active)-1)
	}
	if len(active) == 1 {
		return active[0].CIDR
	}

	return n.CIDR
}

func (n *Network) GetNetworkType() string {
	switch n.AddressFamily {
	case AddressFamilyIPv4:
		return "IPv4"
	case AddressFamilyIPv6:
		return "IPv6"
	case AddressFamilyDual:
		return "Dual Stack"
	default:
		return "Unknown"
	}
}
