package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ConnectToSQLite initializes and returns a SQLite connection
func ConnectToSQLite(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory for SQLite: %w", err)
	}
	dsn := fmt.Sprintf("%s?_journal=WAL&_timeout=30000&_busy_timeout=30000", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=30000",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=10000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA temp_store=MEMORY",
		"PRAGMA mmap_size=268435456",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("failed to set %s: %w", pragma, err)
		}
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping SQLite database: %w", err)
	}

	log.Println("Connected to SQLite database with optimized settings for concurrency")
	return db, nil
}

func InitializeSchema(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS networks (
		id TEXT PRIMARY KEY,
		cidr TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("failed to create networks table: %w", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS devices (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		ipv4 TEXT NOT NULL,
		mac TEXT,
		vendor TEXT,
		status TEXT NOT NULL,
		network_id TEXT,
		hostname TEXT,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		last_seen_online_at TIMESTAMP,
		port_scan_started_at TIMESTAMP,
		port_scan_ended_at TIMESTAMP,
		FOREIGN KEY (network_id) REFERENCES networks(id)
	)`)
	if err != nil {
		return fmt.Errorf("failed to create devices table: %w", err)
	}

	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_ipv4 ON devices(ipv4)`)
	if err != nil {
		return fmt.Errorf("failed to create unique index on devices.ipv4: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_devices_mac ON devices(mac)`)
	if err != nil {
		return fmt.Errorf("failed to create index on devices.mac: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_devices_network_id ON devices(network_id)`)
	if err != nil {
		return fmt.Errorf("failed to create index on devices.network_id: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_devices_ipv6_link_local ON devices(ipv6_link_local)`)
	if err != nil {
		log.Printf("Note: IPv6 link local index might already exist: %v", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_devices_ipv6_unique_local ON devices(ipv6_unique_local)`)
	if err != nil {
		log.Printf("Note: IPv6 unique local index might already exist: %v", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_devices_ipv6_global ON devices(ipv6_global)`)
	if err != nil {
		log.Printf("Note: IPv6 global index might already exist: %v", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS ports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_id TEXT NOT NULL,
		number TEXT NOT NULL,
		protocol TEXT NOT NULL,
		state TEXT NOT NULL,
		service TEXT NOT NULL,
		FOREIGN KEY (device_id) REFERENCES devices(id)
	)`)
	if err != nil {
		return fmt.Errorf("failed to create ports table: %w", err)
	}

	// Create event_logs table
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS event_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		description TEXT NOT NULL,
		device_id TEXT,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("failed to create event_logs table: %w", err)
	}

	// Create system_status table
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS system_status (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		network_id TEXT,
		public_ip TEXT,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		FOREIGN KEY (network_id) REFERENCES networks(id)
	)`)
	if err != nil {
		return fmt.Errorf("failed to create system_status table: %w", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS local_devices (
		system_status_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		ipv4 TEXT NOT NULL,
		mac TEXT,
		vendor TEXT,
		status TEXT NOT NULL,
		hostname TEXT,
		PRIMARY KEY (system_status_id),
		FOREIGN KEY (system_status_id) REFERENCES system_status(id)
	)`)
	if err != nil {
		return fmt.Errorf("failed to create local_devices table: %w", err)
	}

	_, err = db.Exec(`ALTER TABLE devices ADD COLUMN web_scan_ended_at TIMESTAMP`)
	if err != nil {
		log.Printf("Note: web_scan_ended_at column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE devices ADD COLUMN device_type TEXT`)
	if err != nil {
		log.Printf("Note: device_type column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE devices ADD COLUMN os_name TEXT`)
	if err != nil {
		log.Printf("Note: os_name column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE devices ADD COLUMN os_version TEXT`)
	if err != nil {
		log.Printf("Note: os_version column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE devices ADD COLUMN os_family TEXT`)
	if err != nil {
		log.Printf("Note: os_family column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE devices ADD COLUMN os_confidence INTEGER`)
	if err != nil {
		log.Printf("Note: os_confidence column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE devices ADD COLUMN comment TEXT`)
	if err != nil {
		log.Printf("Note: comment column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE devices ADD COLUMN ipv6_link_local TEXT`)
	if err != nil {
		log.Printf("Note: ipv6_link_local column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE devices ADD COLUMN ipv6_unique_local TEXT`)
	if err != nil {
		log.Printf("Note: ipv6_unique_local column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE devices ADD COLUMN ipv6_global TEXT`)
	if err != nil {
		log.Printf("Note: ipv6_global column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE devices ADD COLUMN ipv6_addresses TEXT`)
	if err != nil {
		log.Printf("Note: ipv6_addresses column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE networks ADD COLUMN name TEXT`)
	if err != nil {
		log.Printf("Note: networks.name column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE networks ADD COLUMN description TEXT`)
	if err != nil {
		log.Printf("Note: networks.description column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE networks ADD COLUMN status TEXT DEFAULT 'active'`)
	if err != nil {
		log.Printf("Note: networks.status column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE networks ADD COLUMN last_scanned_at TIMESTAMP`)
	if err != nil {
		log.Printf("Note: networks.last_scanned_at column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE networks ADD COLUMN device_count INTEGER DEFAULT 0`)
	if err != nil {
		log.Printf("Note: networks.device_count column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE networks ADD COLUMN created_at TIMESTAMP`)
	if err != nil {
		log.Printf("Note: networks.created_at column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE networks ADD COLUMN updated_at TIMESTAMP`)
	if err != nil {
		log.Printf("Note: networks.updated_at column might already exist: %v", err)
	}

	// Add IPv6 support to networks table
	_, err = db.Exec(`ALTER TABLE networks ADD COLUMN ipv6_prefix TEXT`)
	if err != nil {
		log.Printf("Note: networks.ipv6_prefix column might already exist: %v", err)
	}

	_, err = db.Exec(`ALTER TABLE networks ADD COLUMN address_family TEXT DEFAULT 'ipv4'`)
	if err != nil {
		log.Printf("Note: networks.address_family column might already exist: %v", err)
	}

	// Create web_services table
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS web_services (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_id TEXT NOT NULL,
		url TEXT NOT NULL,
		title TEXT,
		server TEXT,
		status_code INTEGER NOT NULL,
		content_type TEXT,
		size INTEGER,
		screenshot TEXT,
		port INTEGER NOT NULL,
		protocol TEXT NOT NULL,
		scanned_at TIMESTAMP NOT NULL,
		FOREIGN KEY (device_id) REFERENCES devices(id)
	)`)
	if err != nil {
		return fmt.Errorf("failed to create web_services table: %w", err)
	}

	// Create index on device_id for web_services
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_web_services_device_id ON web_services(device_id)`)
	if err != nil {
		return fmt.Errorf("failed to create index on web_services.device_id: %w", err)
	}

	// Create geolocation_cache table
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS geolocation_cache (
		id TEXT PRIMARY KEY,
		ip TEXT NOT NULL UNIQUE,
		city TEXT,
		region TEXT,
		country TEXT,
		country_code TEXT,
		latitude REAL,
		longitude REAL,
		timezone TEXT,
		isp TEXT,
		source TEXT NOT NULL DEFAULT 'api',
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		expires_at TIMESTAMP NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("failed to create geolocation_cache table: %w", err)
	}

	// Create index on IP for geolocation cache
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_geolocation_cache_ip ON geolocation_cache(ip)`)
	if err != nil {
		return fmt.Errorf("failed to create index on geolocation_cache.ip: %w", err)
	}

	// Create index on expires_at for cache cleanup
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_geolocation_cache_expires_at ON geolocation_cache(expires_at)`)
	if err != nil {
		return fmt.Errorf("failed to create index on geolocation_cache.expires_at: %w", err)
	}

	// Create settings table
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS settings (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		screenshots_enabled BOOLEAN NOT NULL DEFAULT 1,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		UNIQUE(user_id)
	)`)
	if err != nil {
		return fmt.Errorf("failed to create settings table: %w", err)
	}

	// Create index on user_id for settings
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_settings_user_id ON settings(user_id)`)
	if err != nil {
		return fmt.Errorf("failed to create index on settings.user_id: %w", err)
	}

	log.Println("Database schema initialized successfully")
	return nil
}

// ResetPortScanCooldowns clears all port scan timestamps to allow immediate re-scanning (for development)
func ResetPortScanCooldowns(db *sql.DB) error {
	// Clear port scan timestamps
	_, err := db.Exec(`UPDATE devices SET port_scan_ended_at = NULL, port_scan_started_at = NULL`)
	if err != nil {
		return fmt.Errorf("failed to reset port scan cooldowns: %w", err)
	}

	// Clear web scan timestamps if the column exists
	_, err = db.Exec(`UPDATE devices SET web_scan_ended_at = NULL`)
	if err != nil {
		// Column might not exist yet, so we ignore this error
		log.Printf("Note: web_scan_ended_at column might not exist yet: %v", err)
	}

	log.Println("Port scan cooldowns reset - all devices are now eligible for scanning")
	return nil
}
