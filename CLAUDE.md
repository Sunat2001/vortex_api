# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project: Vortex (Project Voronka)

High-load SaaS platform for omnichannel CRM and Ads management. Built with Go 1.25 following Clean Architecture principles.

## Source of Truth Files

**CRITICAL**: These three files define the entire system and must be consulted before making any changes:

1. **`tables.sql`** - Database schema (PostgreSQL). All entity structures, relationships, and constraints are defined here. Use UUIDs for IDs, JSONB for polymorphic data.

2. **`swagger.yml`** - API contract. All HTTP endpoints, request/response DTOs, and status codes must match this specification exactly.

3. **`architecture_v2.drawio`** - System design diagram showing Redis Streams data flow, worker architecture, and service boundaries.

## Architecture Overview

### Clean Architecture Layers

The codebase follows strict Clean Architecture with three layers:

```
Delivery Layer (HTTP/Transport)
    ↓ depends on ↓
Usecase Layer (Business Logic)
    ↓ depends on ↓
Repository Layer (Data Access)
```

**Dependency Rule**: Inner layers never depend on outer layers. All cross-layer communication happens through interfaces.

### Domain Structure Pattern

Each domain follows this exact structure (see `internal/agent/` as reference):

```
internal/{domain}/
├── entity.go           # Domain models (User, Role, Permission)
├── repository.go       # Interface + pgx implementation
├── usecase.go          # Business logic (validates, orchestrates)
└── delivery/
    └── http_handler.go # Gin handlers (HTTP 200/400/500)
```

**Key Pattern**:
- `entity.go` - Pure domain types, validation methods, request/response DTOs
- `repository.go` - Database interface + concrete pgx/v5 implementation
- `usecase.go` - Business logic, calls repository, returns domain errors
- `delivery/http_handler.go` - HTTP concerns only, calls usecase, maps to HTTP status codes

### Dependency Injection Pattern

All dependencies are injected through constructors in `cmd/api/main.go`:

```go
// 1. Initialize infrastructure
pgPool := database.NewPostgresPool(ctx, cfg, logger)
redisClient := redis.NewRedisClient(ctx, cfg, logger)

// 2. Create repositories
agentRepo := agent.NewRepository(pgPool)

// 3. Create use cases
agentUsecase := agent.NewUsecase(agentRepo, logger)

// 4. Create handlers
agentHandler := agentDelivery.NewHTTPHandler(agentUsecase, logger)

// 5. Register routes
agentHandler.RegisterRoutes(v1)
```

**Never use globals or singletons**. Everything flows through constructor injection.

## Redis Streams Architecture

### Stream Names (Constants)
```go
StreamInboundMessages  = "stream:inbound_messages"  // Platform → DB
StreamOutboundMessages = "stream:outbound_messages" // DB → Platform
StreamAdsTasks         = "stream:ads_tasks"         // Ads sync
StreamAIJobs           = "stream:ai_jobs"           // AI/MCP processing
```

### Consumer Group Pattern

All workers use consumer groups for at-least-once delivery:

```go
// 1. Create consumer group (idempotent)
streamManager.CreateConsumerGroup(ctx, streamName, "voronka-workers")

// 2. Read with XREADGROUP
streams := streamManager.ReadGroupMessage(ctx, streamName, group, consumer, count, blockTime)

// 3. Process message
err := processMessage(ctx, message.Values)

// 4. Acknowledge (MUST do this)
streamManager.AckMessage(ctx, streamName, group, message.ID)
```

**Critical**: Always ACK messages after successful processing. Failed messages remain in pending list and can be claimed by other workers.

## Database Patterns

### pgx/v5 Usage

Use `pgxpool.Pool` (not `*sql.DB`). All queries use context:

```go
// Query single row
err := pool.QueryRow(ctx, query, args...).Scan(&dest...)

// Query multiple rows
rows, err := pool.Query(ctx, query, args...)
defer rows.Close()
for rows.Next() {
    rows.Scan(&dest...)
}

// Execute (INSERT/UPDATE/DELETE)
result, err := pool.Exec(ctx, query, args...)
if result.RowsAffected() == 0 {
    return fmt.Errorf("not found")
}
```

**No ORM**. Write raw SQL. Match `tables.sql` schema exactly.

### UUID Handling

Always use `github.com/google/uuid`:

```go
id := uuid.New()                    // Generate new UUID
parsedID, err := uuid.Parse(str)    // Parse from string
```

### JSONB Fields

For polymorphic data (messages.payload, catalog_items.attributes):

```go
import "encoding/json"

type Message struct {
    Payload json.RawMessage `json:"payload" db:"payload"`
}

// Unmarshal when needed
var data map[string]interface{}
json.Unmarshal(msg.Payload, &data)
```

## Configuration Management

Uses Viper with environment variable override:

```go
// Load config (reads from env or config.yaml)
cfg, err := config.Load()

// Access nested values
cfg.Database.Host
cfg.Redis.StreamMaxLen
cfg.Logger.Level
```

**Environment variables**: Uppercase with underscores (e.g., `DATABASE_HOST`, `REDIS_STREAMMAXLEN`).

## Logging Pattern

Use structured logging with zap throughout:

```go
logger.Info("operation successful",
    zap.String("user_id", userID.String()),
    zap.Int("count", count),
)

logger.Error("operation failed",
    zap.Error(err),
    zap.String("context", "details"),
)
```

**Never use `fmt.Println`**. Always use `logger`.

## Error Handling Pattern

Return wrapped errors with context:

```go
if err != nil {
    return fmt.Errorf("failed to create user: %w", err)
}
```

Use `errors.Is()` and `errors.As()` for error checking.

## RBAC Implementation

Permission checking happens in middleware:

```go
// In delivery layer
router.POST("/roles",
    middleware.AuthMiddleware(),
    middleware.RBACMiddleware(permChecker, "roles.create"),
    handler.CreateRole,
)
```

Repository implements `HasPermission()`:

```sql
-- Checks user -> roles -> permissions
SELECT EXISTS(
    SELECT 1 FROM permissions p
    INNER JOIN role_permissions rp ON p.id = rp.permission_id
    INNER JOIN user_roles ur ON rp.role_id = ur.role_id
    WHERE ur.user_id = $1 AND p.slug = $2
)
```

## Common Development Commands

### Docker (Recommended)
```bash
make dev              # Start all services (PostgreSQL, Redis, API, Workers)
make docker-logs      # View logs
make docker-logs-api  # View API logs only
make db-reset         # Reset database
make stop             # Stop all services
```

### Local Development
```bash
make build            # Build binaries to bin/
make run-api          # Run API server
make run-workers      # Run workers
make test             # Run tests
```

### Database
```bash
make db-shell         # psql shell
make db-migrate       # Apply schema changes
```

### Redis
```bash
make redis-cli        # Redis CLI
```

## Testing Patterns

When writing tests:

```go
func TestUsecase(t *testing.T) {
    // Use testify/mock for repository mocks
    mockRepo := new(MockRepository)
    mockRepo.On("GetUserByID", ctx, id).Return(user, nil)

    uc := NewUsecase(mockRepo, logger)
    result, err := uc.GetAgent(ctx, id)

    assert.NoError(t, err)
    assert.Equal(t, expected, result)
    mockRepo.AssertExpectations(t)
}
```

## Adding a New Domain

Follow this checklist:

1. **Define entities** in `entity.go` (match `tables.sql`)
2. **Create repository interface** in `repository.go`
3. **Implement pgx repository** in same file
4. **Write business logic** in `usecase.go`
5. **Create HTTP handlers** in `delivery/http_handler.go`
6. **Match swagger.yml** endpoints exactly
7. **Wire up in `cmd/api/main.go`**:
   ```go
   repo := domain.NewRepository(pgPool)
   uc := domain.NewUsecase(repo, logger)
   handler := delivery.NewHTTPHandler(uc, logger)
   handler.RegisterRoutes(v1)
   ```

## Common Patterns

### Status Enums
```go
type UserStatus string
const (
    UserStatusOnline  UserStatus = "online"
    UserStatusOffline UserStatus = "offline"
    UserStatusBusy    UserStatus = "busy"
)
func (s UserStatus) IsValid() bool { /* ... */ }
```

### Time Handling
```go
CreatedAt: time.Now()  // Always use time.Now() for timestamps
```

### Handler Pattern
```go
func (h *Handler) CreateRole(c *gin.Context) {
    var req CreateRoleRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    result, err := h.usecase.CreateRole(c.Request.Context(), &req)
    if err != nil {
        h.logger.Error("failed", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusCreated, result)
}
```

## Worker Implementation Pattern

See `cmd/workers/main.go` for reference:

```go
func startWorker(ctx context.Context, sm *StreamManager, group, consumer string) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
            streams := sm.ReadGroupMessage(ctx, streamName, group, consumer, 10, 2*time.Second)
            for _, stream := range streams {
                for _, msg := range stream.Messages {
                    processMessage(ctx, msg.Values)
                    sm.AckMessage(ctx, streamName, group, msg.ID)
                }
            }
        }
    }
}
```

## Module Import Path

All imports use `github.com/voronka/backend/internal/...`

## Critical Conventions

- **UUIDs everywhere** for IDs (never auto-increment integers)
- **JSONB for flexible data** (messages.payload, metadata, attributes)
- **Foreign keys with CASCADE** where appropriate
- **Indexes on frequently queried fields** (see `tables.sql`)
- **No global state** - everything injected via constructors
- **Context propagation** - always pass `context.Context` as first parameter
- **Graceful shutdown** - respect context cancellation in workers
- **Health checks** - `/health` endpoint must return `200 OK` with `{"status":"ok"}`

## Performance Considerations

- **Connection pooling**: Configure `MaxOpenConns` and `MaxIdleConns` in config
- **Redis MAXLEN**: Streams auto-trim to prevent memory overflow (default: 10000)
- **Worker concurrency**: Scale workers horizontally by running multiple instances
- **At-least-once delivery**: Design all message processors to be idempotent

## Debugging Tips

```bash
# Check service health
curl http://localhost:8080/health

# View Redis streams
make redis-cli
> XINFO STREAM stream:inbound_messages
> XPENDING stream:inbound_messages voronka-workers

# View database
make db-shell
\d+ users
SELECT * FROM user_roles;
```

## Next Implementation Steps

1. **Chat Domain** - Complete repository, usecase, handlers (see `internal/chat/entity.go` for structure)
2. **Ads Domain** - Meta/Google API integration following same layered pattern
3. **Catalog Domain** - JSONB attribute handling for polymorphic product data
4. **JWT Authentication** - Replace placeholder in `middleware/auth.go`
5. **WebSocket Hub** - Real-time notifications for agents (see `architecture_v2.drawio`)
6. **MCP Integration** - AI orchestration worker implementation