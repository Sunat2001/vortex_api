# Vortex (Project Voronka) - Backend

A high-load SaaS platform for omnichannel CRM and Ads management. Built with Go 1.25, following Clean Architecture principles.

## Project Overview

Vortex aggregates messages from Telegram, WhatsApp, and Meta Ads, uses AI (via MCP) to analyze leads and product catalogs, and provides end-to-end analytics from ad spend to closed deals.

## Tech Stack

- **Language**: Go 1.25
- **Database**: PostgreSQL (Primary)
- **Cache & Streams**: Redis (for high-load message ingestion)
- **Web Framework**: Gin
- **Database Driver**: pgx/v5 (PostgreSQL)
- **Logging**: uber-go/zap (structured JSON logging)
- **Configuration**: Viper

## Architecture

**Modular Monolith** with Clean Architecture:
- **Delivery Layer**: HTTP handlers (Gin), WebSocket
- **Use Case Layer**: Business logic
- **Repository Layer**: Database access (pgx)
- **Entities**: Domain models

### Key Components

1. **API Server** (`cmd/api/main.go`)
   - REST API with Gin
   - WebSocket Hub for real-time updates
   - RBAC middleware
   - Health checks

2. **Workers** (`cmd/workers/main.go`)
   - Redis Stream consumers (at-least-once delivery)
   - Inbound message processor
   - Outbound message processor
   - Ads sync worker
   - AI orchestration worker

3. **Redis Streams**
   - `stream:inbound_messages` - Incoming messages from platforms
   - `stream:outbound_messages` - Outgoing messages to platforms
   - `stream:ads_tasks` - Ads sync and analytics
   - `stream:ai_jobs` - AI/MCP processing

## Project Structure

```
voronka/backend/
├── cmd/
│   ├── api/                    # HTTP server
│   └── workers/                # Background workers
├── internal/
│   ├── shared/
│   │   ├── config/            # Configuration management
│   │   ├── database/          # PostgreSQL connection
│   │   ├── redis/             # Redis client & streams
│   │   └── middleware/        # Auth, RBAC, logging
│   ├── agent/                 # User & RBAC domain
│   │   ├── entity.go
│   │   ├── repository.go
│   │   ├── usecase.go
│   │   └── delivery/
│   │       └── http_handler.go
│   ├── chat/                  # Messaging domain
│   │   ├── entity.go
│   │   └── repository.go
│   ├── ads/                   # Advertising domain (TODO)
│   └── catalog/               # Product catalog (TODO)
├── go.mod
├── go.sum
├── tables.sql                 # Database schema (source of truth)
├── swagger.yml                # API specification (source of truth)
└── architecture_v2.drawio     # System design diagram
```

## Configuration

Configuration is managed via environment variables or `config.yaml`:

### Environment Variables

```bash
# Server
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# Database
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=postgres
DATABASE_PASSWORD=postgres
DATABASE_DATABASE=voronka
DATABASE_SSLMODE=disable

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# Logger
LOGGER_LEVEL=info
LOGGER_DEVELOPMENT=false
LOGGER_ENCODING=json
```

## Quick Start with Docker (Recommended)

The fastest way to get started is using Docker Compose. This will automatically set up PostgreSQL, Redis, API server, and Workers.

### Prerequisites

- Docker 24+
- Docker Compose v2+
- Make (optional, but recommended)

### Start Development Environment

```bash
# Start all services (API, Workers, PostgreSQL, Redis)
make dev

# Or manually with docker compose
docker compose up -d
```

This will start:
- **API Server**: http://localhost:8080
- **PostgreSQL**: localhost:5432
- **Redis**: localhost:6379
- **pgAdmin**: http://localhost:5050 (admin@voronka.local / admin)
- **Redis Commander**: http://localhost:8081

### Useful Commands

```bash
make help              # Show all available commands
make docker-logs       # View logs from all services
make docker-logs-api   # View API logs only
make docker-restart    # Restart all services
make docker-down       # Stop all services
make db-reset          # Reset database
make check-health      # Check service health
```

### View Logs

```bash
# All services
make docker-logs

# Specific service
docker compose logs -f api
docker compose logs -f workers
```

### Stop Services

```bash
make stop
# or
docker compose down
```

---

## Setup (Without Docker)

If you prefer to run services locally without Docker:

### Prerequisites

- Go 1.25+
- PostgreSQL 14+
- Redis 7+

### Installation

1. **Clone the repository**

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Setup PostgreSQL**
   ```bash
   createdb voronka
   psql -U postgres -d voronka -f tables.sql
   ```

4. **Start Redis**
   ```bash
   redis-server
   ```

5. **Configure environment**
   ```bash
   cp .env.example .env
   # Edit .env with your settings
   ```

### Running

**API Server**
```bash
go run cmd/api/main.go
# or
make run-api
```

**Workers**
```bash
go run cmd/workers/main.go
# or
make run-workers
```

**Build for production**
```bash
make build
# or
go build -o bin/voronka-api cmd/api/main.go
go build -o bin/voronka-workers cmd/workers/main.go
```

## API Endpoints

### Health Check
```
GET /health
```

### Agents (RBAC)
```
POST   /v1/agents/invite          # Invite new agent
GET    /v1/agents                 # List all agents
GET    /v1/agents/:id             # Get agent details
PATCH  /v1/agents/me/status       # Update current agent status
GET    /v1/agents/workload        # Get agent workload
```

### Roles
```
GET    /v1/roles                  # List all roles
POST   /v1/roles                  # Create role
GET    /v1/roles/:id              # Get role with permissions
DELETE /v1/roles/:id              # Delete role
POST   /v1/roles/:id/permissions  # Assign permissions to role
```

### Permissions
```
GET    /v1/permissions            # List all permissions
```

## Development

### Code Style

- Follow Clean Architecture principles
- Business logic in `usecase` layer
- Database access in `repository` layer
- HTTP handlers in `delivery` layer
- Use interfaces for dependency injection

### Testing

```bash
go test ./...
```

### Database Migrations

The schema is defined in `tables.sql` (source of truth). Apply changes directly:

```bash
psql -U postgres -d voronka -f tables.sql
```

## Next Steps

1. **Chat Domain Implementation**
   - Complete repository implementation
   - Add use cases for dialog management
   - Implement HTTP handlers per swagger.yml
   - Add WebSocket support

2. **Ads Domain Implementation**
   - Meta Ads API integration
   - Performance tracking
   - Liquidity scoring

3. **Catalog Domain Implementation**
   - Product/service management
   - JSONB attribute handling
   - MCP tool integration

4. **AI/MCP Integration**
   - Claude/OpenAI client
   - MCP protocol implementation
   - Intent extraction
   - Response generation

5. **Authentication**
   - JWT implementation
   - Token validation middleware
   - Refresh token logic

## License

Proprietary