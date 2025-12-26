package models

import "time"

type WebService struct {
	URL         string    `bson:"url" json:"url"`
	Title       string    `bson:"title,omitempty" json:"title,omitempty"`
	Server      string    `bson:"server,omitempty" json:"server,omitempty"`
	StatusCode  int       `bson:"status_code" json:"status_code"`
	ContentType string    `bson:"content_type,omitempty" json:"content_type,omitempty"`
	Size        int64     `bson:"size,omitempty" json:"size,omitempty"`
	Screenshot  string    `bson:"screenshot,omitempty" json:"screenshot,omitempty"`
	Port        int       `bson:"port" json:"port"`
	Protocol    string    `bson:"protocol" json:"protocol"`
	ScannedAt   time.Time `bson:"scanned_at" json:"scanned_at"`
}