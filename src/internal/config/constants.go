package config

import "time"

const (
	DeviceUpdaterInterval      = 5 * time.Second
	NetworkDetectionInterval   = 30 * time.Second
	GeolocationCleanupInterval = 6 * time.Hour
	PublicIPRefreshInterval    = 15 * time.Minute
	AgentScanInterval          = 30 * time.Second
	AgentUIRefreshInterval     = 150 * time.Millisecond
	AgentShutdownPause         = 100 * time.Millisecond
	AgentOnlineThreshold       = 30 * time.Second
	DeviceUpdateRetryDelay     = 1 * time.Second
	PortScanCooldown           = 15 * time.Minute
	MDNSDiscoveryInterval      = 2 * time.Minute
	MDNSDiscoveryTimeout       = 3 * time.Second
	MDNSResolveCommandTimeout  = 2 * time.Second
	MDNSFallbackTimeout        = 500 * time.Millisecond
	ReverseDNSTimeout          = 300 * time.Millisecond
	ScanLoopInterval           = 30 * time.Second
)
