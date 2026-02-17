package migration

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

// Migrator wraps golang-migrate with Vortex patterns
type Migrator struct {
	migrate *migrate.Migrate
	logger  *zap.Logger
}

// NewMigrator creates a new database migrator
func NewMigrator(pool *pgxpool.Pool, migrationsPath string, logger *zap.Logger) (*Migrator, error) {
	// Convert pgxpool to sql.DB for migrate driver
	db := stdlib.OpenDBFromPool(pool)

	// Create pgx driver instance
	driver, err := pgx.WithInstance(db, &pgx.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate driver: %w", err)
	}

	sourceURL := fmt.Sprintf("file://%s", migrationsPath)

	m, err := migrate.NewWithDatabaseInstance(sourceURL, "pgx5", driver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator: %w", err)
	}

	return &Migrator{
		migrate: m,
		logger:  logger,
	}, nil
}

// Up runs all pending migrations
func (m *Migrator) Up(ctx context.Context) error {
	m.logger.Info("running migrations UP")

	if err := m.migrate.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			m.logger.Info("no new migrations to apply")
			return nil
		}
		return fmt.Errorf("migration up failed: %w", err)
	}

	m.logger.Info("migrations completed successfully")
	return nil
}

// Down rolls back the last migration
func (m *Migrator) Down(ctx context.Context) error {
	m.logger.Info("running migration DOWN")

	if err := m.migrate.Steps(-1); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			m.logger.Info("no migrations to rollback")
			return nil
		}
		return fmt.Errorf("migration down failed: %w", err)
	}

	m.logger.Info("migration rolled back successfully")
	return nil
}

// Steps runs n migrations (positive for up, negative for down)
func (m *Migrator) Steps(ctx context.Context, n int) error {
	m.logger.Info("running migration steps", zap.Int("steps", n))

	if err := m.migrate.Steps(n); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			m.logger.Info("no migrations to apply")
			return nil
		}
		return fmt.Errorf("migration steps failed: %w", err)
	}

	return nil
}

// Version returns the current migration version
func (m *Migrator) Version() (uint, bool, error) {
	version, dirty, err := m.migrate.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("failed to get version: %w", err)
	}
	return version, dirty, nil
}

// Force sets the migration version (use with extreme caution)
func (m *Migrator) Force(version int) error {
	m.logger.Warn("forcing migration version", zap.Int("version", version))

	if err := m.migrate.Force(version); err != nil {
		return fmt.Errorf("force version failed: %w", err)
	}

	return nil
}

// Close releases migrator resources
func (m *Migrator) Close() error {
	srcErr, dbErr := m.migrate.Close()
	if srcErr != nil {
		return fmt.Errorf("source close error: %w", srcErr)
	}
	if dbErr != nil {
		return fmt.Errorf("database close error: %w", dbErr)
	}
	return nil
}