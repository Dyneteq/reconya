package models

import (
	"time"
)

// ARPHistory tracks the history of MAC-IP associations for security analysis
type ARPHistory struct {
	ID        string     `json:"id"`
	IP        string     `json:"ip"`
	MAC       string     `json:"mac"`
	NetworkID string     `json:"network_id"`
	FirstSeen time.Time  `json:"first_seen"`
	LastSeen  time.Time  `json:"last_seen"`
	IsGateway bool       `json:"is_gateway"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// ARPChangeType represents the type of ARP change detected
type ARPChangeType string

const (
	// ARPChangeTypeNewDevice indicates a new device was discovered
	ARPChangeTypeNewDevice ARPChangeType = "new_device"
	// ARPChangeTypeMACChange indicates the MAC address changed for an IP
	ARPChangeTypeMACChange ARPChangeType = "mac_change"
	// ARPChangeTypeIPChange indicates the IP changed for a MAC (normal DHCP behavior)
	ARPChangeTypeIPChange ARPChangeType = "ip_change"
	// ARPChangeTypeDuplicateMAC indicates the same MAC is associated with multiple IPs
	ARPChangeTypeDuplicateMAC ARPChangeType = "duplicate_mac"
	// ARPChangeTypeGatewayMAC indicates the gateway MAC address changed (potential MITM)
	ARPChangeTypeGatewayMAC ARPChangeType = "gateway_mac"
)

// ARPHistoryChange represents a detected change in MAC-IP association
type ARPHistoryChange struct {
	Type       ARPChangeType `json:"type"`
	IP         string        `json:"ip"`
	OldMAC     string        `json:"old_mac,omitempty"`
	NewMAC     string        `json:"new_mac"`
	OtherIPs   []string      `json:"other_ips,omitempty"` // For duplicate MAC detection
	NetworkID  string        `json:"network_id"`
	DetectedAt time.Time     `json:"detected_at"`
}

// Severity returns the severity level of the change (for logging/alerting)
func (c *ARPHistoryChange) Severity() string {
	switch c.Type {
	case ARPChangeTypeGatewayMAC:
		return "critical"
	case ARPChangeTypeMACChange:
		return "high"
	case ARPChangeTypeDuplicateMAC:
		return "medium"
	case ARPChangeTypeIPChange, ARPChangeTypeNewDevice:
		return "info"
	default:
		return "unknown"
	}
}

// Description returns a human-readable description of the change
func (c *ARPHistoryChange) Description() string {
	switch c.Type {
	case ARPChangeTypeNewDevice:
		return "New device discovered: " + c.IP + " (" + c.NewMAC + ")"
	case ARPChangeTypeMACChange:
		return "MAC address changed for " + c.IP + ": " + c.OldMAC + " -> " + c.NewMAC
	case ARPChangeTypeIPChange:
		return "IP changed for MAC " + c.NewMAC + " (normal DHCP behavior)"
	case ARPChangeTypeDuplicateMAC:
		return "Duplicate MAC " + c.NewMAC + " detected on multiple IPs"
	case ARPChangeTypeGatewayMAC:
		return "CRITICAL: Gateway " + c.IP + " MAC changed: " + c.OldMAC + " -> " + c.NewMAC
	default:
		return "Unknown ARP change"
	}
}
