package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func multiRangeNetwork() *Network {
	return &Network{
		ID:            "net-1",
		AddressFamily: AddressFamilyIPv4,
		Ranges: []NetworkRange{
			{ID: "r1", CIDR: "10.0.1.0/24", Active: true},
			{ID: "r2", CIDR: "10.0.2.0/24", Active: false},
			{ID: "r3", CIDR: "10.0.3.0/24", Active: true},
		},
	}
}

func TestNetwork_ActiveRanges(t *testing.T) {
	n := multiRangeNetwork()

	active := n.ActiveRanges()

	assert.Len(t, active, 2)
	assert.Equal(t, "10.0.1.0/24", active[0].CIDR)
	assert.Equal(t, "10.0.3.0/24", active[1].CIDR)
}

func TestNetwork_ContainsIPv4_AcrossRanges(t *testing.T) {
	n := multiRangeNetwork()

	assert.True(t, n.ContainsIPv4("10.0.1.5"))
	assert.True(t, n.ContainsIPv4("10.0.2.5"), "inactive ranges still count as owned addresses")
	assert.True(t, n.ContainsIPv4("10.0.3.5"))
	assert.False(t, n.ContainsIPv4("10.0.4.5"))
}

func TestNetwork_ContainsIPv4_FallsBackToLegacyCIDR(t *testing.T) {
	n := &Network{
		AddressFamily: AddressFamilyIPv4,
		CIDR:          "192.168.1.0/24",
	}

	assert.True(t, n.ContainsIPv4("192.168.1.42"))
	assert.False(t, n.ContainsIPv4("10.0.0.1"))
}

func TestNetwork_GetDisplayName(t *testing.T) {
	named := multiRangeNetwork()
	named.Name = "Office LAN"
	assert.Equal(t, "Office LAN", named.GetDisplayName())

	twoRanges := &Network{
		AddressFamily: AddressFamilyIPv4,
		Ranges: []NetworkRange{
			{CIDR: "10.0.1.0/24", Active: true},
			{CIDR: "10.0.2.0/24", Active: true},
		},
	}
	assert.Equal(t, "10.0.1.0/24 + 10.0.2.0/24", twoRanges.GetDisplayName())

	threeRanges := &Network{
		AddressFamily: AddressFamilyIPv4,
		Ranges: []NetworkRange{
			{CIDR: "10.0.1.0/24", Active: true},
			{CIDR: "10.0.2.0/24", Active: true},
			{CIDR: "10.0.3.0/24", Active: true},
		},
	}
	assert.Equal(t, "10.0.1.0/24 +2 more", threeRanges.GetDisplayName())

	single := &Network{
		AddressFamily: AddressFamilyIPv4,
		Ranges:        []NetworkRange{{CIDR: "10.0.1.0/24", Active: true}},
	}
	assert.Equal(t, "10.0.1.0/24", single.GetDisplayName())

	legacy := &Network{AddressFamily: AddressFamilyIPv4, CIDR: "10.0.1.0/24"}
	assert.Equal(t, "10.0.1.0/24", legacy.GetDisplayName())
}

func TestValidateNetworkRanges(t *testing.T) {
	assert.NoError(t, ValidateNetworkRanges([]string{"10.0.1.0/24", "10.0.2.0/24"}))
	assert.Error(t, ValidateNetworkRanges(nil))
	assert.Error(t, ValidateNetworkRanges([]string{}))
	assert.Error(t, ValidateNetworkRanges([]string{"not-a-cidr"}))
	assert.Error(t, ValidateNetworkRanges([]string{"10.0.1.0/24", "garbage"}))
}

func TestValidateStaticRanges(t *testing.T) {
	assert.NoError(t, ValidateStaticRanges(nil), "no static ranges configured is valid")
	assert.NoError(t, ValidateStaticRanges([]string{}))
	assert.NoError(t, ValidateStaticRanges([]string{"10.0.1.0/28"}))
	assert.Error(t, ValidateStaticRanges([]string{"not-a-cidr"}))
}

func TestNetwork_AddressingForIP(t *testing.T) {
	t.Run("no static ranges configured", func(t *testing.T) {
		n := &Network{}
		assert.Equal(t, AddressingUnknown, n.AddressingForIP("10.0.1.5"))
	})

	t.Run("IP inside a static range", func(t *testing.T) {
		n := &Network{StaticRanges: []string{"10.0.1.0/28"}}
		assert.Equal(t, AddressingStatic, n.AddressingForIP("10.0.1.5"))
	})

	t.Run("IP outside all configured ranges is dhcp", func(t *testing.T) {
		n := &Network{StaticRanges: []string{"10.0.1.0/28"}}
		assert.Equal(t, AddressingDHCP, n.AddressingForIP("10.0.1.200"))
	})

	t.Run("unparseable IP is unknown", func(t *testing.T) {
		n := &Network{StaticRanges: []string{"10.0.1.0/28"}}
		assert.Equal(t, AddressingUnknown, n.AddressingForIP("not-an-ip"))
	})
}
