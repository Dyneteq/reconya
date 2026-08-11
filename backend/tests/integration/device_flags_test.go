package integration

import (
	"testing"

	"reconya/db"
	"reconya/internal/device"
	"reconya/internal/network"
	"reconya/models"
	"reconya/tests/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDeviceServiceStack wires a real DeviceService/NetworkService pair against
// a throwaway SQLite database, the same construction device_handlers_test.go
// uses, so CreateOrUpdateWithDelta's network lookup actually resolves.
func newDeviceServiceStack(t *testing.T) (*device.DeviceService, *network.NetworkService) {
	factory, cleanup := testutils.SetupTestRepositoryFactory(t)
	t.Cleanup(cleanup)

	deviceRepo := factory.NewDeviceRepository()
	networkRepo := factory.NewNetworkRepository()
	cfg := testutils.GetTestConfig()
	dbManager := db.NewDBManager()

	networkService := network.NewNetworkService(networkRepo, cfg, dbManager)
	deviceService := device.NewDeviceService(deviceRepo, networkService, cfg, dbManager, nil)
	return deviceService, networkService
}

func TestDeviceService_CreateOrUpdateWithDelta_PreservesFavoriteAndIgnored(t *testing.T) {
	deviceService, networkService := newDeviceServiceStack(t)

	net, err := networkService.Create("Office", []string{"10.0.1.0/24"}, nil, "", nil)
	require.NoError(t, err)

	first := testutils.CreateTestDeviceWithIP("10.0.1.50")
	first.ID = ""
	first.MAC = nil
	first.NetworkID = net.ID
	saved, _, err := deviceService.CreateOrUpdateWithDelta(first)
	require.NoError(t, err)

	// User marks it favorite and ignored via the drawer.
	updated, err := deviceService.UpdateDevice(saved.ID, nil, nil, boolPtr(true), boolPtr(true), nil)
	require.NoError(t, err)
	assert.True(t, updated.IsFavorite)
	assert.True(t, updated.Ignored)

	// A fresh sweep re-discovers the same IP with a bare struct — none of the
	// curation fields set, exactly like a real ping-sweep result.
	resweep := testutils.CreateTestDeviceWithIP("10.0.1.50")
	resweep.ID = ""
	resweep.MAC = nil
	resweep.NetworkID = net.ID
	resaved, _, err := deviceService.CreateOrUpdateWithDelta(resweep)
	require.NoError(t, err)

	assert.True(t, resaved.IsFavorite, "favorite must survive a re-sweep even though the sweep result never sets it")
	assert.True(t, resaved.Ignored, "ignored must survive a re-sweep even though the sweep result never sets it")
}

func TestDeviceService_CreateOrUpdateWithDelta_Addressing(t *testing.T) {
	deviceService, networkService := newDeviceServiceStack(t)

	net, err := networkService.Create("Office", []string{"10.0.1.0/24"}, nil, "", []string{"10.0.1.0/28"})
	require.NoError(t, err)

	t.Run("derived from the network's static ranges on first sight", func(t *testing.T) {
		inRange := testutils.CreateTestDeviceWithIP("10.0.1.5")
		inRange.ID = ""
		inRange.MAC = nil
		inRange.NetworkID = net.ID
		saved, _, err := deviceService.CreateOrUpdateWithDelta(inRange)
		require.NoError(t, err)
		assert.Equal(t, models.AddressingStatic, saved.Addressing)

		outOfRange := testutils.CreateTestDeviceWithIP("10.0.1.200")
		outOfRange.ID = ""
		outOfRange.MAC = nil
		outOfRange.NetworkID = net.ID
		saved2, _, err := deviceService.CreateOrUpdateWithDelta(outOfRange)
		require.NoError(t, err)
		assert.Equal(t, models.AddressingDHCP, saved2.Addressing)
	})

	t.Run("a manual override survives a re-sweep", func(t *testing.T) {
		d := testutils.CreateTestDeviceWithIP("10.0.1.6")
		d.ID = ""
		d.MAC = nil
		d.NetworkID = net.ID
		saved, _, err := deviceService.CreateOrUpdateWithDelta(d)
		require.NoError(t, err)
		require.Equal(t, models.AddressingStatic, saved.Addressing, "10.0.1.6 is inside the static range")

		dhcp := models.AddressingDHCP
		updated, err := deviceService.UpdateDevice(saved.ID, nil, nil, nil, nil, &dhcp)
		require.NoError(t, err)
		assert.Equal(t, models.AddressingDHCP, updated.Addressing)

		resweep := testutils.CreateTestDeviceWithIP("10.0.1.6")
		resweep.ID = ""
		resweep.MAC = nil
		resweep.NetworkID = net.ID
		resaved, _, err := deviceService.CreateOrUpdateWithDelta(resweep)
		require.NoError(t, err)
		assert.Equal(t, models.AddressingDHCP, resaved.Addressing,
			"manual override must not be clobbered by re-derivation on the next sweep")
	})
}

func TestDeviceService_CreateOrUpdateWithDelta_FirstSeenAtIsImmutable(t *testing.T) {
	deviceService, networkService := newDeviceServiceStack(t)

	net, err := networkService.Create("Office", []string{"10.0.1.0/24"}, nil, "", nil)
	require.NoError(t, err)

	first := testutils.CreateTestDeviceWithIP("10.0.1.50")
	first.ID = ""
	first.MAC = nil
	first.NetworkID = net.ID
	saved, _, err := deviceService.CreateOrUpdateWithDelta(first)
	require.NoError(t, err)
	require.False(t, saved.FirstSeenAt.IsZero(), "first sighting must set first_seen_at")
	firstSeen := saved.FirstSeenAt

	// A later sweep re-discovers the same IP with a bare struct, exactly like a
	// real ping-sweep result.
	resweep := testutils.CreateTestDeviceWithIP("10.0.1.50")
	resweep.ID = ""
	resweep.MAC = nil
	resweep.NetworkID = net.ID
	resaved, _, err := deviceService.CreateOrUpdateWithDelta(resweep)
	require.NoError(t, err)

	assert.True(t, firstSeen.Equal(resaved.FirstSeenAt), "first_seen_at must never change once set")
	assert.False(t, resaved.UpdatedAt.Equal(resaved.FirstSeenAt), "updated_at should have moved on from the original sighting")
}

func TestDeviceService_CreateOrUpdateWithDelta_DiscoveryMethodUpdatesEachSweep(t *testing.T) {
	deviceService, networkService := newDeviceServiceStack(t)

	net, err := networkService.Create("Office", []string{"10.0.1.0/24"}, nil, "", nil)
	require.NoError(t, err)

	first := testutils.CreateTestDeviceWithIP("10.0.1.51")
	first.ID = ""
	first.MAC = nil
	first.NetworkID = net.ID
	first.DiscoveryMethod = models.DiscoveryMethodICMP
	saved, _, err := deviceService.CreateOrUpdateWithDelta(first)
	require.NoError(t, err)
	assert.Equal(t, models.DiscoveryMethodICMP, saved.DiscoveryMethod)

	// Unlike favorite/ignored/addressing, discovery_method is not preserved: the
	// most recent sweep's signal always wins.
	resweep := testutils.CreateTestDeviceWithIP("10.0.1.51")
	resweep.ID = ""
	resweep.MAC = nil
	resweep.NetworkID = net.ID
	resweep.DiscoveryMethod = models.DiscoveryMethodARP
	resaved, _, err := deviceService.CreateOrUpdateWithDelta(resweep)
	require.NoError(t, err)
	assert.Equal(t, models.DiscoveryMethodARP, resaved.DiscoveryMethod, "discovery_method must reflect the latest sweep, not the first one")
}

func TestDeviceService_EligibleForPortScan_Ignored(t *testing.T) {
	deviceService, _ := newDeviceServiceStack(t)

	d := &models.Device{Ignored: true}
	assert.False(t, deviceService.EligibleForPortScan(d), "an ignored device is never eligible, regardless of cooldown")

	notIgnored := &models.Device{Ignored: false}
	assert.True(t, deviceService.EligibleForPortScan(notIgnored))
}

func TestDeviceService_UpdateDevice_FieldsAreIndependent(t *testing.T) {
	deviceService, networkService := newDeviceServiceStack(t)

	net, err := networkService.Create("Office", []string{"10.0.1.0/24"}, nil, "", nil)
	require.NoError(t, err)

	d := testutils.CreateTestDeviceWithIP("10.0.1.50")
	d.ID = ""
	d.MAC = nil
	d.NetworkID = net.ID
	saved, _, err := deviceService.CreateOrUpdateWithDelta(d)
	require.NoError(t, err)

	// Set only IsFavorite; Ignored/Addressing must stay at their defaults.
	updated, err := deviceService.UpdateDevice(saved.ID, nil, nil, boolPtr(true), nil, nil)
	require.NoError(t, err)
	assert.True(t, updated.IsFavorite)
	assert.False(t, updated.Ignored)
	assert.Equal(t, models.AddressingUnknown, updated.Addressing)

	// Set only Ignored; IsFavorite set above must be untouched.
	updated2, err := deviceService.UpdateDevice(saved.ID, nil, nil, nil, boolPtr(true), nil)
	require.NoError(t, err)
	assert.True(t, updated2.IsFavorite, "an unrelated update must not reset a previously-set flag")
	assert.True(t, updated2.Ignored)
}

func boolPtr(b bool) *bool { return &b }
