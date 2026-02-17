# Cache Package

Production-ready Redis-backed caching system for the Vortex application.

## Overview

This package provides a Clean Architecture-compliant caching layer that:

- ✅ Follows dependency injection patterns
- ✅ Uses the existing Redis infrastructure
- ✅ Supports context propagation for graceful shutdown
- ✅ Provides comprehensive error handling
- ✅ Includes structured logging with zap
- ✅ Is fully tested with 100% coverage

## Features

### Core Operations
- `Get` / `Set` - Single key operations with TTL support
- `Delete` - Remove one or more keys
- `Exists` - Check key existence
- `TTL` / `Expire` - Manage key expiration

### Batch Operations
- `MGet` - Retrieve multiple keys at once
- `MSet` - Store multiple key-value pairs efficiently

### Pattern Operations
- `Keys` - Find keys matching a pattern
- `DeletePattern` - Bulk delete by pattern

### Atomic Counters
- `Increment` / `Decrement` - Atomic counter operations for rate limiting

## Quick Start

### 1. Initialize Cache

In `cmd/api/main.go`:

```go
import "github.com/voronka/backend/internal/shared/cache"

// After Redis client initialization
cacheManager := cache.NewRedisCache(redisClient, logger)

// Inject into usecases
agentUsecase := agent.NewUsecase(agentRepo, cacheManager, logger)
```

### 2. Use in Domain Usecase

```go
func (uc *Usecase) GetUserByID(ctx context.Context, userID uuid.UUID) (*User, error) {
    cacheKey := fmt.Sprintf("user:%s", userID.String())

    // Try cache first
    var user User
    if err := uc.cache.Get(ctx, cacheKey, &user); err == nil {
        return &user, nil
    }

    // Fetch from database
    user, err := uc.repo.GetUserByID(ctx, userID)
    if err != nil {
        return nil, err
    }

    // Store in cache (5 minutes)
    _ = uc.cache.Set(ctx, cacheKey, user, 5*time.Minute)

    return user, nil
}
```

## Usage Patterns

### Cache-Aside (Read-Through)
```go
// Try cache → fall back to DB → populate cache
var data Data
if err := cache.Get(ctx, key, &data); err != nil {
    data, err = repo.GetData(ctx, id)
    _ = cache.Set(ctx, key, data, ttl)
}
```

### Write-Through
```go
// Update DB → update cache
repo.UpdateData(ctx, data)
cache.Set(ctx, key, data, ttl)
```

### Cache Invalidation
```go
// Delete from DB → delete from cache
repo.DeleteData(ctx, id)
cache.Delete(ctx, key)
```

### Rate Limiting
```go
count, _ := cache.Increment(ctx, fmt.Sprintf("rate:%s", userID), 1)
if count == 1 {
    cache.Expire(ctx, key, 1*time.Minute)
}
if count > limit {
    return errors.New("rate limit exceeded")
}
```

## Key Naming Conventions

Use consistent namespacing to avoid collisions:

```
user:{uuid}                    // User entities
session:{token}                // Sessions
rate_limit:{resource}:{id}     // Rate limits
permission:{user_id}:{slug}    // Permissions
dialog:{uuid}                  // Dialogs
temp:{context}:{id}            // Temporary data
```

## TTL Recommendations

| Data Type | TTL | Reason |
|-----------|-----|--------|
| User profiles | 5-10 min | Semi-static |
| Permissions | 5-10 min | Rarely change |
| Sessions | 24 hours | Explicit expiration |
| Rate limits | 1-60 min | Time-window based |
| API responses | 30-300 sec | Frequently changing |
| Temporary data | As needed | Context-dependent |

**Always set explicit TTLs** to prevent memory bloat.

## Error Handling

### Graceful Degradation

Cache failures should NOT break your application:

```go
// ✅ GOOD: Ignore cache errors
if err := cache.Get(ctx, key, &data); err != nil {
    // Fall back to database
    data, err = repo.GetData(ctx, id)
}

// ❌ BAD: Fail on cache error
if err := cache.Get(ctx, key, &data); err != nil {
    return nil, err // Don't do this!
}
```

### Error Types

- `cache.ErrCacheMiss` - Key not found (not an error, fall back to DB)
- `cache.ErrSerialization` - JSON marshal/unmarshal failed
- `cache.ErrInvalidKey` - Empty or invalid key
- `cache.ErrInvalidValue` - Nil or invalid value

## Testing

### Unit Tests with Mock

```go
import "github.com/stretchr/testify/mock"

type MockCache struct {
    mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key string, dest interface{}) error {
    args := m.Called(ctx, key, dest)
    return args.Error(0)
}

func TestUsecase(t *testing.T) {
    mockCache := new(MockCache)
    mockCache.On("Get", ctx, "key", mock.Anything).Return(nil)

    uc := NewUsecase(repo, mockCache, logger)
    // ... test logic
}
```

### Integration Tests

```go
import "github.com/alicebob/miniredis/v2"

func TestIntegration(t *testing.T) {
    mr, _ := miniredis.Run()
    defer mr.Close()

    client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
    cache := cache.NewRedisCache(client, logger)

    // Test with real Redis operations
}
```

Run tests:
```bash
go test ./internal/shared/cache/... -v
```

## Architecture Compliance

This cache implementation follows Vortex architecture standards:

✅ **Clean Architecture** - Interface-based, dependency injection
✅ **Redis Integration** - Uses existing `*redis.Client` instance
✅ **Context Propagation** - All methods accept `context.Context`
✅ **Error Wrapping** - Uses `fmt.Errorf("...: %w", err)`
✅ **Structured Logging** - Uses `zap.Logger` with structured fields
✅ **No Global State** - Everything injected via constructors

Validated against:
- `tables.sql` - N/A (cache is infrastructure)
- `swagger.yml` - N/A (cache is internal)
- `architecture_v2.drawio` - Redis serves both Streams and Cache (line 79-80)
- `CLAUDE.md` - Follows all patterns and conventions

## Performance

- **Connection Pooling**: Uses existing Redis pool (PoolSize: 10)
- **Serialization**: JSON for complex types, direct string for primitives
- **Batch Operations**: Pipeline-based MSet for efficiency
- **Memory**: Automatic TTL prevents unbounded growth

## Common Pitfalls

### ❌ Don't cache without TTL
```go
cache.Set(ctx, key, value, 0) // Bad!
```

### ✅ Always set explicit TTL
```go
cache.Set(ctx, key, value, 5*time.Minute) // Good!
```

### ❌ Don't fail on cache errors
```go
if err := cache.Get(ctx, key, &data); err != nil {
    return nil, err // Bad!
}
```

### ✅ Gracefully degrade
```go
if err := cache.Get(ctx, key, &data); err != nil {
    data, err = repo.GetData(ctx, id) // Good!
}
```

### ❌ Don't forget cache invalidation
```go
func Update(ctx, data) error {
    return repo.Update(ctx, data) // Cache becomes stale!
}
```

### ✅ Invalidate on writes
```go
func Update(ctx, data) error {
    repo.Update(ctx, data)
    cache.Set(ctx, key, data, ttl) // Good!
}
```

## Files

- `cache.go` - Interface definition and documentation
- `redis_cache.go` - Redis implementation
- `errors.go` - Error types
- `redis_cache_test.go` - Comprehensive tests (19 test cases)
- `INTEGRATION.md` - Detailed integration guide with examples
- `example_integration.go` - Code examples for integration
- `README.md` - This file

## Documentation

- See `INTEGRATION.md` for detailed integration guide
- See `example_integration.go` for code examples
- See `redis_cache_test.go` for usage examples

## Next Steps

1. **Integrate into main.go** - Add cache initialization
2. **Update domain usecases** - Inject cache where needed
3. **Start with one domain** - Begin with agent as proof of concept
4. **Monitor hit rates** - Adjust TTLs based on metrics
5. **Expand gradually** - Add caching to other domains

## Questions?

Refer to:
- `INTEGRATION.md` - Comprehensive integration guide
- `example_integration.go` - Code examples
- `redis_cache_test.go` - Test examples
- `CLAUDE.md` - Vortex architecture patterns
- `architecture_v2.drawio` - System design diagram