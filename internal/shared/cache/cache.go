package cache

import (
	"context"
	"time"
)

// Cache defines the interface for cache operations
// Implementations should be safe for concurrent use
type Cache interface {
	// Get retrieves a value from cache and unmarshals it into dest
	// Returns ErrCacheMiss if key doesn't exist
	// dest should be a pointer to the target type
	Get(ctx context.Context, key string, dest interface{}) error

	// Set stores a value in cache with the specified TTL
	// TTL of 0 means no expiration (use with caution)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Delete removes one or more keys from cache
	// Returns the number of keys deleted
	Delete(ctx context.Context, keys ...string) (int64, error)

	// Exists checks if a key exists in cache
	Exists(ctx context.Context, key string) (bool, error)

	// MGet retrieves multiple values at once
	// Returns a map of key->value for keys that exist
	// Missing keys are not included in the result
	MGet(ctx context.Context, keys []string) (map[string]interface{}, error)

	// MSet stores multiple key-value pairs with the same TTL
	// All operations are executed in a pipeline for efficiency
	MSet(ctx context.Context, items map[string]interface{}, ttl time.Duration) error

	// DeletePattern deletes all keys matching the pattern
	// Pattern uses Redis glob-style patterns (*, ?, [])
	// Returns the number of keys deleted
	// WARNING: Use with caution on large keyspaces
	DeletePattern(ctx context.Context, pattern string) (int64, error)

	// Keys returns all keys matching the pattern
	// Pattern uses Redis glob-style patterns (*, ?, [])
	// WARNING: Use with caution on large keyspaces, prefer SCAN in production
	Keys(ctx context.Context, pattern string) ([]string, error)

	// TTL returns the remaining time to live for a key
	// Returns -1 if key exists but has no expiration
	// Returns -2 if key doesn't exist
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Expire sets a timeout on a key
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// Increment atomically increments a key's value by delta
	// If key doesn't exist, it's set to delta
	// Returns the new value after increment
	Increment(ctx context.Context, key string, delta int64) (int64, error)

	// Decrement atomically decrements a key's value by delta
	// If key doesn't exist, it's set to -delta
	// Returns the new value after decrement
	Decrement(ctx context.Context, key string, delta int64) (int64, error)
}