package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// redisCache implements Cache interface using Redis
type redisCache struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisCache creates a new Redis-backed cache
func NewRedisCache(client *redis.Client, logger *zap.Logger) Cache {
	return &redisCache{
		client: client,
		logger: logger,
	}
}

// Get retrieves a value from cache
func (c *redisCache) Get(ctx context.Context, key string, dest interface{}) error {
	if key == "" {
		return ErrInvalidKey
	}

	if dest == nil {
		return ErrInvalidValue
	}

	result, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.logger.Debug("cache miss", zap.String("key", key))
			return ErrCacheMiss
		}
		return fmt.Errorf("failed to get cache key: %w", err)
	}

	// Try to unmarshal as JSON
	if err := json.Unmarshal([]byte(result), dest); err != nil {
		// If unmarshal fails, try to assign directly (for string types)
		if strDest, ok := dest.(*string); ok {
			*strDest = result
			c.logger.Debug("cache hit", zap.String("key", key))
			return nil
		}
		return fmt.Errorf("%w: %v", ErrSerialization, err)
	}

	c.logger.Debug("cache hit", zap.String("key", key))
	return nil
}

// Set stores a value in cache
func (c *redisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if key == "" {
		return ErrInvalidKey
	}

	if value == nil {
		return ErrInvalidValue
	}

	var data []byte
	var err error

	// Handle string values directly
	if strValue, ok := value.(string); ok {
		data = []byte(strValue)
	} else if byteValue, ok := value.([]byte); ok {
		data = byteValue
	} else {
		// Marshal complex types to JSON
		data, err = json.Marshal(value)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrSerialization, err)
		}
	}

	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set cache key: %w", err)
	}

	c.logger.Debug("cache set",
		zap.String("key", key),
		zap.Duration("ttl", ttl),
	)

	return nil
}

// Delete removes keys from cache
func (c *redisCache) Delete(ctx context.Context, keys ...string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}

	deleted, err := c.client.Del(ctx, keys...).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to delete cache keys: %w", err)
	}

	c.logger.Debug("cache delete",
		zap.Strings("keys", keys),
		zap.Int64("deleted", deleted),
	)

	return deleted, nil
}

// Exists checks if a key exists
func (c *redisCache) Exists(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, ErrInvalidKey
	}

	count, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check cache key existence: %w", err)
	}

	exists := count > 0
	c.logger.Debug("cache exists check",
		zap.String("key", key),
		zap.Bool("exists", exists),
	)

	return exists, nil
}

// MGet retrieves multiple values at once
func (c *redisCache) MGet(ctx context.Context, keys []string) (map[string]interface{}, error) {
	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}

	values, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get multiple cache keys: %w", err)
	}

	result := make(map[string]interface{}, len(keys))
	for i, key := range keys {
		if values[i] != nil {
			result[key] = values[i]
		}
	}

	c.logger.Debug("cache mget",
		zap.Int("requested", len(keys)),
		zap.Int("found", len(result)),
	)

	return result, nil
}

// MSet stores multiple key-value pairs
func (c *redisCache) MSet(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}

	// Use pipeline for efficiency
	pipe := c.client.Pipeline()

	for key, value := range items {
		var data []byte
		var err error

		if strValue, ok := value.(string); ok {
			data = []byte(strValue)
		} else if byteValue, ok := value.([]byte); ok {
			data = byteValue
		} else {
			data, err = json.Marshal(value)
			if err != nil {
				return fmt.Errorf("%w: failed to marshal value for key %s: %v", ErrSerialization, key, err)
			}
		}

		pipe.Set(ctx, key, data, ttl)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to set multiple cache keys: %w", err)
	}

	c.logger.Debug("cache mset",
		zap.Int("count", len(items)),
		zap.Duration("ttl", ttl),
	)

	return nil
}

// DeletePattern deletes all keys matching the pattern
func (c *redisCache) DeletePattern(ctx context.Context, pattern string) (int64, error) {
	if pattern == "" {
		return 0, ErrInvalidKey
	}

	keys, err := c.client.Keys(ctx, pattern).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to find keys by pattern: %w", err)
	}

	if len(keys) == 0 {
		c.logger.Debug("cache delete pattern: no keys found", zap.String("pattern", pattern))
		return 0, nil
	}

	deleted, err := c.client.Del(ctx, keys...).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to delete keys by pattern: %w", err)
	}

	c.logger.Debug("cache delete pattern",
		zap.String("pattern", pattern),
		zap.Int64("deleted", deleted),
	)

	return deleted, nil
}

// Keys returns all keys matching the pattern
func (c *redisCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	if pattern == "" {
		return nil, ErrInvalidKey
	}

	keys, err := c.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get keys by pattern: %w", err)
	}

	c.logger.Debug("cache keys",
		zap.String("pattern", pattern),
		zap.Int("count", len(keys)),
	)

	return keys, nil
}

// TTL returns the remaining time to live for a key
func (c *redisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	if key == "" {
		return 0, ErrInvalidKey
	}

	ttl, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get key TTL: %w", err)
	}

	c.logger.Debug("cache ttl",
		zap.String("key", key),
		zap.Duration("ttl", ttl),
	)

	return ttl, nil
}

// Expire sets a timeout on a key
func (c *redisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if key == "" {
		return ErrInvalidKey
	}

	if err := c.client.Expire(ctx, key, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set key expiration: %w", err)
	}

	c.logger.Debug("cache expire",
		zap.String("key", key),
		zap.Duration("ttl", ttl),
	)

	return nil
}

// Increment atomically increments a key's value
func (c *redisCache) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	if key == "" {
		return 0, ErrInvalidKey
	}

	newValue, err := c.client.IncrBy(ctx, key, delta).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key: %w", err)
	}

	c.logger.Debug("cache increment",
		zap.String("key", key),
		zap.Int64("delta", delta),
		zap.Int64("new_value", newValue),
	)

	return newValue, nil
}

// Decrement atomically decrements a key's value
func (c *redisCache) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	if key == "" {
		return 0, ErrInvalidKey
	}

	newValue, err := c.client.DecrBy(ctx, key, delta).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to decrement key: %w", err)
	}

	c.logger.Debug("cache decrement",
		zap.String("key", key),
		zap.Int64("delta", delta),
		zap.Int64("new_value", newValue),
	)

	return newValue, nil
}