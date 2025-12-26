package models

import (
	"time"
)

// GeolocationCache stores cached geolocation data for IP addresses
type GeolocationCache struct {
	ID          string    `db:"id" json:"id"`
	IP          string    `db:"ip" json:"ip"`
	City        string    `db:"city" json:"city"`
	Region      string    `db:"region" json:"region"`
	Country     string    `db:"country" json:"country"`
	CountryCode string    `db:"country_code" json:"country_code"`
	Latitude    float64   `db:"latitude" json:"latitude"`
	Longitude   float64   `db:"longitude" json:"longitude"`
	Timezone    string    `db:"timezone" json:"timezone"`
	ISP         string    `db:"isp" json:"isp"`
	Source      string    `db:"source" json:"source"` // "api", "fallback", "manual"
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
	ExpiresAt   time.Time `db:"expires_at" json:"expires_at"` // Cache expiration
}