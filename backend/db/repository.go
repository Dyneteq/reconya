package db

import (
	"context"
	"database/sql"
	"errors"
	"reconya/models"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("record not found")
)

// Repository defines a common interface for all repositories
type Repository interface {
	Close() error
}

// NetworkRepository defines the interface for network operations
type NetworkRepository interface {
	Repository
	FindByID(ctx context.Context, id string) (*models.Network, error)
	FindByCIDR(ctx context.Context, cidr string) (*models.Network, error)
	FindAll(ctx context.Context) ([]*models.Network, error)
	CreateOrUpdate(ctx context.Context, network *models.Network) (*models.Network, error)
	Delete(ctx context.Context, id string) error
	GetDeviceCount(ctx context.Context, networkID string) (int, error)
	SetRangeActive(ctx context.Context, rangeID string, active bool) error
	UpdateRangeLastScanned(ctx context.Context, rangeID string, scannedAt time.Time) error
}

// DeviceStatusChange records one device whose status the status sweeper moved,
// so callers can log the transition and evaluate alerts against it.
type DeviceStatusChange struct {
	ID     string
	IPv4   string
	Status models.DeviceStatus
}

// DeviceRepository defines the interface for device operations
type DeviceRepository interface {
	Repository
	FindByID(ctx context.Context, id string) (*models.Device, error)
	FindByIP(ctx context.Context, ip string) (*models.Device, error)
	FindAll(ctx context.Context) ([]*models.Device, error)
	CreateOrUpdate(ctx context.Context, device *models.Device) (*models.Device, error)
	UpdateDeviceStatuses(ctx context.Context, timeout time.Duration) ([]DeviceStatusChange, error)
	DeleteByID(ctx context.Context, id string) error
}

// EventLogRepository defines the interface for event log operations
type EventLogRepository interface {
	Repository
	Create(ctx context.Context, eventLog *models.EventLog) error
	FindLatest(ctx context.Context, limit int) ([]*models.EventLog, error)
	FindAllByDeviceID(ctx context.Context, deviceID string) ([]*models.EventLog, error)
	CountCompletedByType(ctx context.Context, eventType models.EEventLogType) (int, error)
}

// SystemStatusRepository defines the interface for system status operations
type SystemStatusRepository interface {
	Repository
	Create(ctx context.Context, status *models.SystemStatus) error
	FindLatest(ctx context.Context) (*models.SystemStatus, error)
}

// GeolocationRepositoryInterface defines the interface for geolocation cache operations
type GeolocationRepositoryInterface interface {
	Repository
	FindByIP(ctx context.Context, ip string) (*models.GeolocationCache, error)
	Create(ctx context.Context, cache *models.GeolocationCache) error
	Update(ctx context.Context, cache *models.GeolocationCache) error
	Upsert(ctx context.Context, cache *models.GeolocationCache) error
	CleanupExpired(ctx context.Context) error
	IsValidCache(cache *models.GeolocationCache) bool
}

// SettingsRepository defines the interface for settings operations
type SettingsRepository interface {
	Repository
	FindByUserID(userID string) (*models.Settings, error)
	Create(settings *models.Settings) error
	Update(settings *models.Settings) error
}

// AlertRepository defines the interface for alert operations
type AlertRepository interface {
	Repository
	Upsert(ctx context.Context, alert *models.Alert) error
	FindByID(ctx context.Context, id string) (*models.Alert, error)
	FindAll(ctx context.Context, query models.AlertQuery) ([]*models.Alert, error)
	SetState(ctx context.Context, id string, state models.AlertState) error
	AckAll(ctx context.Context) (int, error)
	ResolveMissing(ctx context.Context, ruleID string, liveKeys []string) (int, error)
	CountsBySeverity(ctx context.Context, states []models.AlertState) (models.AlertCounts, error)
	DeleteByDeviceID(ctx context.Context, deviceID string) error
}

// RepositoryFactory creates repositories
type RepositoryFactory struct {
	SQLiteDB *sql.DB
	DBName   string
}

// NewRepositoryFactory creates a new repository factory
func NewRepositoryFactory(sqliteDB *sql.DB, dbName string) *RepositoryFactory {
	return &RepositoryFactory{
		SQLiteDB: sqliteDB,
		DBName:   dbName,
	}
}

// NewNetworkRepository creates a new network repository
func (f *RepositoryFactory) NewNetworkRepository() NetworkRepository {
	return NewSQLiteNetworkRepository(f.SQLiteDB)
}

// NewDeviceRepository creates a new device repository
func (f *RepositoryFactory) NewDeviceRepository() DeviceRepository {
	return NewSQLiteDeviceRepository(f.SQLiteDB)
}

// NewEventLogRepository creates a new event log repository
func (f *RepositoryFactory) NewEventLogRepository() EventLogRepository {
	return NewSQLiteEventLogRepository(f.SQLiteDB)
}

// NewSystemStatusRepository creates a new system status repository
func (f *RepositoryFactory) NewSystemStatusRepository() SystemStatusRepository {
	return NewSQLiteSystemStatusRepository(f.SQLiteDB)
}

// NewGeolocationRepository creates a new geolocation repository
func (f *RepositoryFactory) NewGeolocationRepository() *GeolocationRepository {
	return NewGeolocationRepository(f.SQLiteDB)
}

// NewSettingsRepository creates a new settings repository
func (f *RepositoryFactory) NewSettingsRepository() SettingsRepository {
	return NewSQLiteSettingsRepository(f.SQLiteDB)
}

// NewAlertRepository creates a new alert repository
func (f *RepositoryFactory) NewAlertRepository() AlertRepository {
	return NewSQLiteAlertRepository(f.SQLiteDB)
}

// GenerateID generates a unique ID for a record
func GenerateID() string {
	return uuid.New().String()
}
