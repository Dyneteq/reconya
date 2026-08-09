package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
)

type DatabaseType string

const (
	SQLite DatabaseType = "sqlite"
)

type Config struct {
	Port         string
	DatabaseType DatabaseType
	SQLitePath   string
	Username     string
	Password     string
	DatabaseName string

	// Every one of these defaults to false. reconYa's positioning is that
	// scan data never leaves the network it runs on; each of these flags
	// enables one outbound call to a third party and has to be turned on
	// explicitly rather than shipping on by default.
	PublicIPLookupEnabled bool // nic_identifier_service.go -> api.ipify.org
	GeolocationEnabled    bool // system_status_service.go  -> ip-api.com (plain HTTP)
	VendorLookupEnabled   bool // native_scanner.go         -> api.macvendors.com, per discovered MAC
	OUIDownloadEnabled    bool // oui_service.go             -> standards-oui.ieee.org (plain HTTP)
}

// envBool reads a boolean env var, defaulting to false so that an unset,
// empty, or unparsable value never silently enables an outbound call.
func envBool(name string) bool {
	val, err := strconv.ParseBool(os.Getenv(name))
	if err != nil {
		return false
	}
	return val
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load()

	databaseName := os.Getenv("DATABASE_NAME")
	if databaseName == "" {
		return nil, fmt.Errorf("DATABASE_NAME environment variable is not set")
	}

	username := os.Getenv("LOGIN_USERNAME")
	password := os.Getenv("LOGIN_PASSWORD")
	if username == "" || password == "" {
		return nil, fmt.Errorf("LOGIN_USERNAME or LOGIN_PASSWORD environment variables are not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3008"
	}

	dbType := string(SQLite)

	config := &Config{
		Port:         port,
		DatabaseType: DatabaseType(dbType),
		Username:     username,
		Password:     password,
		DatabaseName: databaseName,

		PublicIPLookupEnabled: envBool("PUBLIC_IP_LOOKUP_ENABLED"),
		GeolocationEnabled:    envBool("GEOLOCATION_ENABLED"),
		VendorLookupEnabled:   envBool("VENDOR_LOOKUP_ONLINE_ENABLED"),
		OUIDownloadEnabled:    envBool("OUI_DOWNLOAD_ENABLED"),
	}

	sqlitePath := os.Getenv("SQLITE_PATH")
	if sqlitePath == "" {
		sqlitePath = filepath.Join("data", fmt.Sprintf("%s.db", databaseName))
	}
	config.SQLitePath = sqlitePath

	return config, nil
}
