package models

import (
	"time"
)

// Settings represents user settings stored in the database
type Settings struct {
	ID                 string     `json:"id" db:"id"`
	UserID             string     `json:"user_id" db:"user_id"`
	ScreenshotsEnabled bool       `json:"screenshots_enabled" db:"screenshots_enabled"`
	CreatedAt          *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          *time.Time `json:"updated_at" db:"updated_at"`
}

// DefaultSettings returns default settings for new users
func DefaultSettings() *Settings {
	return &Settings{
		ScreenshotsEnabled: true, // Screenshots enabled by default
	}
}
