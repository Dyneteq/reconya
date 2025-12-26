package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type DatabaseType string

const (
	SQLite DatabaseType = "sqlite"
)

type Config struct {
	JwtKey       []byte
	Port         string
	DatabaseType DatabaseType
	SQLitePath   string
	Username     string
	Password     string
	DatabaseName string
}

func LoadConfig() (*Config, error) {
	// Try loading .env from multiple locations
	envPaths := []string{
		".env",           // Current directory
		"src/.env",       // From project root
		"../src/.env",    // From cmd directory
		"../.env",        // Parent directory
	}
	for _, path := range envPaths {
		if _, err := os.Stat(path); err == nil {
			_ = godotenv.Load(path)
			break
		}
	}

	databaseName := os.Getenv("DATABASE_NAME")
	if databaseName == "" {
		return nil, fmt.Errorf("DATABASE_NAME environment variable is not set")
	}

	username := os.Getenv("LOGIN_USERNAME")
	password := os.Getenv("LOGIN_PASSWORD")
	if username == "" || password == "" {
		return nil, fmt.Errorf("LOGIN_USERNAME or LOGIN_PASSWORD environment variables are not set")
	}

	jwtSecret := os.Getenv("JWT_SECRET_KEY")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET_KEY environment variable is not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3008"
	}

	dbType := string(SQLite)

	config := &Config{
		JwtKey:       []byte(jwtSecret),
		Port:         port,
		DatabaseType: DatabaseType(dbType),
		Username:     username,
		Password:     password,
		DatabaseName: databaseName,
	}

	sqlitePath := os.Getenv("SQLITE_PATH")
	if sqlitePath == "" {
		// Try to find the data directory in multiple locations
		dataPaths := []string{
			"data",     // Current directory (running from src)
			"src/data", // From project root
		}
		dataDir := "data" // Default
		for _, path := range dataPaths {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				dataDir = path
				break
			}
		}
		// Create data directory if it doesn't exist
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			os.MkdirAll(dataDir, 0755)
		}
		sqlitePath = filepath.Join(dataDir, fmt.Sprintf("%s.db", databaseName))
	}
	config.SQLitePath = sqlitePath

	return config, nil
}
