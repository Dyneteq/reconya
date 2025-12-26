package integration

import (
	"time"

	"reconya/models"

	"github.com/google/uuid"
)

func createTestDevice(ip, name string) *models.Device {
	mac := "00:11:22:33:44:55"
	hostname := "test-device"
	vendor := "Test Vendor"
	now := time.Now()
	
	return &models.Device{
		ID:               uuid.New().String(),
		Name:             name,
		IPv4:             ip,
		MAC:              &mac,
		Hostname:         &hostname,
		Vendor:           &vendor,
		Status:           models.DeviceStatusOnline,
		CreatedAt:        now,
		UpdatedAt:        now,
		LastSeenOnlineAt: &now, // This is required for the device to be considered "online"
	}
}