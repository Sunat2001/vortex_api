package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/voronka/backend/internal/shared/config"
	"go.uber.org/zap"
)

// NewRedisClient creates a new Redis client with the provided configuration
func NewRedisClient(ctx context.Context, cfg *config.RedisConfig, logger *zap.Logger) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.GetRedisAddr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	logger.Info("connecting to Redis",
		zap.String("addr", cfg.GetRedisAddr()),
		zap.Int("db", cfg.DB),
		zap.Int("pool_size", cfg.PoolSize),
	)

	// Ping to verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	logger.Info("successfully connected to Redis")

	return client, nil
}

// Close gracefully closes the Redis client
func Close(client *redis.Client, logger *zap.Logger) error {
	if client != nil {
		if err := client.Close(); err != nil {
			logger.Error("failed to close Redis client", zap.Error(err))
			return err
		}
		logger.Info("Redis client closed")
	}
	return nil
}
