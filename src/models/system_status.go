package models

import (
	"time"
)

type SystemStatus struct {
	LocalDevice Device             `bson:"local_device" json:"local_device"`
	NetworkID   string             `bson:"network_id,omitempty" json:"network_id,omitempty"`
	PublicIP    *string            `bson:"public_ip,omitempty" json:"public_ip,omitempty"`
	Geolocation *GeolocationCache  `bson:"geolocation,omitempty" json:"geolocation,omitempty"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}
