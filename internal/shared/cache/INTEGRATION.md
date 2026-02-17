# Cache Integration Guide

This guide shows how to integrate the cache system into the Vortex application.

## Overview

The cache implementation provides a production-ready Redis-backed caching layer that follows Clean Architecture principles. It's designed to be used across all domains for:

- User session caching
- Rate limiting
- API response caching
- Frequently accessed data (users, roles, permissions)
- Temporary data storage

## Installation

The cache is already implemented in `/internal/shared/cache/`. No additional dependencies are needed beyond the existing Redis setup.

## Integration Steps

### 1. Initialize Cache in `cmd/api/main.go`

Add the cache initialization after the Redis client setup:

```go
import (
    // ... existing imports
    "github.com/voronka/backend/internal/shared/cache"
)

func main() {
    // ... existing code

    // Initialize Redis
    redisClient, err := redis.NewRedisClient(ctx, &cfg.Redis, logger)
    if err != nil {
        logger.Fatal("failed to connect to Redis", zap.Error(err))
    }
    defer redis.Close(redisClient, logger)

    // Initialize Cache Manager (NEW)
    cacheManager := cache.NewRedisCache(redisClient, logger)

    // Initialize Redis Stream Manager
    streamManager := redis.NewStreamManager(redisClient, logger, cfg.Redis.StreamMaxLen)

    // ... rest of initialization
}
```

### 2. Inject Cache into Domain Usecases

Pass the cache to usecases that need it:

```go
// Example: Agent usecase with cache
agentUsecase := agent.NewUsecase(agentRepo, cacheManager, logger)
```

### 3. Update Domain Usecase Constructors

Modify the usecase to accept cache:

```go
// internal/agent/usecase.go

type Usecase struct {
    repo   Repository
    cache  cache.Cache  // Add cache
    logger *zap.Logger
}

func NewUsecase(repo Repository, cache cache.Cache, logger *zap.Logger) *Usecase {
    return &Usecase{
        repo:   repo,
        cache:  cache,
        logger: logger,
    }
}
```

## Usage Patterns

### Pattern 1: Cache-Aside (Lazy Loading)

Most common pattern - check cache first, fall back to database:

```go
func (uc *Usecase) GetUserByID(ctx context.Context, userID uuid.UUID) (*User, error) {
    cacheKey := fmt.Sprintf("user:%s", userID.String())

    // Try cache first
    var cachedUser User
    err := uc.cache.Get(ctx, cacheKey, &cachedUser)
    if err == nil {
        uc.logger.Debug("user retrieved from cache", zap.String("user_id", userID.String()))
        return &cachedUser, nil
    }

    // Log cache miss (optional)
    if errors.Is(err, cache.ErrCacheMiss) {
        uc.logger.Debug("cache miss for user", zap.String("user_id", userID.String()))
    }

    // Fetch from database
    user, err := uc.repo.GetUserByID(ctx, userID)
    if err != nil {
        return nil, fmt.Errorf("failed to get user: %w", err)
    }

    // Store in cache (5 minutes TTL)
    // Ignore cache errors - don't fail the request if cache write fails
    _ = uc.cache.Set(ctx, cacheKey, user, 5*time.Minute)

    return user, nil
}
```

### Pattern 2: Write-Through Cache

Update cache when data changes:

```go
func (uc *Usecase) UpdateUser(ctx context.Context, user *User) error {
    // Update database
    if err := uc.repo.UpdateUser(ctx, user); err != nil {
        return fmt.Errorf("failed to update user: %w", err)
    }

    // Update cache
    cacheKey := fmt.Sprintf("user:%s", user.ID.String())
    _ = uc.cache.Set(ctx, cacheKey, user, 5*time.Minute)

    return nil
}
```

### Pattern 3: Cache Invalidation

Delete cache when data is removed:

```go
func (uc *Usecase) DeleteUser(ctx context.Context, userID uuid.UUID) error {
    // Delete from database
    if err := uc.repo.DeleteUser(ctx, userID); err != nil {
        return fmt.Errorf("failed to delete user: %w", err)
    }

    // Invalidate cache
    cacheKey := fmt.Sprintf("user:%s", userID.String())
    _, _ = uc.cache.Delete(ctx, cacheKey)

    // Also invalidate related caches
    pattern := fmt.Sprintf("user:%s:*", userID.String())
    _, _ = uc.cache.DeletePattern(ctx, pattern)

    return nil
}
```

### Pattern 4: Rate Limiting

Implement rate limiting with counters:

```go
func (uc *Usecase) CheckRateLimit(ctx context.Context, userID uuid.UUID, limit int64) (bool, error) {
    key := fmt.Sprintf("rate_limit:user:%s", userID.String())

    // Increment counter
    count, err := uc.cache.Increment(ctx, key, 1)
    if err != nil {
        return false, fmt.Errorf("failed to increment rate limit: %w", err)
    }

    // Set expiration on first request (1 minute window)
    if count == 1 {
        _ = uc.cache.Expire(ctx, key, 1*time.Minute)
    }

    // Check if limit exceeded
    if count > limit {
        uc.logger.Warn("rate limit exceeded",
            zap.String("user_id", userID.String()),
            zap.Int64("count", count),
            zap.Int64("limit", limit),
        )
        return false, fmt.Errorf("rate limit exceeded")
    }

    return true, nil
}
```

### Pattern 5: Session Management

Store user sessions:

```go
type Session struct {
    UserID    uuid.UUID `json:"user_id"`
    Token     string    `json:"token"`
    ExpiresAt time.Time `json:"expires_at"`
}

func (uc *Usecase) CreateSession(ctx context.Context, userID uuid.UUID) (*Session, error) {
    session := &Session{
        UserID:    userID,
        Token:     generateToken(), // Your token generation logic
        ExpiresAt: time.Now().Add(24 * time.Hour),
    }

    key := fmt.Sprintf("session:%s", session.Token)

    // Store session in cache (24 hours TTL)
    if err := uc.cache.Set(ctx, key, session, 24*time.Hour); err != nil {
        return nil, fmt.Errorf("failed to create session: %w", err)
    }

    return session, nil
}

func (uc *Usecase) ValidateSession(ctx context.Context, token string) (*Session, error) {
    key := fmt.Sprintf("session:%s", token)

    var session Session
    if err := uc.cache.Get(ctx, key, &session); err != nil {
        if errors.Is(err, cache.ErrCacheMiss) {
            return nil, fmt.Errorf("invalid session")
        }
        return nil, fmt.Errorf("failed to validate session: %w", err)
    }

    return &session, nil
}
```

### Pattern 6: Batch Operations

Cache multiple entities efficiently:

```go
func (uc *Usecase) GetUsersByIDs(ctx context.Context, userIDs []uuid.UUID) ([]*User, error) {
    // Build cache keys
    cacheKeys := make([]string, len(userIDs))
    keyToID := make(map[string]uuid.UUID)
    for i, id := range userIDs {
        key := fmt.Sprintf("user:%s", id.String())
        cacheKeys[i] = key
        keyToID[key] = id
    }

    // Try to get from cache
    cached, _ := uc.cache.MGet(ctx, cacheKeys)

    // Identify cache misses
    var missingIDs []uuid.UUID
    for key, id := range keyToID {
        if _, found := cached[key]; !found {
            missingIDs = append(missingIDs, id)
        }
    }

    // Fetch missing users from database
    var users []*User
    if len(missingIDs) > 0 {
        dbUsers, err := uc.repo.GetUsersByIDs(ctx, missingIDs)
        if err != nil {
            return nil, fmt.Errorf("failed to get users: %w", err)
        }

        // Cache the fetched users
        toCache := make(map[string]interface{})
        for _, user := range dbUsers {
            key := fmt.Sprintf("user:%s", user.ID.String())
            toCache[key] = user
            users = append(users, user)
        }
        _ = uc.cache.MSet(ctx, toCache, 5*time.Minute)
    }

    // Add cached users
    for _, data := range cached {
        // Unmarshal cached data
        // (Note: MGet returns interface{}, may need type assertion)
        users = append(users, data.(*User))
    }

    return users, nil
}
```

## Key Naming Conventions

Use consistent naming patterns for cache keys:

- Users: `user:{uuid}`
- Sessions: `session:{token}`
- Rate limits: `rate_limit:{resource}:{id}`
- Permissions: `permission:{user_id}:{slug}`
- Dialogs: `dialog:{uuid}`
- Messages: `message:{uuid}`
- Temporary data: `temp:{context}:{id}`

**Benefits:**
- Easy to identify key purpose
- Pattern-based deletion works cleanly
- Avoids key collisions

## TTL Recommendations

Choose appropriate TTL values based on data volatility:

- **User profiles**: 5-10 minutes (semi-static)
- **Permissions/Roles**: 5-10 minutes (rarely change)
- **Sessions**: 24 hours (explicit expiration)
- **Rate limits**: 1-60 minutes (time-window based)
- **API responses**: 30-300 seconds (frequently changing)
- **Temporary data**: As needed (1-60 minutes)

**Important:** Always set explicit TTLs to prevent memory bloat. Avoid TTL=0 (no expiration) unless absolutely necessary.

## Error Handling

### Graceful Degradation

Cache failures should NOT break your application:

```go
// GOOD: Ignore cache errors, fall back to database
var user User
err := uc.cache.Get(ctx, key, &user)
if err != nil {
    // Fall back to database
    user, err = uc.repo.GetUserByID(ctx, userID)
    if err != nil {
        return nil, err
    }
}
```

### Logging

Log cache operations at Debug level (already implemented):

```go
// Cache hits/misses are automatically logged
// No need to add extra logging in domain code
```

### Monitoring (Future)

Consider adding metrics for:
- Cache hit/miss ratio
- Cache operation latency
- Cache error rate
- Memory usage

## Testing

### Unit Tests with Mock Cache

```go
import "github.com/stretchr/testify/mock"

type MockCache struct {
    mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key string, dest interface{}) error {
    args := m.Called(ctx, key, dest)
    return args.Error(0)
}

// ... implement other methods

func TestUsecase_GetUser(t *testing.T) {
    mockCache := new(MockCache)
    mockRepo := new(MockRepository)

    uc := NewUsecase(mockRepo, mockCache, logger)

    // Test cache hit
    mockCache.On("Get", ctx, "user:123", mock.Anything).Return(nil)

    user, err := uc.GetUserByID(ctx, userID)
    assert.NoError(t, err)
    mockCache.AssertExpectations(t)
}
```

### Integration Tests

See `redis_cache_test.go` for examples using miniredis.

## Performance Considerations

1. **Connection Pooling**: Uses existing Redis pool (PoolSize: 10)
2. **Serialization**: JSON for complex types, direct string for simple values
3. **Batch Operations**: Use MGet/MSet for multiple keys
4. **Pattern Operations**: Use DeletePattern/Keys sparingly (O(N) complexity)

## Security Considerations

1. **Key Namespacing**: Always namespace keys to avoid collisions
2. **Sensitive Data**: Be cautious caching sensitive data (passwords, tokens)
3. **TTL**: Set appropriate TTLs to limit exposure window
4. **Validation**: Always validate data retrieved from cache

## Common Pitfalls

### ❌ DON'T: Cache data without TTL
```go
// Bad: No expiration
cache.Set(ctx, key, value, 0)
```

### ✅ DO: Always set explicit TTL
```go
// Good: Explicit TTL
cache.Set(ctx, key, value, 5*time.Minute)
```

### ❌ DON'T: Fail request on cache errors
```go
// Bad: Request fails if cache fails
if err := cache.Get(ctx, key, &user); err != nil {
    return nil, err // Wrong!
}
```

### ✅ DO: Gracefully degrade
```go
// Good: Fall back to database
if err := cache.Get(ctx, key, &user); err != nil {
    user, err = repo.GetUser(ctx, id)
    // ...
}
```

### ❌ DON'T: Forget to invalidate cache
```go
// Bad: Cache becomes stale
func UpdateUser(ctx, user) error {
    return repo.UpdateUser(ctx, user) // Cache not updated!
}
```

### ✅ DO: Invalidate on writes
```go
// Good: Keep cache in sync
func UpdateUser(ctx, user) error {
    if err := repo.UpdateUser(ctx, user); err != nil {
        return err
    }
    _ = cache.Set(ctx, key, user, ttl) // Update cache
    return nil
}
```

## Next Steps

1. **Start Small**: Begin with one domain (e.g., agent) as proof of concept
2. **Monitor**: Observe cache hit rates and adjust TTLs
3. **Expand**: Gradually add cache to other domains
4. **Optimize**: Fine-tune based on metrics and usage patterns

## Questions?

Refer to:
- `cache.go` - Interface documentation
- `redis_cache_test.go` - Usage examples
- CLAUDE.md - Architecture patterns
- architecture_v2.drawio - System design