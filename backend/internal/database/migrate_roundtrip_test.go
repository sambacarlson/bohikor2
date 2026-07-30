package database

import (
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// TestMigrationRoundtrip validates 000001 against a real Postgres: up, down, up.
// Skipped unless MIGRATION_TEST_DB_URL is set (opt-in; needs a throwaway DB).
func TestMigrationRoundtrip(t *testing.T) {
	url := os.Getenv("MIGRATION_TEST_DB_URL")
	if url == "" {
		t.Skip("set MIGRATION_TEST_DB_URL to run the migration round-trip")
	}

	m, err := migrate.New("file://../../migrations", url)
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := m.Down(); err != nil {
		t.Fatalf("down: %v", err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("second up: %v", err)
	}
}
