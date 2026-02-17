# Quick Start Guide - Vortex (Voronka)

Get up and running in 60 seconds! 🚀

## 1. Start Everything

```bash
make dev
```

This single command will:
- Build Docker images
- Start PostgreSQL, Redis, API, and Workers
- Initialize database schema
- Start development tools (pgAdmin, Redis Commander)

## 2. Verify Services

```bash
# Check all services are healthy
make status

# Test API
curl http://localhost:8080/health
```

## 3. Access Services

| Service | URL | Credentials |
|---------|-----|-------------|
| API | http://localhost:8080 | - |
| pgAdmin | http://localhost:5050 | admin@voronka.local / admin |
| Redis Commander | http://localhost:8081 | - |
| PostgreSQL | localhost:5432 | postgres / postgres |
| Redis | localhost:6379 | - |

## 4. View Logs

```bash
# All services
make docker-logs

# Specific service
make docker-logs-api
make docker-logs-workers
```

## 5. Common Commands

```bash
make help              # Show all commands
make docker-restart    # Restart all services
make db-reset          # Reset database
make stop              # Stop everything
```

## Example API Calls

### Create a Role

```bash
curl -X POST http://localhost:8080/v1/roles \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Administrator",
    "slug": "admin",
    "permission_ids": []
  }'
```

### List Permissions

```bash
curl http://localhost:8080/v1/permissions
```

### Invite an Agent

```bash
curl -X POST http://localhost:8080/v1/agents/invite \
  -H "Content-Type: application/json" \
  -d '{
    "email": "agent@example.com",
    "full_name": "John Doe",
    "role_ids": []
  }'
```

### Get Agent Workload

```bash
curl http://localhost:8080/v1/agents/workload
```

## Troubleshooting

### Services won't start

```bash
# Check what went wrong
make docker-logs

# Rebuild everything
make docker-rebuild
```

### Port conflicts

```bash
# Check what's using the port
lsof -i :8080

# Stop the conflicting service or change port in docker-compose.yml
```

### Database issues

```bash
# Reset database
make db-reset

# Access PostgreSQL shell
make db-shell
```

### Redis issues

```bash
# Access Redis CLI
make redis-cli

# Restart Redis
docker compose restart redis
```

## Next Steps

1. **Read full documentation**: See `README.md` and `DOCKER.md`
2. **Explore API**: Check `swagger.yml` for all endpoints
3. **Review database schema**: See `tables.sql`
4. **Study architecture**: Open `architecture_v2.drawio`

## Development Workflow

```bash
# Morning: Start services
make dev

# During development: Watch logs
make docker-logs

# Made changes to code: Rebuild
make docker-rebuild

# Evening: Stop services
make stop
```

## Tips

- Use `make help` to discover all available commands
- The Makefile has 40+ convenience commands for common tasks
- All environment variables can be customized via `.env` file
- Services auto-restart on failure
- Database schema auto-initializes on first run
- Logs are colorized in development mode

## Getting Help

```bash
# Show all make targets
make help

# View docker-compose configuration
docker compose config

# Check service health
make check-health

# View service status
make status
```

---

**Happy coding!** 🎉

For detailed documentation, see:
- `README.md` - Full project documentation
- `DOCKER.md` - Comprehensive Docker guide
- `swagger.yml` - API specification