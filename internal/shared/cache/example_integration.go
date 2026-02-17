package cache

// This file contains example code showing how to integrate the cache
// into cmd/api/main.go and domain usecases.
//
// DO NOT import this file - it's for reference only.

/*

// ============================================================================
// Example 1: Integration in cmd/api/main.go
// ============================================================================

import (
    "github.com/voronka/backend/internal/shared/cache"
    "github.com/voronka/backend/internal/shared/redis"
    // ... other imports
)

func main() {
    ctx := context.Background()

    // Load configuration
    cfg, err := config.Load()
    if err != nil {
        log.Fatal(err)
    }

    // Initialize logger
    logger, err := logger.New(&cfg.Logger)
    if err != nil {
        log.Fatal(err)
    }

    // Initialize database
    pgPool, err := database.NewPostgresPool(ctx, &cfg.Database, logger)
    if err != nil {
        logger.Fatal("failed to connect to database", zap.Error(err))
    }
    defer database.Close(pgPool, logger)

    // Initialize Redis
    redisClient, err := redis.NewRedisClient(ctx, &cfg.Redis, logger)
    if err != nil {
        logger.Fatal("failed to connect to Redis", zap.Error(err))
    }
    defer redis.Close(redisClient, logger)

    // ========== NEW: Initialize Cache Manager ==========
    cacheManager := cache.NewRedisCache(redisClient, logger)
    // ==================================================

    // Initialize Redis Stream Manager
    streamManager := redis.NewStreamManager(redisClient, logger, cfg.Redis.StreamMaxLen)

    // Initialize repositories
    agentRepo := agent.NewRepository(pgPool)
    // ... other repositories

    // ========== NEW: Inject cache into usecases ==========
    agentUsecase := agent.NewUsecase(agentRepo, cacheManager, logger)
    // ====================================================

    // Initialize HTTP handlers
    agentHandler := agentDelivery.NewHTTPHandler(agentUsecase, logger)

    // Setup Gin router
    router := gin.Default()
    v1 := router.Group("/api/v1")
    agentHandler.RegisterRoutes(v1)

    // Start server
    // ...
}

// ============================================================================
// Example 2: Usage in Domain Usecase (internal/agent/usecase.go)
// ============================================================================

package agent

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/voronka/backend/internal/shared/cache"
    "go.uber.org/zap"
)

type Usecase struct {
    repo   Repository
    cache  cache.Cache  // Add cache field
    logger *zap.Logger
}

// Update constructor to accept cache
func NewUsecase(repo Repository, cache cache.Cache, logger *zap.Logger) *Usecase {
    return &Usecase{
        repo:   repo,
        cache:  cache,
        logger: logger,
    }
}

// Example: Get user with caching
func (uc *Usecase) GetUserByID(ctx context.Context, userID uuid.UUID) (*User, error) {
    cacheKey := fmt.Sprintf("user:%s", userID.String())

    // Try cache first
    var cachedUser User
    err := uc.cache.Get(ctx, cacheKey, &cachedUser)
    if err == nil {
        uc.logger.Debug("user retrieved from cache", zap.String("user_id", userID.String()))
        return &cachedUser, nil
    }

    // Cache miss - fetch from database
    user, err := uc.repo.GetUserByID(ctx, userID)
    if err != nil {
        return nil, fmt.Errorf("failed to get user: %w", err)
    }

    // Store in cache (5 minutes TTL)
    // Ignore cache errors - graceful degradation
    _ = uc.cache.Set(ctx, cacheKey, user, 5*time.Minute)

    return user, nil
}

// Example: Update user and invalidate cache
func (uc *Usecase) UpdateUser(ctx context.Context, req *UpdateUserRequest) (*User, error) {
    // Update in database
    user, err := uc.repo.UpdateUser(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("failed to update user: %w", err)
    }

    // Update cache (write-through)
    cacheKey := fmt.Sprintf("user:%s", user.ID.String())
    _ = uc.cache.Set(ctx, cacheKey, user, 5*time.Minute)

    return user, nil
}

// Example: Delete user and invalidate cache
func (uc *Usecase) DeleteUser(ctx context.Context, userID uuid.UUID) error {
    // Delete from database
    if err := uc.repo.DeleteUser(ctx, userID); err != nil {
        return fmt.Errorf("failed to delete user: %w", err)
    }

    // Invalidate cache
    cacheKey := fmt.Sprintf("user:%s", userID.String())
    _, _ = uc.cache.Delete(ctx, cacheKey)

    // Also delete related caches
    pattern := fmt.Sprintf("user:%s:*", userID.String())
    _, _ = uc.cache.DeletePattern(ctx, pattern)

    return nil
}

// Example: Rate limiting
func (uc *Usecase) CheckRateLimit(ctx context.Context, userID uuid.UUID) error {
    key := fmt.Sprintf("rate_limit:api:user:%s", userID.String())
    limit := int64(100) // 100 requests per minute

    // Increment counter
    count, err := uc.cache.Increment(ctx, key, 1)
    if err != nil {
        // Don't fail request if cache is down - log and continue
        uc.logger.Warn("rate limit check failed", zap.Error(err))
        return nil
    }

    // Set expiration on first request
    if count == 1 {
        _ = uc.cache.Expire(ctx, key, 1*time.Minute)
    }

    // Check limit
    if count > limit {
        uc.logger.Warn("rate limit exceeded",
            zap.String("user_id", userID.String()),
            zap.Int64("count", count),
        )
        return fmt.Errorf("rate limit exceeded")
    }

    return nil
}

// Example: Session management
type Session struct {
    UserID    uuid.UUID `json:"user_id"`
    Token     string    `json:"token"`
    ExpiresAt time.Time `json:"expires_at"`
}

func (uc *Usecase) CreateSession(ctx context.Context, userID uuid.UUID, token string) error {
    session := &Session{
        UserID:    userID,
        Token:     token,
        ExpiresAt: time.Now().Add(24 * time.Hour),
    }

    key := fmt.Sprintf("session:%s", token)
    return uc.cache.Set(ctx, key, session, 24*time.Hour)
}

func (uc *Usecase) ValidateSession(ctx context.Context, token string) (*Session, error) {
    key := fmt.Sprintf("session:%s", token)

    var session Session
    err := uc.cache.Get(ctx, key, &session)
    if err != nil {
        if errors.Is(err, cache.ErrCacheMiss) {
            return nil, fmt.Errorf("invalid or expired session")
        }
        return nil, fmt.Errorf("failed to validate session: %w", err)
    }

    return &session, nil
}

func (uc *Usecase) DeleteSession(ctx context.Context, token string) error {
    key := fmt.Sprintf("session:%s", token)
    _, err := uc.cache.Delete(ctx, key)
    return err
}

// ============================================================================
// Example 3: Using cache without modifying existing usecase
// ============================================================================

// If you don't want to modify existing usecases immediately, you can use
// cache directly in handlers or create a wrapper:

type CachedAgentUsecase struct {
    *Usecase // Embed original usecase
    cache    cache.Cache
}

func NewCachedAgentUsecase(original *Usecase, cache cache.Cache) *CachedAgentUsecase {
    return &CachedAgentUsecase{
        Usecase: original,
        cache:   cache,
    }
}

// Override methods that should use cache
func (uc *CachedAgentUsecase) GetUserByID(ctx context.Context, userID uuid.UUID) (*User, error) {
    cacheKey := fmt.Sprintf("user:%s", userID.String())

    var user User
    if err := uc.cache.Get(ctx, cacheKey, &user); err == nil {
        return &user, nil
    }

    // Call original usecase method
    user, err := uc.Usecase.GetUserByID(ctx, userID)
    if err != nil {
        return nil, err
    }

    _ = uc.cache.Set(ctx, cacheKey, user, 5*time.Minute)
    return user, nil
}

*/