package app

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/voronka/backend/internal/shared/config"
	"github.com/voronka/backend/internal/shared/database"
	"github.com/voronka/backend/internal/shared/redis"
)

// Infrastructure holds all infrastructure dependencies.
// This struct centralizes all common infrastructure components
// used by both API and workers, eliminating bootstrap duplication.
type Infrastructure struct {
	Config        *config.Config
	Logger        *zap.Logger
	PgPool        *pgxpool.Pool
	RedisClient   *goredis.Client
	StreamManager *redis.StreamManager
}

// BootstrapInfrastructure initializes all infrastructure components.
// This is the single entry point for setting up the application infrastructure,
// ensuring consistency between API and workers.
func BootstrapInfrastructure(ctx context.Context) (*Infrastructure, error) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger
	logger, err := NewLogger(&cfg.App, &cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Initialize database
	pgPool, err := database.NewPostgresPool(ctx, &cfg.Database, logger)
	if err != nil {
		logger.Error("failed to connect to PostgreSQL", zap.Error(err))
		_ = logger.Sync()
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Initialize Redis
	redisClient, err := redis.NewRedisClient(ctx, &cfg.Redis, logger)
	if err != nil {
		logger.Error("failed to connect to Redis", zap.Error(err))
		database.Close(pgPool, logger)
		_ = logger.Sync()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Initialize Redis Stream Manager
	streamManager := redis.NewStreamManager(redisClient, logger, cfg.Redis.StreamMaxLen)

	return &Infrastructure{
		Config:        cfg,
		Logger:        logger,
		PgPool:        pgPool,
		RedisClient:   redisClient,
		StreamManager: streamManager,
	}, nil
}

// Close releases all infrastructure resources in the correct order.
// Should be called via defer in main() after successful bootstrap.
func (i *Infrastructure) Close() {
	if i.RedisClient != nil {
		redis.Close(i.RedisClient, i.Logger)
	}
	if i.PgPool != nil {
		database.Close(i.PgPool, i.Logger)
	}
	if i.Logger != nil {
		_ = i.Logger.Sync()
	}
}