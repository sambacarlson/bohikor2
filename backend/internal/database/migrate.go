package database

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(dbURL, migrationsDir string) error {
	absPath, err := filepath.Abs(migrationsDir)
	if err != nil {
		return fmt.Errorf("resolve migrations path: %w", err)
	}

	m, err := migrate.New("file://"+absPath, dbURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			slog.Error("close migrator source", "error", srcErr)
		}
		if dbErr != nil {
			slog.Error("close migrator database", "error", dbErr)
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	slog.Info("database migrations completed")
	return nil
}
