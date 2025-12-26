package models

import "time"

type SensorStatus string
type SensorScanStatus string

const (
	SensorStatusPending SensorStatus = "pending"
	SensorStatusOnline  SensorStatus = "online"
	SensorStatusOffline SensorStatus = "offline"

	SensorScanStatusIdle    SensorScanStatus = "idle"
	SensorScanStatusRunning SensorScanStatus = "running"
	SensorScanStatusStopped SensorScanStatus = "stopped"
	SensorScanStatusPending SensorScanStatus = "pending"
)

type Sensor struct {
	ID                  string           `json:"id"`
	Name                string           `json:"name"`
	Token               string           `json:"token"`
	Status              SensorStatus     `json:"status"`
	ScanStatus          SensorScanStatus `json:"scan_status"`
	Hostname            *string          `json:"hostname,omitempty"`
	IP                  *string          `json:"ip,omitempty"`
	MAC                 *string          `json:"mac,omitempty"`
	Interface           *string          `json:"interface,omitempty"`
	NetworkCIDR         *string          `json:"network_cidr,omitempty"`
	LastSeenAt          *time.Time       `json:"last_seen_at,omitempty"`
	RegisteredAt        *time.Time       `json:"registered_at,omitempty"`
	LastScanStartedAt   *time.Time       `json:"last_scan_started_at,omitempty"`
	LastScanCompletedAt *time.Time       `json:"last_scan_completed_at,omitempty"`
	LastScanError       *string          `json:"last_scan_error,omitempty"`
	CreatedAt           time.Time        `json:"created_at"`
	UpdatedAt           time.Time        `json:"updated_at"`
}
