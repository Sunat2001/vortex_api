# Vortex Tech Strategist - Memory

## Architecture Overview
- Clean Architecture: Delivery -> Usecase -> Repository with constructor DI
- Go 1.25, pgx/v5, Gin, Zap, Redis go-redis/v9
- Module path: `github.com/voronka/backend/internal/...`
- Bootstrap via `internal/app/bootstrap.go` -> Infrastructure struct

## Redis Streams Configuration
- 5 streams: `stream:inbound_raw`, `stream:inbound_messages`, `stream:outbound_messages`, `stream:ads_tasks`, `stream:ai_jobs`
- MAXLEN: 10000 (approximate trimming), configurable via `REDIS_STREAMMAXLEN`
- Consumer group: `voronka-workers`, BLOCK time: 2s
- XREADGROUP COUNT: 10 for inbound/outbound, 5 for ads/ai
- Stream setup split: API vs Workers (`internal/app/streams.go`)

## Webhook Pipeline (key analysis point)
- `webhook_handler.go`: Validate signature -> Enqueue to `stream:inbound_raw` -> 200 OK
- `webhook_processor.go`: XREADGROUP from `stream:inbound_raw` -> parse source -> route to platform parser -> ACK+XDEL
- PermanentError type wraps non-retryable errors (missing source, invalid JSON, no channel)
- Transient errors left in PEL for retry (no explicit XCLAIM worker yet)
- Currently single worker goroutine per stream type in `cmd/workers/main.go`

## Worker Patterns
- Each stream has exactly 1 goroutine consumer (no worker pool scaling)
- Worker name: `worker-{uuid[:8]}` unique per process
- Graceful shutdown: context cancel + WaitGroup + 30s timeout
- XDEL called after ACK (active cleanup beyond MAXLEN trimming)

## Database
- pgxpool: MaxOpenConns=25, MaxIdleConns=5 (defaults)
- UUIDv7 for messages table (time-ordered), gen_random_uuid() elsewhere
- Platforms: telegram, whatsapp, instagram, facebook
- Key tables: channels, contacts, dialogs, messages, dialog_events

## Identified Issues / Observations
- No XCLAIM goroutine for recovering stuck PEL messages -> DESIGN COMPLETE (see below)
- Single goroutine per stream = no horizontal scaling within a single process -> DESIGN COMPLETE
- inbound workers do processRawWebhook -> chat.Usecase.ProcessIncomingWebhook (DB-heavy)
- No backpressure mechanism on webhook ingestion side -> SOLVED via semaphore backpressure

## XCLAIM Recovery + Worker Pool Design (Feb 2026)
- Design: `inboundRawWorkerPool` struct with semaphore (buffered chan), XCLAIM recovery goroutine
- Uses XAUTOCLAIM (Redis 6.2+, go-redis/v9 v9.17.2 supports it) instead of XPENDING+XCLAIM
- Config additions: WorkerPoolSize(10), ClaimMinIdleTime(60s), ClaimInterval(30s), ClaimBatchSize(50), MaxRetryCount(3)
- Dead letter stream: `stream:dead_letters` for messages exceeding MaxRetryCount
- Key concern: XCLAIM + original ACK race -> requires idempotent ProcessIncomingWebhook
- Scaling limit: poolSize capped by pgxpool.MaxConns(25); multiple worker processes compound this
- New StreamManager method: AutoClaimMessages wrapping XAutoClaim
- Per-message timeout: 30s context.WithTimeout prevents goroutine leaks

## Key File Paths
- Stream constants: `internal/shared/redis/streams.go`
- Webhook handler: `internal/chat/delivery/webhook_handler.go`
- Webhook processor: `cmd/workers/webhook_processor.go`
- Worker main: `cmd/workers/main.go`
- Stream setup: `internal/app/streams.go`
- Config: `internal/shared/config/config.go`
- Chat entities: `internal/chat/entity.go`
- DB schema: `tables.sql`
