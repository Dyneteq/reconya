package testutils

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"reconya/db"
	"reconya/internal/config"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func SetupTestDatabase(t *testing.T) (*sql.DB, func()) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	testDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_timeout=10000&_foreign_keys=on")
	require.NoError(t, err)
	
	err = db.InitializeSchema(testDB)
	require.NoError(t, err)
	
	cleanup := func() {
		testDB.Close()
		os.RemoveAll(tempDir)
	}
	
	return testDB, cleanup
}

func SetupTestRepositoryFactory(t *testing.T) (*db.RepositoryFactory, func()) {
	testDB, cleanup := SetupTestDatabase(t)
	factory := db.NewRepositoryFactory(testDB, "reconya_test")
	return factory, cleanup
}

func GetTestConfig() *config.Config {
	return &config.Config{
		DatabaseType: config.SQLite,
		SQLitePath:   ":memory:",
		DatabaseName: "reconya_test",
		NetworkCIDR:  "192.168.1.0/24",
		JwtKey:       []byte("test_jwt_secret_key_for_testing_only"),
		Username:     "test_admin",
		Password:     "test_password",
	}
}