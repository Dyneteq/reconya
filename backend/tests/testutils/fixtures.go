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
	return &models.Network{
		ID:   uuid.New().String(),
		CIDR: "192.168.1.0/24",
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