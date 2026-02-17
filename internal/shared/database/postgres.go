package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/voronka/backend/internal/shared/config"
	"go.uber.org/zap"
)

// NewPostgresPool creates a new PostgreSQL connection pool using pgx/v5
func NewPostgresPool(ctx context.Context, cfg *config.DatabaseConfig, logger *zap.Logger) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.GetDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Connection pool settings
	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = cfg.ConnMaxIdleTime
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	// Custom connection timeout
	poolConfig.ConnConfig.ConnectTimeout = 10 * time.Second

	logger.Info("connecting to PostgreSQL",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Database),
		zap.Int("max_conns", cfg.MaxOpenConns),
	)

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Ping to verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("successfully connected to PostgreSQL")

	return pool, nil
}

// Close gracefully closes the connection pool
func Close(pool *pgxpool.Pool, logger *zap.Logger) {
	if pool != nil {
		pool.Close()
		logger.Info("PostgreSQL connection pool closed")
	}
}