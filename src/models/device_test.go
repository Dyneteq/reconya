package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestDevice_Creation(t *testing.T) {
	mac := "00:11:22:33:44:55"
	hostname := "test-device"
	vendor := "Test Vendor"
	now := time.Now()
	
	device := Device{
		ID:        uuid.New().String(),
		Name:      "Test Device",
		IPv4:      "192.168.1.100",
		MAC:       &mac,
		Hostname:  &hostname,
		Vendor:    &vendor,
		Status:    DeviceStatusOnline,
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.NotEmpty(t, device.ID)
	assert.Equal(t, "Test Device", device.Name)
	assert.Equal(t, "192.168.1.100", device.IPv4)
	assert.Equal(t, mac, *device.MAC)
	assert.Equal(t, hostname, *device.Hostname)
	assert.Equal(t, vendor, *device.Vendor)
	assert.Equal(t, DeviceStatusOnline, device.Status)
}

func TestDeviceStatus_Constants(t *testing.T) {
	assert.Equal(t, DeviceStatus("unknown"), DeviceStatusUnknown)
	assert.Equal(t, DeviceStatus("online"), DeviceStatusOnline)
	assert.Equal(t, DeviceStatus("idle"), DeviceStatusIdle)
	assert.Equal(t, DeviceStatus("offline"), DeviceStatusOffline)
}

func TestDevice_StatusTransitions(t *testing.T) {
	device := &Device{
		ID:     uuid.New().String(),
		IPv4:   "192.168.1.100",
		Status: DeviceStatusOffline,
	}

	// Test status change from offline to online
	device.Status = DeviceStatusOnline
	now := time.Now()
	device.LastSeenOnlineAt = &now

	assert.Equal(t, DeviceStatusOnline, device.Status)
	assert.NotNil(t, device.LastSeenOnlineAt)
	assert.True(t, device.LastSeenOnlineAt.After(time.Now().Add(-time.Minute)))
}