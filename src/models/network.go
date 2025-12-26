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
	ID             string        `bson:"_id,omitempty" json:"id"`
	Name           string        `bson:"name" json:"name"`
	CIDR           string        `bson:"cidr" json:"cidr"`
	IPv6Prefix     *string       `bson:"ipv6_prefix,omitempty" json:"ipv6_prefix,omitempty"`
	AddressFamily  AddressFamily `bson:"address_family" json:"address_family"`
	Description    string        `bson:"description" json:"description"`
	Status         string        `bson:"status" json:"status"`
	LastScannedAt  *time.Time    `bson:"last_scanned_at" json:"last_scanned_at"`
	DeviceCount    int           `bson:"device_count" json:"device_count"`
	CreatedAt      time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time     `bson:"updated_at" json:"updated_at"`
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
	
	_, ipNet, err := net.ParseCIDR(n.CIDR)
	if err != nil {
		return false
	}
	
	testIP := net.ParseIP(ip)
	if testIP == nil {
		return false
	}
	
	return ipNet.Contains(testIP)
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
