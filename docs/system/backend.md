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

Production packaging uses a [multi-stage Dockerfile](../../apps/api/Dockerfile) with a static binary, CA certificates, and a non-root runtime. See [production API deployment](../workflow/development.md#production-api-image) for build commands and runtime configuration.

### Current Backend Foundation

The [account service](../../apps/api/internal/auth/service.go) and [authentication HTTP handler](../../apps/api/internal/httpapi/auth.go) are always initialized. Startup mounts `/auth/` and account-authorized REST/socket handlers; missing origins, public URL, outbox key or delivery provider fails configuration instead of selecting an anonymous mode. Account endpoints bypass legacy wildcard CORS. The [delivery/cleanup worker](../../apps/api/internal/auth/outbox.go) runs on every startup and joins before pool shutdown, with route ticket cleanup registered. Verification/recovery use transactional purpose tokens and encrypted mail queueing through SMTP, log or test-only fake delivery. See the [account contract](authentication-contract.md) and [development settings](../workflow/development.md#staged-account-api).

The [account route service](../../apps/api/internal/routes/accounts.go) locks the account/session and route while applying lifecycle operations. Existing lifecycle repository transactions run as savepoints inside that authority transaction; snapshots use account usernames and membership user IDs. [My routes](../../apps/api/internal/routes/account_list.go) uses safe summaries and account/filter-bound signed keyset cursors. [Socket tickets](../../apps/api/internal/routes/account_live.go) bind session/route/membership, consume once and are rechecked before live commands and outbound data. [Account sockets](../../apps/api/internal/httpapi/account_connections.go) permit independent viewers, with [persisted sharing generations and deadlines](../../apps/api/internal/routes/account_presence.go). Auth revocation releases leases transactionally, then closes affected sockets synchronously. The entry point starts and joins the live worker; it holds a singleton advisory lock, recovers abandoned account state before serving and stops account operations on lock loss. Legacy protocol helpers remain tested but are not mounted by the application. Account mode never issues or accepts independent member/owner credentials, and legacy credentials cannot grant account route access.

Optional [Google authentication](../../apps/api/internal/auth/google.go) uses `go-oidc` for signature/standard-claim verification and cached Google keys, with transport-bound nonce flows and transactional subject/email linking. Third-party email ownership uses the existing encrypted outbox. No Google callback, SDK or client UI is included in this backend increment. [Google configuration](../workflow/authentication-google.md) owns audience and flow settings.

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
