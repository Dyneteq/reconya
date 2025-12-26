package integration

import (
	"context"
	"testing"
	"time"

	"reconya/models"
	"reconya/tests/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceRepository_Integration(t *testing.T) {
	// Setup test database and repositories
	factory, cleanup := testutils.SetupTestRepositoryFactory(t)
	defer cleanup()

	deviceRepo := factory.NewDeviceRepository()
	ctx := context.Background()

	t.Run("CreateAndFindDevice", func(t *testing.T) {
		// Create a test device
		testDevice := createTestDevice("192.168.1.100", "Test Device")

		// Save the device
		savedDevice, err := deviceRepo.CreateOrUpdate(ctx, testDevice)
		require.NoError(t, err)
		require.NotNil(t, savedDevice)

		// Retrieve the device by IP
		retrievedDevice, err := deviceRepo.FindByIP(ctx, testDevice.IPv4)
		require.NoError(t, err)
		require.NotNil(t, retrievedDevice)

		// Verify the device data
		assert.Equal(t, testDevice.IPv4, retrievedDevice.IPv4)
		assert.Equal(t, testDevice.Name, retrievedDevice.Name)
		if testDevice.MAC != nil && retrievedDevice.MAC != nil {
			assert.Equal(t, *testDevice.MAC, *retrievedDevice.MAC)
		}
		assert.Equal(t, testDevice.Status, retrievedDevice.Status)
	})

	t.Run("FindAllDevices", func(t *testing.T) {
		// Create multiple test devices
		devices := []*models.Device{
			createTestDevice("192.168.1.101", "Device 1"),
			createTestDevice("192.168.1.102", "Device 2"),
			createTestDevice("192.168.1.103", "Device 3"),
		}

		// Save all devices
		for _, dev := range devices {
			_, err := deviceRepo.CreateOrUpdate(ctx, dev)
			require.NoError(t, err)
		}

		// Retrieve all devices
		allDevices, err := deviceRepo.FindAll(ctx)
		require.NoError(t, err)

		// Should have at least the devices we created
		assert.GreaterOrEqual(t, len(allDevices), len(devices))

		// Verify our devices are in the result
		deviceIPs := make(map[string]bool)
		for _, dev := range allDevices {
			deviceIPs[dev.IPv4] = true
		}

		for _, dev := range devices {
			assert.True(t, deviceIPs[dev.IPv4], "Device with IP %s should be found", dev.IPv4)
		}
	})

	t.Run("UpdateDeviceStatus", func(t *testing.T) {
		// Create a test device
		testDevice := createTestDevice("192.168.1.104", "Status Test Device")
		testDevice.Status = models.DeviceStatusOffline

		// Save the device
		savedDevice, err := deviceRepo.CreateOrUpdate(ctx, testDevice)
		require.NoError(t, err)

		// Update status to online
		savedDevice.Status = models.DeviceStatusOnline
		now := time.Now()
		savedDevice.LastSeenOnlineAt = &now
		savedDevice.UpdatedAt = now

		updatedDevice, err := deviceRepo.CreateOrUpdate(ctx, savedDevice)
		require.NoError(t, err)

		assert.Equal(t, models.DeviceStatusOnline, updatedDevice.Status)
		assert.NotNil(t, updatedDevice.LastSeenOnlineAt)
	})
}

