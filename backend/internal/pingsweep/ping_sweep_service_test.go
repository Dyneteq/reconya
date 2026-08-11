package pingsweep

import (
	"testing"

	"reconya/models"

	"github.com/stretchr/testify/assert"
)

func TestMergeDevicesByIP_DedupesAcrossRanges(t *testing.T) {
	rangeA := []models.Device{
		{IPv4: "10.0.1.5", Name: "a"},
		{IPv4: "10.0.1.6", Name: "b"},
	}
	rangeB := []models.Device{
		{IPv4: "10.0.2.5", Name: "c"},
	}

	merged := mergeDevicesByIP(rangeA, rangeB)

	assert.Len(t, merged, 3)
	assertContainsIP(t, merged, "10.0.1.5")
	assertContainsIP(t, merged, "10.0.1.6")
	assertContainsIP(t, merged, "10.0.2.5")
}

func TestMergeDevicesByIP_LaterBatchWinsOnOverlap(t *testing.T) {
	rangeA := []models.Device{{IPv4: "10.0.1.5", Name: "stale"}}
	rangeB := []models.Device{{IPv4: "10.0.1.5", Name: "fresh"}}

	merged := mergeDevicesByIP(rangeA, rangeB)

	assert.Len(t, merged, 1)
	assert.Equal(t, "fresh", merged[0].Name)
}

func TestMergeDevicesByIP_Empty(t *testing.T) {
	assert.Empty(t, mergeDevicesByIP())
	assert.Empty(t, mergeDevicesByIP(nil, []models.Device{}))
}

func assertContainsIP(t *testing.T, devices []models.Device, ip string) {
	t.Helper()
	for _, d := range devices {
		if d.IPv4 == ip {
			return
		}
	}
	t.Errorf("expected devices to contain IP %s", ip)
}
