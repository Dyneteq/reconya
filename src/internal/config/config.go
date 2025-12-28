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
	envPaths := []string{
		".env",
		"src/.env",
		"../src/.env",
		"../.env",
	}
	for _, path := range envPaths {
		if _, err := os.Stat(path); err == nil {
			_ = godotenv.Load(path)
			break
		}
	}

	databaseName := os.Getenv("DATABASE_NAME")
	if databaseName == "" {
		databaseName = "reconya"
	}

	username := os.Getenv("LOGIN_USERNAME")
	password := os.Getenv("LOGIN_PASSWORD")

	jwtSecret := os.Getenv("JWT_SECRET_KEY")
	if jwtSecret == "" {
		jwtSecret = "agent-mode-default-secret"
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
		dataPaths := []string{
			"data",
			"src/data",
		}
		dataDir := "data"
		for _, path := range dataPaths {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				dataDir = path
				break
			}
		}
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			os.MkdirAll(dataDir, 0755)
		}
		sqlitePath = filepath.Join(dataDir, fmt.Sprintf("%s.db", databaseName))
	}
	config.SQLitePath = sqlitePath

	return config, nil
}
