package database

import (
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations applies all pending database migrations
func (db *Database) RunMigrations() error {
	// Create source from embedded files
	d, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Build connection URL
	connURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		db.config.User,
		db.config.Password,
		db.config.Host,
		db.config.Port,
		db.config.Database,
		db.config.SSLMode,
	)

	// Create migrator
	m, err := migrate.NewWithSourceInstance("iofs", d, connURL)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Get current version
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	if dirty {
		return fmt.Errorf("database is in dirty state at version %d", version)
	}

	return nil
}
