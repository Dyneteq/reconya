package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"reconya/db"
	"reconya/internal/device"
	"reconya/internal/network"
	"reconya/models"
	"reconya/tests/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceHandlers_Integration(t *testing.T) {
	// Setup test database and repositories
	factory, cleanup := testutils.SetupTestRepositoryFactory(t)
	defer cleanup()

	deviceRepo := factory.NewDeviceRepository()
	networkRepo := factory.NewNetworkRepository()
	
	cfg := testutils.GetTestConfig()
	dbManager := db.NewDBManager()
	
	// Create services
	networkService := network.NewNetworkService(networkRepo, cfg, dbManager)
	deviceService := device.NewDeviceService(deviceRepo, networkService, cfg, dbManager, nil) // nil OUI service for tests
	
	// Create handlers
	deviceHandlers := device.NewDeviceHandlers(deviceService, cfg)

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/devices", deviceHandlers.GetAllDevices)
	
	testServer := testutils.NewTestServer(t, mux)
	defer testServer.Close()

	ctx := context.Background()

	t.Run("GetAllDevices_Empty", func(t *testing.T) {
		resp := testServer.GET("/devices")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var devices []models.Device
		err := json.NewDecoder(resp.Body).Decode(&devices)
		require.NoError(t, err)
		
		// Should return empty array for no devices
		assert.Equal(t, 0, len(devices))
	})

	t.Run("GetAllDevices_WithData", func(t *testing.T) {
		// First create a network for the test CIDR
		testNetwork := &models.Network{
			ID:   testutils.CreateTestNetwork().ID,
			CIDR: "192.168.1.0/24",
		}
		savedNetwork, err := networkRepo.CreateOrUpdate(ctx, testNetwork)
		require.NoError(t, err)

		// Create test devices with network association
		testDevices := []*models.Device{
			createTestDevice("192.168.1.200", "HTTP Test Device 1"),
			createTestDevice("192.168.1.201", "HTTP Test Device 2"),
		}

		// Set network ID for all devices
		for _, dev := range testDevices {
			dev.NetworkID = savedNetwork.ID
		}

		// Save devices to database
		for _, dev := range testDevices {
			_, err = deviceRepo.CreateOrUpdate(ctx, dev)
			require.NoError(t, err)
		}

		// Make request
		resp := testServer.GET("/devices")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var devices []models.Device
		err = json.NewDecoder(resp.Body).Decode(&devices)
		require.NoError(t, err)
		
		// Should return our test devices
		assert.GreaterOrEqual(t, len(devices), len(testDevices))
		
		// Verify our devices are present
		deviceNames := make(map[string]bool)
		for _, dev := range devices {
			deviceNames[dev.Name] = true
		}
		
		for _, testDev := range testDevices {
			assert.True(t, deviceNames[testDev.Name], "Device %s should be in response", testDev.Name)
		}
	})

	t.Run("GetAllDevices_ContentType", func(t *testing.T) {
		resp := testServer.GET("/devices")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})
}

