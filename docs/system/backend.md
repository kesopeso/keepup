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

The staged account service is in [auth](../../apps/api/internal/auth/service.go), with [dedicated HTTP handling](../../apps/api/internal/httpapi/auth.go). Startup mounts `/auth/` and replaces route/socket authority when `AUTH_ENABLED=true`; account endpoints bypass legacy wildcard CORS middleware. Enabled startup also runs the [delivery/cleanup worker](../../apps/api/internal/auth/outbox.go), joined before pool shutdown, with an injected route ticket cleanup callback. Verification/recovery use transactional purpose tokens and encrypted mail queueing through an interchangeable SMTP/fake provider. The [account contract](authentication-contract.md) separates delivered endpoints from future integration. Existing clients still use legacy authority while account mode is disabled. Configuration and enablement are in the [development workflow](../workflow/development.md#staged-account-api) and [email configuration](../workflow/authentication-email.md).

The [account route service](../../apps/api/internal/routes/accounts.go) locks the account/session and route while applying lifecycle operations. Existing lifecycle repository transactions run as savepoints inside that authority transaction; snapshots use account usernames and membership user IDs. [My routes](../../apps/api/internal/routes/account_list.go) uses safe summaries and account/filter-bound signed keyset cursors. [Socket tickets](../../apps/api/internal/routes/account_live.go) bind session/route/membership, consume once and are rechecked before live commands and outbound data. The existing connection restriction/presence timers remain until B6 adds exclusive sharing leases and immediate revocation cleanup. Account mode never issues or accepts independent member/owner credentials, and legacy mode cannot join account-owned routes.

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
