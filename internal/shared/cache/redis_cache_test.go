package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// setupTestCache creates a test cache with miniredis
func setupTestCache(t *testing.T) (Cache, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := zap.NewNop()
	cache := NewRedisCache(client, logger)

	return cache, mr
}

func TestRedisCache_SetAndGet_String(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()
	key := "test:string"
	value := "hello world"

	// Set
	err := cache.Set(ctx, key, value, 1*time.Minute)
	require.NoError(t, err)

	// Get
	var result string
	err = cache.Get(ctx, key, &result)
	require.NoError(t, err)
	assert.Equal(t, value, result)
}

func TestRedisCache_SetAndGet_Struct(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	type User struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	key := "test:user:123"
	user := User{
		ID:   "123",
		Name: "John Doe",
		Age:  30,
	}

	// Set
	err := cache.Set(ctx, key, user, 1*time.Minute)
	require.NoError(t, err)

	// Get
	var result User
	err = cache.Get(ctx, key, &result)
	require.NoError(t, err)
	assert.Equal(t, user, result)
}

func TestRedisCache_Get_CacheMiss(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()
	key := "nonexistent"

	var result string
	err := cache.Get(ctx, key, &result)
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestRedisCache_Get_InvalidKey(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	var result string
	err := cache.Get(ctx, "", &result)
	assert.ErrorIs(t, err, ErrInvalidKey)
}

func TestRedisCache_Set_InvalidKey(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	err := cache.Set(ctx, "", "value", 1*time.Minute)
	assert.ErrorIs(t, err, ErrInvalidKey)
}

func TestRedisCache_Set_InvalidValue(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	err := cache.Set(ctx, "key", nil, 1*time.Minute)
	assert.ErrorIs(t, err, ErrInvalidValue)
}

func TestRedisCache_Delete(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()
	key := "test:delete"

	// Set a value
	err := cache.Set(ctx, key, "value", 1*time.Minute)
	require.NoError(t, err)

	// Verify it exists
	exists, err := cache.Exists(ctx, key)
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete
	deleted, err := cache.Delete(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	// Verify it's gone
	exists, err = cache.Exists(ctx, key)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestRedisCache_Delete_Multiple(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	// Set multiple values
	keys := []string{"test:1", "test:2", "test:3"}
	for _, key := range keys {
		err := cache.Set(ctx, key, "value", 1*time.Minute)
		require.NoError(t, err)
	}

	// Delete all
	deleted, err := cache.Delete(ctx, keys...)
	require.NoError(t, err)
	assert.Equal(t, int64(3), deleted)
}

func TestRedisCache_Exists(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()
	key := "test:exists"

	// Check non-existent key
	exists, err := cache.Exists(ctx, key)
	require.NoError(t, err)
	assert.False(t, exists)

	// Set value
	err = cache.Set(ctx, key, "value", 1*time.Minute)
	require.NoError(t, err)

	// Check existing key
	exists, err = cache.Exists(ctx, key)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRedisCache_MGet(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	// Set multiple values
	items := map[string]string{
		"test:1": "value1",
		"test:2": "value2",
		"test:3": "value3",
	}

	for key, value := range items {
		err := cache.Set(ctx, key, value, 1*time.Minute)
		require.NoError(t, err)
	}

	// Get multiple
	keys := []string{"test:1", "test:2", "test:3", "test:nonexistent"}
	result, err := cache.MGet(ctx, keys)
	require.NoError(t, err)

	// Should have 3 values (test:nonexistent is missing)
	assert.Len(t, result, 3)
	assert.Contains(t, result, "test:1")
	assert.Contains(t, result, "test:2")
	assert.Contains(t, result, "test:3")
	assert.NotContains(t, result, "test:nonexistent")
}

func TestRedisCache_MSet(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	items := map[string]interface{}{
		"test:1": "value1",
		"test:2": "value2",
		"test:3": "value3",
	}

	// Set multiple
	err := cache.MSet(ctx, items, 1*time.Minute)
	require.NoError(t, err)

	// Verify all exist
	for key, expectedValue := range items {
		var result string
		err := cache.Get(ctx, key, &result)
		require.NoError(t, err)
		assert.Equal(t, expectedValue, result)
	}
}

func TestRedisCache_DeletePattern(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	// Set multiple values with pattern
	keys := []string{
		"user:123",
		"user:456",
		"user:789",
		"session:abc",
	}

	for _, key := range keys {
		err := cache.Set(ctx, key, "value", 1*time.Minute)
		require.NoError(t, err)
	}

	// Delete all user: keys
	deleted, err := cache.DeletePattern(ctx, "user:*")
	require.NoError(t, err)
	assert.Equal(t, int64(3), deleted)

	// Verify user keys are gone
	exists, err := cache.Exists(ctx, "user:123")
	require.NoError(t, err)
	assert.False(t, exists)

	// Verify session key still exists
	exists, err = cache.Exists(ctx, "session:abc")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRedisCache_Keys(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()

	// Set multiple values
	expectedKeys := []string{"test:1", "test:2", "test:3"}
	for _, key := range expectedKeys {
		err := cache.Set(ctx, key, "value", 1*time.Minute)
		require.NoError(t, err)
	}

	// Get keys by pattern
	keys, err := cache.Keys(ctx, "test:*")
	require.NoError(t, err)
	assert.ElementsMatch(t, expectedKeys, keys)
}

func TestRedisCache_TTL(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()
	key := "test:ttl"

	// Set with TTL
	err := cache.Set(ctx, key, "value", 1*time.Minute)
	require.NoError(t, err)

	// Check TTL
	ttl, err := cache.TTL(ctx, key)
	require.NoError(t, err)
	assert.Greater(t, ttl.Seconds(), 0.0)
	assert.LessOrEqual(t, ttl.Seconds(), 60.0)
}

func TestRedisCache_Expire(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()
	key := "test:expire"

	// Set without expiration
	err := cache.Set(ctx, key, "value", 0)
	require.NoError(t, err)

	// Set expiration
	err = cache.Expire(ctx, key, 30*time.Second)
	require.NoError(t, err)

	// Verify TTL is set
	ttl, err := cache.TTL(ctx, key)
	require.NoError(t, err)
	assert.Greater(t, ttl.Seconds(), 0.0)
}

func TestRedisCache_Increment(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()
	key := "test:counter"

	// Increment non-existent key
	value, err := cache.Increment(ctx, key, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), value)

	// Increment again
	value, err = cache.Increment(ctx, key, 5)
	require.NoError(t, err)
	assert.Equal(t, int64(6), value)

	// Increment with negative (decrement)
	value, err = cache.Increment(ctx, key, -2)
	require.NoError(t, err)
	assert.Equal(t, int64(4), value)
}

func TestRedisCache_Decrement(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()
	key := "test:counter"

	// Decrement non-existent key
	value, err := cache.Decrement(ctx, key, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(-1), value)

	// Decrement again
	value, err = cache.Decrement(ctx, key, 5)
	require.NoError(t, err)
	assert.Equal(t, int64(-6), value)
}

func TestRedisCache_TTL_Expiration(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()
	key := "test:expiring"

	// Set with very short TTL
	err := cache.Set(ctx, key, "value", 100*time.Millisecond)
	require.NoError(t, err)

	// Fast-forward time in miniredis
	mr.FastForward(200 * time.Millisecond)

	// Key should be expired
	var result string
	err = cache.Get(ctx, key, &result)
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestRedisCache_ByteSlice(t *testing.T) {
	cache, mr := setupTestCache(t)
	defer mr.Close()

	ctx := context.Background()
	key := "test:bytes"
	value := []byte("binary data")

	// Set byte slice
	err := cache.Set(ctx, key, value, 1*time.Minute)
	require.NoError(t, err)

	// Get as string
	var result string
	err = cache.Get(ctx, key, &result)
	require.NoError(t, err)
	assert.Equal(t, string(value), result)
}