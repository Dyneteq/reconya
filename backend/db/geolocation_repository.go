package db

import (
	"context"
	"database/sql"
	"fmt"
	"reconya/models"
	"time"

	"github.com/google/uuid"
)

type GeolocationRepository struct {
	db *sql.DB
}

func NewGeolocationRepository(db *sql.DB) *GeolocationRepository {
	return &GeolocationRepository{db: db}
}

// FindByIP retrieves cached geolocation data for an IP address
func (r *GeolocationRepository) FindByIP(ctx context.Context, ip string) (*models.GeolocationCache, error) {
	query := `
		SELECT id, ip, city, region, country, country_code, latitude, longitude, 
		       timezone, isp, source, created_at, updated_at, expires_at
		FROM geolocation_cache 
		WHERE ip = ? AND expires_at > ?
	`

	var cache models.GeolocationCache
	err := r.db.QueryRowContext(ctx, query, ip, time.Now()).Scan(
		&cache.ID, &cache.IP, &cache.City, &cache.Region, &cache.Country,
		&cache.CountryCode, &cache.Latitude, &cache.Longitude, &cache.Timezone,
		&cache.ISP, &cache.Source, &cache.CreatedAt, &cache.UpdatedAt, &cache.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find geolocation cache by IP: %w", err)
	}

	return &cache, nil
}

// Create stores new geolocation data in cache
func (r *GeolocationRepository) Create(ctx context.Context, cache *models.GeolocationCache) error {
	if cache.ID == "" {
		cache.ID = uuid.New().String()
	}

	now := time.Now()
	cache.CreatedAt = now
	cache.UpdatedAt = now

	// Set cache expiration (7 days for API data, 30 days for fallback data)
	if cache.Source == "api" {
		cache.ExpiresAt = now.Add(7 * 24 * time.Hour)
	} else {
		cache.ExpiresAt = now.Add(30 * 24 * time.Hour)
	}

	query := `
		INSERT INTO geolocation_cache (
			id, ip, city, region, country, country_code, latitude, longitude,
			timezone, isp, source, created_at, updated_at, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		cache.ID, cache.IP, cache.City, cache.Region, cache.Country,
		cache.CountryCode, cache.Latitude, cache.Longitude, cache.Timezone,
		cache.ISP, cache.Source, cache.CreatedAt, cache.UpdatedAt, cache.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create geolocation cache: %w", err)
	}

	return nil
}

// Update updates existing geolocation cache
func (r *GeolocationRepository) Update(ctx context.Context, cache *models.GeolocationCache) error {
	now := time.Now()
	cache.UpdatedAt = now

	// Update cache expiration
	if cache.Source == "api" {
		cache.ExpiresAt = now.Add(7 * 24 * time.Hour)
	} else {
		cache.ExpiresAt = now.Add(30 * 24 * time.Hour)
	}

	query := `
		UPDATE geolocation_cache SET
			city = ?, region = ?, country = ?, country_code = ?, latitude = ?, longitude = ?,
			timezone = ?, isp = ?, source = ?, updated_at = ?, expires_at = ?
		WHERE ip = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		cache.City, cache.Region, cache.Country, cache.CountryCode,
		cache.Latitude, cache.Longitude, cache.Timezone, cache.ISP,
		cache.Source, cache.UpdatedAt, cache.ExpiresAt, cache.IP,
	)

	if err != nil {
		return fmt.Errorf("failed to update geolocation cache: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// Upsert creates or updates geolocation cache
func (r *GeolocationRepository) Upsert(ctx context.Context, cache *models.GeolocationCache) error {
	// Try to find existing cache
	existing, err := r.FindByIP(ctx, cache.IP)
	if err == ErrNotFound {
		// Create new cache entry
		return r.Create(ctx, cache)
	}
	if err != nil {
		return err
	}

	// Update existing cache
	cache.ID = existing.ID
	cache.CreatedAt = existing.CreatedAt
	return r.Update(ctx, cache)
}

// CleanupExpired removes expired cache entries
func (r *GeolocationRepository) CleanupExpired(ctx context.Context) error {
	query := `DELETE FROM geolocation_cache WHERE expires_at < ?`

	result, err := r.db.ExecContext(ctx, query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to cleanup expired geolocation cache: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected > 0 {
		fmt.Printf("Cleaned up %d expired geolocation cache entries\n", rowsAffected)
	}

	return nil
}

// IsValidCache checks if cached data has all required fields
func (r *GeolocationRepository) IsValidCache(cache *models.GeolocationCache) bool {
	return cache != nil &&
		cache.City != "" &&
		cache.Country != "" &&
		cache.CountryCode != "" &&
		cache.CountryCode != "XX" &&
		cache.Latitude != 0.0 &&
		cache.Longitude != 0.0
}

// Close closes the repository (satisfies Repository interface)
func (r *GeolocationRepository) Close() error {
	// SQLite connection is managed by the main DB instance
	return nil
}