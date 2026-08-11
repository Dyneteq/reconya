package testutils

import (
	"time"

	"reconya/models"

	"github.com/google/uuid"
)

func CreateTestDevice() *models.Device {
	mac := "00:11:22:33:44:55"
	hostname := "test-device"
	vendor := "Test Vendor"
	now := time.Now()

	return &models.Device{
		ID:        uuid.New().String(),
		Name:      "Test Device",
		IPv4:      "192.168.1.100",
		MAC:       &mac,
		Hostname:  &hostname,
		Vendor:    &vendor,
		Status:    models.DeviceStatusOnline,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func CreateTestDeviceWithIP(ip string) *models.Device {
	device := CreateTestDevice()
	device.IPv4 = ip
	return device
}

func CreateTestNetwork() *models.Network {
	return CreateTestNetworkWithRanges("192.168.1.0/24")
}

// CreateTestNetworkWithRanges builds a network owning one range per cidr
// given, all active. network.CIDR mirrors the first range for callers that
// still read the legacy single-CIDR field.
func CreateTestNetworkWithRanges(cidrs ...string) *models.Network {
	now := time.Now()
	id := uuid.New().String()

	ranges := make([]models.NetworkRange, len(cidrs))
	for i, cidr := range cidrs {
		ranges[i] = models.NetworkRange{
			ID:        uuid.New().String(),
			NetworkID: id,
			CIDR:      cidr,
			Active:    true,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}

	cidr := ""
	if len(cidrs) > 0 {
		cidr = cidrs[0]
	}

	return &models.Network{
		ID:     id,
		CIDR:   cidr,
		Ranges: ranges,
	}
}

func CreateTestEventLog(deviceID string) *models.EventLog {
	now := time.Now()
	return &models.EventLog{
		Type:        models.DeviceOnline,
		Description: "Test event message",
		DeviceID:    &deviceID,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}
}

func CreateTestSystemStatus() *models.SystemStatus {
	publicIP := "203.0.113.1"
	now := time.Now()

	return &models.SystemStatus{
		LocalDevice: *CreateTestDevice(),
		NetworkID:   uuid.New().String(),
		PublicIP:    &publicIP,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
