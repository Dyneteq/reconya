package security

import (
	"context"
	"log"
	"reconya/db"
	"reconya/models"
	"strings"
	"time"
)

// SecurityEventLogger is a simple interface for logging security events
// This avoids circular dependency with eventlog package
type SecurityEventLogger interface {
	Log(eventType models.EEventLogType, description string, deviceID string) error
}

// MITMDetector detects potential MITM attacks via ARP analysis
type MITMDetector struct {
	arpHistoryRepo db.ARPHistoryRepository
	eventLogger    SecurityEventLogger
	// Time window for considering entries "current" (for detecting active attacks)
	activityWindow time.Duration
}

// NewMITMDetector creates a new MITM detector
func NewMITMDetector(arpHistoryRepo db.ARPHistoryRepository, eventLogger SecurityEventLogger) *MITMDetector {
	return &MITMDetector{
		arpHistoryRepo: arpHistoryRepo,
		eventLogger:    eventLogger,
		activityWindow: 5 * time.Minute,
	}
}

// SetActivityWindow sets the time window for detecting active threats
func (d *MITMDetector) SetActivityWindow(window time.Duration) {
	d.activityWindow = window
}

// AnalyzeARPEntry checks for suspicious ARP behavior and returns detected changes
func (d *MITMDetector) AnalyzeARPEntry(ctx context.Context, ip, mac, networkID string, isGateway bool) []models.ARPHistoryChange {
	var changes []models.ARPHistoryChange

	// 1. Check for MAC change (same IP, different MAC) - potential ARP spoofing
	if change := d.detectMACChange(ctx, ip, mac, networkID); change != nil {
		changes = append(changes, *change)
	}

	// 2. Check for duplicate MAC (same MAC, multiple active IPs) - potential device impersonation
	if change := d.detectDuplicateMAC(ctx, ip, mac, networkID); change != nil {
		changes = append(changes, *change)
	}

	// 3. Check for gateway MAC change (most critical - classic MITM attack)
	if isGateway {
		if change := d.detectGatewayMACChange(ctx, ip, mac, networkID); change != nil {
			changes = append(changes, *change)
		}
	}

	// Log all detected changes
	for _, change := range changes {
		d.logSecurityEvent(change)
	}

	return changes
}

// detectMACChange checks if the IP previously had a different MAC
func (d *MITMDetector) detectMACChange(ctx context.Context, ip, newMAC, networkID string) *models.ARPHistoryChange {
	// Get recent MAC addresses for this IP
	entries, err := d.arpHistoryRepo.GetCurrentMACsForIP(ctx, ip, d.activityWindow)
	if err != nil || len(entries) == 0 {
		return nil
	}

	// Check if any recent entry has a different MAC
	for _, entry := range entries {
		if !strings.EqualFold(entry.MAC, newMAC) {
			return &models.ARPHistoryChange{
				Type:       models.ARPChangeTypeMACChange,
				IP:         ip,
				OldMAC:     entry.MAC,
				NewMAC:     newMAC,
				NetworkID:  networkID,
				DetectedAt: time.Now(),
			}
		}
	}

	return nil
}

// detectDuplicateMAC checks if the MAC is associated with multiple IPs
func (d *MITMDetector) detectDuplicateMAC(ctx context.Context, ip, mac, networkID string) *models.ARPHistoryChange {
	// Get recent IPs for this MAC
	entries, err := d.arpHistoryRepo.GetCurrentIPsForMAC(ctx, mac, d.activityWindow)
	if err != nil || len(entries) <= 1 {
		return nil
	}

	// Collect other IPs with same MAC
	var otherIPs []string
	for _, entry := range entries {
		if entry.IP != ip {
			otherIPs = append(otherIPs, entry.IP)
		}
	}

	// Multiple IPs for same MAC could indicate:
	// 1. Normal: Device with multiple IPs (router, dual-homed host)
	// 2. Suspicious: MAC spoofing / device impersonation
	if len(otherIPs) > 0 {
		return &models.ARPHistoryChange{
			Type:       models.ARPChangeTypeDuplicateMAC,
			IP:         ip,
			NewMAC:     mac,
			OtherIPs:   otherIPs,
			NetworkID:  networkID,
			DetectedAt: time.Now(),
		}
	}

	return nil
}

// detectGatewayMACChange checks if the gateway MAC has changed
func (d *MITMDetector) detectGatewayMACChange(ctx context.Context, ip, newMAC, networkID string) *models.ARPHistoryChange {
	// Get the last known gateway MAC for this network
	gateway, err := d.arpHistoryRepo.FindGatewayForNetwork(ctx, networkID)
	if err != nil || gateway == nil {
		// No previous gateway record - this is the first time we've seen it
		return nil
	}

	// Gateway IP should match and MAC should differ
	if gateway.IP == ip && !strings.EqualFold(gateway.MAC, newMAC) {
		return &models.ARPHistoryChange{
			Type:       models.ARPChangeTypeGatewayMAC,
			IP:         ip,
			OldMAC:     gateway.MAC,
			NewMAC:     newMAC,
			NetworkID:  networkID,
			DetectedAt: time.Now(),
		}
	}

	return nil
}

// logSecurityEvent logs a security event to the event log
func (d *MITMDetector) logSecurityEvent(change models.ARPHistoryChange) {
	var eventType models.EEventLogType
	var prefix string

	switch change.Type {
	case models.ARPChangeTypeMACChange:
		eventType = models.SecurityMACChange
		prefix = "SECURITY [HIGH]"
	case models.ARPChangeTypeDuplicateMAC:
		eventType = models.SecurityDuplicateMAC
		prefix = "SECURITY [MEDIUM]"
	case models.ARPChangeTypeGatewayMAC:
		eventType = models.SecurityGatewayMAC
		prefix = "SECURITY [CRITICAL]"
	default:
		eventType = models.SecurityARPAnomaly
		prefix = "SECURITY [INFO]"
	}

	description := change.Description()
	log.Printf("%s: %s", prefix, description)

	if d.eventLogger != nil {
		err := d.eventLogger.Log(eventType, description, "")
		if err != nil {
			log.Printf("Error logging security event: %v", err)
		}
	}
}

// IsLikelyGateway checks if an IP is likely to be a gateway based on common patterns
func IsLikelyGateway(ip string) bool {
	// Common gateway IP patterns
	gatewayPatterns := []string{".1", ".254"}
	for _, pattern := range gatewayPatterns {
		if strings.HasSuffix(ip, pattern) {
			return true
		}
	}
	return false
}
