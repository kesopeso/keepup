---
type: reference
status: active
---

# Backend boundaries

## Backend

- Go service
- REST API for lifecycle and snapshots
- WebSocket server for live route events
- PostgreSQL + PostGIS for persistence

### Current Go Stack

- Routing: standard-library `net/http` ServeMux
- WebSocket: `github.com/coder/websocket`
- Database: `pgx`, with SQL in the repository implementation
- Logging: `slog`
- Manual migration tooling: [development workflow](../workflow/development.md)

### Current Backend Foundation

- API config is loaded from environment variables
- PostgreSQL connectivity is established on startup with retry/backoff inside a bounded startup window
- API dev tooling runs inside the Docker Compose API service, including `golangci-lint`
- `/livez` reports process liveness
- `/healthz` checks database reachability
- HTTP server shutdown is tied to process signal cancellation
- Migrations are manual and are not applied automatically on API startup
- The local Compose stack provides a tools-profile `migrate` service using the `migrate/migrate:4` image for manual migration commands


Source: [entry point](../../apps/api/cmd/api/main.go), [HTTP handlers](../../apps/api/internal/httpapi/server.go), [route service](../../apps/api/internal/routes/service.go), [repository](../../apps/api/internal/routes/postgres.go), [dependencies](../../apps/api/go.mod).

## Observability

Use structured logs from the start. Add metrics for:

- snapshot sizes
- point counts
- request durations
- accepted/rejected position updates
- live connections/events

This is enough to decide later when snapshot chunking, caching, or path derivation is needed.

The metrics above are desired observability work, not a claim that all metrics exist.
