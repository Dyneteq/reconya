package models

import (
	"time"
)

// EventLog is the append-only activity trail shown in the console's EVENTS feed.
//
// The json tags matter: without them Go marshals the field names verbatim, and
// the frontend read `Message` (a field that has never existed here) so every row
// rendered "No description".
type EventLog struct {
	Type            EEventLogType `bson:"type" json:"type"`
	Description     string        `bson:"description" json:"description"`
	DeviceID        *string       `bson:"device_id,omitempty" json:"device_id,omitempty"`
	DurationSeconds *float64      `bson:"duration_seconds,omitempty" json:"duration_seconds,omitempty"`
	CreatedAt       *time.Time    `bson:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt       *time.Time    `bson:"updated_at,omitempty" json:"updated_at,omitempty"`
}
