package models

import (
	"time"
)

type EventLog struct {
	Type            EEventLogType `bson:"type"`
	Description     string        `bson:"description"`
	DeviceID        *string       `bson:"device_id,omitempty"`
	SensorID        *string       `bson:"sensor_id,omitempty"`
	DurationSeconds *float64      `bson:"duration_seconds,omitempty"`
	CreatedAt       *time.Time    `bson:"created_at,omitempty"`
	UpdatedAt       *time.Time    `bson:"updated_at,omitempty"`
}
