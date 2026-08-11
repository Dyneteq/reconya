package network_test

import (
	"testing"

	"reconya/db"
	"reconya/internal/network"
	"reconya/tests/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T) *network.NetworkService {
	factory, cleanup := testutils.SetupTestRepositoryFactory(t)
	t.Cleanup(cleanup)

	repo := factory.NewNetworkRepository()
	cfg := testutils.GetTestConfig()
	dbManager := db.NewDBManager()

	return network.NewNetworkService(repo, cfg, dbManager)
}

func TestNetworkService_Create_MultipleRanges(t *testing.T) {
	svc := newTestService(t)

	n, err := svc.Create("Office", []string{"10.0.1.0/24", "10.0.2.0/24"}, []string{"floor 1", "floor 2"}, "desc")
	require.NoError(t, err)

	assert.Len(t, n.Ranges, 2)
	assert.Equal(t, "10.0.1.0/24", n.Ranges[0].CIDR)
	assert.Equal(t, "floor 1", n.Ranges[0].Label)
	assert.True(t, n.Ranges[0].Active)
	assert.Equal(t, "10.0.1.0/24", n.CIDR, "legacy CIDR column mirrors the first range")

	reloaded, err := svc.FindByID(n.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded)
	assert.Len(t, reloaded.Ranges, 2)
}

func TestNetworkService_Create_RejectsInvalidCIDR(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.Create("Office", []string{"not-a-cidr"}, nil, "")
	assert.Error(t, err)

	_, err = svc.Create("Office", nil, nil, "")
	assert.Error(t, err)
}

func TestNetworkService_Update_PreservesRangeIDAcrossEdit(t *testing.T) {
	svc := newTestService(t)

	created, err := svc.Create("Office", []string{"10.0.1.0/24", "10.0.2.0/24"}, nil, "")
	require.NoError(t, err)

	originalRangeID := created.Ranges[0].ID
	require.NotEmpty(t, originalRangeID)

	// Update keeps 10.0.1.0/24, drops 10.0.2.0/24, adds 10.0.3.0/24.
	updated, err := svc.Update(created.ID, "Office", []string{"10.0.1.0/24", "10.0.3.0/24"}, nil, "")
	require.NoError(t, err)

	var kept, dropped, added *string
	for i := range updated.Ranges {
		r := updated.Ranges[i]
		switch r.CIDR {
		case "10.0.1.0/24":
			kept = &r.ID
		case "10.0.3.0/24":
			added = &r.ID
		}
	}
	_ = dropped

	require.NotNil(t, kept)
	assert.Equal(t, originalRangeID, *kept, "surviving range keeps its id and scan history")
	require.NotNil(t, added)
	assert.NotEmpty(t, *added)

	reloaded, err := svc.FindByID(created.ID)
	require.NoError(t, err)

	active := reloaded.ActiveRanges()
	activeCIDRs := make([]string, len(active))
	for i, r := range active {
		activeCIDRs[i] = r.CIDR
	}
	assert.ElementsMatch(t, []string{"10.0.1.0/24", "10.0.3.0/24"}, activeCIDRs,
		"dropped range is deactivated, not deleted")
}

func TestNetworkService_SetRangeActive(t *testing.T) {
	svc := newTestService(t)

	created, err := svc.Create("Office", []string{"10.0.1.0/24"}, nil, "")
	require.NoError(t, err)
	rangeID := created.Ranges[0].ID

	require.NoError(t, svc.SetRangeActive(rangeID, false))

	reloaded, err := svc.FindByID(created.ID)
	require.NoError(t, err)
	require.Len(t, reloaded.Ranges, 1)
	assert.False(t, reloaded.Ranges[0].Active)
	assert.Empty(t, reloaded.ActiveRanges())

	require.NoError(t, svc.SetRangeActive(rangeID, true))

	reloaded, err = svc.FindByID(created.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.Ranges[0].Active)
}

func TestNetworkService_FindOrCreate_SingleRange(t *testing.T) {
	svc := newTestService(t)

	created, err := svc.FindOrCreate("10.0.1.0/24")
	require.NoError(t, err)
	require.Len(t, created.Ranges, 1)

	found, err := svc.FindOrCreate("10.0.1.0/24")
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID, "second call finds the existing network by range CIDR")
}
