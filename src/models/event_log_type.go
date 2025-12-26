package models

type EEventLogType string

const (
	PingSweep         EEventLogType = "Ping sweep"
	PortScanStarted   EEventLogType = "Port scan started"
	PortScanCompleted EEventLogType = "Port scan completed"
	DeviceOnline      EEventLogType = "Device online"
	DeviceIdle        EEventLogType = "Device became idle"
	DeviceOffline     EEventLogType = "Device is now offline"
	DeviceDeleted     EEventLogType = "Device deleted"
	LocalIPFound      EEventLogType = "Local IPv4 address found"
	LocalNetworkFound EEventLogType = "Local network found"
	NetworkCreated    EEventLogType = "Network created"
	NetworkUpdated    EEventLogType = "Network updated"
	NetworkDeleted    EEventLogType = "Network deleted"
	ScanStarted       EEventLogType = "Scan started"
	ScanStopped       EEventLogType = "Scan stopped"
	NewNetworkDetected EEventLogType = "New network detected"
	Warning           EEventLogType = "Warning"
	Alert             EEventLogType = "Alert"
)
